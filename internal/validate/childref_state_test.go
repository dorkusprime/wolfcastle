package validate

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/dorkusprime/wolfcastle/internal/state"
)

// buildOrchestratorTree creates an orchestrator with one child leaf.
// The orchestrator's ChildRef.State is set to childRefState, while the
// child's actual on-disk (and index) state is set to actualChildState.
func buildOrchestratorTree(t *testing.T, childRefState, actualChildState state.NodeStatus) (string, *state.RootIndex) {
	t.Helper()
	dir := t.TempDir()

	// Child leaf node on disk
	childDir := filepath.Join(dir, "child-a")
	_ = os.MkdirAll(childDir, 0755)
	childNS := state.NewNodeState("child-a", "Child A", state.NodeLeaf)
	childNS.State = actualChildState
	childNS.Tasks = []state.Task{
		{ID: "task-0001", Description: "do work", State: actualChildState},
		{ID: "audit", Description: "audit", State: actualChildState, IsAudit: true},
	}
	_ = state.SaveNodeState(filepath.Join(childDir, "state.json"), childNS)

	// Orchestrator node on disk, ChildRef carries the (possibly stale) state.
	orchDir := filepath.Join(dir, "orch")
	_ = os.MkdirAll(orchDir, 0755)
	orchNS := state.NewNodeState("orch", "Orchestrator", state.NodeOrchestrator)
	orchNS.Children = []state.ChildRef{
		{ID: "child-a", Address: "child-a", State: childRefState},
	}
	orchNS.State = state.RecomputeState(orchNS.Children, orchNS.Tasks)
	_ = state.SaveNodeState(filepath.Join(orchDir, "state.json"), orchNS)

	idx := state.NewRootIndex()
	idx.Root = []string{"orch"}
	idx.Nodes["orch"] = state.IndexEntry{
		Name:     "Orchestrator",
		Type:     state.NodeOrchestrator,
		State:    orchNS.State,
		Address:  "orch",
		Children: []string{"child-a"},
	}
	idx.Nodes["child-a"] = state.IndexEntry{
		Name:    "Child A",
		Type:    state.NodeLeaf,
		State:   actualChildState,
		Address: "child-a",
		Parent:  "orch",
	}

	return dir, idx
}

func TestChildRefStateMismatch_Detected(t *testing.T) {
	t.Parallel()
	dir, idx := buildOrchestratorTree(t, state.StatusComplete, state.StatusNotStarted)

	engine := NewEngine(dir, DefaultNodeLoader(dir))
	report := engine.ValidateAll(idx)

	found := false
	for _, issue := range report.Issues {
		if issue.Category == CatChildRefStateMismatch && issue.Node == "orch" {
			found = true
			if issue.Severity != SeverityError {
				t.Errorf("expected severity error, got %s", issue.Severity)
			}
			if !issue.CanAutoFix {
				t.Error("expected CanAutoFix to be true")
			}
			if issue.FixType != FixDeterministic {
				t.Errorf("expected fix type deterministic, got %s", issue.FixType)
			}
		}
	}
	if !found {
		t.Error("expected CHILDREF_STATE_MISMATCH for orch, but doctor found no such issue")
		for _, issue := range report.Issues {
			t.Logf("  found: [%s] %s: %s", issue.Category, issue.Node, issue.Description)
		}
	}
}

func TestChildRefStateMismatch_NoFalsePositive(t *testing.T) {
	t.Parallel()
	dir, idx := buildOrchestratorTree(t, state.StatusNotStarted, state.StatusNotStarted)

	engine := NewEngine(dir, DefaultNodeLoader(dir))
	report := engine.ValidateAll(idx)

	for _, issue := range report.Issues {
		if issue.Category == CatChildRefStateMismatch {
			t.Errorf("unexpected CHILDREF_STATE_MISMATCH: %s", issue.Description)
		}
	}
}

func TestChildRefStateMismatch_FixUpdatesParent(t *testing.T) {
	t.Parallel()
	dir, idx := buildOrchestratorTree(t, state.StatusComplete, state.StatusNotStarted)

	indexPath := filepath.Join(dir, "root-index.json")
	_ = state.SaveRootIndex(indexPath, idx)

	fixes, finalReport, err := FixWithVerification(dir, indexPath, DefaultNodeLoader(dir))
	if err != nil {
		t.Fatalf("FixWithVerification failed: %v", err)
	}

	// Verify a CHILDREF_STATE_MISMATCH fix was applied.
	foundFix := false
	for _, f := range fixes {
		if f.Category == CatChildRefStateMismatch {
			foundFix = true
		}
	}
	if !foundFix {
		t.Error("expected a CHILDREF_STATE_MISMATCH fix to be applied")
	}

	// After fix, the orchestrator should have no CHILDREF_STATE_MISMATCH issues.
	for _, issue := range finalReport.Issues {
		if issue.Category == CatChildRefStateMismatch {
			t.Errorf("CHILDREF_STATE_MISMATCH still present after fix: %s", issue.Description)
		}
	}

	// Verify the orchestrator's on-disk state was recomputed.
	orchNS, err := state.LoadNodeState(filepath.Join(dir, "orch", "state.json"))
	if err != nil {
		t.Fatalf("loading orch state: %v", err)
	}
	if orchNS.Children[0].State != state.StatusNotStarted {
		t.Errorf("expected ChildRef state not_started, got %s", orchNS.Children[0].State)
	}
	if orchNS.State != state.StatusNotStarted {
		t.Errorf("expected orchestrator state not_started after recompute, got %s", orchNS.State)
	}
}

func TestChildRefStateMismatch_InStartupCategories(t *testing.T) {
	t.Parallel()
	if !StartupCategories[CatChildRefStateMismatch] {
		t.Error("CatChildRefStateMismatch should be in StartupCategories")
	}
}
