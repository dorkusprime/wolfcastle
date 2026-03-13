package cmd

import (
	"context"
	"fmt"
	"os/exec"
	"path/filepath"
	"time"

	"github.com/dorkusprime/wolfcastle/internal/output"
	"github.com/dorkusprime/wolfcastle/internal/state"
	"github.com/dorkusprime/wolfcastle/internal/tree"
	"github.com/spf13/cobra"
)

var taskCompleteCmd = &cobra.Command{
	Use:   "complete",
	Short: "Complete a task (transition from in_progress to complete)",
	RunE: func(cmd *cobra.Command, args []string) error {
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

		if err := state.TaskComplete(ns, taskID); err != nil {
			return err
		}

		// Run configured validation commands before saving
		if cfg != nil {
			for _, vc := range cfg.Validation.Commands {
				timeout := 30 * time.Second
				if vc.TimeoutSeconds > 0 {
					timeout = time.Duration(vc.TimeoutSeconds) * time.Second
				}
				ctx, cancel := context.WithTimeout(context.Background(), timeout)
				out, err := exec.CommandContext(ctx, "sh", "-c", vc.Run).CombinedOutput()
				cancel()
				if err != nil {
					// Undo the completion by reverting task state
					return fmt.Errorf("validation command %q failed: %v\n%s", vc.Name, err, string(out))
				}
			}
		}

		if err := state.SaveNodeState(statePath, ns); err != nil {
			return err
		}

		// Propagate state up through parent orchestrators and root index
		if err := propagateState(nodeAddr, ns.State); err != nil {
			return fmt.Errorf("propagating state: %w", err)
		}

		if jsonOutput {
			output.Print(output.Ok("task_complete", map[string]any{
				"address":    nodeFlag,
				"task_id":    taskID,
				"state":      string(state.StatusComplete),
				"node_state": string(ns.State),
			}))
		} else {
			output.PrintHuman("Completed task %s", nodeFlag)
			if ns.State == state.StatusComplete {
				output.PrintHuman("Node %s is now complete", nodeAddr)
			}
		}
		return nil
	},
}

func init() {
	taskCompleteCmd.Flags().String("node", "", "Task address: node-path/task-id (required)")
	taskCompleteCmd.MarkFlagRequired("node")
	taskCmd.AddCommand(taskCompleteCmd)
}
