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
		Short: "Start the Wolfcastle daemon",
		Long: `Starts the Wolfcastle daemon loop, executing tasks from the project tree.

The daemon picks the next actionable task, invokes the configured model
pipeline, and updates state. Use --node to scope execution to a subtree.
Use -d to run in the background, then 'wolfcastle follow' to watch output.

Examples:
  wolfcastle start
  wolfcastle start --node auth-system
  wolfcastle start -d
  wolfcastle start --worktree feature-branch`,
		RunE: func(cmd *cobra.Command, args []string) error {
			nodeScope, _ := cmd.Flags().GetString("node")
			background, _ := cmd.Flags().GetBool("daemon")
			worktreeBranch, _ := cmd.Flags().GetString("worktree")

			if app.Resolver == nil {
				return fmt.Errorf("identity not configured — run 'wolfcastle init' first")
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
				output.PrintHuman("Working in worktree: %s (branch: %s)", wtDir, worktreeBranch)
			}
			defer func() {
				if worktreeBranch != "" {
					cleanupWorktree(originalRepoDir, wtDir)
				}
			}()

			// Recover stale daemon state
			recoverStaleDaemonState(app.WolfcastleDir)

			// Check for running daemon
			pid, err := daemon.ReadPID(app.WolfcastleDir)
			if err == nil && daemon.IsProcessRunning(pid) {
				return fmt.Errorf("Wolfcastle is already running (PID %d) — use 'wolfcastle stop' first", pid)
			}
			daemon.RemovePID(app.WolfcastleDir)

			// Startup validation gate — block on error-severity issues
			if app.Resolver != nil {
				idx, idxErr := app.Resolver.LoadRootIndex()
				if idxErr == nil {
					engine := validate.NewEngine(app.Resolver.ProjectsDir(), validate.DefaultNodeLoader(app.Resolver.ProjectsDir()), app.WolfcastleDir)
					report := engine.ValidateStartup(idx)
					if report.HasErrors() {
						output.PrintHuman("Startup validation found %d errors:", report.Errors)
						for _, issue := range report.Issues {
							if issue.Severity == validate.SeverityError {
								output.PrintHuman("  ERROR [%s] %s: %s", issue.Category, issue.Node, issue.Description)
							}
						}
						return fmt.Errorf("startup blocked by validation errors — run 'wolfcastle doctor --fix' to repair")
					}
					if report.Warnings > 0 {
						output.PrintHuman("Startup validation: %d warnings (proceeding)", report.Warnings)
					}
				}
			}

			if background {
				return startBackground(app.WolfcastleDir, nodeScope, worktreeBranch)
			}

			d, err := daemon.New(app.Cfg, app.WolfcastleDir, app.Resolver, nodeScope, repoDir)
			if err != nil {
				return err
			}

			return d.RunWithSupervisor(context.Background())
		},
	}
}

func startBackground(wolfcastleDir, nodeScope, worktreeBranch string) error {
	// Re-exec ourselves with the same flags but without -d
	execPath, err := os.Executable()
	if err != nil {
		return fmt.Errorf("finding executable: %w", err)
	}

	cmdArgs := []string{"start"}
	if nodeScope != "" {
		cmdArgs = append(cmdArgs, "--node", nodeScope)
	}
	if worktreeBranch != "" {
		cmdArgs = append(cmdArgs, "--worktree", worktreeBranch)
	}

	proc := exec.Command(execPath, cmdArgs...)
	proc.Stdout = nil
	proc.Stderr = nil
	proc.Stdin = nil
	// Detach from parent process group
	proc.Dir = filepath.Dir(wolfcastleDir)

	if err := proc.Start(); err != nil {
		return fmt.Errorf("starting background process: %w", err)
	}

	// Write PID file
	pidFile := filepath.Join(wolfcastleDir, "wolfcastle.pid")
	if err := os.WriteFile(pidFile, []byte(fmt.Sprintf("%d\n", proc.Process.Pid)), 0644); err != nil {
		return fmt.Errorf("writing PID file: %w", err)
	}

	output.PrintHuman("Wolfcastle started in background (PID %d)", proc.Process.Pid)
	output.PrintHuman("  Use 'wolfcastle follow' to watch output")
	output.PrintHuman("  Use 'wolfcastle stop' to stop")

	// Detach
	proc.Process.Release()
	return nil
}

func cleanupWorktree(repoDir, wtDir string) {
	removeCmd := exec.Command("git", "worktree", "remove", wtDir)
	removeCmd.Dir = repoDir
	if out, err := removeCmd.CombinedOutput(); err != nil {
		output.PrintHuman("WARNING: could not remove worktree %s: %s (%v)", wtDir, string(out), err)
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
	pidPath := filepath.Join(wolfcastleDir, "wolfcastle.pid")
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
		_ = os.Remove(filepath.Join(wolfcastleDir, "daemon.meta.json"))
		_ = os.Remove(filepath.Join(wolfcastleDir, "stop"))
	}
}
