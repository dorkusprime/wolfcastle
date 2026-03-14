package cmd

import (
	"github.com/dorkusprime/wolfcastle/internal/output"
	"github.com/spf13/cobra"
)

// Version is set at build time via ldflags.
var Version = "dev"

var versionCmd = &cobra.Command{
	Use:   "version",
	Short: "Print the Wolfcastle version",
	Run: func(cmd *cobra.Command, args []string) {
		if app.JSONOutput {
			output.Print(output.Ok("version", map[string]string{
				"version": Version,
			}))
		} else {
			output.PrintHuman("wolfcastle %s", Version)
		}
	},
}

func init() {
	rootCmd.AddCommand(versionCmd)
}
