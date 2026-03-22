package task

import (
	"io/fs"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"

	"github.com/dorkusprime/wolfcastle/internal/pipeline"
	"github.com/dorkusprime/wolfcastle/internal/project"
	"github.com/dorkusprime/wolfcastle/internal/testutil"
)

// ---------------------------------------------------------------------------
// findFilesByName — unit tests
// ---------------------------------------------------------------------------

func TestFindFilesByName_SkipsWolfcastleDir(t *testing.T) {
	root := t.TempDir()
	// Place target file inside .wolfcastle; it should be skipped.
	wcDir := filepath.Join(root, ".wolfcastle")
	_ = os.MkdirAll(wcDir, 0755)
	_ = os.WriteFile(filepath.Join(wcDir, "target.txt"), []byte("hidden"), 0644)

	matches := findFilesByName(root, "target.txt")
	if len(matches) != 0 {
		t.Errorf("expected no matches (file is inside .wolfcastle), got %v", matches)
	}
}

func TestFindFilesByName_SkipsGitDir(t *testing.T) {
	root := t.TempDir()
	gitDir := filepath.Join(root, ".git")
	_ = os.MkdirAll(gitDir, 0755)
	_ = os.WriteFile(filepath.Join(gitDir, "target.txt"), []byte("hidden"), 0644)

	matches := findFilesByName(root, "target.txt")
	if len(matches) != 0 {
		t.Errorf("expected no matches (file is inside .git), got %v", matches)
	}
}

func TestFindFilesByName_ReturnsMatches(t *testing.T) {
	root := t.TempDir()
	_ = os.MkdirAll(filepath.Join(root, "a", "b"), 0755)
	_ = os.WriteFile(filepath.Join(root, "a", "target.txt"), []byte("1"), 0644)
	_ = os.WriteFile(filepath.Join(root, "a", "b", "target.txt"), []byte("2"), 0644)

	matches := findFilesByName(root, "target.txt")
	if len(matches) != 2 {
		t.Fatalf("expected 2 matches, got %d: %v", len(matches), matches)
	}
}

func TestFindFilesByName_LimitsToFive(t *testing.T) {
	root := t.TempDir()
	for i := 0; i < 7; i++ {
		dir := filepath.Join(root, "d"+string(rune('a'+i)))
		_ = os.MkdirAll(dir, 0755)
		_ = os.WriteFile(filepath.Join(dir, "dup.txt"), []byte("x"), 0644)
	}

	matches := findFilesByName(root, "dup.txt")
	if len(matches) > 5 {
		t.Errorf("expected at most 5 matches, got %d", len(matches))
	}
}

func TestFindFilesByName_NoMatches(t *testing.T) {
	root := t.TempDir()
	_ = os.WriteFile(filepath.Join(root, "other.txt"), []byte("x"), 0644)

	matches := findFilesByName(root, "missing.txt")
	if len(matches) != 0 {
		t.Errorf("expected no matches, got %v", matches)
	}
}

func TestFindFilesByName_WalkErrorPermission(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("chmod restrictions have no effect on Windows")
	}
	root := t.TempDir()
	noAccess := filepath.Join(root, "noperm")
	_ = os.MkdirAll(noAccess, 0755)
	_ = os.WriteFile(filepath.Join(noAccess, "target.txt"), []byte("x"), 0644)
	// Place an accessible match too.
	_ = os.WriteFile(filepath.Join(root, "target.txt"), []byte("y"), 0644)
	_ = os.Chmod(noAccess, 0000)
	t.Cleanup(func() { _ = os.Chmod(noAccess, 0755) })

	matches := findFilesByName(root, "target.txt")
	// Should still find the root-level match without crashing.
	if len(matches) < 1 {
		t.Error("expected at least the accessible match")
	}
}

// ---------------------------------------------------------------------------
// isGlobPath — unit tests
// ---------------------------------------------------------------------------

func TestIsGlobPath(t *testing.T) {
	tests := []struct {
		path string
		want bool
	}{
		{"docs/report.md", false},
		{"docs/*.md", true},
		{"src/?oo.go", true},
		{"src/[ab].go", true},
		{"plain", false},
	}
	for _, tc := range tests {
		if got := isGlobPath(tc.path); got != tc.want {
			t.Errorf("isGlobPath(%q) = %v, want %v", tc.path, got, tc.want)
		}
	}
}

// ---------------------------------------------------------------------------
// deliverable command — uncovered branches
// ---------------------------------------------------------------------------

func TestTaskDeliverable_EmptyPath(t *testing.T) {
	env := newTestEnv(t)
	env.createLeafNode(t, "my-project", "My Project")

	env.RootCmd.SetArgs([]string{"task", "add", "--node", "my-project", "work"})
	_ = env.RootCmd.Execute()

	env.RootCmd.SetArgs([]string{"task", "deliverable", "   ", "--node", "my-project/task-0001"})
	err := env.RootCmd.Execute()
	if err == nil {
		t.Error("expected error for empty deliverable path")
	}
}

func TestTaskDeliverable_AbsolutePath(t *testing.T) {
	env := newTestEnv(t)
	env.createLeafNode(t, "my-project", "My Project")

	env.RootCmd.SetArgs([]string{"task", "add", "--node", "my-project", "work"})
	_ = env.RootCmd.Execute()

	env.RootCmd.SetArgs([]string{"task", "deliverable", "/etc/passwd", "--node", "my-project/task-0001"})
	err := env.RootCmd.Execute()
	if err == nil {
		t.Error("expected error for absolute deliverable path")
	}
	if err != nil && !strings.Contains(err.Error(), "relative") {
		t.Errorf("error should mention 'relative', got: %v", err)
	}
}

func TestTaskDeliverable_TraversalPath(t *testing.T) {
	env := newTestEnv(t)
	env.createLeafNode(t, "my-project", "My Project")

	env.RootCmd.SetArgs([]string{"task", "add", "--node", "my-project", "work"})
	_ = env.RootCmd.Execute()

	env.RootCmd.SetArgs([]string{"task", "deliverable", "../../../etc/passwd", "--node", "my-project/task-0001"})
	err := env.RootCmd.Execute()
	if err == nil {
		t.Error("expected error for path with '..' traversal")
	}
}

func TestTaskDeliverable_InvalidTaskAddress(t *testing.T) {
	env := newTestEnv(t)

	env.RootCmd.SetArgs([]string{"task", "deliverable", "docs/out.md", "--node", "single"})
	err := env.RootCmd.Execute()
	if err == nil {
		t.Error("expected error for non-task address")
	}
}

func TestTaskDeliverable_NoIdentity(t *testing.T) {
	env := newTestEnv(t)
	env.App.Identity = nil

	env.RootCmd.SetArgs([]string{"task", "deliverable", "docs/out.md", "--node", "my-project/task-0001"})
	err := env.RootCmd.Execute()
	if err == nil {
		t.Error("expected error when identity is nil")
	}
}

func TestTaskDeliverable_JSONOutput(t *testing.T) {
	env := newTestEnv(t)
	env.createLeafNode(t, "my-project", "My Project")

	env.RootCmd.SetArgs([]string{"task", "add", "--node", "my-project", "work"})
	_ = env.RootCmd.Execute()

	env.App.JSON = true
	defer func() { env.App.JSON = false }()

	env.RootCmd.SetArgs([]string{"task", "deliverable", "docs/out.md", "--node", "my-project/task-0001"})
	if err := env.RootCmd.Execute(); err != nil {
		t.Fatalf("deliverable (json) failed: %v", err)
	}
}

func TestTaskDeliverable_GlobPathSkipsStat(t *testing.T) {
	env := newTestEnv(t)
	env.createLeafNode(t, "my-project", "My Project")

	env.RootCmd.SetArgs([]string{"task", "add", "--node", "my-project", "work"})
	_ = env.RootCmd.Execute()

	// Glob paths should not trigger the file-existence warning path.
	env.RootCmd.SetArgs([]string{"task", "deliverable", "docs/*.md", "--node", "my-project/task-0001"})
	if err := env.RootCmd.Execute(); err != nil {
		t.Fatalf("deliverable with glob path failed: %v", err)
	}
}

func TestTaskDeliverable_NonexistentFileWithSuggestions(t *testing.T) {
	env := newTestEnv(t)
	env.createLeafNode(t, "my-project", "My Project")

	env.RootCmd.SetArgs([]string{"task", "add", "--node", "my-project", "work"})
	_ = env.RootCmd.Execute()

	// Create a file that will match the base name in a different directory.
	repoDir := filepath.Dir(env.App.Config.Root())
	_ = os.MkdirAll(filepath.Join(repoDir, "elsewhere"), 0755)
	_ = os.WriteFile(filepath.Join(repoDir, "elsewhere", "report.md"), []byte("x"), 0644)

	// Request a non-existent path whose base name exists elsewhere.
	env.RootCmd.SetArgs([]string{"task", "deliverable", "wrong/report.md", "--node", "my-project/task-0001"})
	if err := env.RootCmd.Execute(); err != nil {
		t.Fatalf("deliverable should succeed (warning is non-fatal): %v", err)
	}
}

func TestTaskDeliverable_NonexistentFileNoSuggestions(t *testing.T) {
	env := newTestEnv(t)
	env.createLeafNode(t, "my-project", "My Project")

	env.RootCmd.SetArgs([]string{"task", "add", "--node", "my-project", "work"})
	_ = env.RootCmd.Execute()

	// Use a base name that definitely doesn't exist anywhere.
	env.RootCmd.SetArgs([]string{"task", "deliverable", "nonexistent/zzz-never-exists.xyz", "--node", "my-project/task-0001"})
	if err := env.RootCmd.Execute(); err != nil {
		t.Fatalf("deliverable should succeed (warning is non-fatal): %v", err)
	}
}

func TestTaskDeliverable_ExistingFileNoWarning(t *testing.T) {
	env := newTestEnv(t)
	env.createLeafNode(t, "my-project", "My Project")

	env.RootCmd.SetArgs([]string{"task", "add", "--node", "my-project", "work"})
	_ = env.RootCmd.Execute()

	// Create the file so no warning is emitted.
	repoDir := filepath.Dir(env.App.Config.Root())
	_ = os.MkdirAll(filepath.Join(repoDir, "docs"), 0755)
	_ = os.WriteFile(filepath.Join(repoDir, "docs", "exists.md"), []byte("real"), 0644)

	env.RootCmd.SetArgs([]string{"task", "deliverable", "docs/exists.md", "--node", "my-project/task-0001"})
	if err := env.RootCmd.Execute(); err != nil {
		t.Fatalf("deliverable for existing file failed: %v", err)
	}
}

// ---------------------------------------------------------------------------
// add command — uncovered flag combinations
// ---------------------------------------------------------------------------

func TestTaskAdd_WithBody(t *testing.T) {
	env := newTestEnv(t)
	env.createLeafNode(t, "my-project", "My Project")

	env.RootCmd.SetArgs([]string{
		"task", "add", "--node", "my-project",
		"--body", "Detailed description of what to do",
		"add rate limiting",
	})
	if err := env.RootCmd.Execute(); err != nil {
		t.Fatalf("add with body: %v", err)
	}

	ns := env.loadNodeState(t, "my-project")
	for _, task := range ns.Tasks {
		if task.ID == "task-0001" {
			if task.Body != "Detailed description of what to do" {
				t.Errorf("expected body to be set, got %q", task.Body)
			}
			return
		}
	}
	t.Error("task-0001 not found")
}

func TestTaskAdd_WithType(t *testing.T) {
	env := newTestEnv(t)
	env.createLeafNode(t, "my-project", "My Project")

	env.RootCmd.SetArgs([]string{
		"task", "add", "--node", "my-project",
		"--type", "discovery",
		"research POS systems",
	})
	if err := env.RootCmd.Execute(); err != nil {
		t.Fatalf("add with type: %v", err)
	}

	ns := env.loadNodeState(t, "my-project")
	for _, task := range ns.Tasks {
		if task.ID == "task-0001" {
			if task.TaskType != "discovery" {
				t.Errorf("expected type 'discovery', got %q", task.TaskType)
			}
			return
		}
	}
	t.Error("task-0001 not found")
}

func TestTaskAdd_InvalidType(t *testing.T) {
	env := newTestEnv(t)
	env.createLeafNode(t, "my-project", "My Project")

	env.RootCmd.SetArgs([]string{
		"task", "add", "--node", "my-project",
		"--type", "bogus",
		"bad type task",
	})
	err := env.RootCmd.Execute()
	if err == nil {
		t.Error("expected error for invalid task type")
	}
	if err != nil && !strings.Contains(err.Error(), "invalid task type") {
		t.Errorf("error should mention 'invalid task type', got: %v", err)
	}
}

func TestTaskAdd_WithAllFlags(t *testing.T) {
	env := newTestEnv(t)
	env.createLeafNode(t, "my-project", "My Project")

	env.RootCmd.SetArgs([]string{
		"task", "add", "--node", "my-project",
		"--body", "Full description",
		"--type", "implementation",
		"--class", "coding/go",
		"--deliverable", "cmd/api/handler.go",
		"--constraint", "no external deps",
		"--acceptance", "tests pass",
		"--reference", "RFC-9999",
		"--integration", "feeds into auth module",
		"implement everything",
	})
	if err := env.RootCmd.Execute(); err != nil {
		t.Fatalf("add with all flags: %v", err)
	}

	ns := env.loadNodeState(t, "my-project")
	for _, task := range ns.Tasks {
		if task.ID != "task-0001" {
			continue
		}
		if task.Body != "Full description" {
			t.Errorf("body: got %q", task.Body)
		}
		if task.TaskType != "implementation" {
			t.Errorf("type: got %q", task.TaskType)
		}
		if task.Class != "coding/go" {
			t.Errorf("class: got %q", task.Class)
		}
		if len(task.Deliverables) != 1 || task.Deliverables[0] != "cmd/api/handler.go" {
			t.Errorf("deliverables: got %v", task.Deliverables)
		}
		if len(task.Constraints) != 1 || task.Constraints[0] != "no external deps" {
			t.Errorf("constraints: got %v", task.Constraints)
		}
		if len(task.AcceptanceCriteria) != 1 || task.AcceptanceCriteria[0] != "tests pass" {
			t.Errorf("acceptance: got %v", task.AcceptanceCriteria)
		}
		if len(task.References) != 1 || task.References[0] != "RFC-9999" {
			t.Errorf("references: got %v", task.References)
		}
		if task.Integration != "feeds into auth module" {
			t.Errorf("integration: got %q", task.Integration)
		}
		return
	}
	t.Error("task-0001 not found")
}

func TestTaskAdd_WithParent(t *testing.T) {
	env := newTestEnv(t)
	env.createLeafNode(t, "my-project", "My Project")

	// Add a parent task.
	env.RootCmd.SetArgs([]string{"task", "add", "--node", "my-project", "parent task"})
	_ = env.RootCmd.Execute()

	// Claim the parent so it's in_progress (required for decomposition).
	env.RootCmd.SetArgs([]string{"task", "claim", "--node", "my-project/task-0001"})
	_ = env.RootCmd.Execute()

	// Add a child task.
	env.RootCmd.SetArgs([]string{
		"task", "add", "--node", "my-project",
		"--parent", "task-0001",
		"child task",
	})
	if err := env.RootCmd.Execute(); err != nil {
		t.Fatalf("add with parent: %v", err)
	}

	ns := env.loadNodeState(t, "my-project")
	found := false
	for _, task := range ns.Tasks {
		if task.ID == "task-0001.0001" {
			found = true
			break
		}
	}
	if !found {
		t.Error("expected child task task-0001.0001")
	}
}

func TestTaskAdd_JSONOutputWithDeliverables(t *testing.T) {
	env := newTestEnv(t)
	env.App.JSON = true
	defer func() { env.App.JSON = false }()

	env.createLeafNode(t, "my-project", "My Project")

	env.RootCmd.SetArgs([]string{
		"task", "add", "--node", "my-project",
		"--deliverable", "out.md",
		"task with deliverables",
	})
	if err := env.RootCmd.Execute(); err != nil {
		t.Fatalf("add (json with deliverables) failed: %v", err)
	}
}

// ---------------------------------------------------------------------------
// writeTaskMD — coverage for body inclusion and error path
// ---------------------------------------------------------------------------

// newTaskPrompts creates a PromptRepository with the task template seeded.
func newTaskPrompts(t *testing.T) *pipeline.PromptRepository {
	t.Helper()
	env := testutil.NewEnvironment(t)
	sub, err := fs.Sub(project.Templates, "templates")
	if err != nil {
		t.Fatalf("extracting templates sub-FS: %v", err)
	}
	data, err := fs.ReadFile(sub, "artifacts/task.md.tmpl")
	if err != nil {
		t.Fatalf("reading task template: %v", err)
	}
	env.WithTemplate("artifacts/task.md.tmpl", string(data))
	return env.ToAppFields().Prompts
}

func TestWriteTaskMD_WithBody(t *testing.T) {
	prompts := newTaskPrompts(t)
	dir := t.TempDir()
	writeTaskMD(prompts, dir, "task-0001", "Test Title", "Some body text")

	data, err := os.ReadFile(filepath.Join(dir, "task-0001.md"))
	if err != nil {
		t.Fatalf("reading task md: %v", err)
	}
	content := string(data)
	if !strings.Contains(content, "# Test Title") {
		t.Error("expected title in markdown")
	}
	if !strings.Contains(content, "Some body text") {
		t.Error("expected body in markdown")
	}
}

func TestWriteTaskMD_NoBody(t *testing.T) {
	prompts := newTaskPrompts(t)
	dir := t.TempDir()
	writeTaskMD(prompts, dir, "task-0002", "Title Only", "")

	data, err := os.ReadFile(filepath.Join(dir, "task-0002.md"))
	if err != nil {
		t.Fatalf("reading task md: %v", err)
	}
	content := string(data)
	if !strings.Contains(content, "# Title Only") {
		t.Error("expected title in markdown")
	}
	// With empty body, the file should only have the title line.
	if strings.Contains(content, "\n\n") {
		t.Error("expected no body section for empty body")
	}
}

func TestWriteTaskMD_WhitespaceOnlyBody(t *testing.T) {
	prompts := newTaskPrompts(t)
	dir := t.TempDir()
	writeTaskMD(prompts, dir, "task-0003", "Title", "   \n  ")

	data, err := os.ReadFile(filepath.Join(dir, "task-0003.md"))
	if err != nil {
		t.Fatalf("reading task md: %v", err)
	}
	// Whitespace-only body should be treated as empty.
	if strings.Contains(string(data), "\n\n") {
		t.Error("whitespace-only body should be omitted")
	}
}

func TestWriteTaskMD_NonexistentDir(t *testing.T) {
	prompts := newTaskPrompts(t)
	// Writing to a non-existent directory is silently ignored.
	writeTaskMD(prompts, "/nonexistent/path/that/does/not/exist", "task-0001", "Title", "body")
	// No panic or error; the function is best-effort.
}

// ---------------------------------------------------------------------------
// amend command — remaining uncovered branches
// ---------------------------------------------------------------------------

func TestTaskAmend_NoIdentity(t *testing.T) {
	env := newTestEnv(t)
	env.App.Identity = nil

	env.RootCmd.SetArgs([]string{"task", "amend", "--node", "my-project/task-0001", "--body", "new"})
	err := env.RootCmd.Execute()
	if err == nil {
		t.Error("expected error when identity is nil")
	}
}

func TestTaskAmend_JSONOutput(t *testing.T) {
	env := newTestEnv(t)
	env.createLeafNode(t, "my-project", "My Project")

	env.RootCmd.SetArgs([]string{"task", "add", "--node", "my-project", "some task"})
	_ = env.RootCmd.Execute()

	env.App.JSON = true
	defer func() { env.App.JSON = false }()

	env.RootCmd.SetArgs([]string{"task", "amend", "--node", "my-project/task-0001", "--body", "updated"})
	if err := env.RootCmd.Execute(); err != nil {
		t.Fatalf("amend (json) failed: %v", err)
	}
}

func TestTaskAmend_Integration(t *testing.T) {
	env := newTestEnv(t)
	env.createLeafNode(t, "my-project", "My Project")

	env.RootCmd.SetArgs([]string{"task", "add", "--node", "my-project", "integrateable"})
	_ = env.RootCmd.Execute()

	env.RootCmd.SetArgs([]string{
		"task", "amend", "--node", "my-project/task-0001",
		"--integration", "connects to auth module",
	})
	if err := env.RootCmd.Execute(); err != nil {
		t.Fatalf("amend integration: %v", err)
	}

	ns := env.loadNodeState(t, "my-project")
	for _, task := range ns.Tasks {
		if task.ID == "task-0001" {
			if task.Integration != "connects to auth module" {
				t.Errorf("expected integration field set, got %q", task.Integration)
			}
			return
		}
	}
	t.Error("task-0001 not found")
}

func TestTaskAmend_InvalidAddress(t *testing.T) {
	env := newTestEnv(t)

	env.RootCmd.SetArgs([]string{"task", "amend", "--node", "single", "--body", "nope"})
	err := env.RootCmd.Execute()
	if err == nil {
		t.Error("expected error for non-task address")
	}
}
