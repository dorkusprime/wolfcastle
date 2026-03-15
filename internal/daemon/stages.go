package daemon

import (
	"context"
	"fmt"
	"path/filepath"
	"strings"
	"time"

	"github.com/dorkusprime/wolfcastle/internal/config"
	werrors "github.com/dorkusprime/wolfcastle/internal/errors"
	"github.com/dorkusprime/wolfcastle/internal/output"
	"github.com/dorkusprime/wolfcastle/internal/pipeline"
	"github.com/dorkusprime/wolfcastle/internal/state"
)

// runInboxLoop is the parallel goroutine that watches for new inbox items
// and runs the intake stage. It polls inbox.json at the configured interval
// and processes any items with status "new".
func (d *Daemon) runInboxLoop(ctx context.Context) {
	pollInterval := time.Duration(d.Config.Daemon.InboxPollIntervalSeconds) * time.Second
	if pollInterval <= 0 {
		pollInterval = time.Duration(d.Config.Daemon.BlockedPollIntervalSeconds) * time.Second
	}
	if pollInterval <= 0 {
		pollInterval = 5 * time.Second
	}

	inboxCounter := 0

	for {
		select {
		case <-ctx.Done():
			return
		default:
		}

		inboxPath := filepath.Join(d.Resolver.ProjectsDir(), "inbox.json")
		hasNew := d.checkInboxForNew(inboxPath)

		if hasNew {
			inboxCounter++
			output.PrintHuman("inbox-%04d: Processing inbox items...", inboxCounter)

			// Find the intake stage in the pipeline
			for _, stage := range d.Config.Pipeline.Stages {
				if stage.Name == "intake" && stage.IsEnabled() {
					_ = d.Logger.StartIterationWithPrefix("intake")
					if err := d.runIntakeStage(ctx, stage); err != nil {
						output.PrintHuman("  Intake stage error (non-fatal): %v", err)
					}
					d.Logger.Close()
					break
				}
			}
		}

		if !sleepWithContext(ctx, pollInterval) {
			return
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

	_ = d.Logger.Log(map[string]any{"type": "stage_start", "stage": "intake", "new_items": len(newItems)})

	invokeCtx := ctx
	if d.Config.Daemon.InvocationTimeoutSeconds > 0 {
		var cancel context.CancelFunc
		invokeCtx, cancel = context.WithTimeout(ctx, time.Duration(d.Config.Daemon.InvocationTimeoutSeconds)*time.Second)
		defer cancel()
	}

	result, err := d.invokeWithRetry(invokeCtx, model, prompt, d.RepoDir, d.Logger.AssistantWriter(), "intake")
	if err != nil {
		return err
	}

	_ = d.Logger.Log(map[string]any{
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

	for _, idx := range newIndices {
		inboxData.Items[idx].Status = "filed"
	}

	if err := state.SaveInbox(inboxPath, inboxData); err != nil {
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
