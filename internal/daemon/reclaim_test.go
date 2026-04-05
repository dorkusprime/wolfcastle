package daemon

import (
	"path/filepath"
	"testing"

	"github.com/dorkusprime/wolfcastle/internal/state"
)

// Regression test for #201: in_progress tasks with no active worker
// should be reset to not_started by reclaimOrphans.
func TestReclaimOrphans_ResetsOrphanedTask(t *testing.T) {
	t.Parallel()
	d := testDaemon(t)
	pd := NewParallelDispatcher(d, 4)
	d.dispatcher = pd

	projDir := d.Store.Dir()
	_ = d.Logger.StartIteration()
	defer d.Logger.Close()

	// Set up a leaf with one in_progress task (orphaned, no worker).
	ns := state.NewNodeState("leaf", "Leaf", state.NodeLeaf)
	ns.State = state.StatusInProgress
	ns.Tasks = []state.Task{
		{ID: "task-0001", Title: "Orphaned", State: state.StatusInProgress},
		{ID: "audit-0001", Title: "Audit", IsAudit: true, State: state.StatusNotStarted},
	}
	writeJSON(t, filepath.Join(projDir, "leaf", "state.json"), ns)

	idx := state.NewRootIndex()
	idx.Root = []string{"leaf"}
	idx.Nodes["leaf"] = state.IndexEntry{
		Name: "Leaf", Type: state.NodeLeaf, State: state.StatusInProgress, Address: "leaf",
	}
	writeJSON(t, filepath.Join(projDir, "state.json"), idx)

	// No active workers in the dispatcher.
	reclaimed := pd.reclaimOrphans(idx)
	if reclaimed != 1 {
		t.Errorf("expected 1 reclaimed, got %d", reclaimed)
	}

	updated, err := d.Store.ReadNode("leaf")
	if err != nil {
		t.Fatalf("reading node: %v", err)
	}

	for _, task := range updated.Tasks {
		if task.ID == "task-0001" && task.State != state.StatusNotStarted {
			t.Errorf("task-0001 state = %s, want not_started", task.State)
		}
	}
	if updated.State != state.StatusNotStarted {
		t.Errorf("node state = %s, want not_started", updated.State)
	}
}

// Active workers should not be reclaimed.
func TestReclaimOrphans_SkipsActiveWorker(t *testing.T) {
	t.Parallel()
	d := testDaemon(t)
	pd := NewParallelDispatcher(d, 4)
	d.dispatcher = pd

	projDir := d.Store.Dir()
	_ = d.Logger.StartIteration()
	defer d.Logger.Close()

	ns := state.NewNodeState("leaf", "Leaf", state.NodeLeaf)
	ns.State = state.StatusInProgress
	ns.Tasks = []state.Task{
		{ID: "task-0001", Title: "Active", State: state.StatusInProgress},
	}
	writeJSON(t, filepath.Join(projDir, "leaf", "state.json"), ns)

	idx := state.NewRootIndex()
	idx.Root = []string{"leaf"}
	idx.Nodes["leaf"] = state.IndexEntry{
		Name: "Leaf", Type: state.NodeLeaf, State: state.StatusInProgress, Address: "leaf",
	}
	writeJSON(t, filepath.Join(projDir, "state.json"), idx)

	// Register an active worker for this task.
	pd.mu.Lock()
	pd.active["leaf/task-0001"] = &WorkerSlot{Node: "leaf", Task: "task-0001"}
	pd.mu.Unlock()

	reclaimed := pd.reclaimOrphans(idx)
	if reclaimed != 0 {
		t.Errorf("expected 0 reclaimed (worker active), got %d", reclaimed)
	}

	updated, _ := d.Store.ReadNode("leaf")
	for _, task := range updated.Tasks {
		if task.ID == "task-0001" && task.State != state.StatusInProgress {
			t.Errorf("active task should stay in_progress, got %s", task.State)
		}
	}
}

// Completed and not_started tasks should be ignored.
func TestReclaimOrphans_IgnoresNonInProgressTasks(t *testing.T) {
	t.Parallel()
	d := testDaemon(t)
	pd := NewParallelDispatcher(d, 4)
	d.dispatcher = pd

	projDir := d.Store.Dir()
	_ = d.Logger.StartIteration()
	defer d.Logger.Close()

	ns := state.NewNodeState("leaf", "Leaf", state.NodeLeaf)
	ns.State = state.StatusInProgress
	ns.Tasks = []state.Task{
		{ID: "task-0001", Title: "Done", State: state.StatusComplete},
		{ID: "task-0002", Title: "Waiting", State: state.StatusNotStarted},
	}
	writeJSON(t, filepath.Join(projDir, "leaf", "state.json"), ns)

	idx := state.NewRootIndex()
	idx.Root = []string{"leaf"}
	idx.Nodes["leaf"] = state.IndexEntry{
		Name: "Leaf", Type: state.NodeLeaf, State: state.StatusInProgress, Address: "leaf",
	}
	writeJSON(t, filepath.Join(projDir, "state.json"), idx)

	reclaimed := pd.reclaimOrphans(idx)
	if reclaimed != 0 {
		t.Errorf("expected 0 reclaimed (no in_progress tasks), got %d", reclaimed)
	}
}

// Orchestrators should be skipped (only leaf tasks are dispatched).
func TestReclaimOrphans_SkipsOrchestrators(t *testing.T) {
	t.Parallel()
	d := testDaemon(t)
	pd := NewParallelDispatcher(d, 4)
	d.dispatcher = pd

	projDir := d.Store.Dir()
	_ = d.Logger.StartIteration()
	defer d.Logger.Close()

	ns := state.NewNodeState("orch", "Orch", state.NodeOrchestrator)
	ns.State = state.StatusInProgress
	ns.Tasks = []state.Task{
		{ID: "task-0001", Title: "Orphaned", State: state.StatusInProgress},
	}
	writeJSON(t, filepath.Join(projDir, "orch", "state.json"), ns)

	idx := state.NewRootIndex()
	idx.Root = []string{"orch"}
	idx.Nodes["orch"] = state.IndexEntry{
		Name: "Orch", Type: state.NodeOrchestrator, State: state.StatusInProgress, Address: "orch",
	}
	writeJSON(t, filepath.Join(projDir, "state.json"), idx)

	reclaimed := pd.reclaimOrphans(idx)
	if reclaimed != 0 {
		t.Errorf("expected 0 reclaimed (orchestrator, not leaf), got %d", reclaimed)
	}
}
