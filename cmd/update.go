package cmd

import (
	"github.com/dorkusprime/wolfcastle/internal/output"
	"github.com/dorkusprime/wolfcastle/internal/project"
	"github.com/spf13/cobra"
)

var updateCmd = &cobra.Command{
	Use:   "update",
	Short: "Update Wolfcastle binary and regenerate base/",
	Long:  "Regenerates the base/ directory from the installed Wolfcastle version. Does not touch custom/, local/, or any state files.",
	RunE: func(cmd *cobra.Command, args []string) error {
		// TODO: self-update binary from release channel

		// Regenerate base/ prompts and rules
		project.WriteBasePrompts(wolfcastleDir)

		if jsonOutput {
			output.Print(output.Ok("update", map[string]string{
				"path": wolfcastleDir,
			}))
		} else {
			output.PrintHuman("Regenerated base/ prompts and rules in %s", wolfcastleDir)
		}
		return nil
	},
}

func init() {
	rootCmd.AddCommand(updateCmd)
}
