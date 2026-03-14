package daemon

import (
	"fmt"
	"os"
	"syscall"

	"github.com/dorkusprime/wolfcastle/cmd/cmdutil"
	"github.com/dorkusprime/wolfcastle/internal/daemon"
	"github.com/dorkusprime/wolfcastle/internal/output"
	"github.com/spf13/cobra"
)

func newStopCmd(app *cmdutil.App) *cobra.Command {
	return &cobra.Command{
		Use:   "stop",
		Short: "Stop the Wolfcastle daemon",
		Long: `Sends a stop signal to the running Wolfcastle daemon.

By default, sends SIGTERM for a graceful shutdown. Use --force to send
SIGKILL if the daemon is not responding.

Examples:
  wolfcastle stop
  wolfcastle stop --force`,
		RunE: func(cmd *cobra.Command, args []string) error {
			force, _ := cmd.Flags().GetBool("force")

			pid, err := daemon.ReadPID(app.WolfcastleDir)
			if err != nil {
				return fmt.Errorf("no PID file found. Is Wolfcastle running?")
			}

			if !daemon.IsProcessRunning(pid) {
				daemon.RemovePID(app.WolfcastleDir)
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

			if app.JSONOutput {
				output.Print(output.Ok("stop", map[string]any{
					"pid":   pid,
					"force": force,
				}))
			} else {
				if force {
					output.PrintHuman("Force-killed Wolfcastle (PID %d)", pid)
				} else {
					output.PrintHuman("Sent stop signal to Wolfcastle (PID %d)", pid)
				}
			}
			return nil
		},
	}
}
