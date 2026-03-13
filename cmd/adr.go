package cmd

import "github.com/spf13/cobra"

var adrCmd = &cobra.Command{
	Use:   "adr",
	Short: "Manage architecture decision records",
}

func init() {
	rootCmd.AddCommand(adrCmd)
}
