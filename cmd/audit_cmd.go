package cmd

import "github.com/spf13/cobra"

var auditCmd = &cobra.Command{
	Use:   "audit",
	Short: "Audit commands — breadcrumb, escalate, and codebase audit",
	Long: `Commands for the audit trail and codebase auditing.

Breadcrumbs record progress notes on nodes. Escalations flag gaps to parent
orchestrators. The 'run' subcommand performs model-driven codebase audits.

Examples:
  wolfcastle audit breadcrumb --node my-project "refactored auth module"
  wolfcastle audit escalate --node my-project/login "missing error handling spec"
  wolfcastle audit run --scope security
  wolfcastle audit list`,
}

func init() {
	rootCmd.AddCommand(auditCmd)
}
