package config

import (
	"fmt"

	"github.com/dorkusprime/wolfcastle/cmd/cmdutil"
	"github.com/dorkusprime/wolfcastle/internal/config"
	"github.com/dorkusprime/wolfcastle/internal/output"
	"github.com/spf13/cobra"
)

func newAppendCmd(app *cmdutil.App) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "append <key> <value>",
		Short: "Append a value to a configuration array",
		Long: `Append a value to an array in a tier overlay using dot-notation paths.

The value is parsed as JSON first (handling numbers, booleans, null,
objects, and arrays). If JSON parsing fails, the value is stored as
a plain string.

If the key does not exist, a new single-element array is created.
If the key exists but is not an array, the command returns an error.

Examples:
  wolfcastle config append identity.tags foo
  wolfcastle config append identity.tags '"quoted"'
  wolfcastle config append pipeline.steps '{"name":"lint"}'
  wolfcastle config append identity.tags bar --tier custom`,
		Args: cobra.ExactArgs(2),
		RunE: func(cmd *cobra.Command, args []string) error {
			key := args[0]
			rawValue := args[1]
			tier, _ := cmd.Flags().GetString("tier")

			if tier != "local" && tier != "custom" {
				return fmt.Errorf("--tier must be \"local\" or \"custom\"")
			}

			value := parseValue(rawValue)

			if err := app.Config.ApplyMutation(tier, func(overlay map[string]any) error {
				existing, found := config.GetPath(overlay, key)
				switch {
				case !found || existing == nil:
					return config.SetPath(overlay, key, []any{value})
				default:
					arr, ok := existing.([]any)
					if !ok {
						return fmt.Errorf("%s is not an array", key)
					}
					return config.SetPath(overlay, key, append(arr, value))
				}
			}); err != nil {
				return err
			}

			if app.JSON {
				output.Print(output.Ok("config_append", map[string]any{
					"key":   key,
					"value": value,
					"tier":  tier,
				}))
				return nil
			}

			output.PrintHuman("Appended %s to %s in %s/config.json", rawValue, key, tier)
			return nil
		},
	}

	cmd.Flags().String("tier", "local", "Target tier (local or custom)")
	return cmd
}
