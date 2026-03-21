package daemon

import (
	"os"
	"os/exec"
	"path/filepath"
	"testing"

	"github.com/dorkusprime/wolfcastle/internal/git"
	"github.com/dorkusprime/wolfcastle/internal/state"
)

func TestCheckDeliverables_NoDeliverables(t *testing.T) {
	t.Parallel()
	ns := &state.NodeState{
		Tasks: []state.Task{
			{ID: "task-0001", Description: "no deliverables"},
		},
	}
	missing := checkDeliverables(t.TempDir(), ns, "task-0001")
	if len(missing) != 0 {
		t.Errorf("expected no missing deliverables, got %v", missing)
	}
}

func TestCheckDeliverables_AllExist(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	_ = os.MkdirAll(filepath.Join(dir, "docs"), 0755)
	_ = os.WriteFile(filepath.Join(dir, "docs/report.md"), []byte("content"), 0644)
	_ = os.WriteFile(filepath.Join(dir, "docs/summary.md"), []byte("more content"), 0644)

	ns := &state.NodeState{
		Tasks: []state.Task{
			{
				ID:           "task-0001",
				Description:  "with deliverables",
				Deliverables: []string{"docs/report.md", "docs/summary.md"},
			},
		},
	}
	missing := checkDeliverables(dir, ns, "task-0001")
	if len(missing) != 0 {
		t.Errorf("expected no missing deliverables, got %v", missing)
	}
}

func TestCheckDeliverables_MissingFile(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()

	ns := &state.NodeState{
		Tasks: []state.Task{
			{
				ID:           "task-0001",
				Description:  "missing file",
				Deliverables: []string{"docs/nonexistent.md"},
			},
		},
	}
	missing := checkDeliverables(dir, ns, "task-0001")
	if len(missing) != 1 || missing[0] != "docs/nonexistent.md" {
		t.Errorf("expected [docs/nonexistent.md], got %v", missing)
	}
}

func TestCheckDeliverables_EmptyFile(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	_ = os.MkdirAll(filepath.Join(dir, "docs"), 0755)
	_ = os.WriteFile(filepath.Join(dir, "docs/empty.md"), []byte(""), 0644)

	ns := &state.NodeState{
		Tasks: []state.Task{
			{
				ID:           "task-0001",
				Description:  "empty file",
				Deliverables: []string{"docs/empty.md"},
			},
		},
	}
	missing := checkDeliverables(dir, ns, "task-0001")
	if len(missing) != 1 || missing[0] != "docs/empty.md" {
		t.Errorf("expected [docs/empty.md], got %v", missing)
	}
}

func TestCheckDeliverables_MixedResults(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	_ = os.MkdirAll(filepath.Join(dir, "docs"), 0755)
	_ = os.WriteFile(filepath.Join(dir, "docs/exists.md"), []byte("content"), 0644)

	ns := &state.NodeState{
		Tasks: []state.Task{
			{
				ID:           "task-0001",
				Description:  "mixed",
				Deliverables: []string{"docs/exists.md", "docs/missing.md"},
			},
		},
	}
	missing := checkDeliverables(dir, ns, "task-0001")
	if len(missing) != 1 || missing[0] != "docs/missing.md" {
		t.Errorf("expected [docs/missing.md], got %v", missing)
	}
}

func TestCheckDeliverables_TaskNotFound(t *testing.T) {
	t.Parallel()
	ns := &state.NodeState{
		Tasks: []state.Task{
			{ID: "task-0001", Description: "other task"},
		},
	}
	missing := checkDeliverables(t.TempDir(), ns, "task-9999")
	if len(missing) != 0 {
		t.Errorf("expected no missing for nonexistent task, got %v", missing)
	}
}

// ═══════════════════════════════════════════════════════════════════════════
// Glob pattern deliverables
// ═══════════════════════════════════════════════════════════════════════════

func TestCheckDeliverables_GlobMatch(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	_ = os.WriteFile(filepath.Join(dir, "helloworld-2026-01-01.txt"), []byte("content"), 0644)

	ns := &state.NodeState{
		Tasks: []state.Task{
			{
				ID:           "task-0001",
				Deliverables: []string{"helloworld-*.txt"},
			},
		},
	}
	missing := checkDeliverables(dir, ns, "task-0001")
	if len(missing) != 0 {
		t.Errorf("glob should match existing file, got missing: %v", missing)
	}
}

func TestCheckDeliverables_GlobNoMatch(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()

	ns := &state.NodeState{
		Tasks: []state.Task{
			{
				ID:           "task-0001",
				Deliverables: []string{"helloworld-*.txt"},
			},
		},
	}
	missing := checkDeliverables(dir, ns, "task-0001")
	if len(missing) != 1 {
		t.Errorf("glob with no matches should be missing, got: %v", missing)
	}
}

func TestCheckDeliverables_GlobMatchesEmptyFile(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	_ = os.WriteFile(filepath.Join(dir, "report-v1.md"), []byte(""), 0644)

	ns := &state.NodeState{
		Tasks: []state.Task{
			{
				ID:           "task-0001",
				Deliverables: []string{"report-*.md"},
			},
		},
	}
	missing := checkDeliverables(dir, ns, "task-0001")
	if len(missing) != 1 {
		t.Errorf("glob matching only empty files should be missing, got: %v", missing)
	}
}

func TestCheckDeliverables_GlobWithLiteralMix(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	_ = os.WriteFile(filepath.Join(dir, "output-2026.csv"), []byte("data"), 0644)
	_ = os.WriteFile(filepath.Join(dir, "summary.md"), []byte("summary"), 0644)

	ns := &state.NodeState{
		Tasks: []state.Task{
			{
				ID:           "task-0001",
				Deliverables: []string{"output-*.csv", "summary.md"},
			},
		},
	}
	missing := checkDeliverables(dir, ns, "task-0001")
	if len(missing) != 0 {
		t.Errorf("both glob and literal should pass, got missing: %v", missing)
	}
}

// ═══════════════════════════════════════════════════════════════════════════
// git.HasProgress
// ═══════════════════════════════════════════════════════════════════════════

func TestCheckGitProgress_DirtyWorktree(t *testing.T) {
	t.Parallel()
	dir := initGitRepo(t)
	head := git.NewService(dir).HEAD()
	_ = os.WriteFile(filepath.Join(dir, "new-file.txt"), []byte("changes"), 0644)

	if !git.NewService(dir).HasProgress(head) {
		t.Error("dirty worktree should report progress")
	}
}

func TestCheckGitProgress_CleanWorktree(t *testing.T) {
	t.Parallel()
	dir := initGitRepo(t)
	head := git.NewService(dir).HEAD()

	if git.NewService(dir).HasProgress(head) {
		t.Error("clean worktree with same HEAD should report no progress")
	}
}

func TestCheckGitProgress_NewCommit(t *testing.T) {
	t.Parallel()
	dir := initGitRepo(t)
	head := git.NewService(dir).HEAD()

	// Make a new commit (simulates model committing its work)
	_ = os.WriteFile(filepath.Join(dir, "new-file.txt"), []byte("changes"), 0644)
	gitRun(t, dir, "add", ".")
	gitRun(t, dir, "commit", "-m", "model work")

	// Working tree is clean but HEAD moved
	if !git.NewService(dir).HasProgress(head) {
		t.Error("new commit should report progress even with clean tree")
	}
}

func TestCheckGitProgress_NotGitRepo(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()

	if !git.NewService(dir).HasProgress("") {
		t.Error("non-git directory should assume progress")
	}
}

func TestCheckGitProgress_EmptyBeforeHEAD(t *testing.T) {
	t.Parallel()
	dir := initGitRepo(t)
	_ = os.WriteFile(filepath.Join(dir, "new-file.txt"), []byte("changes"), 0644)

	// Empty beforeHEAD skips commit check, falls through to status check
	if !git.NewService(dir).HasProgress("") {
		t.Error("dirty worktree with empty beforeHEAD should report progress")
	}
}

// gitRun executes a git command in the given directory.
func gitRun(t *testing.T, dir string, args ...string) {
	t.Helper()
	cmd := exec.Command("git", args...)
	cmd.Dir = dir
	cmd.Env = append(os.Environ(),
		"GIT_AUTHOR_NAME=test",
		"GIT_AUTHOR_EMAIL=test@test.com",
		"GIT_COMMITTER_NAME=test",
		"GIT_COMMITTER_EMAIL=test@test.com",
	)
	if out, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("git %v failed: %v\n%s", args, err, out)
	}
}

// initGitRepo creates a temporary git repo with one commit.
func initGitRepo(t *testing.T) string {
	t.Helper()
	dir := t.TempDir()
	run := func(args ...string) {
		t.Helper()
		cmd := exec.Command("git", args...)
		cmd.Dir = dir
		cmd.Env = append(os.Environ(),
			"GIT_AUTHOR_NAME=test",
			"GIT_AUTHOR_EMAIL=test@test.com",
			"GIT_COMMITTER_NAME=test",
			"GIT_COMMITTER_EMAIL=test@test.com",
		)
		if out, err := cmd.CombinedOutput(); err != nil {
			t.Fatalf("git %v failed: %v\n%s", args, err, out)
		}
	}
	run("init")
	run("config", "user.email", "test@test.com")
	run("config", "user.name", "test")
	_ = os.WriteFile(filepath.Join(dir, "README.md"), []byte("init"), 0644)
	run("add", ".")
	run("commit", "-m", "init")
	return dir
}

// ═══════════════════════════════════════════════════════════════════════════
// globRecursive
// ═══════════════════════════════════════════════════════════════════════════

func TestGlobRecursive_MatchesSubdirectories(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	_ = os.MkdirAll(filepath.Join(dir, "cmd", "task"), 0755)
	_ = os.WriteFile(filepath.Join(dir, "cmd", "root.go"), []byte("package cmd"), 0644)
	_ = os.WriteFile(filepath.Join(dir, "cmd", "task", "add.go"), []byte("package task"), 0644)

	matches := globRecursive(filepath.Join(dir, "cmd", "*.go"))
	if len(matches) < 2 {
		t.Errorf("expected at least 2 matches (root.go + task/add.go), got %d: %v", len(matches), matches)
	}
	foundSubdir := false
	for _, m := range matches {
		if filepath.Base(m) == "add.go" {
			foundSubdir = true
		}
	}
	if !foundSubdir {
		t.Error("globRecursive should find files in subdirectories")
	}
}

func TestGlobRecursive_NoWildcard(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	_ = os.WriteFile(filepath.Join(dir, "exact.txt"), []byte("content"), 0644)

	matches := globRecursive(filepath.Join(dir, "exact.txt"))
	if len(matches) != 1 {
		t.Errorf("exact path should return 1 match, got %d", len(matches))
	}
}

func TestGlobRecursive_NoMatches(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()

	matches := globRecursive(filepath.Join(dir, "*.xyz"))
	if len(matches) != 0 {
		t.Errorf("expected 0 matches, got %d", len(matches))
	}
}

// ═══════════════════════════════════════════════════════════════════════════
// isGlob
// ═══════════════════════════════════════════════════════════════════════════

func TestIsGlob(t *testing.T) {
	t.Parallel()
	cases := []struct {
		path string
		want bool
	}{
		{"hello.txt", false},
		{"docs/report.md", false},
		{"helloworld-*.txt", true},
		{"output-?.csv", true},
		{"data[0-9].json", true},
	}
	for _, tc := range cases {
		if got := isGlob(tc.path); got != tc.want {
			t.Errorf("isGlob(%q) = %v, want %v", tc.path, got, tc.want)
		}
	}
}
