package cmd

import (
	"github.com/dorkusprime/wolfcastle/internal/output"
	"github.com/spf13/cobra"
)

// Build-time variables injected via ldflags.
var (
	Version = "dev"
	Commit  = "unknown"
	Date    = "unknown"
)

var versionCmd = &cobra.Command{
	Use:   "version",
	Short: "Print the Wolfcastle version",
	Run: func(cmd *cobra.Command, args []string) {
		if app.JSONOutput {
			output.Print(output.Ok("version", map[string]string{
				"version": Version,
				"commit":  Commit,
				"date":    Date,
			}))
		} else {
			output.PrintHuman("wolfcastle %s (%s, %s)", Version, Commit, Date)
		}
	},
}

func init() {
	rootCmd.AddCommand(versionCmd)
}
