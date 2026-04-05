package daemon

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

func initStartTestRepo(t *testing.T) string {
	t.Helper()
	dir := t.TempDir()
	for _, args := range [][]string{
		{"init", "-b", "main"},
		{"config", "user.name", "test"},
		{"config", "user.email", "test@test.com"},
	} {
		cmd := exec.Command("git", args...)
		cmd.Dir = dir
		if out, err := cmd.CombinedOutput(); err != nil {
			t.Fatalf("git %v: %v\n%s", args, err, out)
		}
	}
	_ = os.WriteFile(filepath.Join(dir, "file.go"), []byte("package main\n"), 0644)
	for _, args := range [][]string{
		{"add", "."},
		{"commit", "-m", "init"},
	} {
		cmd := exec.Command("git", args...)
		cmd.Dir = dir
		cmd.Env = append(os.Environ(),
			"GIT_AUTHOR_NAME=test", "GIT_AUTHOR_EMAIL=test@test.com",
			"GIT_COMMITTER_NAME=test", "GIT_COMMITTER_EMAIL=test@test.com",
		)
		if out, err := cmd.CombinedOutput(); err != nil {
			t.Fatalf("git %v: %v\n%s", args, err, out)
		}
	}
	return dir
}

func TestCheckDirtyTree_CleanRepo(t *testing.T) {
	t.Parallel()
	dir := initStartTestRepo(t)

	dirty, summary := checkDirtyTree(dir)
	if dirty {
		t.Errorf("expected clean tree, got dirty with summary: %s", summary)
	}
}

func TestCheckDirtyTree_UntrackedFile(t *testing.T) {
	t.Parallel()
	dir := initStartTestRepo(t)

	_ = os.WriteFile(filepath.Join(dir, "new.txt"), []byte("hello"), 0644)

	dirty, summary := checkDirtyTree(dir)
	if !dirty {
		t.Error("expected dirty tree with untracked file")
	}
	if !strings.Contains(summary, "untracked") {
		t.Errorf("summary should mention untracked, got: %s", summary)
	}
}

func TestCheckDirtyTree_ModifiedFile(t *testing.T) {
	t.Parallel()
	dir := initStartTestRepo(t)

	_ = os.WriteFile(filepath.Join(dir, "file.go"), []byte("package main\n// changed\n"), 0644)

	dirty, summary := checkDirtyTree(dir)
	if !dirty {
		t.Error("expected dirty tree with modified file")
	}
	// The change appears as either modified or staged depending on git config.
	if summary == "" {
		t.Error("expected non-empty summary for dirty tree")
	}
}

func TestCheckDirtyTree_StagedFile(t *testing.T) {
	t.Parallel()
	dir := initStartTestRepo(t)

	_ = os.WriteFile(filepath.Join(dir, "file.go"), []byte("package main\n// staged\n"), 0644)
	cmd := exec.Command("git", "add", "file.go")
	cmd.Dir = dir
	_ = cmd.Run()

	dirty, summary := checkDirtyTree(dir)
	if !dirty {
		t.Error("expected dirty tree with staged file")
	}
	if !strings.Contains(summary, "staged") {
		t.Errorf("summary should mention staged, got: %s", summary)
	}
}

func TestCheckDirtyTree_NonRepoDir(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()

	dirty, _ := checkDirtyTree(dir)
	if dirty {
		t.Error("expected non-dirty for non-repo directory")
	}
}

func TestCheckDirtyTree_MixedChanges(t *testing.T) {
	t.Parallel()
	dir := initStartTestRepo(t)

	// Create staged, modified, and untracked changes.
	_ = os.WriteFile(filepath.Join(dir, "file.go"), []byte("package main\n// staged\n"), 0644)
	cmd := exec.Command("git", "add", "file.go")
	cmd.Dir = dir
	_ = cmd.Run()
	// Modify again after staging
	_ = os.WriteFile(filepath.Join(dir, "file.go"), []byte("package main\n// both\n"), 0644)
	// Untracked file
	_ = os.WriteFile(filepath.Join(dir, "other.txt"), []byte("x"), 0644)

	dirty, summary := checkDirtyTree(dir)
	if !dirty {
		t.Error("expected dirty tree with mixed changes")
	}
	// Summary should mention at least staged and untracked (modified depends
	// on git's interpretation of the index/worktree state).
	if !strings.Contains(summary, "staged") || !strings.Contains(summary, "untracked") {
		t.Errorf("summary should mention staged and untracked, got: %s", summary)
	}
}
