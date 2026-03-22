package cmd

import (
	"github.com/dorkusprime/wolfcastle/cmd/cmdutil"
)

func init() {
	// Node address completions for commands that take --node with node addresses
	_ = navigateCmd.RegisterFlagCompletionFunc("node", cmdutil.CompleteNodeAddresses(app))
	_ = specCreateCmd.RegisterFlagCompletionFunc("node", cmdutil.CompleteNodeAddresses(app))
	_ = specLinkCmd.RegisterFlagCompletionFunc("node", cmdutil.CompleteNodeAddresses(app))
	_ = specListCmd.RegisterFlagCompletionFunc("node", cmdutil.CompleteNodeAddresses(app))

	// Task address completions for commands that operate on tasks
	_ = unblockCmd.RegisterFlagCompletionFunc("node", cmdutil.CompleteTaskAddresses(app))

	// Node completions for foreground execution commands
	_ = executeCmd.RegisterFlagCompletionFunc("node", cmdutil.CompleteNodeAddresses(app))
	_ = intakeCmd.RegisterFlagCompletionFunc("node", cmdutil.CompleteNodeAddresses(app))

	_ = archiveAddCmd.RegisterFlagCompletionFunc("node", cmdutil.CompleteNodeAddresses(app))
	_ = archiveDeleteCmd.RegisterFlagCompletionFunc("node", cmdutil.CompleteNodeAddresses(app))
	_ = archiveRestoreCmd.RegisterFlagCompletionFunc("node", cmdutil.CompleteNodeAddresses(app))
}
