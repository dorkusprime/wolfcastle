package config

import (
	"encoding/json"
	"fmt"

	"github.com/dorkusprime/wolfcastle/cmd/cmdutil"
	"github.com/dorkusprime/wolfcastle/internal/config"
	"github.com/dorkusprime/wolfcastle/internal/output"
	"github.com/spf13/cobra"
)

func newSetCmd(app *cmdutil.App) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "set <key> <value>",
		Short: "Set a configuration value",
		Long: `Set a value in a tier overlay using dot-notation paths.

The value is parsed as JSON first (handling numbers, booleans, null,
objects, and arrays). If JSON parsing fails, the value is stored as
a plain string.

Examples:
  wolfcastle config set logs.level debug
  wolfcastle config set pipeline.timeout 30
  wolfcastle config set pipeline.enabled true
  wolfcastle config set identity.tags '["a","b"]'
  wolfcastle config set logs.level warn --tier custom`,
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) < 2 {
				return fmt.Errorf("missing required arguments: <key> <value>")
			}
			key := args[0]
			rawValue := args[1]
			tier, _ := cmd.Flags().GetString("tier")

			if tier != "local" && tier != "custom" {
				return fmt.Errorf("--tier must be \"local\" or \"custom\"")
			}

			value := parseValue(rawValue)

			if err := app.Config.ApplyMutation(tier, func(overlay map[string]any) error {
				return config.SetPath(overlay, key, value)
			}); err != nil {
				return err
			}

			if app.JSON {
				output.Print(output.Ok("config_set", map[string]any{
					"key":   key,
					"value": value,
					"tier":  tier,
				}))
				return nil
			}

			output.PrintHuman("Set %s = %s in %s/config.json", key, rawValue, tier)
			return nil
		},
	}

	cmd.Flags().String("tier", "local", "Target tier (local or custom)")
	return cmd
}

// parseValue attempts JSON parsing first; falls back to a bare string.
func parseValue(raw string) any {
	var v any
	if err := json.Unmarshal([]byte(raw), &v); err == nil {
		return v
	}
	return raw
}
