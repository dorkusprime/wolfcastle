package cmd

import (
	"fmt"
	"path/filepath"
	"strings"

	"github.com/dorkusprime/wolfcastle/internal/output"
	"github.com/dorkusprime/wolfcastle/internal/state"
	"github.com/dorkusprime/wolfcastle/internal/tree"
	"github.com/spf13/cobra"
)

var taskBlockCmd = &cobra.Command{
	Use:   "block [reason]",
	Short: "Block a task (transition from in_progress to blocked)",
	Long: `Marks a task as blocked with a reason. The task must currently be in_progress.

Blocking a task propagates state changes up through parent orchestrators.
Use 'wolfcastle task unblock' or 'wolfcastle unblock' to resolve the block.

Examples:
  wolfcastle task block --node my-project/task-1 "waiting on upstream API"
  wolfcastle task block --node auth/login/task-2 "needs design review"`,
	Args: cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		if err := requireResolver(); err != nil {
			return err
		}
		reason := args[0]
		if strings.TrimSpace(reason) == "" {
			return fmt.Errorf("block reason cannot be empty — describe why the task is blocked")
		}
		nodeFlag, _ := cmd.Flags().GetString("node")
		if nodeFlag == "" {
			return fmt.Errorf("--node is required — specify the task address (e.g. my-project/task-1)")
		}

		nodeAddr, taskID, err := tree.SplitTaskAddress(nodeFlag)
		if err != nil {
			return fmt.Errorf("--node must be a task address: %w", err)
		}

		addr, err := tree.ParseAddress(nodeAddr)
		if err != nil {
			return fmt.Errorf("invalid node address: %w", err)
		}
		statePath := filepath.Join(resolver.ProjectsDir(), filepath.Join(addr.Parts...), "state.json")

		ns, err := state.LoadNodeState(statePath)
		if err != nil {
			return fmt.Errorf("loading node state: %w", err)
		}

		if err := state.TaskBlock(ns, taskID, reason); err != nil {
			return err
		}

		if err := state.SaveNodeState(statePath, ns); err != nil {
			return err
		}

		// Propagate state up through parent orchestrators and root index
		if err := propagateState(nodeAddr, ns.State); err != nil {
			return fmt.Errorf("propagating state: %w", err)
		}

		if jsonOutput {
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

func init() {
	taskBlockCmd.Flags().String("node", "", "Task address: node-path/task-id (required)")
	taskBlockCmd.MarkFlagRequired("node")
	taskCmd.AddCommand(taskBlockCmd)
}
