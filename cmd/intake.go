package cmd

import (
	"context"
	"os"
	"os/signal"
	"path/filepath"
	"time"

	dmn "github.com/dorkusprime/wolfcastle/internal/daemon"
	"github.com/dorkusprime/wolfcastle/internal/logrender"
	"github.com/dorkusprime/wolfcastle/internal/output"
	"github.com/dorkusprime/wolfcastle/internal/signals"
	"github.com/dorkusprime/wolfcastle/internal/tierfs"
	"github.com/spf13/cobra"
)

var intakeCmd = &cobra.Command{
	Use:   "intake",
	Short: "Process inbox items with live interleaved output",
	Long: `Processes pending inbox items in the foreground, streaming interleaved
output to stdout. A background goroutine tails the NDJSON log files as
they are written and renders them through the same interleaved renderer
used by "wolfcastle log --interleaved --follow".

The intake loop watches inbox.json for new items and runs the intake
stage for each one, just as the daemon would in the background.

Examples:
  wolfcastle intake
  wolfcastle intake --node auth-module`,
	RunE: func(cmd *cobra.Command, args []string) error {
		nodeScope, _ := cmd.Flags().GetString("node")

		cfg, err := app.Config.Load()
		if err != nil {
			return err
		}

		if err := app.RequireIdentity(); err != nil {
			return err
		}

		logDir := filepath.Join(app.Config.Root(), tierfs.SystemPrefix, "logs")
		repoDir := filepath.Dir(app.Config.Root())

		if app.Daemon.IsAlive() {
			return errDaemonRunning
		}

		d, err := dmn.New(cfg, app.Config.Root(), app.State, nodeScope, repoDir)
		if err != nil {
			return err
		}

		output.PrintHuman("wolfcastle intake %s", app.Version)

		ctx, cancel := signal.NotifyContext(context.Background(), signals.Shutdown...)
		defer cancel()

		// Start the interleaved renderer goroutine, identical to the
		// one in the execute command.
		reader := logrender.NewFollowReader(logDir, 200*time.Millisecond)
		records := reader.Records(ctx)
		renderDone := make(chan struct{})
		go func() {
			defer close(renderDone)
			ir := logrender.NewInterleavedRenderer(&output.SpinnerWriter{W: os.Stdout})
			ir.Render(ctx, records)
		}()

		// Run the inbox loop. It blocks until the context is cancelled.
		d.RunInbox(ctx)
		cancel()
		<-renderDone

		return nil
	},
}

func init() {
	intakeCmd.Flags().String("node", "", "Scope intake to a subtree")
	rootCmd.AddCommand(intakeCmd)
}
