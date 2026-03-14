package state

import (
	"fmt"
	"testing"
)

func makeLoadNode(nodes map[string]*NodeState) func(string) (*NodeState, error) {
	return func(addr string) (*NodeState, error) {
		ns, ok := nodes[addr]
		if !ok {
			return nil, fmt.Errorf("node %q not found", addr)
		}
		return ns, nil
	}
}

func TestFindNextTask_EmptyTree(t *testing.T) {
	t.Parallel()
	idx := NewRootIndex()
	result, err := FindNextTask(idx, "", makeLoadNode(nil))
	if err != nil {
		t.Fatal(err)
	}
	if result.Found {
		t.Error("expected not found for empty tree")
	}
	if result.Reason != "all_complete" {
		t.Errorf("expected reason 'all_complete', got %q", result.Reason)
	}
}

func TestFindNextTask_FindsFirstNotStartedTask(t *testing.T) {
	t.Parallel()
	idx := NewRootIndex()
	idx.Nodes["leaf-a"] = IndexEntry{
		Name:  "Leaf A",
		Type:  NodeLeaf,
		State: StatusNotStarted,
	}

	leafState := NewNodeState("leaf-a", "Leaf A", NodeLeaf)
	leafState.Tasks = []Task{
		{ID: "task-1", Description: "do thing", State: StatusNotStarted},
	}

	result, err := FindNextTask(idx, "", makeLoadNode(map[string]*NodeState{
		"leaf-a": leafState,
	}))
	if err != nil {
		t.Fatal(err)
	}
	if !result.Found {
		t.Fatal("expected to find a task")
	}
	if result.TaskID != "task-1" {
		t.Errorf("expected task-1, got %s", result.TaskID)
	}
	if result.NodeAddress != "leaf-a" {
		t.Errorf("expected node leaf-a, got %s", result.NodeAddress)
	}
}

func TestFindNextTask_PrefersInProgressForSelfHealing(t *testing.T) {
	t.Parallel()
	idx := NewRootIndex()
	idx.Nodes["leaf-a"] = IndexEntry{
		Name:  "Leaf A",
		Type:  NodeLeaf,
		State: StatusInProgress,
	}

	leafA := NewNodeState("leaf-a", "Leaf A", NodeLeaf)
	leafA.Tasks = []Task{
		{ID: "task-1", Description: "already done", State: StatusComplete},
		{ID: "task-2", Description: "was working on this", State: StatusInProgress},
		{ID: "task-3", Description: "not started yet", State: StatusNotStarted},
	}

	result, err := FindNextTask(idx, "", makeLoadNode(map[string]*NodeState{
		"leaf-a": leafA,
	}))
	if err != nil {
		t.Fatal(err)
	}
	if !result.Found {
		t.Fatal("expected to find a task")
	}
	if result.TaskID != "task-2" {
		t.Errorf("expected in_progress task-2 (self-healing), got %s", result.TaskID)
	}
}

func TestFindNextTask_SkipsCompleteNodes(t *testing.T) {
	t.Parallel()
	idx := NewRootIndex()
	idx.Nodes["leaf-a"] = IndexEntry{
		Name:  "Leaf A",
		Type:  NodeLeaf,
		State: StatusComplete,
	}
	idx.Nodes["leaf-b"] = IndexEntry{
		Name:  "Leaf B",
		Type:  NodeLeaf,
		State: StatusNotStarted,
	}

	leafB := NewNodeState("leaf-b", "Leaf B", NodeLeaf)
	leafB.Tasks = []Task{
		{ID: "task-1", Description: "do thing", State: StatusNotStarted},
	}

	result, err := FindNextTask(idx, "", makeLoadNode(map[string]*NodeState{
		"leaf-b": leafB,
	}))
	if err != nil {
		t.Fatal(err)
	}
	if !result.Found {
		t.Fatal("expected to find a task")
	}
	if result.NodeAddress != "leaf-b" {
		t.Errorf("expected leaf-b, got %s", result.NodeAddress)
	}
}

func TestFindNextTask_SkipsBlockedNodes(t *testing.T) {
	t.Parallel()
	idx := NewRootIndex()
	idx.Nodes["leaf-a"] = IndexEntry{
		Name:  "Leaf A",
		Type:  NodeLeaf,
		State: StatusBlocked,
	}
	idx.Nodes["leaf-b"] = IndexEntry{
		Name:  "Leaf B",
		Type:  NodeLeaf,
		State: StatusNotStarted,
	}

	leafB := NewNodeState("leaf-b", "Leaf B", NodeLeaf)
	leafB.Tasks = []Task{
		{ID: "task-1", Description: "available", State: StatusNotStarted},
	}

	result, err := FindNextTask(idx, "", makeLoadNode(map[string]*NodeState{
		"leaf-b": leafB,
	}))
	if err != nil {
		t.Fatal(err)
	}
	if !result.Found {
		t.Fatal("expected to find a task")
	}
	if result.NodeAddress != "leaf-b" {
		t.Errorf("expected leaf-b, got %s", result.NodeAddress)
	}
}

func TestFindNextTask_WithScopeLimitsSearch(t *testing.T) {
	t.Parallel()
	idx := NewRootIndex()
	idx.Nodes["orch"] = IndexEntry{
		Name:     "Orchestrator",
		Type:     NodeOrchestrator,
		State:    StatusNotStarted,
		Children: []string{"orch/leaf-a", "orch/leaf-b"},
	}
	idx.Nodes["orch/leaf-a"] = IndexEntry{
		Name:   "Leaf A",
		Type:   NodeLeaf,
		State:  StatusNotStarted,
		Parent: "orch",
	}
	idx.Nodes["orch/leaf-b"] = IndexEntry{
		Name:   "Leaf B",
		Type:   NodeLeaf,
		State:  StatusNotStarted,
		Parent: "orch",
	}

	leafA := NewNodeState("leaf-a", "Leaf A", NodeLeaf)
	leafA.Tasks = []Task{
		{ID: "task-1", Description: "task in A", State: StatusNotStarted},
	}
	leafB := NewNodeState("leaf-b", "Leaf B", NodeLeaf)
	leafB.Tasks = []Task{
		{ID: "task-1", Description: "task in B", State: StatusNotStarted},
	}

	// Scope to leaf-b only
	result, err := FindNextTask(idx, "orch/leaf-b", makeLoadNode(map[string]*NodeState{
		"orch/leaf-a": leafA,
		"orch/leaf-b": leafB,
	}))
	if err != nil {
		t.Fatal(err)
	}
	if !result.Found {
		t.Fatal("expected to find task")
	}
	if result.NodeAddress != "orch/leaf-b" {
		t.Errorf("expected orch/leaf-b, got %s", result.NodeAddress)
	}
}

func TestFindNextTask_AllComplete(t *testing.T) {
	t.Parallel()
	idx := NewRootIndex()
	idx.Nodes["leaf-a"] = IndexEntry{
		Name:  "Leaf A",
		Type:  NodeLeaf,
		State: StatusComplete,
	}

	result, err := FindNextTask(idx, "", makeLoadNode(nil))
	if err != nil {
		t.Fatal(err)
	}
	if result.Found {
		t.Error("expected not found")
	}
	if result.Reason != "all_complete" {
		t.Errorf("expected reason 'all_complete', got %q", result.Reason)
	}
}

func TestFindNextTask_ScopeNotFound(t *testing.T) {
	t.Parallel()
	idx := NewRootIndex()
	result, err := FindNextTask(idx, "nonexistent", makeLoadNode(nil))
	if err != nil {
		t.Fatal(err)
	}
	if result.Found {
		t.Error("expected not found")
	}
	if result.Reason != "scope address not found" {
		t.Errorf("expected 'scope address not found', got %q", result.Reason)
	}
}

func TestFindNextTask_DeterministicOrder(t *testing.T) {
	t.Parallel()
	// Create a root index with multiple top-level nodes and no Root array
	// to force the map-iteration fallback path.
	idx := NewRootIndex()
	for _, name := range []string{"zz-last", "aa-first", "mm-middle"} {
		idx.Nodes[name] = IndexEntry{
			Name:  name,
			Type:  NodeLeaf,
			State: StatusNotStarted,
		}
	}

	nodes := map[string]*NodeState{}
	for _, name := range []string{"zz-last", "aa-first", "mm-middle"} {
		ns := NewNodeState(name, name, NodeLeaf)
		ns.Tasks = []Task{{ID: "task-1", Description: "work", State: StatusNotStarted}}
		nodes[name] = ns
	}

	// Run 100 times and verify same result every time
	var firstAddr string
	for i := 0; i < 100; i++ {
		result, err := FindNextTask(idx, "", makeLoadNode(nodes))
		if err != nil {
			t.Fatal(err)
		}
		if !result.Found {
			t.Fatal("expected to find a task")
		}
		if i == 0 {
			firstAddr = result.NodeAddress
			if firstAddr != "aa-first" {
				t.Errorf("expected aa-first (alphabetically first), got %s", firstAddr)
			}
		} else if result.NodeAddress != firstAddr {
			t.Errorf("iteration %d: got %s, expected %s (non-deterministic)", i, result.NodeAddress, firstAddr)
		}
	}
}

func TestFindNextTask_TraversesOrchestratorChildren(t *testing.T) {
	t.Parallel()
	idx := NewRootIndex()
	idx.Nodes["root"] = IndexEntry{
		Name:     "Root",
		Type:     NodeOrchestrator,
		State:    StatusInProgress,
		Children: []string{"root/child-a", "root/child-b"},
	}
	idx.Nodes["root/child-a"] = IndexEntry{
		Name:   "Child A",
		Type:   NodeLeaf,
		State:  StatusComplete,
		Parent: "root",
	}
	idx.Nodes["root/child-b"] = IndexEntry{
		Name:   "Child B",
		Type:   NodeLeaf,
		State:  StatusNotStarted,
		Parent: "root",
	}

	childB := NewNodeState("child-b", "Child B", NodeLeaf)
	childB.Tasks = []Task{
		{ID: "task-1", Description: "work here", State: StatusNotStarted},
	}

	result, err := FindNextTask(idx, "", makeLoadNode(map[string]*NodeState{
		"root/child-b": childB,
	}))
	if err != nil {
		t.Fatal(err)
	}
	if !result.Found {
		t.Fatal("expected to find task in child-b")
	}
	if result.NodeAddress != "root/child-b" {
		t.Errorf("expected root/child-b, got %s", result.NodeAddress)
	}
}
