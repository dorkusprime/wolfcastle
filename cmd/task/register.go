// Package task implements the task command group: add, claim, complete,
// block, and unblock. Tasks live within leaf nodes and follow a lifecycle
// of not_started -> in_progress -> complete (or blocked). State changes
// propagate up through parent orchestrators.
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
		Short: "Command the task lifecycle",
		Long: `Tasks are the smallest unit of destruction. Add them, claim them,
complete them, block them, unblock them. They obey or they don't.
Either way, Wolfcastle keeps moving.

Examples:
  wolfcastle task add --node my-project "implement the API endpoint"
  wolfcastle task claim my-project/task-1
  wolfcastle task complete my-project/task-1
  wolfcastle task block my-project/task-1 "waiting on API spec"
  wolfcastle task unblock my-project/task-1`,
	}

	addCmd := newAddCmd(app)
	claimCmd := newClaimCmd(app)
	completeCmd := newCompleteCmd(app)
	blockCmd := newBlockCmd(app)
	unblockCmd := newUnblockCmd(app)
	deliverableCmd := newDeliverableCmd(app)
	amendCmd := newAmendCmd(app)
	scopeCmd := newScopeCmd(app)

	// Node address completions for task add (takes a node address)
	_ = addCmd.RegisterFlagCompletionFunc("node", cmdutil.CompleteNodeAddresses(app))

	// Task address completions for commands that operate on tasks.
	// Registered on both the --node flag and as positional argument completions.
	completeFn := cmdutil.CompleteTaskAddresses(app)
	_ = claimCmd.RegisterFlagCompletionFunc("node", completeFn)
	_ = completeCmd.RegisterFlagCompletionFunc("node", completeFn)
	_ = blockCmd.RegisterFlagCompletionFunc("node", completeFn)
	_ = unblockCmd.RegisterFlagCompletionFunc("node", completeFn)
	_ = deliverableCmd.RegisterFlagCompletionFunc("node", completeFn)
	_ = amendCmd.RegisterFlagCompletionFunc("node", completeFn)

	// Positional argument completions for commands that now accept task address
	// as the first positional argument.
	claimCmd.ValidArgsFunction = completeFn
	completeCmd.ValidArgsFunction = completeFn
	unblockCmd.ValidArgsFunction = completeFn
	amendCmd.ValidArgsFunction = completeFn
	blockCmd.ValidArgsFunction = completeFn

	taskCmd.AddCommand(addCmd, claimCmd, completeCmd, blockCmd, unblockCmd, deliverableCmd, amendCmd, scopeCmd)
	taskCmd.GroupID = "work"
	rootCmd.AddCommand(taskCmd)
}
