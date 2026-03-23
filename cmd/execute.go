package cmd

import (
	"context"
	"fmt"
	"os"
	"os/signal"
	"path/filepath"
	"time"

	dmn "github.com/dorkusprime/wolfcastle/internal/daemon"
	"github.com/dorkusprime/wolfcastle/internal/logrender"
	"github.com/dorkusprime/wolfcastle/internal/output"
	"github.com/dorkusprime/wolfcastle/internal/signals"
	"github.com/spf13/cobra"
)

var executeCmd = &cobra.Command{
	Use:   "execute",
	Short: "Run the execution loop with live interleaved output",
	Long: `Runs the daemon execution loop in the foreground, streaming interleaved
output to stdout. A background goroutine tails the NDJSON log files as
they are written and renders them through the same interleaved renderer
used by "wolfcastle log --interleaved --follow".

This is the non-daemon counterpart to "wolfcastle start": same execution
loop, but with formatted output directly on the terminal instead of
requiring a separate "wolfcastle log" session.

Examples:
  wolfcastle execute
  wolfcastle execute --node auth-module`,
	RunE: func(cmd *cobra.Command, args []string) error {
		nodeScope, _ := cmd.Flags().GetString("node")

		cfg, err := app.Config.Load()
		if err != nil {
			return err
		}

		if err := app.RequireIdentity(); err != nil {
			return err
		}

		repoDir := filepath.Dir(app.Config.Root())
		logDir := filepath.Join(app.Config.Root(), "system", "logs")

		// Check for a running daemon before proceeding.
		if app.Daemon.IsAlive() {
			return errDaemonRunning
		}

		d, err := dmn.New(cfg, app.Config.Root(), app.State, nodeScope, repoDir)
		if err != nil {
			return err
		}

		output.PrintHuman("wolfcastle %s", app.Version)

		ctx, cancel := signal.NotifyContext(context.Background(), signals.Shutdown...)
		defer cancel()

		// Start the interleaved renderer goroutine. It tails the log
		// directory for new NDJSON files and renders them to stdout,
		// producing output identical to "wolfcastle log -i -f".
		reader := logrender.NewFollowReader(logDir, 200*time.Millisecond)
		records := reader.Records(ctx)
		renderDone := make(chan struct{})
		go func() {
			defer close(renderDone)
			ir := logrender.NewInterleavedRenderer(os.Stdout)
			ir.Render(ctx, records)
		}()

		// Run the daemon loop. When it returns (completion, signal, or
		// error), cancel the renderer context and wait for it to drain.
		runErr := d.Run(ctx)
		cancel()
		<-renderDone

		return runErr
	},
}

var errDaemonRunning = fmt.Errorf("daemon is already running. Use 'wolfcastle stop' first, or use 'wolfcastle log -i -f' to watch it")

func init() {
	executeCmd.Flags().String("node", "", "Scope execution to a subtree")
	rootCmd.AddCommand(executeCmd)
}
