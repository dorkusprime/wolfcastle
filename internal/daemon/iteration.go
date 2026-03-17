package daemon

import (
	"context"
	"encoding/json"
	"fmt"
	"os/exec"
	"path/filepath"
	"strings"
	"time"

	werrors "github.com/dorkusprime/wolfcastle/internal/errors"
	"github.com/dorkusprime/wolfcastle/internal/logging"
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

			// Check if the model created new tasks during this iteration.
			// If so, block the parent so navigation moves to the subtasks
			// instead of re-picking this yielded task.
			if updatedNS, readErr := d.Store.ReadNode(nav.NodeAddress); readErr == nil {
				newTasks := findNewTasks(ns, updatedNS)
				if len(newTasks) > 0 {
					reason := "decomposed into subtasks: " + strings.Join(newTasks, ", ")
					_ = d.Store.MutateNode(nav.NodeAddress, func(ns2 *state.NodeState) error {
						return state.TaskBlock(ns2, nav.TaskID, reason)
					})
					_ = d.Logger.Log(map[string]any{
						"type":      "yield_decomposition",
						"task":      nav.TaskID,
						"new_tasks": newTasks,
					})
				}
			}
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
		if marker == "WOLFCASTLE_SKIP" {
			_ = d.Logger.Log(map[string]any{"type": "terminal_marker", "marker": "WOLFCASTLE_SKIP", "task": nav.TaskID})
			if err := d.Store.MutateNode(nav.NodeAddress, func(ns *state.NodeState) error {
				return state.TaskComplete(ns, nav.TaskID)
			}); err != nil {
				_ = d.Logger.Log(map[string]any{"type": "complete_error", "task": nav.TaskID, "error": err.Error()})
			}
			d.autoCompleteDecomposedParents(nav.NodeAddress)
			return nil
		}
		if marker == "WOLFCASTLE_COMPLETE" {
			// Re-read state from disk since the model may have added
			// deliverables via CLI during execution.
			if updated, readErr := d.Store.ReadNode(nav.NodeAddress); readErr == nil {
				ns = updated
			}

			// Verify deliverables exist. Missing deliverables are a warning,
			// not a completion failure. Git progress is the hard gate.
			missing := checkDeliverables(d.RepoDir, ns, nav.TaskID)
			if len(missing) > 0 {
				_ = d.Logger.Log(map[string]any{
					"type":    "deliverable_warning",
					"task":    nav.TaskID,
					"missing": missing,
				})
				output.PrintHuman("  Warning: declared deliverables missing: %v", missing)
			}
		}
		if marker == "WOLFCASTLE_COMPLETE" {
			// Audit tasks skip the git progress check: their output is
			// state mutations in .wolfcastle/system/, not code changes.
			isAudit := false
			for _, t := range ns.Tasks {
				if t.ID == nav.TaskID {
					isAudit = t.IsAudit
					break
				}
			}
			if !isAudit && !checkGitProgress(d.RepoDir, beforeHEAD) {
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
			d.autoCompleteDecomposedParents(nav.NodeAddress)
			return nil
		}

		// No terminal marker — auto-commit partial work, then increment failure count
		autoCommitPartialWork(d.RepoDir, d.Logger, nav.TaskID)

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
	markers := []string{"WOLFCASTLE_COMPLETE", "WOLFCASTLE_SKIP", "WOLFCASTLE_BLOCKED", "WOLFCASTLE_YIELD"}

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
				sub = strings.Trim(sub, "*_`")
				sub = strings.TrimSpace(sub)
				if sub == m {
					found[m] = true
				}
				// SKIP matches as a prefix: "WOLFCASTLE_SKIP reason text"
				if m == "WOLFCASTLE_SKIP" && strings.HasPrefix(sub, m+" ") {
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

// autoCommitPartialWork commits any uncommitted changes in the repo when a
// task fails without a terminal marker. This preserves partial work that the
// model did before failing, preventing it from being lost on the next iteration.
func autoCommitPartialWork(repoDir string, logger *logging.Logger, taskID string) {
	// Check for uncommitted changes
	statusCmd := exec.Command("git", "status", "--porcelain")
	statusCmd.Dir = repoDir
	out, err := statusCmd.Output()
	if err != nil || len(strings.TrimSpace(string(out))) == 0 {
		return // no changes or git unavailable
	}

	// Stage and commit
	addCmd := exec.Command("git", "add", "-A")
	addCmd.Dir = repoDir
	if err := addCmd.Run(); err != nil {
		_ = logger.Log(map[string]any{"type": "auto_commit_error", "task": taskID, "error": err.Error()})
		return
	}

	msg := fmt.Sprintf("wolfcastle: auto-commit partial work [%s]", taskID)
	commitCmd := exec.Command("git", "commit", "-m", msg, "--no-verify")
	commitCmd.Dir = repoDir
	if err := commitCmd.Run(); err != nil {
		_ = logger.Log(map[string]any{"type": "auto_commit_error", "task": taskID, "error": err.Error()})
		return
	}

	_ = logger.Log(map[string]any{"type": "auto_commit", "task": taskID})
}

// autoCompleteDecomposedParents checks if any blocked task in the node was
// decomposed into subtasks and all those subtasks are now complete. If so,
// the parent is auto-completed.
func (d *Daemon) autoCompleteDecomposedParents(nodeAddr string) {
	ns, err := d.Store.ReadNode(nodeAddr)
	if err != nil {
		return
	}
	const prefix = "decomposed into subtasks: "
	for _, t := range ns.Tasks {
		if t.State != state.StatusBlocked || !strings.HasPrefix(t.BlockedReason, prefix) {
			continue
		}
		parts := strings.TrimPrefix(t.BlockedReason, prefix)
		subtaskIDs := strings.Split(parts, ", ")
		allComplete := true
		for _, subID := range subtaskIDs {
			subID = strings.TrimSpace(subID)
			found := false
			for _, sub := range ns.Tasks {
				if sub.ID == subID {
					found = true
					if sub.State != state.StatusComplete {
						allComplete = false
					}
					break
				}
			}
			if !found {
				allComplete = false
			}
			if !allComplete {
				break
			}
		}
		if allComplete {
			taskID := t.ID
			_ = d.Store.MutateNode(nodeAddr, func(ns2 *state.NodeState) error {
				return state.TaskComplete(ns2, taskID)
			})
			_ = d.Logger.Log(map[string]any{
				"type": "auto_complete_parent",
				"task": taskID,
			})
		}
	}
}

// findNewTasks returns the IDs of tasks present in after but not in before,
// excluding audit tasks. Used to detect subtasks created during a YIELD.
func findNewTasks(before, after *state.NodeState) []string {
	beforeIDs := make(map[string]bool)
	for _, t := range before.Tasks {
		beforeIDs[t.ID] = true
	}
	var newIDs []string
	for _, t := range after.Tasks {
		if !beforeIDs[t.ID] && !t.IsAudit {
			newIDs = append(newIDs, t.ID)
		}
	}
	return newIDs
}
