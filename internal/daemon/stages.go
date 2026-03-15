package daemon

import (
	"context"
	"fmt"
	"path/filepath"
	"strings"
	"time"

	"github.com/dorkusprime/wolfcastle/internal/config"
	"github.com/dorkusprime/wolfcastle/internal/output"
	"github.com/dorkusprime/wolfcastle/internal/pipeline"
	"github.com/dorkusprime/wolfcastle/internal/state"
)

// processInboxIfNeeded runs the expand and file stages if there are
// inbox items to process. This runs before navigation so that inbox
// items can create projects and tasks that navigation then finds.
func (d *Daemon) processInboxIfNeeded(ctx context.Context) {
	inboxPath := filepath.Join(d.Resolver.ProjectsDir(), "inbox.json")
	hasNew, hasExpanded := d.checkInboxState(inboxPath)

	if !hasNew && !hasExpanded {
		return
	}

	// Start a log iteration so expand/file stages can write logs
	_ = d.Logger.StartIteration()
	defer d.Logger.Close()

	output.PrintHuman("Processing inbox items...")

	// Find the expand and file stages in the pipeline
	for _, stage := range d.Config.Pipeline.Stages {
		if !stage.IsEnabled() {
			continue
		}
		switch stage.Name {
		case "expand":
			if !hasNew {
				continue
			}
			if err := d.runExpandStage(ctx, stage); err != nil {
				output.PrintHuman("  Expand stage error (non-fatal): %v", err)
			}
			// Re-check after expand
			hasNew, hasExpanded = d.checkInboxState(inboxPath)
		case "file":
			if !hasExpanded {
				continue
			}
			if err := d.runFileStage(ctx, stage); err != nil {
				output.PrintHuman("  File stage error (non-fatal): %v", err)
			}
		}
	}
}

func (d *Daemon) runExpandStage(ctx context.Context, stage config.PipelineStage) error {
	inboxPath := filepath.Join(d.Resolver.ProjectsDir(), "inbox.json")
	inboxData, err := state.LoadInbox(inboxPath)
	if err != nil {
		return nil // No inbox file = nothing to expand
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
		return fmt.Errorf("model %q not found", stage.Model)
	}

	// Build context with only new items
	var itemsCtx strings.Builder
	expandHeader := resolveContextHeader(d.WolfcastleDir, "expand-context.md", "# Inbox Items to Expand\n")
	itemsCtx.WriteString(expandHeader + "\n")
	for i, item := range newItems {
		fmt.Fprintf(&itemsCtx, "### Item %d\n", i+1)
		fmt.Fprintf(&itemsCtx, "- **Timestamp:** %s\n", item.Timestamp)
		fmt.Fprintf(&itemsCtx, "- **Text:** %s\n\n", item.Text)
	}

	prompt, err := pipeline.AssemblePrompt(d.WolfcastleDir, d.Config, stage, itemsCtx.String())
	if err != nil {
		return err
	}

	_ = d.Logger.Log(map[string]any{"type": "stage_start", "stage": "expand", "new_items": len(newItems)})

	invokeCtx := ctx
	if d.Config.Daemon.InvocationTimeoutSeconds > 0 {
		var cancel context.CancelFunc
		invokeCtx, cancel = context.WithTimeout(ctx, time.Duration(d.Config.Daemon.InvocationTimeoutSeconds)*time.Second)
		defer cancel()
	}

	result, err := d.invokeWithRetry(invokeCtx, model, prompt, d.RepoDir, d.Logger.AssistantWriter(), "expand")
	if err != nil {
		return err
	}

	_ = d.Logger.Log(map[string]any{
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

	if err := state.SaveInbox(inboxPath, inboxData); err != nil {
		return fmt.Errorf("saving inbox after expand: %w", err)
	}

	output.PrintHuman("  Expand stage: %d items expanded", len(newItems))
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
	inboxData, err := state.LoadInbox(inboxPath)
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
	fileHeader := resolveContextHeader(d.WolfcastleDir, "file-context.md", "# Expanded Inbox Items to File\n")
	itemsCtx.WriteString(fileHeader + "\n")
	for _, idx := range expandedIndices {
		item := inboxData.Items[idx]
		fmt.Fprintf(&itemsCtx, "---\n\n**Original:** %s\n\n", item.Text)
		if item.Expanded != "" {
			itemsCtx.WriteString(item.Expanded)
			itemsCtx.WriteString("\n\n")
		}
	}

	prompt, err := pipeline.AssemblePrompt(d.WolfcastleDir, d.Config, stage, itemsCtx.String())
	if err != nil {
		return err
	}

	_ = d.Logger.Log(map[string]any{"type": "stage_start", "stage": "file", "expanded_items": len(expandedIndices)})

	invokeCtx := ctx
	if d.Config.Daemon.InvocationTimeoutSeconds > 0 {
		var cancel context.CancelFunc
		invokeCtx, cancel = context.WithTimeout(ctx, time.Duration(d.Config.Daemon.InvocationTimeoutSeconds)*time.Second)
		defer cancel()
	}

	// The model executes wolfcastle commands directly via tool calls
	result, err := d.invokeWithRetry(invokeCtx, model, prompt, d.RepoDir, d.Logger.AssistantWriter(), "file")
	if err != nil {
		return err
	}

	_ = d.Logger.Log(map[string]any{
		"type":       "stage_complete",
		"stage":      "file",
		"exit_code":  result.ExitCode,
		"output_len": len(result.Stdout),
	})

	// Mark expanded items as filed.
	for _, idx := range expandedIndices {
		inboxData.Items[idx].Status = "filed"
	}
	output.PrintHuman("  File stage: %d items filed", len(expandedIndices))

	if err := state.SaveInbox(inboxPath, inboxData); err != nil {
		return fmt.Errorf("saving inbox after file stage: %w", err)
	}

	output.PrintHuman("  File stage: %d items filed", len(expandedIndices))
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
