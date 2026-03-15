package cmd

import "github.com/spf13/cobra"

// archiveCmd is the parent command for archive management.
var archiveCmd = &cobra.Command{
	Use:   "archive",
	Short: "Preserve the record of conquered targets",
	Long: `Archive entries capture the final state, audit trail, and results of
completed nodes. The permanent record of what fell and how.

Examples:
  wolfcastle archive add --node my-project`,
}

func init() {
	rootCmd.AddCommand(archiveCmd)
}
