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
	"github.com/dorkusprime/wolfcastle/internal/invoke"
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
		nodeDir := filepath.Join(d.Store.Dir(), filepath.Join(addr.Parts...))
		iterCtx, err := d.ContextBuilder.Build(nav.NodeAddress, nodeDir, ns, nav.TaskID, d.Config)
		if err != nil {
			return werrors.Config(fmt.Errorf("building context for node %s task %s: %w", nav.NodeAddress, nav.TaskID, err))
		}

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

		// Record HEAD and task list before invocation so we can detect
		// new commits and new tasks (for YIELD+decomposition).
		beforeHEAD := gitHEAD(d.RepoDir)
		preInvocationNS := ns

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
		if marker == invoke.MarkerStringYield {
			_ = d.Logger.Log(map[string]any{"type": "terminal_marker", "marker": invoke.MarkerStringYield})

			// If the model created child tasks (hierarchical IDs like task-0001.0001),
			// navigation handles them automatically via depth-first ordering.
			// The parent task's status derives from its children.
			//
			// Legacy support: if the model created sibling tasks (flat IDs like
			// task-0002) and the planning pipeline is disabled, fall back to the
			// old block-parent behavior.
			if !d.Config.Pipeline.Planning.Enabled {
				if updatedNS, readErr := d.Store.ReadNode(nav.NodeAddress); readErr == nil {
					newTasks := findNewTasks(preInvocationNS, updatedNS)
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
			} else {
				// With planning enabled, just log the yield. Navigation
				// handles child task ordering via hierarchical IDs.
				if updatedNS, readErr := d.Store.ReadNode(nav.NodeAddress); readErr == nil {
					newTasks := findNewTasks(preInvocationNS, updatedNS)
					if len(newTasks) > 0 {
						_ = d.Logger.Log(map[string]any{
							"type":      "yield_decomposition",
							"task":      nav.TaskID,
							"new_tasks": newTasks,
						})
					}
				}
			}
			return nil
		}
		if marker == invoke.MarkerStringBlocked {
			_ = d.Logger.Log(map[string]any{"type": "terminal_marker", "marker": invoke.MarkerStringBlocked, "task": nav.TaskID})

			// Check if the model blocked a task that's actually
			// superseded. Superseded work should be SKIP, not BLOCKED.
			// Treat it as complete so it doesn't poison node state.
			if d.isSupersededBlock(nav.NodeAddress, nav.TaskID) {
				_ = d.Logger.Log(map[string]any{"type": "superseded_to_skip", "task": nav.TaskID})
				output.PrintHuman("  Superseded (treating as skip).")
				if err := d.Store.MutateNode(nav.NodeAddress, func(ns *state.NodeState) error {
					return state.TaskComplete(ns, nav.TaskID)
				}); err != nil {
					_ = d.Logger.Log(map[string]any{"type": "save_error", "error": err.Error()})
				}
				return nil
			}

			// For audit tasks with gaps, create remediation subtasks
			// instead of blocking. The subtasks fix each gap, and when
			// they all complete, DeriveParentStatus resets the audit to
			// not_started so it re-runs to verify the fixes.
			if created := d.createRemediationSubtasks(nav.NodeAddress, nav.TaskID); created > 0 {
				_ = d.Logger.Log(map[string]any{"type": "audit_remediation", "task": nav.TaskID, "subtasks": created})
				output.PrintHuman("  Audit: %d gap(s), remediating.", created)
				return nil
			}

			// Spec review blocked: feed issues back to the original spec
			// task so it can be revised. The review task stays blocked;
			// the spec task resets to not_started for another pass.
			if d.handleSpecReviewBlocked(nav.NodeAddress, nav.TaskID) {
				return nil
			}

			if err := d.Store.MutateNode(nav.NodeAddress, func(ns *state.NodeState) error {
				return state.TaskBlock(ns, nav.TaskID, "blocked by model")
			}); err != nil {
				_ = d.Logger.Log(map[string]any{"type": "save_error", "error": err.Error()})
			}
			// Propagate blocked state so parent orchestrators can detect
			// the block and trigger remediation planning.
			if err := d.propagateState(nav.NodeAddress, state.StatusBlocked, idx); err != nil {
				_ = d.Logger.Log(map[string]any{"type": "propagate_error", "error": err.Error()})
			}
			return nil
		}
		if marker == invoke.MarkerStringSkip {
			_ = d.Logger.Log(map[string]any{"type": "terminal_marker", "marker": invoke.MarkerStringSkip, "task": nav.TaskID})
			if err := d.Store.MutateNode(nav.NodeAddress, func(ns *state.NodeState) error {
				return state.TaskComplete(ns, nav.TaskID)
			}); err != nil {
				_ = d.Logger.Log(map[string]any{"type": "complete_error", "task": nav.TaskID, "error": err.Error()})
			}
			if !d.Config.Pipeline.Planning.Enabled {
				d.autoCompleteDecomposedParents(nav.NodeAddress)
			}
			// Propagate completion up through parent orchestrators so their
			// persisted state derives from children. MutateNode propagates
			// internally, but re-propagating here updates the in-memory idx
			// and guards against silent propagation failures in the store.
			if updatedNS, readErr := d.Store.ReadNode(nav.NodeAddress); readErr == nil {
				if err := d.propagateState(nav.NodeAddress, updatedNS.State, idx); err != nil {
					_ = d.Logger.Log(map[string]any{"type": "propagate_error", "error": err.Error()})
				}
			}
			return nil
		}
		if marker == invoke.MarkerStringComplete {
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
		if marker == invoke.MarkerStringComplete {
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
		if marker == invoke.MarkerStringComplete {
			_ = d.Logger.Log(map[string]any{"type": "terminal_marker", "marker": invoke.MarkerStringComplete})
			if err := d.Store.MutateNode(nav.NodeAddress, func(ns *state.NodeState) error {
				return state.TaskComplete(ns, nav.TaskID)
			}); err != nil {
				_ = d.Logger.Log(map[string]any{"type": "complete_error", "task": nav.TaskID, "error": err.Error()})
			}

			// Generate audit report when an audit task completes.
			d.maybeWriteAuditReport(nav.NodeAddress, nav.TaskID)

			if !d.Config.Pipeline.Planning.Enabled {
				d.autoCompleteDecomposedParents(nav.NodeAddress)
			}
			// Propagate completion up through parent orchestrators so their
			// persisted state derives from children. MutateNode propagates
			// internally, but re-propagating here updates the in-memory idx
			// and guards against silent propagation failures in the store.
			if updatedNS, readErr := d.Store.ReadNode(nav.NodeAddress); readErr == nil {
				if err := d.propagateState(nav.NodeAddress, updatedNS.State, idx); err != nil {
					_ = d.Logger.Log(map[string]any{"type": "propagate_error", "error": err.Error()})
				}
			}
			return nil
		}

		// Determine failure type for context injection on retry
		failureType := "no_terminal_marker"
		if scanTerminalMarker(result.Stdout) != "" {
			// A marker was found but cleared by deliverable or progress check
			failureType = "no_progress"
		}

		// Auto-commit partial work, then increment failure count
		autoCommitPartialWork(d.RepoDir, d.Logger, nav.TaskID, d.Config.Git.SkipHooksOnAutoCommit)

		_ = d.Logger.Log(map[string]any{
			"type":  failureType,
			"empty": result.Stdout == "",
			"task":  nav.TaskID,
		})

		var failCount int
		mutErr := d.Store.MutateNode(nav.NodeAddress, func(ns *state.NodeState) error {
			// Record the failure type for context injection on next retry
			for i := range ns.Tasks {
				if ns.Tasks[i].ID == nav.TaskID {
					ns.Tasks[i].LastFailureType = failureType
					break
				}
			}

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
	markers := []string{invoke.MarkerStringComplete, invoke.MarkerStringSkip, invoke.MarkerStringContinue, invoke.MarkerStringBlocked, invoke.MarkerStringYield}

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
				if m == invoke.MarkerStringSkip && strings.HasPrefix(sub, m+" ") {
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
func autoCommitPartialWork(repoDir string, logger *logging.Logger, taskID string, skipHooks bool) {
	// Check for uncommitted changes
	statusCmd := exec.Command("git", "status", "--porcelain")
	statusCmd.Dir = repoDir
	out, err := statusCmd.Output()
	if err != nil || len(strings.TrimSpace(string(out))) == 0 {
		return // no changes or git unavailable
	}

	// Stage tracked files only (git add -u). This avoids staging
	// untracked files like .env or credentials that happen to be
	// sitting in the working tree.
	addCmd := exec.Command("git", "add", "-u")
	addCmd.Dir = repoDir
	if err := addCmd.Run(); err != nil {
		_ = logger.Log(map[string]any{"type": "auto_commit_error", "task": taskID, "error": err.Error()})
		return
	}

	msg := fmt.Sprintf("wolfcastle: auto-commit partial work [%s]", taskID)
	commitArgs := []string{"commit", "-m", msg}
	if skipHooks {
		commitArgs = append(commitArgs, "--no-verify")
	}
	commitCmd := exec.Command("git", commitArgs...)
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
				// Unblock the parent first, then complete it.
				// TaskComplete treats blocked as terminal and won't transition.
				for i := range ns2.Tasks {
					if ns2.Tasks[i].ID == taskID && ns2.Tasks[i].State == state.StatusBlocked {
						ns2.Tasks[i].State = state.StatusInProgress
						ns2.Tasks[i].BlockedReason = ""
						break
					}
				}
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

// createRemediationSubtasks checks if the given task is an audit with
// open gaps and, if so, creates a subtask for each gap. Returns the
// number of subtasks created (0 if the task isn't an audit or has no gaps).
func (d *Daemon) createRemediationSubtasks(nodeAddr, taskID string) int {
	var created int
	_ = d.Store.MutateNode(nodeAddr, func(ns *state.NodeState) error {
		// Find the audit task
		auditIdx := -1
		for i, t := range ns.Tasks {
			if t.ID == taskID && t.IsAudit {
				auditIdx = i
				break
			}
		}
		if auditIdx < 0 {
			return nil
		}

		// Collect open gaps
		var openGaps []state.Gap
		for _, g := range ns.Audit.Gaps {
			if g.Status == state.GapOpen {
				openGaps = append(openGaps, g)
			}
		}
		if len(openGaps) == 0 {
			return nil
		}

		// Create a subtask for each open gap
		for i, g := range openGaps {
			childID := fmt.Sprintf("%s.%04d", taskID, i+1)
			ns.Tasks = append(ns.Tasks, state.Task{
				ID:          childID,
				Description: fmt.Sprintf("Fix: %s", g.Description),
				State:       state.StatusNotStarted,
			})
			created++
		}

		// Reset the audit task to not_started so it doesn't stay blocked.
		// Navigation will pick up the children first (depth-first), and
		// when they complete, DeriveParentStatus resets the audit to
		// not_started for re-verification.
		ns.Tasks[auditIdx].State = state.StatusNotStarted
		ns.Tasks[auditIdx].BlockedReason = ""

		return nil
	})
	return created
}

// maybeWriteAuditReport checks if the completed task is an audit and, if so,
// writes a markdown report to the node's directory. This is a best-effort
// operation; failures are logged but do not block task completion.
func (d *Daemon) maybeWriteAuditReport(nodeAddr, taskID string) {
	ns, err := d.Store.ReadNode(nodeAddr)
	if err != nil {
		return
	}

	isAudit := false
	for _, t := range ns.Tasks {
		if t.ID == taskID && t.IsAudit {
			isAudit = true
			break
		}
	}
	if !isAudit {
		return
	}

	now := d.Clock.Now()
	reportPath, err := state.WriteAuditReport(d.Store.Dir(), nodeAddr, ns.Audit, ns.Name, now)
	if err != nil {
		_ = d.Logger.Log(map[string]any{
			"type":  "audit_report_error",
			"node":  nodeAddr,
			"error": err.Error(),
		})
		return
	}

	_ = d.Logger.Log(map[string]any{
		"type": "audit_report_written",
		"node": nodeAddr,
		"path": reportPath,
	})
	output.PrintHuman("  Audit report: %s", reportPath)
}

// isSupersededBlock checks whether a blocked task was actually superseded
// (work done via a different path). The model should use WOLFCASTLE_SKIP
// for these, but sometimes uses BLOCKED instead. This catches the mistake.
func (d *Daemon) isSupersededBlock(nodeAddr, taskID string) bool {
	ns, err := d.Store.ReadNode(nodeAddr)
	if err != nil {
		return false
	}
	for _, t := range ns.Tasks {
		if t.ID != taskID {
			continue
		}
		reason := strings.ToLower(t.BlockedReason)
		return strings.Contains(reason, "supersed") ||
			strings.Contains(reason, "already done") ||
			strings.Contains(reason, "already completed") ||
			strings.Contains(reason, "no longer needed") ||
			strings.Contains(reason, "replaced by") ||
			strings.Contains(reason, "done in") ||
			strings.Contains(reason, "done directly")
	}
	return false
}
