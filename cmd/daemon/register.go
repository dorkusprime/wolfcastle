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
	_ = startCmd.RegisterFlagCompletionFunc("node", cmdutil.CompleteNodeAddresses(app))

	stopCmd := newStopCmd(app)
	stopCmd.Flags().Bool("force", false, "Force kill (SIGKILL) instead of graceful stop")

	logCmd := newLogCmd(app)
	logCmd.Flags().BoolP("follow", "f", false, "Stream output in real time (like tail -f)")
	logCmd.Flags().Int("lines", 20, "Number of lines to show")
	logCmd.Flags().StringP("level", "l", "", "Minimum log level (debug, info, warn, error)")

	statusCmd := newStatusCmd(app)
	statusCmd.Flags().Bool("all", false, "Show status across all engineers")
	statusCmd.Flags().String("node", "", "Show status for a specific subtree")
	statusCmd.Flags().BoolP("watch", "w", false, "Refresh status on an interval")
	statusCmd.Flags().Float64P("interval", "n", 5, "Refresh interval in seconds (with --watch)")
	_ = statusCmd.RegisterFlagCompletionFunc("node", cmdutil.CompleteNodeAddresses(app))

	startCmd.GroupID = "lifecycle"
	stopCmd.GroupID = "lifecycle"
	logCmd.GroupID = "lifecycle"
	statusCmd.GroupID = "lifecycle"
	rootCmd.AddCommand(startCmd)
	rootCmd.AddCommand(stopCmd)
	rootCmd.AddCommand(logCmd)
	rootCmd.AddCommand(statusCmd)
}
