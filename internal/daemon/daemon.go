package daemon

import (
	"context"
	"fmt"
	"os"
	"os/signal"
	"path/filepath"
	"strings"
	"sync"
	"syscall"
	"time"

	"github.com/dorkusprime/wolfcastle/internal/config"
	"github.com/dorkusprime/wolfcastle/internal/inbox"
	"github.com/dorkusprime/wolfcastle/internal/invoke"
	"github.com/dorkusprime/wolfcastle/internal/logging"
	"github.com/dorkusprime/wolfcastle/internal/pipeline"
	"github.com/dorkusprime/wolfcastle/internal/state"
	"github.com/dorkusprime/wolfcastle/internal/tree"
)

// Daemon is the main Wolfcastle daemon loop.
type Daemon struct {
	Config        *config.Config
	WolfcastleDir string
	Resolver      *tree.Resolver
	ScopeNode     string
	Logger        *logging.Logger
	RepoDir       string

	shutdown     chan struct{}
	shutdownOnce sync.Once
	branch       string
}

// New creates a new daemon.
func New(cfg *config.Config, wolfcastleDir string, resolver *tree.Resolver, scopeNode string, repoDir string) (*Daemon, error) {
	logDir := filepath.Join(wolfcastleDir, "logs")
	logger, err := logging.NewLogger(logDir)
	if err != nil {
		return nil, err
	}

	return &Daemon{
		Config:        cfg,
		WolfcastleDir: wolfcastleDir,
		Resolver:      resolver,
		ScopeNode:     scopeNode,
		Logger:        logger,
		RepoDir:       repoDir,
		shutdown:      make(chan struct{}),
	}, nil
}

// Run executes the daemon loop.
func (d *Daemon) Run(ctx context.Context) error {
	// Set up signal handling
	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, syscall.SIGINT, syscall.SIGTERM)
	go func() {
		<-sigCh
		d.shutdownOnce.Do(func() { close(d.shutdown) })
	}()

	// Record starting branch
	if d.Config.Git.VerifyBranch {
		var err error
		d.branch, err = currentBranch(d.RepoDir)
		if err != nil {
			return fmt.Errorf("getting current branch: %w", err)
		}
	}

	iteration := 0
	maxIter := d.Config.Daemon.MaxIterations

	fmt.Printf("=== Wolfcastle starting (scope=%s) ===\n", d.scopeLabel())

	for {
		// Check shutdown
		select {
		case <-d.shutdown:
			fmt.Println("=== Wolfcastle stopped by signal ===")
			return nil
		default:
		}

		// Max iterations check
		if maxIter > 0 && iteration >= maxIter {
			fmt.Printf("=== Wolfcastle hit iteration cap (%d) ===\n", maxIter)
			return nil
		}

		// Verify branch hasn't changed
		if d.Config.Git.VerifyBranch {
			current, err := currentBranch(d.RepoDir)
			if err == nil && current != d.branch {
				return fmt.Errorf("WOLFCASTLE_BLOCKED: branch changed from %s to %s", d.branch, current)
			}
		}

		// Navigate to find work
		idx, err := d.Resolver.LoadRootIndex()
		if err != nil {
			return fmt.Errorf("loading root index: %w", err)
		}

		navResult, err := state.FindNextTask(idx, d.ScopeNode, func(addr string) (*state.NodeState, error) {
			a, err := tree.ParseAddress(addr)
			if err != nil {
				return nil, fmt.Errorf("parsing address %q: %w", addr, err)
			}
			return state.LoadNodeState(filepath.Join(d.Resolver.ProjectsDir(), filepath.Join(a.Parts...), "state.json"))
		})
		if err != nil {
			return fmt.Errorf("navigation failed: %w", err)
		}

		if !navResult.Found {
			if navResult.Reason == "all_complete" {
				fmt.Println("WOLFCASTLE_COMPLETE")
				time.Sleep(time.Duration(d.Config.Daemon.BlockedPollIntervalSeconds) * time.Second)
				continue
			}
			fmt.Printf("No work: %s — sleeping %ds\n", navResult.Reason, d.Config.Daemon.BlockedPollIntervalSeconds)
			time.Sleep(time.Duration(d.Config.Daemon.BlockedPollIntervalSeconds) * time.Second)
			continue
		}

		iteration++
		fmt.Printf("--- Iteration %d: %s/%s ---\n", iteration, navResult.NodeAddress, navResult.TaskID)

		// Start iteration log
		d.Logger.StartIteration()

		// Run pipeline stages
		err = d.runIteration(ctx, navResult, idx)
		d.Logger.Close()

		if err != nil {
			fmt.Printf("Iteration error: %v\n", err)
			time.Sleep(time.Duration(d.Config.Daemon.PollIntervalSeconds) * time.Second)
			continue
		}

		// Log retention
		logging.EnforceRetention(
			filepath.Join(d.WolfcastleDir, "logs"),
			d.Config.Logs.MaxFiles,
			d.Config.Logs.MaxAgeDays,
		)

		time.Sleep(time.Duration(d.Config.Daemon.PollIntervalSeconds) * time.Second)
	}
}

func (d *Daemon) runIteration(ctx context.Context, nav *state.NavigationResult, idx *state.RootIndex) error {
	// Claim the task
	addr, err := tree.ParseAddress(nav.NodeAddress)
	if err != nil {
		return fmt.Errorf("parsing node address %q: %w", nav.NodeAddress, err)
	}
	statePath := filepath.Join(d.Resolver.ProjectsDir(), filepath.Join(addr.Parts...), "state.json")
	ns, err := state.LoadNodeState(statePath)
	if err != nil {
		return err
	}

	if err := state.TaskClaim(ns, nav.TaskID); err != nil {
		return err
	}
	if err := state.SaveNodeState(statePath, ns); err != nil {
		return err
	}

	// Update root index
	if entry, ok := idx.Nodes[nav.NodeAddress]; ok {
		entry.State = ns.State
		idx.Nodes[nav.NodeAddress] = entry
		if err := state.SaveRootIndex(d.Resolver.RootIndexPath(), idx); err != nil {
			return fmt.Errorf("saving root index after claim: %w", err)
		}
	}

	// Run pipeline stages
	for _, stage := range d.Config.Pipeline.Stages {
		if !stage.IsEnabled() {
			continue
		}

		switch stage.Name {
		case "expand":
			if err := d.runExpandStage(ctx, stage); err != nil {
				d.Logger.Log(map[string]any{"type": "stage_error", "stage": "expand", "error": err.Error()})
				// Non-fatal: expand failure doesn't block execution
				fmt.Printf("  Expand stage error (non-fatal): %v\n", err)
			}
			continue

		case "file":
			if err := d.runFileStage(ctx, stage); err != nil {
				d.Logger.Log(map[string]any{"type": "stage_error", "stage": "file", "error": err.Error()})
				fmt.Printf("  File stage error (non-fatal): %v\n", err)
			}
			continue
		}

		// Execute stage (and any other custom stages)
		iterCtx := pipeline.BuildIterationContext(nav.NodeAddress, ns, nav.TaskID)

		prompt, err := pipeline.AssemblePrompt(d.WolfcastleDir, d.Config, stage, iterCtx)
		if err != nil {
			return fmt.Errorf("assembling prompt for stage %s: %w", stage.Name, err)
		}

		model, ok := d.Config.Models[stage.Model]
		if !ok {
			return fmt.Errorf("model %q not found for stage %s", stage.Model, stage.Name)
		}

		invokeCtx := ctx
		var cancel context.CancelFunc
		if d.Config.Daemon.InvocationTimeoutSeconds > 0 {
			invokeCtx, cancel = context.WithTimeout(ctx, time.Duration(d.Config.Daemon.InvocationTimeoutSeconds)*time.Second)
		}

		d.Logger.Log(map[string]any{
			"type":  "stage_start",
			"stage": stage.Name,
			"model": stage.Model,
			"node":  nav.NodeAddress,
			"task":  nav.TaskID,
		})

		result, err := invoke.InvokeStreaming(invokeCtx, model, prompt, d.RepoDir, d.Logger.AssistantWriter())
		if cancel != nil {
			cancel()
		}
		if err != nil {
			d.Logger.Log(map[string]any{"type": "stage_error", "stage": stage.Name, "error": err.Error()})
			return err
		}

		d.Logger.Log(map[string]any{
			"type":       "stage_complete",
			"stage":      stage.Name,
			"exit_code":  result.ExitCode,
			"output_len": len(result.Stdout),
		})

		// Check for terminal markers
		if strings.Contains(result.Stdout, "WOLFCASTLE_YIELD") {
			fmt.Println("  Task yielded successfully")
			return nil
		}
		if strings.Contains(result.Stdout, "WOLFCASTLE_BLOCKED") {
			fmt.Printf("  Task blocked: %s/%s\n", nav.NodeAddress, nav.TaskID)
			return nil
		}
		if strings.Contains(result.Stdout, "WOLFCASTLE_COMPLETE") {
			fmt.Println("WOLFCASTLE_COMPLETE")
			return nil
		}

		if result.Stdout == "" {
			fmt.Println("  WARNING: Empty output from model")
		} else {
			fmt.Println("  WARNING: WOLFCASTLE_YIELD not detected")
		}

		// Increment failure count for non-yielding output
		failCount, err := state.IncrementFailure(ns, nav.TaskID)
		if err != nil {
			d.Logger.Log(map[string]any{"type": "failure_increment_error", "error": err.Error()})
		} else {
			d.Logger.Log(map[string]any{"type": "failure_increment", "task": nav.TaskID, "count": failCount})

			if failCount == d.Config.Failure.DecompositionThreshold {
				if ns.DecompositionDepth < d.Config.Failure.MaxDecompositionDepth {
					fmt.Printf("  Decomposition threshold reached for %s/%s (depth=%d)\n", nav.NodeAddress, nav.TaskID, ns.DecompositionDepth)
					d.Logger.Log(map[string]any{"type": "decomposition_threshold", "task": nav.TaskID, "depth": ns.DecompositionDepth})
				} else {
					fmt.Printf("  Auto-blocking %s/%s: decomposition threshold at max depth\n", nav.NodeAddress, nav.TaskID)
					d.Logger.Log(map[string]any{"type": "auto_block", "task": nav.TaskID, "reason": "max_decomposition_depth"})
					if blockErr := state.TaskBlock(ns, nav.TaskID, "auto-blocked: decomposition threshold reached at max depth"); blockErr != nil {
						d.Logger.Log(map[string]any{"type": "auto_block_error", "task": nav.TaskID, "error": blockErr.Error()})
					}
				}
			}

			if failCount >= d.Config.Failure.HardCap && d.Config.Failure.HardCap > 0 {
				fmt.Printf("  Auto-blocking %s/%s: hard cap reached (%d)\n", nav.NodeAddress, nav.TaskID, failCount)
				d.Logger.Log(map[string]any{"type": "auto_block", "task": nav.TaskID, "reason": "hard_cap"})
				if blockErr := state.TaskBlock(ns, nav.TaskID, fmt.Sprintf("auto-blocked: failure hard cap reached (%d)", failCount)); blockErr != nil {
					d.Logger.Log(map[string]any{"type": "auto_block_error", "task": nav.TaskID, "error": blockErr.Error()})
				}
			}

			if err := state.SaveNodeState(statePath, ns); err != nil {
				d.Logger.Log(map[string]any{"type": "save_error", "error": err.Error()})
			}
		}
	}

	return nil
}

func (d *Daemon) runExpandStage(ctx context.Context, stage config.PipelineStage) error {
	inboxPath := filepath.Join(d.Resolver.ProjectsDir(), "inbox.json")
	inboxData, err := inbox.Load(inboxPath)
	if err != nil {
		return nil // No inbox file = nothing to expand
	}

	// Filter to only "new" status items
	var newItems []inbox.Item
	var newIndices []int
	for i, item := range inboxData.Items {
		if item.Status == "new" {
			newItems = append(newItems, item)
			newIndices = append(newIndices, i)
		}
	}
	if len(newItems) == 0 {
		return nil
	}

	model, ok := d.Config.Models[stage.Model]
	if !ok {
		return fmt.Errorf("model %q not found", stage.Model)
	}

	// Build context with only new items
	var itemsCtx strings.Builder
	itemsCtx.WriteString("# Inbox Items to Expand\n\n")
	for i, item := range newItems {
		itemsCtx.WriteString(fmt.Sprintf("### Item %d\n", i+1))
		itemsCtx.WriteString(fmt.Sprintf("- **Timestamp:** %s\n", item.Timestamp))
		itemsCtx.WriteString(fmt.Sprintf("- **Text:** %s\n\n", item.Text))
	}

	prompt, err := pipeline.AssemblePrompt(d.WolfcastleDir, d.Config, stage, itemsCtx.String())
	if err != nil {
		return err
	}

	d.Logger.Log(map[string]any{"type": "stage_start", "stage": "expand", "new_items": len(newItems)})

	invokeCtx, cancel := context.WithTimeout(ctx, time.Duration(d.Config.Daemon.InvocationTimeoutSeconds)*time.Second)
	defer cancel()

	result, err := invoke.InvokeStreaming(invokeCtx, model, prompt, d.RepoDir, d.Logger.AssistantWriter())
	if err != nil {
		return err
	}

	d.Logger.Log(map[string]any{
		"type":       "stage_complete",
		"stage":      "expand",
		"exit_code":  result.ExitCode,
		"output_len": len(result.Stdout),
	})

	// Parse model output — split on ## headings as item boundaries
	sections := parseExpandedSections(result.Stdout)

	// Match sections to new items (by position)
	for i, idx := range newIndices {
		inboxData.Items[idx].Status = "expanded"
		if i < len(sections) {
			inboxData.Items[idx].Expanded = strings.TrimSpace(sections[i])
		} else {
			// If the model returned fewer sections than items, still mark expanded
			inboxData.Items[idx].Expanded = ""
		}
	}

	if err := inbox.Save(inboxPath, inboxData); err != nil {
		return fmt.Errorf("saving inbox after expand: %w", err)
	}

	fmt.Printf("  Expand stage: %d items expanded\n", len(newItems))
	return nil
}

// parseExpandedSections splits model output on ## headings and returns
// the content of each section (heading included).
func parseExpandedSections(output string) []string {
	lines := strings.Split(output, "\n")
	var sections []string
	var current strings.Builder
	inSection := false

	for _, line := range lines {
		if strings.HasPrefix(line, "## ") {
			if inSection {
				sections = append(sections, current.String())
				current.Reset()
			}
			inSection = true
		}
		if inSection {
			if current.Len() > 0 {
				current.WriteString("\n")
			}
			current.WriteString(line)
		}
	}
	if inSection && current.Len() > 0 {
		sections = append(sections, current.String())
	}
	return sections
}

func (d *Daemon) runFileStage(ctx context.Context, stage config.PipelineStage) error {
	inboxPath := filepath.Join(d.Resolver.ProjectsDir(), "inbox.json")
	inboxData, err := inbox.Load(inboxPath)
	if err != nil {
		return nil
	}

	// Filter to only "expanded" status items
	var expandedIndices []int
	for i, item := range inboxData.Items {
		if item.Status == "expanded" {
			expandedIndices = append(expandedIndices, i)
		}
	}
	if len(expandedIndices) == 0 {
		return nil
	}

	model, ok := d.Config.Models[stage.Model]
	if !ok {
		return fmt.Errorf("model %q not found", stage.Model)
	}

	// Build context with expanded items
	var itemsCtx strings.Builder
	itemsCtx.WriteString("# Expanded Inbox Items to File\n\n")
	for _, idx := range expandedIndices {
		item := inboxData.Items[idx]
		itemsCtx.WriteString(fmt.Sprintf("---\n\n**Original:** %s\n\n", item.Text))
		if item.Expanded != "" {
			itemsCtx.WriteString(item.Expanded)
			itemsCtx.WriteString("\n\n")
		}
	}

	prompt, err := pipeline.AssemblePrompt(d.WolfcastleDir, d.Config, stage, itemsCtx.String())
	if err != nil {
		return err
	}

	d.Logger.Log(map[string]any{"type": "stage_start", "stage": "file", "expanded_items": len(expandedIndices)})

	invokeCtx, cancel := context.WithTimeout(ctx, time.Duration(d.Config.Daemon.InvocationTimeoutSeconds)*time.Second)
	defer cancel()

	// The model executes wolfcastle commands directly via tool calls
	result, err := invoke.InvokeStreaming(invokeCtx, model, prompt, d.RepoDir, d.Logger.AssistantWriter())
	if err != nil {
		return err
	}

	d.Logger.Log(map[string]any{
		"type":       "stage_complete",
		"stage":      "file",
		"exit_code":  result.ExitCode,
		"output_len": len(result.Stdout),
	})

	// Mark all expanded items as filed
	for _, idx := range expandedIndices {
		inboxData.Items[idx].Status = "filed"
	}

	if err := inbox.Save(inboxPath, inboxData); err != nil {
		return fmt.Errorf("saving inbox after file stage: %w", err)
	}

	fmt.Printf("  File stage: %d items filed\n", len(expandedIndices))
	return nil
}

func (d *Daemon) runSummaryStage(ctx context.Context, nodeAddr string, statePath string, ns *state.NodeState) error {
	model, ok := d.Config.Models[d.Config.Summary.Model]
	if !ok {
		return fmt.Errorf("summary model %q not found", d.Config.Summary.Model)
	}

	// Build summary context from breadcrumbs and audit state
	var summaryCtx strings.Builder
	summaryCtx.WriteString("# Summary Request\n\n")
	summaryCtx.WriteString(fmt.Sprintf("**Node:** %s\n", nodeAddr))
	summaryCtx.WriteString(fmt.Sprintf("**State:** %s\n\n", ns.State))

	if len(ns.Audit.Breadcrumbs) > 0 {
		summaryCtx.WriteString("## Breadcrumbs\n\n")
		for _, bc := range ns.Audit.Breadcrumbs {
			summaryCtx.WriteString(fmt.Sprintf("- [%s] %s: %s\n", bc.Timestamp.Format("2006-01-02 15:04:05"), bc.Task, bc.Text))
		}
		summaryCtx.WriteString("\n")
	}

	summaryCtx.WriteString("## Audit Status\n\n")
	summaryCtx.WriteString(fmt.Sprintf("Status: %s\n", ns.Audit.Status))
	if len(ns.Audit.Gaps) > 0 {
		summaryCtx.WriteString(fmt.Sprintf("Gaps: %d\n", len(ns.Audit.Gaps)))
	}
	if len(ns.Audit.Escalations) > 0 {
		summaryCtx.WriteString(fmt.Sprintf("Escalations: %d\n", len(ns.Audit.Escalations)))
	}

	// Resolve summary prompt file
	promptContent, err := pipeline.ResolveFragment(d.WolfcastleDir, "prompts/"+d.Config.Summary.PromptFile)
	if err != nil {
		return fmt.Errorf("resolving summary prompt: %w", err)
	}

	prompt := promptContent + "\n\n---\n\n" + summaryCtx.String()

	d.Logger.Log(map[string]any{
		"type":  "stage_start",
		"stage": "summary",
		"model": d.Config.Summary.Model,
		"node":  nodeAddr,
	})

	invokeCtx, cancel := context.WithTimeout(ctx, time.Duration(d.Config.Daemon.InvocationTimeoutSeconds)*time.Second)
	defer cancel()

	result, err := invoke.InvokeStreaming(invokeCtx, model, prompt, d.RepoDir, d.Logger.AssistantWriter())
	if err != nil {
		return err
	}

	d.Logger.Log(map[string]any{
		"type":       "stage_complete",
		"stage":      "summary",
		"exit_code":  result.ExitCode,
		"output_len": len(result.Stdout),
	})

	// Store summary result in node state
	ns.Audit.ResultSummary = strings.TrimSpace(result.Stdout)
	if err := state.SaveNodeState(statePath, ns); err != nil {
		return fmt.Errorf("saving summary to node state: %w", err)
	}

	fmt.Printf("  Summary generated for node %s\n", nodeAddr)
	return nil
}

func (d *Daemon) scopeLabel() string {
	if d.ScopeNode != "" {
		return d.ScopeNode
	}
	return "full tree"
}
