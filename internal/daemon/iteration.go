package daemon

import (
	"context"
	"encoding/json"
	"fmt"
	"path/filepath"
	"strings"
	"time"

	"github.com/dorkusprime/wolfcastle/internal/output"
	"github.com/dorkusprime/wolfcastle/internal/pipeline"
	"github.com/dorkusprime/wolfcastle/internal/state"
	"github.com/dorkusprime/wolfcastle/internal/tree"
)

// runIteration executes a single daemon iteration: claims the task, runs each
// enabled pipeline stage in order, applies model output markers, persists
// state mutations, and handles failure escalation (decomposition, auto-block).
func (d *Daemon) runIteration(ctx context.Context, nav *state.NavigationResult, idx *state.RootIndex) error {
	// Check inbox state before claiming. If expanded items are pending
	// filing, the execute stage will be skipped, so we must not claim
	// the task (otherwise it's stuck in_progress with no work done).
	inboxPath := filepath.Join(d.Resolver.ProjectsDir(), "inbox.json")
	hasNewItems, hasExpandedItems := d.checkInboxState(inboxPath)

	// Claim the task
	addr, err := tree.ParseAddress(nav.NodeAddress)
	if err != nil {
		return fmt.Errorf("parsing node address %q: %w", nav.NodeAddress, err)
	}
	statePath := filepath.Join(d.Resolver.ProjectsDir(), filepath.Join(addr.Parts...), "state.json")
	ns, err := state.LoadNodeState(statePath)
	if err != nil {
		return fmt.Errorf("loading node state for %s: %w", nav.NodeAddress, err)
	}

	// Only claim when execute will actually run. If filing takes
	// priority, run expand/file stages without claiming. If the task
	// is already in_progress (resumption after YIELD or crash recovery),
	// skip the claim.
	alreadyInProgress := false
	for _, t := range ns.Tasks {
		if t.ID == nav.TaskID && t.State == state.StatusInProgress {
			alreadyInProgress = true
			break
		}
	}
	if !hasExpandedItems && !alreadyInProgress {
		if err := state.TaskClaim(ns, nav.TaskID); err != nil {
			return fmt.Errorf("claiming task %s: %w", nav.TaskID, err)
		}
		if err := state.SaveNodeState(statePath, ns); err != nil {
			return fmt.Errorf("saving node state after claim: %w", err)
		}
		if err := d.propagateState(nav.NodeAddress, ns.State, idx); err != nil {
			return fmt.Errorf("propagating state after claim: %w", err)
		}
	}

	// Run pipeline stages
	for _, stage := range d.Config.Pipeline.Stages {
		if !stage.IsEnabled() {
			continue
		}

		switch stage.Name {
		case "expand":
			if !hasNewItems {
				_ = d.Logger.Log(map[string]any{"type": "stage_skip", "stage": "expand", "reason": "no_new_inbox_items"})
				continue
			}
			if err := d.runExpandStage(ctx, stage); err != nil {
				_ = d.Logger.Log(map[string]any{"type": "stage_error", "stage": "expand", "error": err.Error()})
				// Non-fatal: expand failure doesn't block execution
				output.PrintHuman("  Expand stage error (non-fatal): %v", err)
			}
			// Re-check inbox state after expand — items may now be expanded
			hasNewItems, hasExpandedItems = d.checkInboxState(inboxPath)
			continue

		case "file":
			if !hasExpandedItems {
				_ = d.Logger.Log(map[string]any{"type": "stage_skip", "stage": "file", "reason": "no_expanded_inbox_items"})
				continue
			}
			if err := d.runFileStage(ctx, stage); err != nil {
				_ = d.Logger.Log(map[string]any{"type": "stage_error", "stage": "file", "error": err.Error()})
				output.PrintHuman("  File stage error (non-fatal): %v", err)
			}
			continue
		}

		// Skip execute stage if there are expanded items awaiting filing —
		// prioritize filing over execution to avoid working on a stale tree.
		if hasExpandedItems {
			_ = d.Logger.Log(map[string]any{"type": "stage_skip", "stage": stage.Name, "reason": "pending_filing"})
			output.PrintHuman("  Skipping %s stage: expanded items await filing", stage.Name)
			continue
		}

		// Execute stage (and any other custom stages)
		iterCtx := pipeline.BuildIterationContextWithDir(d.WolfcastleDir, nav.NodeAddress, ns, nav.TaskID, d.Config)

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

		_ = d.Logger.Log(map[string]any{
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
			_ = d.Logger.Log(map[string]any{"type": "stage_error", "stage": stage.Name, "error": err.Error()})
			return err
		}

		_ = d.Logger.Log(map[string]any{
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
			_ = d.Logger.Log(map[string]any{"type": "save_error", "error": err.Error()})
		}
		if err := d.propagateState(nav.NodeAddress, ns.State, idx); err != nil {
			_ = d.Logger.Log(map[string]any{"type": "propagate_error", "error": err.Error()})
		}

		// Check for terminal markers and transition task state.
		// Use line-by-line scanning to avoid false matches against
		// prompt instructions echoed in the model's JSON stream.
		marker := scanTerminalMarker(result.Stdout)
		if marker == "WOLFCASTLE_YIELD" {
			_ = d.Logger.Log(map[string]any{"type": "terminal_marker", "marker": "WOLFCASTLE_YIELD"})
			return nil
		}
		if marker == "WOLFCASTLE_BLOCKED" {
			_ = d.Logger.Log(map[string]any{"type": "terminal_marker", "marker": "WOLFCASTLE_BLOCKED", "task": nav.TaskID})
			if blockErr := state.TaskBlock(ns, nav.TaskID, "blocked by model"); blockErr == nil {
				_ = state.SaveNodeState(statePath, ns)
				_ = d.propagateState(nav.NodeAddress, ns.State, idx)
			}
			return nil
		}
		if marker == "WOLFCASTLE_COMPLETE" {
			_ = d.Logger.Log(map[string]any{"type": "terminal_marker", "marker": "WOLFCASTLE_COMPLETE"})
			if completeErr := state.TaskComplete(ns, nav.TaskID); completeErr != nil {
				_ = d.Logger.Log(map[string]any{"type": "complete_error", "task": nav.TaskID, "error": completeErr.Error()})
			} else {
				_ = state.SaveNodeState(statePath, ns)
				_ = d.propagateState(nav.NodeAddress, ns.State, idx)
			}
			return nil
		}

		// No terminal marker — increment failure count
		_ = d.Logger.Log(map[string]any{
			"type":  "no_terminal_marker",
			"empty": result.Stdout == "",
			"task":  nav.TaskID,
		})

		failCount, err := state.IncrementFailure(ns, nav.TaskID)
		if err != nil {
			_ = d.Logger.Log(map[string]any{"type": "failure_increment_error", "error": err.Error()})
		} else {
			_ = d.Logger.Log(map[string]any{"type": "failure_increment", "task": nav.TaskID, "count": failCount})

			if failCount >= d.Config.Failure.DecompositionThreshold && d.Config.Failure.DecompositionThreshold > 0 {
				if ns.DecompositionDepth < d.Config.Failure.MaxDecompositionDepth {
					_ = d.Logger.Log(map[string]any{"type": "decomposition_threshold", "task": nav.TaskID, "depth": ns.DecompositionDepth})
					state.SetNeedsDecomposition(ns, nav.TaskID, true)
				} else {
					_ = d.Logger.Log(map[string]any{"type": "auto_block", "task": nav.TaskID, "reason": "max_decomposition_depth"})
					if blockErr := state.TaskBlock(ns, nav.TaskID, "auto-blocked: decomposition threshold reached at max depth"); blockErr != nil {
						_ = d.Logger.Log(map[string]any{"type": "auto_block_error", "task": nav.TaskID, "error": blockErr.Error()})
					}
				}
			}

			if failCount >= d.Config.Failure.HardCap && d.Config.Failure.HardCap > 0 {
				_ = d.Logger.Log(map[string]any{"type": "auto_block", "task": nav.TaskID, "reason": "hard_cap", "count": failCount})
				if blockErr := state.TaskBlock(ns, nav.TaskID, fmt.Sprintf("auto-blocked: failure hard cap reached (%d)", failCount)); blockErr != nil {
					_ = d.Logger.Log(map[string]any{"type": "auto_block_error", "task": nav.TaskID, "error": blockErr.Error()})
				}
			}

			if err := state.SaveNodeState(statePath, ns); err != nil {
				_ = d.Logger.Log(map[string]any{"type": "save_error", "error": err.Error()})
			}
			if err := d.propagateState(nav.NodeAddress, ns.State, idx); err != nil {
				_ = d.Logger.Log(map[string]any{"type": "propagate_error", "error": err.Error()})
			}
		}
	}

	return nil
}

// scanTerminalMarker scans model output line-by-line for terminal markers.
// It handles two formats:
// 1. Raw text: marker appears as a standalone line or at the end of a line
// 2. JSON stream (Claude Code --output-format stream-json): marker appears
//    inside the "text" field of a {"type":"assistant","text":"..."} envelope
//
// Returns the marker name or empty string if none found.
func scanTerminalMarker(output string) string {
	// Scan all lines and collect all matched markers, then return
	// the highest-priority one. Priority: COMPLETE > BLOCKED > YIELD.
	// This prevents an early YIELD (from prompt echo or an intermediate
	// model message) from shadowing a later COMPLETE.
	found := map[string]bool{}
	markers := []string{"WOLFCASTLE_COMPLETE", "WOLFCASTLE_BLOCKED", "WOLFCASTLE_YIELD"}

	for _, line := range strings.Split(output, "\n") {
		trimmed := strings.TrimSpace(line)

		// Try to extract text from JSON stream envelope
		text := extractAssistantText(trimmed)
		if text == "" {
			text = trimmed
		}

		for _, m := range markers {
			for _, subline := range strings.Split(text, "\n") {
				sub := strings.TrimSpace(subline)
				if sub == m {
					found[m] = true
				}
				if strings.HasSuffix(sub, m) && (len(sub) == len(m) || sub[len(sub)-len(m)-1] == ' ') {
					found[m] = true
				}
			}
		}
	}

	// Return highest priority
	for _, m := range markers {
		if found[m] {
			return m
		}
	}
	return ""
}

// extractAssistantText extracts the text content from a Claude Code
// stream-json assistant message. Returns empty string if the line is
// not a valid assistant JSON envelope.
func extractAssistantText(line string) string {
	// Quick reject: must look like JSON
	if len(line) < 2 || line[0] != '{' {
		return ""
	}
	var envelope struct {
		Type    string `json:"type"`
		Text    string `json:"text"`
		Result  string `json:"result"`
		Message struct {
			Content []struct {
				Type string `json:"type"`
				Text string `json:"text"`
			} `json:"content"`
		} `json:"message"`
	}
	if err := json.Unmarshal([]byte(line), &envelope); err != nil {
		return ""
	}
	switch envelope.Type {
	case "assistant":
		// Simple format: {"type":"assistant","text":"..."}
		if envelope.Text != "" {
			return envelope.Text
		}
		// Claude Code format: {"type":"assistant","message":{"content":[{"type":"text","text":"..."}]}}
		for _, c := range envelope.Message.Content {
			if c.Type == "text" && c.Text != "" {
				return c.Text
			}
		}
	case "result":
		// Simple: {"type":"result","text":"..."}
		if envelope.Text != "" {
			return envelope.Text
		}
		// Claude Code: {"type":"result","result":"..."}
		if envelope.Result != "" {
			return envelope.Result
		}
	}
	return ""
}

