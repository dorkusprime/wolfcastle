package audit

import (
	"fmt"
	"strings"

	"github.com/dorkusprime/wolfcastle/cmd/cmdutil"
	"github.com/dorkusprime/wolfcastle/internal/output"
	"github.com/dorkusprime/wolfcastle/internal/state"
	"github.com/spf13/cobra"
)

func newFixGapCmd(app *cmdutil.App) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "fix-gap [gap-id]",
		Short: "Close an audit gap",
		Long: `Marks an open gap as fixed. Records who and when.

Examples:
  wolfcastle audit fix-gap --node my-project gap-my-project-1`,
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) < 1 {
				return fmt.Errorf("missing required argument: <gap-id>")
			}
			if err := app.RequireIdentity(); err != nil {
				return err
			}
			gapID := args[0]
			nodeAddr, _ := cmd.Flags().GetString("node")
			if nodeAddr == "" {
				return fmt.Errorf("--node is required: specify the target node address")
			}

			if err := app.State.MutateNode(nodeAddr, func(ns *state.NodeState) error {
				// Find and fix the gap.
				gapIdx := -1
				for i := range ns.Audit.Gaps {
					if ns.Audit.Gaps[i].ID == gapID {
						gapIdx = i
						break
					}
				}
				if gapIdx < 0 {
					return fmt.Errorf("gap %s not found in %s", gapID, nodeAddr)
				}
				if ns.Audit.Gaps[gapIdx].Status == state.GapFixed {
					return fmt.Errorf("gap %s is already fixed", gapID)
				}
				ns.Audit.Gaps[gapIdx].Status = state.GapFixed
				ns.Audit.Gaps[gapIdx].FixedBy = nodeAddr
				now := app.Clock.Now()
				ns.Audit.Gaps[gapIdx].FixedAt = &now

				// Complete the corresponding remediation task.
				remTaskID := ns.Audit.Gaps[gapIdx].RemediationTaskID
				if remTaskID == "" {
					// Backward compat: scan task descriptions for gap ID.
					remTaskID = findRemediationTaskByDescription(ns, gapID)
				}
				if remTaskID != "" {
					completeRemediationTask(ns, remTaskID)
				}

				// Sync audit lifecycle and derive audit task state.
				state.SyncAuditLifecycle(ns, app.Clock)
				syncAuditTaskState(ns)

				// If all gaps are fixed and the node was blocked, transition
				// to in_progress so the daemon can re-enter it.
				if ns.State == state.StatusBlocked && !hasOpenGaps(ns) {
					ns.State = state.StatusInProgress
				}

				return nil
			}); err != nil {
				return err
			}

			if app.JSON {
				output.Print(output.Ok("audit_fix_gap", map[string]string{
					"node":   nodeAddr,
					"gap_id": gapID,
				}))
			} else {
				output.PrintHuman("Gap %s marked as fixed on %s", gapID, nodeAddr)
			}
			return nil
		},
	}

	cmd.Flags().String("node", "", "Target node address (required)")
	_ = cmd.MarkFlagRequired("node")
	return cmd
}

// findRemediationTaskByDescription scans task descriptions for a gap ID
// as a fallback when RemediationTaskID is not set (pre-existing gaps).
func findRemediationTaskByDescription(ns *state.NodeState, gapID string) string {
	for _, t := range ns.Tasks {
		if strings.Contains(t.Description, "wolfcastle audit fix-gap") && strings.HasSuffix(strings.TrimSpace(t.Description), gapID) {
			return t.ID
		}
	}
	return ""
}

// completeRemediationTask force-completes a remediation task regardless
// of its current state (not_started, in_progress, or blocked).
func completeRemediationTask(ns *state.NodeState, taskID string) {
	for i := range ns.Tasks {
		if ns.Tasks[i].ID == taskID && ns.Tasks[i].State != state.StatusComplete {
			ns.Tasks[i].State = state.StatusComplete
			ns.Tasks[i].BlockedReason = ""
			return
		}
	}
}

// syncAuditTaskState re-derives the audit task status from its children.
// When all remediation subtasks are complete, the audit task resets to
// not_started so it can be re-verified.
func syncAuditTaskState(ns *state.NodeState) {
	for i := range ns.Tasks {
		if !ns.Tasks[i].IsAudit {
			continue
		}
		derived, hasChildren := state.DeriveParentStatus(ns, ns.Tasks[i].ID)
		if hasChildren && derived != ns.Tasks[i].State {
			ns.Tasks[i].State = derived
			ns.Tasks[i].BlockedReason = ""
		}
	}
}

// hasOpenGaps returns true if any gap in the node has status "open".
func hasOpenGaps(ns *state.NodeState) bool {
	for _, g := range ns.Audit.Gaps {
		if g.Status == state.GapOpen {
			return true
		}
	}
	return false
}
