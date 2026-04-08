// Package daemon implements the lifecycle commands: start, stop, status,
// and log. These control the daemon loop and provide visibility into
// its execution.
package daemon

import (
	"github.com/dorkusprime/wolfcastle/cmd/cmdutil"
	"github.com/spf13/cobra"
)

// Register wires up the daemon-related commands (start, stop, log,
// status) and attaches them directly to rootCmd as top-level commands.
func Register(app *cmdutil.App, rootCmd *cobra.Command) {
	startCmd := newStartCmd(app)
	startCmd.Flags().String("node", "", "Scope execution to a subtree")
	startCmd.Flags().String("worktree", "", "Run in a git worktree on the specified branch")
	startCmd.Flags().BoolP("daemon", "d", false, "Run as background daemon")
	startCmd.Flags().BoolP("verbose", "v", false, "Set console log level to debug")
	startCmd.Flags().Bool("exit-when-done", false, "Exit after all available work is complete")
	startCmd.Flags().String("instance", "", "Worktree path to target (bypasses CWD-based discovery)")
	_ = startCmd.RegisterFlagCompletionFunc("node", cmdutil.CompleteNodeAddresses(app))

	stopCmd := newStopCmd(app)
	stopCmd.Flags().Bool("force", false, "Force kill (SIGKILL) instead of graceful stop")
	stopCmd.Flags().Bool("drain", false, "Finish current work then exit")
	stopCmd.Flags().String("instance", "", "Worktree path to target (bypasses CWD-based discovery)")

	logCmd := newLogCmd(app)
	logCmd.Flags().BoolP("follow", "f", false, "Follow live output (default when daemon is running)")
	logCmd.Flags().IntP("session", "s", 0, "Session index (0 = latest, 1 = previous, etc.)")
	logCmd.Flags().String("instance", "", "Worktree path to target (bypasses CWD-based discovery)")

	statusCmd := newStatusCmd(app)
	statusCmd.Flags().Bool("all", false, "Show status across all engineers")
	statusCmd.Flags().String("node", "", "Show status for a specific subtree")
	statusCmd.Flags().String("instance", "", "Worktree path to target (bypasses CWD-based discovery)")
	// --watch/-w is registered inside newStatusCmd
	statusCmd.Flags().BoolP("expand", "x", false, "Show completed nodes expanded (default: collapsed)")
	statusCmd.Flags().BoolP("detail", "d", false, "Show task bodies, failure reasons, deliverables, and breadcrumbs")
	statusCmd.Flags().Bool("archived", false, "Show only archived nodes")
	statusCmd.Flags().Int("width", 120, "Truncation width for text fields")
	_ = statusCmd.RegisterFlagCompletionFunc("node", cmdutil.CompleteNodeAddresses(app))

	startCmd.GroupID = "lifecycle"
	stopCmd.GroupID = "lifecycle"
	logCmd.GroupID = "lifecycle"
	statusCmd.GroupID = "lifecycle"
	daemonRunCmd := newDaemonRunCmd(app)

	rootCmd.AddCommand(startCmd)
	rootCmd.AddCommand(stopCmd)
	rootCmd.AddCommand(logCmd)
	rootCmd.AddCommand(statusCmd)
	rootCmd.AddCommand(daemonRunCmd)
}

// resolveInstance reads the --instance flag from cmd and, when set,
// re-initializes the App to point at that worktree's .wolfcastle.
// Used by start, follow, and status — every daemon command that needs
// the App's repositories pointed at a specific instance instead of the
// default CWD-resolved one. stop is intentionally not a caller: it
// only needs the worktree path to look up a PID via instance.Resolve,
// not a fully reconfigured App.
func resolveInstance(cmd *cobra.Command, app *cmdutil.App) error {
	instancePath, _ := cmd.Flags().GetString("instance")
	if instancePath == "" {
		return nil
	}
	return app.InitFromDir(instancePath)
}
