package daemon

import (
	"context"
	"fmt"
	"path/filepath"
	"strings"
	"time"

	"github.com/fsnotify/fsnotify"

	"github.com/dorkusprime/wolfcastle/internal/config"
	werrors "github.com/dorkusprime/wolfcastle/internal/errors"
	"github.com/dorkusprime/wolfcastle/internal/output"
	"github.com/dorkusprime/wolfcastle/internal/pipeline"
	"github.com/dorkusprime/wolfcastle/internal/state"
)

// runInboxLoop is the parallel goroutine that watches for new inbox items
// and runs the intake stage. It tries fsnotify first for instant reaction
// to file changes, falling back to polling if the watcher can't be created.
func (d *Daemon) runInboxLoop(ctx context.Context) {
	projDir := d.Store.Dir()

	// Try fsnotify first. Watch the projects directory because
	// inbox.json might not exist yet when the daemon starts.
	watcher, err := fsnotify.NewWatcher()
	if err == nil {
		defer func() { _ = watcher.Close() }()
		if addErr := watcher.Add(projDir); addErr == nil {
			output.PrintHuman("Inbox watcher deployed.")
			d.runInboxWithFsnotify(ctx, watcher)
			return
		}
		_ = watcher.Close()
	}

	// Fallback: polling
	output.PrintHuman("Inbox watcher deployed. (polling)")
	d.runInboxWithPolling(ctx)
}

// runInboxWithFsnotify watches for inbox.json changes via fsnotify and
// runs the intake stage when new items appear.
func (d *Daemon) runInboxWithFsnotify(ctx context.Context, watcher *fsnotify.Watcher) {
	inboxCounter := 0
	inboxPath := filepath.Join(d.Store.Dir(), "inbox.json")

	// Check once at startup in case items were added while the daemon was down.
	if d.checkInboxForNew(inboxPath) {
		inboxCounter++
		d.processInbox(ctx, inboxCounter)
	}

	for {
		select {
		case <-ctx.Done():
			return
		case event, ok := <-watcher.Events:
			if !ok {
				return
			}
			// Only react to writes/creates on inbox.json
			if filepath.Base(event.Name) != "inbox.json" {
				continue
			}
			if event.Op&(fsnotify.Write|fsnotify.Create) == 0 {
				continue
			}
			if d.checkInboxForNew(inboxPath) {
				inboxCounter++
				d.processInbox(ctx, inboxCounter)
			}
		case err, ok := <-watcher.Errors:
			if !ok {
				return
			}
			output.PrintHuman("Inbox watcher error: %v", err)
		}
	}
}

// runInboxWithPolling is the fallback when fsnotify is unavailable.
func (d *Daemon) runInboxWithPolling(ctx context.Context) {
	pollInterval := time.Duration(d.Config.Daemon.InboxPollIntervalSeconds) * time.Second
	if pollInterval <= 0 {
		pollInterval = time.Duration(d.Config.Daemon.BlockedPollIntervalSeconds) * time.Second
	}
	if pollInterval <= 0 {
		pollInterval = 5 * time.Second
	}

	inboxCounter := 0
	inboxPath := filepath.Join(d.Store.Dir(), "inbox.json")

	for {
		select {
		case <-ctx.Done():
			return
		default:
		}

		if d.checkInboxForNew(inboxPath) {
			inboxCounter++
			d.processInbox(ctx, inboxCounter)
		}

		if !sleepWithContext(ctx, pollInterval) {
			return
		}
	}
}

// processInbox runs the intake stage for pending inbox items.
func (d *Daemon) processInbox(ctx context.Context, counter int) {
	output.PrintHuman("inbox-%04d: Processing...", counter)

	for _, stage := range d.Config.Pipeline.Stages {
		if stage.Name == "intake" && stage.IsEnabled() {
			_ = d.InboxLogger.StartIterationWithPrefix("intake")
			if err := d.runIntakeStage(ctx, stage); err != nil {
				output.PrintHuman("  Intake stage error (non-fatal): %v", err)
			}
			d.InboxLogger.Close()
			break
		}
	}
}

// checkInboxForNew returns whether the inbox has items with status "new".
func (d *Daemon) checkInboxForNew(inboxPath string) bool {
	inboxData, err := state.LoadInbox(inboxPath)
	if err != nil {
		return false
	}
	for _, item := range inboxData.Items {
		if item.Status == "new" {
			return true
		}
	}
	return false
}

// runIntakeStage processes inbox items by invoking a model that calls
// wolfcastle CLI commands directly to create projects and tasks. Items
// that are successfully processed (i.e., the model completes without
// error) are marked as "filed". Items that fail remain "new" for retry.
func (d *Daemon) runIntakeStage(ctx context.Context, stage config.PipelineStage) error {
	inboxPath := filepath.Join(d.Store.Dir(), "inbox.json")
	inboxData, err := state.LoadInbox(inboxPath)
	if err != nil {
		return nil // No inbox file = nothing to process
	}

	// Filter to only "new" status items
	var newItems []state.InboxItem
	for _, item := range inboxData.Items {
		if item.Status == "new" {
			newItems = append(newItems, item)
		}
	}
	if len(newItems) == 0 {
		return nil
	}

	// When planning is enabled, use the planning-aware intake prompt
	// that creates orchestrators instead of full project trees.
	intakeStage := stage
	if d.Config.Pipeline.Planning.Enabled {
		intakeStage.PromptFile = "intake-planning.md"
	}

	model, ok := d.Config.Models[intakeStage.Model]
	if !ok {
		return werrors.Config(fmt.Errorf("model %q not found", intakeStage.Model))
	}

	// Build context with inbox items
	var itemsCtx strings.Builder

	// Include current tree state so the model can file work under existing projects
	idx, err := d.Store.ReadIndex()
	if err == nil && len(idx.Nodes) > 0 {
		itemsCtx.WriteString("# Existing Project Tree\n\n")
		itemsCtx.WriteString("These projects already exist. File new work under existing projects when appropriate rather than creating duplicates.\n\n")
		for addr, entry := range idx.Nodes {
			fmt.Fprintf(&itemsCtx, "- **%s** (%s, %s)\n", entry.Name, entry.Type, entry.State)
			if entry.Parent != "" {
				fmt.Fprintf(&itemsCtx, "  Address: %s (child of %s)\n", addr, entry.Parent)
			} else {
				fmt.Fprintf(&itemsCtx, "  Address: %s\n", addr)
			}
		}
		itemsCtx.WriteString("\n")
	}

	intakeHeader := resolveContextHeader(d.WolfcastleDir, "intake-context.md", "# Inbox Items to Process\n")
	itemsCtx.WriteString(intakeHeader + "\n")
	for i, item := range newItems {
		fmt.Fprintf(&itemsCtx, "### Item %d\n", i+1)
		fmt.Fprintf(&itemsCtx, "- **Timestamp:** %s\n", item.Timestamp)
		fmt.Fprintf(&itemsCtx, "- **Text:** %s\n\n", item.Text)
	}

	prompt, err := pipeline.AssemblePrompt(d.WolfcastleDir, d.Config, intakeStage, itemsCtx.String())
	if err != nil {
		return err
	}

	_ = d.InboxLogger.Log(map[string]any{"type": "stage_start", "stage": "intake", "new_items": len(newItems)})

	invokeCtx := ctx
	if d.Config.Daemon.InvocationTimeoutSeconds > 0 {
		var cancel context.CancelFunc
		invokeCtx, cancel = context.WithTimeout(ctx, time.Duration(d.Config.Daemon.InvocationTimeoutSeconds)*time.Second)
		defer cancel()
	}

	result, err := d.invokeWithRetry(invokeCtx, model, prompt, d.RepoDir, d.InboxLogger.AssistantWriter(), "intake")
	if err != nil {
		return err
	}

	_ = d.InboxLogger.Log(map[string]any{
		"type":       "stage_complete",
		"stage":      "intake",
		"exit_code":  result.ExitCode,
		"output_len": len(result.Stdout),
	})

	// Only mark items as filed if the model succeeded.
	if result.ExitCode != 0 {
		output.PrintHuman("  Intake failed (exit %d). Items queued for retry.", result.ExitCode)
		return nil
	}

	// Parse OVERLAP markers from intake output and deliver as pending scope.
	if d.Config.Pipeline.Planning.Enabled {
		d.parseOverlapMarkers(result.Stdout)
	}

	// Mark processed items as filed under a lock. We re-read the inbox
	// because new items may have been added while the model was running.
	// We match by timestamp+text to find the items we processed.
	processedSet := make(map[string]bool)
	for _, item := range newItems {
		processedSet[item.Timestamp+"|"+item.Text] = true
	}
	if err := state.InboxMutate(inboxPath, func(f *state.InboxFile) error {
		for i, item := range f.Items {
			if item.Status == "new" && processedSet[item.Timestamp+"|"+item.Text] {
				f.Items[i].Status = "filed"
			}
		}
		return nil
	}); err != nil {
		return fmt.Errorf("saving inbox after intake: %w", err)
	}

	output.PrintHuman("  Intake: %d items filed", len(newItems))

	// Signal the execute loop that new work may be available.
	// Non-blocking: if the channel already has a signal, the loop
	// will find the work on its next navigation pass.
	select {
	case d.workAvailable <- struct{}{}:
	default:
	}

	return nil
}

// parseOverlapMarkers scans intake output for OVERLAP markers and delivers
// the overlapping scope as pending scope to the target orchestrator.
// Format: OVERLAP: "item summary" overlaps with Project Name (address)
func (d *Daemon) parseOverlapMarkers(output string) {
	for _, line := range strings.Split(output, "\n") {
		line = strings.TrimSpace(line)
		if !strings.HasPrefix(line, "OVERLAP:") {
			continue
		}

		// Extract the quoted scope text
		quoteStart := strings.Index(line, "\"")
		if quoteStart < 0 {
			continue
		}
		quoteEnd := strings.Index(line[quoteStart+1:], "\"")
		if quoteEnd < 0 {
			continue
		}
		scopeText := line[quoteStart+1 : quoteStart+1+quoteEnd]

		// Extract the address in parentheses at the end
		parenStart := strings.LastIndex(line, "(")
		parenEnd := strings.LastIndex(line, ")")
		if parenStart < 0 || parenEnd <= parenStart {
			continue
		}
		targetAddr := line[parenStart+1 : parenEnd]

		// Deliver as pending scope
		if err := d.Store.MutateNode(targetAddr, func(ns *state.NodeState) error {
			ns.PendingScope = append(ns.PendingScope, scopeText)
			return nil
		}); err != nil {
			_ = d.InboxLogger.Log(map[string]any{
				"type":   "overlap_delivery_failed",
				"target": targetAddr,
				"error":  err.Error(),
			})
			continue
		}

		_ = d.InboxLogger.Log(map[string]any{
			"type":   "overlap_delivered",
			"target": targetAddr,
			"scope":  scopeText,
		})
	}
}

// resolveContextHeader loads a context header prompt from the three-tier
// template system, falling back to a hardcoded default.
func resolveContextHeader(wolfcastleDir, promptFile, fallback string) string {
	if wolfcastleDir != "" {
		content, err := pipeline.ResolvePromptTemplate(wolfcastleDir, promptFile, nil)
		if err == nil {
			return strings.TrimRight(content, "\n")
		}
	}
	return strings.TrimRight(fallback, "\n")
}
