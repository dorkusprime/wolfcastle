package cmd

import (
	"fmt"
	"path/filepath"

	"github.com/dorkusprime/wolfcastle/internal/output"
	"github.com/dorkusprime/wolfcastle/internal/state"
	"github.com/dorkusprime/wolfcastle/internal/tree"
	"github.com/spf13/cobra"
)

var taskBlockCmd = &cobra.Command{
	Use:   "block [reason]",
	Short: "Block a task (transition from in_progress to blocked)",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		reason := args[0]
		nodeFlag, _ := cmd.Flags().GetString("node")
		if nodeFlag == "" {
			return fmt.Errorf("--node is required")
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
