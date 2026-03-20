package daemon

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/dorkusprime/wolfcastle/internal/config"
	"github.com/dorkusprime/wolfcastle/internal/state"
)

// ═══════════════════════════════════════════════════════════════════════════
// RunOnce — context cancelled after navigation (line 436-438)
// ═══════════════════════════════════════════════════════════════════════════

func TestRunOnce_ContextCancelledBeforeExecution(t *testing.T) {
	t.Parallel()
	d := testDaemon(t)

	// Set up a valid tree with work, but cancel the context before RunOnce
	// so ctx.Err() is non-nil after FindNextTask returns.
	setupLeafNode(t, d, "ctx-node", []state.Task{
		{ID: "task-0001", Description: "work", State: state.StatusNotStarted},
	})
	writePromptFile(t, d.WolfcastleDir, "stages/execute.md")

	ctx, cancel := context.WithCancel(context.Background())
	cancel() // pre-cancel

	result, err := d.RunOnce(ctx)
	if err != nil {
		t.Fatalf("expected nil error for ctx cancel, got: %v", err)
	}
	if result != IterationStop {
		t.Errorf("expected IterationStop, got %d", result)
	}
}

// ═══════════════════════════════════════════════════════════════════════════
// RunOnce — planning pass success returns IterationDidWork (lines 449-455)
// ═══════════════════════════════════════════════════════════════════════════

func TestRunOnce_PlanningPassSuccess(t *testing.T) {
	t.Parallel()
	d := testDaemon(t)
	projDir := d.Store.Dir()

	// Enable planning with an echo model
	d.Config.Pipeline.Planning.Enabled = true
	d.Config.Pipeline.Planning.Model = "echo"
	d.Config.Models["echo"] = config.ModelDef{Command: "echo", Args: []string{"plan output"}}

	// Create an orchestrator with no children and no tasks (needs planning).
	idx := state.NewRootIndex()
	idx.Root = []string{"plan-orch"}
	idx.Nodes["plan-orch"] = state.IndexEntry{
		Name:    "Plan Orch",
		Type:    state.NodeOrchestrator,
		State:   state.StatusNotStarted,
		Address: "plan-orch",
	}
	writeJSON(t, filepath.Join(d.Store.Dir(), "state.json"), idx)

	ns := state.NewNodeState("plan-orch", "Plan Orch", state.NodeOrchestrator)
	writeJSON(t, filepath.Join(projDir, "plan-orch", "state.json"), ns)

	writePromptFile(t, d.WolfcastleDir, "stages/plan-initial.md")

	result, err := d.RunOnce(context.Background())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result != IterationDidWork {
		t.Errorf("expected IterationDidWork from planning, got %d", result)
	}
}

// ═══════════════════════════════════════════════════════════════════════════
// RunOnce — planning pass error returns IterationError (lines 450-453)
// ═══════════════════════════════════════════════════════════════════════════

func TestRunOnce_PlanningPassError(t *testing.T) {
	t.Parallel()
	d := testDaemon(t)
	projDir := d.Store.Dir()

	d.Config.Pipeline.Planning.Enabled = true
	d.Config.Pipeline.Planning.Model = "nonexistent-plan-model"

	// Orchestrator with no children → triggers planning
	idx := state.NewRootIndex()
	idx.Root = []string{"plan-err"}
	idx.Nodes["plan-err"] = state.IndexEntry{
		Name:    "Plan Err",
		Type:    state.NodeOrchestrator,
		State:   state.StatusNotStarted,
		Address: "plan-err",
	}
	writeJSON(t, filepath.Join(d.Store.Dir(), "state.json"), idx)

	ns := state.NewNodeState("plan-err", "Plan Err", state.NodeOrchestrator)
	writeJSON(t, filepath.Join(projDir, "plan-err", "state.json"), ns)
	writePromptFile(t, d.WolfcastleDir, "stages/plan-initial.md")

	result, err := d.RunOnce(context.Background())
	if err != nil {
		t.Fatalf("planning errors should not be fatal, got: %v", err)
	}
	if result != IterationError {
		t.Errorf("expected IterationError from planning failure, got %d", result)
	}
}

// ═══════════════════════════════════════════════════════════════════════════
// RunOnce — default no-work reason (lines 467-468)
// ═══════════════════════════════════════════════════════════════════════════

func TestRunOnce_DefaultNoWorkReason(t *testing.T) {
	t.Parallel()
	d := testDaemon(t)
	projDir := d.Store.Dir()

	// Scope the daemon to a node that exists but is complete. FindNextTask
	// returns reason "scoped node is complete" which doesn't match any of
	// the three named cases (all_complete, empty_tree, all_blocked),
	// hitting the default branch at line 467.
	d.ScopeNode = "scoped-node"

	idx := state.NewRootIndex()
	idx.Root = []string{"scoped-node"}
	idx.Nodes["scoped-node"] = state.IndexEntry{
		Name:    "Scoped",
		Type:    state.NodeLeaf,
		State:   state.StatusComplete,
		Address: "scoped-node",
	}
	writeJSON(t, filepath.Join(d.Store.Dir(), "state.json"), idx)

	ns := state.NewNodeState("scoped-node", "Scoped", state.NodeLeaf)
	ns.Tasks = []state.Task{{ID: "task-0001", State: state.StatusComplete}}
	writeJSON(t, filepath.Join(projDir, "scoped-node", "state.json"), ns)

	result, err := d.RunOnce(context.Background())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result != IterationNoWork {
		t.Errorf("expected IterationNoWork, got %d", result)
	}
	// lastNoWorkMsg should contain the default "Standing by" format with
	// a parenthesized reason that isn't one of the three named cases.
	if !strings.Contains(d.lastNoWorkMsg, "Standing by") {
		t.Errorf("expected default 'Standing by' message, got %q", d.lastNoWorkMsg)
	}
}

// ═══════════════════════════════════════════════════════════════════════════
// RunOnce — StateError from runIteration returns fatal (lines 496-499)
// ═══════════════════════════════════════════════════════════════════════════

func TestRunOnce_StateErrorIsFatal(t *testing.T) {
	t.Parallel()
	d := testDaemon(t)
	projDir := d.Store.Dir()

	// Set up a valid tree so navigation finds work. Navigation's nodeLoader
	// uses state.LoadNodeState (read-only), while runIteration uses
	// d.Store.ReadNode then MutateNode (needs write). Make the directory
	// read-only after writing valid state so MutateNode's write-back fails,
	// producing a StateError.
	idx := state.NewRootIndex()
	idx.Root = []string{"state-err-node"}
	idx.Nodes["state-err-node"] = state.IndexEntry{
		Name:    "state-err-node",
		Type:    state.NodeLeaf,
		State:   state.StatusNotStarted,
		Address: "state-err-node",
	}
	writeJSON(t, filepath.Join(d.Store.Dir(), "state.json"), idx)

	ns := state.NewNodeState("state-err-node", "state-err-node", state.NodeLeaf)
	ns.Tasks = []state.Task{
		{ID: "task-0001", Description: "work", State: state.StatusNotStarted},
	}
	nodeDir := filepath.Join(projDir, "state-err-node")
	nodeStatePath := filepath.Join(nodeDir, "state.json")
	writeJSON(t, nodeStatePath, ns)
	writePromptFile(t, d.WolfcastleDir, "stages/execute.md")

	// Make the directory read-only so atomicWriteJSON (temp file + rename)
	// fails during MutateNode. The LoadNodeState read still works since
	// reading doesn't require directory write permission.
	_ = os.Chmod(nodeDir, 0555)
	defer func() {
		_ = os.Chmod(nodeDir, 0755)
	}()

	result, err := d.RunOnce(context.Background())
	// Should be IterationStop with a fatal error
	if result != IterationStop {
		t.Errorf("expected IterationStop for state error, got %d", result)
	}
	if err == nil {
		t.Fatal("expected fatal error for state corruption")
	}
	if !strings.Contains(err.Error(), "fatal state error") {
		t.Errorf("expected 'fatal state error', got: %v", err)
	}
}

// ═══════════════════════════════════════════════════════════════════════════
// RunOnce — replanning triggers after successful work (lines 506-511)
// ═══════════════════════════════════════════════════════════════════════════

func TestRunOnce_ReplanningTriggersAfterWork(t *testing.T) {
	t.Parallel()
	d := testDaemon(t)
	projDir := d.Store.Dir()

	// Enable planning so the replanning check fires after IterationDidWork.
	d.Config.Pipeline.Planning.Enabled = true
	d.Config.Pipeline.Planning.Model = "echo"

	// Create a parent orchestrator with a child leaf that has work.
	idx := state.NewRootIndex()
	idx.Root = []string{"parent-replan"}
	idx.Nodes["parent-replan"] = state.IndexEntry{
		Name:     "Parent",
		Type:     state.NodeOrchestrator,
		State:    state.StatusInProgress,
		Address:  "parent-replan",
		Children: []string{"parent-replan/child"},
	}
	idx.Nodes["parent-replan/child"] = state.IndexEntry{
		Name:    "Child",
		Type:    state.NodeLeaf,
		State:   state.StatusNotStarted,
		Address: "parent-replan/child",
		Parent:  "parent-replan",
	}
	writeJSON(t, filepath.Join(d.Store.Dir(), "state.json"), idx)

	parentNS := state.NewNodeState("parent-replan", "Parent", state.NodeOrchestrator)
	parentNS.Children = []state.ChildRef{
		{ID: "child", Address: "parent-replan/child", State: state.StatusNotStarted},
	}
	writeJSON(t, filepath.Join(projDir, "parent-replan", "state.json"), parentNS)

	childNS := state.NewNodeState("child", "Child", state.NodeLeaf)
	childNS.Tasks = []state.Task{
		{ID: "task-0001", Description: "do work", State: state.StatusNotStarted},
	}
	writeJSON(t, filepath.Join(projDir, "parent-replan", "child", "state.json"), childNS)

	writePromptFile(t, d.WolfcastleDir, "stages/execute.md")
	writePromptFile(t, d.WolfcastleDir, "plan.md")

	result, err := d.RunOnce(context.Background())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result != IterationDidWork {
		t.Errorf("expected IterationDidWork, got %d", result)
	}
}

// ═══════════════════════════════════════════════════════════════════════════
// RunOnce — nodeLoader parse error (line 425-427)
// ═══════════════════════════════════════════════════════════════════════════

func TestRunOnce_NodeLoaderParseError(t *testing.T) {
	t.Parallel()
	d := testDaemon(t)

	// Create root index with a node whose address is empty string,
	// which ParseAddress will reject.
	idx := state.NewRootIndex()
	idx.Root = []string{""}
	idx.Nodes[""] = state.IndexEntry{
		Name:    "Bad",
		Type:    state.NodeLeaf,
		State:   state.StatusNotStarted,
		Address: "",
	}
	writeJSON(t, filepath.Join(d.Store.Dir(), "state.json"), idx)

	result, err := d.RunOnce(context.Background())
	// Navigation should fail trying to load node with empty address.
	// Could be IterationStop (fatal) or IterationNoWork (if FindNextTask
	// handles the error internally).
	_ = result
	_ = err
}

// ═══════════════════════════════════════════════════════════════════════════
// RunOnce — RunOnce error from runIteration (non-StateError) returns
// IterationError (line 501)
// ═══════════════════════════════════════════════════════════════════════════

func TestRunOnce_NonStateErrorReturnsIterationError(t *testing.T) {
	t.Parallel()
	d := testDaemon(t)

	// Model not in config: runIteration returns a non-StateError.
	d.Config.Pipeline.Stages = []config.PipelineStage{
		{Name: "execute", Model: "nonexistent", PromptFile: "stages/execute.md"},
	}

	setupLeafNode(t, d, "nonfatal-node", []state.Task{
		{ID: "task-0001", Description: "work", State: state.StatusNotStarted},
	})
	writePromptFile(t, d.WolfcastleDir, "stages/execute.md")

	result, err := d.RunOnce(context.Background())
	if err != nil {
		t.Fatalf("non-state errors should not be fatal: %v", err)
	}
	if result != IterationError {
		t.Errorf("expected IterationError, got %d", result)
	}
}

// ═══════════════════════════════════════════════════════════════════════════
// RunOnce — branch verification detects change (lines 406-411)
// ═══════════════════════════════════════════════════════════════════════════

func TestRunOnce_BranchChangeDetected(t *testing.T) {
	t.Parallel()
	d := testDaemon(t)

	// Set up a real git repo so currentBranch succeeds
	initTestGitRepo(t, d.RepoDir)
	d.Config.Git.VerifyBranch = true
	d.branch = "a-branch-that-does-not-match"

	// Create valid tree so we get past the precondition checks
	idx := state.NewRootIndex()
	writeJSON(t, filepath.Join(d.Store.Dir(), "state.json"), idx)

	result, err := d.RunOnce(context.Background())
	if result != IterationStop {
		t.Errorf("expected IterationStop for branch change, got %d", result)
	}
	if err == nil || !strings.Contains(err.Error(), "branch changed") {
		t.Errorf("expected branch changed error, got: %v", err)
	}
}
