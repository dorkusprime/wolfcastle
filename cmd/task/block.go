package task

import (
	"fmt"
	"strings"

	"github.com/dorkusprime/wolfcastle/cmd/cmdutil"
	"github.com/dorkusprime/wolfcastle/internal/output"
	"github.com/dorkusprime/wolfcastle/internal/state"
	"github.com/dorkusprime/wolfcastle/internal/tree"
	"github.com/spf13/cobra"
)

func newBlockCmd(app *cmdutil.App) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "block [reason]",
		Short: "Block a task with a reason",
		Long: `Something stopped the advance. Record why. The task must be in_progress.
State propagates upward. Use 'wolfcastle task unblock' to resume.

Examples:
  wolfcastle task block --node my-project/task-1 "waiting on upstream API"
  wolfcastle task block --node auth/login/task-2 "needs design review"`,
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) < 1 {
				return fmt.Errorf("missing required argument: <reason>")
			}
			if err := app.RequireIdentity(); err != nil {
				return err
			}
			reason := args[0]
			if strings.TrimSpace(reason) == "" {
				return fmt.Errorf("block reason cannot be empty. State why")
			}
			nodeFlag, _ := cmd.Flags().GetString("node")
			if nodeFlag == "" {
				return fmt.Errorf("--node is required: specify the task address (e.g. my-project/task-1)")
			}

			nodeAddr, taskID, err := tree.SplitTaskAddress(nodeFlag)
			if err != nil {
				return fmt.Errorf("--node must be a task address: %w", err)
			}

			// MutateNode handles save + propagation automatically.
			if err := app.State.MutateNode(nodeAddr, func(ns *state.NodeState) error {
				return state.TaskBlock(ns, taskID, reason)
			}); err != nil {
				return err
			}

			if app.JSON {
				output.Print(output.Ok("task_block", map[string]any{
					"address": nodeFlag,
					"task_id": taskID,
					"state":   string(state.StatusBlocked),
					"reason":  reason,
				}))
			} else {
				output.PrintHuman("Blocked task %s: %s", nodeFlag, reason)
			}
			return nil
		},
	}

	cmd.Flags().String("node", "", "Task address: node-path/task-id (required)")
	_ = cmd.MarkFlagRequired("node")
	return cmd
}
