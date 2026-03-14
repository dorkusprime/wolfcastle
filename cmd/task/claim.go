package task

import (
	"fmt"
	"path/filepath"

	"github.com/dorkusprime/wolfcastle/cmd/cmdutil"
	"github.com/dorkusprime/wolfcastle/internal/output"
	"github.com/dorkusprime/wolfcastle/internal/state"
	"github.com/dorkusprime/wolfcastle/internal/tree"
	"github.com/spf13/cobra"
)

func newClaimCmd(app *cmdutil.App) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "claim",
		Short: "Claim a task (transition from not_started to in_progress)",
		Long: `Claims a task, transitioning it from not_started to in_progress.

Use 'wolfcastle navigate' to find the next claimable task, then claim it.

Examples:
  wolfcastle task claim --node my-project/task-1`,
		RunE: func(cmd *cobra.Command, args []string) error {
			if err := app.RequireResolver(); err != nil {
				return err
			}
			nodeFlag, _ := cmd.Flags().GetString("node")
			if nodeFlag == "" {
				return fmt.Errorf("--node is required — specify the task address (e.g. my-project/task-1)")
			}

			// Parse as task address (node/task-N)
			nodeAddr, taskID, err := tree.SplitTaskAddress(nodeFlag)
			if err != nil {
				return fmt.Errorf("--node must be a task address (e.g. my-project/task-1): %w", err)
			}

			addr, err := tree.ParseAddress(nodeAddr)
			if err != nil {
				return fmt.Errorf("invalid node address: %w", err)
			}
			statePath := filepath.Join(app.Resolver.ProjectsDir(), filepath.Join(addr.Parts...), "state.json")

			ns, err := state.LoadNodeState(statePath)
			if err != nil {
				return fmt.Errorf("loading node state: %w", err)
			}

			if err := state.TaskClaim(ns, taskID); err != nil {
				return err
			}

			if err := state.SaveNodeState(statePath, ns); err != nil {
				return err
			}

			// Propagate state up through parent orchestrators and root index
			if err := app.PropagateState(nodeAddr, ns.State); err != nil {
				return fmt.Errorf("propagating state: %w", err)
			}

			if app.JSONOutput {
				output.Print(output.Ok("task_claim", map[string]any{
					"address": nodeFlag,
					"task_id": taskID,
					"state":   string(state.StatusInProgress),
				}))
			} else {
				output.PrintHuman("Claimed task %s", nodeFlag)
			}
			return nil
		},
	}

	cmd.Flags().String("node", "", "Task address: node-path/task-id (required)")
	_ = cmd.MarkFlagRequired("node")
	return cmd
}
