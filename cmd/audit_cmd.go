package cmd

import "github.com/spf13/cobra"

var auditCmd = &cobra.Command{
	Use:   "audit",
	Short: "Audit commands — breadcrumb, escalate, and codebase audit",
}

func init() {
	rootCmd.AddCommand(auditCmd)
}
