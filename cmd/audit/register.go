// Package audit implements the audit command group: codebase auditing with
// discoverable scopes, staged review workflow (pending/approve/reject/history),
// and per-node audit trail management (breadcrumb, escalate, gap, fix-gap,
// resolve, scope, show).
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
		Short: "Inspect, review, and track audit trails",
		Long: `Codebase auditing, staged review, and trail management.

Run an audit to generate findings, then decide their fate:
  wolfcastle audit run --scope security    # scan for weaknesses
  wolfcastle audit pending                 # see what awaits judgment
  wolfcastle audit approve finding-1       # promote to project
  wolfcastle audit reject finding-2        # dismiss
  wolfcastle audit history                 # review past decisions

Breadcrumbs and escalations track the per-node audit trail:
  wolfcastle audit breadcrumb --node my-project "refactored auth module"
  wolfcastle audit escalate --node my-project/login "missing error handling spec"`,
	}

	completeNode := cmdutil.CompleteNodeAddresses(app)

	showCmd := newShowCmd(app)
	_ = showCmd.RegisterFlagCompletionFunc("node", completeNode)
	auditCmd.AddCommand(showCmd)

	gapCmd := newGapCmd(app)
	_ = gapCmd.RegisterFlagCompletionFunc("node", completeNode)
	auditCmd.AddCommand(gapCmd)

	fixGapCmd := newFixGapCmd(app)
	_ = fixGapCmd.RegisterFlagCompletionFunc("node", completeNode)
	auditCmd.AddCommand(fixGapCmd)

	breadcrumbCmd := newBreadcrumbCmd(app)
	_ = breadcrumbCmd.RegisterFlagCompletionFunc("node", completeNode)
	auditCmd.AddCommand(breadcrumbCmd)

	escalateCmd := newEscalateCmd(app)
	_ = escalateCmd.RegisterFlagCompletionFunc("node", completeNode)
	auditCmd.AddCommand(escalateCmd)

	resolveCmd := newResolveCmd(app)
	_ = resolveCmd.RegisterFlagCompletionFunc("node", completeNode)
	auditCmd.AddCommand(resolveCmd)

	scopeCmd := newScopeCmd(app)
	_ = scopeCmd.RegisterFlagCompletionFunc("node", completeNode)
	auditCmd.AddCommand(scopeCmd)

	summaryCmd := newSummaryCmd(app)
	_ = summaryCmd.RegisterFlagCompletionFunc("node", completeNode)
	auditCmd.AddCommand(summaryCmd)

	enrichCmd := newEnrichCmd(app)
	_ = enrichCmd.RegisterFlagCompletionFunc("node", completeNode)
	auditCmd.AddCommand(enrichCmd)

	approveCmd := newApproveCmd(app)
	_ = approveCmd.RegisterFlagCompletionFunc("node", completeNode)
	auditCmd.AddCommand(approveCmd)

	rejectCmd := newRejectCmd(app)
	_ = rejectCmd.RegisterFlagCompletionFunc("node", completeNode)
	auditCmd.AddCommand(rejectCmd)

	auditCmd.AddCommand(newPendingCmd(app))
	auditCmd.AddCommand(newHistoryCmd(app))

	runCmd, listCmd := newCodebaseCmd(app)
	auditCmd.AddCommand(runCmd)
	auditCmd.AddCommand(listCmd)

	auditCmd.GroupID = "audit"
	rootCmd.AddCommand(auditCmd)
}
