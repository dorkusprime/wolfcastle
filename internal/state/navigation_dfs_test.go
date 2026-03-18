package state

import (
	"fmt"
	"testing"
)

func TestDfs_ChildLoadError(t *testing.T) {
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

	loadErr := func(addr string) (*NodeState, error) {
		return nil, fmt.Errorf("disk error")
	}

	_, err := dfs(idx, "orch", loadErr)
	if err == nil {
		t.Error("expected error from child dfs when loadNode fails")
	}
}

func TestDfs_OrchestratorAllChildrenComplete(t *testing.T) {
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
		State:  StatusComplete,
		Parent: "orch",
	}

	orchState := NewNodeState("orch", "Orchestrator", NodeOrchestrator)
	// No actionable tasks on the orchestrator itself
	orchState.Tasks = []Task{
		{ID: "audit-1", Description: "audit", State: StatusComplete, IsAudit: true},
	}

	result, err := dfs(idx, "orch", makeLoadNode(map[string]*NodeState{
		"orch": orchState,
	}))
	if err != nil {
		t.Fatal(err)
	}
	if result != nil && result.Found {
		t.Error("expected no task found when all children complete and orchestrator audit is complete")
	}
}

func TestDfs_OrchestratorAuditTaskAfterChildrenComplete(t *testing.T) {
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
		State:  StatusComplete,
		Parent: "orch",
	}

	orchState := NewNodeState("orch", "Orchestrator", NodeOrchestrator)
	orchState.Children = []ChildRef{
		{ID: "leaf-a", Address: "orch/leaf-a", State: StatusComplete},
	}
	orchState.Tasks = []Task{
		{ID: "audit-1", Description: "audit the orchestrator", State: StatusNotStarted, IsAudit: true},
	}

	result, err := dfs(idx, "orch", makeLoadNode(map[string]*NodeState{
		"orch": orchState,
	}))
	if err != nil {
		t.Fatal(err)
	}
	if result == nil || !result.Found {
		t.Fatal("expected to find orchestrator audit task after all children complete")
	}
	if result.NodeAddress != "orch" {
		t.Errorf("expected node address 'orch', got %s", result.NodeAddress)
	}
	if result.TaskID != "audit-1" {
		t.Errorf("expected task 'audit-1', got %s", result.TaskID)
	}
}

func TestDfs_OrchestratorAuditTaskInProgress(t *testing.T) {
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
		State:  StatusComplete,
		Parent: "orch",
	}

	orchState := NewNodeState("orch", "Orchestrator", NodeOrchestrator)
	orchState.Tasks = []Task{
		{ID: "audit-1", Description: "audit the orchestrator", State: StatusInProgress, IsAudit: true},
	}

	result, err := dfs(idx, "orch", makeLoadNode(map[string]*NodeState{
		"orch": orchState,
	}))
	if err != nil {
		t.Fatal(err)
	}
	if result == nil || !result.Found {
		t.Fatal("expected to find in-progress orchestrator audit task")
	}
	if result.TaskID != "audit-1" {
		t.Errorf("expected task 'audit-1', got %s", result.TaskID)
	}
}

func TestDfs_BlockedNode(t *testing.T) {
	t.Parallel()
	idx := NewRootIndex()
	idx.Nodes["leaf-a"] = IndexEntry{
		Name:  "Leaf A",
		Type:  NodeLeaf,
		State: StatusBlocked,
	}

	result, err := dfs(idx, "leaf-a", makeLoadNode(nil))
	if err != nil {
		t.Fatal(err)
	}
	if result != nil {
		t.Error("expected nil result for blocked node")
	}
}

func TestDfs_CompleteNode(t *testing.T) {
	t.Parallel()
	idx := NewRootIndex()
	idx.Nodes["leaf-a"] = IndexEntry{
		Name:  "Leaf A",
		Type:  NodeLeaf,
		State: StatusComplete,
	}

	result, err := dfs(idx, "leaf-a", makeLoadNode(nil))
	if err != nil {
		t.Fatal(err)
	}
	if result != nil {
		t.Error("expected nil result for complete node")
	}
}
