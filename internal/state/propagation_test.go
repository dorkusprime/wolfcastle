package state

import (
	"testing"
)

func TestRecomputeState_AllNotStarted(t *testing.T) {
	t.Parallel()
	children := []ChildRef{
		{ID: "a", State: StatusNotStarted},
		{ID: "b", State: StatusNotStarted},
	}
	if got := RecomputeState(children); got != StatusNotStarted {
		t.Errorf("expected not_started, got %s", got)
	}
}

func TestRecomputeState_AllComplete(t *testing.T) {
	t.Parallel()
	children := []ChildRef{
		{ID: "a", State: StatusComplete},
		{ID: "b", State: StatusComplete},
	}
	if got := RecomputeState(children); got != StatusComplete {
		t.Errorf("expected complete, got %s", got)
	}
}

func TestRecomputeState_Mixed(t *testing.T) {
	t.Parallel()
	children := []ChildRef{
		{ID: "a", State: StatusComplete},
		{ID: "b", State: StatusNotStarted},
		{ID: "c", State: StatusInProgress},
	}
	if got := RecomputeState(children); got != StatusInProgress {
		t.Errorf("expected in_progress, got %s", got)
	}
}

func TestRecomputeState_AllNonCompleteBlocked(t *testing.T) {
	t.Parallel()
	children := []ChildRef{
		{ID: "a", State: StatusComplete},
		{ID: "b", State: StatusBlocked},
		{ID: "c", State: StatusBlocked},
	}
	if got := RecomputeState(children); got != StatusBlocked {
		t.Errorf("expected blocked, got %s", got)
	}
}

func TestRecomputeState_OneBlockedOthersAvailable(t *testing.T) {
	t.Parallel()
	children := []ChildRef{
		{ID: "a", State: StatusBlocked},
		{ID: "b", State: StatusNotStarted},
	}
	if got := RecomputeState(children); got != StatusInProgress {
		t.Errorf("expected in_progress, got %s", got)
	}
}

func TestRecomputeState_EmptyChildren(t *testing.T) {
	t.Parallel()
	if got := RecomputeState(nil); got != StatusNotStarted {
		t.Errorf("expected not_started for empty, got %s", got)
	}
}

func TestRecomputeState_AllBlocked(t *testing.T) {
	t.Parallel()
	children := []ChildRef{
		{ID: "a", State: StatusBlocked},
		{ID: "b", State: StatusBlocked},
	}
	if got := RecomputeState(children); got != StatusBlocked {
		t.Errorf("expected blocked, got %s", got)
	}
}

func TestPropagateUp_DetectsCycle(t *testing.T) {
	t.Parallel()
	states := map[string]*NodeState{
		"a": {ID: "a", Type: NodeOrchestrator, Children: []ChildRef{{ID: "b", Address: "b", State: StatusInProgress}}},
		"b": {ID: "b", Type: NodeOrchestrator, Children: []ChildRef{{ID: "a", Address: "a", State: StatusInProgress}}},
	}
	parents := map[string]string{"a": "b", "b": "a"}

	_, err := PropagateUp(
		"a",
		StatusInProgress,
		func(addr string) (*NodeState, error) { return states[addr], nil },
		func(addr string, ns *NodeState) error { return nil },
		func(addr string) string { return parents[addr] },
	)
	if err == nil {
		t.Error("expected error for cycle in parent chain")
	}
}

func TestPropagateUp_NeedsPlanning_PreventsComplete(t *testing.T) {
	t.Parallel()
	// An orchestrator with NeedsPlanning=true should not be set to complete
	// even when all children are complete. This prevents propagation from
	// overwriting the planning trigger before the planning pass runs.
	orchState := &NodeState{
		ID:            "orch",
		Type:          NodeOrchestrator,
		NeedsPlanning: true,
		Children:      []ChildRef{{ID: "leaf", Address: "leaf", State: StatusNotStarted}},
	}
	states := map[string]*NodeState{"orch": orchState}
	parents := map[string]string{"leaf": "orch"}
	var savedState NodeStatus

	_, err := PropagateUp(
		"leaf",
		StatusComplete,
		func(addr string) (*NodeState, error) { return states[addr], nil },
		func(addr string, ns *NodeState) error { savedState = ns.State; return nil },
		func(addr string) string { return parents[addr] },
	)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if savedState == StatusComplete {
		t.Error("orchestrator with NeedsPlanning=true should not be marked complete")
	}
	if savedState != StatusInProgress {
		t.Errorf("expected in_progress, got %s", savedState)
	}
}

func TestPropagateUp_NormalChain(t *testing.T) {
	t.Parallel()
	states := map[string]*NodeState{
		"root": {ID: "root", Type: NodeOrchestrator, Children: []ChildRef{{ID: "child", Address: "child", State: StatusNotStarted}}},
	}
	parents := map[string]string{"child": "root"}

	updated, err := PropagateUp(
		"child",
		StatusInProgress,
		func(addr string) (*NodeState, error) { return states[addr], nil },
		func(addr string, ns *NodeState) error { return nil },
		func(addr string) string { return parents[addr] },
	)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(updated) != 1 || updated[0] != "root" {
		t.Errorf("expected [root], got %v", updated)
	}
}
