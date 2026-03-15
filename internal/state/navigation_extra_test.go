package state

import (
	"fmt"
	"testing"
)

func TestFindNextTask_ScopeComplete(t *testing.T) {
	t.Parallel()
	idx := NewRootIndex()
	idx.Nodes["leaf-a"] = IndexEntry{
		Name:  "Leaf A",
		Type:  NodeLeaf,
		State: StatusComplete,
	}

	result, err := FindNextTask(idx, "leaf-a", makeLoadNode(nil))
	if err != nil {
		t.Fatal(err)
	}
	if result.Found {
		t.Error("expected not found for complete scope")
	}
	if result.Reason != "scoped node is complete" {
		t.Errorf("expected 'scoped node is complete', got %q", result.Reason)
	}
}

func TestFindNextTask_ScopeBlocked(t *testing.T) {
	t.Parallel()
	idx := NewRootIndex()
	idx.Nodes["leaf-a"] = IndexEntry{
		Name:  "Leaf A",
		Type:  NodeLeaf,
		State: StatusBlocked,
	}

	result, err := FindNextTask(idx, "leaf-a", makeLoadNode(nil))
	if err != nil {
		t.Fatal(err)
	}
	if result.Found {
		t.Error("expected not found for blocked scope")
	}
	if result.Reason != "scoped node is blocked" {
		t.Errorf("expected 'scoped node is blocked', got %q", result.Reason)
	}
}

func TestFindNextTask_UsesRootArrayWhenAvailable(t *testing.T) {
	t.Parallel()
	idx := NewRootIndex()
	idx.Root = []string{"leaf-b", "leaf-a"}
	idx.Nodes["leaf-a"] = IndexEntry{
		Name:  "Leaf A",
		Type:  NodeLeaf,
		State: StatusNotStarted,
	}
	idx.Nodes["leaf-b"] = IndexEntry{
		Name:  "Leaf B",
		Type:  NodeLeaf,
		State: StatusNotStarted,
	}

	leafA := NewNodeState("leaf-a", "Leaf A", NodeLeaf)
	leafA.Tasks = []Task{{ID: "task-1", Description: "A work", State: StatusNotStarted}}
	leafB := NewNodeState("leaf-b", "Leaf B", NodeLeaf)
	leafB.Tasks = []Task{{ID: "task-1", Description: "B work", State: StatusNotStarted}}

	result, err := FindNextTask(idx, "", makeLoadNode(map[string]*NodeState{
		"leaf-a": leafA,
		"leaf-b": leafB,
	}))
	if err != nil {
		t.Fatal(err)
	}
	if !result.Found {
		t.Fatal("expected to find a task")
	}
	// Root array ordering: leaf-b comes first
	if result.NodeAddress != "leaf-b" {
		t.Errorf("expected leaf-b (first in Root array), got %s", result.NodeAddress)
	}
}

func TestFindNextTask_AllBlocked(t *testing.T) {
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
		State: StatusBlocked,
	}

	result, err := FindNextTask(idx, "", makeLoadNode(nil))
	if err != nil {
		t.Fatal(err)
	}
	if result.Found {
		t.Error("expected not found when all nodes are blocked")
	}
	if result.Reason != "all_blocked" {
		t.Errorf("expected 'all_blocked', got %q", result.Reason)
	}
}

func TestFindNextTask_LeafWithAllCompleteTasks(t *testing.T) {
	t.Parallel()
	idx := NewRootIndex()
	idx.Nodes["leaf-a"] = IndexEntry{
		Name:  "Leaf A",
		Type:  NodeLeaf,
		State: StatusInProgress, // Status says in progress but all tasks are done
	}

	leafA := NewNodeState("leaf-a", "Leaf A", NodeLeaf)
	leafA.Tasks = []Task{
		{ID: "task-1", Description: "done", State: StatusComplete},
		{ID: "task-2", Description: "also done", State: StatusComplete},
	}

	result, err := FindNextTask(idx, "", makeLoadNode(map[string]*NodeState{
		"leaf-a": leafA,
	}))
	if err != nil {
		t.Fatal(err)
	}
	// No actionable tasks in the leaf
	if result.Found {
		t.Error("expected not found when all tasks in leaf are complete")
	}
}

func TestFindNextTask_LeafWithAllBlockedTasks(t *testing.T) {
	t.Parallel()
	idx := NewRootIndex()
	idx.Nodes["leaf-a"] = IndexEntry{
		Name:  "Leaf A",
		Type:  NodeLeaf,
		State: StatusInProgress,
	}

	leafA := NewNodeState("leaf-a", "Leaf A", NodeLeaf)
	leafA.Tasks = []Task{
		{ID: "task-1", Description: "stuck", State: StatusBlocked},
	}

	result, err := FindNextTask(idx, "", makeLoadNode(map[string]*NodeState{
		"leaf-a": leafA,
	}))
	if err != nil {
		t.Fatal(err)
	}
	if result.Found {
		t.Error("expected not found when all tasks are blocked")
	}
}

func TestFindNextTask_AuditDeferredUntilNonAuditComplete(t *testing.T) {
	t.Parallel()
	idx := NewRootIndex()
	idx.Nodes["leaf-a"] = IndexEntry{
		Name:  "Leaf A",
		Type:  NodeLeaf,
		State: StatusInProgress,
	}

	leafA := NewNodeState("leaf-a", "Leaf A", NodeLeaf)
	leafA.Tasks = []Task{
		{ID: "task-1", Description: "real work", State: StatusNotStarted},
		{ID: "audit", Description: "audit", State: StatusNotStarted, IsAudit: true},
	}

	result, err := FindNextTask(idx, "", makeLoadNode(map[string]*NodeState{
		"leaf-a": leafA,
	}))
	if err != nil {
		t.Fatal(err)
	}
	if !result.Found {
		t.Fatal("expected to find work")
	}
	if result.TaskID != "task-1" {
		t.Errorf("expected task-1, got %s (audit should be deferred)", result.TaskID)
	}
}

func TestFindNextTask_AuditEligibleWhenNonAuditComplete(t *testing.T) {
	t.Parallel()
	idx := NewRootIndex()
	idx.Nodes["leaf-a"] = IndexEntry{
		Name:  "Leaf A",
		Type:  NodeLeaf,
		State: StatusInProgress,
	}

	leafA := NewNodeState("leaf-a", "Leaf A", NodeLeaf)
	leafA.Tasks = []Task{
		{ID: "task-1", Description: "real work", State: StatusComplete},
		{ID: "audit", Description: "audit", State: StatusNotStarted, IsAudit: true},
	}

	result, err := FindNextTask(idx, "", makeLoadNode(map[string]*NodeState{
		"leaf-a": leafA,
	}))
	if err != nil {
		t.Fatal(err)
	}
	if !result.Found {
		t.Fatal("expected to find audit task")
	}
	if result.TaskID != "audit" {
		t.Errorf("expected audit, got %s", result.TaskID)
	}
}

func TestFindNextTask_AuditOnlyNode_NoNonAuditTasks(t *testing.T) {
	t.Parallel()
	idx := NewRootIndex()
	idx.Nodes["leaf-a"] = IndexEntry{
		Name:  "Leaf A",
		Type:  NodeLeaf,
		State: StatusInProgress,
	}

	leafA := NewNodeState("leaf-a", "Leaf A", NodeLeaf)
	leafA.Tasks = []Task{
		{ID: "audit", Description: "audit", State: StatusNotStarted, IsAudit: true},
	}

	result, err := FindNextTask(idx, "", makeLoadNode(map[string]*NodeState{
		"leaf-a": leafA,
	}))
	if err != nil {
		t.Fatal(err)
	}
	// All non-audit tasks are vacuously complete, so audit is eligible
	if !result.Found {
		t.Fatal("expected to find audit task when it's the only task")
	}
	if result.TaskID != "audit" {
		t.Errorf("expected audit, got %s", result.TaskID)
	}
}

func TestFindNextTask_LoadNodeError(t *testing.T) {
	t.Parallel()
	idx := NewRootIndex()
	idx.Nodes["leaf-a"] = IndexEntry{
		Name:  "Leaf A",
		Type:  NodeLeaf,
		State: StatusNotStarted,
	}

	loadErr := func(addr string) (*NodeState, error) {
		return nil, fmt.Errorf("disk failure")
	}

	_, err := FindNextTask(idx, "", loadErr)
	if err == nil {
		t.Error("expected error when loadNode fails")
	}
}

func TestFindNextTask_ScopedOrchestratorTraversesChildren(t *testing.T) {
	t.Parallel()
	idx := NewRootIndex()
	idx.Nodes["orch"] = IndexEntry{
		Name:     "Orchestrator",
		Type:     NodeOrchestrator,
		State:    StatusInProgress,
		Children: []string{"orch/leaf-a"},
	}
	idx.Nodes["orch/leaf-a"] = IndexEntry{
		Name:   "Leaf A",
		Type:   NodeLeaf,
		State:  StatusNotStarted,
		Parent: "orch",
	}

	leafA := NewNodeState("leaf-a", "Leaf A", NodeLeaf)
	leafA.Tasks = []Task{{ID: "task-1", Description: "work", State: StatusNotStarted}}

	result, err := FindNextTask(idx, "orch", makeLoadNode(map[string]*NodeState{
		"orch/leaf-a": leafA,
	}))
	if err != nil {
		t.Fatal(err)
	}
	if !result.Found {
		t.Fatal("expected to find task in scoped orchestrator")
	}
	if result.NodeAddress != "orch/leaf-a" {
		t.Errorf("expected orch/leaf-a, got %s", result.NodeAddress)
	}
}

func TestDfs_UnknownAddress(t *testing.T) {
	t.Parallel()
	idx := NewRootIndex()
	result, err := dfs(idx, "nonexistent", makeLoadNode(nil))
	if err != nil {
		t.Fatal(err)
	}
	if result != nil {
		t.Error("expected nil result for unknown address")
	}
}
