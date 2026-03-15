package cmd

import (
	"fmt"

	"github.com/dorkusprime/wolfcastle/internal/output"
	"github.com/dorkusprime/wolfcastle/internal/project"
	"github.com/dorkusprime/wolfcastle/internal/selfupdate"
	"github.com/spf13/cobra"
)

// updateCmd checks for binary updates and regenerates base/ prompts.
var updateCmd = &cobra.Command{
	Use:   "update",
	Short: "Upgrade the binary and refresh base/",
	Long: `Checks for a newer version and regenerates the base/ directory.
custom/, local/, and state files are not touched.

Examples:
  wolfcastle update`,
	RunE: func(cmd *cobra.Command, args []string) error {
		// Check for binary update
		updater := selfupdate.NewUpdater(Version)
		result, err := updater.Apply()
		if err != nil {
			output.PrintHuman("Update check failed: %v. Regenerating base/ anyway.", err)
		} else if result.Updated {
			output.PrintHuman("Upgraded: %s -> %s", result.CurrentVersion, result.LatestVersion)
		} else if result.AlreadyCurrent {
			output.PrintHuman("Already running %s. No upgrade needed.", result.CurrentVersion)
		}

		// Regenerate base/ prompts and rules
		if err := project.WriteBasePrompts(app.WolfcastleDir); err != nil {
			return fmt.Errorf("regenerating base prompts: %w", err)
		}

		if app.JSONOutput {
			output.Print(output.Ok("update", map[string]string{
				"path":    app.WolfcastleDir,
				"version": Version,
			}))
		} else {
			output.PrintHuman("base/ regenerated in %s", app.WolfcastleDir)
		}
		return nil
	},
}

func init() {
	rootCmd.AddCommand(updateCmd)
}
