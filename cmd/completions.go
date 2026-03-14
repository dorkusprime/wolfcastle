package cmd

import (
	"github.com/dorkusprime/wolfcastle/cmd/cmdutil"
)

func init() {
	// Node address completions for commands that take --node with node addresses
	navigateCmd.RegisterFlagCompletionFunc("node", cmdutil.CompleteNodeAddresses(app))
	specCreateCmd.RegisterFlagCompletionFunc("node", cmdutil.CompleteNodeAddresses(app))
	specLinkCmd.RegisterFlagCompletionFunc("node", cmdutil.CompleteNodeAddresses(app))
	specListCmd.RegisterFlagCompletionFunc("node", cmdutil.CompleteNodeAddresses(app))

	// Task address completions for commands that operate on tasks
	unblockCmd.RegisterFlagCompletionFunc("node", cmdutil.CompleteTaskAddresses(app))

	archiveAddCmd.RegisterFlagCompletionFunc("node", cmdutil.CompleteNodeAddresses(app))
}
