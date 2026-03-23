package task

import (
	"fmt"

	"github.com/dorkusprime/wolfcastle/cmd/cmdutil"
	"github.com/dorkusprime/wolfcastle/internal/output"
	"github.com/dorkusprime/wolfcastle/internal/state"
	"github.com/dorkusprime/wolfcastle/internal/tree"
	"github.com/spf13/cobra"
)

func newUnblockCmd(app *cmdutil.App) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "unblock [task-address]",
		Short: "Free a blocked task",
		Long: `Resets a blocked task to not_started. Failure counter goes to zero.
This is the simple reset. For model-assisted diagnosis, use
'wolfcastle unblock --node <task>' instead.

Examples:
  wolfcastle task unblock my-project/task-1
  wolfcastle task unblock --node my-project/task-1`,
		Args: cobra.MaximumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			if err := app.RequireIdentity(); err != nil {
				return err
			}
			nodeFlag, err := resolveNode(cmd, args, 0)
			if err != nil {
				return err
			}

			nodeAddr, taskID, err := tree.SplitTaskAddress(nodeFlag)
			if err != nil {
				return fmt.Errorf("task address must be node-path/task-id: %w", err)
			}

			// MutateNode handles save + propagation automatically.
			if err := app.State.MutateNode(nodeAddr, func(ns *state.NodeState) error {
				return state.TaskUnblock(ns, taskID)
			}); err != nil {
				return err
			}

			if app.JSON {
				output.Print(output.Ok("task_unblock", map[string]any{
					"address": nodeFlag,
					"task_id": taskID,
					"state":   string(state.StatusNotStarted),
				}))
			} else {
				output.PrintHuman("Unblocked %s. Counter reset. Ready for another round.", nodeFlag)
			}
			return nil
		},
	}

	cmd.Flags().String("node", "", "Task address: node-path/task-id (alias for positional argument)")
	return cmd
}
