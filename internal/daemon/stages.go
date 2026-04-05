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
	inboxPath := d.Store.InboxPath()

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
	inboxPath := d.Store.InboxPath()

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

	if stage, ok := d.Config.Pipeline.Stages["intake"]; ok && stage.IsEnabled() {
		_ = d.InboxLogger.StartIterationWithPrefix("intake")
		_ = d.InboxLogger.LogIterationStart("intake", "")
		if err := d.runIntakeStage(ctx, stage); err != nil {
			output.PrintHuman("  Intake stage error (non-fatal): %v", err)
		}
		d.InboxLogger.Close()
	}
}

// checkInboxForNew returns whether the inbox has items with status "new".
func (d *Daemon) checkInboxForNew(inboxPath string) bool {
	inboxData, err := state.LoadInbox(inboxPath)
	if err != nil {
		return false
	}
	for _, item := range inboxData.Items {
		if item.Status == state.InboxNew {
			return true
		}
	}
	return false
}

// runIntakeStage processes inbox items one at a time by invoking the model
// separately for each item. Between invocations the root index is re-read
// so the model sees projects created by earlier items in the same batch.
// This prevents duplicate root projects when multiple inbox items relate
// to the same feature. Items that are successfully processed (i.e., the
// model completes without error) are marked as "filed". Items that fail
// remain "new" for retry.
func (d *Daemon) runIntakeStage(ctx context.Context, stage config.PipelineStage) error {
	inboxPath := d.Store.InboxPath()
	inboxData, err := state.LoadInbox(inboxPath)
	if err != nil {
		return nil // No inbox file = nothing to process
	}

	// Filter to only new-status items
	var newItems []state.InboxItem
	for _, item := range inboxData.Items {
		if item.Status == state.InboxNew {
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
		intakeStage.PromptFile = "stages/intake-planning.md"
	}

	model, ok := d.Config.Models[intakeStage.Model]
	if !ok {
		return werrors.Config(fmt.Errorf("model %q not found for intake stage. Add it to the models section of your config", intakeStage.Model))
	}

	_ = d.InboxLogger.Log(map[string]any{"type": "stage_start", "stage": "intake", "new_items": len(newItems)})

	filedCount := 0
	for itemIdx, item := range newItems {
		// Re-read the root index before each item so the model sees
		// projects created by previous invocations in this batch.
		var itemsCtx strings.Builder
		idx, err := d.Store.ReadIndex()
		if err == nil && len(idx.Root) > 0 {
			itemsCtx.WriteString("# Existing Root Projects\n\n")
			itemsCtx.WriteString("Before creating a new root project, check this list. If an inbox item's work belongs under an existing project, add it there with --node instead of creating a duplicate.\n\n")
			for _, rootAddr := range idx.Root {
				entry, ok := idx.Nodes[rootAddr]
				if !ok {
					continue
				}
				fmt.Fprintf(&itemsCtx, "- **%s** (address: `%s`, %s, %s)", entry.Name, rootAddr, entry.Type, entry.State)
				if ns, loadErr := d.Store.ReadNode(rootAddr); loadErr == nil && ns.Scope != "" {
					scope := ns.Scope
					if len(scope) > 120 {
						scope = scope[:120] + "..."
					}
					fmt.Fprintf(&itemsCtx, "\n  Scope: %s", scope)
				}
				itemsCtx.WriteString("\n")
			}
			itemsCtx.WriteString("\n")
		}

		intakeHeader := resolveContextHeader(d.WolfcastleDir, "intake-context.md", "# Inbox Items to Process\n")
		itemsCtx.WriteString(intakeHeader + "\n")
		fmt.Fprintf(&itemsCtx, "### Item %d\n", itemIdx+1)
		fmt.Fprintf(&itemsCtx, "- **Timestamp:** %s\n", item.Timestamp)
		fmt.Fprintf(&itemsCtx, "- **Text:** %s\n\n", item.Text)

		prompt, err := pipeline.AssemblePrompt(d.WolfcastleDir, d.Config, intakeStage, itemsCtx.String())
		if err != nil {
			return err
		}

		invokeCtx := ctx
		var invokeCancel context.CancelFunc
		if d.Config.Daemon.InvocationTimeoutSeconds > 0 {
			invokeCtx, invokeCancel = context.WithTimeout(ctx, time.Duration(d.Config.Daemon.InvocationTimeoutSeconds)*time.Second)
		}

		itemStart := time.Now()
		result, err := d.invokeWithRetry(invokeCtx, model, prompt, d.RepoDir, d.InboxLogger.AssistantWriter(), "intake")
		if invokeCancel != nil {
			invokeCancel()
		}
		if err != nil {
			return err
		}

		_ = d.InboxLogger.Log(map[string]any{
			"type":        "stage_complete",
			"stage":       "intake",
			"item_index":  itemIdx,
			"exit_code":   result.ExitCode,
			"output_len":  len(result.Stdout),
			"duration_ms": time.Since(itemStart).Milliseconds(),
		})

		if result.Summary != "" {
			output.PrintHuman("  intake-%04d: %s", itemIdx+1, result.Summary)
			_ = d.InboxLogger.Log(map[string]any{
				"type":    "intake_summary",
				"item":    itemIdx + 1,
				"summary": result.Summary,
			})
		}

		if result.ExitCode != 0 {
			output.PrintHuman("  Intake item %d failed (exit %d). Queued for retry.", itemIdx+1, result.ExitCode)
			continue
		}

		// Parse OVERLAP markers from intake output and deliver as pending scope.
		if d.Config.Pipeline.Planning.Enabled {
			d.parseOverlapMarkers(result.Stdout)
		}

		// Mark this single item as filed. We re-read the inbox because
		// new items may have been added while the model was running.
		itemKey := item.Timestamp + "|" + item.Text
		if err := state.InboxMutate(inboxPath, func(f *state.InboxFile) error {
			for i, fi := range f.Items {
				if fi.Status == state.InboxNew && fi.Timestamp+"|"+fi.Text == itemKey {
					f.Items[i].Status = state.InboxFiled
					break
				}
			}
			return nil
		}); err != nil {
			return fmt.Errorf("saving inbox after intake: %w", err)
		}

		filedCount++
	}

	output.PrintHuman("  Intake: %d items filed", filedCount)

	// Signal the execute loop that new work may be available.
	// Non-blocking: if the channel already has a signal, the loop
	// will find the work on its next navigation pass.
	if filedCount > 0 {
		select {
		case d.workAvailable <- struct{}{}:
		default:
		}
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
