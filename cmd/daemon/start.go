package daemon

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"syscall"

	"github.com/dorkusprime/wolfcastle/cmd/cmdutil"
	"github.com/dorkusprime/wolfcastle/internal/daemon"
	"github.com/dorkusprime/wolfcastle/internal/output"
	"github.com/dorkusprime/wolfcastle/internal/validate"
	"github.com/spf13/cobra"
)

func newStartCmd(app *cmdutil.App) *cobra.Command {
	return &cobra.Command{
		Use:   "start",
		Short: "Unleash the daemon",
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

			// ADR-046: --verbose overrides daemon.log_level to debug.
			if verbose {
				app.Cfg.Daemon.LogLevel = "debug"
			}

			if err := app.RequireResolver(); err != nil {
				return err
			}

			// Find repo root (parent of .wolfcastle)
			repoDir := filepath.Dir(app.WolfcastleDir)
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
			recoverStaleDaemonState(app.WolfcastleDir)

			// Check global daemon lock (one daemon at a time, globally)
			if err := daemon.AcquireGlobalLock(repoDir, repoDir); err != nil {
				return err
			}
			defer daemon.ReleaseGlobalLock()

			// Check for running daemon (per-project PID, backward compat)
			pid, err := daemon.ReadPID(app.WolfcastleDir)
			if err == nil && daemon.IsProcessRunning(pid) {
				daemon.ReleaseGlobalLock()
				return fmt.Errorf("already running (PID %d). Use 'wolfcastle stop' first", pid)
			}
			daemon.RemovePID(app.WolfcastleDir)

			// Self-heal stale tasks BEFORE validation. Without this,
			// validation blocks startup for conditions self-heal would fix.
			daemon.PreStartSelfHeal(app.Resolver, app.WolfcastleDir)

			// Startup validation gate — block on error-severity issues
			idx, idxErr := app.Resolver.LoadRootIndex()
			if idxErr == nil {
				engine := validate.NewEngine(app.Resolver.ProjectsDir(), validate.DefaultNodeLoader(app.Resolver.ProjectsDir()), app.WolfcastleDir)
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
				return startBackground(app.WolfcastleDir, nodeScope, worktreeBranch, "")
			}

			d, err := daemon.New(app.Cfg, app.WolfcastleDir, app.Resolver, nodeScope, repoDir)
			if err != nil {
				return err
			}

			// Write PID file for foreground mode too, so `wolfcastle status`
			// can detect a running daemon regardless of how it was started.
			pidFile := filepath.Join(app.WolfcastleDir, "system", "wolfcastle.pid")
			_ = os.WriteFile(pidFile, []byte(fmt.Sprintf("%d\n", os.Getpid())), 0644)
			defer func() { _ = os.Remove(pidFile) }()

			return d.RunWithSupervisor(context.Background())
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
	pidFile := filepath.Join(wolfcastleDir, "system", "wolfcastle.pid")
	if err := os.WriteFile(pidFile, []byte(fmt.Sprintf("%d\n", proc.Process.Pid)), 0644); err != nil {
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
	pidPath := filepath.Join(wolfcastleDir, "system", "wolfcastle.pid")
	data, err := os.ReadFile(pidPath)
	if os.IsNotExist(err) {
		return
	}
	if err != nil {
		return
	}
	pid, err := strconv.Atoi(strings.TrimSpace(string(data)))
	if err != nil {
		_ = os.Remove(pidPath)
		return
	}
	process, err := os.FindProcess(pid)
	if err != nil {
		_ = os.Remove(pidPath)
		return
	}
	if err := process.Signal(syscall.Signal(0)); err != nil {
		// Process is dead — clean up stale files
		_ = os.Remove(pidPath)
		_ = os.Remove(filepath.Join(wolfcastleDir, "system", "daemon.meta.json"))
		_ = os.Remove(filepath.Join(wolfcastleDir, "system", "stop"))
	}
}
