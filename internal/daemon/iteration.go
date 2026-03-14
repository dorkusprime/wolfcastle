package daemon

import (
	"context"
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

	if err := state.TaskClaim(ns, nav.TaskID); err != nil {
		return fmt.Errorf("claiming task %s: %w", nav.TaskID, err)
	}
	if err := state.SaveNodeState(statePath, ns); err != nil {
		return fmt.Errorf("saving node state after claim: %w", err)
	}

	// Propagate claim state to ancestors and root index (ADR-024)
	if err := d.propagateState(nav.NodeAddress, ns.State, idx); err != nil {
		return fmt.Errorf("propagating state after claim: %w", err)
	}

	// Check inbox state once for stage-skip decisions (ADR-039)
	inboxPath := filepath.Join(d.Resolver.ProjectsDir(), "inbox.json")
	hasNewItems, hasExpandedItems := d.checkInboxState(inboxPath)

	// Run pipeline stages
	for _, stage := range d.Config.Pipeline.Stages {
		if !stage.IsEnabled() {
			continue
		}

		switch stage.Name {
		case "expand":
			if !hasNewItems {
				d.Logger.Log(map[string]any{"type": "stage_skip", "stage": "expand", "reason": "no_new_inbox_items"})
				continue
			}
			if err := d.runExpandStage(ctx, stage); err != nil {
				d.Logger.Log(map[string]any{"type": "stage_error", "stage": "expand", "error": err.Error()})
				// Non-fatal: expand failure doesn't block execution
				output.PrintHuman("  Expand stage error (non-fatal): %v", err)
			}
			// Re-check inbox state after expand — items may now be expanded
			hasNewItems, hasExpandedItems = d.checkInboxState(inboxPath)
			continue

		case "file":
			if !hasExpandedItems {
				d.Logger.Log(map[string]any{"type": "stage_skip", "stage": "file", "reason": "no_expanded_inbox_items"})
				continue
			}
			if err := d.runFileStage(ctx, stage); err != nil {
				d.Logger.Log(map[string]any{"type": "stage_error", "stage": "file", "error": err.Error()})
				output.PrintHuman("  File stage error (non-fatal): %v", err)
			}
			continue
		}

		// Skip execute stage if there are expanded items awaiting filing —
		// prioritize filing over execution to avoid working on a stale tree.
		if hasExpandedItems {
			d.Logger.Log(map[string]any{"type": "stage_skip", "stage": stage.Name, "reason": "pending_filing"})
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

		d.Logger.Log(map[string]any{
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
			d.Logger.Log(map[string]any{"type": "stage_error", "stage": stage.Name, "error": err.Error()})
			return err
		}

		d.Logger.Log(map[string]any{
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
			d.Logger.Log(map[string]any{"type": "save_error", "error": err.Error()})
		}
		if err := d.propagateState(nav.NodeAddress, ns.State, idx); err != nil {
			d.Logger.Log(map[string]any{"type": "propagate_error", "error": err.Error()})
		}

		// Check for terminal markers
		if strings.Contains(result.Stdout, "WOLFCASTLE_YIELD") {
			d.Logger.Log(map[string]any{"type": "terminal_marker", "marker": "WOLFCASTLE_YIELD"})
			return nil
		}
		if strings.Contains(result.Stdout, "WOLFCASTLE_BLOCKED") {
			d.Logger.Log(map[string]any{"type": "terminal_marker", "marker": "WOLFCASTLE_BLOCKED", "task": nav.TaskID})
			return nil
		}
		if strings.Contains(result.Stdout, "WOLFCASTLE_COMPLETE") {
			d.Logger.Log(map[string]any{"type": "terminal_marker", "marker": "WOLFCASTLE_COMPLETE"})
			return nil
		}

		// No terminal marker — increment failure count
		d.Logger.Log(map[string]any{
			"type":  "no_terminal_marker",
			"empty": result.Stdout == "",
			"task":  nav.TaskID,
		})

		failCount, err := state.IncrementFailure(ns, nav.TaskID)
		if err != nil {
			d.Logger.Log(map[string]any{"type": "failure_increment_error", "error": err.Error()})
		} else {
			d.Logger.Log(map[string]any{"type": "failure_increment", "task": nav.TaskID, "count": failCount})

			if failCount >= d.Config.Failure.DecompositionThreshold && d.Config.Failure.DecompositionThreshold > 0 {
				if ns.DecompositionDepth < d.Config.Failure.MaxDecompositionDepth {
					d.Logger.Log(map[string]any{"type": "decomposition_threshold", "task": nav.TaskID, "depth": ns.DecompositionDepth})
					state.SetNeedsDecomposition(ns, nav.TaskID, true)
				} else {
					d.Logger.Log(map[string]any{"type": "auto_block", "task": nav.TaskID, "reason": "max_decomposition_depth"})
					if blockErr := state.TaskBlock(ns, nav.TaskID, "auto-blocked: decomposition threshold reached at max depth"); blockErr != nil {
						d.Logger.Log(map[string]any{"type": "auto_block_error", "task": nav.TaskID, "error": blockErr.Error()})
					}
				}
			}

			if failCount >= d.Config.Failure.HardCap && d.Config.Failure.HardCap > 0 {
				d.Logger.Log(map[string]any{"type": "auto_block", "task": nav.TaskID, "reason": "hard_cap", "count": failCount})
				if blockErr := state.TaskBlock(ns, nav.TaskID, fmt.Sprintf("auto-blocked: failure hard cap reached (%d)", failCount)); blockErr != nil {
					d.Logger.Log(map[string]any{"type": "auto_block_error", "task": nav.TaskID, "error": blockErr.Error()})
				}
			}

			if err := state.SaveNodeState(statePath, ns); err != nil {
				d.Logger.Log(map[string]any{"type": "save_error", "error": err.Error()})
			}
			if err := d.propagateState(nav.NodeAddress, ns.State, idx); err != nil {
				d.Logger.Log(map[string]any{"type": "propagate_error", "error": err.Error()})
			}
		}
	}

	return nil
}
