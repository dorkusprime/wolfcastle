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

	// Blocked scope with only blocked tasks: no actionable work.
	leafA := NewNodeState("leaf-a", "Leaf A", NodeLeaf)
	leafA.Tasks = []Task{
		{ID: "task-0001", Description: "stuck", State: StatusBlocked},
	}

	result, err := FindNextTask(idx, "leaf-a", makeLoadNode(map[string]*NodeState{
		"leaf-a": leafA,
	}))
	if err != nil {
		t.Fatal(err)
	}
	if result.Found {
		t.Error("expected not found for blocked scope without remediation work")
	}
}

func TestFindNextTask_ScopeBlockedWithRemediation(t *testing.T) {
	t.Parallel()
	idx := NewRootIndex()
	idx.Nodes["leaf-a"] = IndexEntry{
		Name:  "Leaf A",
		Type:  NodeLeaf,
		State: StatusBlocked,
	}

	// Blocked scope with remediation subtasks should find work.
	leafA := NewNodeState("leaf-a", "Leaf A", NodeLeaf)
	leafA.Tasks = []Task{
		{ID: "task-0001", Description: "done", State: StatusComplete},
		{ID: "audit", Description: "audit", State: StatusNotStarted, IsAudit: true},
		{ID: "audit.0001", Description: "fix gap", State: StatusNotStarted},
	}

	result, err := FindNextTask(idx, "leaf-a", makeLoadNode(map[string]*NodeState{
		"leaf-a": leafA,
	}))
	if err != nil {
		t.Fatal(err)
	}
	if !result.Found {
		t.Fatal("expected to find remediation task in blocked scoped node")
	}
	if result.TaskID != "audit.0001" {
		t.Errorf("expected audit.0001, got %s", result.TaskID)
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
	leafA.Tasks = []Task{{ID: "task-0001", Description: "A work", State: StatusNotStarted}}
	leafB := NewNodeState("leaf-b", "Leaf B", NodeLeaf)
	leafB.Tasks = []Task{{ID: "task-0001", Description: "B work", State: StatusNotStarted}}

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

	// All blocked with only blocked tasks: no actionable work anywhere.
	leafA := NewNodeState("leaf-a", "Leaf A", NodeLeaf)
	leafA.Tasks = []Task{
		{ID: "task-0001", Description: "stuck", State: StatusBlocked},
	}
	leafB := NewNodeState("leaf-b", "Leaf B", NodeLeaf)
	leafB.Tasks = []Task{
		{ID: "task-0001", Description: "stuck", State: StatusBlocked},
	}

	result, err := FindNextTask(idx, "", makeLoadNode(map[string]*NodeState{
		"leaf-a": leafA,
		"leaf-b": leafB,
	}))
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
		{ID: "task-0001", Description: "done", State: StatusComplete},
		{ID: "task-0002", Description: "also done", State: StatusComplete},
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
		{ID: "task-0001", Description: "stuck", State: StatusBlocked},
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
		{ID: "task-0001", Description: "real work", State: StatusNotStarted},
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
	if result.TaskID != "task-0001" {
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
		{ID: "task-0001", Description: "real work", State: StatusComplete},
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
	// A node with only an audit task and no real tasks is not ready.
	// The intake model hasn't finished adding tasks yet.
	if result.Found {
		t.Error("expected not found: audit-only node is not ready for execution")
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
	leafA.Tasks = []Task{{ID: "task-0001", Description: "work", State: StatusNotStarted}}

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
