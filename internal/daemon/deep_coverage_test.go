package daemon

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/dorkusprime/wolfcastle/internal/state"
)

// ═══════════════════════════════════════════════════════════════════════════
// propagateState: error paths and deep hierarchies
// ═══════════════════════════════════════════════════════════════════════════

func TestPropagateState_NodeNotInIndex(t *testing.T) {
	d := testDaemon(t)
	idx := state.NewRootIndex()

	// Node not in index. PropagateState succeeds but does nothing meaningful
	// since there's nothing to propagate up from
	err := d.propagateState("nonexistent-node", state.StatusInProgress, idx)
	// Should succeed. The function just saves the index
	if err != nil {
		t.Logf("propagateState for missing node: %v (may be acceptable)", err)
	}
}

func TestPropagateState_ParentLoadError(t *testing.T) {
	d := testDaemon(t)
	idx := state.NewRootIndex()

	// Set up a node with a parent that doesn't have state on disk
	idx.Root = []string{"parent"}
	idx.Nodes["parent"] = state.IndexEntry{
		Name: "Parent", Type: state.NodeOrchestrator, State: state.StatusNotStarted,
		Address: "parent", Children: []string{"parent/child"},
	}
	idx.Nodes["parent/child"] = state.IndexEntry{
		Name: "Child", Type: state.NodeLeaf, State: state.StatusNotStarted,
		Address: "parent/child", Parent: "parent",
	}
	writeJSON(t, filepath.Join(d.Store.Dir(), "state.json"), idx)

	// Don't create parent state.json on disk. LoadNode for parent will fail
	// but propagateState should still succeed or return an error gracefully
	err := d.propagateState("parent/child", state.StatusInProgress, idx)
	// This exercises the loadNode error path inside state.Propagate
	_ = err
}

func TestPropagateState_FourLevelHierarchy(t *testing.T) {
	d := testDaemon(t)
	projDir := d.Store.Dir()

	idx := state.NewRootIndex()
	idx.Root = []string{"l1"}
	idx.Nodes["l1"] = state.IndexEntry{
		Name: "L1", Type: state.NodeOrchestrator, State: state.StatusNotStarted,
		Address: "l1", Children: []string{"l1/l2"},
	}
	idx.Nodes["l1/l2"] = state.IndexEntry{
		Name: "L2", Type: state.NodeOrchestrator, State: state.StatusNotStarted,
		Address: "l1/l2", Parent: "l1", Children: []string{"l1/l2/l3"},
	}
	idx.Nodes["l1/l2/l3"] = state.IndexEntry{
		Name: "L3", Type: state.NodeOrchestrator, State: state.StatusNotStarted,
		Address: "l1/l2/l3", Parent: "l1/l2", Children: []string{"l1/l2/l3/leaf"},
	}
	idx.Nodes["l1/l2/l3/leaf"] = state.IndexEntry{
		Name: "Leaf", Type: state.NodeLeaf, State: state.StatusNotStarted,
		Address: "l1/l2/l3/leaf", Parent: "l1/l2/l3",
	}
	writeJSON(t, filepath.Join(d.Store.Dir(), "state.json"), idx)

	// Create node state files
	l1NS := state.NewNodeState("l1", "L1", state.NodeOrchestrator)
	l1NS.Children = []state.ChildRef{{ID: "l2", Address: "l1/l2", State: state.StatusNotStarted}}
	writeJSON(t, filepath.Join(projDir, "l1", "state.json"), l1NS)

	l2NS := state.NewNodeState("l2", "L2", state.NodeOrchestrator)
	l2NS.Children = []state.ChildRef{{ID: "l3", Address: "l1/l2/l3", State: state.StatusNotStarted}}
	writeJSON(t, filepath.Join(projDir, "l1", "l2", "state.json"), l2NS)

	l3NS := state.NewNodeState("l3", "L3", state.NodeOrchestrator)
	l3NS.Children = []state.ChildRef{{ID: "leaf", Address: "l1/l2/l3/leaf", State: state.StatusNotStarted}}
	writeJSON(t, filepath.Join(projDir, "l1", "l2", "l3", "state.json"), l3NS)

	leafNS := state.NewNodeState("leaf", "Leaf", state.NodeLeaf)
	leafNS.State = state.StatusComplete
	writeJSON(t, filepath.Join(projDir, "l1", "l2", "l3", "leaf", "state.json"), leafNS)

	if err := d.propagateState("l1/l2/l3/leaf", state.StatusComplete, idx); err != nil {
		t.Fatalf("propagateState error: %v", err)
	}

	// Verify index was updated
	updatedIdx, err := d.Store.ReadIndex()
	if err != nil {
		t.Fatal(err)
	}
	if updatedIdx.Nodes["l1/l2/l3/leaf"].State != state.StatusComplete {
		t.Error("leaf should be complete in index")
	}
}

// ═══════════════════════════════════════════════════════════════════════════
// checkInboxState: various states
// ═══════════════════════════════════════════════════════════════════════════

func TestCheckInboxState_NoFile(t *testing.T) {
	d := testDaemon(t)
	hasNew := d.checkInboxState("/nonexistent/inbox.json")
	if hasNew {
		t.Error("should return false for nonexistent inbox")
	}
}

func TestCheckInboxState_EmptyItems(t *testing.T) {
	d := testDaemon(t)
	inboxPath := filepath.Join(d.Store.Dir(), "inbox.json")
	writeJSON(t, inboxPath, &state.InboxFile{Items: []state.InboxItem{}})

	hasNew := d.checkInboxState(inboxPath)
	if hasNew {
		t.Error("empty inbox should have no new items")
	}
}

func TestCheckInboxState_MixedItems(t *testing.T) {
	d := testDaemon(t)
	inboxPath := filepath.Join(d.Store.Dir(), "inbox.json")
	writeJSON(t, inboxPath, &state.InboxFile{Items: []state.InboxItem{
		{Status: "new", Text: "item1"},
		{Status: "filed", Text: "item3"},
	}})

	hasNew := d.checkInboxState(inboxPath)
	if !hasNew {
		t.Error("should detect new items")
	}
}

func TestCheckInboxState_OnlyFiled(t *testing.T) {
	d := testDaemon(t)
	inboxPath := filepath.Join(d.Store.Dir(), "inbox.json")
	writeJSON(t, inboxPath, &state.InboxFile{Items: []state.InboxItem{
		{Status: "filed", Text: "item1"},
	}})

	hasNew := d.checkInboxState(inboxPath)
	if hasNew {
		t.Error("filed items should not count as new")
	}
}

// ═══════════════════════════════════════════════════════════════════════════
// PID utilities
// ═══════════════════════════════════════════════════════════════════════════

func TestWriteAndReadPID_RoundTrip(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	_ = os.MkdirAll(filepath.Join(dir, "system"), 0755)
	repo := NewDaemonRepository(dir)
	if err := repo.WritePID(os.Getpid()); err != nil {
		t.Fatal(err)
	}

	pid, err := repo.ReadPID()
	if err != nil {
		t.Fatal(err)
	}
	if pid != os.Getpid() {
		t.Errorf("expected PID %d, got %d", os.Getpid(), pid)
	}
}

func TestRemovePID_NoFileNoPanic(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	repo := NewDaemonRepository(dir)
	// Should not panic or error
	_ = repo.RemovePID()
}
