package cmd

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"syscall"

	"github.com/dorkusprime/wolfcastle/internal/daemon"
	"github.com/dorkusprime/wolfcastle/internal/validate"
	"github.com/spf13/cobra"
)

var startCmd = &cobra.Command{
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
		background, _ := cmd.Flags().GetBool("d")
		worktreeBranch, _ := cmd.Flags().GetString("worktree")

		if resolver == nil {
			return fmt.Errorf("identity not configured — run 'wolfcastle init' first")
		}

		// Find repo root (parent of .wolfcastle)
		repoDir := filepath.Dir(wolfcastleDir)
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
			fmt.Printf("Working in worktree: %s (branch: %s)\n", wtDir, worktreeBranch)
		}
		defer func() {
			if worktreeBranch != "" {
				cleanupWorktree(originalRepoDir, wtDir)
			}
		}()

		// Recover stale daemon state
		recoverStaleDaemonState(wolfcastleDir)

		// Check for running daemon
		pid, err := daemon.ReadPID(wolfcastleDir)
		if err == nil && daemon.IsProcessRunning(pid) {
			return fmt.Errorf("Wolfcastle is already running (PID %d) — use 'wolfcastle stop' first", pid)
		}
		daemon.RemovePID(wolfcastleDir)

		// Startup validation gate — block on error-severity issues
		if resolver != nil {
			idx, idxErr := resolver.LoadRootIndex()
			if idxErr == nil {
				engine := validate.NewEngine(resolver.ProjectsDir(), validate.DefaultNodeLoader(resolver.ProjectsDir()), wolfcastleDir)
				report := engine.ValidateStartup(idx)
				if report.HasErrors() {
					fmt.Printf("Startup validation found %d errors:\n", report.Errors)
					for _, issue := range report.Issues {
						if issue.Severity == validate.SeverityError {
							fmt.Printf("  ERROR [%s] %s: %s\n", issue.Category, issue.Node, issue.Description)
						}
					}
					return fmt.Errorf("startup blocked by validation errors — run 'wolfcastle doctor --fix' to repair")
				}
				if report.Warnings > 0 {
					fmt.Printf("Startup validation: %d warnings (proceeding)\n", report.Warnings)
				}
			}
		}

		if background {
			return startBackground(nodeScope, worktreeBranch)
		}

		d, err := daemon.New(cfg, wolfcastleDir, resolver, nodeScope, repoDir)
		if err != nil {
			return err
		}

		return d.RunWithSupervisor(context.Background())
	},
}

func startBackground(nodeScope, worktreeBranch string) error {
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

	fmt.Printf("Wolfcastle started in background (PID %d)\n", proc.Process.Pid)
	fmt.Println("  Use 'wolfcastle follow' to watch output")
	fmt.Println("  Use 'wolfcastle stop' to stop")

	// Detach
	proc.Process.Release()
	return nil
}

func cleanupWorktree(repoDir, wtDir string) {
	removeCmd := exec.Command("git", "worktree", "remove", wtDir)
	removeCmd.Dir = repoDir
	if out, err := removeCmd.CombinedOutput(); err != nil {
		fmt.Printf("WARNING: could not remove worktree %s: %s (%v)\n", wtDir, string(out), err)
	} else {
		fmt.Printf("Cleaned up worktree: %s\n", wtDir)
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
		os.Remove(pidPath)
		return
	}
	process, err := os.FindProcess(pid)
	if err != nil {
		os.Remove(pidPath)
		return
	}
	if err := process.Signal(syscall.Signal(0)); err != nil {
		// Process is dead — clean up stale files
		os.Remove(pidPath)
		os.Remove(filepath.Join(wolfcastleDir, "daemon.meta.json"))
		os.Remove(filepath.Join(wolfcastleDir, "stop"))
	}
}

func init() {
	startCmd.Flags().String("node", "", "Scope execution to a subtree")
	startCmd.Flags().String("worktree", "", "Run in a git worktree on the specified branch")
	startCmd.Flags().BoolP("d", "d", false, "Run as background daemon")
	rootCmd.AddCommand(startCmd)
}
