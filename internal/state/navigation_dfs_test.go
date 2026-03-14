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

	result, err := dfs(idx, "orch", makeLoadNode(nil))
	if err != nil {
		t.Fatal(err)
	}
	if result != nil && result.Found {
		t.Error("expected no task found when all children complete")
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
