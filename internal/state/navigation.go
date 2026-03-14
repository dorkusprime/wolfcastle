package state

import "sort"

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
			return &NavigationResult{Reason: "scope address not found"}, nil
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

	// Determine whether tree is all complete or has blocked nodes
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
		// Traverse children in order
		for _, childAddr := range entry.Children {
			result, err := dfs(idx, childAddr, loadNode)
			if err != nil {
				return nil, err
			}
			if result != nil && result.Found {
				return result, nil
			}
		}
		// Children exhausted — check orchestrator's own tasks (e.g. audit)
		return findActionableTask(addr, loadNode)
	}

	// Leaf node — find next actionable task
	return findActionableTask(addr, loadNode)
}

// findActionableTask loads a node's state and returns the first actionable task.
// It prefers in_progress tasks (self-healing) over not_started ones.
func findActionableTask(addr string, loadNode func(addr string) (*NodeState, error)) (*NavigationResult, error) {
	ns, err := loadNode(addr)
	if err != nil {
		return nil, err
	}

	// Return in_progress tasks first (self-healing: resume after crash)
	for _, task := range ns.Tasks {
		if task.State == StatusInProgress {
			return &NavigationResult{
				NodeAddress: addr,
				TaskID:      task.ID,
				Description: task.Description,
				Found:       true,
			}, nil
		}
	}
	// Then not_started tasks
	for _, task := range ns.Tasks {
		if task.State == StatusNotStarted {
			return &NavigationResult{
				NodeAddress: addr,
				TaskID:      task.ID,
				Description: task.Description,
				Found:       true,
			}, nil
		}
	}

	return nil, nil
}
