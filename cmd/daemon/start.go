package daemon

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"os/signal"
	"path/filepath"
	"time"

	"github.com/dorkusprime/wolfcastle/cmd/cmdutil"
	dmn "github.com/dorkusprime/wolfcastle/internal/daemon"
	"github.com/dorkusprime/wolfcastle/internal/logrender"
	"github.com/dorkusprime/wolfcastle/internal/output"
	"github.com/dorkusprime/wolfcastle/internal/signals"
	"github.com/dorkusprime/wolfcastle/internal/validate"
	"github.com/spf13/cobra"
)

func newStartCmd(app *cmdutil.App) *cobra.Command {
	return &cobra.Command{
		Use:   "start",
		Short: "Start the daemon",
		Long: `Starts the execution loop. Wolfcastle picks up tasks, calls models,
validates results, and moves to the next target. Use --node to restrict
the carnage to a subtree. Use -d to run in the background.

Examples:
  wolfcastle start
  wolfcastle start --node auth-system
  wolfcastle start -d
  wolfcastle start --worktree feature-branch`,
		RunE: func(cmd *cobra.Command, args []string) error {
			output.PrintHuman("wolfcastle %s", app.Version)
			nodeScope, _ := cmd.Flags().GetString("node")
			background, _ := cmd.Flags().GetBool("daemon")
			worktreeBranch, _ := cmd.Flags().GetString("worktree")
			verbose, _ := cmd.Flags().GetBool("verbose")

			cfg, err := app.Config.Load()
			if err != nil {
				return err
			}

			// ADR-046: --verbose overrides daemon.log_level to debug.
			if verbose {
				cfg.Daemon.LogLevel = "debug"
			}

			if err := app.RequireIdentity(); err != nil {
				return err
			}

			// Find repo root (parent of .wolfcastle)
			repoDir := filepath.Dir(app.Config.Root())
			originalRepoDir := repoDir

			// Handle worktree mode
			var wtDir string
			if worktreeBranch != "" {
				var err error
				wtDir, err = createWorktree(repoDir, worktreeBranch)
				if err != nil {
					return fmt.Errorf("creating worktree: %w", err)
				}
				repoDir = wtDir
				output.PrintHuman("Operating in worktree: %s (branch: %s)", wtDir, worktreeBranch)
			}
			defer func() {
				if worktreeBranch != "" {
					cleanupWorktree(originalRepoDir, wtDir)
				}
			}()

			// Recover stale daemon state
			recoverStaleDaemonState(app.Config.Root())

			// Check global daemon lock (one daemon at a time, globally)
			if err := dmn.AcquireGlobalLock(repoDir, repoDir); err != nil {
				return err
			}
			defer dmn.ReleaseGlobalLock()

			// Check for running daemon (per-project PID, backward compat)
			pid, pidErr := app.Daemon.ReadPID()
			if pidErr == nil && dmn.IsProcessRunning(pid) {
				dmn.ReleaseGlobalLock()
				return fmt.Errorf("already running (PID %d). Use 'wolfcastle stop' first", pid)
			}
			_ = app.Daemon.RemovePID()

			// Self-heal before validation: fix deterministic issues so
			// startup validation doesn't block on repairable state.
			// Omit wolfcastleDir so the fix pass skips daemon artifact
			// checks (PID file, stop file are intentional at startup).
			idx, idxErr := app.State.ReadIndex()
			if idxErr == nil {
				nodeLoader := validate.DefaultNodeLoader(app.State.Dir())
				healFixes, _, healErr := validate.FixWithVerification(
					app.State.Dir(),
					filepath.Join(app.State.Dir(), "state.json"),
					nodeLoader,
				)
				if healErr != nil {
					output.PrintHuman("Pre-start self-heal error: %v", healErr)
				}
				if len(healFixes) > 0 {
					output.PrintHuman("Self-healed %s before startup:", output.Plural(len(healFixes), "issue", "issues"))
					for _, f := range healFixes {
						output.PrintHuman("  FIXED [%s] %s: %s", f.Category, f.Node, f.Description)
					}
					idx, idxErr = app.State.ReadIndex()
				}
			}

			// Startup validation gate — block on error-severity issues
			if idxErr == nil {
				engine := validate.NewEngine(app.State.Dir(), validate.DefaultNodeLoader(app.State.Dir()), app.Config.Root())
				report := engine.ValidateStartup(idx)
				if report.HasErrors() {
					output.PrintHuman("Startup blocked. %d error(s):", report.Errors)
					for _, issue := range report.Issues {
						if issue.Severity == validate.SeverityError {
							output.PrintHuman("  ERROR [%s] %s: %s", issue.Category, issue.Node, issue.Description)
						}
					}
					return fmt.Errorf("validation errors. Run 'wolfcastle doctor --fix' to repair")
				}
				if report.Warnings > 0 {
					output.PrintHuman("%d warning(s). Proceeding anyway.", report.Warnings)
				}
			}

			if background {
				return startBackground(app.Config.Root(), nodeScope, worktreeBranch, "")
			}

			d, err := dmn.New(cfg, app.Config.Root(), app.State, nodeScope, repoDir)
			if err != nil {
				return err
			}

			// Write PID file for foreground mode too, so `wolfcastle status`
			// can detect a running daemon regardless of how it was started.
			if err := app.Daemon.WritePID(os.Getpid()); err != nil {
				return fmt.Errorf("writing PID file: %w", err)
			}
			defer func() { _ = app.Daemon.RemovePID() }()

			ctx, cancel := signal.NotifyContext(context.Background(), signals.Shutdown...)
			defer cancel()

			// Start the interleaved renderer goroutine. It tails the
			// log directory for new NDJSON files and renders them to
			// stdout, identical to "wolfcastle log -i -f".
			logDir := filepath.Join(app.Config.Root(), "system", "logs")
			reader := logrender.NewFollowReader(logDir, 200*time.Millisecond)
			records := reader.Records(ctx)
			renderDone := make(chan struct{})
			go func() {
				defer close(renderDone)
				ir := logrender.NewInterleavedRenderer(os.Stdout)
				ir.Render(ctx, records)
			}()

			runErr := d.RunWithSupervisor(ctx)
			cancel()
			<-renderDone

			return runErr
		},
	}
}

// startBackground launches the daemon as a detached background process.
// executablePath is the binary to re-exec; pass "" to use os.Executable().
func startBackground(wolfcastleDir, nodeScope, worktreeBranch, executablePath string) error {
	if executablePath == "" {
		var err error
		executablePath, err = os.Executable()
		if err != nil {
			return fmt.Errorf("finding executable: %w", err)
		}
	}

	cmdArgs := []string{"start"}
	if nodeScope != "" {
		cmdArgs = append(cmdArgs, "--node", nodeScope)
	}
	if worktreeBranch != "" {
		cmdArgs = append(cmdArgs, "--worktree", worktreeBranch)
	}

	proc := exec.Command(executablePath, cmdArgs...)
	proc.Stdin = nil
	proc.Dir = filepath.Dir(wolfcastleDir)

	// Redirect stdout/stderr to a daemon log file so startup errors
	// aren't silently lost.
	daemonLog := filepath.Join(wolfcastleDir, "system", "daemon.log")
	logFile, err := os.OpenFile(daemonLog, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0644)
	if err != nil {
		return fmt.Errorf("creating daemon log: %w", err)
	}
	proc.Stdout = logFile
	proc.Stderr = logFile

	if err := proc.Start(); err != nil {
		_ = logFile.Close()
		return fmt.Errorf("starting background process: %w", err)
	}

	// Write PID file
	repo := dmn.NewDaemonRepository(wolfcastleDir)
	if err := repo.WritePID(proc.Process.Pid); err != nil {
		return fmt.Errorf("writing PID file: %w", err)
	}

	output.PrintHuman("Daemon deployed (PID %d)", proc.Process.Pid)
	output.PrintHuman("  wolfcastle log -f    Watch the operation")
	output.PrintHuman("  wolfcastle stop      Stand down")

	// Detach
	_ = proc.Process.Release()
	return nil
}

func cleanupWorktree(repoDir, wtDir string) {
	removeCmd := exec.Command("git", "worktree", "remove", wtDir)
	removeCmd.Dir = repoDir
	if out, err := removeCmd.CombinedOutput(); err != nil {
		output.PrintHuman("Could not remove worktree %s: %s (%v)", wtDir, string(out), err)
	} else {
		output.PrintHuman("Cleaned up worktree: %s", wtDir)
	}
}

func createWorktree(repoDir, branch string) (string, error) {
	wtDir := filepath.Join(filepath.Dir(repoDir), ".wolfcastle", "worktrees", branch)

	// Check if branch exists
	checkCmd := exec.Command("git", "rev-parse", "--verify", branch)
	checkCmd.Dir = repoDir
	branchExists := checkCmd.Run() == nil

	var gitCmd *exec.Cmd
	if branchExists {
		gitCmd = exec.Command("git", "worktree", "add", wtDir, branch)
	} else {
		gitCmd = exec.Command("git", "worktree", "add", "-b", branch, wtDir)
	}
	gitCmd.Dir = repoDir

	if out, err := gitCmd.CombinedOutput(); err != nil {
		return "", fmt.Errorf("git worktree add: %s: %w", string(out), err)
	}

	return wtDir, nil
}

func recoverStaleDaemonState(wolfcastleDir string) {
	repo := dmn.NewDaemonRepository(wolfcastleDir)
	if !repo.PIDFileExists() {
		return
	}
	if repo.IsAlive() {
		return
	}
	// Process is dead or PID is unreadable. Clean up stale files.
	_ = repo.RemovePID()
	_ = os.Remove(filepath.Join(wolfcastleDir, "system", "daemon.meta.json"))
	_ = repo.RemoveStopFile()
}
