package state

import (
	"fmt"
	"sort"
	"strings"
)

// NavigationResult is the output of FindNextTask.
type NavigationResult struct {
	NodeAddress string `json:"node_address"`
	TaskID      string `json:"task_id"`
	Description string `json:"description"`
	Found       bool   `json:"found"`
	Reason      string `json:"reason,omitempty"`
}

// FindNextTask performs depth-first traversal to find the next actionable task.
// If scopeAddr is non-empty, only searches within that subtree.
func FindNextTask(idx *RootIndex, scopeAddr string, loadNode func(addr string) (*NodeState, error)) (*NavigationResult, error) {
	// Find root nodes to traverse
	var roots []string
	if scopeAddr != "" {
		entry, ok := idx.Nodes[scopeAddr]
		if !ok {
			return nil, fmt.Errorf("node %q not found in index", scopeAddr)
		}
		if entry.State == StatusComplete {
			return &NavigationResult{Reason: "scoped node is complete"}, nil
		}
		if entry.State == StatusBlocked {
			return &NavigationResult{Reason: "scoped node is blocked"}, nil
		}
		roots = []string{scopeAddr}
	} else {
		// Use Root array for deterministic O(1) root discovery
		if len(idx.Root) > 0 {
			roots = idx.Root
		} else {
			// Fallback: find all top-level nodes (no parent), sorted for determinism
			for addr, entry := range idx.Nodes {
				if entry.Parent == "" {
					roots = append(roots, addr)
				}
			}
			sort.Strings(roots)
		}
	}

	// Depth-first search
	for _, root := range roots {
		result, err := dfs(idx, root, loadNode)
		if err != nil {
			return nil, err
		}
		if result != nil && result.Found {
			return result, nil
		}
	}

	// Determine why no work was found
	if len(idx.Nodes) == 0 {
		return &NavigationResult{Reason: "empty_tree"}, nil
	}
	reason := "all_complete"
	for _, entry := range idx.Nodes {
		if entry.State == StatusBlocked {
			reason = "all_blocked"
			break
		}
	}
	return &NavigationResult{Reason: reason}, nil
}

func dfs(idx *RootIndex, addr string, loadNode func(addr string) (*NodeState, error)) (*NavigationResult, error) {
	entry, ok := idx.Nodes[addr]
	if !ok {
		return nil, nil
	}

	// Skip complete or blocked nodes
	if entry.State == StatusComplete || entry.State == StatusBlocked {
		return nil, nil
	}

	if entry.Type == NodeOrchestrator {
		// Traverse children strictly in creation order. If an earlier
		// child is incomplete, do not skip to later siblings. This
		// enforces the creation-order-equals-execution-order contract.
		for _, childAddr := range entry.Children {
			childEntry, childOk := idx.Nodes[childAddr]
			if !childOk {
				continue
			}
			if childEntry.State == StatusComplete || childEntry.State == StatusBlocked {
				continue
			}
			// If this child is an orchestrator that needs planning (no
			// children yet), stop searching. The daemon's planning pass
			// will handle it before any later sibling executes.
			if childEntry.Type == NodeOrchestrator && len(childEntry.Children) == 0 {
				return nil, nil
			}
			result, err := dfs(idx, childAddr, loadNode)
			if err != nil {
				return nil, err
			}
			if result != nil && result.Found {
				return result, nil
			}
			// If DFS found nothing actionable in this incomplete child,
			// stop. Don't skip to later siblings. The child has work
			// that needs planning or unblocking before later siblings
			// can proceed.
			return nil, nil
		}
		// Children exhausted — check orchestrator's own tasks (e.g. audit)
		return findActionableTask(addr, loadNode)
	}

	// Leaf node — find next actionable task
	return findActionableTask(addr, loadNode)
}

// findActionableTask loads a node's state and returns the first actionable task.
// It prefers in_progress tasks (self-healing) over not_started ones.
// Tasks are sorted lexicographically for depth-first hierarchical ordering.
// Audit tasks are deferred until all non-audit tasks are complete.
// Child tasks are skipped if their parent task is not_started.
func findActionableTask(addr string, loadNode func(addr string) (*NodeState, error)) (*NavigationResult, error) {
	ns, err := loadNode(addr)
	if err != nil {
		return nil, err
	}

	// Sort tasks lexicographically for depth-first hierarchical ordering.
	// This ensures task-0001.0001 comes before task-0002.
	sorted := make([]Task, len(ns.Tasks))
	copy(sorted, ns.Tasks)
	sort.Slice(sorted, func(i, j int) bool {
		return sorted[i].ID < sorted[j].ID
	})

	// Check whether all non-audit tasks are finished (complete or blocked).
	// For parent tasks with children, use derived status.
	nonAuditCount := 0
	nonAuditDone := 0
	nonAuditBlocked := 0
	for _, task := range sorted {
		if task.IsAudit {
			continue
		}
		// Skip child tasks in the count; parent status covers them
		if isChildTask(task.ID) && parentInList(task.ID, sorted) {
			continue
		}
		nonAuditCount++
		status := task.State
		if derived, hasChildren := DeriveParentStatus(ns, task.ID); hasChildren {
			status = derived
		}
		switch status {
		case StatusComplete:
			nonAuditDone++
		case StatusBlocked:
			nonAuditBlocked++
			nonAuditDone++
		}
	}
	allNonAuditDone := nonAuditDone == nonAuditCount
	if ns.Type == NodeLeaf && nonAuditCount == 0 {
		// Leaf with no real tasks yet. Don't run audit on empty leaves.
		allNonAuditDone = false
	}
	if ns.Type == NodeOrchestrator {
		// Orchestrator audits require all children to be complete,
		// and at least one child must exist (otherwise planning
		// hasn't run yet).
		allChildrenComplete := len(ns.Children) > 0
		for _, child := range ns.Children {
			if child.State != StatusComplete {
				allChildrenComplete = false
				break
			}
		}
		allNonAuditDone = allChildrenComplete
	}
	allNonAuditBlocked := nonAuditCount > 0 && nonAuditBlocked == nonAuditCount

	// Return in_progress tasks first (self-healing: resume after crash).
	for _, task := range sorted {
		if task.State == StatusInProgress {
			// Skip parent tasks that have children (their status is derived)
			if TaskChildren(ns, task.ID) {
				continue
			}
			return &NavigationResult{
				NodeAddress: addr,
				TaskID:      task.ID,
				Description: task.Description,
				Found:       true,
			}, nil
		}
	}

	// Then not_started leaf tasks in lexicographic (depth-first) order.
	for _, task := range sorted {
		if task.State != StatusNotStarted {
			continue
		}
		// Skip parent tasks that have incomplete children. Audit tasks
		// are the exception: when all remediation children complete,
		// the audit becomes actionable again for re-verification.
		if TaskChildren(ns, task.ID) {
			if !task.IsAudit || !allChildrenComplete(ns, task.ID) {
				continue
			}
		}
		// Skip child tasks whose parent is not_started
		if hasNotStartedAncestor(task.ID, ns) {
			continue
		}
		if task.IsAudit {
			if !allNonAuditDone {
				continue
			}
			if allNonAuditBlocked {
				continue
			}
		}
		return &NavigationResult{
			NodeAddress: addr,
			TaskID:      task.ID,
			Description: task.Description,
			Found:       true,
		}, nil
	}

	return nil, nil
}

// FindParallelTasks finds up to maxCount actionable tasks that can run concurrently.
// It starts with the same DFS as FindNextTask, then scans siblings of the first
// result's parent orchestrator for additional independent work. In-progress siblings
// are skipped (not treated as blockers), and an unplanned orchestrator sibling stops
// further scanning of later children.
func FindParallelTasks(
	idx *RootIndex,
	scopeAddr string,
	loadNode func(addr string) (*NodeState, error),
	maxCount int,
) ([]*NavigationResult, error) {
	first, err := FindNextTask(idx, scopeAddr, loadNode)
	if err != nil {
		return nil, err
	}
	if first == nil || !first.Found {
		return nil, nil
	}

	// Look up the node's index entry to find its parent.
	nodeEntry, ok := idx.Nodes[first.NodeAddress]
	if !ok {
		return []*NavigationResult{first}, nil
	}
	if nodeEntry.Parent == "" {
		return []*NavigationResult{first}, nil
	}

	parentEntry, ok := idx.Nodes[nodeEntry.Parent]
	if !ok {
		return []*NavigationResult{first}, nil
	}

	// Scan siblings in creation order (Children array order).
	var results []*NavigationResult
	for _, childAddr := range parentEntry.Children {
		if len(results) >= maxCount {
			break
		}

		childEntry, childOk := idx.Nodes[childAddr]
		if !childOk {
			continue
		}

		// Skip complete or blocked siblings.
		if childEntry.State == StatusComplete || childEntry.State == StatusBlocked {
			continue
		}

		// Unplanned orchestrator: stop scanning further siblings.
		if childEntry.Type == NodeOrchestrator && len(childEntry.Children) == 0 {
			break
		}

		// Skip in-progress siblings (already claimed by another worker).
		if childEntry.State == StatusInProgress {
			continue
		}

		result, loadErr := findActionableTask(childAddr, loadNode)
		if loadErr != nil {
			return nil, loadErr
		}
		if result != nil && result.Found {
			results = append(results, result)
		}
	}

	// Ensure the first result (from DFS) is always present. It may have
	// come from a deeper path (grandchild) that findActionableTask on
	// the sibling orchestrator didn't reach.
	found := false
	for _, r := range results {
		if r.NodeAddress == first.NodeAddress && r.TaskID == first.TaskID {
			found = true
			break
		}
	}
	if !found {
		// Prepend: the DFS result has highest priority.
		results = append([]*NavigationResult{first}, results...)
	}

	if len(results) > maxCount {
		results = results[:maxCount]
	}

	return results, nil
}

// isChildTask returns true if the task ID contains a dot (hierarchical child).
func isChildTask(id string) bool {
	return strings.Contains(id, ".")
}

// parentTaskID returns the parent portion of a hierarchical task ID.
// e.g., "task-0001.0002" → "task-0001"
func parentTaskID(childID string) string {
	dot := strings.LastIndex(childID, ".")
	if dot < 0 {
		return ""
	}
	return childID[:dot]
}

// parentInList checks if the immediate parent of a child task exists in the list.
func parentInList(childID string, tasks []Task) bool {
	dot := strings.LastIndex(childID, ".")
	if dot < 0 {
		return false
	}
	parentID := childID[:dot]
	for _, t := range tasks {
		if t.ID == parentID {
			return true
		}
	}
	return false
}

// hasNotStartedAncestor checks if any ancestor of the task is not_started.
// An ancestor is a task whose ID is a prefix of this task's ID.
// Audit ancestors are exempt: when an audit is reset to not_started for
// re-verification, its remediation children must still be runnable.
func hasNotStartedAncestor(taskID string, ns *NodeState) bool {
	// Walk up the hierarchy: task-0001.0002.0001 → task-0001.0002 → task-0001
	id := taskID
	for {
		dot := strings.LastIndex(id, ".")
		if dot < 0 {
			break
		}
		parentID := id[:dot]
		for _, t := range ns.Tasks {
			if t.ID == parentID && t.State == StatusNotStarted && !t.IsAudit {
				return true
			}
		}
		id = parentID
	}
	return false
}

// allChildrenComplete returns true if every immediate child of taskID
// is complete.
func allChildrenComplete(ns *NodeState, taskID string) bool {
	prefix := taskID + "."
	found := false
	for _, t := range ns.Tasks {
		if !strings.HasPrefix(t.ID, prefix) {
			continue
		}
		rest := t.ID[len(prefix):]
		if strings.Contains(rest, ".") {
			continue
		}
		found = true
		if t.State != StatusComplete {
			return false
		}
	}
	return found
}
