package git

import (
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"testing"
)

// initRepo creates a git repo in dir with "main" as the default branch.
// It configures local user.name and user.email so tests never depend on
// the host machine's global git config.
func initRepo(t *testing.T, dir string) {
	t.Helper()
	run(t, dir, "git", "init", "-b", "main")
	run(t, dir, "git", "config", "user.name", "Test")
	run(t, dir, "git", "config", "user.email", "test@localhost")
}

// commitFile creates (or overwrites) a file and commits it.
func commitFile(t *testing.T, dir, name, content, msg string) {
	t.Helper()
	if err := os.WriteFile(filepath.Join(dir, name), []byte(content), 0o644); err != nil {
		t.Fatalf("writing %s: %v", name, err)
	}
	run(t, dir, "git", "add", name)
	run(t, dir, "git", "commit", "-m", msg)
}

// run executes a command in the given directory, failing the test on error.
func run(t *testing.T, dir string, name string, args ...string) {
	t.Helper()
	cmd := exec.Command(name, args...)
	cmd.Dir = dir
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("%s %v failed: %v\n%s", name, args, err, out)
	}
}

func TestService_IsRepo(t *testing.T) {
	t.Parallel()

	repoDir := t.TempDir()
	initRepo(t, repoDir)
	svc := NewService(repoDir)

	if !svc.IsRepo() {
		t.Error("expected IsRepo=true for initialized repo")
	}

	plainDir := t.TempDir()
	notRepo := NewService(plainDir)
	if notRepo.IsRepo() {
		t.Error("expected IsRepo=false for plain directory")
	}
}

func TestService_CurrentBranch(t *testing.T) {
	t.Parallel()

	repoDir := t.TempDir()
	initRepo(t, repoDir)
	commitFile(t, repoDir, "init.txt", "hello", "initial commit")

	svc := NewService(repoDir)
	branch, err := svc.CurrentBranch()
	if err != nil {
		t.Fatalf("CurrentBranch: %v", err)
	}
	if branch != "main" {
		t.Errorf("expected branch 'main', got %q", branch)
	}

	// Create and checkout a new branch.
	run(t, repoDir, "git", "checkout", "-b", "feature-x")

	branch, err = svc.CurrentBranch()
	if err != nil {
		t.Fatalf("CurrentBranch after checkout: %v", err)
	}
	if branch != "feature-x" {
		t.Errorf("expected branch 'feature-x', got %q", branch)
	}
}

func TestService_CurrentBranch_EmptyRepo(t *testing.T) {
	t.Parallel()

	repoDir := t.TempDir()
	initRepo(t, repoDir)
	// No commits: HEAD can't resolve via rev-parse.

	svc := NewService(repoDir)
	branch, err := svc.CurrentBranch()
	if err != nil {
		t.Fatalf("CurrentBranch on empty repo: %v", err)
	}
	// The fallback should return the default branch name ("main" with -b main).
	if branch != "main" {
		t.Errorf("expected fallback branch 'main', got %q", branch)
	}
}

func TestService_HEAD(t *testing.T) {
	t.Parallel()

	repoDir := t.TempDir()
	initRepo(t, repoDir)
	commitFile(t, repoDir, "init.txt", "hello", "initial commit")

	svc := NewService(repoDir)
	sha := svc.HEAD()

	hexPattern := regexp.MustCompile(`^[0-9a-f]{40}$`)
	if !hexPattern.MatchString(sha) {
		t.Errorf("expected 40-char hex SHA, got %q", sha)
	}

	// Non-repo should return empty string.
	plainDir := t.TempDir()
	noRepo := NewService(plainDir)
	if got := noRepo.HEAD(); got != "" {
		t.Errorf("expected empty HEAD for non-repo, got %q", got)
	}
}

func TestService_IsDirty(t *testing.T) {
	t.Parallel()

	repoDir := t.TempDir()
	initRepo(t, repoDir)
	commitFile(t, repoDir, "init.txt", "hello", "initial commit")

	svc := NewService(repoDir)

	if svc.IsDirty() {
		t.Error("expected clean working tree after commit")
	}

	// Create an untracked file: should be dirty.
	if err := os.WriteFile(filepath.Join(repoDir, "untracked.txt"), []byte("x"), 0o644); err != nil {
		t.Fatal(err)
	}
	if !svc.IsDirty() {
		t.Error("expected dirty after adding untracked file")
	}

	// The same file should be invisible when its prefix is excluded.
	if svc.IsDirty("untracked") {
		t.Error("expected clean when the dirty file's prefix is excluded")
	}

	// A file inside a subdirectory, excluded by directory prefix.
	subdir := filepath.Join(repoDir, ".wolfcastle")
	if err := os.MkdirAll(subdir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(subdir, "state.json"), []byte("{}"), 0o644); err != nil {
		t.Fatal(err)
	}
	// Both untracked.txt and .wolfcastle/state.json are dirty, but excluding both prefixes
	// should yield clean.
	if svc.IsDirty("untracked", ".wolfcastle/") {
		t.Error("expected clean when all dirty paths are excluded")
	}
}

func TestService_CurrentBranch_NonRepo(t *testing.T) {
	t.Parallel()

	plainDir := t.TempDir()
	svc := NewService(plainDir)

	_, err := svc.CurrentBranch()
	if err == nil {
		t.Error("expected error from CurrentBranch on non-repo directory")
	}
}

func TestService_IsDirty_NonRepo(t *testing.T) {
	t.Parallel()

	plainDir := t.TempDir()
	svc := NewService(plainDir)

	if svc.IsDirty() {
		t.Error("expected IsDirty=false for non-repo directory")
	}
}

func TestService_IsDirty_Rename(t *testing.T) {
	t.Parallel()

	repoDir := t.TempDir()
	initRepo(t, repoDir)
	commitFile(t, repoDir, "old-name.txt", "content", "add file")

	// Stage a rename so porcelain shows "R  old-name.txt -> new-name.txt".
	run(t, repoDir, "git", "mv", "old-name.txt", "new-name.txt")

	svc := NewService(repoDir)

	// The rename should make the tree dirty.
	if !svc.IsDirty() {
		t.Error("expected dirty after staged rename")
	}

	// Excluding the new name (the parsed rename target) should hide it.
	if svc.IsDirty("new-name") {
		t.Error("expected clean when renamed path's new name is excluded")
	}

	// Excluding the old name should NOT hide it (parser extracts the new path).
	if !svc.IsDirty("old-name") {
		t.Error("expected dirty when only old rename path is excluded")
	}
}

func TestService_HasProgress(t *testing.T) {
	t.Parallel()

	repoDir := t.TempDir()
	initRepo(t, repoDir)
	commitFile(t, repoDir, "init.txt", "hello", "initial commit")

	svc := NewService(repoDir)
	baseline := svc.HEAD()

	// No changes since baseline: no progress.
	if svc.HasProgress(baseline) {
		t.Error("expected no progress immediately after recording HEAD")
	}

	// New commit moves HEAD: progress.
	commitFile(t, repoDir, "second.txt", "world", "second commit")
	if !svc.HasProgress(baseline) {
		t.Error("expected progress after new commit")
	}

	// Dirty-but-no-new-commits case: progress (dirty files outside .wolfcastle/).
	newBaseline := svc.HEAD()
	if err := os.WriteFile(filepath.Join(repoDir, "dirty.txt"), []byte("d"), 0o644); err != nil {
		t.Fatal(err)
	}
	if !svc.HasProgress(newBaseline) {
		t.Error("expected progress when working tree is dirty (outside .wolfcastle/)")
	}
}

func TestService_HasProgressScoped(t *testing.T) {
	t.Parallel()

	repoDir := t.TempDir()
	initRepo(t, repoDir)
	commitFile(t, repoDir, "init.txt", "hello", "initial commit")

	svc := NewService(repoDir)

	// Clean tree, no scope files changed: no progress.
	if svc.HasProgressScoped("ignored", []string{"foo.go"}) {
		t.Error("expected no progress on clean tree")
	}

	// Create an untracked file at root level (porcelain shows "?? foo.go").
	if err := os.WriteFile(filepath.Join(repoDir, "foo.go"), []byte("package foo"), 0o644); err != nil {
		t.Fatal(err)
	}
	if !svc.HasProgressScoped("ignored", []string{"foo.go"}) {
		t.Error("expected progress when scoped file is untracked")
	}

	// Same dirty file, but scope doesn't include it: no progress.
	if svc.HasProgressScoped("ignored", []string{"bar.go"}) {
		t.Error("expected no progress when dirty file is outside scope")
	}

	// Untracked directory shows as "internal/" in porcelain; directory prefix
	// scope should match it.
	if err := os.MkdirAll(filepath.Join(repoDir, "internal"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(repoDir, "internal", "main.go"), []byte("package main"), 0o644); err != nil {
		t.Fatal(err)
	}
	if !svc.HasProgressScoped("ignored", []string{"internal/"}) {
		t.Error("expected progress with directory prefix scope matching untracked dir")
	}

	// sinceCommit is deliberately unused; verify HEAD movement alone
	// doesn't trigger progress when no scoped files are dirty.
	_ = os.Remove(filepath.Join(repoDir, "foo.go"))
	_ = os.RemoveAll(filepath.Join(repoDir, "internal"))
	baseline := svc.HEAD()
	commitFile(t, repoDir, "unrelated.txt", "data", "unrelated commit")
	if svc.HasProgressScoped(baseline, []string{"unrelated.txt"}) {
		t.Error("expected no progress: HEAD moved but scoped file is committed and clean")
	}
}

func TestService_HasProgressScoped_NonRepo(t *testing.T) {
	t.Parallel()

	plainDir := t.TempDir()
	svc := NewService(plainDir)

	// Non-repo returns true (conservative assumption).
	if !svc.HasProgressScoped("", []string{"any.go"}) {
		t.Error("expected true for non-repo (conservative)")
	}
}

func TestService_HasProgressScoped_Rename(t *testing.T) {
	t.Parallel()

	repoDir := t.TempDir()
	initRepo(t, repoDir)
	commitFile(t, repoDir, "old.go", "package old", "add old.go")

	// Stage a rename.
	run(t, repoDir, "git", "mv", "old.go", "new.go")

	svc := NewService(repoDir)

	// Scope includes the new name: should detect progress.
	if !svc.HasProgressScoped("", []string{"new.go"}) {
		t.Error("expected progress when renamed file's new name is in scope")
	}

	// Scope includes only the old name: no progress (parser extracts new path).
	if svc.HasProgressScoped("", []string{"old.go"}) {
		t.Error("expected no progress when only old rename path is in scope")
	}
}

func TestService_HasProgressScoped_MultipleScopes(t *testing.T) {
	t.Parallel()

	repoDir := t.TempDir()
	initRepo(t, repoDir)
	commitFile(t, repoDir, "main.go", "package main", "init")
	commitFile(t, repoDir, "util.go", "package main", "add util")
	commitFile(t, repoDir, "readme.md", "# hello", "add readme")

	svc := NewService(repoDir)

	// Only out-of-scope file is dirty: should return false.
	if err := os.WriteFile(filepath.Join(repoDir, "readme.md"), []byte("# updated"), 0o644); err != nil {
		t.Fatal(err)
	}
	if svc.HasProgressScoped("", []string{"main.go", "util.go"}) {
		t.Error("expected no progress when only out-of-scope file is dirty")
	}

	// Now stage a modification to an in-scope file too: should return true.
	if err := os.WriteFile(filepath.Join(repoDir, "util.go"), []byte("package main\n// changed"), 0o644); err != nil {
		t.Fatal(err)
	}
	run(t, repoDir, "git", "add", "util.go")
	if !svc.HasProgressScoped("", []string{"main.go", "util.go"}) {
		t.Error("expected progress when one of multiple scoped files is dirty")
	}

	// Clean up and verify multiple scope entries all out-of-scope still returns false.
	run(t, repoDir, "git", "checkout", "--", "readme.md")
	run(t, repoDir, "git", "reset", "HEAD", "util.go")
	run(t, repoDir, "git", "checkout", "--", "util.go")
	if err := os.WriteFile(filepath.Join(repoDir, "other.txt"), []byte("x"), 0o644); err != nil {
		t.Fatal(err)
	}
	if svc.HasProgressScoped("", []string{"main.go", "util.go", "lib/"}) {
		t.Error("expected no progress with multiple scope entries when only unscoped file is dirty")
	}
}

func TestService_HasProgressScoped_StagedModification(t *testing.T) {
	t.Parallel()

	repoDir := t.TempDir()
	initRepo(t, repoDir)
	commitFile(t, repoDir, "main.go", "package main", "init")

	// Modify and stage.
	if err := os.WriteFile(filepath.Join(repoDir, "main.go"), []byte("package main\n// changed"), 0o644); err != nil {
		t.Fatal(err)
	}
	run(t, repoDir, "git", "add", "main.go")

	svc := NewService(repoDir)

	if !svc.HasProgressScoped("", []string{"main.go"}) {
		t.Error("expected progress for staged modification")
	}
	if svc.HasProgressScoped("", []string{"other.go"}) {
		t.Error("expected no progress for unrelated scope")
	}
}

func TestService_Worktree(t *testing.T) {
	t.Parallel()

	repoDir := t.TempDir()
	initRepo(t, repoDir)
	commitFile(t, repoDir, "init.txt", "hello", "initial commit")

	svc := NewService(repoDir)

	wtPath := filepath.Join(t.TempDir(), "my-worktree")

	if err := svc.CreateWorktree(wtPath, "wt-branch"); err != nil {
		t.Fatalf("CreateWorktree: %v", err)
	}

	// The worktree directory should exist and itself be a git checkout.
	if _, err := os.Stat(filepath.Join(wtPath, ".git")); err != nil {
		t.Errorf("worktree .git missing: %v", err)
	}

	wtSvc := NewService(wtPath)
	if !wtSvc.IsRepo() {
		t.Error("worktree should report as a repo")
	}
	branch, err := wtSvc.CurrentBranch()
	if err != nil {
		t.Fatalf("worktree CurrentBranch: %v", err)
	}
	if branch != "wt-branch" {
		t.Errorf("expected worktree branch 'wt-branch', got %q", branch)
	}

	// Remove the worktree.
	if err := svc.RemoveWorktree(wtPath); err != nil {
		t.Fatalf("RemoveWorktree: %v", err)
	}
	if _, err := os.Stat(wtPath); !os.IsNotExist(err) {
		t.Errorf("expected worktree directory to be removed, but stat returned: %v", err)
	}
}
