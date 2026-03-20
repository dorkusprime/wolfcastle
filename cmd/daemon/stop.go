package daemon

import (
	"fmt"
	"os"
	"syscall"

	"github.com/dorkusprime/wolfcastle/cmd/cmdutil"
	dmn "github.com/dorkusprime/wolfcastle/internal/daemon"
	"github.com/dorkusprime/wolfcastle/internal/output"
	"github.com/spf13/cobra"
)

func newStopCmd(app *cmdutil.App) *cobra.Command {
	return &cobra.Command{
		Use:   "stop",
		Short: "Stand down",
		Long: `Sends a stop signal to the running daemon. Graceful by default.
Use --force if it refuses to listen.

Examples:
  wolfcastle stop
  wolfcastle stop --force`,
		RunE: func(cmd *cobra.Command, args []string) error {
			force, _ := cmd.Flags().GetBool("force")

			pid, err := app.Daemon.ReadPID()
			if err != nil {
				return fmt.Errorf("no PID file found. Is the daemon running? Check with 'wolfcastle status'")
			}

			if !dmn.IsProcessRunning(pid) {
				_ = app.Daemon.RemovePID()
				return fmt.Errorf("PID %d is not running (stale PID file removed)", pid)
			}

			process, err := os.FindProcess(pid)
			if err != nil {
				return err
			}

			sig := syscall.SIGTERM
			if force {
				sig = syscall.SIGKILL
			}

			if err := process.Signal(sig); err != nil {
				return fmt.Errorf("sending signal to PID %d: %w", pid, err)
			}

			if app.JSON {
				output.Print(output.Ok("stop", map[string]any{
					"pid":   pid,
					"force": force,
				}))
			} else {
				if force {
					output.PrintHuman("Terminated with prejudice (PID %d)", pid)
				} else {
					output.PrintHuman("Stand-down signal sent (PID %d)", pid)
				}
			}
			return nil
		},
	}
}
