package state

import (
	"fmt"
	"testing"
)

func TestPropagate_ClaimTransitionsParentToInProgress(t *testing.T) {
	t.Parallel()
	states := map[string]*NodeState{
		"root": {
			ID: "root", Type: NodeOrchestrator, State: StatusNotStarted,
			Children: []ChildRef{
				{ID: "child", Address: "root/child", State: StatusNotStarted},
			},
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

	loadNode := func(addr string) (*NodeState, error) {
		ns, ok := states[addr]
		if !ok {
			return nil, fmt.Errorf("not found: %s", addr)
		}
		return ns, nil
	}
	saveNode := func(addr string, ns *NodeState) error {
		states[addr] = ns
		return nil
	}

	err := Propagate("root/child", StatusInProgress, idx, loadNode, saveNode)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if states["root"].State != StatusInProgress {
		t.Errorf("expected root in_progress, got %s", states["root"].State)
	}
	if idx.Nodes["root"].State != StatusInProgress {
		t.Errorf("expected root index entry in_progress, got %s", idx.Nodes["root"].State)
	}
	if idx.Nodes["root/child"].State != StatusInProgress {
		t.Errorf("expected child index entry in_progress, got %s", idx.Nodes["root/child"].State)
	}
}

func TestPropagate_LastChildCompleteTransitionsAncestors(t *testing.T) {
	t.Parallel()
	states := map[string]*NodeState{
		"root": {
			ID: "root", Type: NodeOrchestrator, State: StatusInProgress,
			Children: []ChildRef{
				{ID: "a", Address: "root/a", State: StatusComplete},
				{ID: "b", Address: "root/b", State: StatusInProgress},
			},
		},
	}
	idx := &RootIndex{
		Version: 1,
		Root:    []string{"root"},
		Nodes: map[string]IndexEntry{
			"root":   {Name: "Root", Type: NodeOrchestrator, State: StatusInProgress, Children: []string{"root/a", "root/b"}},
			"root/a": {Name: "A", Type: NodeLeaf, State: StatusComplete, Parent: "root"},
			"root/b": {Name: "B", Type: NodeLeaf, State: StatusInProgress, Parent: "root"},
		},
	}

	loadNode := func(addr string) (*NodeState, error) {
		ns, ok := states[addr]
		if !ok {
			return nil, fmt.Errorf("not found: %s", addr)
		}
		return ns, nil
	}
	saveNode := func(addr string, ns *NodeState) error {
		states[addr] = ns
		return nil
	}

	err := Propagate("root/b", StatusComplete, idx, loadNode, saveNode)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if states["root"].State != StatusComplete {
		t.Errorf("expected root complete, got %s", states["root"].State)
	}
	if idx.Nodes["root"].State != StatusComplete {
		t.Errorf("expected root index complete, got %s", idx.Nodes["root"].State)
	}
}

func TestPropagate_BlockOnlyWhenAllNonCompleteBlocked(t *testing.T) {
	t.Parallel()
	states := map[string]*NodeState{
		"root": {
			ID: "root", Type: NodeOrchestrator, State: StatusInProgress,
			Children: []ChildRef{
				{ID: "a", Address: "root/a", State: StatusComplete},
				{ID: "b", Address: "root/b", State: StatusBlocked},
				{ID: "c", Address: "root/c", State: StatusNotStarted},
			},
		},
	}
	idx := &RootIndex{
		Version: 1,
		Root:    []string{"root"},
		Nodes: map[string]IndexEntry{
			"root":   {Name: "Root", Type: NodeOrchestrator, State: StatusInProgress, Children: []string{"root/a", "root/b", "root/c"}},
			"root/a": {Name: "A", Type: NodeLeaf, State: StatusComplete, Parent: "root"},
			"root/b": {Name: "B", Type: NodeLeaf, State: StatusBlocked, Parent: "root"},
			"root/c": {Name: "C", Type: NodeLeaf, State: StatusNotStarted, Parent: "root"},
		},
	}

	loadNode := func(addr string) (*NodeState, error) {
		ns, ok := states[addr]
		if !ok {
			return nil, fmt.Errorf("not found: %s", addr)
		}
		return ns, nil
	}
	saveNode := func(addr string, ns *NodeState) error {
		states[addr] = ns
		return nil
	}

	// Block c. Now b and c are blocked, a is complete
	err := Propagate("root/c", StatusBlocked, idx, loadNode, saveNode)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if states["root"].State != StatusBlocked {
		t.Errorf("expected root blocked (all non-complete blocked), got %s", states["root"].State)
	}
}

func TestPropagate_BlockDoesNotBubbleWithAvailableSiblings(t *testing.T) {
	t.Parallel()
	states := map[string]*NodeState{
		"root": {
			ID: "root", Type: NodeOrchestrator, State: StatusInProgress,
			Children: []ChildRef{
				{ID: "a", Address: "root/a", State: StatusNotStarted},
				{ID: "b", Address: "root/b", State: StatusInProgress},
			},
		},
	}
	idx := &RootIndex{
		Version: 1,
		Root:    []string{"root"},
		Nodes: map[string]IndexEntry{
			"root":   {Name: "Root", Type: NodeOrchestrator, State: StatusInProgress, Children: []string{"root/a", "root/b"}},
			"root/a": {Name: "A", Type: NodeLeaf, State: StatusNotStarted, Parent: "root"},
			"root/b": {Name: "B", Type: NodeLeaf, State: StatusInProgress, Parent: "root"},
		},
	}

	loadNode := func(addr string) (*NodeState, error) {
		ns, ok := states[addr]
		if !ok {
			return nil, fmt.Errorf("not found: %s", addr)
		}
		return ns, nil
	}
	saveNode := func(addr string, ns *NodeState) error {
		states[addr] = ns
		return nil
	}

	// Block b. But a is still not_started, so root stays in_progress
	err := Propagate("root/b", StatusBlocked, idx, loadNode, saveNode)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if states["root"].State != StatusInProgress {
		t.Errorf("expected root in_progress (sibling available), got %s", states["root"].State)
	}
}

func TestPropagate_UpdatesRootState(t *testing.T) {
	t.Parallel()
	states := map[string]*NodeState{
		"root": {
			ID: "root", Type: NodeOrchestrator, State: StatusNotStarted,
			Children: []ChildRef{
				{ID: "child", Address: "root/child", State: StatusNotStarted},
			},
		},
	}
	idx := &RootIndex{
		Version:   1,
		Root:      []string{"root"},
		RootState: StatusNotStarted,
		Nodes: map[string]IndexEntry{
			"root":       {Name: "Root", Type: NodeOrchestrator, State: StatusNotStarted, Children: []string{"root/child"}},
			"root/child": {Name: "Child", Type: NodeLeaf, State: StatusNotStarted, Parent: "root"},
		},
	}

	loadNode := func(addr string) (*NodeState, error) {
		ns, ok := states[addr]
		if !ok {
			return nil, fmt.Errorf("not found: %s", addr)
		}
		return ns, nil
	}
	saveNode := func(addr string, ns *NodeState) error {
		states[addr] = ns
		return nil
	}

	err := Propagate("root/child", StatusInProgress, idx, loadNode, saveNode)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if idx.RootState != StatusInProgress {
		t.Errorf("expected RootState in_progress, got %s", idx.RootState)
	}
}
