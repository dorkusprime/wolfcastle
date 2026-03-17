// Package orchestrator implements the orchestrator command group: planning
// and coordination commands for orchestrator nodes in the decomposition tree.
package orchestrator

import (
	"github.com/dorkusprime/wolfcastle/cmd/cmdutil"
	"github.com/spf13/cobra"
)

// Register creates the "orchestrator" parent command, wires up its
// subcommands, and attaches the whole subtree to rootCmd.
func Register(app *cmdutil.App, rootCmd *cobra.Command) {
	orchCmd := &cobra.Command{
		Use:   "orchestrator",
		Short: "Plan and coordinate orchestrator nodes",
		Long: `Commands for orchestrator-level planning: success criteria,
enrichment, and coordination across child nodes.

Examples:
  wolfcastle orchestrator criteria --node my-project "all tests pass"
  wolfcastle orchestrator criteria --node my-project --list`,
	}

	completeNode := cmdutil.CompleteNodeAddresses(app)

	criteriaCmd := newCriteriaCmd(app)
	_ = criteriaCmd.RegisterFlagCompletionFunc("node", completeNode)
	orchCmd.AddCommand(criteriaCmd)

	orchCmd.GroupID = "work"
	rootCmd.AddCommand(orchCmd)
}
