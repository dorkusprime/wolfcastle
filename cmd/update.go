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
	Short: "Upgrade the binary and refresh system/base/",
	Long: `Checks for a newer version and regenerates the system/base/ directory.
system/custom/, system/local/, and state files are not touched.

Examples:
  wolfcastle update`,
	RunE: func(cmd *cobra.Command, args []string) error {
		// Check for binary update
		updater := selfupdate.NewUpdater(Version)
		result, err := updater.Apply()
		if !app.JSONOutput {
			if err != nil {
				output.PrintHuman("Update check failed: %v. Regenerating system/base/ anyway.", err)
			} else if result.Updated {
				output.PrintHuman("Upgraded: %s -> %s", result.CurrentVersion, result.LatestVersion)
			} else if result.AlreadyCurrent {
				output.PrintHuman("Already running %s. No upgrade needed.", result.CurrentVersion)
			}
		}

		// Regenerate base tier (prompts, rules, configs, identity)
		root := app.Config.Root()
		svc := project.NewScaffoldService(app.Config, app.Prompts, nil, root)
		if err := svc.Reinit(); err != nil {
			return fmt.Errorf("regenerating system/base/: %w", err)
		}

		if app.JSONOutput {
			updateStatus := "unchanged"
			if err != nil {
				updateStatus = "check_failed"
			} else if result.Updated {
				updateStatus = "updated"
			}
			output.Print(output.Ok("update", map[string]string{
				"path":          root,
				"version":       Version,
				"update_status": updateStatus,
			}))
		} else {
			output.PrintHuman("system/base/ regenerated in %s", root)
		}
		return nil
	},
}

func init() {
	rootCmd.AddCommand(updateCmd)
}
