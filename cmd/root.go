// Package cmd implements the Wolfcastle CLI surface. It wires together
// all top-level commands (init, version, update, navigate, doctor, unblock,
// install, spec, adr, archive) and delegates subcommand registration to
// the audit, daemon, inbox, project, and task subpackages.
package cmd

import (
	"os"

	"github.com/dorkusprime/wolfcastle/cmd/audit"
	"github.com/dorkusprime/wolfcastle/cmd/cmdutil"
	"github.com/dorkusprime/wolfcastle/cmd/daemon"
	"github.com/dorkusprime/wolfcastle/cmd/inbox"
	"github.com/dorkusprime/wolfcastle/cmd/orchestrator"
	"github.com/dorkusprime/wolfcastle/cmd/project"
	"github.com/dorkusprime/wolfcastle/cmd/task"
	"github.com/dorkusprime/wolfcastle/internal/clock"
	"github.com/dorkusprime/wolfcastle/internal/invoke"
	"github.com/dorkusprime/wolfcastle/internal/output"
	"github.com/spf13/cobra"
)

// app is the shared runtime state for the CLI.
var app = &cmdutil.App{Clock: clock.New(), Invoker: invoke.NewProcessInvoker(), Version: Version}

var rootCmd = &cobra.Command{
	Use:     "wolfcastle",
	Version: Version + " (" + Commit + ", " + Date + ")",
	Short:   "Wolfcastle crushes your project backlog so you don't have to",
	Long: `You give Wolfcastle a goal. It breaks that goal into pieces. Then it
breaks those pieces. Then it does the work while you go do whatever it
is you do when you're not supervising software.

PRE-RELEASE: The CLI surface may change before v1.0.

Quick start:
  wolfcastle init                          Claim a directory
  wolfcastle project create "my-feature"   Name your target
  wolfcastle task add --node my-feature "implement API"
  wolfcastle start                         Start the daemon

Use "wolfcastle [command] --help" for more information about a command.
All commands support --json for machine-readable output.`,
	SilenceErrors: true,
	PersistentPreRunE: func(cmd *cobra.Command, args []string) error {
		// Skip config loading for commands that don't need it
		switch cmd.Name() {
		case "init", "version", "help":
			return nil
		}
		return app.Init()
	},
}

func init() {
	rootCmd.PersistentFlags().BoolVar(&app.JSONOutput, "json", false, "Output in JSON format")
}

// setupCommands registers all subcommand groups and wires up
// command-group associations. It is separated from Execute so that
// tests can exercise the registration logic without triggering os.Exit.
func setupCommands() {
	// Command groups for organized help output (ADR-030)
	rootCmd.AddGroup(
		&cobra.Group{ID: "lifecycle", Title: "Lifecycle:"},
		&cobra.Group{ID: "work", Title: "Work Management:"},
		&cobra.Group{ID: "audit", Title: "Auditing:"},
		&cobra.Group{ID: "docs", Title: "Documentation:"},
		&cobra.Group{ID: "diagnostics", Title: "Diagnostics:"},
		&cobra.Group{ID: "integration", Title: "Integration:"},
	)

	// Assign groups to top-level commands (direct children of rootCmd).
	// Subcommands inherit their parent's group; setting GroupID on a
	// child of a non-root parent causes a Cobra panic.
	initCmd.GroupID = "lifecycle"
	versionCmd.GroupID = "lifecycle"
	updateCmd.GroupID = "lifecycle"
	navigateCmd.GroupID = "work"
	archiveCmd.GroupID = "work"
	doctorCmd.GroupID = "diagnostics"
	unblockCmd.GroupID = "diagnostics"
	installCmd.GroupID = "integration"
	specCmd.GroupID = "docs"
	adrCmd.GroupID = "docs"

	audit.Register(app, rootCmd)
	daemon.Register(app, rootCmd)
	inbox.Register(app, rootCmd)
	orchestrator.Register(app, rootCmd)
	project.Register(app, rootCmd)
	task.Register(app, rootCmd)
}

// executeRoot runs the root command and returns any error along with
// whether JSON mode was active — allowing the caller to format the
// error appropriately. Separated from Execute for testability.
func executeRoot() error {
	setupCommands()
	if err := rootCmd.Execute(); err != nil {
		if app.JSONOutput {
			output.Print(output.Err("error", 1, err.Error()))
		} else {
			output.PrintError("%s", err)
		}
		return err
	}
	return nil
}

// Execute registers all subcommand groups and runs the root command.
// It handles top-level error formatting (JSON or human-readable) and
// exits with code 1 on any command failure.
func Execute() {
	if err := executeRoot(); err != nil {
		os.Exit(1)
	}
}
