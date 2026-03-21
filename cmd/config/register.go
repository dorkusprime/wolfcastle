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
		Long: `Inspect and manage the Wolfcastle configuration.

The resolved config merges hardcoded defaults with three tier files
(base < custom < local). Use subcommands to inspect tiers or modify
the custom and local overlays.

Examples:
  wolfcastle config show                  Show fully resolved config
  wolfcastle config show --tier local     Show local tier overrides only
  wolfcastle config set logs.level debug  Set a value in the local tier
  wolfcastle config show --json           Machine-readable envelope output`,
	}

	configCmd.AddCommand(newShowCmd(app))
	configCmd.AddCommand(newSetCmd(app))
	configCmd.AddCommand(newUnsetCmd(app))
	configCmd.AddCommand(newAppendCmd(app))

	configCmd.GroupID = "diagnostics"
	rootCmd.AddCommand(configCmd)
}
