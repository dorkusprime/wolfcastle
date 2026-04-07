// Package cmd implements the Wolfcastle CLI surface. It wires together
// all top-level commands (init, version, update, navigate, doctor, unblock,
// install, spec, adr, archive) and delegates subcommand registration to
// the audit, daemon, inbox, project, and task subpackages.
package cmd

import (
	"fmt"
	"os"
	"path/filepath"

	tea "charm.land/bubbletea/v2"

	"github.com/dorkusprime/wolfcastle/cmd/audit"
	"github.com/dorkusprime/wolfcastle/cmd/cmdutil"
	wcconfig "github.com/dorkusprime/wolfcastle/cmd/config"
	"github.com/dorkusprime/wolfcastle/cmd/daemon"
	"github.com/dorkusprime/wolfcastle/cmd/inbox"
	"github.com/dorkusprime/wolfcastle/cmd/knowledge"
	"github.com/dorkusprime/wolfcastle/cmd/orchestrator"
	"github.com/dorkusprime/wolfcastle/cmd/project"
	"github.com/dorkusprime/wolfcastle/cmd/task"
	"github.com/dorkusprime/wolfcastle/internal/clock"
	wcDaemon "github.com/dorkusprime/wolfcastle/internal/daemon"
	wcInstance "github.com/dorkusprime/wolfcastle/internal/instance"
	"github.com/dorkusprime/wolfcastle/internal/invoke"
	"github.com/dorkusprime/wolfcastle/internal/output"
	"github.com/dorkusprime/wolfcastle/internal/state"
	tuiApp "github.com/dorkusprime/wolfcastle/internal/tui/app"
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
	SilenceUsage:  true,
	RunE: func(cmd *cobra.Command, args []string) error {
		return launchTUI()
	},
	PersistentPreRunE: func(cmd *cobra.Command, args []string) error {
		// Skip config loading for commands that don't need it.
		// The TUI (root command with no subcommand) handles its own
		// directory detection, including the welcome screen for missing
		// .wolfcastle directories.
		switch cmd.Name() {
		case "init", "version", "help", "wolfcastle":
			return nil
		}
		return app.Init()
	},
}

func init() {
	rootCmd.PersistentFlags().BoolVar(&app.JSON, "json", false, "Output in JSON format")
}

// launchTUI starts the Bubbletea TUI program. Resolution order:
//
//  1. Walk CWD upward for .wolfcastle/ (local project)
//  2. instance.Resolve(cwd) (running daemon owns this directory)
//  3. instance.List() (any running daemons anywhere)
//  4. Welcome screen (nothing found)
//
// Steps 2 and 3 let the TUI manage running instances from any directory.
func launchTUI() error {
	var (
		store       *state.Store
		daemonRepo  *wcDaemon.DaemonRepository
		worktreeDir string
	)

	cwd, err := os.Getwd()
	if err != nil {
		return fmt.Errorf("getwd: %w", err)
	}

	// Step 1: Walk CWD upward looking for .wolfcastle/
	dir := cwd
	for {
		candidate := filepath.Join(dir, ".wolfcastle")
		if info, statErr := os.Stat(candidate); statErr == nil && info.IsDir() {
			worktreeDir = dir
			break
		}
		parent := filepath.Dir(dir)
		if parent == dir {
			break
		}
		dir = parent
	}

	// Step 2: If no local .wolfcastle, try instance resolution for CWD.
	if worktreeDir == "" {
		if entry, resolveErr := wcInstance.Resolve(cwd); resolveErr == nil {
			worktreeDir = entry.Worktree
		}
	}

	// Step 3: If still nothing, check for any running instance anywhere.
	if worktreeDir == "" {
		if instances, listErr := wcInstance.List(); listErr == nil && len(instances) > 0 {
			worktreeDir = instances[0].Worktree
		}
	}

	// Initialize from the resolved worktree.
	if worktreeDir != "" {
		wolfcastleDir := filepath.Join(worktreeDir, ".wolfcastle")
		if info, statErr := os.Stat(wolfcastleDir); statErr == nil && info.IsDir() {
			daemonRepo = wcDaemon.NewDaemonRepository(wolfcastleDir)

			// Try full app init for Store access. Failure is non-fatal;
			// the TUI runs in cold-start mode without node data.
			if initErr := app.Init(); initErr == nil && app.State != nil {
				store = app.State
			}
		}
	}

	// Step 4: No worktree found at all; welcome screen uses CWD.
	if worktreeDir == "" {
		worktreeDir = cwd
	}

	model := tuiApp.NewTUIModel(store, daemonRepo, worktreeDir, Version)
	p := tea.NewProgram(model)
	_, err = p.Run()
	return err
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
	describeCmd.GroupID = "work"
	archiveCmd.GroupID = "work"
	executeCmd.GroupID = "lifecycle"
	intakeCmd.GroupID = "lifecycle"
	doctorCmd.GroupID = "diagnostics"
	unblockCmd.GroupID = "diagnostics"
	installCmd.GroupID = "integration"
	specCmd.GroupID = "docs"
	adrCmd.GroupID = "docs"

	audit.Register(app, rootCmd)
	wcconfig.Register(app, rootCmd)
	daemon.Register(app, rootCmd)
	inbox.Register(app, rootCmd)
	knowledge.Register(app, rootCmd)
	orchestrator.Register(app, rootCmd)
	project.Register(app, rootCmd)
	task.Register(app, rootCmd)
}

// executeRoot runs the root command and returns any error along with
// whether JSON mode was active, allowing the caller to format the
// error appropriately. Separated from Execute for testability.
func executeRoot() error {
	setupCommands()
	if err := rootCmd.Execute(); err != nil {
		if app.JSON {
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
