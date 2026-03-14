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
	startCmd.RegisterFlagCompletionFunc("node", cmdutil.CompleteNodeAddresses(app))

	stopCmd := newStopCmd(app)
	stopCmd.Flags().Bool("force", false, "Force kill (SIGKILL) instead of graceful stop")

	followCmd := newFollowCmd(app)
	followCmd.Flags().Int("lines", 20, "Number of historical lines to show before streaming")

	statusCmd := newStatusCmd(app)
	statusCmd.Flags().Bool("all", false, "Show status across all engineers")
	statusCmd.Flags().String("node", "", "Show status for a specific subtree")
	statusCmd.RegisterFlagCompletionFunc("node", cmdutil.CompleteNodeAddresses(app))

	rootCmd.AddCommand(startCmd)
	rootCmd.AddCommand(stopCmd)
	rootCmd.AddCommand(followCmd)
	rootCmd.AddCommand(statusCmd)
}
