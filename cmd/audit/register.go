package audit

import (
	"github.com/dorkusprime/wolfcastle/cmd/cmdutil"
	"github.com/spf13/cobra"
)

// Register creates the "audit" parent command, wires up its
// subcommands, and attaches the whole subtree to rootCmd.
func Register(app *cmdutil.App, rootCmd *cobra.Command) {
	auditCmd := &cobra.Command{
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

	completeNode := cmdutil.CompleteNodeAddresses(app)

	showCmd := newShowCmd(app)
	showCmd.RegisterFlagCompletionFunc("node", completeNode)
	auditCmd.AddCommand(showCmd)

	gapCmd := newGapCmd(app)
	gapCmd.RegisterFlagCompletionFunc("node", completeNode)
	auditCmd.AddCommand(gapCmd)

	fixGapCmd := newFixGapCmd(app)
	fixGapCmd.RegisterFlagCompletionFunc("node", completeNode)
	auditCmd.AddCommand(fixGapCmd)

	breadcrumbCmd := newBreadcrumbCmd(app)
	breadcrumbCmd.RegisterFlagCompletionFunc("node", completeNode)
	auditCmd.AddCommand(breadcrumbCmd)

	escalateCmd := newEscalateCmd(app)
	escalateCmd.RegisterFlagCompletionFunc("node", completeNode)
	auditCmd.AddCommand(escalateCmd)

	resolveCmd := newResolveCmd(app)
	resolveCmd.RegisterFlagCompletionFunc("node", completeNode)
	auditCmd.AddCommand(resolveCmd)

	scopeCmd := newScopeCmd(app)
	scopeCmd.RegisterFlagCompletionFunc("node", completeNode)
	auditCmd.AddCommand(scopeCmd)

	approveCmd := newApproveCmd(app)
	approveCmd.RegisterFlagCompletionFunc("node", completeNode)
	auditCmd.AddCommand(approveCmd)

	rejectCmd := newRejectCmd(app)
	rejectCmd.RegisterFlagCompletionFunc("node", completeNode)
	auditCmd.AddCommand(rejectCmd)

	auditCmd.AddCommand(newPendingCmd(app))
	auditCmd.AddCommand(newHistoryCmd(app))

	runCmd, listCmd := newCodebaseCmd(app)
	auditCmd.AddCommand(runCmd)
	auditCmd.AddCommand(listCmd)

	auditCmd.GroupID = "audit"
	rootCmd.AddCommand(auditCmd)
}
