package state

import (
	"testing"
)

func TestFindParallelTasks_EmptyTree(t *testing.T) {
	t.Parallel()
	idx := NewRootIndex()
	results, err := FindParallelTasks(idx, "", makeLoadNode(nil), 3)
	if err != nil {
		t.Fatal(err)
	}
	if len(results) != 0 {
		t.Errorf("expected empty results, got %d", len(results))
	}
}

func TestFindParallelTasks_SingleRootNode(t *testing.T) {
	t.Parallel()
	idx := NewRootIndex()
	idx.Root = []string{"leaf-a"}
	idx.Nodes["leaf-a"] = IndexEntry{
		Name:  "Leaf A",
		Type:  NodeLeaf,
		State: StatusNotStarted,
	}

	leafState := NewNodeState("leaf-a", "Leaf A", NodeLeaf)
	leafState.Tasks = []Task{
		{ID: "task-0001", Description: "first task", State: StatusNotStarted},
	}

	results, err := FindParallelTasks(idx, "", makeLoadNode(map[string]*NodeState{
		"leaf-a": leafState,
	}), 3)
	if err != nil {
		t.Fatal(err)
	}
	if len(results) != 1 {
		t.Fatalf("expected 1 result for root-level node, got %d", len(results))
	}
	if results[0].TaskID != "task-0001" {
		t.Errorf("expected task-0001, got %s", results[0].TaskID)
	}
}

func TestFindParallelTasks_MultipleSiblings(t *testing.T) {
	t.Parallel()
	idx := NewRootIndex()
	idx.Root = []string{"parent"}
	idx.Nodes["parent"] = IndexEntry{
		Name:     "Parent",
		Type:     NodeOrchestrator,
		State:    StatusNotStarted,
		Children: []string{"child-a", "child-b", "child-c"},
	}
	idx.Nodes["child-a"] = IndexEntry{
		Name:   "Child A",
		Type:   NodeLeaf,
		State:  StatusNotStarted,
		Parent: "parent",
	}
	idx.Nodes["child-b"] = IndexEntry{
		Name:   "Child B",
		Type:   NodeLeaf,
		State:  StatusNotStarted,
		Parent: "parent",
	}
	idx.Nodes["child-c"] = IndexEntry{
		Name:   "Child C",
		Type:   NodeLeaf,
		State:  StatusNotStarted,
		Parent: "parent",
	}

	childA := NewNodeState("child-a", "Child A", NodeLeaf)
	childA.Tasks = []Task{
		{ID: "task-0001", Description: "task A", State: StatusNotStarted},
	}
	childB := NewNodeState("child-b", "Child B", NodeLeaf)
	childB.Tasks = []Task{
		{ID: "task-0001", Description: "task B", State: StatusNotStarted},
	}
	childC := NewNodeState("child-c", "Child C", NodeLeaf)
	childC.Tasks = []Task{
		{ID: "task-0001", Description: "task C", State: StatusNotStarted},
	}

	results, err := FindParallelTasks(idx, "", makeLoadNode(map[string]*NodeState{
		"child-a": childA,
		"child-b": childB,
		"child-c": childC,
	}), 5)
	if err != nil {
		t.Fatal(err)
	}
	if len(results) != 3 {
		t.Fatalf("expected 3 parallel tasks, got %d", len(results))
	}
	if results[0].NodeAddress != "child-a" {
		t.Errorf("expected first result from child-a, got %s", results[0].NodeAddress)
	}
	if results[1].NodeAddress != "child-b" {
		t.Errorf("expected second result from child-b, got %s", results[1].NodeAddress)
	}
	if results[2].NodeAddress != "child-c" {
		t.Errorf("expected third result from child-c, got %s", results[2].NodeAddress)
	}
}

func TestFindParallelTasks_SkipsInProgressSiblings(t *testing.T) {
	t.Parallel()
	idx := NewRootIndex()
	idx.Root = []string{"parent"}
	idx.Nodes["parent"] = IndexEntry{
		Name:     "Parent",
		Type:     NodeOrchestrator,
		State:    StatusInProgress,
		Children: []string{"child-a", "child-b", "child-c"},
	}
	idx.Nodes["child-a"] = IndexEntry{
		Name:   "Child A",
		Type:   NodeLeaf,
		State:  StatusNotStarted,
		Parent: "parent",
	}
	idx.Nodes["child-b"] = IndexEntry{
		Name:   "Child B",
		Type:   NodeLeaf,
		State:  StatusInProgress,
		Parent: "parent",
	}
	idx.Nodes["child-c"] = IndexEntry{
		Name:   "Child C",
		Type:   NodeLeaf,
		State:  StatusNotStarted,
		Parent: "parent",
	}

	childA := NewNodeState("child-a", "Child A", NodeLeaf)
	childA.Tasks = []Task{
		{ID: "task-0001", Description: "task A", State: StatusNotStarted},
	}
	childB := NewNodeState("child-b", "Child B", NodeLeaf)
	childB.Tasks = []Task{
		{ID: "task-0001", Description: "task B", State: StatusInProgress},
	}
	childC := NewNodeState("child-c", "Child C", NodeLeaf)
	childC.Tasks = []Task{
		{ID: "task-0001", Description: "task C", State: StatusNotStarted},
	}

	results, err := FindParallelTasks(idx, "", makeLoadNode(map[string]*NodeState{
		"child-a": childA,
		"child-b": childB,
		"child-c": childC,
	}), 5)
	if err != nil {
		t.Fatal(err)
	}
	if len(results) != 2 {
		t.Fatalf("expected 2 results (in-progress sibling skipped), got %d", len(results))
	}
	if results[0].NodeAddress != "child-a" {
		t.Errorf("expected first result from child-a, got %s", results[0].NodeAddress)
	}
	if results[1].NodeAddress != "child-c" {
		t.Errorf("expected second result from child-c, got %s", results[1].NodeAddress)
	}
}

func TestFindParallelTasks_UnplannedOrchestratorStopsScanning(t *testing.T) {
	t.Parallel()
	idx := NewRootIndex()
	idx.Root = []string{"parent"}
	idx.Nodes["parent"] = IndexEntry{
		Name:     "Parent",
		Type:     NodeOrchestrator,
		State:    StatusNotStarted,
		Children: []string{"child-a", "unplanned-orch", "child-c"},
	}
	idx.Nodes["child-a"] = IndexEntry{
		Name:   "Child A",
		Type:   NodeLeaf,
		State:  StatusNotStarted,
		Parent: "parent",
	}
	idx.Nodes["unplanned-orch"] = IndexEntry{
		Name:     "Unplanned Orch",
		Type:     NodeOrchestrator,
		State:    StatusNotStarted,
		Parent:   "parent",
		Children: []string{}, // no children = unplanned
	}
	idx.Nodes["child-c"] = IndexEntry{
		Name:   "Child C",
		Type:   NodeLeaf,
		State:  StatusNotStarted,
		Parent: "parent",
	}

	childA := NewNodeState("child-a", "Child A", NodeLeaf)
	childA.Tasks = []Task{
		{ID: "task-0001", Description: "task A", State: StatusNotStarted},
	}
	childC := NewNodeState("child-c", "Child C", NodeLeaf)
	childC.Tasks = []Task{
		{ID: "task-0001", Description: "task C", State: StatusNotStarted},
	}

	results, err := FindParallelTasks(idx, "", makeLoadNode(map[string]*NodeState{
		"child-a": childA,
		"child-c": childC,
	}), 5)
	if err != nil {
		t.Fatal(err)
	}
	// child-a is found, unplanned-orch stops scanning, child-c is ineligible
	if len(results) != 1 {
		t.Fatalf("expected 1 result (unplanned orch stops scanning), got %d", len(results))
	}
	if results[0].NodeAddress != "child-a" {
		t.Errorf("expected result from child-a, got %s", results[0].NodeAddress)
	}
}

func TestFindParallelTasks_MaxCountCaps(t *testing.T) {
	t.Parallel()
	idx := NewRootIndex()
	idx.Root = []string{"parent"}
	idx.Nodes["parent"] = IndexEntry{
		Name:     "Parent",
		Type:     NodeOrchestrator,
		State:    StatusNotStarted,
		Children: []string{"child-a", "child-b", "child-c"},
	}
	idx.Nodes["child-a"] = IndexEntry{
		Name:   "Child A",
		Type:   NodeLeaf,
		State:  StatusNotStarted,
		Parent: "parent",
	}
	idx.Nodes["child-b"] = IndexEntry{
		Name:   "Child B",
		Type:   NodeLeaf,
		State:  StatusNotStarted,
		Parent: "parent",
	}
	idx.Nodes["child-c"] = IndexEntry{
		Name:   "Child C",
		Type:   NodeLeaf,
		State:  StatusNotStarted,
		Parent: "parent",
	}

	childA := NewNodeState("child-a", "Child A", NodeLeaf)
	childA.Tasks = []Task{
		{ID: "task-0001", Description: "task A", State: StatusNotStarted},
	}
	childB := NewNodeState("child-b", "Child B", NodeLeaf)
	childB.Tasks = []Task{
		{ID: "task-0001", Description: "task B", State: StatusNotStarted},
	}
	childC := NewNodeState("child-c", "Child C", NodeLeaf)
	childC.Tasks = []Task{
		{ID: "task-0001", Description: "task C", State: StatusNotStarted},
	}

	results, err := FindParallelTasks(idx, "", makeLoadNode(map[string]*NodeState{
		"child-a": childA,
		"child-b": childB,
		"child-c": childC,
	}), 2)
	if err != nil {
		t.Fatal(err)
	}
	if len(results) != 2 {
		t.Fatalf("expected 2 results (capped by maxCount), got %d", len(results))
	}
}

func TestFindParallelTasks_SkipsCompleteAndBlocked(t *testing.T) {
	t.Parallel()
	idx := NewRootIndex()
	idx.Root = []string{"parent"}
	idx.Nodes["parent"] = IndexEntry{
		Name:     "Parent",
		Type:     NodeOrchestrator,
		State:    StatusInProgress,
		Children: []string{"child-a", "child-b", "child-c"},
	}
	idx.Nodes["child-a"] = IndexEntry{
		Name:   "Child A",
		Type:   NodeLeaf,
		State:  StatusComplete,
		Parent: "parent",
	}
	idx.Nodes["child-b"] = IndexEntry{
		Name:   "Child B",
		Type:   NodeLeaf,
		State:  StatusBlocked,
		Parent: "parent",
	}
	idx.Nodes["child-c"] = IndexEntry{
		Name:   "Child C",
		Type:   NodeLeaf,
		State:  StatusNotStarted,
		Parent: "parent",
	}

	childC := NewNodeState("child-c", "Child C", NodeLeaf)
	childC.Tasks = []Task{
		{ID: "task-0001", Description: "task C", State: StatusNotStarted},
	}

	results, err := FindParallelTasks(idx, "", makeLoadNode(map[string]*NodeState{
		"child-c": childC,
	}), 5)
	if err != nil {
		t.Fatal(err)
	}
	if len(results) != 1 {
		t.Fatalf("expected 1 result (complete and blocked skipped), got %d", len(results))
	}
	if results[0].NodeAddress != "child-c" {
		t.Errorf("expected result from child-c, got %s", results[0].NodeAddress)
	}
}
