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
	projDir := d.Resolver.ProjectsDir()

	// Try fsnotify first. Watch the projects directory because
	// inbox.json might not exist yet when the daemon starts.
	watcher, err := fsnotify.NewWatcher()
	if err == nil {
		defer func() { _ = watcher.Close() }()
		if addErr := watcher.Add(projDir); addErr == nil {
			output.PrintHuman("Inbox watcher: using fsnotify on %s", projDir)
			d.runInboxWithFsnotify(ctx, watcher)
			return
		}
		_ = watcher.Close()
	}

	// Fallback: polling
	output.PrintHuman("Inbox watcher: fsnotify unavailable, using polling")
	d.runInboxWithPolling(ctx)
}

// runInboxWithFsnotify watches for inbox.json changes via fsnotify and
// runs the intake stage when new items appear.
func (d *Daemon) runInboxWithFsnotify(ctx context.Context, watcher *fsnotify.Watcher) {
	inboxCounter := 0
	inboxPath := filepath.Join(d.Resolver.ProjectsDir(), "inbox.json")

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
	inboxPath := filepath.Join(d.Resolver.ProjectsDir(), "inbox.json")

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
	output.PrintHuman("inbox-%04d: Processing inbox items...", counter)

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
	inboxPath := filepath.Join(d.Resolver.ProjectsDir(), "inbox.json")
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

	model, ok := d.Config.Models[stage.Model]
	if !ok {
		return werrors.Config(fmt.Errorf("model %q not found", stage.Model))
	}

	// Build context with inbox items
	var itemsCtx strings.Builder
	intakeHeader := resolveContextHeader(d.WolfcastleDir, "intake-context.md", "# Inbox Items to Process\n")
	itemsCtx.WriteString(intakeHeader + "\n")
	for i, item := range newItems {
		fmt.Fprintf(&itemsCtx, "### Item %d\n", i+1)
		fmt.Fprintf(&itemsCtx, "- **Timestamp:** %s\n", item.Timestamp)
		fmt.Fprintf(&itemsCtx, "- **Text:** %s\n\n", item.Text)
	}

	prompt, err := pipeline.AssemblePrompt(d.WolfcastleDir, d.Config, stage, itemsCtx.String())
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
		output.PrintHuman("  Intake stage failed (exit %d). Items remain new for retry.", result.ExitCode)
		return nil
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

	output.PrintHuman("  Intake stage: %d items filed", len(newItems))

	// Signal the execute loop that new work may be available.
	// Non-blocking: if the channel already has a signal, the loop
	// will find the work on its next navigation pass.
	select {
	case d.workAvailable <- struct{}{}:
	default:
	}

	return nil
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
