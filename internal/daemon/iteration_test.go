package daemon

import (
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/dorkusprime/wolfcastle/internal/config"
	"github.com/dorkusprime/wolfcastle/internal/git"
	"github.com/dorkusprime/wolfcastle/internal/logging"
	"github.com/dorkusprime/wolfcastle/internal/state"
)

// ═══════════════════════════════════════════════════════════════════════════
// commitAfterIteration — config flag combinations
// ═══════════════════════════════════════════════════════════════════════════

func TestCommitAfterIteration_AutoCommitDisabled(t *testing.T) {
	t.Parallel()
	repoDir := initGitRepo(t)
	modifyTrackedFile(t, repoDir)

	logger := iterTestLogger(t)
	defer logger.Close()

	cfg := testGitCfg()
	cfg.AutoCommit = false

	beforeLog := gitLog(t, repoDir)
	commitAfterIteration(repoDir, logger, "task-0001", "success", 0, cfg)
	afterLog := gitLog(t, repoDir)

	if beforeLog != afterLog {
		t.Error("no commit should happen when auto_commit is false")
	}
}

func TestCommitAfterIteration_CommitOnSuccessDisabled(t *testing.T) {
	t.Parallel()
	repoDir := initGitRepo(t)
	modifyTrackedFile(t, repoDir)

	logger := iterTestLogger(t)
	defer logger.Close()

	cfg := testGitCfg()
	cfg.CommitOnSuccess = false

	beforeLog := gitLog(t, repoDir)
	commitAfterIteration(repoDir, logger, "task-0001", "success", 0, cfg)
	afterLog := gitLog(t, repoDir)

	if beforeLog != afterLog {
		t.Error("no commit should happen on success when commit_on_success is false")
	}
}

func TestCommitAfterIteration_CommitOnFailureDisabled(t *testing.T) {
	t.Parallel()
	repoDir := initGitRepo(t)
	modifyTrackedFile(t, repoDir)

	logger := iterTestLogger(t)
	defer logger.Close()

	cfg := testGitCfg()
	cfg.CommitOnFailure = false

	beforeLog := gitLog(t, repoDir)
	commitAfterIteration(repoDir, logger, "task-0001", "failure", 1, cfg)
	afterLog := gitLog(t, repoDir)

	if beforeLog != afterLog {
		t.Error("no commit should happen on failure when commit_on_failure is false")
	}
}

func TestCommitAfterIteration_CommitOnSuccessEnabled_Commits(t *testing.T) {
	t.Parallel()
	repoDir := initGitRepo(t)
	modifyTrackedFile(t, repoDir)

	logger := iterTestLogger(t)
	defer logger.Close()

	commitAfterIteration(repoDir, logger, "task-0001", "success", 0, testGitCfg())

	log := gitLog(t, repoDir)
	if !strings.Contains(log, "task-0001 complete") {
		t.Errorf("success commit should have message 'task-0001 complete', got: %s", log)
	}
}

func TestCommitAfterIteration_FailureCommitMessageFormat(t *testing.T) {
	t.Parallel()
	repoDir := initGitRepo(t)
	modifyTrackedFile(t, repoDir)

	logger := iterTestLogger(t)
	defer logger.Close()

	commitAfterIteration(repoDir, logger, "task-0003", "failure", 5, testGitCfg())

	log := gitLog(t, repoDir)
	if !strings.Contains(log, "task-0003 partial (attempt 5)") {
		t.Errorf("failure commit should have message 'task-0003 partial (attempt 5)', got: %s", log)
	}
}

func TestCommitAfterIteration_CommitStateDisabled_SkipsWolfcastle(t *testing.T) {
	t.Parallel()
	repoDir := initGitRepo(t)

	// Create a tracked code file and modify it
	codeFile := filepath.Join(repoDir, "main.go")
	_ = os.WriteFile(codeFile, []byte("package main\n"), 0644)
	gitRunEnv(t, repoDir, "add", "main.go")
	gitRunEnv(t, repoDir, "commit", "-m", "add code")
	_ = os.WriteFile(codeFile, []byte("package main\n// changed\n"), 0644)

	// Create .wolfcastle/ state (untracked, so it needs explicit git add)
	wcDir := filepath.Join(repoDir, ".wolfcastle")
	_ = os.MkdirAll(wcDir, 0755)
	_ = os.WriteFile(filepath.Join(wcDir, "state.json"), []byte(`{"test":true}`), 0644)

	logger := iterTestLogger(t)
	defer logger.Close()

	cfg := testGitCfg()
	cfg.CommitState = false

	commitAfterIteration(repoDir, logger, "task-0001", "success", 0, cfg)

	// Verify the commit exists and does NOT contain .wolfcastle/
	showCmd := exec.Command("git", "show", "--name-only", "--format=")
	showCmd.Dir = repoDir
	showOut, err := showCmd.Output()
	if err != nil {
		t.Fatalf("git show failed: %v", err)
	}
	committed := string(showOut)
	if strings.Contains(committed, ".wolfcastle") {
		t.Error(".wolfcastle/ should NOT be included when commit_state is false")
	}
	if !strings.Contains(committed, "main.go") {
		t.Error("main.go should still be committed")
	}
}

func TestCommitAfterIteration_CommitStateEnabled_IncludesWolfcastle(t *testing.T) {
	t.Parallel()
	repoDir := initGitRepo(t)

	// Create a tracked code file
	codeFile := filepath.Join(repoDir, "main.go")
	_ = os.WriteFile(codeFile, []byte("package main\n"), 0644)
	// Create .wolfcastle/ state and track both
	wcDir := filepath.Join(repoDir, ".wolfcastle")
	_ = os.MkdirAll(wcDir, 0755)
	_ = os.WriteFile(filepath.Join(wcDir, "state.json"), []byte(`{"v":1}`), 0644)
	gitRunEnv(t, repoDir, "add", ".")
	gitRunEnv(t, repoDir, "commit", "-m", "initial")

	// Modify both
	_ = os.WriteFile(codeFile, []byte("package main\n// v2\n"), 0644)
	_ = os.WriteFile(filepath.Join(wcDir, "state.json"), []byte(`{"v":2}`), 0644)

	logger := iterTestLogger(t)
	defer logger.Close()

	cfg := testGitCfg()
	cfg.CommitState = true

	commitAfterIteration(repoDir, logger, "task-0001", "success", 0, cfg)

	showCmd := exec.Command("git", "show", "--name-only", "--format=")
	showCmd.Dir = repoDir
	showOut, err := showCmd.Output()
	if err != nil {
		t.Fatalf("git show failed: %v", err)
	}
	committed := string(showOut)
	if !strings.Contains(committed, ".wolfcastle/state.json") {
		t.Error(".wolfcastle/state.json should be included when commit_state is true")
	}
	if !strings.Contains(committed, "main.go") {
		t.Error("main.go should be committed")
	}
}

func TestCommitAfterIteration_InvalidTaskID_Skips(t *testing.T) {
	t.Parallel()
	repoDir := initGitRepo(t)
	modifyTrackedFile(t, repoDir)

	logger := iterTestLogger(t)
	defer logger.Close()

	beforeLog := gitLog(t, repoDir)
	commitAfterIteration(repoDir, logger, "../../etc/passwd", "success", 0, testGitCfg())
	afterLog := gitLog(t, repoDir)

	if beforeLog != afterLog {
		t.Error("should not commit with an invalid task ID")
	}
}

// ═══════════════════════════════════════════════════════════════════════════
// runIteration — success-path commit integration
// ═══════════════════════════════════════════════════════════════════════════

func TestRunIteration_SuccessCommit_CreatesCommit(t *testing.T) {
	t.Parallel()
	d := testDaemon(t)
	_ = d.Logger.StartIteration()
	defer d.Logger.Close()

	repoDir := initGitRepo(t)
	d.RepoDir = repoDir
	d.Git = git.NewService(repoDir)

	// Enable auto-commit on success
	d.Config.Git = testGitCfg()

	setupLeafNode(t, d, "success-commit", []state.Task{
		{ID: "task-0001", Description: "work", State: state.StatusNotStarted},
	})
	writePromptFile(t, d.WolfcastleDir, "stages/execute.md")

	// Model writes a file and emits COMPLETE
	codeFile := filepath.Join(repoDir, "output.go")
	_ = os.WriteFile(codeFile, []byte("package main\n"), 0644)
	gitRunEnv(t, repoDir, "add", "output.go")
	gitRunEnv(t, repoDir, "commit", "-m", "track output.go")

	d.Config.Models["writer"] = config.ModelDef{
		Command: "sh",
		Args: []string{"-c", "echo '// generated' >> " + codeFile + " && echo WOLFCASTLE_COMPLETE"},
	}
	d.Config.Pipeline.Stages = map[string]config.PipelineStage{
		"execute": {Model: "writer", PromptFile: "stages/execute.md"},
	}
	d.Config.Pipeline.StageOrder = []string{"execute"}

	idx, _ := d.Store.ReadIndex()
	nav := &state.NavigationResult{NodeAddress: "success-commit", TaskID: "task-0001", Found: true}
	err := d.runIteration(context.Background(), nav, idx)
	if err != nil {
		t.Fatalf("runIteration error: %v", err)
	}

	// Task should be complete
	ns, _ := d.Store.ReadNode("success-commit")
	for _, task := range ns.Tasks {
		if task.ID == "task-0001" && task.State != state.StatusComplete {
			t.Errorf("expected complete, got %s", task.State)
		}
	}

	// There should be a commit with the success message format
	log := gitLog(t, repoDir)
	if !strings.Contains(log, "task-0001 complete") {
		t.Errorf("expected success commit message in git log, got: %s", log)
	}
}

func TestRunIteration_SuccessCommit_SkippedWhenDisabled(t *testing.T) {
	t.Parallel()
	d := testDaemon(t)
	_ = d.Logger.StartIteration()
	defer d.Logger.Close()

	repoDir := initGitRepo(t)
	d.RepoDir = repoDir
	d.Git = git.NewService(repoDir)

	d.Config.Git = testGitCfg()
	d.Config.Git.CommitOnSuccess = false

	setupLeafNode(t, d, "no-success-commit", []state.Task{
		{ID: "task-0001", Description: "work", State: state.StatusNotStarted},
	})
	writePromptFile(t, d.WolfcastleDir, "stages/execute.md")

	codeFile := filepath.Join(repoDir, "output.go")
	_ = os.WriteFile(codeFile, []byte("package main\n"), 0644)
	gitRunEnv(t, repoDir, "add", "output.go")
	gitRunEnv(t, repoDir, "commit", "-m", "track output.go")

	d.Config.Models["writer"] = config.ModelDef{
		Command: "sh",
		Args: []string{"-c", "echo '// generated' >> " + codeFile + " && echo WOLFCASTLE_COMPLETE"},
	}
	d.Config.Pipeline.Stages = map[string]config.PipelineStage{
		"execute": {Model: "writer", PromptFile: "stages/execute.md"},
	}
	d.Config.Pipeline.StageOrder = []string{"execute"}

	beforeLog := gitLog(t, repoDir)

	idx, _ := d.Store.ReadIndex()
	nav := &state.NavigationResult{NodeAddress: "no-success-commit", TaskID: "task-0001", Found: true}
	_ = d.runIteration(context.Background(), nav, idx)

	afterLog := gitLog(t, repoDir)
	if strings.Contains(afterLog, "task-0001 complete") {
		t.Error("should not commit on success when commit_on_success is false")
	}
	// Commit count should be unchanged
	if strings.Count(afterLog, "\n") != strings.Count(beforeLog, "\n") {
		t.Error("no new commit should be created")
	}
}

func TestRunIteration_SuccessCommit_SkippedWhenAutoCommitDisabled(t *testing.T) {
	t.Parallel()
	d := testDaemon(t)
	_ = d.Logger.StartIteration()
	defer d.Logger.Close()

	repoDir := initGitRepo(t)
	d.RepoDir = repoDir
	d.Git = git.NewService(repoDir)

	d.Config.Git = testGitCfg()
	d.Config.Git.AutoCommit = false

	setupLeafNode(t, d, "no-auto-commit", []state.Task{
		{ID: "task-0001", Description: "work", State: state.StatusNotStarted},
	})
	writePromptFile(t, d.WolfcastleDir, "stages/execute.md")

	codeFile := filepath.Join(repoDir, "output.go")
	_ = os.WriteFile(codeFile, []byte("package main\n"), 0644)
	gitRunEnv(t, repoDir, "add", "output.go")
	gitRunEnv(t, repoDir, "commit", "-m", "track output.go")

	d.Config.Models["writer"] = config.ModelDef{
		Command: "sh",
		Args: []string{"-c", "echo '// generated' >> " + codeFile + " && echo WOLFCASTLE_COMPLETE"},
	}
	d.Config.Pipeline.Stages = map[string]config.PipelineStage{
		"execute": {Model: "writer", PromptFile: "stages/execute.md"},
	}
	d.Config.Pipeline.StageOrder = []string{"execute"}

	beforeLog := gitLog(t, repoDir)

	idx, _ := d.Store.ReadIndex()
	nav := &state.NavigationResult{NodeAddress: "no-auto-commit", TaskID: "task-0001", Found: true}
	_ = d.runIteration(context.Background(), nav, idx)

	afterLog := gitLog(t, repoDir)
	if strings.Count(afterLog, "\n") != strings.Count(beforeLog, "\n") {
		t.Error("no new commit should be created when auto_commit is false")
	}
}

// ═══════════════════════════════════════════════════════════════════════════
// runIteration — failure-path commit integration
// ═══════════════════════════════════════════════════════════════════════════

func TestRunIteration_FailureCommit_CreatesCommit(t *testing.T) {
	t.Parallel()
	d := testDaemon(t)
	d.Config.Failure.DecompositionThreshold = 0
	d.Config.Failure.HardCap = 0
	_ = d.Logger.StartIteration()
	defer d.Logger.Close()

	repoDir := initGitRepo(t)
	d.RepoDir = repoDir
	d.Git = git.NewService(repoDir)

	d.Config.Git = testGitCfg()

	setupLeafNode(t, d, "fail-commit", []state.Task{
		{ID: "task-0001", Description: "work", State: state.StatusNotStarted},
	})
	writePromptFile(t, d.WolfcastleDir, "stages/execute.md")

	// Model modifies a tracked file but emits no terminal marker (failure path)
	codeFile := filepath.Join(repoDir, "output.go")
	_ = os.WriteFile(codeFile, []byte("package main\n"), 0644)
	gitRunEnv(t, repoDir, "add", "output.go")
	gitRunEnv(t, repoDir, "commit", "-m", "track output.go")

	d.Config.Models["partial"] = config.ModelDef{
		Command: "sh",
		Args: []string{"-c", "echo '// partial work' >> " + codeFile + " && echo 'no marker here'"},
	}
	d.Config.Pipeline.Stages = map[string]config.PipelineStage{
		"execute": {Model: "partial", PromptFile: "stages/execute.md"},
	}
	d.Config.Pipeline.StageOrder = []string{"execute"}

	idx, _ := d.Store.ReadIndex()
	nav := &state.NavigationResult{NodeAddress: "fail-commit", TaskID: "task-0001", Found: true}
	_ = d.runIteration(context.Background(), nav, idx)

	log := gitLog(t, repoDir)
	if !strings.Contains(log, "task-0001 partial (attempt 1)") {
		t.Errorf("expected failure commit message, got: %s", log)
	}
}

func TestRunIteration_FailureCommit_SkippedWhenDisabled(t *testing.T) {
	t.Parallel()
	d := testDaemon(t)
	d.Config.Failure.DecompositionThreshold = 0
	d.Config.Failure.HardCap = 0
	_ = d.Logger.StartIteration()
	defer d.Logger.Close()

	repoDir := initGitRepo(t)
	d.RepoDir = repoDir
	d.Git = git.NewService(repoDir)

	d.Config.Git = testGitCfg()
	d.Config.Git.CommitOnFailure = false

	setupLeafNode(t, d, "no-fail-commit", []state.Task{
		{ID: "task-0001", Description: "work", State: state.StatusNotStarted},
	})
	writePromptFile(t, d.WolfcastleDir, "stages/execute.md")

	codeFile := filepath.Join(repoDir, "output.go")
	_ = os.WriteFile(codeFile, []byte("package main\n"), 0644)
	gitRunEnv(t, repoDir, "add", "output.go")
	gitRunEnv(t, repoDir, "commit", "-m", "track output.go")

	d.Config.Models["partial"] = config.ModelDef{
		Command: "sh",
		Args: []string{"-c", "echo '// partial' >> " + codeFile + " && echo 'no marker'"},
	}
	d.Config.Pipeline.Stages = map[string]config.PipelineStage{
		"execute": {Model: "partial", PromptFile: "stages/execute.md"},
	}
	d.Config.Pipeline.StageOrder = []string{"execute"}

	beforeLog := gitLog(t, repoDir)

	idx, _ := d.Store.ReadIndex()
	nav := &state.NavigationResult{NodeAddress: "no-fail-commit", TaskID: "task-0001", Found: true}
	_ = d.runIteration(context.Background(), nav, idx)

	afterLog := gitLog(t, repoDir)
	if strings.Contains(afterLog, "partial (attempt") {
		t.Error("should not commit on failure when commit_on_failure is false")
	}
	if strings.Count(afterLog, "\n") != strings.Count(beforeLog, "\n") {
		t.Error("no new commit should be created")
	}
}

// ═══════════════════════════════════════════════════════════════════════════
// runIteration — clean working tree skip (no empty commits)
// ═══════════════════════════════════════════════════════════════════════════

func TestRunIteration_SuccessCleanTree_NoEmptyCommit(t *testing.T) {
	t.Parallel()
	d := testDaemon(t)
	_ = d.Logger.StartIteration()
	defer d.Logger.Close()

	// The model emits COMPLETE but doesn't modify any files.
	// In a real git repo with HasProgress returning false,
	// the task falls through to failure. But the commit path
	// should also not create empty commits.
	repoDir := initGitRepo(t)
	d.RepoDir = repoDir
	d.Git = git.NewService(repoDir)
	d.Config.Git = testGitCfg()

	setupLeafNode(t, d, "clean-tree", []state.Task{
		{ID: "task-0001", Description: "work", State: state.StatusNotStarted},
	})
	writePromptFile(t, d.WolfcastleDir, "stages/execute.md")

	// Model makes no file changes
	d.Config.Models["echo"] = config.ModelDef{Command: "echo", Args: []string{"WOLFCASTLE_COMPLETE"}}
	d.Config.Pipeline.Stages = map[string]config.PipelineStage{
		"execute": {Model: "echo", PromptFile: "stages/execute.md"},
	}
	d.Config.Pipeline.StageOrder = []string{"execute"}
	d.Config.Failure.DecompositionThreshold = 0
	d.Config.Failure.HardCap = 0

	beforeLog := gitLog(t, repoDir)

	idx, _ := d.Store.ReadIndex()
	nav := &state.NavigationResult{NodeAddress: "clean-tree", TaskID: "task-0001", Found: true}
	_ = d.runIteration(context.Background(), nav, idx)

	afterLog := gitLog(t, repoDir)
	if strings.Contains(afterLog, "wolfcastle:") && !strings.Contains(beforeLog, "wolfcastle:") {
		t.Error("should not create any wolfcastle commit when working tree is clean")
	}
}

// ═══════════════════════════════════════════════════════════════════════════
// Helpers
// ═══════════════════════════════════════════════════════════════════════════

// modifyTrackedFile creates a tracked file, commits it, then modifies it
// so there's a dirty working tree for commitAfterIteration to find.
func modifyTrackedFile(t *testing.T, repoDir string) {
	t.Helper()
	f := filepath.Join(repoDir, "code.go")
	_ = os.WriteFile(f, []byte("package main\n"), 0644)
	gitRunEnv(t, repoDir, "add", "code.go")
	gitRunEnv(t, repoDir, "commit", "-m", "add code.go")
	_ = os.WriteFile(f, []byte("package main\n// modified\n"), 0644)
}

// gitRunEnv runs a git command with test author/committer env set.
func gitRunEnv(t *testing.T, dir string, args ...string) {
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

// gitLog returns the oneline git log for the repo.
func gitLog(t *testing.T, dir string) string {
	t.Helper()
	cmd := exec.Command("git", "log", "--oneline")
	cmd.Dir = dir
	out, err := cmd.Output()
	if err != nil {
		t.Fatalf("git log failed: %v", err)
	}
	return string(out)
}

// iterTestLogger creates a test logger with a started iteration.
func iterTestLogger(t *testing.T) *logging.Logger {
	t.Helper()
	logDir := filepath.Join(t.TempDir(), "logs")
	_ = os.MkdirAll(logDir, 0755)
	l, err := logging.NewLogger(logDir)
	if err != nil {
		t.Fatal(err)
	}
	_ = l.StartIteration()
	return l
}
