package cmd

import "github.com/spf13/cobra"

var inboxCmd = &cobra.Command{
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

func init() {
	rootCmd.AddCommand(inboxCmd)
}
