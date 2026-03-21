package state

import (
	"os"
	"path/filepath"
	"testing"
	"time"
)

// scaffoldTree creates an orchestrator with N leaf children on disk,
// each leaf having a single task and an audit task. Returns the store
// directory and a ready-to-use Store. The index wires parent/child
// relationships so MutateNode's built-in propagation can walk the tree.
func scaffoldTree(t *testing.T, childCount int) (string, *Store) {
	t.Helper()
	dir := t.TempDir()
	s := NewStore(dir, 5*time.Second)

	// Build index
	idx := NewRootIndex()
	idx.Root = []string{"orch"}
	idx.RootState = StatusNotStarted

	// Orchestrator entry
	var childAddrs []string
	var childRefs []ChildRef
	for i := 0; i < childCount; i++ {
		addr := "orch/leaf-" + string(rune('a'+i))
		childAddrs = append(childAddrs, addr)
		childRefs = append(childRefs, ChildRef{
			ID:      "leaf-" + string(rune('a'+i)),
			Address: addr,
			State:   StatusNotStarted,
		})
	}
	idx.Nodes["orch"] = IndexEntry{
		Name:     "Orchestrator",
		Type:     NodeOrchestrator,
		State:    StatusNotStarted,
		Address:  "orch",
		Children: childAddrs,
	}

	for i, addr := range childAddrs {
		idx.Nodes[addr] = IndexEntry{
			Name:    childRefs[i].ID,
			Type:    NodeLeaf,
			State:   StatusNotStarted,
			Address: addr,
			Parent:  "orch",
		}
	}

	if err := SaveRootIndex(filepath.Join(dir, "state.json"), idx); err != nil {
		t.Fatal(err)
	}

	// Orchestrator node state (no tasks of its own)
	orchDir := filepath.Join(dir, "orch")
	if err := os.MkdirAll(orchDir, 0755); err != nil {
		t.Fatal(err)
	}
	orchNS := NewNodeState("orch", "Orchestrator", NodeOrchestrator)
	orchNS.Children = childRefs
	if err := SaveNodeState(filepath.Join(orchDir, "state.json"), orchNS); err != nil {
		t.Fatal(err)
	}

	// Leaf node states
	for i, addr := range childAddrs {
		leafDir := filepath.Join(dir, "orch", childRefs[i].ID)
		if err := os.MkdirAll(leafDir, 0755); err != nil {
			t.Fatal(err)
		}
		leafNS := NewNodeState(childRefs[i].ID, childRefs[i].ID, NodeLeaf)
		leafNS.Tasks = []Task{
			{ID: "task-0001", Description: "do work", State: StatusNotStarted},
			{ID: "audit", Description: "audit", State: StatusNotStarted, IsAudit: true},
		}
		if err := SaveNodeState(filepath.Join(leafDir, "state.json"), leafNS); err != nil {
			t.Fatal(err)
		}
		_ = addr
	}

	return dir, s
}

func TestMutateNode_PropagatesCompletionToOrchestrator(t *testing.T) {
	t.Parallel()
	_, s := scaffoldTree(t, 2)

	// Complete both leaves: claim task, complete task, claim audit, complete audit.
	for _, addr := range []string{"orch/leaf-a", "orch/leaf-b"} {
		if err := s.MutateNode(addr, func(ns *NodeState) error {
			return TaskClaim(ns, "task-0001")
		}); err != nil {
			t.Fatalf("claim task on %s: %v", addr, err)
		}
		if err := s.MutateNode(addr, func(ns *NodeState) error {
			return TaskComplete(ns, "task-0001")
		}); err != nil {
			t.Fatalf("complete task on %s: %v", addr, err)
		}
		if err := s.MutateNode(addr, func(ns *NodeState) error {
			return TaskClaim(ns, "audit")
		}); err != nil {
			t.Fatalf("claim audit on %s: %v", addr, err)
		}
		if err := s.MutateNode(addr, func(ns *NodeState) error {
			return TaskComplete(ns, "audit")
		}); err != nil {
			t.Fatalf("complete audit on %s: %v", addr, err)
		}
	}

	// Verify leaf states
	for _, addr := range []string{"orch/leaf-a", "orch/leaf-b"} {
		ns, err := s.ReadNode(addr)
		if err != nil {
			t.Fatalf("reading %s: %v", addr, err)
		}
		if ns.State != StatusComplete {
			t.Errorf("leaf %s: expected complete, got %s", addr, ns.State)
		}
	}

	// Verify orchestrator state propagated to complete on disk
	orchNS, err := s.ReadNode("orch")
	if err != nil {
		t.Fatalf("reading orchestrator: %v", err)
	}
	if orchNS.State != StatusComplete {
		t.Errorf("orchestrator state: expected complete, got %s", orchNS.State)
	}

	// Verify all child refs updated
	for _, ref := range orchNS.Children {
		if ref.State != StatusComplete {
			t.Errorf("child ref %s: expected complete, got %s", ref.Address, ref.State)
		}
	}

	// Verify root index updated
	idx, err := s.ReadIndex()
	if err != nil {
		t.Fatalf("reading index: %v", err)
	}
	if entry, ok := idx.Nodes["orch"]; !ok {
		t.Error("orchestrator missing from index")
	} else if entry.State != StatusComplete {
		t.Errorf("index entry for orch: expected complete, got %s", entry.State)
	}
	if idx.RootState != StatusComplete {
		t.Errorf("root state: expected complete, got %s", idx.RootState)
	}
}

func TestMutateNode_PartialCompletion_OrchestratorStaysInProgress(t *testing.T) {
	t.Parallel()
	_, s := scaffoldTree(t, 2)

	// Complete only leaf-a
	for _, step := range []struct {
		task string
		fn   func(*NodeState) error
	}{
		{"task-0001", func(ns *NodeState) error { return TaskClaim(ns, "task-0001") }},
		{"task-0001", func(ns *NodeState) error { return TaskComplete(ns, "task-0001") }},
		{"audit", func(ns *NodeState) error { return TaskClaim(ns, "audit") }},
		{"audit", func(ns *NodeState) error { return TaskComplete(ns, "audit") }},
	} {
		if err := s.MutateNode("orch/leaf-a", step.fn); err != nil {
			t.Fatalf("step %s: %v", step.task, err)
		}
	}

	orchNS, err := s.ReadNode("orch")
	if err != nil {
		t.Fatal(err)
	}
	if orchNS.State == StatusComplete {
		t.Error("orchestrator should not be complete when only one child is done")
	}
	if orchNS.State != StatusInProgress {
		t.Errorf("orchestrator: expected in_progress, got %s", orchNS.State)
	}
}

func TestMutateNode_OrchestratorWithOwnTasks_WaitsForAll(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	s := NewStore(dir, 5*time.Second)

	// Build a tree: orchestrator with one leaf child and an audit task on the orchestrator
	idx := NewRootIndex()
	idx.Root = []string{"orch"}
	idx.Nodes["orch"] = IndexEntry{
		Name:     "Orchestrator",
		Type:     NodeOrchestrator,
		State:    StatusNotStarted,
		Address:  "orch",
		Children: []string{"orch/leaf-a"},
	}
	idx.Nodes["orch/leaf-a"] = IndexEntry{
		Name:    "leaf-a",
		Type:    NodeLeaf,
		State:   StatusNotStarted,
		Address: "orch/leaf-a",
		Parent:  "orch",
	}
	if err := SaveRootIndex(filepath.Join(dir, "state.json"), idx); err != nil {
		t.Fatal(err)
	}

	// Orchestrator has an audit task
	orchDir := filepath.Join(dir, "orch")
	if err := os.MkdirAll(orchDir, 0755); err != nil {
		t.Fatal(err)
	}
	orchNS := NewNodeState("orch", "Orchestrator", NodeOrchestrator)
	orchNS.Children = []ChildRef{{ID: "leaf-a", Address: "orch/leaf-a", State: StatusNotStarted}}
	orchNS.Tasks = []Task{
		{ID: "audit", Description: "orchestrator audit", State: StatusNotStarted, IsAudit: true},
	}
	if err := SaveNodeState(filepath.Join(orchDir, "state.json"), orchNS); err != nil {
		t.Fatal(err)
	}

	// Leaf
	leafDir := filepath.Join(dir, "orch", "leaf-a")
	if err := os.MkdirAll(leafDir, 0755); err != nil {
		t.Fatal(err)
	}
	leafNS := NewNodeState("leaf-a", "leaf-a", NodeLeaf)
	leafNS.Tasks = []Task{
		{ID: "task-0001", Description: "work", State: StatusNotStarted},
		{ID: "audit", Description: "audit", State: StatusNotStarted, IsAudit: true},
	}
	if err := SaveNodeState(filepath.Join(leafDir, "state.json"), leafNS); err != nil {
		t.Fatal(err)
	}

	// Complete the leaf entirely
	for _, fn := range []func(*NodeState) error{
		func(ns *NodeState) error { return TaskClaim(ns, "task-0001") },
		func(ns *NodeState) error { return TaskComplete(ns, "task-0001") },
		func(ns *NodeState) error { return TaskClaim(ns, "audit") },
		func(ns *NodeState) error { return TaskComplete(ns, "audit") },
	} {
		if err := s.MutateNode("orch/leaf-a", fn); err != nil {
			t.Fatal(err)
		}
	}

	// Orchestrator should NOT be complete (its own audit task is still not_started)
	orchResult, _ := s.ReadNode("orch")
	if orchResult.State == StatusComplete {
		t.Error("orchestrator should not be complete while its own audit task is not_started")
	}
}

func TestMutateNode_ThreeChildren_AllComplete(t *testing.T) {
	t.Parallel()
	_, s := scaffoldTree(t, 3)

	for _, addr := range []string{"orch/leaf-a", "orch/leaf-b", "orch/leaf-c"} {
		for _, fn := range []func(*NodeState) error{
			func(ns *NodeState) error { return TaskClaim(ns, "task-0001") },
			func(ns *NodeState) error { return TaskComplete(ns, "task-0001") },
			func(ns *NodeState) error { return TaskClaim(ns, "audit") },
			func(ns *NodeState) error { return TaskComplete(ns, "audit") },
		} {
			if err := s.MutateNode(addr, fn); err != nil {
				t.Fatalf("%s: %v", addr, err)
			}
		}
	}

	orchNS, err := s.ReadNode("orch")
	if err != nil {
		t.Fatal(err)
	}
	if orchNS.State != StatusComplete {
		t.Errorf("orchestrator with 3 complete children: expected complete, got %s", orchNS.State)
	}

	idx, err := s.ReadIndex()
	if err != nil {
		t.Fatal(err)
	}
	if idx.RootState != StatusComplete {
		t.Errorf("root state: expected complete, got %s", idx.RootState)
	}
}
