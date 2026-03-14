package state

import (
	"fmt"
	"testing"
)

func TestRecomputeState_SingleNotStarted(t *testing.T) {
	t.Parallel()
	children := []ChildRef{{ID: "a", State: StatusNotStarted}}
	if got := RecomputeState(children); got != StatusNotStarted {
		t.Errorf("expected not_started, got %s", got)
	}
}

func TestRecomputeState_SingleComplete(t *testing.T) {
	t.Parallel()
	children := []ChildRef{{ID: "a", State: StatusComplete}}
	if got := RecomputeState(children); got != StatusComplete {
		t.Errorf("expected complete, got %s", got)
	}
}

func TestRecomputeState_SingleInProgress(t *testing.T) {
	t.Parallel()
	children := []ChildRef{{ID: "a", State: StatusInProgress}}
	if got := RecomputeState(children); got != StatusInProgress {
		t.Errorf("expected in_progress, got %s", got)
	}
}

func TestRecomputeState_SingleBlocked(t *testing.T) {
	t.Parallel()
	children := []ChildRef{{ID: "a", State: StatusBlocked}}
	if got := RecomputeState(children); got != StatusBlocked {
		t.Errorf("expected blocked, got %s", got)
	}
}

func TestRecomputeState_BlockedAndInProgress(t *testing.T) {
	t.Parallel()
	children := []ChildRef{
		{ID: "a", State: StatusBlocked},
		{ID: "b", State: StatusInProgress},
	}
	if got := RecomputeState(children); got != StatusInProgress {
		t.Errorf("expected in_progress (blocked + in_progress mix), got %s", got)
	}
}

func TestPropagateUp_NoParent(t *testing.T) {
	t.Parallel()
	updated, err := PropagateUp(
		"child",
		StatusInProgress,
		func(addr string) (*NodeState, error) { return nil, nil },
		func(addr string, ns *NodeState) error { return nil },
		func(addr string) string { return "" },
	)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(updated) != 0 {
		t.Errorf("expected no updates, got %v", updated)
	}
}

func TestPropagateUp_LoadParentError(t *testing.T) {
	t.Parallel()
	_, err := PropagateUp(
		"child",
		StatusInProgress,
		func(addr string) (*NodeState, error) { return nil, fmt.Errorf("disk error") },
		func(addr string, ns *NodeState) error { return nil },
		func(addr string) string {
			if addr == "child" {
				return "parent"
			}
			return ""
		},
	)
	if err == nil {
		t.Error("expected error when loadParent fails")
	}
}

func TestPropagateUp_SaveParentError(t *testing.T) {
	t.Parallel()
	parent := &NodeState{
		ID:       "parent",
		Type:     NodeOrchestrator,
		Children: []ChildRef{{ID: "child", Address: "child", State: StatusNotStarted}},
	}
	_, err := PropagateUp(
		"child",
		StatusInProgress,
		func(addr string) (*NodeState, error) { return parent, nil },
		func(addr string, ns *NodeState) error { return fmt.Errorf("write error") },
		func(addr string) string {
			if addr == "child" {
				return "parent"
			}
			return ""
		},
	)
	if err == nil {
		t.Error("expected error when saveParent fails")
	}
}

func TestPropagateUp_MaxDepthExceeded(t *testing.T) {
	t.Parallel()
	// Build a chain longer than maxPropagationDepth (100)
	// Each node points to a unique parent, no cycles, just very deep
	states := make(map[string]*NodeState)
	parents := make(map[string]string)
	for i := 0; i <= maxPropagationDepth+2; i++ {
		addr := fmt.Sprintf("node-%d", i)
		states[addr] = &NodeState{
			ID:   addr,
			Type: NodeOrchestrator,
			Children: []ChildRef{{
				ID:      fmt.Sprintf("node-%d", i-1),
				Address: fmt.Sprintf("node-%d", i-1),
				State:   StatusNotStarted,
			}},
		}
		if i > 0 {
			parents[fmt.Sprintf("node-%d", i-1)] = addr
		}
	}

	_, err := PropagateUp(
		"node-0",
		StatusInProgress,
		func(addr string) (*NodeState, error) { return states[addr], nil },
		func(addr string, ns *NodeState) error { return nil },
		func(addr string) string { return parents[addr] },
	)
	if err == nil {
		t.Error("expected error for exceeding max propagation depth")
	}
}

func TestPropagateUp_MultiLevelChain(t *testing.T) {
	t.Parallel()
	states := map[string]*NodeState{
		"mid": {ID: "mid", Type: NodeOrchestrator, Children: []ChildRef{{ID: "child", Address: "child", State: StatusNotStarted}}},
		"root": {ID: "root", Type: NodeOrchestrator, Children: []ChildRef{{ID: "mid", Address: "mid", State: StatusNotStarted}}},
	}
	parents := map[string]string{"child": "mid", "mid": "root"}

	updated, err := PropagateUp(
		"child",
		StatusInProgress,
		func(addr string) (*NodeState, error) { return states[addr], nil },
		func(addr string, ns *NodeState) error { states[addr] = ns; return nil },
		func(addr string) string { return parents[addr] },
	)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(updated) != 2 {
		t.Errorf("expected 2 updates (mid, root), got %v", updated)
	}
	if states["root"].State != StatusInProgress {
		t.Errorf("expected root in_progress, got %s", states["root"].State)
	}
}
