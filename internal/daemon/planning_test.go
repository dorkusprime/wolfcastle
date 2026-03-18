package daemon

import (
	"context"
	"path/filepath"
	"testing"

	"github.com/dorkusprime/wolfcastle/internal/config"
	"github.com/dorkusprime/wolfcastle/internal/state"
)

// ═══════════════════════════════════════════════════════════════════════════
// findPlanningTarget
// ═══════════════════════════════════════════════════════════════════════════

func TestFindPlanningTarget_FindsNeedsPlanningOrchestrator(t *testing.T) {
	t.Parallel()
	d := testDaemon(t)
	d.Config.Pipeline.Planning.Enabled = true

	projDir := d.Resolver.ProjectsDir()
	idx := state.NewRootIndex()
	idx.Root = []string{"orch-node"}
	idx.Nodes["orch-node"] = state.IndexEntry{
		Name: "Orch", Type: state.NodeOrchestrator, State: state.StatusInProgress, Address: "orch-node",
	}
	writeJSON(t, d.Resolver.RootIndexPath(), idx)

	ns := state.NewNodeState("orch-node", "Orch", state.NodeOrchestrator)
	ns.NeedsPlanning = true
	ns.Scope = "test scope"
	writeJSON(t, filepath.Join(projDir, "orch-node", "state.json"), ns)

	addr, foundNS := d.findPlanningTarget(idx)
	if addr != "orch-node" {
		t.Errorf("expected address 'orch-node', got %q", addr)
	}
	if foundNS == nil {
		t.Fatal("expected non-nil node state")
	}
	if !foundNS.NeedsPlanning {
		t.Error("expected NeedsPlanning to be true on returned state")
	}
}

func TestFindPlanningTarget_ReturnsNilWhenDisabled(t *testing.T) {
	t.Parallel()
	d := testDaemon(t)
	// Planning is disabled by default in testConfig

	projDir := d.Resolver.ProjectsDir()
	idx := state.NewRootIndex()
	idx.Root = []string{"orch-node"}
	idx.Nodes["orch-node"] = state.IndexEntry{
		Name: "Orch", Type: state.NodeOrchestrator, State: state.StatusInProgress, Address: "orch-node",
	}
	writeJSON(t, d.Resolver.RootIndexPath(), idx)

	ns := state.NewNodeState("orch-node", "Orch", state.NodeOrchestrator)
	ns.NeedsPlanning = true
	writeJSON(t, filepath.Join(projDir, "orch-node", "state.json"), ns)

	addr, foundNS := d.findPlanningTarget(idx)
	if addr != "" {
		t.Errorf("expected empty address when planning disabled, got %q", addr)
	}
	if foundNS != nil {
		t.Error("expected nil node state when planning disabled")
	}
}

func TestFindPlanningTarget_SkipsCompletedOrchestrators(t *testing.T) {
	t.Parallel()
	d := testDaemon(t)
	d.Config.Pipeline.Planning.Enabled = true

	projDir := d.Resolver.ProjectsDir()
	idx := state.NewRootIndex()
	idx.Root = []string{"orch-node"}
	idx.Nodes["orch-node"] = state.IndexEntry{
		Name: "Orch", Type: state.NodeOrchestrator, State: state.StatusComplete, Address: "orch-node",
	}
	writeJSON(t, d.Resolver.RootIndexPath(), idx)

	ns := state.NewNodeState("orch-node", "Orch", state.NodeOrchestrator)
	ns.NeedsPlanning = true
	ns.State = state.StatusComplete
	writeJSON(t, filepath.Join(projDir, "orch-node", "state.json"), ns)

	addr, foundNS := d.findPlanningTarget(idx)
	if addr != "" {
		t.Errorf("expected empty address for completed orchestrator, got %q", addr)
	}
	if foundNS != nil {
		t.Error("expected nil node state for completed orchestrator")
	}
}

func TestFindPlanningTarget_InfersNeedFromChildlessOrchestrator(t *testing.T) {
	t.Parallel()
	d := testDaemon(t)
	d.Config.Pipeline.Planning.Enabled = true

	projDir := d.Resolver.ProjectsDir()
	idx := state.NewRootIndex()
	idx.Root = []string{"orch-node"}
	idx.Nodes["orch-node"] = state.IndexEntry{
		Name: "Orch", Type: state.NodeOrchestrator, State: state.StatusNotStarted, Address: "orch-node",
	}
	writeJSON(t, d.Resolver.RootIndexPath(), idx)

	// Orchestrator with no children, no tasks, and NeedsPlanning=false.
	// The daemon should infer it needs planning from the empty structure.
	ns := state.NewNodeState("orch-node", "Orch", state.NodeOrchestrator)
	ns.NeedsPlanning = false
	ns.Scope = "some scope"
	writeJSON(t, filepath.Join(projDir, "orch-node", "state.json"), ns)

	addr, foundNS := d.findPlanningTarget(idx)
	if addr != "orch-node" {
		t.Errorf("expected childless orchestrator to be inferred as planning target, got %q", addr)
	}
	if foundNS != nil && foundNS.PlanningTrigger != "initial" {
		t.Errorf("expected inferred trigger 'initial', got %q", foundNS.PlanningTrigger)
	}
}

// ═══════════════════════════════════════════════════════════════════════════
// runPlanningPass
// ═══════════════════════════════════════════════════════════════════════════

func TestRunPlanningPass_Complete(t *testing.T) {
	t.Parallel()
	d := testDaemon(t)
	d.Config.Pipeline.Planning.Enabled = true
	d.Config.Pipeline.Planning.Model = "echo"
	d.Config.Models["echo"] = config.ModelDef{Command: "echo", Args: []string{"WOLFCASTLE_COMPLETE"}}

	projDir := d.Resolver.ProjectsDir()
	idx := state.NewRootIndex()
	idx.Root = []string{"orch-node"}
	idx.Nodes["orch-node"] = state.IndexEntry{
		Name: "Orch", Type: state.NodeOrchestrator, State: state.StatusInProgress, Address: "orch-node",
	}
	writeJSON(t, d.Resolver.RootIndexPath(), idx)

	ns := state.NewNodeState("orch-node", "Orch", state.NodeOrchestrator)
	ns.NeedsPlanning = true
	ns.State = state.StatusInProgress
	ns.Scope = "test scope"
	writeJSON(t, filepath.Join(projDir, "orch-node", "state.json"), ns)

	writePromptFile(t, d.WolfcastleDir, "plan-initial.md")
	writePromptFile(t, d.WolfcastleDir, "plan-amend.md")
	writePromptFile(t, d.WolfcastleDir, "plan-remediate.md")
	writePromptFile(t, d.WolfcastleDir, "plan-review.md")

	_ = d.Logger.StartIteration()
	defer d.Logger.Close()

	err := d.runPlanningPass(context.Background(), "orch-node", ns, idx)
	if err != nil {
		t.Fatalf("runPlanningPass error: %v", err)
	}

	updated, err := d.Store.ReadNode("orch-node")
	if err != nil {
		t.Fatalf("failed to read node state: %v", err)
	}
	if updated.NeedsPlanning {
		t.Error("NeedsPlanning should be cleared after COMPLETE marker")
	}
	if len(updated.PlanningHistory) == 0 {
		t.Error("PlanningHistory should have at least one entry")
	}
}

func TestRunPlanningPass_Blocked(t *testing.T) {
	t.Parallel()
	d := testDaemon(t)
	d.Config.Pipeline.Planning.Enabled = true
	d.Config.Pipeline.Planning.Model = "echo"
	d.Config.Models["echo"] = config.ModelDef{Command: "echo", Args: []string{"WOLFCASTLE_BLOCKED"}}

	projDir := d.Resolver.ProjectsDir()
	idx := state.NewRootIndex()
	idx.Root = []string{"orch-node"}
	idx.Nodes["orch-node"] = state.IndexEntry{
		Name: "Orch", Type: state.NodeOrchestrator, State: state.StatusInProgress, Address: "orch-node",
	}
	writeJSON(t, d.Resolver.RootIndexPath(), idx)

	ns := state.NewNodeState("orch-node", "Orch", state.NodeOrchestrator)
	ns.NeedsPlanning = true
	ns.State = state.StatusInProgress
	ns.Scope = "test scope"
	writeJSON(t, filepath.Join(projDir, "orch-node", "state.json"), ns)

	writePromptFile(t, d.WolfcastleDir, "plan-initial.md")

	_ = d.Logger.StartIteration()
	defer d.Logger.Close()

	err := d.runPlanningPass(context.Background(), "orch-node", ns, idx)
	if err != nil {
		t.Fatalf("runPlanningPass error: %v", err)
	}

	updated, err := d.Store.ReadNode("orch-node")
	if err != nil {
		t.Fatalf("failed to read node state: %v", err)
	}
	if updated.NeedsPlanning {
		t.Error("NeedsPlanning should be cleared after BLOCKED marker")
	}
	if updated.State != state.StatusBlocked {
		t.Errorf("expected node state StatusBlocked, got %s", updated.State)
	}
}

func TestRunPlanningPass_Continue(t *testing.T) {
	t.Parallel()
	d := testDaemon(t)
	d.Config.Pipeline.Planning.Enabled = true
	d.Config.Pipeline.Planning.Model = "echo"
	d.Config.Models["echo"] = config.ModelDef{Command: "echo", Args: []string{"WOLFCASTLE_CONTINUE"}}

	projDir := d.Resolver.ProjectsDir()
	idx := state.NewRootIndex()
	idx.Root = []string{"orch-node"}
	idx.Nodes["orch-node"] = state.IndexEntry{
		Name: "Orch", Type: state.NodeOrchestrator, State: state.StatusInProgress, Address: "orch-node",
	}
	writeJSON(t, d.Resolver.RootIndexPath(), idx)

	ns := state.NewNodeState("orch-node", "Orch", state.NodeOrchestrator)
	ns.NeedsPlanning = true
	ns.State = state.StatusInProgress
	ns.Scope = "test scope"
	writeJSON(t, filepath.Join(projDir, "orch-node", "state.json"), ns)

	writePromptFile(t, d.WolfcastleDir, "plan-initial.md")

	_ = d.Logger.StartIteration()
	defer d.Logger.Close()

	err := d.runPlanningPass(context.Background(), "orch-node", ns, idx)
	if err != nil {
		t.Fatalf("runPlanningPass error: %v", err)
	}

	updated, err := d.Store.ReadNode("orch-node")
	if err != nil {
		t.Fatalf("failed to read node state: %v", err)
	}
	if updated.NeedsPlanning {
		t.Error("NeedsPlanning should be cleared after CONTINUE marker")
	}
}

// ═══════════════════════════════════════════════════════════════════════════
// selectPlanningPrompt
// ═══════════════════════════════════════════════════════════════════════════

func TestSelectPlanningPrompt(t *testing.T) {
	t.Parallel()

	cases := []struct {
		trigger  string
		expected string
	}{
		{"", "plan-initial.md"},
		{"initial", "plan-initial.md"},
		{"new_scope", "plan-amend.md"},
		{"child_blocked", "plan-remediate.md"},
		{"completion_review", "plan-review.md"},
	}

	for _, tc := range cases {
		t.Run(tc.trigger, func(t *testing.T) {
			got := selectPlanningPrompt(tc.trigger)
			if got != tc.expected {
				t.Errorf("selectPlanningPrompt(%q) = %q, want %q", tc.trigger, got, tc.expected)
			}
		})
	}
}

// ═══════════════════════════════════════════════════════════════════════════
// checkReplanningTriggers
// ═══════════════════════════════════════════════════════════════════════════

func TestCheckReplanningTriggers_CompletionReview(t *testing.T) {
	t.Parallel()
	d := testDaemon(t)
	d.Config.Pipeline.Planning.Enabled = true
	_ = d.Logger.StartIteration()
	defer d.Logger.Close()

	projDir := d.Resolver.ProjectsDir()
	idx := state.NewRootIndex()
	idx.Root = []string{"parent"}
	idx.Nodes["parent"] = state.IndexEntry{
		Name: "Parent", Type: state.NodeOrchestrator, State: state.StatusInProgress,
		Address: "parent", Children: []string{"parent/child-a", "parent/child-b"},
	}
	idx.Nodes["parent/child-a"] = state.IndexEntry{
		Name: "ChildA", Type: state.NodeLeaf, State: state.StatusComplete,
		Address: "parent/child-a", Parent: "parent",
	}
	idx.Nodes["parent/child-b"] = state.IndexEntry{
		Name: "ChildB", Type: state.NodeLeaf, State: state.StatusComplete,
		Address: "parent/child-b", Parent: "parent",
	}
	writeJSON(t, d.Resolver.RootIndexPath(), idx)

	parentNS := state.NewNodeState("parent", "Parent", state.NodeOrchestrator)
	parentNS.State = state.StatusInProgress
	parentNS.SuccessCriteria = []string{"all tests pass", "docs updated"}
	parentNS.Children = []state.ChildRef{
		{ID: "child-a", Address: "parent/child-a", State: state.StatusComplete},
		{ID: "child-b", Address: "parent/child-b", State: state.StatusComplete},
	}
	writeJSON(t, filepath.Join(projDir, "parent", "state.json"), parentNS)

	childNS := state.NewNodeState("child-a", "ChildA", state.NodeLeaf)
	childNS.State = state.StatusComplete
	writeJSON(t, filepath.Join(projDir, "parent", "child-a", "state.json"), childNS)

	childBNS := state.NewNodeState("child-b", "ChildB", state.NodeLeaf)
	childBNS.State = state.StatusComplete
	writeJSON(t, filepath.Join(projDir, "parent", "child-b", "state.json"), childBNS)

	d.checkReplanningTriggers("parent/child-a", "", idx)

	updated, err := d.Store.ReadNode("parent")
	if err != nil {
		t.Fatalf("failed to read parent state: %v", err)
	}
	if !updated.NeedsPlanning {
		t.Error("parent NeedsPlanning should be set for completion review")
	}
	if updated.PlanningTrigger != "completion_review" {
		t.Errorf("expected trigger 'completion_review', got %q", updated.PlanningTrigger)
	}
}

func TestCheckReplanningTriggers_ChildBlocked(t *testing.T) {
	t.Parallel()
	d := testDaemon(t)
	d.Config.Pipeline.Planning.Enabled = true
	_ = d.Logger.StartIteration()
	defer d.Logger.Close()

	projDir := d.Resolver.ProjectsDir()
	idx := state.NewRootIndex()
	idx.Root = []string{"parent"}
	idx.Nodes["parent"] = state.IndexEntry{
		Name: "Parent", Type: state.NodeOrchestrator, State: state.StatusInProgress,
		Address: "parent", Children: []string{"parent/child-a", "parent/child-b"},
	}
	idx.Nodes["parent/child-a"] = state.IndexEntry{
		Name: "ChildA", Type: state.NodeLeaf, State: state.StatusComplete,
		Address: "parent/child-a", Parent: "parent",
	}
	idx.Nodes["parent/child-b"] = state.IndexEntry{
		Name: "ChildB", Type: state.NodeLeaf, State: state.StatusBlocked,
		Address: "parent/child-b", Parent: "parent",
	}
	writeJSON(t, d.Resolver.RootIndexPath(), idx)

	parentNS := state.NewNodeState("parent", "Parent", state.NodeOrchestrator)
	parentNS.State = state.StatusInProgress
	parentNS.Children = []state.ChildRef{
		{ID: "child-a", Address: "parent/child-a", State: state.StatusComplete},
		{ID: "child-b", Address: "parent/child-b", State: state.StatusBlocked},
	}
	writeJSON(t, filepath.Join(projDir, "parent", "state.json"), parentNS)

	childNS := state.NewNodeState("child-a", "ChildA", state.NodeLeaf)
	childNS.State = state.StatusComplete
	writeJSON(t, filepath.Join(projDir, "parent", "child-a", "state.json"), childNS)

	childBNS := state.NewNodeState("child-b", "ChildB", state.NodeLeaf)
	childBNS.State = state.StatusBlocked
	writeJSON(t, filepath.Join(projDir, "parent", "child-b", "state.json"), childBNS)

	d.checkReplanningTriggers("parent/child-b", "", idx)

	updated, err := d.Store.ReadNode("parent")
	if err != nil {
		t.Fatalf("failed to read parent state: %v", err)
	}
	if !updated.NeedsPlanning {
		t.Error("parent NeedsPlanning should be set for child blocked")
	}
	if updated.PlanningTrigger != "child_blocked" {
		t.Errorf("expected trigger 'child_blocked', got %q", updated.PlanningTrigger)
	}
}

// ═══════════════════════════════════════════════════════════════════════════
// incrementReplanCount
// ═══════════════════════════════════════════════════════════════════════════

func TestIncrementReplanCount_ExhaustsBudget(t *testing.T) {
	t.Parallel()
	d := testDaemon(t)
	_ = d.Logger.StartIteration()
	defer d.Logger.Close()

	projDir := d.Resolver.ProjectsDir()
	idx := state.NewRootIndex()
	idx.Root = []string{"orch-node"}
	idx.Nodes["orch-node"] = state.IndexEntry{
		Name: "Orch", Type: state.NodeOrchestrator, State: state.StatusInProgress, Address: "orch-node",
	}
	writeJSON(t, d.Resolver.RootIndexPath(), idx)

	ns := state.NewNodeState("orch-node", "Orch", state.NodeOrchestrator)
	ns.NeedsPlanning = true
	ns.State = state.StatusInProgress
	ns.MaxReplans = 2
	writeJSON(t, filepath.Join(projDir, "orch-node", "state.json"), ns)

	// First increment: should not block
	d.incrementReplanCount("orch-node", "initial")
	after1, err := d.Store.ReadNode("orch-node")
	if err != nil {
		t.Fatalf("failed to read node state: %v", err)
	}
	if after1.State == state.StatusBlocked {
		t.Error("node should not be blocked after first increment")
	}
	if after1.ReplanCount["initial"] != 1 {
		t.Errorf("expected replan count 1, got %d", after1.ReplanCount["initial"])
	}

	// Second increment: should exhaust budget and block
	d.incrementReplanCount("orch-node", "initial")
	after2, err := d.Store.ReadNode("orch-node")
	if err != nil {
		t.Fatalf("failed to read node state: %v", err)
	}
	if after2.State != state.StatusBlocked {
		t.Errorf("expected StatusBlocked after exhausting budget, got %s", after2.State)
	}
	if after2.ReplanCount["initial"] != 2 {
		t.Errorf("expected replan count 2, got %d", after2.ReplanCount["initial"])
	}
	if after2.NeedsPlanning {
		t.Error("NeedsPlanning should be cleared when budget exhausted")
	}
}
