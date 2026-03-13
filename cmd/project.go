package cmd

import "github.com/spf13/cobra"

var projectCmd = &cobra.Command{
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

func init() {
	rootCmd.AddCommand(projectCmd)
}
