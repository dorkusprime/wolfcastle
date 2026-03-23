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
		Use:   "block [task-address] <reason>",
		Short: "Block a task with a reason",
		Long: `Something stopped the advance. Record why. The task must be in_progress.
State propagates upward. Use 'wolfcastle task unblock' to resume.

When using the positional form, the task address comes first and the
reason comes second. When using --node, the reason is the only argument.

Examples:
  wolfcastle task block my-project/task-1 "waiting on upstream API"
  wolfcastle task block --node my-project/task-1 "waiting on upstream API"
  wolfcastle task block --node auth/login/task-2 "needs design review"`,
		RunE: func(cmd *cobra.Command, args []string) error {
			if err := app.RequireIdentity(); err != nil {
				return err
			}

			flagChanged := cmd.Flags().Changed("node")
			nodeFlag, _ := cmd.Flags().GetString("node")

			var nodeValue, reason string

			switch {
			case flagChanged && len(args) >= 2:
				// --node provided AND two positional args: ambiguous
				return fmt.Errorf("specify the task address as a positional argument or with --node, not both")
			case flagChanged:
				// --node provided; args[0] is the reason (original behavior)
				if nodeFlag == "" {
					return fmt.Errorf("--node value cannot be empty: specify the task address (e.g. my-project/task-1)")
				}
				nodeValue = nodeFlag
				if len(args) < 1 {
					return fmt.Errorf("missing required argument: <reason>")
				}
				reason = args[0]
			case len(args) >= 2:
				// Positional form: args[0]=node, args[1]=reason
				nodeValue = args[0]
				reason = args[1]
			case len(args) == 1:
				// Single arg without --node: could be reason (old habit) or node. Error clearly.
				return fmt.Errorf("two arguments required: <task-address> <reason>, or use --node <task-address> <reason>")
			default:
				return fmt.Errorf("task address and reason required (e.g. wolfcastle task block my-project/task-1 \"reason\")")
			}

			if strings.TrimSpace(reason) == "" {
				return fmt.Errorf("block reason cannot be empty. State why")
			}

			nodeAddr, taskID, err := tree.SplitTaskAddress(nodeValue)
			if err != nil {
				return fmt.Errorf("task address must be node-path/task-id: %w", err)
			}

			// MutateNode handles save + propagation automatically.
			if err := app.State.MutateNode(nodeAddr, func(ns *state.NodeState) error {
				return state.TaskBlock(ns, taskID, reason)
			}); err != nil {
				return err
			}

			if app.JSON {
				output.Print(output.Ok("task_block", map[string]any{
					"address": nodeValue,
					"task_id": taskID,
					"state":   string(state.StatusBlocked),
					"reason":  reason,
				}))
			} else {
				output.PrintHuman("Blocked task %s: %s", nodeValue, reason)
			}
			return nil
		},
	}

	cmd.Flags().String("node", "", "Task address: node-path/task-id (alias for positional argument)")
	return cmd
}
