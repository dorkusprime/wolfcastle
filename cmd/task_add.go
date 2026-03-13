package cmd

import (
	"fmt"
	"path/filepath"

	"github.com/dorkusprime/wolfcastle/internal/output"
	"github.com/dorkusprime/wolfcastle/internal/state"
	"github.com/dorkusprime/wolfcastle/internal/tree"
	"github.com/spf13/cobra"
)

var taskAddCmd = &cobra.Command{
	Use:   "add [description]",
	Short: "Add a task to a leaf node",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		description := args[0]
		nodeAddr, _ := cmd.Flags().GetString("node")
		if nodeAddr == "" {
			return fmt.Errorf("--node is required")
		}

		addr, err := tree.ParseAddress(nodeAddr)
		if err != nil {
			return fmt.Errorf("invalid node address: %w", err)
		}
		nsPath := filepath.Join(resolver.ProjectsDir(), filepath.Join(addr.Parts...))
		statePath := filepath.Join(nsPath, "state.json")

		ns, err := state.LoadNodeState(statePath)
		if err != nil {
			return fmt.Errorf("loading node state: %w", err)
		}

		task, err := state.TaskAdd(ns, description)
		if err != nil {
			return err
		}

		if err := state.SaveNodeState(statePath, ns); err != nil {
			return err
		}

		taskAddr := nodeAddr + "/" + task.ID
		if jsonOutput {
			output.Print(output.Ok("task_add", map[string]string{
				"address":     taskAddr,
				"task_id":     task.ID,
				"description": description,
				"state":       string(task.State),
			}))
		} else {
			output.PrintHuman("Added task %s: %s", taskAddr, description)
		}
		return nil
	},
}

func init() {
	taskAddCmd.Flags().String("node", "", "Target leaf node address (required)")
	taskAddCmd.MarkFlagRequired("node")
	taskCmd.AddCommand(taskAddCmd)
}
