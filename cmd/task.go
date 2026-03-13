package cmd

import "github.com/spf13/cobra"

var taskCmd = &cobra.Command{
	Use:   "task",
	Short: "Manage tasks within leaf nodes",
	Long: `Add, claim, complete, block, and unblock tasks within leaf project nodes.

Tasks follow a lifecycle: not_started -> in_progress -> complete (or blocked).
Each leaf node must have an audit task as its final task.

Examples:
  wolfcastle task add --node my-project "implement the API endpoint"
  wolfcastle task claim --node my-project/task-1
  wolfcastle task complete --node my-project/task-1
  wolfcastle task block --node my-project/task-1 "waiting on API spec"
  wolfcastle task unblock --node my-project/task-1`,
}

func init() {
	rootCmd.AddCommand(taskCmd)
}
