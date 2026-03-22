package state

import (
	"fmt"
	"math/rand"
	"testing"
	"testing/quick"
)

// ---------- random tree generation ----------

type testTree struct {
	nodes map[string]*NodeState
	idx   *RootIndex
	// ordered list of leaf addresses (mutation targets)
	leaves []string
	// ordered list of orchestrator addresses
	orchestrators []string
}

// generateRandomTree builds an in-memory tree with the given depth and branching constraints.
// Depth ranges from 2 to maxDepth, branching from 1 to maxBranching children per orchestrator.
func generateRandomTree(rng *rand.Rand, maxDepth, maxBranching int) *testTree {
	t := &testTree{
		nodes: make(map[string]*NodeState),
		idx:   NewRootIndex(),
	}

	depth := rng.Intn(maxDepth-1) + 2 // 2..maxDepth
	rootAddr := "root"
	t.buildSubtree(rng, rootAddr, 0, depth, maxBranching)

	t.idx.Root = []string{rootAddr}
	t.idx.RootState = t.idx.Nodes[rootAddr].State

	return t
}

func (t *testTree) buildSubtree(rng *rand.Rand, addr string, currentDepth, maxDepth, maxBranching int) {
	isLeaf := currentDepth >= maxDepth-1

	nodeType := NodeLeaf
	if !isLeaf {
		nodeType = NodeOrchestrator
	}

	ns := NewNodeState(addr, addr, nodeType)
	ns.DecompositionDepth = currentDepth
	t.nodes[addr] = ns

	entry := IndexEntry{
		Name:               addr,
		Type:               nodeType,
		State:              StatusNotStarted,
		Address:            addr,
		DecompositionDepth: currentDepth,
	}

	if isLeaf {
		// Add 1-3 tasks to leaves
		nTasks := rng.Intn(3) + 1
		for i := 0; i < nTasks; i++ {
			ns.Tasks = append(ns.Tasks, Task{
				ID:          fmt.Sprintf("%s/task-%d", addr, i),
				Description: fmt.Sprintf("task %d", i),
				State:       StatusNotStarted,
			})
		}
		t.leaves = append(t.leaves, addr)
	} else {
		nChildren := rng.Intn(maxBranching) + 1
		var childAddrs []string
		for i := 0; i < nChildren; i++ {
			childAddr := fmt.Sprintf("%s/c%d", addr, i)
			childAddrs = append(childAddrs, childAddr)
			t.buildSubtree(rng, childAddr, currentDepth+1, maxDepth, maxBranching)

			ns.Children = append(ns.Children, ChildRef{
				ID:      childAddr,
				Address: childAddr,
				State:   StatusNotStarted,
			})

			// Set parent in child's index entry
			childEntry := t.idx.Nodes[childAddr]
			childEntry.Parent = addr
			t.idx.Nodes[childAddr] = childEntry
		}
		entry.Children = childAddrs
		t.orchestrators = append(t.orchestrators, addr)
	}

	t.idx.Nodes[addr] = entry
}

func (t *testTree) load(addr string) (*NodeState, error) {
	ns, ok := t.nodes[addr]
	if !ok {
		return nil, fmt.Errorf("node not found: %s", addr)
	}
	return ns, nil
}

func (t *testTree) save(addr string, ns *NodeState) error {
	t.nodes[addr] = ns
	return nil
}

// ---------- mutation types ----------

type mutationType int

const (
	mutClaim mutationType = iota
	mutComplete
	mutBlock
	mutUnblock
	mutAddChild
	mutAddTask
)

// randomMutation picks a valid mutation and applies it to the tree.
func applyRandomMutation(rng *rand.Rand, tree *testTree) {
	// Try up to 20 times to find an applicable mutation
	for attempt := 0; attempt < 20; attempt++ {
		mut := mutationType(rng.Intn(6))
		switch mut {
		case mutClaim:
			if addr, ok := findLeafWithTaskInState(rng, tree, StatusNotStarted); ok {
				claimTask(tree, addr)
				return
			}
		case mutComplete:
			if addr, ok := findLeafWithTaskInState(rng, tree, StatusInProgress); ok {
				completeTask(tree, addr)
				return
			}
		case mutBlock:
			if addr, ok := findLeafWithTaskInState(rng, tree, StatusInProgress); ok {
				blockTask(tree, addr)
				return
			}
		case mutUnblock:
			if addr, ok := findLeafWithTaskInState(rng, tree, StatusBlocked); ok {
				unblockTask(tree, addr)
				return
			}
		case mutAddChild:
			if len(tree.orchestrators) > 0 {
				addChild(rng, tree)
				return
			}
		case mutAddTask:
			if len(tree.leaves) > 0 {
				addTask(rng, tree)
				return
			}
		}
	}
}

func findLeafWithTaskInState(rng *rand.Rand, tree *testTree, status NodeStatus) (string, bool) {
	// Shuffle leaves to pick randomly
	perm := rng.Perm(len(tree.leaves))
	for _, i := range perm {
		addr := tree.leaves[i]
		ns := tree.nodes[addr]
		for _, task := range ns.Tasks {
			if task.State == status {
				return addr, true
			}
		}
	}
	return "", false
}

func leafState(ns *NodeState) NodeStatus {
	// Derive leaf state from its tasks, mirroring RecomputeState logic
	if len(ns.Tasks) == 0 {
		return StatusNotStarted
	}
	allNotStarted := true
	allComplete := true
	anyBlocked := false
	for _, t := range ns.Tasks {
		if t.State != StatusNotStarted {
			allNotStarted = false
		}
		if t.State != StatusComplete {
			allComplete = false
		}
		if t.State == StatusBlocked {
			anyBlocked = true
		}
	}
	if allNotStarted {
		return StatusNotStarted
	}
	if allComplete {
		return StatusComplete
	}
	if anyBlocked {
		allNonCompleteBlocked := true
		for _, t := range ns.Tasks {
			if t.State != StatusComplete && t.State != StatusBlocked {
				allNonCompleteBlocked = false
				break
			}
		}
		if allNonCompleteBlocked {
			return StatusBlocked
		}
	}
	return StatusInProgress
}

func claimTask(tree *testTree, addr string) {
	ns := tree.nodes[addr]
	for i := range ns.Tasks {
		if ns.Tasks[i].State == StatusNotStarted {
			ns.Tasks[i].State = StatusInProgress
			break
		}
	}
	ns.State = leafState(ns)
	_ = Propagate(addr, ns.State, tree.idx, tree.load, tree.save)
}

func completeTask(tree *testTree, addr string) {
	ns := tree.nodes[addr]
	for i := range ns.Tasks {
		if ns.Tasks[i].State == StatusInProgress {
			ns.Tasks[i].State = StatusComplete
			break
		}
	}
	ns.State = leafState(ns)
	_ = Propagate(addr, ns.State, tree.idx, tree.load, tree.save)
}

func blockTask(tree *testTree, addr string) {
	ns := tree.nodes[addr]
	for i := range ns.Tasks {
		if ns.Tasks[i].State == StatusInProgress {
			ns.Tasks[i].State = StatusBlocked
			break
		}
	}
	ns.State = leafState(ns)
	_ = Propagate(addr, ns.State, tree.idx, tree.load, tree.save)
}

func unblockTask(tree *testTree, addr string) {
	ns := tree.nodes[addr]
	for i := range ns.Tasks {
		if ns.Tasks[i].State == StatusBlocked {
			ns.Tasks[i].State = StatusNotStarted
			break
		}
	}
	ns.State = leafState(ns)
	_ = Propagate(addr, ns.State, tree.idx, tree.load, tree.save)
}

func addChild(rng *rand.Rand, tree *testTree) {
	parentAddr := tree.orchestrators[rng.Intn(len(tree.orchestrators))]
	parent := tree.nodes[parentAddr]
	childAddr := fmt.Sprintf("%s/new%d", parentAddr, len(parent.Children))

	childNS := NewNodeState(childAddr, childAddr, NodeLeaf)
	childNS.DecompositionDepth = parent.DecompositionDepth + 1
	childNS.Tasks = []Task{{
		ID:          childAddr + "/task-0",
		Description: "new task",
		State:       StatusNotStarted,
	}}
	tree.nodes[childAddr] = childNS
	tree.leaves = append(tree.leaves, childAddr)

	parent.Children = append(parent.Children, ChildRef{
		ID:      childAddr,
		Address: childAddr,
		State:   StatusNotStarted,
	})

	// Update index
	parentEntry := tree.idx.Nodes[parentAddr]
	parentEntry.Children = append(parentEntry.Children, childAddr)
	tree.idx.Nodes[parentAddr] = parentEntry

	tree.idx.Nodes[childAddr] = IndexEntry{
		Name:               childAddr,
		Type:               NodeLeaf,
		State:              StatusNotStarted,
		Address:            childAddr,
		DecompositionDepth: childNS.DecompositionDepth,
		Parent:             parentAddr,
	}

	// Recompute parent state after adding child
	newState := RecomputeState(parent.Children)
	parent.State = newState
	_ = Propagate(parentAddr, newState, tree.idx, tree.load, tree.save)
}

func addTask(rng *rand.Rand, tree *testTree) {
	addr := tree.leaves[rng.Intn(len(tree.leaves))]
	ns := tree.nodes[addr]
	ns.Tasks = append(ns.Tasks, Task{
		ID:          fmt.Sprintf("%s/task-%d", addr, len(ns.Tasks)),
		Description: "added task",
		State:       StatusNotStarted,
	})
	// Adding a not_started task might change state if leaf was complete
	newState := leafState(ns)
	ns.State = newState
	_ = Propagate(addr, newState, tree.idx, tree.load, tree.save)
}

// ---------- invariant checks ----------

func verifyParentChildConsistency(t *testing.T, tree *testTree) bool {
	t.Helper()
	for addr, ns := range tree.nodes {
		if ns.Type != NodeOrchestrator || len(ns.Children) == 0 {
			continue
		}
		expected := RecomputeState(ns.Children)
		if ns.State != expected {
			t.Logf("parent-child inconsistency at %s: state=%s, expected=%s, children=%v",
				addr, ns.State, expected, ns.Children)
			return false
		}
	}
	return true
}

func verifyRootIndexConsistency(t *testing.T, tree *testTree) bool {
	t.Helper()
	for addr, entry := range tree.idx.Nodes {
		ns, ok := tree.nodes[addr]
		if !ok {
			t.Logf("index references missing node: %s", addr)
			return false
		}
		if entry.State != ns.State {
			t.Logf("index inconsistency at %s: index=%s, node=%s", addr, entry.State, ns.State)
			return false
		}
	}
	// Every node must also appear in the index
	for addr := range tree.nodes {
		if _, ok := tree.idx.Nodes[addr]; !ok {
			t.Logf("node %s not in index", addr)
			return false
		}
	}
	return true
}

func verifyIdempotency(t *testing.T, tree *testTree) bool {
	t.Helper()
	// Save current states
	statesBefore := make(map[string]NodeStatus)
	for addr, ns := range tree.nodes {
		statesBefore[addr] = ns.State
	}
	indexBefore := make(map[string]NodeStatus)
	for addr, entry := range tree.idx.Nodes {
		indexBefore[addr] = entry.State
	}

	// Propagate every leaf again
	for _, addr := range tree.leaves {
		ns := tree.nodes[addr]
		_ = Propagate(addr, ns.State, tree.idx, tree.load, tree.save)
	}

	// Verify nothing changed
	for addr, ns := range tree.nodes {
		if ns.State != statesBefore[addr] {
			t.Logf("idempotency violation at node %s: was %s, now %s", addr, statesBefore[addr], ns.State)
			return false
		}
	}
	for addr, entry := range tree.idx.Nodes {
		if entry.State != indexBefore[addr] {
			t.Logf("idempotency violation in index at %s: was %s, now %s", addr, indexBefore[addr], entry.State)
			return false
		}
	}
	return true
}

func verifyDepthConsistency(t *testing.T, tree *testTree) bool {
	t.Helper()
	for addr, entry := range tree.idx.Nodes {
		if entry.Parent == "" {
			continue
		}
		parentEntry, ok := tree.idx.Nodes[entry.Parent]
		if !ok {
			t.Logf("depth check: parent %s of %s not in index", entry.Parent, addr)
			return false
		}
		if entry.DecompositionDepth != parentEntry.DecompositionDepth+1 {
			t.Logf("depth inconsistency at %s: depth=%d, parent %s depth=%d",
				addr, entry.DecompositionDepth, entry.Parent, parentEntry.DecompositionDepth)
			return false
		}
	}
	return true
}

// ---------- property-based test ----------

func TestPropagationInvariantsRandom(t *testing.T) {
	t.Parallel()

	f := func(seed int64) bool {
		rng := rand.New(rand.NewSource(seed))

		tree := generateRandomTree(rng, 5, 4)

		// Apply 10-50 random mutations
		nMutations := rng.Intn(41) + 10
		for i := 0; i < nMutations; i++ {
			applyRandomMutation(rng, tree)
		}

		return verifyParentChildConsistency(t, tree) &&
			verifyRootIndexConsistency(t, tree) &&
			verifyIdempotency(t, tree) &&
			verifyDepthConsistency(t, tree)
	}

	if err := quick.Check(f, &quick.Config{MaxCount: 500}); err != nil {
		t.Error(err)
	}
}

// ---------- cycle detection test ----------

func TestPropagationDetectsCycle(t *testing.T) {
	t.Parallel()

	idx := NewRootIndex()
	idx.Nodes["parent"] = IndexEntry{
		Name:     "parent",
		Type:     NodeOrchestrator,
		State:    StatusNotStarted,
		Address:  "parent",
		Children: []string{"child"},
	}
	idx.Nodes["child"] = IndexEntry{
		Name:    "child",
		Type:    NodeLeaf,
		State:   StatusComplete,
		Address: "child",
		Parent:  "parent",
	}

	// Corrupt: make parent's parent point to child, creating a cycle
	entry := idx.Nodes["parent"]
	entry.Parent = "child"
	idx.Nodes["parent"] = entry

	nodes := map[string]*NodeState{
		"parent": NewNodeState("parent", "parent", NodeOrchestrator),
		"child":  NewNodeState("child", "child", NodeLeaf),
	}
	nodes["child"].State = StatusComplete
	nodes["parent"].Children = []ChildRef{
		{ID: "child", Address: "child", State: StatusNotStarted},
	}

	load := func(addr string) (*NodeState, error) { return nodes[addr], nil }
	save := func(addr string, ns *NodeState) error { nodes[addr] = ns; return nil }

	// Propagate must return an error, not hang
	err := Propagate("child", StatusComplete, idx, load, save)
	if err == nil {
		t.Fatal("expected error from cyclic parent chain, got nil")
	}
}
