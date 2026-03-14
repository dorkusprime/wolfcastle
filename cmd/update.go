package cmd

import (
	"fmt"

	"github.com/dorkusprime/wolfcastle/internal/output"
	"github.com/dorkusprime/wolfcastle/internal/project"
	"github.com/spf13/cobra"
)

var updateCmd = &cobra.Command{
	Use:   "update",
	Short: "Update Wolfcastle binary and regenerate base/",
	Long: `Regenerates the base/ directory from the installed Wolfcastle version.
Does not touch custom/, local/, or any state files.

Run this after upgrading the Wolfcastle binary to pick up new base prompts
and rules.

Examples:
  wolfcastle update`,
	RunE: func(cmd *cobra.Command, args []string) error {
		// TODO: self-update binary from release channel

		// Regenerate base/ prompts and rules
		if err := project.WriteBasePrompts(app.WolfcastleDir); err != nil {
			return fmt.Errorf("regenerating base prompts: %w", err)
		}

		if app.JSONOutput {
			output.Print(output.Ok("update", map[string]string{
				"path": app.WolfcastleDir,
			}))
		} else {
			output.PrintHuman("Regenerated base/ prompts and rules in %s", app.WolfcastleDir)
		}
		return nil
	},
}

func init() {
	rootCmd.AddCommand(updateCmd)
}
