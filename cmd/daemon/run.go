package daemon

import (
	"context"
	"os/signal"
	"path/filepath"

	"github.com/dorkusprime/wolfcastle/cmd/cmdutil"
	dmn "github.com/dorkusprime/wolfcastle/internal/daemon"
	"github.com/dorkusprime/wolfcastle/internal/output"
	"github.com/dorkusprime/wolfcastle/internal/signals"
	"github.com/spf13/cobra"
)

// newDaemonRunCmd creates the hidden _daemon-run command. This is the
// background half of "wolfcastle start -d": no interactive checks, no
// validation, no dirty-tree prompts. The foreground start command
// handles all of that before re-execing into this command.
func newDaemonRunCmd(app *cmdutil.App) *cobra.Command {
	cmd := &cobra.Command{
		Use:    "_daemon-run",
		Hidden: true,
		Short:  "Internal: run the daemon loop (called by start -d)",
		RunE: func(cmd *cobra.Command, args []string) error {
			output.PrintHuman("wolfcastle %s", app.Version)
			nodeScope, _ := cmd.Flags().GetString("node")
			exitWhenDone, _ := cmd.Flags().GetBool("exit-when-done")
			verbose, _ := cmd.Flags().GetBool("verbose")

			cfg, err := app.Config.Load()
			if err != nil {
				return err
			}

			if verbose {
				cfg.Daemon.LogLevel = "debug"
			}

			repoDir := filepath.Dir(app.Config.Root())

			// Acquire per-worktree lock. The foreground start command does not
			// hold the lock across the re-exec boundary, so we acquire
			// it here.
			if err := dmn.AcquireLock(app.Config.Root(), repoDir, ""); err != nil {
				return err
			}
			defer dmn.ReleaseLock(app.Config.Root())

			d, err := dmn.New(cfg, app.Config.Root(), app.State, nodeScope, repoDir)
			if err != nil {
				return err
			}
			d.ExitWhenDone = exitWhenDone

			ctx, cancel := signal.NotifyContext(context.Background(), signals.Shutdown...)
			defer cancel()

			runErr := d.RunWithSupervisor(ctx)
			return runErr
		},
	}

	cmd.Flags().String("node", "", "Scope execution to a subtree")
	cmd.Flags().Bool("exit-when-done", false, "Exit after all available work is complete")
	cmd.Flags().BoolP("verbose", "v", false, "Set console log level to debug")

	return cmd
}
