package cmd

import "github.com/spf13/cobra"

var projectCmd = &cobra.Command{
	Use:   "project",
	Short: "Manage projects in the work tree",
}

func init() {
	rootCmd.AddCommand(projectCmd)
}
