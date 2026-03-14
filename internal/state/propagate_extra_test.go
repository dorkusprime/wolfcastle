package state

import (
	"fmt"
	"testing"
)

func TestPropagate_UnknownNodeAddr(t *testing.T) {
	t.Parallel()
	idx := &RootIndex{
		Version: 1,
		Nodes:   map[string]IndexEntry{},
	}

	err := Propagate("unknown", StatusInProgress, idx,
		func(addr string) (*NodeState, error) { return nil, nil },
		func(addr string, ns *NodeState) error { return nil },
	)
	if err != nil {
		t.Fatalf("unexpected error for unknown node: %v", err)
	}
}

func TestPropagate_NoRootArray(t *testing.T) {
	t.Parallel()
	states := map[string]*NodeState{
		"root": {
			ID: "root", Type: NodeOrchestrator, State: StatusNotStarted,
			Children: []ChildRef{{ID: "child", Address: "root/child", State: StatusNotStarted}},
		},
	}
	idx := &RootIndex{
		Version: 1,
		Root:    nil, // no Root array
		Nodes: map[string]IndexEntry{
			"root":       {Name: "Root", Type: NodeOrchestrator, State: StatusNotStarted, Children: []string{"root/child"}},
			"root/child": {Name: "Child", Type: NodeLeaf, State: StatusNotStarted, Parent: "root"},
		},
	}

	err := Propagate("root/child", StatusInProgress, idx,
		func(addr string) (*NodeState, error) {
			ns, ok := states[addr]
			if !ok {
				return nil, fmt.Errorf("not found: %s", addr)
			}
			return ns, nil
		},
		func(addr string, ns *NodeState) error {
			states[addr] = ns
			return nil
		},
	)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	// RootState should not be updated since Root is empty
	if idx.RootState != "" {
		t.Errorf("expected empty RootState when Root array is empty, got %s", idx.RootState)
	}
}

func TestPropagate_PropagateError(t *testing.T) {
	t.Parallel()
	idx := &RootIndex{
		Version: 1,
		Root:    []string{"root"},
		Nodes: map[string]IndexEntry{
			"root":       {Name: "Root", Type: NodeOrchestrator, State: StatusNotStarted, Children: []string{"root/child"}},
			"root/child": {Name: "Child", Type: NodeLeaf, State: StatusNotStarted, Parent: "root"},
		},
	}

	err := Propagate("root/child", StatusInProgress, idx,
		func(addr string) (*NodeState, error) { return nil, fmt.Errorf("load failure") },
		func(addr string, ns *NodeState) error { return nil },
	)
	if err == nil {
		t.Error("expected error when loadNode fails during propagation")
	}
}

func TestPropagate_LoadParentForIndexUpdateError(t *testing.T) {
	t.Parallel()
	callCount := 0
	states := map[string]*NodeState{
		"root": {
			ID: "root", Type: NodeOrchestrator, State: StatusNotStarted,
			Children: []ChildRef{{ID: "child", Address: "root/child", State: StatusNotStarted}},
		},
	}
	idx := &RootIndex{
		Version: 1,
		Root:    []string{"root"},
		Nodes: map[string]IndexEntry{
			"root":       {Name: "Root", Type: NodeOrchestrator, State: StatusNotStarted, Children: []string{"root/child"}},
			"root/child": {Name: "Child", Type: NodeLeaf, State: StatusNotStarted, Parent: "root"},
		},
	}

	err := Propagate("root/child", StatusInProgress, idx,
		func(addr string) (*NodeState, error) {
			callCount++
			// First call from PropagateUp succeeds, second call for re-walk fails
			if callCount > 1 {
				return nil, fmt.Errorf("second load failure")
			}
			ns, ok := states[addr]
			if !ok {
				return nil, fmt.Errorf("not found: %s", addr)
			}
			return ns, nil
		},
		func(addr string, ns *NodeState) error {
			states[addr] = ns
			return nil
		},
	)
	if err == nil {
		t.Error("expected error when loadNode fails during ancestor re-walk")
	}
}
