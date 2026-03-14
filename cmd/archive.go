package cmd

import "github.com/spf13/cobra"

// archiveCmd is the parent command for archive management.
var archiveCmd = &cobra.Command{
	Use:   "archive",
	Short: "Manage the archive of completed work",
	Long: `Generate and manage archive entries for completed project nodes.

Archive entries capture the final state, audit trail, and results for
completed work, providing a persistent record of what was done.

Examples:
  wolfcastle archive add --node my-project`,
}

func init() {
	rootCmd.AddCommand(archiveCmd)
}
