package state

import (
	"testing"
)

func TestFindParallelTasks(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name      string
		idx       *RootIndex
		scopeAddr string
		nodes     map[string]*NodeState
		maxCount  int
		wantAddrs []string // expected NodeAddress values in order
		wantTasks []string // expected TaskID values in order (parallel to wantAddrs)
	}{
		{
			name: "empty tree returns nil",
			idx:  NewRootIndex(),
			nodes: nil,
			maxCount:  3,
			wantAddrs: nil,
			wantTasks: nil,
		},
		{
			name: "root-level node with no parent returns single result",
			idx: func() *RootIndex {
				idx := NewRootIndex()
				idx.Root = []string{"solo"}
				idx.Nodes["solo"] = IndexEntry{
					Name: "Solo", Type: NodeLeaf, State: StatusNotStarted,
				}
				return idx
			}(),
			nodes: map[string]*NodeState{
				"solo": func() *NodeState {
					ns := NewNodeState("solo", "Solo", NodeLeaf)
					ns.Tasks = []Task{{ID: "task-0001", Description: "only task", State: StatusNotStarted}}
					return ns
				}(),
			},
			maxCount:  5,
			wantAddrs: []string{"solo"},
			wantTasks: []string{"task-0001"},
		},
		{
			name: "three not_started siblings maxCount=3 returns all three in creation order",
			idx: func() *RootIndex {
				idx := NewRootIndex()
				idx.Root = []string{"p"}
				idx.Nodes["p"] = IndexEntry{
					Name: "P", Type: NodeOrchestrator, State: StatusNotStarted,
					Children: []string{"p/a", "p/b", "p/c"},
				}
				idx.Nodes["p/a"] = IndexEntry{Name: "A", Type: NodeLeaf, State: StatusNotStarted, Parent: "p"}
				idx.Nodes["p/b"] = IndexEntry{Name: "B", Type: NodeLeaf, State: StatusNotStarted, Parent: "p"}
				idx.Nodes["p/c"] = IndexEntry{Name: "C", Type: NodeLeaf, State: StatusNotStarted, Parent: "p"}
				return idx
			}(),
			nodes: map[string]*NodeState{
				"p/a": func() *NodeState {
					ns := NewNodeState("p/a", "A", NodeLeaf)
					ns.Tasks = []Task{{ID: "task-0001", Description: "a work", State: StatusNotStarted}}
					return ns
				}(),
				"p/b": func() *NodeState {
					ns := NewNodeState("p/b", "B", NodeLeaf)
					ns.Tasks = []Task{{ID: "task-0001", Description: "b work", State: StatusNotStarted}}
					return ns
				}(),
				"p/c": func() *NodeState {
					ns := NewNodeState("p/c", "C", NodeLeaf)
					ns.Tasks = []Task{{ID: "task-0001", Description: "c work", State: StatusNotStarted}}
					return ns
				}(),
			},
			maxCount:  3,
			wantAddrs: []string{"p/a", "p/b", "p/c"},
			wantTasks: []string{"task-0001", "task-0001", "task-0001"},
		},
		{
			name: "three not_started siblings maxCount=2 returns first two",
			idx: func() *RootIndex {
				idx := NewRootIndex()
				idx.Root = []string{"p"}
				idx.Nodes["p"] = IndexEntry{
					Name: "P", Type: NodeOrchestrator, State: StatusNotStarted,
					Children: []string{"p/a", "p/b", "p/c"},
				}
				idx.Nodes["p/a"] = IndexEntry{Name: "A", Type: NodeLeaf, State: StatusNotStarted, Parent: "p"}
				idx.Nodes["p/b"] = IndexEntry{Name: "B", Type: NodeLeaf, State: StatusNotStarted, Parent: "p"}
				idx.Nodes["p/c"] = IndexEntry{Name: "C", Type: NodeLeaf, State: StatusNotStarted, Parent: "p"}
				return idx
			}(),
			nodes: map[string]*NodeState{
				"p/a": func() *NodeState {
					ns := NewNodeState("p/a", "A", NodeLeaf)
					ns.Tasks = []Task{{ID: "task-0001", Description: "a", State: StatusNotStarted}}
					return ns
				}(),
				"p/b": func() *NodeState {
					ns := NewNodeState("p/b", "B", NodeLeaf)
					ns.Tasks = []Task{{ID: "task-0001", Description: "b", State: StatusNotStarted}}
					return ns
				}(),
				"p/c": func() *NodeState {
					ns := NewNodeState("p/c", "C", NodeLeaf)
					ns.Tasks = []Task{{ID: "task-0001", Description: "c", State: StatusNotStarted}}
					return ns
				}(),
			},
			maxCount:  2,
			wantAddrs: []string{"p/a", "p/b"},
			wantTasks: []string{"task-0001", "task-0001"},
		},
		{
			name: "in_progress sibling is skipped not stopped",
			idx: func() *RootIndex {
				idx := NewRootIndex()
				idx.Root = []string{"p"}
				idx.Nodes["p"] = IndexEntry{
					Name: "P", Type: NodeOrchestrator, State: StatusInProgress,
					Children: []string{"p/a", "p/b", "p/c"},
				}
				idx.Nodes["p/a"] = IndexEntry{Name: "A", Type: NodeLeaf, State: StatusNotStarted, Parent: "p"}
				idx.Nodes["p/b"] = IndexEntry{Name: "B", Type: NodeLeaf, State: StatusInProgress, Parent: "p"}
				idx.Nodes["p/c"] = IndexEntry{Name: "C", Type: NodeLeaf, State: StatusNotStarted, Parent: "p"}
				return idx
			}(),
			nodes: map[string]*NodeState{
				"p/a": func() *NodeState {
					ns := NewNodeState("p/a", "A", NodeLeaf)
					ns.Tasks = []Task{{ID: "task-0001", Description: "a", State: StatusNotStarted}}
					return ns
				}(),
				"p/c": func() *NodeState {
					ns := NewNodeState("p/c", "C", NodeLeaf)
					ns.Tasks = []Task{{ID: "task-0001", Description: "c", State: StatusNotStarted}}
					return ns
				}(),
			},
			maxCount:  5,
			wantAddrs: []string{"p/a", "p/c"},
			wantTasks: []string{"task-0001", "task-0001"},
		},
		{
			name: "unplanned orchestrator stops scanning of later siblings",
			idx: func() *RootIndex {
				idx := NewRootIndex()
				idx.Root = []string{"p"}
				idx.Nodes["p"] = IndexEntry{
					Name: "P", Type: NodeOrchestrator, State: StatusNotStarted,
					Children: []string{"p/a", "p/unplanned", "p/c"},
				}
				idx.Nodes["p/a"] = IndexEntry{Name: "A", Type: NodeLeaf, State: StatusNotStarted, Parent: "p"}
				idx.Nodes["p/unplanned"] = IndexEntry{
					Name: "Unplanned", Type: NodeOrchestrator, State: StatusNotStarted,
					Parent: "p", Children: []string{}, // empty children = unplanned
				}
				idx.Nodes["p/c"] = IndexEntry{Name: "C", Type: NodeLeaf, State: StatusNotStarted, Parent: "p"}
				return idx
			}(),
			nodes: map[string]*NodeState{
				"p/a": func() *NodeState {
					ns := NewNodeState("p/a", "A", NodeLeaf)
					ns.Tasks = []Task{{ID: "task-0001", Description: "a", State: StatusNotStarted}}
					return ns
				}(),
				"p/c": func() *NodeState {
					ns := NewNodeState("p/c", "C", NodeLeaf)
					ns.Tasks = []Task{{ID: "task-0001", Description: "c", State: StatusNotStarted}}
					return ns
				}(),
			},
			maxCount:  5,
			wantAddrs: []string{"p/a"},
			wantTasks: []string{"task-0001"},
		},
		{
			name: "all siblings complete except audit returns audit task",
			idx: func() *RootIndex {
				idx := NewRootIndex()
				idx.Root = []string{"p"}
				idx.Nodes["p"] = IndexEntry{
					Name: "P", Type: NodeOrchestrator, State: StatusInProgress,
					Children: []string{"p/a", "p/b"},
				}
				idx.Nodes["p/a"] = IndexEntry{Name: "A", Type: NodeLeaf, State: StatusComplete, Parent: "p"}
				idx.Nodes["p/b"] = IndexEntry{Name: "B", Type: NodeLeaf, State: StatusComplete, Parent: "p"}
				return idx
			}(),
			nodes: map[string]*NodeState{
				// The orchestrator itself has an audit task. When all children
				// are complete, findActionableTask on the orchestrator returns
				// the audit.
				"p": func() *NodeState {
					ns := NewNodeState("p", "P", NodeOrchestrator)
					ns.Children = []ChildRef{
						{ID: "a", Address: "p/a", State: StatusComplete},
						{ID: "b", Address: "p/b", State: StatusComplete},
					}
					ns.Tasks = []Task{
						{ID: "task-0001", Description: "implement things", State: StatusComplete},
						{ID: "audit", Description: "audit the node", State: StatusNotStarted, IsAudit: true},
					}
					return ns
				}(),
			},
			maxCount:  3,
			wantAddrs: []string{"p"},
			wantTasks: []string{"audit"},
		},
		{
			name: "mixed states: complete + blocked + not_started returns only not_started",
			idx: func() *RootIndex {
				idx := NewRootIndex()
				idx.Root = []string{"p"}
				idx.Nodes["p"] = IndexEntry{
					Name: "P", Type: NodeOrchestrator, State: StatusInProgress,
					Children: []string{"p/done", "p/stuck", "p/ready"},
				}
				idx.Nodes["p/done"] = IndexEntry{Name: "Done", Type: NodeLeaf, State: StatusComplete, Parent: "p"}
				idx.Nodes["p/stuck"] = IndexEntry{Name: "Stuck", Type: NodeLeaf, State: StatusBlocked, Parent: "p"}
				idx.Nodes["p/ready"] = IndexEntry{Name: "Ready", Type: NodeLeaf, State: StatusNotStarted, Parent: "p"}
				return idx
			}(),
			nodes: map[string]*NodeState{
				"p/ready": func() *NodeState {
					ns := NewNodeState("p/ready", "Ready", NodeLeaf)
					ns.Tasks = []Task{{ID: "task-0001", Description: "go", State: StatusNotStarted}}
					return ns
				}(),
			},
			maxCount:  5,
			wantAddrs: []string{"p/ready"},
			wantTasks: []string{"task-0001"},
		},
		{
			name: "scopeAddr restricts search to subtree",
			idx: func() *RootIndex {
				idx := NewRootIndex()
				idx.Root = []string{"alpha", "beta"}
				idx.Nodes["alpha"] = IndexEntry{
					Name: "Alpha", Type: NodeOrchestrator, State: StatusNotStarted,
					Children: []string{"alpha/x", "alpha/y"},
				}
				idx.Nodes["alpha/x"] = IndexEntry{Name: "X", Type: NodeLeaf, State: StatusNotStarted, Parent: "alpha"}
				idx.Nodes["alpha/y"] = IndexEntry{Name: "Y", Type: NodeLeaf, State: StatusNotStarted, Parent: "alpha"}
				idx.Nodes["beta"] = IndexEntry{Name: "Beta", Type: NodeLeaf, State: StatusNotStarted}
				return idx
			}(),
			scopeAddr: "alpha",
			nodes: map[string]*NodeState{
				"alpha/x": func() *NodeState {
					ns := NewNodeState("alpha/x", "X", NodeLeaf)
					ns.Tasks = []Task{{ID: "task-0001", Description: "x work", State: StatusNotStarted}}
					return ns
				}(),
				"alpha/y": func() *NodeState {
					ns := NewNodeState("alpha/y", "Y", NodeLeaf)
					ns.Tasks = []Task{{ID: "task-0001", Description: "y work", State: StatusNotStarted}}
					return ns
				}(),
				"beta": func() *NodeState {
					ns := NewNodeState("beta", "Beta", NodeLeaf)
					ns.Tasks = []Task{{ID: "task-0001", Description: "beta work", State: StatusNotStarted}}
					return ns
				}(),
			},
			maxCount:  5,
			wantAddrs: []string{"alpha/x", "alpha/y"},
			wantTasks: []string{"task-0001", "task-0001"},
		},
		{
			name: "orchestrator sibling with children recurses into it",
			idx: func() *RootIndex {
				idx := NewRootIndex()
				idx.Root = []string{"p"}
				idx.Nodes["p"] = IndexEntry{
					Name: "P", Type: NodeOrchestrator, State: StatusNotStarted,
					Children: []string{"p/leaf", "p/sub-orch"},
				}
				idx.Nodes["p/leaf"] = IndexEntry{
					Name: "Leaf", Type: NodeLeaf, State: StatusNotStarted, Parent: "p",
				}
				idx.Nodes["p/sub-orch"] = IndexEntry{
					Name: "SubOrch", Type: NodeOrchestrator, State: StatusNotStarted, Parent: "p",
					Children: []string{"p/sub-orch/inner"},
				}
				idx.Nodes["p/sub-orch/inner"] = IndexEntry{
					Name: "Inner", Type: NodeLeaf, State: StatusNotStarted, Parent: "p/sub-orch",
				}
				return idx
			}(),
			nodes: map[string]*NodeState{
				"p/leaf": func() *NodeState {
					ns := NewNodeState("p/leaf", "Leaf", NodeLeaf)
					ns.Tasks = []Task{{ID: "task-0001", Description: "leaf work", State: StatusNotStarted}}
					return ns
				}(),
				"p/sub-orch": func() *NodeState {
					ns := NewNodeState("p/sub-orch", "SubOrch", NodeOrchestrator)
					ns.Children = []ChildRef{
						{ID: "inner", Address: "p/sub-orch/inner", State: StatusNotStarted},
					}
					ns.Tasks = []Task{{ID: "task-0001", Description: "sub-orch setup", State: StatusNotStarted}}
					return ns
				}(),
				"p/sub-orch/inner": func() *NodeState {
					ns := NewNodeState("p/sub-orch/inner", "Inner", NodeLeaf)
					ns.Tasks = []Task{{ID: "task-0001", Description: "inner work", State: StatusNotStarted}}
					return ns
				}(),
			},
			maxCount:  5,
			wantAddrs: []string{"p/leaf", "p/sub-orch"},
			wantTasks: []string{"task-0001", "task-0001"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			results, err := FindParallelTasks(tt.idx, tt.scopeAddr, makeLoadNode(tt.nodes), tt.maxCount)
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}

			if len(results) != len(tt.wantAddrs) {
				t.Fatalf("got %d results, want %d", len(results), len(tt.wantAddrs))
			}

			for i, r := range results {
				if r.NodeAddress != tt.wantAddrs[i] {
					t.Errorf("result[%d].NodeAddress = %q, want %q", i, r.NodeAddress, tt.wantAddrs[i])
				}
				if r.TaskID != tt.wantTasks[i] {
					t.Errorf("result[%d].TaskID = %q, want %q", i, r.TaskID, tt.wantTasks[i])
				}
				if !r.Found {
					t.Errorf("result[%d].Found = false, want true", i)
				}
			}
		})
	}
}
