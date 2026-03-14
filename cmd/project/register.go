package project

import (
	"github.com/dorkusprime/wolfcastle/cmd/cmdutil"
	"github.com/spf13/cobra"
)

// Register creates the "project" parent command, wires up its
// subcommands, and attaches the whole subtree to rootCmd.
func Register(app *cmdutil.App, rootCmd *cobra.Command) {
	projectCmd := &cobra.Command{
		Use:   "project",
		Short: "Manage projects in the work tree",
		Long: `Create and manage project nodes in the Wolfcastle work tree.

Projects can be root-level or nested under a parent orchestrator node.
Each project has a type (leaf or orchestrator) and tracks its own state.

Examples:
  wolfcastle project create "auth-system"
  wolfcastle project create --node auth-system --type orchestrator "oauth"
  wolfcastle project create --node auth-system/oauth "token-refresh"`,
	}

	createCmd := newCreateCmd(app)
	createCmd.RegisterFlagCompletionFunc("node", cmdutil.CompleteNodeAddresses(app))
	projectCmd.AddCommand(createCmd)

	projectCmd.GroupID = "work"
	rootCmd.AddCommand(projectCmd)
}
