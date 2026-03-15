// Package inbox implements the inbox command group: add, list, and clear.
// The inbox is a lightweight staging area for ideas that flow through
// the expand/file pipeline before becoming projects in the work tree.
package inbox

import (
	"github.com/dorkusprime/wolfcastle/cmd/cmdutil"
	"github.com/spf13/cobra"
)

// Register creates the inbox command tree and attaches it to rootCmd.
func Register(app *cmdutil.App, rootCmd *cobra.Command) {
	inboxCmd := &cobra.Command{
		Use:   "inbox",
		Short: "Capture and triage incoming work",
		Long: `The inbox holds raw ideas before they become targets. Throw things in,
review them later, clear the wreckage when you're done.

Examples:
  wolfcastle inbox add "refactor the auth middleware"
  wolfcastle inbox list
  wolfcastle inbox clear
  wolfcastle inbox clear --all`,
	}

	inboxCmd.AddCommand(newAddCmd(app))
	inboxCmd.AddCommand(newListCmd(app))
	inboxCmd.AddCommand(newClearCmd(app))

	inboxCmd.GroupID = "work"
	rootCmd.AddCommand(inboxCmd)
}
