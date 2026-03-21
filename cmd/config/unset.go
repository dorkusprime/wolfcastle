package config

import (
	"fmt"

	"github.com/dorkusprime/wolfcastle/cmd/cmdutil"
	"github.com/dorkusprime/wolfcastle/internal/config"
	"github.com/dorkusprime/wolfcastle/internal/output"
	"github.com/spf13/cobra"
)

func newUnsetCmd(app *cmdutil.App) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "unset <key>",
		Short: "Remove a configuration value",
		Long: `Remove a key from a tier overlay using dot-notation paths.

The key and any nested structure beneath it are deleted from the
target tier's config.json. If the key does not exist, the command
succeeds silently.

Examples:
  wolfcastle config unset logs.level
  wolfcastle config unset pipeline.timeout --tier custom`,
		Args: cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			key := args[0]
			tier, _ := cmd.Flags().GetString("tier")

			if tier != "local" && tier != "custom" {
				return fmt.Errorf("--tier must be \"local\" or \"custom\"")
			}

			if err := app.Config.ApplyMutation(tier, func(overlay map[string]any) error {
				return config.DeletePath(overlay, key)
			}); err != nil {
				return err
			}

			if app.JSON {
				output.Print(output.Ok("config_unset", map[string]any{
					"key":  key,
					"tier": tier,
				}))
				return nil
			}

			output.PrintHuman("Unset %s from %s/config.json", key, tier)
			return nil
		},
	}

	cmd.Flags().String("tier", "local", "Target tier (local or custom)")
	return cmd
}
