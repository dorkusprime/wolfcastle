package daemon

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

// initGitRepo creates a temp git repo with one commit so worktree operations work.
func initGitRepo(t *testing.T) string {
	t.Helper()
	dir := t.TempDir()

	for _, args := range [][]string{
		{"init"},
		{"config", "user.email", "test@test.com"},
		{"config", "user.name", "Test"},
		{"commit", "--allow-empty", "-m", "init"},
	} {
		cmd := exec.Command("git", args...)
		cmd.Dir = dir
		if out, err := cmd.CombinedOutput(); err != nil {
			t.Fatalf("git %s: %s (%v)", args[0], out, err)
		}
	}
	return dir
}

// ═══════════════════════════════════════════════════════════════════════════
// createWorktree
// ═══════════════════════════════════════════════════════════════════════════

func TestCreateWorktree_NewBranch(t *testing.T) {
	repoDir := initGitRepo(t)

	wtDir, err := createWorktree(repoDir, "fresh-branch")
	if err != nil {
		t.Fatalf("createWorktree (new branch) failed: %v", err)
	}

	// The worktree directory should exist on disk.
	if _, err := os.Stat(wtDir); os.IsNotExist(err) {
		t.Fatal("worktree directory was not created")
	}

	// The returned path should land under .wolfcastle/worktrees/<branch>.
	expected := filepath.Join(filepath.Dir(repoDir), ".wolfcastle", "worktrees", "fresh-branch")
	if wtDir != expected {
		t.Errorf("unexpected worktree path\n  got:  %s\n  want: %s", wtDir, expected)
	}

	// The new branch should now exist in the repo.
	check := exec.Command("git", "rev-parse", "--verify", "fresh-branch")
	check.Dir = repoDir
	if err := check.Run(); err != nil {
		t.Error("branch 'fresh-branch' should exist after createWorktree with -b flag")
	}
}

func TestCreateWorktree_ExistingBranch(t *testing.T) {
	repoDir := initGitRepo(t)

	// Create the branch ahead of time so createWorktree takes the non -b path.
	branchCmd := exec.Command("git", "branch", "pre-existing")
	branchCmd.Dir = repoDir
	if out, err := branchCmd.CombinedOutput(); err != nil {
		t.Fatalf("creating branch: %s (%v)", out, err)
	}

	wtDir, err := createWorktree(repoDir, "pre-existing")
	if err != nil {
		t.Fatalf("createWorktree (existing branch) failed: %v", err)
	}

	if _, err := os.Stat(wtDir); os.IsNotExist(err) {
		t.Fatal("worktree directory was not created for existing branch")
	}
}

func TestCreateWorktree_Failure(t *testing.T) {
	// Point at a directory that isn't a git repo at all.
	fakeDir := t.TempDir()

	_, err := createWorktree(fakeDir, "any-branch")
	if err == nil {
		t.Fatal("expected error when repoDir is not a git repository")
	}
	if !strings.Contains(err.Error(), "git worktree add") {
		t.Errorf("error should mention 'git worktree add', got: %v", err)
	}
}

// ═══════════════════════════════════════════════════════════════════════════
// cleanupWorktree
// ═══════════════════════════════════════════════════════════════════════════

func TestCleanupWorktree_Success(t *testing.T) {
	repoDir := initGitRepo(t)

	// Create a worktree so there's something to remove.
	wtDir, err := createWorktree(repoDir, "to-remove")
	if err != nil {
		t.Fatalf("setup: createWorktree failed: %v", err)
	}

	// Sanity: the directory exists.
	if _, err := os.Stat(wtDir); os.IsNotExist(err) {
		t.Fatal("setup: worktree directory missing before cleanup")
	}

	// Should complete without panic; the success path prints a message.
	cleanupWorktree(repoDir, wtDir)

	// The worktree directory should be gone.
	if _, err := os.Stat(wtDir); !os.IsNotExist(err) {
		t.Error("worktree directory should be removed after cleanup")
	}
}

func TestCleanupWorktree_NonexistentDir(t *testing.T) {
	repoDir := initGitRepo(t)

	// Attempt to remove a worktree path that was never created.
	// This exercises the error/failure branch (git worktree remove fails).
	cleanupWorktree(repoDir, filepath.Join(t.TempDir(), "phantom"))
}
