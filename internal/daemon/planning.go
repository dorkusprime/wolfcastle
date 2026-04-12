package daemon

import (
	"context"
	"fmt"
	"time"

	"github.com/dorkusprime/wolfcastle/internal/config"
	"github.com/dorkusprime/wolfcastle/internal/invoke"
	"github.com/dorkusprime/wolfcastle/internal/pipeline"
	"github.com/dorkusprime/wolfcastle/internal/state"
)

// reconcileOrchestratorStates walks every orchestrator in the index and
// reconciles its persisted state against reality. For each orchestrator,
// it reads the current state of every child from disk, patches the
// ChildRef entries, and recomputes the orchestrator's own status via
// RecomputeState. This catches staleness that accumulates between
// planning passes (e.g., a parent stuck at not_started while all its
// children have completed). The daemon's selfHeal at startup remains
// as a safety net; this routine handles the same class of drift during
// normal operation.
func (d *Daemon) reconcileOrchestratorStates(idx *state.RootIndex) {
	if !d.Config.Pipeline.Planning.Enabled {
		return
	}

	for addr, entry := range idx.Nodes {
		if entry.Type != state.NodeOrchestrator {
			continue
		}

		ns, err := d.Store.ReadNode(addr)
		if err != nil {
			continue
		}

		// Skip orchestrators with no children; nothing to reconcile.
		if len(ns.Children) == 0 {
			continue
		}

		// Check whether any ChildRef is out of sync with the index.
		needsUpdate := false
		for _, child := range ns.Children {
			idxEntry, ok := idx.Nodes[child.Address]
			if !ok {
				continue
			}
			if child.State != idxEntry.State {
				needsUpdate = true
				break
			}
		}
		if !needsUpdate {
			continue
		}

		_ = d.Store.MutateNode(addr, func(ns *state.NodeState) error {
			changed := false
			for i := range ns.Children {
				idxEntry, ok := idx.Nodes[ns.Children[i].Address]
				if !ok {
					continue
				}
				if ns.Children[i].State != idxEntry.State {
					ns.Children[i].State = idxEntry.State
					changed = true
				}
			}
			if !changed {
				return errNoChange
			}
			newState := state.RecomputeState(ns.Children, ns.Tasks)
			if newState == state.StatusComplete && ns.NeedsPlanning {
				newState = state.StatusInProgress
			}
			ns.State = newState
			return nil
		})
	}
}

// findPlanningTarget searches the tree depth-first for an orchestrator
// that needs planning. Called only when no actionable task exists, making
// planning lazy: each orchestrator gets planned right before its subtree
// needs work. Returns the node address and state, or empty string if no
// orchestrator needs planning.
func (d *Daemon) findPlanningTarget(idx *state.RootIndex) (string, *state.NodeState) {
	if !d.Config.Pipeline.Planning.Enabled {
		return "", nil
	}

	// DFS through the tree looking for NeedsPlanning
	var roots []string
	if d.ScopeNode != "" {
		roots = []string{d.ScopeNode}
	} else if len(idx.Root) > 0 {
		roots = idx.Root
	}

	for _, root := range roots {
		addr, ns := d.dfsFindPlanning(idx, root)
		if addr != "" {
			return addr, ns
		}
	}
	return "", nil
}

// dfsFindPlanning does depth-first search for an orchestrator needing planning.
func (d *Daemon) dfsFindPlanning(idx *state.RootIndex, addr string) (string, *state.NodeState) {
	entry, ok := idx.Nodes[addr]
	if !ok || entry.State == state.StatusComplete {
		return "", nil
	}

	if entry.Type == state.NodeOrchestrator {
		ns, err := d.Store.ReadNode(addr)
		if err != nil {
			return "", nil
		}
		// An orchestrator needs planning if:
		// 1. NeedsPlanning is explicitly set (re-planning triggers, intake), or
		// 2. It has no children and no non-audit tasks (never planned).
		// Case 2 means the daemon infers the need from structure rather than
		// requiring the creator to set a flag. Audit tasks don't count as
		// "real" tasks since they're created automatically on all nodes.
		needsPlanning := ns.NeedsPlanning
		if !needsPlanning && len(ns.Children) == 0 {
			hasNonAuditTasks := false
			for _, t := range ns.Tasks {
				if !t.IsAudit {
					hasNonAuditTasks = true
					break
				}
			}
			if !hasNonAuditTasks {
				needsPlanning = true
				ns.PlanningTrigger = "initial"
			}
		}
		if needsPlanning {
			return addr, ns
		}
		// Check children depth-first
		for _, childAddr := range entry.Children {
			found, foundNS := d.dfsFindPlanning(idx, childAddr)
			if found != "" {
				return found, foundNS
			}
		}
	}

	return "", nil
}

// runPlanningPass executes a planning invocation for the given orchestrator.
func (d *Daemon) runPlanningPass(ctx context.Context, nodeAddr string, ns *state.NodeState, idx *state.RootIndex) error {
	trigger := ns.PlanningTrigger
	if trigger == "" {
		trigger = "initial"
	}

	// Snapshot pending scope count before invocation. Scope items that
	// arrive during the pass (from intake OVERLAP delivery) must survive;
	// we only clear items that existed when the pass started.
	prePlanScopeCount := len(ns.PendingScope)

	d.iteration.Add(1)

	_ = d.Logger.StartIterationWithPrefix("plan")
	d.log(map[string]any{"type": "iteration_header", "iteration": int(d.iteration.Load()), "kind": "plan", "text": fmt.Sprintf("%s (%s)", nodeAddr, trigger)})
	_ = d.Logger.LogIterationStart("plan", nodeAddr)

	// Select the planning prompt variant
	promptFile := selectPlanningPrompt(trigger)

	// Ensure max review passes is set from config for context rendering.
	if ns.MaxReviewPasses == 0 {
		ns.MaxReviewPasses = d.Config.Pipeline.Planning.MaxReviewPasses
	}

	// Write activity so the TUI shows what the daemon is working on.
	d.writeActivity(nodeAddr, trigger)

	// Build planning context
	planCtx := pipeline.BuildPlanningContext(nodeAddr, ns, trigger)

	// Select model
	modelName := d.Config.Pipeline.Planning.Model
	if ns.PlanningModel != "" {
		modelName = ns.PlanningModel
	}
	model, ok := d.Config.Models[modelName]
	if !ok {
		d.Logger.Close()
		return fmt.Errorf("planning model %q not found in config. Add it to the models section or set pipeline.planning.model to an existing model name", modelName)
	}

	// Assemble the prompt
	prompt, err := pipeline.AssemblePrompt(d.WolfcastleDir, d.Config, config.PipelineStage{
		Model:      modelName,
		PromptFile: promptFile,
	}, planCtx)
	if err != nil {
		d.Logger.Close()
		return fmt.Errorf("assembling planning prompt: %w", err)
	}

	planStartTime := time.Now()
	_ = d.Logger.Log(map[string]any{
		"type":    "planning_start",
		"node":    nodeAddr,
		"trigger": trigger,
		"model":   modelName,
	})

	// Invoke the model
	result, err := d.invokeWithRetry(ctx, model, prompt, d.RepoDir, d.Logger.AssistantWriter(), "plan")
	if err != nil {
		_ = d.Logger.Log(map[string]any{"type": "planning_error", "error": err.Error()})
		d.Logger.Close()
		return fmt.Errorf("planning invocation for %s: %w", nodeAddr, err)
	}

	_ = d.Logger.Log(map[string]any{
		"type":        "planning_complete",
		"node":        nodeAddr,
		"exit_code":   result.ExitCode,
		"duration_ms": time.Since(planStartTime).Milliseconds(),
	})

	// Handle the terminal marker
	marker := scanTerminalMarker(result.Stdout)

	// Record planning pass in history
	d.recordPlanningPass(nodeAddr, trigger, marker)

	switch marker {
	case invoke.MarkerStringComplete:
		_ = d.Logger.Log(map[string]any{"type": "planning_marker", "marker": invoke.MarkerStringComplete})
		// Clear planning state. Only complete the audit task when all
		// children are already done; otherwise a premature audit
		// completion breaks the allNotStarted check in RecomputeState
		// and makes the orchestrator appear in_progress when all its
		// children are still not_started (#192).
		_ = d.Store.MutateNode(nodeAddr, func(ns *state.NodeState) error {
			ns.NeedsPlanning = false
			ns.PlanningTrigger = ""
			if len(ns.PendingScope) > prePlanScopeCount {
				ns.PendingScope = ns.PendingScope[prePlanScopeCount:]
			} else {
				ns.PendingScope = nil
			}
			// Only complete the audit task when all children are done.
			// The audit may be not_started (never executed separately for
			// orchestrators), so set it directly rather than going through
			// TaskComplete which requires in_progress.
			allChildrenComplete := true
			for _, c := range ns.Children {
				if c.State != state.StatusComplete {
					allChildrenComplete = false
					break
				}
			}
			if allChildrenComplete {
				for i := range ns.Tasks {
					if ns.Tasks[i].IsAudit {
						ns.Tasks[i].State = state.StatusComplete
						break
					}
				}
			}
			ns.State = state.RecomputeState(ns.Children, ns.Tasks)
			return nil
		})
		// Propagate the actual derived state. If planning created children
		// that haven't executed yet, the orchestrator won't be complete
		// despite its own tasks being done.
		derivedState := state.StatusNotStarted
		if freshNS, readErr := d.Store.ReadNode(nodeAddr); readErr == nil {
			derivedState = freshNS.State
		}
		if err := d.propagateState(nodeAddr, derivedState, idx); err != nil {
			_ = d.Logger.Log(map[string]any{"type": "propagate_error", "error": err.Error()})
		}

	case invoke.MarkerStringBlocked:
		_ = d.Logger.Log(map[string]any{"type": "planning_marker", "marker": invoke.MarkerStringBlocked})
		// Block the orchestrator
		_ = d.Store.MutateNode(nodeAddr, func(ns *state.NodeState) error {
			ns.NeedsPlanning = false
			ns.PlanningTrigger = ""
			ns.State = state.StatusBlocked
			return nil
		})

	case invoke.MarkerStringContinue:
		_ = d.Logger.Log(map[string]any{"type": "planning_marker", "marker": invoke.MarkerStringContinue})
		// Review found gaps and created new work. Clear NeedsPlanning;
		// it will be re-set when the new children complete.
		_ = d.Store.MutateNode(nodeAddr, func(ns *state.NodeState) error {
			ns.NeedsPlanning = false
			ns.PlanningTrigger = ""
			// Track review passes for completion_review so the exploratory
			// review loop converges at max_review_passes.
			if trigger == "completion_review" {
				ns.ReviewPass++
			}
			return nil
		})

	default:
		_ = d.Logger.Log(map[string]any{
			"type":   "planning_no_marker",
			"output": truncateOutput(result.Stdout, 200),
		})
		// No marker: treat as a failed planning pass. Increment replan count.
		d.incrementReplanCount(nodeAddr, trigger)
	}

	d.Logger.Close()
	return nil
}

// selectPlanningPrompt returns the prompt filename for the given trigger.
func selectPlanningPrompt(trigger string) string {
	switch trigger {
	case "new_scope":
		return "stages/plan-amend.md"
	case "child_blocked":
		return "stages/plan-remediate.md"
	case "completion_review":
		return "stages/plan-review.md"
	default:
		return "stages/plan-initial.md"
	}
}

// recordPlanningPass adds an entry to the orchestrator's planning history.
func (d *Daemon) recordPlanningPass(nodeAddr, trigger, marker string) {
	_ = d.Store.MutateNode(nodeAddr, func(ns *state.NodeState) error {
		pass := state.PlanningPass{
			Timestamp: d.Clock.Now(),
			Trigger:   trigger,
			Summary:   fmt.Sprintf("marker=%s", marker),
		}
		ns.PlanningHistory = append(ns.PlanningHistory, pass)
		// Cap at 5 entries
		if len(ns.PlanningHistory) > 5 {
			ns.PlanningHistory = ns.PlanningHistory[len(ns.PlanningHistory)-5:]
		}
		return nil
	})
}

// incrementReplanCount tracks cumulative replans across all triggers.
// If the budget is exceeded, the orchestrator blocks itself.
func (d *Daemon) incrementReplanCount(nodeAddr, trigger string) {
	_ = d.Store.MutateNode(nodeAddr, func(ns *state.NodeState) error {
		ns.TotalReplans++

		maxReplans := ns.MaxReplans
		if maxReplans == 0 {
			maxReplans = d.Config.Pipeline.Planning.MaxReplans
		}
		if maxReplans == 0 {
			maxReplans = 3
		}

		if ns.TotalReplans >= maxReplans {
			ns.NeedsPlanning = false
			ns.State = state.StatusBlocked
			_ = d.Logger.Log(map[string]any{
				"type":    "planning_budget_exhausted",
				"node":    nodeAddr,
				"trigger": trigger,
				"count":   ns.TotalReplans,
			})
		}
		return nil
	})
}

// checkReplanningTriggers checks if any orchestrator needs re-planning
// after a task completion or block event.
func (d *Daemon) checkReplanningTriggers(nodeAddr, taskID string, idx *state.RootIndex) {
	if !d.Config.Pipeline.Planning.Enabled {
		return
	}

	// Find the parent orchestrator for this node
	entry, ok := idx.Nodes[nodeAddr]
	if !ok {
		return
	}
	parentAddr := entry.Parent
	if parentAddr == "" {
		return
	}

	parentEntry, ok := idx.Nodes[parentAddr]
	if !ok || parentEntry.Type != state.NodeOrchestrator {
		return
	}

	parentNS, err := d.Store.ReadNode(parentAddr)
	if err != nil {
		return
	}

	// Check if all children are complete
	allComplete := true
	anyBlocked := false
	for _, child := range parentNS.Children {
		childEntry, ok := idx.Nodes[child.Address]
		if !ok {
			continue
		}
		if childEntry.State == state.StatusBlocked {
			anyBlocked = true
			allComplete = false
		} else if childEntry.State != state.StatusComplete {
			allComplete = false
		}
	}

	if allComplete && len(parentNS.SuccessCriteria) > 0 {
		// Trigger completion review
		_ = d.Store.MutateNode(parentAddr, func(ns *state.NodeState) error {
			ns.NeedsPlanning = true
			ns.PlanningTrigger = "completion_review"
			return nil
		})
		_ = d.Logger.Log(map[string]any{
			"type":    "replan_trigger",
			"node":    parentAddr,
			"trigger": "completion_review",
		})
	}
	// Orchestrators without success criteria auto-complete via existing logic.

	if anyBlocked {
		// Trigger remediation
		_ = d.Store.MutateNode(parentAddr, func(ns *state.NodeState) error {
			if !ns.NeedsPlanning {
				ns.NeedsPlanning = true
				ns.PlanningTrigger = "child_blocked"
				return nil
			}
			return nil
		})
		_ = d.Logger.Log(map[string]any{
			"type":    "replan_trigger",
			"node":    parentAddr,
			"trigger": "child_blocked",
		})
	}
}

// truncateOutput returns the first n characters of s.
func truncateOutput(s string, n int) string {
	// Handle potential rune boundary issues
	r := []rune(s)
	if len(r) <= n {
		return s
	}
	return string(r[:n]) + "..."
}

// deliverPendingScope moves buffered scope items from intake to orchestrator state.
// This is called at the start of each daemon iteration to ensure scope delivery
// happens between passes, not during them.
func (d *Daemon) deliverPendingScope(idx *state.RootIndex) {
	if !d.Config.Pipeline.Planning.Enabled {
		return
	}

	// Check each orchestrator for pending scope that needs to trigger re-planning
	for addr, entry := range idx.Nodes {
		if entry.Type != state.NodeOrchestrator {
			continue
		}
		ns, err := d.Store.ReadNode(addr)
		if err != nil {
			continue
		}
		if len(ns.PendingScope) > 0 && !ns.NeedsPlanning {
			_ = d.Store.MutateNode(addr, func(ns *state.NodeState) error {
				ns.NeedsPlanning = true
				ns.PlanningTrigger = "new_scope"
				return nil
			})
		}
	}
}
