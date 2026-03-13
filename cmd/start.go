package cmd

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"

	"github.com/dorkusprime/wolfcastle/internal/daemon"
	"github.com/spf13/cobra"
)

var startCmd = &cobra.Command{
	Use:   "start",
	Short: "Start the Wolfcastle daemon",
	Long:  "Runs the daemon loop, executing tasks from the project tree.",
	RunE: func(cmd *cobra.Command, args []string) error {
		nodeScope, _ := cmd.Flags().GetString("node")
		background, _ := cmd.Flags().GetBool("d")
		worktreeBranch, _ := cmd.Flags().GetString("worktree")

		if resolver == nil {
			return fmt.Errorf("identity not configured — run 'wolfcastle init' first")
		}

		// Find repo root (parent of .wolfcastle)
		repoDir := filepath.Dir(wolfcastleDir)

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
				cleanupWorktree(repoDir, wtDir)
			}
		}()

		// Check for stale PID
		pid, err := daemon.ReadPID(wolfcastleDir)
		if err == nil && daemon.IsProcessRunning(pid) {
			return fmt.Errorf("Wolfcastle is already running (PID %d) — use 'wolfcastle stop' first", pid)
		}
		daemon.RemovePID(wolfcastleDir)

		if background {
			return startBackground(nodeScope, worktreeBranch)
		}

		d, err := daemon.New(cfg, wolfcastleDir, resolver, nodeScope, repoDir)
		if err != nil {
			return err
		}

		return d.Run(context.Background())
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
	os.WriteFile(pidFile, []byte(fmt.Sprintf("%d\n", proc.Process.Pid)), 0644)

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

func init() {
	startCmd.Flags().String("node", "", "Scope execution to a subtree")
	startCmd.Flags().String("worktree", "", "Run in a git worktree on the specified branch")
	startCmd.Flags().BoolP("d", "d", false, "Run as background daemon")
	rootCmd.AddCommand(startCmd)
}
