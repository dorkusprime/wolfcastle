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

func newUnblockCmd(app *cmdutil.App) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "unblock",
		Short: "Unblock a task (transition from blocked to not_started, reset failure counter)",
		Long: `Resets a blocked task back to not_started and clears its failure counter.

This is the simple (Tier 1) unblock. For model-assisted debugging, use
'wolfcastle unblock --node <task>' instead.

Examples:
  wolfcastle task unblock --node my-project/task-1`,
		RunE: func(cmd *cobra.Command, args []string) error {
			if err := app.RequireResolver(); err != nil {
				return err
			}
			nodeFlag, _ := cmd.Flags().GetString("node")
			if nodeFlag == "" {
				return fmt.Errorf("--node is required: specify the blocked task address (e.g. my-project/task-1)")
			}

			nodeAddr, taskID, err := tree.SplitTaskAddress(nodeFlag)
			if err != nil {
				return fmt.Errorf("--node must be a task address: %w", err)
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

			if err := state.TaskUnblock(ns, taskID); err != nil {
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
				output.Print(output.Ok("task_unblock", map[string]any{
					"address": nodeFlag,
					"task_id": taskID,
					"state":   string(state.StatusNotStarted),
				}))
			} else {
				output.PrintHuman("Unblocked task %s (reset to not_started, failure counter cleared)", nodeFlag)
			}
			return nil
		},
	}

	cmd.Flags().String("node", "", "Task address: node-path/task-id (required)")
	_ = cmd.MarkFlagRequired("node")
	return cmd
}
