package cmd

import "github.com/spf13/cobra"

var inboxCmd = &cobra.Command{
	Use:   "inbox",
	Short: "Manage the inbox for new work items",
}

func init() {
	rootCmd.AddCommand(inboxCmd)
}
