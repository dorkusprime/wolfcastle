package cmd

import "github.com/spf13/cobra"

var archiveCmd = &cobra.Command{
	Use:   "archive",
	Short: "Manage the archive of completed work",
}

func init() {
	rootCmd.AddCommand(archiveCmd)
}
