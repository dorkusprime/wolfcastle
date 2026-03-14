package cmd

import (
	"github.com/dorkusprime/wolfcastle/internal/output"
	"github.com/spf13/cobra"
)

// Version is the semantic version, injected at build time via ldflags.
var Version = "dev"

// Commit is the git commit hash, injected at build time via ldflags.
var Commit = "unknown"

// Date is the build timestamp, injected at build time via ldflags.
var Date = "unknown"

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
