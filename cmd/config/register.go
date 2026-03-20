package config

import (
	"github.com/dorkusprime/wolfcastle/cmd/cmdutil"
	"github.com/spf13/cobra"
)

// Register adds the "config" command group and its subcommands to rootCmd.
func Register(app *cmdutil.App, rootCmd *cobra.Command) {
	configCmd := &cobra.Command{
		Use:   "config",
		Short: "Inspect and manage configuration",
		Long: `Read-only access to the Wolfcastle configuration.

The resolved config merges hardcoded defaults with three tier files
(base < custom < local). Use flags to inspect individual tiers or
suppress the defaults layer.

Examples:
  wolfcastle config show                  Show fully resolved config
  wolfcastle config show --tier local     Show local tier overrides only
  wolfcastle config show --raw            Show merged tiers without defaults
  wolfcastle config show --json           Machine-readable envelope output`,
	}

	configCmd.AddCommand(newShowCmd(app))

	configCmd.GroupID = "diagnostics"
	rootCmd.AddCommand(configCmd)
}
