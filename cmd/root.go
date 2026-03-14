package cmd

import (
	"os"

	"github.com/dorkusprime/wolfcastle/cmd/audit"
	"github.com/dorkusprime/wolfcastle/cmd/cmdutil"
	"github.com/dorkusprime/wolfcastle/cmd/daemon"
	"github.com/dorkusprime/wolfcastle/cmd/inbox"
	"github.com/dorkusprime/wolfcastle/cmd/project"
	"github.com/dorkusprime/wolfcastle/cmd/task"
	"github.com/dorkusprime/wolfcastle/internal/output"
	"github.com/spf13/cobra"
)

// app is the shared runtime state for the CLI.
var app = &cmdutil.App{}

var rootCmd = &cobra.Command{
	Use:   "wolfcastle",
	Short: "Model-agnostic autonomous project orchestrator",
	Long: `Wolfcastle breaks complex work into a persistent tree of projects,
sub-projects, and tasks, then executes them through configurable
multi-model pipelines.

Quick start:
  wolfcastle init                          Initialize a project
  wolfcastle project create "my-feature"   Create a root project
  wolfcastle task add --node my-feature "implement API"
  wolfcastle start                         Run the daemon

Use "wolfcastle [command] --help" for more information about a command.
All commands support --json for machine-readable output.`,
	SilenceErrors: true,
	PersistentPreRunE: func(cmd *cobra.Command, args []string) error {
		// Skip config loading for commands that don't need it
		switch cmd.Name() {
		case "init", "version", "help":
			return nil
		}
		return app.LoadConfig()
	},
}

func init() {
	rootCmd.PersistentFlags().BoolVar(&app.JSONOutput, "json", false, "Output in JSON format")
}

func Execute() {
	audit.Register(app, rootCmd)
	daemon.Register(app, rootCmd)
	inbox.Register(app, rootCmd)
	project.Register(app, rootCmd)
	task.Register(app, rootCmd)

	if err := rootCmd.Execute(); err != nil {
		if app.JSONOutput {
			output.Print(output.Err("error", 1, err.Error()))
		} else {
			output.PrintError("%s", err)
		}
		os.Exit(1)
	}
}
