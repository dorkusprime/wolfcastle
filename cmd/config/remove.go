package config

import (
	"encoding/json"
	"fmt"

	"github.com/dorkusprime/wolfcastle/cmd/cmdutil"
	"github.com/dorkusprime/wolfcastle/internal/config"
	"github.com/dorkusprime/wolfcastle/internal/output"
	"github.com/spf13/cobra"
)

func newRemoveCmd(app *cmdutil.App) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "remove <key> <value>",
		Short: "Remove a value from a configuration array",
		Long: `Remove the first matching value from an array in a tier overlay
using dot-notation paths.

The value is parsed as JSON first (handling numbers, booleans, null,
objects, and arrays). If JSON parsing fails, the value is treated as
a plain string. Comparison uses JSON equality (both values are marshaled
to JSON and the resulting strings are compared).

Examples:
  wolfcastle config remove identity.tags foo
  wolfcastle config remove identity.tags 42
  wolfcastle config remove pipeline.steps '{"name":"lint"}'
  wolfcastle config remove identity.tags bar --tier custom`,
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
				existing, found := config.GetPath(overlay, key)
				if !found || existing == nil {
					return fmt.Errorf("%s is not an array", key)
				}

				arr, ok := existing.([]any)
				if !ok {
					return fmt.Errorf("%s is not an array", key)
				}

				targetJSON, err := json.Marshal(value)
				if err != nil {
					return fmt.Errorf("marshaling target value: %w", err)
				}

				idx := -1
				for i, elem := range arr {
					elemJSON, err := json.Marshal(elem)
					if err != nil {
						continue
					}
					if string(elemJSON) == string(targetJSON) {
						idx = i
						break
					}
				}

				if idx < 0 {
					return fmt.Errorf("%s not found in %s", rawValue, key)
				}

				result := append(arr[:idx], arr[idx+1:]...)
				return config.SetPath(overlay, key, result)
			}); err != nil {
				return err
			}

			if app.JSON {
				output.Print(output.Ok("config_remove", map[string]any{
					"key":   key,
					"value": value,
					"tier":  tier,
				}))
				return nil
			}

			output.PrintHuman("Removed %s from %s in %s/config.json", rawValue, key, tier)
			return nil
		},
	}

	cmd.Flags().String("tier", "local", "Target tier (local or custom)")
	return cmd
}
