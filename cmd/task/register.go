package task

import (
	"github.com/dorkusprime/wolfcastle/cmd/cmdutil"
	"github.com/spf13/cobra"
)

// Register creates the "task" parent command, wires up all
// subcommands, and attaches the tree to rootCmd.
func Register(app *cmdutil.App, rootCmd *cobra.Command) {
	taskCmd := &cobra.Command{
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

	addCmd := newAddCmd(app)
	claimCmd := newClaimCmd(app)
	completeCmd := newCompleteCmd(app)
	blockCmd := newBlockCmd(app)
	unblockCmd := newUnblockCmd(app)

	// Node address completions for task add (takes a node address)
	addCmd.RegisterFlagCompletionFunc("node", cmdutil.CompleteNodeAddresses(app))

	// Task address completions for commands that operate on tasks
	completeFn := cmdutil.CompleteTaskAddresses(app)
	claimCmd.RegisterFlagCompletionFunc("node", completeFn)
	completeCmd.RegisterFlagCompletionFunc("node", completeFn)
	blockCmd.RegisterFlagCompletionFunc("node", completeFn)
	unblockCmd.RegisterFlagCompletionFunc("node", completeFn)

	taskCmd.AddCommand(addCmd, claimCmd, completeCmd, blockCmd, unblockCmd)
	rootCmd.AddCommand(taskCmd)
}
