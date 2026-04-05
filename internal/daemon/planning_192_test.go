package daemon

import (
	"context"
	"path/filepath"
	"testing"

	"github.com/dorkusprime/wolfcastle/internal/config"
	"github.com/dorkusprime/wolfcastle/internal/state"
)

// Regression test for #192: an orchestrator whose children are all
// not_started should derive to not_started after planning completes,
// not in_progress.
func TestRunPlanningPass_CompleteWithNotStartedChildren(t *testing.T) {
	t.Parallel()
	d := testDaemon(t)
	d.Config.Pipeline.Planning.Enabled = true
	d.Config.Pipeline.Planning.Model = "echo"
	d.Config.Models["echo"] = config.ModelDef{Command: "echo", Args: []string{"WOLFCASTLE_COMPLETE"}}

	projDir := d.Store.Dir()

	// Set up an orchestrator with two not_started children and an audit task.
	ns := state.NewNodeState("orch", "Orch", state.NodeOrchestrator)
	ns.NeedsPlanning = true
	ns.State = state.StatusNotStarted
	ns.Tasks = []state.Task{{ID: "audit-0001", Title: "Audit", IsAudit: true, State: state.StatusNotStarted}}
	ns.Children = []state.ChildRef{
		{ID: "alpha", Address: "orch/alpha", State: state.StatusNotStarted},
		{ID: "beta", Address: "orch/beta", State: state.StatusNotStarted},
	}
	writeJSON(t, filepath.Join(projDir, "orch", "state.json"), ns)

	idx := state.NewRootIndex()
	idx.Root = []string{"orch"}
	idx.Nodes["orch"] = state.IndexEntry{
		Name: "Orch", Type: state.NodeOrchestrator, State: state.StatusNotStarted,
		Address: "orch", Children: []string{"orch/alpha", "orch/beta"},
	}
	idx.Nodes["orch/alpha"] = state.IndexEntry{
		Name: "Alpha", Type: state.NodeLeaf, State: state.StatusNotStarted,
		Address: "orch/alpha", Parent: "orch",
	}
	idx.Nodes["orch/beta"] = state.IndexEntry{
		Name: "Beta", Type: state.NodeLeaf, State: state.StatusNotStarted,
		Address: "orch/beta", Parent: "orch",
	}
	writeJSON(t, filepath.Join(projDir, "state.json"), idx)

	writePromptFile(t, d.WolfcastleDir, "stages/plan-initial.md")

	_ = d.Logger.StartIteration()
	defer d.Logger.Close()

	if err := d.runPlanningPass(context.Background(), "orch", ns, idx); err != nil {
		t.Fatalf("runPlanningPass failed: %v", err)
	}

	updated, err := d.Store.ReadNode("orch")
	if err != nil {
		t.Fatalf("failed to read node: %v", err)
	}

	// The orchestrator's audit task must NOT be completed when children
	// are all not_started.
	for _, task := range updated.Tasks {
		if task.IsAudit && task.State == state.StatusComplete {
			t.Error("audit task should not be completed when children are all not_started")
		}
	}

	// The orchestrator itself must be not_started, not in_progress.
	if updated.State != state.StatusNotStarted {
		t.Errorf("orchestrator state = %s, want not_started (children are all not_started)", updated.State)
	}
}

// Verify the audit task IS completed when all children are complete.
func TestRunPlanningPass_CompleteWithAllChildrenComplete(t *testing.T) {
	t.Parallel()
	d := testDaemon(t)
	d.Config.Pipeline.Planning.Enabled = true
	d.Config.Pipeline.Planning.Model = "echo"
	d.Config.Models["echo"] = config.ModelDef{Command: "echo", Args: []string{"WOLFCASTLE_COMPLETE"}}

	projDir := d.Store.Dir()

	ns := state.NewNodeState("orch", "Orch", state.NodeOrchestrator)
	ns.NeedsPlanning = true
	ns.State = state.StatusInProgress
	ns.Tasks = []state.Task{{ID: "audit-0001", Title: "Audit", IsAudit: true, State: state.StatusNotStarted}}
	ns.Children = []state.ChildRef{
		{ID: "alpha", Address: "orch/alpha", State: state.StatusComplete},
		{ID: "beta", Address: "orch/beta", State: state.StatusComplete},
	}
	writeJSON(t, filepath.Join(projDir, "orch", "state.json"), ns)

	idx := state.NewRootIndex()
	idx.Root = []string{"orch"}
	idx.Nodes["orch"] = state.IndexEntry{
		Name: "Orch", Type: state.NodeOrchestrator, State: state.StatusInProgress,
		Address: "orch", Children: []string{"orch/alpha", "orch/beta"},
	}
	idx.Nodes["orch/alpha"] = state.IndexEntry{
		Name: "Alpha", Type: state.NodeLeaf, State: state.StatusComplete,
		Address: "orch/alpha", Parent: "orch",
	}
	idx.Nodes["orch/beta"] = state.IndexEntry{
		Name: "Beta", Type: state.NodeLeaf, State: state.StatusComplete,
		Address: "orch/beta", Parent: "orch",
	}
	writeJSON(t, filepath.Join(projDir, "state.json"), idx)

	writePromptFile(t, d.WolfcastleDir, "stages/plan-initial.md")

	_ = d.Logger.StartIteration()
	defer d.Logger.Close()

	if err := d.runPlanningPass(context.Background(), "orch", ns, idx); err != nil {
		t.Fatalf("runPlanningPass failed: %v", err)
	}

	updated, err := d.Store.ReadNode("orch")
	if err != nil {
		t.Fatalf("failed to read node: %v", err)
	}

	// Audit task should be complete when all children are complete.
	auditComplete := false
	for _, task := range updated.Tasks {
		if task.IsAudit && task.State == state.StatusComplete {
			auditComplete = true
		}
	}
	if !auditComplete {
		t.Error("audit task should be completed when all children are complete")
	}

	// The orchestrator should derive to complete.
	if updated.State != state.StatusComplete {
		t.Errorf("orchestrator state = %s, want complete", updated.State)
	}
}

// Mixed children: some complete, some not_started. Orchestrator should
// be in_progress, audit should stay not_started.
func TestRunPlanningPass_CompleteWithMixedChildren(t *testing.T) {
	t.Parallel()
	d := testDaemon(t)
	d.Config.Pipeline.Planning.Enabled = true
	d.Config.Pipeline.Planning.Model = "echo"
	d.Config.Models["echo"] = config.ModelDef{Command: "echo", Args: []string{"WOLFCASTLE_COMPLETE"}}

	projDir := d.Store.Dir()

	ns := state.NewNodeState("orch", "Orch", state.NodeOrchestrator)
	ns.NeedsPlanning = true
	ns.State = state.StatusInProgress
	ns.Tasks = []state.Task{{ID: "audit-0001", Title: "Audit", IsAudit: true, State: state.StatusNotStarted}}
	ns.Children = []state.ChildRef{
		{ID: "alpha", Address: "orch/alpha", State: state.StatusComplete},
		{ID: "beta", Address: "orch/beta", State: state.StatusNotStarted},
	}
	writeJSON(t, filepath.Join(projDir, "orch", "state.json"), ns)

	idx := state.NewRootIndex()
	writeJSON(t, filepath.Join(projDir, "state.json"), idx)

	writePromptFile(t, d.WolfcastleDir, "stages/plan-initial.md")

	_ = d.Logger.StartIteration()
	defer d.Logger.Close()

	if err := d.runPlanningPass(context.Background(), "orch", ns, idx); err != nil {
		t.Fatalf("runPlanningPass failed: %v", err)
	}

	updated, err := d.Store.ReadNode("orch")
	if err != nil {
		t.Fatalf("failed to read node: %v", err)
	}

	// Audit task should stay not_started with mixed children.
	for _, task := range updated.Tasks {
		if task.IsAudit && task.State == state.StatusComplete {
			t.Error("audit task should not be completed when children are mixed")
		}
	}

	// Orchestrator should be in_progress (mixed children).
	if updated.State != state.StatusInProgress {
		t.Errorf("orchestrator state = %s, want in_progress (mixed children)", updated.State)
	}
}
