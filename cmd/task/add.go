package task

import (
	"fmt"
	"path/filepath"
	"strings"

	"github.com/dorkusprime/wolfcastle/cmd/cmdutil"
	"github.com/dorkusprime/wolfcastle/internal/output"
	"github.com/dorkusprime/wolfcastle/internal/state"
	"github.com/dorkusprime/wolfcastle/internal/tree"
	"github.com/spf13/cobra"
)

func newAddCmd(app *cmdutil.App) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "add [description]",
		Short: "Add a task to a leaf node",
		Long: `Adds a new task to a leaf node's task list.

The task is created in the not_started state. Tasks are executed in order
by the daemon. Use 'wolfcastle navigate' to find the next actionable task.

Examples:
  wolfcastle task add --node my-project "implement the API endpoint"
  wolfcastle task add --node auth/login "add rate limiting"`,
		Args: cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			if err := app.RequireResolver(); err != nil {
				return err
			}
			description := args[0]
			if strings.TrimSpace(description) == "" {
				return fmt.Errorf("task description cannot be empty")
			}
			nodeAddr, _ := cmd.Flags().GetString("node")
			if nodeAddr == "" {
				return fmt.Errorf("--node is required: specify the target leaf node address")
			}

			addr, err := tree.ParseAddress(nodeAddr)
			if err != nil {
				return fmt.Errorf("invalid node address: %w", err)
			}
			nsPath := filepath.Join(app.Resolver.ProjectsDir(), filepath.Join(addr.Parts...))
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
			if app.JSONOutput {
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

	cmd.Flags().String("node", "", "Target leaf node address (required)")
	_ = cmd.MarkFlagRequired("node")
	return cmd
}
