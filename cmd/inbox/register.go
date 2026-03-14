package inbox

import (
	"github.com/dorkusprime/wolfcastle/cmd/cmdutil"
	"github.com/spf13/cobra"
)

// Register creates the inbox command tree and attaches it to rootCmd.
func Register(app *cmdutil.App, rootCmd *cobra.Command) {
	inboxCmd := &cobra.Command{
		Use:   "inbox",
		Short: "Manage the inbox for new work items",
		Long: `Capture, list, and clear quick ideas and work items in the inbox.

The inbox is a lightweight staging area for ideas that haven't been
triaged into projects yet.

Examples:
  wolfcastle inbox add "refactor the auth middleware"
  wolfcastle inbox list
  wolfcastle inbox clear
  wolfcastle inbox clear --all`,
	}

	inboxCmd.AddCommand(newAddCmd(app))
	inboxCmd.AddCommand(newListCmd(app))
	inboxCmd.AddCommand(newClearCmd(app))

	rootCmd.AddCommand(inboxCmd)
}
