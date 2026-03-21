package config

import (
	"bytes"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/dorkusprime/wolfcastle/cmd/cmdutil"
	"github.com/dorkusprime/wolfcastle/internal/config"
	"github.com/dorkusprime/wolfcastle/internal/output"
	"github.com/dorkusprime/wolfcastle/internal/tierfs"
	"github.com/spf13/cobra"
)

func newShowCmd(app *cmdutil.App) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "show [section]",
		Short: "Display the resolved configuration",
		Long: `Prints the Wolfcastle configuration as indented JSON.

By default the output reflects the fully resolved config: hardcoded
defaults merged with base, custom, and local tiers.

An optional section argument filters the output to a single top-level
key (e.g. "wolfcastle config show pipeline"). If the section does not
exist, the command lists the valid section names.

Two flags modify what is shown:
  --tier   restricts output to a single tier file's raw content
  --raw    suppresses the hardcoded defaults layer

Examples:
  wolfcastle config show
  wolfcastle config show pipeline
  wolfcastle config show --tier local
  wolfcastle config show --raw
  wolfcastle config show logs --json
  wolfcastle config show --tier base --json`,
		RunE: func(cmd *cobra.Command, args []string) error {
			tier, _ := cmd.Flags().GetString("tier")
			raw, _ := cmd.Flags().GetBool("raw")

			if tier != "" && !isValidTier(tier) {
				return fmt.Errorf("--tier must be one of: base, custom, local")
			}

			root := app.Config.Root()

			var result any
			var err error

			switch {
			case tier != "":
				result, err = readTierFile(root, tier)
			case raw:
				result, err = mergeRawTiers(root)
			default:
				result, err = app.Config.Load()
			}
			if err != nil {
				return err
			}

			if len(args) == 1 {
				result, err = extractSection(result, args[0])
				if err != nil {
					return err
				}
			}

			formatted, err := marshalPretty(result)
			if err != nil {
				return fmt.Errorf("formatting config: %w", err)
			}

			if app.JSON {
				output.Print(output.Ok("config_show", result))
				return nil
			}

			output.PrintHuman("%s", formatted)
			return nil
		},
	}

	cmd.Flags().String("tier", "", "Display a single tier (base, custom, local)")
	cmd.Flags().Bool("raw", false, "Suppress hardcoded defaults layer")
	return cmd
}

// isValidTier checks whether name is one of the canonical tier names.
func isValidTier(name string) bool {
	for _, t := range tierfs.TierNames {
		if t == name {
			return true
		}
	}
	return false
}

// readTierFile reads and parses a single tier's config.json. Returns an
// empty map if the file does not exist; returns an error for malformed JSON.
func readTierFile(wolfcastleRoot, tier string) (map[string]any, error) {
	path := filepath.Join(wolfcastleRoot, "system", tier, "config.json")
	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return map[string]any{}, nil
		}
		return nil, fmt.Errorf("reading %s/config.json: %w", tier, err)
	}
	var m map[string]any
	if err := json.Unmarshal(data, &m); err != nil {
		return nil, fmt.Errorf("%s/config.json is not valid JSON: %w", tier, err)
	}
	return m, nil
}

// mergeRawTiers deep-merges the three tier files without seeding from defaults.
func mergeRawTiers(wolfcastleRoot string) (map[string]any, error) {
	result := map[string]any{}
	for _, tier := range tierfs.TierNames {
		m, err := readTierFile(wolfcastleRoot, tier)
		if err != nil {
			return nil, err
		}
		result = config.DeepMerge(result, m)
	}
	return result, nil
}

// extractSection converts result to a map and returns the value under key.
// If the key is missing, it returns an error listing valid section names.
func extractSection(result any, key string) (any, error) {
	m, ok := result.(map[string]any)
	if !ok {
		raw, err := json.Marshal(result)
		if err != nil {
			return nil, fmt.Errorf("converting config to map: %w", err)
		}
		if err := json.Unmarshal(raw, &m); err != nil {
			return nil, fmt.Errorf("converting config to map: %w", err)
		}
	}

	if val, exists := m[key]; exists {
		return val, nil
	}

	names := make([]string, 0, len(m))
	for k := range m {
		names = append(names, k)
	}
	sort.Strings(names)
	return nil, fmt.Errorf("unknown section %q; valid sections: %s", key, strings.Join(names, ", "))
}

// marshalPretty formats v as indented JSON with HTML escaping disabled.
func marshalPretty(v any) (string, error) {
	var buf bytes.Buffer
	enc := json.NewEncoder(&buf)
	enc.SetIndent("", "  ")
	enc.SetEscapeHTML(false)
	if err := enc.Encode(v); err != nil {
		return "", err
	}
	// Encode appends a trailing newline; trim it since PrintHuman adds one.
	return string(bytes.TrimRight(buf.Bytes(), "\n")), nil
}
