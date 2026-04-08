package daemon

import (
	"fmt"
	"os"
	"syscall"

	"github.com/dorkusprime/wolfcastle/cmd/cmdutil"
	dmn "github.com/dorkusprime/wolfcastle/internal/daemon"
	"github.com/dorkusprime/wolfcastle/internal/instance"
	"github.com/dorkusprime/wolfcastle/internal/output"
	"github.com/spf13/cobra"
)

func newStopCmd(app *cmdutil.App) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "stop",
		Short: "Stand down",
		Long: `Sends a stop signal to the running daemon. Graceful by default.
Use --force if it refuses to listen.
Use --drain to let the daemon finish its current work before exiting.
Use --all to stop every running instance.

Examples:
  wolfcastle stop
  wolfcastle stop --force
  wolfcastle stop --drain
  wolfcastle stop --all`,
		RunE: func(cmd *cobra.Command, args []string) error {
			force, _ := cmd.Flags().GetBool("force")
			drain, _ := cmd.Flags().GetBool("drain")
			all, _ := cmd.Flags().GetBool("all")

			if all {
				return stopAllInstances(force, app.JSON)
			}

			if drain {
				if err := app.Daemon.WriteDrainFile(); err != nil {
					return fmt.Errorf("writing drain file: %w", err)
				}
				if app.JSON {
					output.Print(output.Ok("stop", map[string]any{
						"drain": true,
					}))
				} else {
					output.PrintHuman("Drain signal sent. Daemon will exit after current work completes.")
				}
				return nil
			}

			instancePath, _ := cmd.Flags().GetString("instance")
			resolveDir := instancePath
			if resolveDir == "" {
				var err error
				resolveDir, err = os.Getwd()
				if err != nil {
					return fmt.Errorf("resolving working directory: %w", err)
				}
			}

			entry, err := instance.Resolve(resolveDir)
			if err != nil {
				return fmt.Errorf("no running instance found for this directory, check with 'wolfcastle status'")
			}

			return stopInstance(entry, force, app.JSON)
		},
	}

	cmd.Flags().Bool("all", false, "Stop all running instances")

	return cmd
}

func stopInstance(entry *instance.Entry, force, jsonMode bool) error {
	if !dmn.IsProcessRunning(entry.PID) {
		// Stale entry; List() would have cleaned it, but be safe.
		_ = instance.Deregister(entry.Worktree)
		return fmt.Errorf("pid %d is not running (stale registry entry removed)", entry.PID)
	}

	process, err := os.FindProcess(entry.PID)
	if err != nil {
		return err
	}

	sig := syscall.SIGTERM
	if force {
		sig = syscall.SIGKILL
	}

	if err := process.Signal(sig); err != nil {
		return fmt.Errorf("sending signal to PID %d: %w", entry.PID, err)
	}

	if jsonMode {
		output.Print(output.Ok("stop", map[string]any{
			"pid":   entry.PID,
			"force": force,
		}))
	} else {
		if force {
			output.PrintHuman("Terminated with prejudice (PID %d)", entry.PID)
		} else {
			output.PrintHuman("Stand-down signal sent (PID %d)", entry.PID)
		}
	}
	return nil
}

func stopAllInstances(force, jsonMode bool) error {
	instances, err := instance.List()
	if err != nil {
		return fmt.Errorf("listing instances: %w", err)
	}
	if len(instances) == 0 {
		output.PrintHuman("No running instances found.")
		return nil
	}

	var stopped int
	for i := range instances {
		if err := stopInstance(&instances[i], force, jsonMode); err != nil {
			output.PrintHuman("Failed to stop PID %d: %v", instances[i].PID, err)
			continue
		}
		stopped++
	}
	if !jsonMode {
		output.PrintHuman("Stopped %d of %d instances.", stopped, len(instances))
	}
	return nil
}
