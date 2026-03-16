package daemon

import (
	"context"
	"encoding/json"
	"fmt"
	"path/filepath"
	"strings"
	"time"

	werrors "github.com/dorkusprime/wolfcastle/internal/errors"
	"github.com/dorkusprime/wolfcastle/internal/output"
	"github.com/dorkusprime/wolfcastle/internal/pipeline"
	"github.com/dorkusprime/wolfcastle/internal/state"
	"github.com/dorkusprime/wolfcastle/internal/tree"
)

// runIteration executes a single daemon iteration: claims the task, runs each
// enabled pipeline stage in order, reloads state from disk (to pick up CLI
// mutations), handles terminal markers, and manages failure escalation.
func (d *Daemon) runIteration(ctx context.Context, nav *state.NavigationResult, idx *state.RootIndex) error {
	// Claim the task
	addr, err := tree.ParseAddress(nav.NodeAddress)
	if err != nil {
		return werrors.Navigation(fmt.Errorf("parsing node address %q: %w", nav.NodeAddress, err))
	}
	ns, err := d.Store.ReadNode(nav.NodeAddress)
	if err != nil {
		return werrors.State(fmt.Errorf("loading node state for %s: %w", nav.NodeAddress, err))
	}

	// Skip the claim if the task is already in_progress (resumption
	// after YIELD or crash recovery).
	alreadyInProgress := false
	for _, t := range ns.Tasks {
		if t.ID == nav.TaskID && t.State == state.StatusInProgress {
			alreadyInProgress = true
			break
		}
	}
	if !alreadyInProgress {
		if err := d.Store.MutateNode(nav.NodeAddress, func(ns *state.NodeState) error {
			return state.TaskClaim(ns, nav.TaskID)
		}); err != nil {
			return werrors.State(fmt.Errorf("claiming task %s: %w", nav.TaskID, err))
		}
		// Re-read after mutation for the rest of the iteration.
		ns, err = d.Store.ReadNode(nav.NodeAddress)
		if err != nil {
			return werrors.State(fmt.Errorf("reloading node state after claim: %w", err))
		}
	}

	// Run pipeline stages. Intake runs in a parallel goroutine
	// (ADR-064), so the iteration loop only handles execute and
	// custom stages.
	for _, stage := range d.Config.Pipeline.Stages {
		if !stage.IsEnabled() {
			continue
		}

		// Skip intake stage here; it runs in the parallel inbox goroutine.
		if stage.Name == "intake" {
			continue
		}

		// Execute stage (and any other custom stages)
		nodeDir := filepath.Join(d.Resolver.ProjectsDir(), filepath.Join(addr.Parts...))
		iterCtx := pipeline.BuildIterationContextFull(d.WolfcastleDir, nodeDir, nav.NodeAddress, ns, nav.TaskID, d.Config)

		prompt, err := pipeline.AssemblePrompt(d.WolfcastleDir, d.Config, stage, iterCtx)
		if err != nil {
			return werrors.Config(fmt.Errorf("assembling prompt for stage %s: %w", stage.Name, err))
		}

		model, ok := d.Config.Models[stage.Model]
		if !ok {
			return werrors.Config(fmt.Errorf("model %q not found for stage %s", stage.Model, stage.Name))
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

		// Record HEAD before invocation so we can detect new commits.
		beforeHEAD := gitHEAD(d.RepoDir)

		result, err := d.invokeWithRetry(invokeCtx, model, prompt, d.RepoDir, d.Logger.AssistantWriter(), stage.Name)
		if cancel != nil {
			cancel()
		}

		// Restore terminal to cooked mode in case the child left it in raw mode.
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

		// Reload state from disk — CLI commands invoked by the model may
		// have mutated state.json during execution (breadcrumbs, gaps, scope, etc.)
		ns, err = d.Store.ReadNode(nav.NodeAddress)
		if err != nil {
			_ = d.Logger.Log(map[string]any{"type": "reload_error", "error": err.Error()})
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
			if err := d.Store.MutateNode(nav.NodeAddress, func(ns *state.NodeState) error {
				return state.TaskBlock(ns, nav.TaskID, "blocked by model")
			}); err != nil {
				_ = d.Logger.Log(map[string]any{"type": "save_error", "error": err.Error()})
			}
			return nil
		}
		if marker == "WOLFCASTLE_COMPLETE" {
			// Re-read state from disk since the model may have added
			// deliverables via CLI during execution.
			if updated, readErr := d.Store.ReadNode(nav.NodeAddress); readErr == nil {
				ns = updated
			}

			// Verify deliverables exist before accepting completion.
			missing := checkDeliverables(d.RepoDir, ns, nav.TaskID)
			if len(missing) > 0 {
				_ = d.Logger.Log(map[string]any{
					"type":    "deliverable_missing",
					"task":    nav.TaskID,
					"missing": missing,
				})
				output.PrintHuman("  Deliverables missing: %v. Failing task.", missing)
				marker = "" // clear so we fall through to the failure path
			}
		}
		if marker == "WOLFCASTLE_COMPLETE" {
			// Verify the model made real changes via git diff.
			// Deliverables check existence (structural); git diff
			// checks progress (activity). Two different questions.
			if !checkGitProgress(d.RepoDir, beforeHEAD) {
				_ = d.Logger.Log(map[string]any{
					"type": "no_progress",
					"task": nav.TaskID,
				})
				output.PrintHuman("  No changes detected. Failing task.")
				marker = ""
			}
		}
		if marker == "WOLFCASTLE_COMPLETE" {
			_ = d.Logger.Log(map[string]any{"type": "terminal_marker", "marker": "WOLFCASTLE_COMPLETE"})
			if err := d.Store.MutateNode(nav.NodeAddress, func(ns *state.NodeState) error {
				return state.TaskComplete(ns, nav.TaskID)
			}); err != nil {
				_ = d.Logger.Log(map[string]any{"type": "complete_error", "task": nav.TaskID, "error": err.Error()})
			}
			return nil
		}

		// No terminal marker — increment failure count
		_ = d.Logger.Log(map[string]any{
			"type":  "no_terminal_marker",
			"empty": result.Stdout == "",
			"task":  nav.TaskID,
		})

		var failCount int
		mutErr := d.Store.MutateNode(nav.NodeAddress, func(ns *state.NodeState) error {
			var err error
			failCount, err = state.IncrementFailure(ns, nav.TaskID)
			if err != nil {
				return err
			}

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

			return nil
		})
		if mutErr != nil {
			_ = d.Logger.Log(map[string]any{"type": "failure_increment_error", "error": mutErr.Error()})
		} else {
			_ = d.Logger.Log(map[string]any{"type": "failure_increment", "task": nav.TaskID, "count": failCount})
		}
	}

	return nil
}

// scanTerminalMarker scans model output line-by-line for terminal markers.
// It handles two formats:
//  1. Raw text: marker appears as a standalone line or at the end of a line
//  2. JSON stream (Claude Code --output-format stream-json): marker appears
//     inside the "text" field of a {"type":"assistant","text":"..."} envelope
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
