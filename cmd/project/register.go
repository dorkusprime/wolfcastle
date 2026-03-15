// Package project implements the project command group. Currently
// supports project creation with automatic leaf-to-orchestrator
// promotion and overlap advisory.
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
		Short: "Organize targets in the work tree",
		Long: `Projects are the targets Wolfcastle destroys. Root-level or nested,
leaf or orchestrator. Each one tracks its own state and reports
to its parent.

Examples:
  wolfcastle project create "auth-system"
  wolfcastle project create --node auth-system --type orchestrator "oauth"
  wolfcastle project create --node auth-system/oauth "token-refresh"`,
	}

	createCmd := newCreateCmd(app)
	_ = createCmd.RegisterFlagCompletionFunc("node", cmdutil.CompleteNodeAddresses(app))
	projectCmd.AddCommand(createCmd)

	projectCmd.GroupID = "work"
	rootCmd.AddCommand(projectCmd)
}
