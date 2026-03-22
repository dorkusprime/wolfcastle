package state

import (
	"fmt"
	"math/rand"
	"strings"
	"testing"
	"testing/quick"
)

// ---------- navigation-specific mutations ----------

// mutAddAuditTask adds an audit task to a random leaf.
func mutAddAuditTask(rng *rand.Rand, tree *testTree) {
	if len(tree.leaves) == 0 {
		return
	}
	addr := tree.leaves[rng.Intn(len(tree.leaves))]
	ns := tree.nodes[addr]
	ns.Tasks = append(ns.Tasks, Task{
		ID:          fmt.Sprintf("%s/audit-%d", addr, len(ns.Tasks)),
		Description: "audit task",
		State:       StatusNotStarted,
		IsAudit:     true,
	})
	newState := leafState(ns)
	ns.State = newState
	_ = Propagate(addr, newState, tree.idx, tree.load, tree.save)
}

// mutAddChildTask adds a hierarchical child task under an existing task.
func mutAddChildTask(rng *rand.Rand, tree *testTree) {
	if len(tree.leaves) == 0 {
		return
	}
	addr := tree.leaves[rng.Intn(len(tree.leaves))]
	ns := tree.nodes[addr]
	if len(ns.Tasks) == 0 {
		return
	}
	parent := ns.Tasks[rng.Intn(len(ns.Tasks))]
	childID := fmt.Sprintf("%s.%04d", parent.ID, len(ns.Tasks))
	ns.Tasks = append(ns.Tasks, Task{
		ID:          childID,
		Description: "child task",
		State:       StatusNotStarted,
	})
	newState := leafState(ns)
	ns.State = newState
	_ = Propagate(addr, newState, tree.idx, tree.load, tree.save)
}

// mutAddAuditChildTask adds an audit child task (for re-verification patterns).
func mutAddAuditChildTask(rng *rand.Rand, tree *testTree) {
	if len(tree.leaves) == 0 {
		return
	}
	addr := tree.leaves[rng.Intn(len(tree.leaves))]
	ns := tree.nodes[addr]
	// Find an audit task to parent under.
	var auditTasks []int
	for i, t := range ns.Tasks {
		if t.IsAudit {
			auditTasks = append(auditTasks, i)
		}
	}
	if len(auditTasks) == 0 {
		return
	}
	parent := ns.Tasks[auditTasks[rng.Intn(len(auditTasks))]]
	childID := fmt.Sprintf("%s.%04d", parent.ID, len(ns.Tasks))
	ns.Tasks = append(ns.Tasks, Task{
		ID:          childID,
		Description: "audit remediation child",
		State:       StatusNotStarted,
	})
	newState := leafState(ns)
	ns.State = newState
	_ = Propagate(addr, newState, tree.idx, tree.load, tree.save)
}

// applyNavigationMutation applies mutations including the audit/hierarchy ones.
func applyNavigationMutation(rng *rand.Rand, tree *testTree) {
	for attempt := 0; attempt < 20; attempt++ {
		// 9 mutation types: 6 original + 3 navigation-specific
		mut := rng.Intn(9)
		switch mut {
		case 0:
			if addr, ok := findLeafWithTaskInState(rng, tree, StatusNotStarted); ok {
				claimTask(tree, addr)
				return
			}
		case 1:
			if addr, ok := findLeafWithTaskInState(rng, tree, StatusInProgress); ok {
				completeTask(tree, addr)
				return
			}
		case 2:
			if addr, ok := findLeafWithTaskInState(rng, tree, StatusInProgress); ok {
				blockTask(tree, addr)
				return
			}
		case 3:
			if addr, ok := findLeafWithTaskInState(rng, tree, StatusBlocked); ok {
				unblockTask(tree, addr)
				return
			}
		case 4:
			if len(tree.orchestrators) > 0 {
				addChild(rng, tree)
				return
			}
		case 5:
			if len(tree.leaves) > 0 {
				addTask(rng, tree)
				return
			}
		case 6:
			if len(tree.leaves) > 0 {
				mutAddAuditTask(rng, tree)
				return
			}
		case 7:
			if len(tree.leaves) > 0 {
				mutAddChildTask(rng, tree)
				return
			}
		case 8:
			mutAddAuditChildTask(rng, tree)
			return
		}
	}
}

// ---------- invariant verification ----------

// verifyINV1 checks that a returned task is actionable (not complete, not blocked).
func verifyINV1(t *testing.T, tree *testTree, result *NavigationResult) bool {
	t.Helper()
	if !result.Found {
		return true
	}
	ns := tree.nodes[result.NodeAddress]
	for _, task := range ns.Tasks {
		if task.ID == result.TaskID {
			if task.State != StatusInProgress && task.State != StatusNotStarted {
				t.Logf("INV-1 violated: task %s in node %s has state %s",
					result.TaskID, result.NodeAddress, task.State)
				return false
			}
			return true
		}
	}
	t.Logf("INV-1 violated: task %s not found in node %s", result.TaskID, result.NodeAddress)
	return false
}

// verifyINV2 checks that non-audit tasks take priority over audit tasks within
// the same node. The code enforces audit deferral per-node (allNonAuditDone),
// not tree-wide, because the DFS returns the first actionable task it finds.
// This mirrors the exact allNonAuditDone computation in findActionableTask.
func verifyINV2(t *testing.T, tree *testTree, result *NavigationResult) bool {
	t.Helper()
	if !result.Found {
		return true
	}
	ns := tree.nodes[result.NodeAddress]
	var returnedTask Task
	for _, task := range ns.Tasks {
		if task.ID == result.TaskID {
			returnedTask = task
			break
		}
	}
	if !returnedTask.IsAudit {
		return true
	}
	// The in_progress self-healing loop returns any in_progress task without
	// checking IsAudit. Audit deferral only applies to not_started tasks.
	if returnedTask.State == StatusInProgress {
		return true
	}
	// Mirror the code's allNonAuditDone computation exactly.
	allNonAuditDone := computeAllNonAuditDone(ns)
	if !allNonAuditDone {
		t.Logf("INV-2 violated: audit task %s returned in node %s but allNonAuditDone is false",
			result.TaskID, result.NodeAddress)
		return false
	}
	// The code also skips not_started audit tasks when all non-audit tasks
	// are blocked (navigation.go:207-209). A returned audit task means
	// allNonAuditBlocked must be false.
	if computeAllNonAuditBlocked(ns) {
		t.Logf("INV-2 violated: audit task %s returned in node %s but allNonAuditBlocked is true",
			result.TaskID, result.NodeAddress)
		return false
	}
	return true
}

// computeAllNonAuditDone mirrors findActionableTask's allNonAuditDone logic.
func computeAllNonAuditDone(ns *NodeState) bool {
	sorted := make([]Task, len(ns.Tasks))
	copy(sorted, ns.Tasks)

	nonAuditCount := 0
	nonAuditDone := 0
	for _, task := range sorted {
		if task.IsAudit {
			continue
		}
		if isChildTask(task.ID) && parentInList(task.ID, sorted) {
			continue
		}
		nonAuditCount++
		status := task.State
		if derived, hasChildren := DeriveParentStatus(ns, task.ID); hasChildren {
			status = derived
		}
		switch status {
		case StatusComplete, StatusBlocked:
			nonAuditDone++
		}
	}
	allDone := nonAuditDone == nonAuditCount

	if ns.Type == NodeLeaf && nonAuditCount == 0 {
		allDone = false
	}
	if ns.Type == NodeOrchestrator {
		allChildrenComplete := len(ns.Children) > 0
		for _, child := range ns.Children {
			if child.State != StatusComplete {
				allChildrenComplete = false
				break
			}
		}
		allDone = allChildrenComplete
	}
	return allDone
}

// computeAllNonAuditBlocked mirrors findActionableTask's allNonAuditBlocked logic.
// Returns true when all non-audit top-level tasks are blocked.
func computeAllNonAuditBlocked(ns *NodeState) bool {
	sorted := make([]Task, len(ns.Tasks))
	copy(sorted, ns.Tasks)

	nonAuditCount := 0
	nonAuditBlocked := 0
	for _, task := range sorted {
		if task.IsAudit {
			continue
		}
		if isChildTask(task.ID) && parentInList(task.ID, sorted) {
			continue
		}
		nonAuditCount++
		status := task.State
		if derived, hasChildren := DeriveParentStatus(ns, task.ID); hasChildren {
			status = derived
		}
		if status == StatusBlocked {
			nonAuditBlocked++
		}
	}
	return nonAuditCount > 0 && nonAuditBlocked == nonAuditCount
}

// verifyINV3 checks that parent tasks with children are never returned
// (unless audit with all children complete).
func verifyINV3(t *testing.T, tree *testTree, result *NavigationResult) bool {
	t.Helper()
	if !result.Found {
		return true
	}
	ns := tree.nodes[result.NodeAddress]
	hasChildren := TaskChildren(ns, result.TaskID)
	if !hasChildren {
		return true
	}
	// Exception: audit task whose children are all complete.
	var returnedTask Task
	for _, task := range ns.Tasks {
		if task.ID == result.TaskID {
			returnedTask = task
			break
		}
	}
	if returnedTask.IsAudit && allChildrenComplete(ns, result.TaskID) {
		return true
	}
	t.Logf("INV-3 violated: parent task %s in node %s has children but was returned (isAudit=%v, allChildrenComplete=%v)",
		result.TaskID, result.NodeAddress, returnedTask.IsAudit, allChildrenComplete(ns, result.TaskID))
	return false
}

// verifyINV4 checks that not_started child tasks with not_started non-audit
// ancestors are not returned. The ancestor check applies only to not_started
// tasks; in_progress tasks bypass it (self-healing resumes crashed work
// regardless of ancestor state).
func verifyINV4(t *testing.T, tree *testTree, result *NavigationResult) bool {
	t.Helper()
	if !result.Found {
		return true
	}
	ns := tree.nodes[result.NodeAddress]
	var returnedTask Task
	for _, task := range ns.Tasks {
		if task.ID == result.TaskID {
			returnedTask = task
			break
		}
	}
	// The code only checks hasNotStartedAncestor for not_started tasks.
	if returnedTask.State != StatusNotStarted {
		return true
	}
	if hasNotStartedAncestor(result.TaskID, ns) {
		t.Logf("INV-4 violated: not_started task %s in node %s has a not_started non-audit ancestor",
			result.TaskID, result.NodeAddress)
		return false
	}
	return true
}

// verifyINV5 checks that all-complete yields Found==false with reason "all_complete".
func verifyINV5(t *testing.T, tree *testTree, result *NavigationResult) bool {
	t.Helper()
	allComplete := len(tree.idx.Nodes) > 0
	for _, entry := range tree.idx.Nodes {
		if entry.State != StatusComplete {
			allComplete = false
			break
		}
	}
	if !allComplete {
		return true
	}
	if result.Found {
		t.Logf("INV-5 violated: all nodes complete but Found==true (task=%s, node=%s)",
			result.TaskID, result.NodeAddress)
		return false
	}
	if result.Reason != "all_complete" {
		t.Logf("INV-5 violated: all nodes complete but reason=%q", result.Reason)
		return false
	}
	return true
}

// verifyINV6 checks the all_blocked / all_complete reason logic.
func verifyINV6(t *testing.T, tree *testTree, result *NavigationResult) bool {
	t.Helper()
	if result.Found || len(tree.idx.Nodes) == 0 {
		return true
	}
	hasBlocked := false
	for _, entry := range tree.idx.Nodes {
		if entry.State == StatusBlocked {
			hasBlocked = true
			break
		}
	}
	if hasBlocked {
		if result.Reason != "all_blocked" {
			t.Logf("INV-6 violated: has blocked nodes but reason=%q", result.Reason)
			return false
		}
	} else {
		if result.Reason != "all_complete" {
			t.Logf("INV-6 violated: no blocked nodes but reason=%q", result.Reason)
			return false
		}
	}
	return true
}

// verifyINV7 checks that empty trees yield Found==false with reason "empty_tree".
func verifyINV7(t *testing.T, tree *testTree, result *NavigationResult) bool {
	t.Helper()
	if len(tree.idx.Nodes) != 0 {
		return true
	}
	if result.Found {
		t.Logf("INV-7 violated: empty tree but Found==true")
		return false
	}
	if result.Reason != "empty_tree" {
		t.Logf("INV-7 violated: empty tree but reason=%q", result.Reason)
		return false
	}
	return true
}

// verifyInProgressPriority checks that not_started tasks are only returned
// when no in_progress non-parent task exists in the same node.
func verifyInProgressPriority(t *testing.T, tree *testTree, result *NavigationResult) bool {
	t.Helper()
	if !result.Found {
		return true
	}
	ns := tree.nodes[result.NodeAddress]
	var returnedTask Task
	for _, task := range ns.Tasks {
		if task.ID == result.TaskID {
			returnedTask = task
			break
		}
	}
	if returnedTask.State != StatusNotStarted {
		return true
	}
	// There should be no in_progress non-parent task in this node.
	for _, task := range ns.Tasks {
		if task.State == StatusInProgress && !TaskChildren(ns, task.ID) {
			t.Logf("in-progress priority violated: not_started task %s returned but in_progress task %s exists in node %s",
				result.TaskID, task.ID, result.NodeAddress)
			return false
		}
	}
	return true
}

// ---------- property-based test ----------

func TestFindNextTaskInvariantsRandom(t *testing.T) {
	t.Parallel()

	f := func(seed int64) bool {
		rng := rand.New(rand.NewSource(seed))

		tree := generateRandomTree(rng, 5, 4)

		// Apply 10-50 mutations including audit and hierarchy mutations.
		nMutations := rng.Intn(41) + 10
		for i := 0; i < nMutations; i++ {
			applyNavigationMutation(rng, tree)
		}

		result, err := FindNextTask(tree.idx, "", tree.load)
		if err != nil {
			t.Logf("FindNextTask error: %v", err)
			return false
		}

		return verifyINV1(t, tree, result) &&
			verifyINV2(t, tree, result) &&
			verifyINV3(t, tree, result) &&
			verifyINV4(t, tree, result) &&
			verifyINV5(t, tree, result) &&
			verifyINV6(t, tree, result) &&
			verifyINV7(t, tree, result) &&
			verifyInProgressPriority(t, tree, result)
	}

	if err := quick.Check(f, &quick.Config{MaxCount: 500}); err != nil {
		t.Error(err)
	}
}

func TestFindNextTaskInvariantsRandom_Scoped(t *testing.T) {
	t.Parallel()

	f := func(seed int64) bool {
		rng := rand.New(rand.NewSource(seed))

		tree := generateRandomTree(rng, 5, 4)

		nMutations := rng.Intn(41) + 10
		for i := 0; i < nMutations; i++ {
			applyNavigationMutation(rng, tree)
		}

		// Pick a random node to scope to.
		var addrs []string
		for addr := range tree.idx.Nodes {
			addrs = append(addrs, addr)
		}
		if len(addrs) == 0 {
			return true
		}
		scopeAddr := addrs[rng.Intn(len(addrs))]

		result, err := FindNextTask(tree.idx, scopeAddr, tree.load)
		if err != nil {
			t.Logf("FindNextTask scoped error: %v", err)
			return false
		}

		// When scoped, if Found, the result must be within the scope subtree.
		if result.Found {
			if !strings.HasPrefix(result.NodeAddress, scopeAddr) && result.NodeAddress != scopeAddr {
				t.Logf("scoped violation: result node %s not under scope %s",
					result.NodeAddress, scopeAddr)
				return false
			}
		}

		// INV-1, INV-3, INV-4, and in-progress priority still hold within scope.
		return verifyINV1(t, tree, result) &&
			verifyINV3(t, tree, result) &&
			verifyINV4(t, tree, result) &&
			verifyInProgressPriority(t, tree, result)
	}

	if err := quick.Check(f, &quick.Config{MaxCount: 500}); err != nil {
		t.Error(err)
	}
}

func TestFindNextTaskInvariantsRandom_EmptyTree(t *testing.T) {
	t.Parallel()

	tree := &testTree{
		nodes: make(map[string]*NodeState),
		idx:   NewRootIndex(),
	}

	result, err := FindNextTask(tree.idx, "", tree.load)
	if err != nil {
		t.Fatal(err)
	}

	if !verifyINV7(t, tree, result) {
		t.Fatal("INV-7 failed on empty tree")
	}
}

func TestFindNextTaskInvariantsRandom_AllBlocked(t *testing.T) {
	t.Parallel()

	f := func(seed int64) bool {
		rng := rand.New(rand.NewSource(seed))

		tree := generateRandomTree(rng, 4, 3)

		// Claim then block every task in every leaf.
		for _, addr := range tree.leaves {
			ns := tree.nodes[addr]
			for i := range ns.Tasks {
				ns.Tasks[i].State = StatusInProgress
			}
			for i := range ns.Tasks {
				ns.Tasks[i].State = StatusBlocked
			}
			ns.State = leafState(ns)
			_ = Propagate(addr, ns.State, tree.idx, tree.load, tree.save)
		}

		result, err := FindNextTask(tree.idx, "", tree.load)
		if err != nil {
			t.Logf("FindNextTask error: %v", err)
			return false
		}

		if result.Found {
			t.Logf("expected not found when all tasks blocked, got task %s", result.TaskID)
			return false
		}
		return verifyINV6(t, tree, result)
	}

	if err := quick.Check(f, &quick.Config{MaxCount: 100}); err != nil {
		t.Error(err)
	}
}

func TestFindNextTaskInvariantsRandom_AllComplete(t *testing.T) {
	t.Parallel()

	f := func(seed int64) bool {
		rng := rand.New(rand.NewSource(seed))

		tree := generateRandomTree(rng, 4, 3)

		// Complete every task in every leaf.
		for _, addr := range tree.leaves {
			ns := tree.nodes[addr]
			for i := range ns.Tasks {
				ns.Tasks[i].State = StatusComplete
			}
			ns.State = leafState(ns)
			_ = Propagate(addr, ns.State, tree.idx, tree.load, tree.save)
		}

		result, err := FindNextTask(tree.idx, "", tree.load)
		if err != nil {
			t.Logf("FindNextTask error: %v", err)
			return false
		}

		return verifyINV5(t, tree, result)
	}

	if err := quick.Check(f, &quick.Config{MaxCount: 100}); err != nil {
		t.Error(err)
	}
}

func TestFindNextTaskInvariantsRandom_AuditOnly(t *testing.T) {
	t.Parallel()

	f := func(seed int64) bool {
		rng := rand.New(rand.NewSource(seed))

		tree := generateRandomTree(rng, 4, 3)

		// Replace all tasks with audit tasks.
		for _, addr := range tree.leaves {
			ns := tree.nodes[addr]
			for i := range ns.Tasks {
				ns.Tasks[i].IsAudit = true
				ns.Tasks[i].ID = fmt.Sprintf("%s/audit-%d", addr, i)
			}
		}

		nMutations := rng.Intn(21) + 5
		for i := 0; i < nMutations; i++ {
			applyNavigationMutation(rng, tree)
		}

		result, err := FindNextTask(tree.idx, "", tree.load)
		if err != nil {
			t.Logf("FindNextTask error: %v", err)
			return false
		}

		return verifyINV1(t, tree, result) &&
			verifyINV3(t, tree, result) &&
			verifyINV4(t, tree, result) &&
			verifyInProgressPriority(t, tree, result)
	}

	if err := quick.Check(f, &quick.Config{MaxCount: 100}); err != nil {
		t.Error(err)
	}
}

func TestFindNextTaskInvariantsRandom_DeeplyNested(t *testing.T) {
	t.Parallel()

	f := func(seed int64) bool {
		rng := rand.New(rand.NewSource(seed))

		// Deep trees: depth up to 8, narrow branching.
		tree := generateRandomTree(rng, 8, 2)

		nMutations := rng.Intn(41) + 10
		for i := 0; i < nMutations; i++ {
			applyNavigationMutation(rng, tree)
		}

		result, err := FindNextTask(tree.idx, "", tree.load)
		if err != nil {
			t.Logf("FindNextTask error: %v", err)
			return false
		}

		return verifyINV1(t, tree, result) &&
			verifyINV2(t, tree, result) &&
			verifyINV3(t, tree, result) &&
			verifyINV4(t, tree, result) &&
			verifyINV5(t, tree, result) &&
			verifyINV6(t, tree, result) &&
			verifyINV7(t, tree, result) &&
			verifyInProgressPriority(t, tree, result)
	}

	if err := quick.Check(f, &quick.Config{MaxCount: 100}); err != nil {
		t.Error(err)
	}
}
