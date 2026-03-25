package daemon

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"testing"

	"github.com/dorkusprime/wolfcastle/internal/config"
	"github.com/dorkusprime/wolfcastle/internal/git"
	"github.com/dorkusprime/wolfcastle/internal/logging"
	"github.com/dorkusprime/wolfcastle/internal/state"
)

// ═══════════════════════════════════════════════════════════════════════════
// runIteration — already in_progress task skips claim
// ═══════════════════════════════════════════════════════════════════════════

func TestRunIteration_AlreadyInProgress_SkipsClaim(t *testing.T) {
	t.Parallel()
	d := testDaemon(t)
	_ = d.Logger.StartIteration()
	defer d.Logger.Close()

	// Set task to in_progress before calling runIteration. The claim
	// logic should detect this and skip the MutateNode call.
	setupLeafNode(t, d, "skip-claim", []state.Task{
		{ID: "task-0001", Description: "resumed", State: state.StatusInProgress},
	})
	writePromptFile(t, d.WolfcastleDir, "stages/execute.md")

	idx, _ := d.Store.ReadIndex()
	nav := &state.NavigationResult{NodeAddress: "skip-claim", TaskID: "task-0001", Found: true}
	err := d.runIteration(context.Background(), nav, idx)
	if err != nil {
		t.Fatalf("runIteration error: %v", err)
	}

	// Task should be complete (echo outputs WOLFCASTLE_COMPLETE)
	ns, _ := d.Store.ReadNode("skip-claim")
	for _, task := range ns.Tasks {
		if task.ID == "task-0001" && task.State != state.StatusComplete {
			t.Errorf("expected complete after skip-claim path, got %s", task.State)
		}
	}
}

// ═══════════════════════════════════════════════════════════════════════════
// runIteration — WOLFCASTLE_YIELD with planning disabled + new tasks
// ═══════════════════════════════════════════════════════════════════════════

func TestRunIteration_YieldDecomposition_PlanningDisabled(t *testing.T) {
	t.Parallel()
	d := testDaemon(t)
	d.Config.Pipeline.Planning.Enabled = false
	_ = d.Logger.StartIteration()
	defer d.Logger.Close()

	projDir := d.Store.Dir()
	stateFile := filepath.Join(projDir, "yield-decomp", "state.json")

	setupLeafNode(t, d, "yield-decomp", []state.Task{
		{ID: "task-0001", Description: "parent task", State: state.StatusNotStarted},
	})
	writePromptFile(t, d.WolfcastleDir, "stages/execute.md")

	// Model command: add a new task to the state file, then emit YIELD.
	d.Config.Models["yield-add"] = config.ModelDef{
		Command: "sh",
		Args: []string{"-c", fmt.Sprintf(
			`python3 -c "
import json
with open('%s') as f: data = json.load(f)
data['tasks'].append({'id':'task-0002','description':'subtask','state':'not_started'})
with open('%s','w') as f: json.dump(data, f)
" 2>/dev/null; echo WOLFCASTLE_YIELD`, stateFile, stateFile,
		)},
	}
	d.Config.Pipeline.Stages = map[string]config.PipelineStage{
		"execute": {Model: "yield-add", PromptFile: "stages/execute.md"},
	}
	d.Config.Pipeline.StageOrder = []string{"execute"}

	idx, _ := d.Store.ReadIndex()
	nav := &state.NavigationResult{NodeAddress: "yield-decomp", TaskID: "task-0001", Found: true}
	err := d.runIteration(context.Background(), nav, idx)
	if err != nil {
		t.Fatalf("runIteration error: %v", err)
	}

	// With planning disabled and new tasks, the parent should be blocked
	// with reason "decomposed into subtasks: task-0002"
	ns, _ := d.Store.ReadNode("yield-decomp")
	for _, task := range ns.Tasks {
		if task.ID == "task-0001" {
			if task.State != state.StatusBlocked {
				t.Errorf("expected parent blocked after yield decomposition, got %s", task.State)
			}
			if !strings.Contains(task.BlockedReason, "decomposed into subtasks") {
				t.Errorf("expected decomposition reason, got %q", task.BlockedReason)
			}
			return
		}
	}
	t.Error("task-0001 not found")
}

// ═══════════════════════════════════════════════════════════════════════════
// runIteration — WOLFCASTLE_YIELD with planning enabled + new tasks
// ═══════════════════════════════════════════════════════════════════════════

func TestRunIteration_YieldDecomposition_PlanningEnabled(t *testing.T) {
	t.Parallel()
	d := testDaemon(t)
	d.Config.Pipeline.Planning.Enabled = true
	_ = d.Logger.StartIteration()
	defer d.Logger.Close()

	projDir := d.Store.Dir()
	stateFile := filepath.Join(projDir, "yield-plan", "state.json")

	setupLeafNode(t, d, "yield-plan", []state.Task{
		{ID: "task-0001", Description: "parent", State: state.StatusNotStarted},
	})
	writePromptFile(t, d.WolfcastleDir, "stages/execute.md")

	d.Config.Models["yield-plan"] = config.ModelDef{
		Command: "sh",
		Args: []string{"-c", fmt.Sprintf(
			`python3 -c "
import json
with open('%s') as f: data = json.load(f)
data['tasks'].append({'id':'task-0002','description':'child','state':'not_started'})
with open('%s','w') as f: json.dump(data, f)
" 2>/dev/null; echo WOLFCASTLE_YIELD`, stateFile, stateFile,
		)},
	}
	d.Config.Pipeline.Stages = map[string]config.PipelineStage{
		"execute": {Model: "yield-plan", PromptFile: "stages/execute.md"},
	}
	d.Config.Pipeline.StageOrder = []string{"execute"}

	idx, _ := d.Store.ReadIndex()
	nav := &state.NavigationResult{NodeAddress: "yield-plan", TaskID: "task-0001", Found: true}
	err := d.runIteration(context.Background(), nav, idx)
	if err != nil {
		t.Fatalf("runIteration error: %v", err)
	}

	// With planning enabled, the parent task stays in_progress (not blocked).
	ns, _ := d.Store.ReadNode("yield-plan")
	for _, task := range ns.Tasks {
		if task.ID == "task-0001" {
			if task.State == state.StatusBlocked {
				t.Error("with planning enabled, parent should NOT be blocked on yield")
			}
			return
		}
	}
	t.Error("task-0001 not found")
}

// ═══════════════════════════════════════════════════════════════════════════
// runIteration — WOLFCASTLE_YIELD with no new tasks
// ═══════════════════════════════════════════════════════════════════════════

func TestRunIteration_YieldNoNewTasks(t *testing.T) {
	t.Parallel()
	d := testDaemon(t)
	d.Config.Pipeline.Planning.Enabled = false
	d.Config.Models["yield"] = config.ModelDef{Command: "echo", Args: []string{"WOLFCASTLE_YIELD"}}
	d.Config.Pipeline.Stages = map[string]config.PipelineStage{
		"execute": {Model: "yield", PromptFile: "stages/execute.md"},
	}
	d.Config.Pipeline.StageOrder = []string{"execute"}
	_ = d.Logger.StartIteration()
	defer d.Logger.Close()

	setupLeafNode(t, d, "yield-no-new", []state.Task{
		{ID: "task-0001", Description: "work", State: state.StatusNotStarted},
	})
	writePromptFile(t, d.WolfcastleDir, "stages/execute.md")

	idx, _ := d.Store.ReadIndex()
	nav := &state.NavigationResult{NodeAddress: "yield-no-new", TaskID: "task-0001", Found: true}
	err := d.runIteration(context.Background(), nav, idx)
	if err != nil {
		t.Fatalf("runIteration error: %v", err)
	}

	ns, _ := d.Store.ReadNode("yield-no-new")
	for _, task := range ns.Tasks {
		if task.ID == "task-0001" {
			if task.State != state.StatusInProgress {
				t.Errorf("expected in_progress after yield with no new tasks, got %s", task.State)
			}
			return
		}
	}
}

// ═══════════════════════════════════════════════════════════════════════════
// runIteration — WOLFCASTLE_BLOCKED with superseded detection
// ═══════════════════════════════════════════════════════════════════════════

func TestRunIteration_BlockedSuperseded_TreatedAsSkip(t *testing.T) {
	t.Parallel()
	d := testDaemon(t)
	_ = d.Logger.StartIteration()
	defer d.Logger.Close()

	setupLeafNode(t, d, "superseded-node", []state.Task{
		{ID: "task-0001", Description: "work", State: state.StatusNotStarted,
			BlockedReason: "already done in prior commit"},
	})
	writePromptFile(t, d.WolfcastleDir, "stages/execute.md")

	d.Config.Models["blocked"] = config.ModelDef{Command: "echo", Args: []string{"WOLFCASTLE_BLOCKED"}}
	d.Config.Pipeline.Stages = map[string]config.PipelineStage{
		"execute": {Model: "blocked", PromptFile: "stages/execute.md"},
	}
	d.Config.Pipeline.StageOrder = []string{"execute"}

	idx, _ := d.Store.ReadIndex()
	nav := &state.NavigationResult{NodeAddress: "superseded-node", TaskID: "task-0001", Found: true}
	err := d.runIteration(context.Background(), nav, idx)
	if err != nil {
		t.Fatalf("runIteration error: %v", err)
	}

	ns, _ := d.Store.ReadNode("superseded-node")
	for _, task := range ns.Tasks {
		if task.ID == "task-0001" {
			if task.State != state.StatusComplete {
				t.Errorf("superseded block should be completed, got %s", task.State)
			}
			return
		}
	}
	t.Error("task-0001 not found")
}

// ═══════════════════════════════════════════════════════════════════════════
// runIteration — WOLFCASTLE_BLOCKED triggers remediation subtasks
// ═══════════════════════════════════════════════════════════════════════════

func TestRunIteration_BlockedAudit_CreatesRemediationSubtasks(t *testing.T) {
	t.Parallel()
	d := testDaemon(t)
	_ = d.Logger.StartIteration()
	defer d.Logger.Close()

	projDir := d.Store.Dir()

	setupLeafNode(t, d, "audit-remediation", []state.Task{
		{ID: "task-0001", State: state.StatusComplete},
		{ID: "audit", Description: "audit", State: state.StatusNotStarted, IsAudit: true},
	})

	nsPath := filepath.Join(projDir, "audit-remediation", "state.json")
	ns, _ := state.LoadNodeState(nsPath)
	ns.Audit.Gaps = []state.Gap{
		{ID: "gap-1", Description: "Missing error handling", Status: state.GapOpen},
		{ID: "gap-2", Description: "No input validation", Status: state.GapOpen},
	}
	writeJSON(t, nsPath, ns)

	writePromptFile(t, d.WolfcastleDir, "stages/execute.md")

	d.Config.Models["blocked"] = config.ModelDef{Command: "echo", Args: []string{"WOLFCASTLE_BLOCKED"}}
	d.Config.Pipeline.Stages = map[string]config.PipelineStage{
		"execute": {Model: "blocked", PromptFile: "stages/execute.md"},
	}
	d.Config.Pipeline.StageOrder = []string{"execute"}

	idx, _ := d.Store.ReadIndex()
	nav := &state.NavigationResult{NodeAddress: "audit-remediation", TaskID: "audit", Found: true}
	err := d.runIteration(context.Background(), nav, idx)
	if err != nil {
		t.Fatalf("runIteration error: %v", err)
	}

	after, _ := d.Store.ReadNode("audit-remediation")
	var childCount int
	for _, task := range after.Tasks {
		if strings.HasPrefix(task.ID, "audit.") {
			childCount++
		}
		if task.ID == "audit" {
			if task.State != state.StatusNotStarted {
				t.Errorf("audit task should be reset to not_started, got %s", task.State)
			}
		}
	}
	if childCount != 2 {
		t.Errorf("expected 2 remediation subtasks, got %d", childCount)
	}
}

// ═══════════════════════════════════════════════════════════════════════════
// runIteration — WOLFCASTLE_BLOCKED normal path (propagateState)
// ═══════════════════════════════════════════════════════════════════════════

func TestRunIteration_BlockedNormal_BlocksAndPropagates(t *testing.T) {
	t.Parallel()
	d := testDaemon(t)
	_ = d.Logger.StartIteration()
	defer d.Logger.Close()

	setupLeafNode(t, d, "blocked-normal", []state.Task{
		{ID: "task-0001", Description: "work", State: state.StatusNotStarted},
	})
	writePromptFile(t, d.WolfcastleDir, "stages/execute.md")

	d.Config.Models["blocked"] = config.ModelDef{Command: "echo", Args: []string{"WOLFCASTLE_BLOCKED"}}
	d.Config.Pipeline.Stages = map[string]config.PipelineStage{
		"execute": {Model: "blocked", PromptFile: "stages/execute.md"},
	}
	d.Config.Pipeline.StageOrder = []string{"execute"}

	idx, _ := d.Store.ReadIndex()
	nav := &state.NavigationResult{NodeAddress: "blocked-normal", TaskID: "task-0001", Found: true}
	err := d.runIteration(context.Background(), nav, idx)
	if err != nil {
		t.Fatalf("runIteration error: %v", err)
	}

	ns, _ := d.Store.ReadNode("blocked-normal")
	for _, task := range ns.Tasks {
		if task.ID == "task-0001" {
			if task.State != state.StatusBlocked {
				t.Errorf("expected blocked, got %s", task.State)
			}
			if task.BlockedReason != "blocked by model" {
				t.Errorf("expected 'blocked by model' reason, got %q", task.BlockedReason)
			}
			return
		}
	}
	t.Error("task-0001 not found")
}

// ═══════════════════════════════════════════════════════════════════════════
// runIteration — WOLFCASTLE_COMPLETE with missing deliverables (warning)
// ═══════════════════════════════════════════════════════════════════════════

func TestRunIteration_Complete_MissingDeliverables(t *testing.T) {
	t.Parallel()
	d := testDaemon(t)
	_ = d.Logger.StartIteration()
	defer d.Logger.Close()

	setupLeafNode(t, d, "deliv-warn", []state.Task{
		{ID: "task-0001", Description: "work", State: state.StatusNotStarted,
			Deliverables: []string{"nonexistent/file.go"}},
	})
	writePromptFile(t, d.WolfcastleDir, "stages/execute.md")

	idx, _ := d.Store.ReadIndex()
	nav := &state.NavigationResult{NodeAddress: "deliv-warn", TaskID: "task-0001", Found: true}
	err := d.runIteration(context.Background(), nav, idx)
	if err != nil {
		t.Fatalf("runIteration error: %v", err)
	}

	// Deliverables are advisory. The echo model outputs WOLFCASTLE_COMPLETE;
	// in a non-git temp dir, git.HasProgress returns true (git unavailable
	// is treated as progress), so the task completes.
	ns, _ := d.Store.ReadNode("deliv-warn")
	for _, task := range ns.Tasks {
		if task.ID == "task-0001" {
			if task.State != state.StatusComplete {
				t.Errorf("expected complete despite missing deliverables, got %s", task.State)
			}
			return
		}
	}
	t.Error("task-0001 not found")
}

// ═══════════════════════════════════════════════════════════════════════════
// runIteration — WOLFCASTLE_COMPLETE for audit task (skips git check)
// ═══════════════════════════════════════════════════════════════════════════

func TestRunIteration_CompleteAudit_SkipsGitCheck(t *testing.T) {
	t.Parallel()
	d := testDaemon(t)
	_ = d.Logger.StartIteration()
	defer d.Logger.Close()

	setupLeafNode(t, d, "audit-complete", []state.Task{
		{ID: "audit", Description: "audit task", State: state.StatusNotStarted, IsAudit: true},
	})
	writePromptFile(t, d.WolfcastleDir, "stages/execute.md")

	idx, _ := d.Store.ReadIndex()
	nav := &state.NavigationResult{NodeAddress: "audit-complete", TaskID: "audit", Found: true}
	err := d.runIteration(context.Background(), nav, idx)
	if err != nil {
		t.Fatalf("runIteration error: %v", err)
	}

	ns, _ := d.Store.ReadNode("audit-complete")
	for _, task := range ns.Tasks {
		if task.ID == "audit" {
			if task.State != state.StatusComplete {
				t.Errorf("audit task should complete without git check, got %s", task.State)
			}
			return
		}
	}
	t.Error("audit task not found")
}

// ═══════════════════════════════════════════════════════════════════════════
// runIteration — WOLFCASTLE_COMPLETE with no git progress (non-audit)
// ═══════════════════════════════════════════════════════════════════════════

func TestRunIteration_CompleteNoGitProgress_FailsTask(t *testing.T) {
	t.Parallel()
	d := testDaemon(t)
	d.Config.Failure.DecompositionThreshold = 0
	d.Config.Failure.HardCap = 0
	_ = d.Logger.StartIteration()
	defer d.Logger.Close()

	// Use the existing initGitRepo which returns the dir
	repoDir := initGitRepo(t)
	d.RepoDir = repoDir
	d.Git = git.NewService(repoDir)

	setupLeafNode(t, d, "no-progress", []state.Task{
		{ID: "task-0001", Description: "work", State: state.StatusNotStarted},
	})
	writePromptFile(t, d.WolfcastleDir, "stages/execute.md")

	d.Config.Models["echo"] = config.ModelDef{Command: "echo", Args: []string{"WOLFCASTLE_COMPLETE"}}

	idx, _ := d.Store.ReadIndex()
	nav := &state.NavigationResult{NodeAddress: "no-progress", TaskID: "task-0001", Found: true}
	err := d.runIteration(context.Background(), nav, idx)
	if err != nil {
		t.Fatalf("runIteration error: %v", err)
	}

	// No git progress: COMPLETE marker is cleared, task falls through to failure.
	ns, _ := d.Store.ReadNode("no-progress")
	for _, task := range ns.Tasks {
		if task.ID == "task-0001" {
			if task.State == state.StatusComplete {
				t.Error("task should NOT be complete when there's no git progress")
			}
			if task.FailureCount < 1 {
				t.Error("failure count should be incremented")
			}
			if task.LastFailureType != "no_progress" {
				t.Errorf("expected last_failure_type 'no_progress', got %q", task.LastFailureType)
			}
			return
		}
	}
	t.Error("task-0001 not found")
}

// ═══════════════════════════════════════════════════════════════════════════
// runIteration — WOLFCASTLE_SKIP with planning disabled (autoComplete)
// ═══════════════════════════════════════════════════════════════════════════

func TestRunIteration_Skip_AutoCompleteDecomposedParent(t *testing.T) {
	t.Parallel()
	d := testDaemon(t)
	d.Config.Pipeline.Planning.Enabled = false
	_ = d.Logger.StartIteration()
	defer d.Logger.Close()

	setupLeafNode(t, d, "skip-auto", []state.Task{
		{ID: "task-0001", Description: "parent", State: state.StatusBlocked,
			BlockedReason: "decomposed into subtasks: task-0002, task-0003"},
		{ID: "task-0002", Description: "subtask 1", State: state.StatusComplete},
		{ID: "task-0003", Description: "subtask 2", State: state.StatusNotStarted},
	})
	writePromptFile(t, d.WolfcastleDir, "stages/execute.md")

	d.Config.Models["skip"] = config.ModelDef{Command: "echo", Args: []string{"WOLFCASTLE_SKIP already done"}}
	d.Config.Pipeline.Stages = map[string]config.PipelineStage{
		"execute": {Model: "skip", PromptFile: "stages/execute.md"},
	}
	d.Config.Pipeline.StageOrder = []string{"execute"}

	idx, _ := d.Store.ReadIndex()
	nav := &state.NavigationResult{NodeAddress: "skip-auto", TaskID: "task-0003", Found: true}
	err := d.runIteration(context.Background(), nav, idx)
	if err != nil {
		t.Fatalf("runIteration error: %v", err)
	}

	ns, _ := d.Store.ReadNode("skip-auto")
	for _, task := range ns.Tasks {
		if task.ID == "task-0003" && task.State != state.StatusComplete {
			t.Errorf("skipped task should be complete, got %s", task.State)
		}
		if task.ID == "task-0001" && task.State != state.StatusComplete {
			t.Errorf("decomposed parent should be auto-completed, got %s", task.State)
		}
	}
}

// ═══════════════════════════════════════════════════════════════════════════
// commitAfterIteration
// ═══════════════════════════════════════════════════════════════════════════

func TestCommitAfterIteration_NoChanges(t *testing.T) {
	t.Parallel()
	repoDir := initGitRepo(t)

	logger := iterCovTestLogger(t)
	defer logger.Close()

	commitAfterIteration(repoDir, logger, "task-0001", "failure", 1, testGitCfg(), taskCommitMeta{}, nil)

	out := iterCovGitLog(t, repoDir)
	if strings.Count(out, "\n") > 1 {
		t.Error("should not create a commit when there are no changes")
	}
}

func TestCommitAfterIteration_CommitsTrackedChanges(t *testing.T) {
	t.Parallel()
	repoDir := initGitRepo(t)

	// Create and track a file, then modify it
	filePath := filepath.Join(repoDir, "tracked.go")
	_ = os.WriteFile(filePath, []byte("package main\n"), 0644)
	run := func(args ...string) {
		t.Helper()
		cmd := exec.Command("git", args...)
		cmd.Dir = repoDir
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
	run("add", "tracked.go")
	run("commit", "-m", "add tracked file")
	_ = os.WriteFile(filePath, []byte("package main\n// modified\n"), 0644)

	logger := iterCovTestLogger(t)
	defer logger.Close()

	commitAfterIteration(repoDir, logger, "task-0001", "failure", 1, testGitCfg(), taskCommitMeta{}, nil)

	out := iterCovGitLog(t, repoDir)
	if !strings.Contains(out, "partial (attempt") {
		t.Error("expected partial attempt commit message in git log")
	}
	if !strings.Contains(out, "task-0001") {
		t.Error("expected task ID in commit message")
	}
}

func TestCommitAfterIteration_MultipleTrackedFiles(t *testing.T) {
	t.Parallel()
	repoDir := initGitRepo(t)

	run := func(args ...string) {
		t.Helper()
		cmd := exec.Command("git", args...)
		cmd.Dir = repoDir
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

	// Create, track, and commit multiple files
	for _, name := range []string{"a.go", "b.go", "c.go"} {
		_ = os.WriteFile(filepath.Join(repoDir, name), []byte("package main\n"), 0644)
	}
	run("add", "a.go", "b.go", "c.go")
	run("commit", "-m", "add files")

	// Modify all tracked files
	for _, name := range []string{"a.go", "b.go", "c.go"} {
		_ = os.WriteFile(filepath.Join(repoDir, name), []byte("package main\n// changed\n"), 0644)
	}

	logger := iterCovTestLogger(t)
	defer logger.Close()

	commitAfterIteration(repoDir, logger, "task-0002", "failure", 1, testGitCfg(), taskCommitMeta{}, nil)

	// Verify the commit was created and contains all three files.
	// The separate-index approach leaves the default index untouched,
	// so git status may show entries, but the commit itself is correct.
	showCmd := exec.Command("git", "show", "--name-only", "--format=")
	showCmd.Dir = repoDir
	showOut, err := showCmd.Output()
	if err != nil {
		t.Fatalf("git show failed: %v", err)
	}
	committed := string(showOut)
	for _, name := range []string{"a.go", "b.go", "c.go"} {
		if !strings.Contains(committed, name) {
			t.Errorf("expected %s in commit, got: %s", name, committed)
		}
	}
}

func TestCommitAfterIteration_NotAGitRepo(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	_ = os.WriteFile(filepath.Join(dir, "file.txt"), []byte("content"), 0644)

	logger := iterCovTestLogger(t)
	defer logger.Close()

	// Should not panic
	commitAfterIteration(dir, logger, "task-0001", "failure", 1, testGitCfg(), taskCommitMeta{}, nil)
}

func TestCommitAfterIteration_GitAddFails(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("permission-based test not supported on Windows")
	}
	t.Parallel()
	repoDir := initGitRepo(t)

	_ = os.WriteFile(filepath.Join(repoDir, "file.go"), []byte("content"), 0644)

	// Make .git read-only to cause git add to fail
	gitDir := filepath.Join(repoDir, ".git")
	_ = os.Chmod(gitDir, 0444)
	defer func() { _ = os.Chmod(gitDir, 0755) }()

	logger := iterCovTestLogger(t)
	defer logger.Close()

	commitAfterIteration(repoDir, logger, "task-0001", "failure", 1, testGitCfg(), taskCommitMeta{}, nil)
}

func TestCommitAfterIteration_UntrackedFilesNotStaged(t *testing.T) {
	t.Parallel()
	repoDir := initGitRepo(t)

	// Create a tracked file and commit it so we have something to modify
	trackedFile := filepath.Join(repoDir, "tracked.go")
	_ = os.WriteFile(trackedFile, []byte("package main\n"), 0644)
	run := func(args ...string) {
		t.Helper()
		cmd := exec.Command("git", args...)
		cmd.Dir = repoDir
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
	run("add", "tracked.go")
	run("commit", "-m", "add tracked file")

	// Modify the tracked file (will be staged by git add -u)
	_ = os.WriteFile(trackedFile, []byte("package main\n// modified\n"), 0644)

	// Create untracked files that should NOT be staged (protected by .gitignore)
	_ = os.WriteFile(filepath.Join(repoDir, ".gitignore"), []byte(".env\ncredentials.json\n"), 0644)
	run("add", ".gitignore")
	run("commit", "-m", "add gitignore")
	_ = os.WriteFile(filepath.Join(repoDir, ".env"), []byte("SECRET=password\n"), 0644)
	_ = os.WriteFile(filepath.Join(repoDir, "credentials.json"), []byte("{}\n"), 0644)

	logger := iterCovTestLogger(t)
	defer logger.Close()

	commitAfterIteration(repoDir, logger, "task-0001", "failure", 1, testGitCfg(), taskCommitMeta{}, nil)

	// Verify .env and credentials.json are NOT in the commit
	showCmd := exec.Command("git", "show", "--name-only", "--format=")
	showCmd.Dir = repoDir
	showOut, err := showCmd.Output()
	if err != nil {
		t.Fatalf("git show failed: %v", err)
	}
	committed := string(showOut)
	if strings.Contains(committed, ".env") {
		t.Error("untracked .env should NOT be in auto-commit")
	}
	if strings.Contains(committed, "credentials.json") {
		t.Error("untracked credentials.json should NOT be in auto-commit")
	}
	if !strings.Contains(committed, "tracked.go") {
		t.Error("modified tracked file should be in auto-commit")
	}

	// Verify ignored files still exist on disk
	if _, err := os.Stat(filepath.Join(repoDir, ".env")); os.IsNotExist(err) {
		t.Error(".env should still exist on disk")
	}
	if _, err := os.Stat(filepath.Join(repoDir, "credentials.json")); os.IsNotExist(err) {
		t.Error("credentials.json should still exist on disk")
	}
}

func TestCommitAfterIteration_SkipHooksFalse(t *testing.T) {
	t.Parallel()
	repoDir := initGitRepo(t)

	// Create and modify a tracked file
	trackedFile := filepath.Join(repoDir, "main.go")
	_ = os.WriteFile(trackedFile, []byte("package main\n"), 0644)
	run := func(args ...string) {
		t.Helper()
		cmd := exec.Command("git", args...)
		cmd.Dir = repoDir
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
	run("add", "main.go")
	run("commit", "-m", "initial")
	_ = os.WriteFile(trackedFile, []byte("package main\n// changed\n"), 0644)

	logger := iterCovTestLogger(t)
	defer logger.Close()

	// With SkipHooksOnAutoCommit=false, the commit should still succeed (no hooks installed)
	cfg := testGitCfg()
	cfg.SkipHooksOnAutoCommit = false
	commitAfterIteration(repoDir, logger, "task-0001", "failure", 1, cfg, taskCommitMeta{}, nil)

	out := iterCovGitLog(t, repoDir)
	if !strings.Contains(out, "partial (attempt") {
		t.Error("expected partial attempt commit message even with skipHooks=false")
	}
}

// ═══════════════════════════════════════════════════════════════════════════
// staging area preservation
// ═══════════════════════════════════════════════════════════════════════════

func TestCommitAfterIteration_DirectCommitCleanStatus(t *testing.T) {
	t.Parallel()
	repoDir := initGitRepo(t)

	run := func(args ...string) {
		t.Helper()
		cmd := exec.Command("git", args...)
		cmd.Dir = repoDir
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

	// Create a tracked file and commit it
	daemonFile := filepath.Join(repoDir, "daemon.go")
	_ = os.WriteFile(daemonFile, []byte("package main\n"), 0644)
	run("add", "daemon.go")
	run("commit", "-m", "initial")

	// Daemon modifies the file
	_ = os.WriteFile(daemonFile, []byte("package main\n// daemon edit\n"), 0644)

	logger := iterCovTestLogger(t)
	defer logger.Close()

	commitAfterIteration(repoDir, logger, "task-0001", "success", 0, testGitCfg(), taskCommitMeta{}, nil)

	// Verify git status shows no phantom diffs after direct commit
	statusCmd := exec.Command("git", "status", "--porcelain")
	statusCmd.Dir = repoDir
	out, _ := statusCmd.Output()
	if len(strings.TrimSpace(string(out))) != 0 {
		t.Errorf("git status should be clean after daemon commit, got: %s", out)
	}
}

func TestCommitAfterIteration_PreservesUserUnstagedChanges(t *testing.T) {
	t.Parallel()
	repoDir := initGitRepo(t)

	run := func(args ...string) {
		t.Helper()
		cmd := exec.Command("git", args...)
		cmd.Dir = repoDir
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

	// Create two tracked files
	userFile := filepath.Join(repoDir, "user.go")
	daemonFile := filepath.Join(repoDir, "daemon.go")
	_ = os.WriteFile(userFile, []byte("package main\n"), 0644)
	_ = os.WriteFile(daemonFile, []byte("package main\n"), 0644)
	run("add", "user.go", "daemon.go")
	run("commit", "-m", "initial")

	// User modifies user.go but does NOT stage it
	_ = os.WriteFile(userFile, []byte("package main\n// user unstaged edit\n"), 0644)

	// Daemon modifies daemon.go
	_ = os.WriteFile(daemonFile, []byte("package main\n// daemon edit\n"), 0644)

	logger := iterCovTestLogger(t)
	defer logger.Close()

	commitAfterIteration(repoDir, logger, "task-0001", "failure", 1, testGitCfg(), taskCommitMeta{}, nil)

	// Both files were modified tracked files, so git add -u in the temp
	// index will stage both. The daemon commit should include both.
	// The key assertion: user.go's working tree content is unchanged.
	content, err := os.ReadFile(userFile)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(content), "user unstaged edit") {
		t.Error("user's working tree modification to user.go should be preserved")
	}
}

func TestCommitAfterIteration_CleanTreeNoCommit(t *testing.T) {
	t.Parallel()
	repoDir := initGitRepo(t)

	logger := iterCovTestLogger(t)
	defer logger.Close()

	// Get initial commit count
	beforeLog := iterCovGitLog(t, repoDir)
	beforeCount := strings.Count(beforeLog, "\n")

	commitAfterIteration(repoDir, logger, "task-0001", "success", 0, testGitCfg(), taskCommitMeta{}, nil)

	afterLog := iterCovGitLog(t, repoDir)
	afterCount := strings.Count(afterLog, "\n")
	if afterCount != beforeCount {
		t.Error("should not create a commit when the working tree is clean")
	}
}

func TestCommitWithSeparateIndex_TempFileCleanedUp(t *testing.T) {
	t.Parallel()
	repoDir := initGitRepo(t)

	run := func(args ...string) {
		t.Helper()
		cmd := exec.Command("git", args...)
		cmd.Dir = repoDir
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

	trackedFile := filepath.Join(repoDir, "code.go")
	_ = os.WriteFile(trackedFile, []byte("package main\n"), 0644)
	run("add", "code.go")
	run("commit", "-m", "initial")
	_ = os.WriteFile(trackedFile, []byte("package main\n// changed\n"), 0644)

	logger := iterCovTestLogger(t)
	defer logger.Close()

	commitAfterIteration(repoDir, logger, "task-0001", "success", 0, testGitCfg(), taskCommitMeta{}, nil)

	// Verify no temp index files remain
	entries, _ := filepath.Glob(filepath.Join(repoDir, ".git-daemon-index-*"))
	if len(entries) > 0 {
		t.Errorf("temp index files should be cleaned up, found: %v", entries)
	}
}

// ═══════════════════════════════════════════════════════════════════════════
// autoCompleteDecomposedParents — edge cases
// ═══════════════════════════════════════════════════════════════════════════

func TestAutoCompleteDecomposedParents_MissingSubtask(t *testing.T) {
	t.Parallel()
	d := testDaemon(t)

	setupLeafNode(t, d, "auto-missing", []state.Task{
		{ID: "task-0001", Description: "parent", State: state.StatusBlocked,
			BlockedReason: "decomposed into subtasks: task-0002, task-9999"},
		{ID: "task-0002", Description: "child 1", State: state.StatusComplete},
	})

	d.autoCompleteDecomposedParents("auto-missing")

	ns, _ := d.Store.ReadNode("auto-missing")
	for _, task := range ns.Tasks {
		if task.ID == "task-0001" {
			if task.State != state.StatusBlocked {
				t.Errorf("parent should stay blocked when subtask is missing, got %s", task.State)
			}
			return
		}
	}
}

func TestAutoCompleteDecomposedParents_NotDecomposed(t *testing.T) {
	t.Parallel()
	d := testDaemon(t)

	setupLeafNode(t, d, "auto-notdecomp", []state.Task{
		{ID: "task-0001", Description: "parent", State: state.StatusBlocked,
			BlockedReason: "some other reason"},
	})

	d.autoCompleteDecomposedParents("auto-notdecomp")

	ns, _ := d.Store.ReadNode("auto-notdecomp")
	for _, task := range ns.Tasks {
		if task.ID == "task-0001" {
			if task.State != state.StatusBlocked {
				t.Errorf("non-decomposed task should stay blocked, got %s", task.State)
			}
			return
		}
	}
}

func TestAutoCompleteDecomposedParents_ReadError(t *testing.T) {
	t.Parallel()
	d := testDaemon(t)
	d.autoCompleteDecomposedParents("nonexistent-node")
}

// ═══════════════════════════════════════════════════════════════════════════
// isSupersededBlock
// ═══════════════════════════════════════════════════════════════════════════

func TestIsSupersededBlock_VariousKeywords(t *testing.T) {
	t.Parallel()
	cases := []struct {
		reason string
		expect bool
	}{
		{"superseded by task-0003", true},
		{"already done in prior commit", true},
		{"already completed by previous task", true},
		{"no longer needed after refactor", true},
		{"replaced by new implementation", true},
		{"done in task-0002", true},
		{"done directly by orchestrator", true},
		{"waiting for database migration", false},
		{"stuck on API rate limit", false},
		{"", false},
	}

	for _, tc := range cases {
		t.Run(tc.reason, func(t *testing.T) {
			d := testDaemon(t)
			setupLeafNode(t, d, "sup-kw", []state.Task{
				{ID: "task-0001", State: state.StatusBlocked, BlockedReason: tc.reason},
			})

			got := d.isSupersededBlock("sup-kw", "task-0001")
			if got != tc.expect {
				t.Errorf("isSupersededBlock(%q) = %v, want %v", tc.reason, got, tc.expect)
			}
		})
	}
}

func TestIsSupersededBlock_TaskNotFound(t *testing.T) {
	t.Parallel()
	d := testDaemon(t)
	setupLeafNode(t, d, "sup-missing", []state.Task{
		{ID: "task-0001", State: state.StatusBlocked, BlockedReason: "stuck"},
	})

	got := d.isSupersededBlock("sup-missing", "task-9999")
	if got {
		t.Error("should return false for non-existent task")
	}
}

// ═══════════════════════════════════════════════════════════════════════════
// findNewTasks — audit exclusion
// ═══════════════════════════════════════════════════════════════════════════

func TestFindNewTasks_ExcludesAuditTasks(t *testing.T) {
	t.Parallel()
	before := &state.NodeState{
		Tasks: []state.Task{
			{ID: "task-0001", State: state.StatusInProgress},
		},
	}
	after := &state.NodeState{
		Tasks: []state.Task{
			{ID: "task-0001", State: state.StatusInProgress},
			{ID: "task-0002", State: state.StatusNotStarted},
			{ID: "audit", State: state.StatusNotStarted, IsAudit: true},
		},
	}

	newIDs := findNewTasks(before, after)
	if len(newIDs) != 1 {
		t.Fatalf("expected 1 new task (audit excluded), got %d: %v", len(newIDs), newIDs)
	}
	if newIDs[0] != "task-0002" {
		t.Errorf("expected task-0002, got %v", newIDs)
	}
}

// ═══════════════════════════════════════════════════════════════════════════
// runIteration — failure type detection
// ═══════════════════════════════════════════════════════════════════════════

func TestRunIteration_NoProgress_FailureType(t *testing.T) {
	t.Parallel()
	d := testDaemon(t)
	d.Config.Failure.DecompositionThreshold = 0
	d.Config.Failure.HardCap = 0
	_ = d.Logger.StartIteration()
	defer d.Logger.Close()

	repoDir := initGitRepo(t)
	d.RepoDir = repoDir
	d.Git = git.NewService(repoDir)

	setupLeafNode(t, d, "fp-node", []state.Task{
		{ID: "task-0001", Description: "work", State: state.StatusNotStarted},
	})
	writePromptFile(t, d.WolfcastleDir, "stages/execute.md")

	d.Config.Models["echo"] = config.ModelDef{Command: "echo", Args: []string{"WOLFCASTLE_COMPLETE"}}

	idx, _ := d.Store.ReadIndex()
	nav := &state.NavigationResult{NodeAddress: "fp-node", TaskID: "task-0001", Found: true}
	_ = d.runIteration(context.Background(), nav, idx)

	ns, _ := d.Store.ReadNode("fp-node")
	for _, task := range ns.Tasks {
		if task.ID == "task-0001" {
			if task.LastFailureType != "no_progress" {
				t.Errorf("expected failure type 'no_progress', got %q", task.LastFailureType)
			}
			return
		}
	}
	t.Error("task-0001 not found")
}

func TestRunIteration_NoMarker_FailureType(t *testing.T) {
	t.Parallel()
	d := testDaemon(t)
	d.Config.Failure.DecompositionThreshold = 0
	d.Config.Failure.HardCap = 0
	_ = d.Logger.StartIteration()
	defer d.Logger.Close()

	setupLeafNode(t, d, "ftm-node", []state.Task{
		{ID: "task-0001", Description: "work", State: state.StatusNotStarted},
	})
	writePromptFile(t, d.WolfcastleDir, "stages/execute.md")

	d.Config.Models["echo"] = config.ModelDef{Command: "echo", Args: []string{"just some text"}}
	d.Config.Pipeline.Stages = map[string]config.PipelineStage{
		"execute": {Model: "echo", PromptFile: "stages/execute.md"},
	}
	d.Config.Pipeline.StageOrder = []string{"execute"}

	idx, _ := d.Store.ReadIndex()
	nav := &state.NavigationResult{NodeAddress: "ftm-node", TaskID: "task-0001", Found: true}
	_ = d.runIteration(context.Background(), nav, idx)

	ns, _ := d.Store.ReadNode("ftm-node")
	for _, task := range ns.Tasks {
		if task.ID == "task-0001" {
			if task.LastFailureType != "no_terminal_marker" {
				t.Errorf("expected 'no_terminal_marker', got %q", task.LastFailureType)
			}
			return
		}
	}
	t.Error("task-0001 not found")
}

// ═══════════════════════════════════════════════════════════════════════════
// Helpers (namespaced to avoid collisions with deliverables_test.go)
// ═══════════════════════════════════════════════════════════════════════════

func iterCovGitLog(t *testing.T, dir string) string {
	t.Helper()
	cmd := exec.Command("git", "log", "--oneline")
	cmd.Dir = dir
	out, err := cmd.Output()
	if err != nil {
		t.Fatalf("git log failed: %v", err)
	}
	return string(out)
}

// testGitCfg returns a GitConfig with auto-commit enabled and all flags on,
// matching the default behavior of the old autoCommitPartialWork.
func testGitCfg() config.GitConfig {
	return config.GitConfig{
		AutoCommit:            true,
		CommitOnSuccess:       true,
		CommitOnFailure:       true,
		CommitState:           true,
		SkipHooksOnAutoCommit: true,
	}
}

func iterCovTestLogger(t *testing.T) *logging.Logger {
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
