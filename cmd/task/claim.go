package task

import (
	"fmt"

	"github.com/dorkusprime/wolfcastle/cmd/cmdutil"
	"github.com/dorkusprime/wolfcastle/internal/output"
	"github.com/dorkusprime/wolfcastle/internal/state"
	"github.com/dorkusprime/wolfcastle/internal/tree"
	"github.com/spf13/cobra"
)

func newClaimCmd(app *cmdutil.App) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "claim [task-address]",
		Short: "Claim a task and begin the assault",
		Long: `Transitions a task from not_started to in_progress. The target is yours.
Use 'wolfcastle navigate' to find the next one worth claiming.

Examples:
  wolfcastle task claim my-project/task-1
  wolfcastle task claim --node my-project/task-1`,
		Args: cobra.MaximumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			if err := app.RequireIdentity(); err != nil {
				return err
			}
			nodeFlag, err := resolveNode(cmd, args, 0)
			if err != nil {
				return err
			}

			// Parse as task address (node/task-N)
			nodeAddr, taskID, err := tree.SplitTaskAddress(nodeFlag)
			if err != nil {
				return fmt.Errorf("task address must be node-path/task-id (e.g. my-project/task-1): %w", err)
			}

			// MutateNode handles save + propagation automatically.
			if err := app.State.MutateNode(nodeAddr, func(ns *state.NodeState) error {
				return state.TaskClaim(ns, taskID)
			}); err != nil {
				return err
			}

			if app.JSON {
				output.Print(output.Ok("task_claim", map[string]any{
					"address": nodeFlag,
					"task_id": taskID,
					"state":   string(state.StatusInProgress),
				}))
			} else {
				output.PrintHuman("Claimed %s", nodeFlag)
			}
			return nil
		},
	}

	cmd.Flags().String("node", "", "Task address: node-path/task-id (alias for positional argument)")
	return cmd
}
