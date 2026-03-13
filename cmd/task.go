package cmd

import "github.com/spf13/cobra"

var taskCmd = &cobra.Command{
	Use:   "task",
	Short: "Manage tasks within leaf nodes",
}

func init() {
	rootCmd.AddCommand(taskCmd)
}
