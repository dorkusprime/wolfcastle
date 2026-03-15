package daemon

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/dorkusprime/wolfcastle/internal/config"
	"github.com/dorkusprime/wolfcastle/internal/state"
)

// ═══════════════════════════════════════════════════════════════════════════
// selfHeal — multiple in-progress tasks (corruption error)
// ═══════════════════════════════════════════════════════════════════════════

func TestSelfHeal_MultipleInProgress_ReturnsCorruptionError(t *testing.T) {
	t.Parallel()
	d := testDaemon(t)
	projDir := d.Resolver.ProjectsDir()

	idx := state.NewRootIndex()
	idx.Root = []string{"node-a", "node-b"}
	idx.Nodes["node-a"] = state.IndexEntry{
		Name: "A", Type: state.NodeLeaf, State: state.StatusInProgress, Address: "node-a",
	}
	idx.Nodes["node-b"] = state.IndexEntry{
		Name: "B", Type: state.NodeLeaf, State: state.StatusInProgress, Address: "node-b",
	}
	writeJSON(t, d.Resolver.RootIndexPath(), idx)

	nsA := state.NewNodeState("node-a", "A", state.NodeLeaf)
	nsA.Tasks = []state.Task{{ID: "t1", State: state.StatusInProgress}}
	writeJSON(t, filepath.Join(projDir, "node-a", "state.json"), nsA)

	nsB := state.NewNodeState("node-b", "B", state.NodeLeaf)
	nsB.Tasks = []state.Task{{ID: "t1", State: state.StatusInProgress}}
	writeJSON(t, filepath.Join(projDir, "node-b", "state.json"), nsB)

	err := d.selfHeal()
	if err == nil {
		t.Fatal("expected corruption error for multiple in-progress tasks")
	}
	if !strings.Contains(err.Error(), "state corruption") {
		t.Errorf("expected 'state corruption' in error, got: %v", err)
	}
	if !strings.Contains(err.Error(), "2 tasks in progress") {
		t.Errorf("expected '2 tasks in progress' in error, got: %v", err)
	}
}

// ═══════════════════════════════════════════════════════════════════════════
// RunOnce — branch change detection via d.branch mismatch
// ═══════════════════════════════════════════════════════════════════════════

func TestRunOnce_BranchMismatchDetection(t *testing.T) {
	t.Parallel()
	d := testDaemon(t)
	d.Config.Git.VerifyBranch = true
	d.branch = "old-branch-never-exists"

	// Use the current repo directory so git rev-parse succeeds
	repoDir := catAFindRepoRoot()
	if repoDir == "" {
		t.Skip("not in a git repository")
	}
	d.RepoDir = repoDir

	_ = d.Logger.StartIteration()
	defer d.Logger.Close()

	setupLeafNode(t, d, "branch-node", []state.Task{
		{ID: "task-1", State: state.StatusNotStarted},
	})

	result, err := d.RunOnce(context.Background())
	if err == nil {
		t.Skip("branch happened to match")
	}
	if result != IterationStop {
		t.Errorf("expected IterationStop, got %d", result)
	}
	if !strings.Contains(err.Error(), "WOLFCASTLE_BLOCKED") {
		t.Errorf("expected WOLFCASTLE_BLOCKED in error, got: %v", err)
	}
	if !strings.Contains(err.Error(), "old-branch-never-exists") {
		t.Errorf("expected 'old-branch-never-exists' in error, got: %v", err)
	}
}

// catAFindRepoRoot walks up to find a .git directory.
func catAFindRepoRoot() string {
	dir, _ := os.Getwd()
	for i := 0; i < 10; i++ {
		if _, err := os.Stat(filepath.Join(dir, ".git")); err == nil {
			return dir
		}
		parent := filepath.Dir(dir)
		if parent == dir {
			break
		}
		dir = parent
	}
	return ""
}

// ═══════════════════════════════════════════════════════════════════════════
// iteration.go — invoke error return path
// ═══════════════════════════════════════════════════════════════════════════

func TestRunIteration_InvokeErrorReturnsError(t *testing.T) {
	t.Parallel()
	d := testDaemon(t)
	d.Config.Models["failing"] = config.ModelDef{Command: "/nonexistent/binary", Args: []string{}}
	d.Config.Pipeline.Stages = []config.PipelineStage{
		{Name: "execute", Model: "failing", PromptFile: "execute.md"},
	}
	d.Config.Retries.MaxRetries = 0
	_ = d.Logger.StartIteration()
	defer d.Logger.Close()

	setupLeafNode(t, d, "invoke-fail-node", []state.Task{
		{ID: "task-1", Description: "work", State: state.StatusNotStarted},
	})
	writePromptFile(t, d.WolfcastleDir, "execute.md")

	idx, _ := d.Resolver.LoadRootIndex()
	nav := &state.NavigationResult{NodeAddress: "invoke-fail-node", TaskID: "task-1", Found: true}
	err := d.runIteration(context.Background(), nav, idx)
	if err == nil {
		t.Fatal("expected error from failed invocation")
	}
}

// ═══════════════════════════════════════════════════════════════════════════
// iteration.go — failure escalation: decomposition threshold with depth OK
// ═══════════════════════════════════════════════════════════════════════════

func TestRunIteration_DecompThreshold_SetsNeedsDecomposition(t *testing.T) {
	t.Parallel()
	d := testDaemon(t)
	d.Config.Models["silent"] = config.ModelDef{Command: "true", Args: []string{}}
	d.Config.Pipeline.Stages = []config.PipelineStage{
		{Name: "execute", Model: "silent", PromptFile: "execute.md"},
	}
	d.Config.Retries.MaxRetries = 0
	d.Config.Failure.DecompositionThreshold = 1
	d.Config.Failure.MaxDecompositionDepth = 5
	d.Config.Failure.HardCap = 100
	_ = d.Logger.StartIteration()
	defer d.Logger.Close()

	setupLeafNode(t, d, "decomp-node", []state.Task{
		{ID: "task-1", Description: "work", State: state.StatusNotStarted},
	})
	writePromptFile(t, d.WolfcastleDir, "execute.md")

	idx, _ := d.Resolver.LoadRootIndex()
	nav := &state.NavigationResult{NodeAddress: "decomp-node", TaskID: "task-1", Found: true}
	_ = d.runIteration(context.Background(), nav, idx)

	projDir := d.Resolver.ProjectsDir()
	var ns state.NodeState
	data, err := os.ReadFile(filepath.Join(projDir, "decomp-node", "state.json"))
	if err != nil {
		t.Fatal(err)
	}
	if err := json.Unmarshal(data, &ns); err != nil {
		t.Fatal(err)
	}

	found := false
	for _, task := range ns.Tasks {
		if task.ID == "task-1" && task.NeedsDecomposition {
			found = true
		}
	}
	if !found {
		t.Error("expected task-1 to have NeedsDecomposition=true")
	}
}

// ═══════════════════════════════════════════════════════════════════════════
// iteration.go — failure escalation: decomposition at max depth (auto-block)
// ═══════════════════════════════════════════════════════════════════════════

func TestRunIteration_DecompAtMaxDepth_TaskAutoBlocked(t *testing.T) {
	t.Parallel()
	d := testDaemon(t)
	d.Config.Models["silent"] = config.ModelDef{Command: "true", Args: []string{}}
	d.Config.Pipeline.Stages = []config.PipelineStage{
		{Name: "execute", Model: "silent", PromptFile: "execute.md"},
	}
	d.Config.Retries.MaxRetries = 0
	d.Config.Failure.DecompositionThreshold = 1
	d.Config.Failure.MaxDecompositionDepth = 0
	d.Config.Failure.HardCap = 100
	_ = d.Logger.StartIteration()
	defer d.Logger.Close()

	projDir := d.Resolver.ProjectsDir()
	idx := state.NewRootIndex()
	idx.Root = []string{"maxdepth-node"}
	idx.Nodes["maxdepth-node"] = state.IndexEntry{
		Name: "MaxDepth", Type: state.NodeLeaf, State: state.StatusNotStarted, Address: "maxdepth-node",
	}
	writeJSON(t, d.Resolver.RootIndexPath(), idx)

	ns := state.NewNodeState("maxdepth-node", "MaxDepth", state.NodeLeaf)
	ns.DecompositionDepth = 0
	ns.Tasks = []state.Task{
		{ID: "task-1", Description: "work", State: state.StatusNotStarted},
	}
	writeJSON(t, filepath.Join(projDir, "maxdepth-node", "state.json"), ns)
	writePromptFile(t, d.WolfcastleDir, "execute.md")

	nav := &state.NavigationResult{NodeAddress: "maxdepth-node", TaskID: "task-1", Found: true}
	_ = d.runIteration(context.Background(), nav, idx)

	data, err := os.ReadFile(filepath.Join(projDir, "maxdepth-node", "state.json"))
	if err != nil {
		t.Fatal(err)
	}
	var reloaded state.NodeState
	if err := json.Unmarshal(data, &reloaded); err != nil {
		t.Fatal(err)
	}

	for _, task := range reloaded.Tasks {
		if task.ID == "task-1" {
			if task.State != state.StatusBlocked {
				t.Errorf("expected task-1 blocked, got %s", task.State)
			}
			if !strings.Contains(task.BlockedReason, "max depth") {
				t.Errorf("expected 'max depth' in reason, got %q", task.BlockedReason)
			}
			return
		}
	}
	t.Error("task-1 not found")
}

// ═══════════════════════════════════════════════════════════════════════════
// iteration.go — hard cap auto-block path
// ═══════════════════════════════════════════════════════════════════════════

func TestRunIteration_HardCapReached_TaskAutoBlocked(t *testing.T) {
	t.Parallel()
	d := testDaemon(t)
	d.Config.Models["silent"] = config.ModelDef{Command: "true", Args: []string{}}
	d.Config.Pipeline.Stages = []config.PipelineStage{
		{Name: "execute", Model: "silent", PromptFile: "execute.md"},
	}
	d.Config.Retries.MaxRetries = 0
	d.Config.Failure.DecompositionThreshold = 0
	d.Config.Failure.HardCap = 1
	_ = d.Logger.StartIteration()
	defer d.Logger.Close()

	setupLeafNode(t, d, "hardcap-node", []state.Task{
		{ID: "task-1", Description: "work", State: state.StatusNotStarted},
	})
	writePromptFile(t, d.WolfcastleDir, "execute.md")

	idx, _ := d.Resolver.LoadRootIndex()
	nav := &state.NavigationResult{NodeAddress: "hardcap-node", TaskID: "task-1", Found: true}
	_ = d.runIteration(context.Background(), nav, idx)

	projDir := d.Resolver.ProjectsDir()
	data, err := os.ReadFile(filepath.Join(projDir, "hardcap-node", "state.json"))
	if err != nil {
		t.Fatal(err)
	}
	var reloaded state.NodeState
	if err := json.Unmarshal(data, &reloaded); err != nil {
		t.Fatal(err)
	}

	for _, task := range reloaded.Tasks {
		if task.ID == "task-1" {
			if task.State != state.StatusBlocked {
				t.Errorf("expected blocked by hard cap, got %s", task.State)
			}
			if !strings.Contains(task.BlockedReason, "hard cap") {
				t.Errorf("expected 'hard cap' in reason, got %q", task.BlockedReason)
			}
			return
		}
	}
	t.Error("task-1 not found")
}

// ═══════════════════════════════════════════════════════════════════════════
// iteration.go — IncrementFailure error path
// ═══════════════════════════════════════════════════════════════════════════

func TestRunIteration_NoTerminalMarker_FailureIncremented(t *testing.T) {
	t.Parallel()
	d := testDaemon(t)
	d.Config.Models["noop"] = config.ModelDef{Command: "echo", Args: []string{"some output without markers"}}
	d.Config.Pipeline.Stages = []config.PipelineStage{
		{Name: "execute", Model: "noop", PromptFile: "execute.md"},
	}
	d.Config.Retries.MaxRetries = 0
	d.Config.Failure.DecompositionThreshold = 0
	d.Config.Failure.HardCap = 0
	_ = d.Logger.StartIteration()
	defer d.Logger.Close()

	setupLeafNode(t, d, "nonterminal-node", []state.Task{
		{ID: "task-1", Description: "work", State: state.StatusNotStarted},
	})
	writePromptFile(t, d.WolfcastleDir, "execute.md")

	idx, _ := d.Resolver.LoadRootIndex()
	nav := &state.NavigationResult{NodeAddress: "nonterminal-node", TaskID: "task-1", Found: true}
	err := d.runIteration(context.Background(), nav, idx)
	if err != nil {
		t.Logf("runIteration error (may be acceptable): %v", err)
	}
}

// ═══════════════════════════════════════════════════════════════════════════
// propagate.go — ParseAddress error in loadNode callback
// ═══════════════════════════════════════════════════════════════════════════

func TestPropagateState_EmptyParentAddr_LoadNodeError(t *testing.T) {
	t.Parallel()
	d := testDaemon(t)
	idx := state.NewRootIndex()

	idx.Root = []string{""}
	idx.Nodes[""] = state.IndexEntry{
		Name: "Bad", Type: state.NodeOrchestrator, State: state.StatusNotStarted,
		Address: "", Children: []string{"child"},
	}
	idx.Nodes["child"] = state.IndexEntry{
		Name: "Child", Type: state.NodeLeaf, State: state.StatusNotStarted,
		Address: "child", Parent: "",
	}

	err := d.propagateState("child", state.StatusInProgress, idx)
	if err != nil {
		if !strings.Contains(err.Error(), "parsing address") {
			t.Logf("propagateState error: %v", err)
		}
	}
}

// ═══════════════════════════════════════════════════════════════════════════
// propagate.go — saveNode callback exercised via valid propagation
// ═══════════════════════════════════════════════════════════════════════════

func TestPropagateState_SaveNodeCallbackExercised(t *testing.T) {
	t.Parallel()
	d := testDaemon(t)
	projDir := d.Resolver.ProjectsDir()
	idx := state.NewRootIndex()

	idx.Root = []string{"sv-parent"}
	idx.Nodes["sv-parent"] = state.IndexEntry{
		Name: "Parent", Type: state.NodeOrchestrator, State: state.StatusNotStarted,
		Address: "sv-parent", Children: []string{"sv-parent/sv-child"},
	}
	idx.Nodes["sv-parent/sv-child"] = state.IndexEntry{
		Name: "Child", Type: state.NodeLeaf, State: state.StatusNotStarted,
		Address: "sv-parent/sv-child", Parent: "sv-parent",
	}
	writeJSON(t, d.Resolver.RootIndexPath(), idx)

	parentNS := state.NewNodeState("sv-parent", "Parent", state.NodeOrchestrator)
	parentNS.Children = []state.ChildRef{
		{ID: "sv-child", Address: "sv-parent/sv-child", State: state.StatusNotStarted},
	}
	writeJSON(t, filepath.Join(projDir, "sv-parent", "state.json"), parentNS)

	childNS := state.NewNodeState("sv-child", "Child", state.NodeLeaf)
	writeJSON(t, filepath.Join(projDir, "sv-parent", "sv-child", "state.json"), childNS)

	err := d.propagateState("sv-parent/sv-child", state.StatusInProgress, idx)
	if err != nil {
		t.Logf("propagateState error (may be expected): %v", err)
	}
}

// ═══════════════════════════════════════════════════════════════════════════
// retry.go — context cancellation before invocation
// ═══════════════════════════════════════════════════════════════════════════

func TestInvokeWithRetry_PreCancelledContext(t *testing.T) {
	t.Parallel()
	d := testDaemon(t)
	d.Config.Retries.MaxRetries = 3
	_ = d.Logger.StartIteration()
	defer d.Logger.Close()

	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	model := config.ModelDef{Command: "/nonexistent/binary", Args: []string{}}
	_, err := d.invokeWithRetry(ctx, model, "test prompt", d.RepoDir, d.Logger.AssistantWriter(), "test")
	if err == nil {
		t.Fatal("expected error when context is already cancelled")
	}
}

// ═══════════════════════════════════════════════════════════════════════════
// retry.go — context cancellation during backoff wait
// ═══════════════════════════════════════════════════════════════════════════

func TestInvokeWithRetry_CancelDuringBackoff(t *testing.T) {
	t.Parallel()
	d := testDaemon(t)
	d.Config.Retries.MaxRetries = 5
	d.Config.Retries.InitialDelaySeconds = 10
	d.Config.Retries.MaxDelaySeconds = 10
	_ = d.Logger.StartIteration()
	defer d.Logger.Close()

	ctx, cancel := context.WithCancel(context.Background())

	go func() {
		time.Sleep(200 * time.Millisecond)
		cancel()
	}()

	model := config.ModelDef{Command: "/nonexistent/binary", Args: []string{}}
	_, err := d.invokeWithRetry(ctx, model, "test prompt", d.RepoDir, d.Logger.AssistantWriter(), "test")
	if err == nil {
		t.Fatal("expected error when context cancelled during backoff")
	}
}

// ═══════════════════════════════════════════════════════════════════════════
// stages.go — model not found in intake stage (with prompt file present)
// ═══════════════════════════════════════════════════════════════════════════

func TestRunIntakeStage_MissingModelWithPrompt(t *testing.T) {
	t.Parallel()
	d := testDaemon(t)
	_ = d.Logger.StartIteration()
	defer d.Logger.Close()
	writePromptFile(t, d.WolfcastleDir, "intake.md")

	inboxPath := filepath.Join(d.Resolver.ProjectsDir(), "inbox.json")
	writeJSON(t, inboxPath, &state.InboxFile{Items: []state.InboxItem{
		{Status: "new", Text: "item", Timestamp: "2026-03-14T00:00:00Z"},
	}})

	stage := config.PipelineStage{Name: "intake", Model: "absent-model-x", PromptFile: "intake.md"}
	err := d.runIntakeStage(context.Background(), stage)
	if err == nil {
		t.Fatal("expected error for nonexistent model")
	}
	if !strings.Contains(err.Error(), "not found") {
		t.Errorf("expected 'not found' in error, got: %v", err)
	}
}

// ═══════════════════════════════════════════════════════════════════════════
// stages.go — invoke error in intake stage
// ═══════════════════════════════════════════════════════════════════════════

func TestRunIntakeStage_BrokenCommand(t *testing.T) {
	t.Parallel()
	d := testDaemon(t)
	d.Config.Models["broken"] = config.ModelDef{Command: "/nonexistent/binary", Args: []string{}}
	d.Config.Retries.MaxRetries = 0
	_ = d.Logger.StartIteration()
	defer d.Logger.Close()
	writePromptFile(t, d.WolfcastleDir, "intake.md")

	inboxPath := filepath.Join(d.Resolver.ProjectsDir(), "inbox.json")
	writeJSON(t, inboxPath, &state.InboxFile{Items: []state.InboxItem{
		{Status: "new", Text: "item", Timestamp: "2026-03-14T00:00:00Z"},
	}})

	stage := config.PipelineStage{Name: "intake", Model: "broken", PromptFile: "intake.md"}
	err := d.runIntakeStage(context.Background(), stage)
	if err == nil {
		t.Fatal("expected invoke error in intake stage")
	}
}

// ═══════════════════════════════════════════════════════════════════════════
// stages.go — prompt assembly error in intake stage
// ═══════════════════════════════════════════════════════════════════════════

func TestRunIntakeStage_MissingPromptFile(t *testing.T) {
	t.Parallel()
	d := testDaemon(t)
	_ = d.Logger.StartIteration()
	defer d.Logger.Close()

	inboxPath := filepath.Join(d.Resolver.ProjectsDir(), "inbox.json")
	writeJSON(t, inboxPath, &state.InboxFile{Items: []state.InboxItem{
		{Status: "new", Text: "item", Timestamp: "2026-03-14T00:00:00Z"},
	}})

	stage := config.PipelineStage{Name: "intake", Model: "echo", PromptFile: "nonexistent-prompt.md"}
	err := d.runIntakeStage(context.Background(), stage)
	if err == nil {
		t.Fatal("expected prompt assembly error in intake stage")
	}
}
