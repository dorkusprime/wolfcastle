// Package daemon implements the lifecycle commands: start, stop, status,
// and follow. These control the daemon loop and provide real-time
// visibility into its execution.
package daemon

import (
	"github.com/dorkusprime/wolfcastle/cmd/cmdutil"
	"github.com/spf13/cobra"
)

// Register wires up the daemon-related commands (start, stop, follow,
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

	followCmd := newFollowCmd(app)
	followCmd.Flags().Int("lines", 20, "Number of historical lines to show before streaming")
	followCmd.Flags().StringP("level", "l", "", "Minimum log level to display (debug, info, warn, error)")

	statusCmd := newStatusCmd(app)
	statusCmd.Flags().Bool("all", false, "Show status across all engineers")
	statusCmd.Flags().String("node", "", "Show status for a specific subtree")
	_ = statusCmd.RegisterFlagCompletionFunc("node", cmdutil.CompleteNodeAddresses(app))

	startCmd.GroupID = "lifecycle"
	stopCmd.GroupID = "lifecycle"
	followCmd.GroupID = "lifecycle"
	statusCmd.GroupID = "lifecycle"
	rootCmd.AddCommand(startCmd)
	rootCmd.AddCommand(stopCmd)
	rootCmd.AddCommand(followCmd)
	rootCmd.AddCommand(statusCmd)
}
