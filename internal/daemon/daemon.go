package daemon

import (
	"context"
	"fmt"
	"io"
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
	iteration    int
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

// selfHeal scans the tree for stale in_progress tasks on startup (ADR-020).
func (d *Daemon) selfHeal() error {
	fmt.Println("Running self-healing check...")
	idx, err := d.Resolver.LoadRootIndex()
	if err != nil {
		fmt.Println("No root index found — nothing to heal.")
		return nil
	}

	var inProgress []struct{ addr, taskID string }
	for addr, entry := range idx.Nodes {
		if entry.Type != state.NodeLeaf {
			continue
		}
		a, err := tree.ParseAddress(addr)
		if err != nil {
			continue
		}
		ns, err := state.LoadNodeState(filepath.Join(d.Resolver.ProjectsDir(), filepath.Join(a.Parts...), "state.json"))
		if err != nil {
			continue
		}
		for _, t := range ns.Tasks {
			if t.State == state.StatusInProgress {
				inProgress = append(inProgress, struct{ addr, taskID string }{addr, t.ID})
			}
		}
	}

	if len(inProgress) > 1 {
		return fmt.Errorf("state corruption: %d tasks in progress (serial execution requires at most 1)", len(inProgress))
	}
	if len(inProgress) == 1 {
		fmt.Printf("Found interrupted task: %s/%s — will resume on next iteration\n",
			inProgress[0].addr, inProgress[0].taskID)
	} else {
		fmt.Println("No interrupted tasks found.")
	}
	return nil
}

// RunWithSupervisor wraps Run with crash recovery and configurable restarts.
func (d *Daemon) RunWithSupervisor(ctx context.Context) error {
	maxRestarts := d.Config.Daemon.MaxRestarts
	delay := time.Duration(d.Config.Daemon.RestartDelaySeconds) * time.Second

	for restart := 0; ; restart++ {
		err := d.Run(ctx)
		if err == nil || ctx.Err() != nil {
			return err
		}
		if restart >= maxRestarts {
			return fmt.Errorf("daemon exceeded max restarts (%d): %w", maxRestarts, err)
		}
		fmt.Printf("Daemon crashed (attempt %d/%d): %v — restarting in %v\n", restart+1, maxRestarts, err, delay)
		time.Sleep(delay)

		// Reset daemon state for next Run() invocation
		d.shutdown = make(chan struct{})
		d.shutdownOnce = sync.Once{}
		d.iteration = 0
	}
}

// IterationResult describes the outcome of a single daemon iteration.
type IterationResult int

const (
	// IterationDidWork means work was found and the pipeline ran.
	IterationDidWork IterationResult = iota
	// IterationNoWork means no actionable tasks were found.
	IterationNoWork
	// IterationStop means the daemon should shut down (signal, stop file, cap).
	IterationStop
	// IterationError means the iteration encountered a recoverable error.
	IterationError
)

// Run executes the daemon loop.
func (d *Daemon) Run(ctx context.Context) error {
	// Root the daemon in a cancelable signal context so SIGINT/SIGTERM
	// cancels in-flight model invocations (ADR-024 shutdown compliance).
	ctx, cancel := signal.NotifyContext(ctx, syscall.SIGINT, syscall.SIGTERM)
	defer cancel()

	// Also close the shutdown channel for backward compatibility with
	// stop-file and supervisor checks.
	go func() {
		<-ctx.Done()
		d.shutdownOnce.Do(func() { close(d.shutdown) })
	}()

	// Self-healing phase (ADR-020)
	if err := d.selfHeal(); err != nil {
		return fmt.Errorf("self-healing failed: %w", err)
	}

	// Record starting branch
	if d.Config.Git.VerifyBranch {
		var err error
		d.branch, err = currentBranch(d.RepoDir)
		if err != nil {
			return fmt.Errorf("getting current branch: %w", err)
		}
	}

	d.iteration = 0
	d.Logger.Log(map[string]any{"type": "daemon_start", "scope": d.scopeLabel()})
	fmt.Printf("=== Wolfcastle starting (scope=%s) ===\n", d.scopeLabel())

	for {
		result, err := d.RunOnce(ctx)
		if err != nil {
			return err
		}

		switch result {
		case IterationStop:
			return nil
		case IterationNoWork:
			time.Sleep(time.Duration(d.Config.Daemon.BlockedPollIntervalSeconds) * time.Second)
		case IterationError:
			time.Sleep(time.Duration(d.Config.Daemon.PollIntervalSeconds) * time.Second)
		case IterationDidWork:
			logging.EnforceRetention(
				filepath.Join(d.WolfcastleDir, "logs"),
				d.Config.Logs.MaxFiles,
				d.Config.Logs.MaxAgeDays,
			)
			time.Sleep(time.Duration(d.Config.Daemon.PollIntervalSeconds) * time.Second)
		}
	}
}

// RunOnce executes a single daemon iteration: check preconditions, find work,
// and run the pipeline. Returns a result indicating what happened and a
// non-nil error only for fatal conditions that should halt the daemon.
func (d *Daemon) RunOnce(ctx context.Context) (IterationResult, error) {
	// Check shutdown signal
	select {
	case <-d.shutdown:
		d.Logger.Log(map[string]any{"type": "daemon_stop", "reason": "signal"})
		fmt.Println("=== Wolfcastle stopped by signal ===")
		return IterationStop, nil
	default:
	}

	// Check stop file
	stopFilePath := filepath.Join(d.WolfcastleDir, "stop")
	if _, err := os.Stat(stopFilePath); err == nil {
		os.Remove(stopFilePath)
		d.Logger.Log(map[string]any{"type": "daemon_stop", "reason": "stop_file"})
		fmt.Println("=== Wolfcastle stopped by stop file ===")
		return IterationStop, nil
	}

	// Max iterations check
	maxIter := d.Config.Daemon.MaxIterations
	if maxIter > 0 && d.iteration >= maxIter {
		d.Logger.Log(map[string]any{"type": "daemon_stop", "reason": "iteration_cap", "iterations": d.iteration})
		fmt.Printf("=== Wolfcastle hit iteration cap (%d) ===\n", maxIter)
		return IterationStop, nil
	}

	// Verify branch hasn't changed
	if d.Config.Git.VerifyBranch {
		current, err := currentBranch(d.RepoDir)
		if err == nil && current != d.branch {
			return IterationStop, fmt.Errorf("WOLFCASTLE_BLOCKED: branch changed from %s to %s", d.branch, current)
		}
	}

	// Navigate to find work
	idx, err := d.Resolver.LoadRootIndex()
	if err != nil {
		return IterationStop, fmt.Errorf("loading root index: %w", err)
	}

	navResult, err := state.FindNextTask(idx, d.ScopeNode, func(addr string) (*state.NodeState, error) {
		a, err := tree.ParseAddress(addr)
		if err != nil {
			return nil, fmt.Errorf("parsing address %q: %w", addr, err)
		}
		return state.LoadNodeState(filepath.Join(d.Resolver.ProjectsDir(), filepath.Join(a.Parts...), "state.json"))
	})
	if err != nil {
		return IterationStop, fmt.Errorf("navigation failed: %w", err)
	}

	if !navResult.Found {
		if navResult.Reason == "all_complete" {
			fmt.Println("WOLFCASTLE_COMPLETE")
		} else {
			fmt.Printf("No work: %s — sleeping %ds\n", navResult.Reason, d.Config.Daemon.BlockedPollIntervalSeconds)
		}
		return IterationNoWork, nil
	}

	d.iteration++
	fmt.Printf("--- Iteration %d: %s/%s ---\n", d.iteration, navResult.NodeAddress, navResult.TaskID)

	// Start iteration log
	d.Logger.StartIteration()

	// Run pipeline stages
	err = d.runIteration(ctx, navResult, idx)
	d.Logger.Close()

	if err != nil {
		fmt.Printf("Iteration error: %v\n", err)
		return IterationError, nil
	}

	return IterationDidWork, nil
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

	// Propagate claim state to ancestors and root index (ADR-024)
	if err := d.propagateState(nav.NodeAddress, ns.State, idx); err != nil {
		return fmt.Errorf("propagating state after claim: %w", err)
	}

	// Check inbox state once for stage-skip decisions (ADR-039)
	inboxPath := filepath.Join(d.Resolver.ProjectsDir(), "inbox.json")
	hasNewItems, hasExpandedItems := d.checkInboxState(inboxPath)

	// Run pipeline stages
	for _, stage := range d.Config.Pipeline.Stages {
		if !stage.IsEnabled() {
			continue
		}

		switch stage.Name {
		case "expand":
			if !hasNewItems {
				d.Logger.Log(map[string]any{"type": "stage_skip", "stage": "expand", "reason": "no_new_inbox_items"})
				continue
			}
			if err := d.runExpandStage(ctx, stage); err != nil {
				d.Logger.Log(map[string]any{"type": "stage_error", "stage": "expand", "error": err.Error()})
				// Non-fatal: expand failure doesn't block execution
				fmt.Printf("  Expand stage error (non-fatal): %v\n", err)
			}
			// Re-check inbox state after expand — items may now be expanded
			hasNewItems, hasExpandedItems = d.checkInboxState(inboxPath)
			continue

		case "file":
			if !hasExpandedItems {
				d.Logger.Log(map[string]any{"type": "stage_skip", "stage": "file", "reason": "no_expanded_inbox_items"})
				continue
			}
			if err := d.runFileStage(ctx, stage); err != nil {
				d.Logger.Log(map[string]any{"type": "stage_error", "stage": "file", "error": err.Error()})
				fmt.Printf("  File stage error (non-fatal): %v\n", err)
			}
			continue
		}

		// Skip execute stage if there are expanded items awaiting filing —
		// prioritize filing over execution to avoid working on a stale tree.
		if hasExpandedItems {
			d.Logger.Log(map[string]any{"type": "stage_skip", "stage": stage.Name, "reason": "pending_filing"})
			fmt.Printf("  Skipping %s stage: expanded items await filing\n", stage.Name)
			continue
		}

		// Execute stage (and any other custom stages)
		iterCtx := pipeline.BuildIterationContext(nav.NodeAddress, ns, nav.TaskID, d.Config)

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

		result, err := d.invokeWithRetry(invokeCtx, model, prompt, d.RepoDir, d.Logger.AssistantWriter(), stage.Name)
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

		// Parse mutation markers from model output
		d.applyModelMarkers(result.Stdout, ns, nav)

		// Sync audit lifecycle after marker mutations (Item 2)
		state.SyncAuditLifecycle(ns)

		// Persist marker mutations immediately — ensures durability even if
		// the stage errors before reaching a terminal marker (Item 6)
		if err := state.SaveNodeState(statePath, ns); err != nil {
			d.Logger.Log(map[string]any{"type": "save_error", "error": err.Error()})
		}
		if err := d.propagateState(nav.NodeAddress, ns.State, idx); err != nil {
			d.Logger.Log(map[string]any{"type": "propagate_error", "error": err.Error()})
		}

		// Check for terminal markers
		if strings.Contains(result.Stdout, "WOLFCASTLE_YIELD") {
			d.Logger.Log(map[string]any{"type": "terminal_marker", "marker": "WOLFCASTLE_YIELD"})
			return nil
		}
		if strings.Contains(result.Stdout, "WOLFCASTLE_BLOCKED") {
			d.Logger.Log(map[string]any{"type": "terminal_marker", "marker": "WOLFCASTLE_BLOCKED", "task": nav.TaskID})
			return nil
		}
		if strings.Contains(result.Stdout, "WOLFCASTLE_COMPLETE") {
			d.Logger.Log(map[string]any{"type": "terminal_marker", "marker": "WOLFCASTLE_COMPLETE"})
			return nil
		}

		// No terminal marker — increment failure count
		d.Logger.Log(map[string]any{
			"type":   "no_terminal_marker",
			"empty":  result.Stdout == "",
			"task":   nav.TaskID,
		})

		failCount, err := state.IncrementFailure(ns, nav.TaskID)
		if err != nil {
			d.Logger.Log(map[string]any{"type": "failure_increment_error", "error": err.Error()})
		} else {
			d.Logger.Log(map[string]any{"type": "failure_increment", "task": nav.TaskID, "count": failCount})

			if failCount >= d.Config.Failure.DecompositionThreshold && d.Config.Failure.DecompositionThreshold > 0 {
				if ns.DecompositionDepth < d.Config.Failure.MaxDecompositionDepth {
					d.Logger.Log(map[string]any{"type": "decomposition_threshold", "task": nav.TaskID, "depth": ns.DecompositionDepth})
					state.SetNeedsDecomposition(ns, nav.TaskID, true)
				} else {
					d.Logger.Log(map[string]any{"type": "auto_block", "task": nav.TaskID, "reason": "max_decomposition_depth"})
					if blockErr := state.TaskBlock(ns, nav.TaskID, "auto-blocked: decomposition threshold reached at max depth"); blockErr != nil {
						d.Logger.Log(map[string]any{"type": "auto_block_error", "task": nav.TaskID, "error": blockErr.Error()})
					}
				}
			}

			if failCount >= d.Config.Failure.HardCap && d.Config.Failure.HardCap > 0 {
				d.Logger.Log(map[string]any{"type": "auto_block", "task": nav.TaskID, "reason": "hard_cap", "count": failCount})
				if blockErr := state.TaskBlock(ns, nav.TaskID, fmt.Sprintf("auto-blocked: failure hard cap reached (%d)", failCount)); blockErr != nil {
					d.Logger.Log(map[string]any{"type": "auto_block_error", "task": nav.TaskID, "error": blockErr.Error()})
				}
			}

			if err := state.SaveNodeState(statePath, ns); err != nil {
				d.Logger.Log(map[string]any{"type": "save_error", "error": err.Error()})
			}
			if err := d.propagateState(nav.NodeAddress, ns.State, idx); err != nil {
				d.Logger.Log(map[string]any{"type": "propagate_error", "error": err.Error()})
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

	result, err := d.invokeWithRetry(invokeCtx, model, prompt, d.RepoDir, d.Logger.AssistantWriter(), "expand")
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
	result, err := d.invokeWithRetry(invokeCtx, model, prompt, d.RepoDir, d.Logger.AssistantWriter(), "file")
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

// applyModelMarkers parses WOLFCASTLE_* mutation markers from model output.
func (d *Daemon) applyModelMarkers(output string, ns *state.NodeState, nav *state.NavigationResult) {
	lines := strings.Split(output, "\n")
	for _, line := range lines {
		line = strings.TrimSpace(line)
		switch {
		case strings.HasPrefix(line, "WOLFCASTLE_BREADCRUMB:"):
			text := strings.TrimSpace(strings.TrimPrefix(line, "WOLFCASTLE_BREADCRUMB:"))
			if text != "" {
				state.AddBreadcrumb(ns, nav.NodeAddress+"/"+nav.TaskID, text)
				d.Logger.Log(map[string]any{"type": "marker_breadcrumb", "text": text})
			}

		case strings.HasPrefix(line, "WOLFCASTLE_GAP:"):
			desc := strings.TrimSpace(strings.TrimPrefix(line, "WOLFCASTLE_GAP:"))
			if desc != "" {
				gapID := fmt.Sprintf("gap-%s-%d", ns.ID, len(ns.Audit.Gaps)+1)
				ns.Audit.Gaps = append(ns.Audit.Gaps, state.Gap{
					ID:          gapID,
					Timestamp:   time.Now().UTC(),
					Description: desc,
					Source:      nav.NodeAddress,
					Status:      "open",
				})
				d.Logger.Log(map[string]any{"type": "marker_gap", "gap_id": gapID})
			}

		case strings.HasPrefix(line, "WOLFCASTLE_FIX_GAP:"):
			gapID := strings.TrimSpace(strings.TrimPrefix(line, "WOLFCASTLE_FIX_GAP:"))
			for i := range ns.Audit.Gaps {
				if ns.Audit.Gaps[i].ID == gapID && ns.Audit.Gaps[i].Status == "open" {
					ns.Audit.Gaps[i].Status = "fixed"
					ns.Audit.Gaps[i].FixedBy = nav.NodeAddress + "/" + nav.TaskID
					now := time.Now().UTC()
					ns.Audit.Gaps[i].FixedAt = &now
					d.Logger.Log(map[string]any{"type": "marker_fix_gap", "gap_id": gapID})
					break
				}
			}

		case strings.HasPrefix(line, "WOLFCASTLE_SCOPE:"):
			desc := strings.TrimSpace(strings.TrimPrefix(line, "WOLFCASTLE_SCOPE:"))
			if desc != "" {
				if ns.Audit.Scope == nil {
					ns.Audit.Scope = &state.AuditScope{}
				}
				ns.Audit.Scope.Description = desc
				d.Logger.Log(map[string]any{"type": "marker_scope", "description": desc})
			}

		case strings.HasPrefix(line, "WOLFCASTLE_SCOPE_FILES:"):
			raw := strings.TrimSpace(strings.TrimPrefix(line, "WOLFCASTLE_SCOPE_FILES:"))
			if ns.Audit.Scope == nil {
				ns.Audit.Scope = &state.AuditScope{}
			}
			ns.Audit.Scope.Files = dedupPipe(raw)

		case strings.HasPrefix(line, "WOLFCASTLE_SCOPE_SYSTEMS:"):
			raw := strings.TrimSpace(strings.TrimPrefix(line, "WOLFCASTLE_SCOPE_SYSTEMS:"))
			if ns.Audit.Scope == nil {
				ns.Audit.Scope = &state.AuditScope{}
			}
			ns.Audit.Scope.Systems = dedupPipe(raw)

		case strings.HasPrefix(line, "WOLFCASTLE_SCOPE_CRITERIA:"):
			raw := strings.TrimSpace(strings.TrimPrefix(line, "WOLFCASTLE_SCOPE_CRITERIA:"))
			if ns.Audit.Scope == nil {
				ns.Audit.Scope = &state.AuditScope{}
			}
			ns.Audit.Scope.Criteria = dedupPipe(raw)

		case strings.HasPrefix(line, "WOLFCASTLE_SUMMARY:"):
			text := strings.TrimSpace(strings.TrimPrefix(line, "WOLFCASTLE_SUMMARY:"))
			if text != "" {
				ns.Audit.ResultSummary = text
				d.Logger.Log(map[string]any{"type": "marker_summary", "text": text})
			}

		case strings.HasPrefix(line, "WOLFCASTLE_RESOLVE_ESCALATION:"):
			escID := strings.TrimSpace(strings.TrimPrefix(line, "WOLFCASTLE_RESOLVE_ESCALATION:"))
			for i := range ns.Audit.Escalations {
				if ns.Audit.Escalations[i].ID == escID && ns.Audit.Escalations[i].Status == "open" {
					ns.Audit.Escalations[i].Status = "resolved"
					ns.Audit.Escalations[i].ResolvedBy = nav.NodeAddress + "/" + nav.TaskID
					now := time.Now().UTC()
					ns.Audit.Escalations[i].ResolvedAt = &now
					d.Logger.Log(map[string]any{"type": "marker_resolve_escalation", "escalation_id": escID})
					break
				}
			}
		}
	}
}

// dedupPipe splits a pipe-delimited string and deduplicates entries.
func dedupPipe(s string) []string {
	seen := make(map[string]bool)
	var result []string
	for _, part := range strings.Split(s, "|") {
		trimmed := strings.TrimSpace(part)
		if trimmed != "" && !seen[trimmed] {
			seen[trimmed] = true
			result = append(result, trimmed)
		}
	}
	return result
}

// propagateState propagates a node's state to all ancestors and updates the
// root index. This ensures ADR-024 compliance for all daemon mutations.
func (d *Daemon) propagateState(nodeAddr string, nodeState state.NodeStatus, idx *state.RootIndex) error {
	loadNode := func(addr string) (*state.NodeState, error) {
		a, err := tree.ParseAddress(addr)
		if err != nil {
			return nil, fmt.Errorf("parsing address %q: %w", addr, err)
		}
		return state.LoadNodeState(filepath.Join(d.Resolver.ProjectsDir(), filepath.Join(a.Parts...), "state.json"))
	}

	saveNode := func(addr string, ns *state.NodeState) error {
		a, err := tree.ParseAddress(addr)
		if err != nil {
			return fmt.Errorf("parsing address %q: %w", addr, err)
		}
		return state.SaveNodeState(filepath.Join(d.Resolver.ProjectsDir(), filepath.Join(a.Parts...), "state.json"), ns)
	}

	if err := state.Propagate(nodeAddr, nodeState, idx, loadNode, saveNode); err != nil {
		return err
	}

	return state.SaveRootIndex(d.Resolver.RootIndexPath(), idx)
}

// checkInboxState returns whether the inbox has new items (needing expand)
// and expanded items (needing filing). Returns false, false if the inbox
// file doesn't exist or can't be read.
func (d *Daemon) checkInboxState(inboxPath string) (hasNew, hasExpanded bool) {
	inboxData, err := inbox.Load(inboxPath)
	if err != nil {
		return false, false
	}
	for _, item := range inboxData.Items {
		switch item.Status {
		case "new":
			hasNew = true
		case "expanded":
			hasExpanded = true
		}
	}
	return
}

func (d *Daemon) scopeLabel() string {
	if d.ScopeNode != "" {
		return d.ScopeNode
	}
	return "full tree"
}

// invokeWithRetry wraps invoke.InvokeStreaming with exponential backoff
// governed by the config's retries settings. Only invocation errors (non-nil
// error returns) are retried — a successful process exit (even with a
// non-zero exit code captured in Result) is not retried here.
func (d *Daemon) invokeWithRetry(ctx context.Context, model config.ModelDef, prompt string, workDir string, logWriter io.Writer, stageName string) (*invoke.Result, error) {
	rc := d.Config.Retries
	delay := time.Duration(rc.InitialDelaySeconds) * time.Second
	maxDelay := time.Duration(rc.MaxDelaySeconds) * time.Second

	for attempt := 0; ; attempt++ {
		result, err := invoke.InvokeStreaming(ctx, model, prompt, workDir, logWriter)
		if err == nil {
			return result, nil
		}

		// Context cancellation is not retryable — the daemon is shutting down.
		if ctx.Err() != nil {
			return result, err
		}

		// Check whether we've exhausted our retry budget (-1 = unlimited).
		if rc.MaxRetries >= 0 && attempt >= rc.MaxRetries {
			d.Logger.Log(map[string]any{
				"type":     "retry_exhausted",
				"stage":    stageName,
				"attempts": attempt + 1,
				"error":    err.Error(),
			})
			return result, err
		}

		d.Logger.Log(map[string]any{
			"type":    "retry",
			"stage":   stageName,
			"attempt": attempt + 1,
			"delay_s": delay.Seconds(),
			"error":   err.Error(),
		})
		fmt.Printf("  Invocation error (attempt %d): %v — retrying in %v\n", attempt+1, err, delay)

		select {
		case <-ctx.Done():
			return result, ctx.Err()
		case <-time.After(delay):
		}

		// Exponential backoff: double the delay, capped at max.
		delay *= 2
		if delay > maxDelay {
			delay = maxDelay
		}
	}
}
