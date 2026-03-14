package cmd

import "github.com/spf13/cobra"

var auditCmd = &cobra.Command{
	Use:   "audit",
	Short: "Audit commands — codebase audit, review workflow, trail management",
	Long: `Commands for codebase auditing, staged review, and audit trail management.

Run a codebase audit to generate findings, then review them at your own pace:
  wolfcastle audit run --scope security    # generate findings
  wolfcastle audit pending                 # see what needs review
  wolfcastle audit approve finding-1       # create project from finding
  wolfcastle audit reject finding-2        # dismiss a finding
  wolfcastle audit history                 # see past decisions

Breadcrumbs and escalations manage the per-node audit trail:
  wolfcastle audit breadcrumb --node my-project "refactored auth module"
  wolfcastle audit escalate --node my-project/login "missing error handling spec"`,
}

func init() {
	rootCmd.AddCommand(auditCmd)
}
