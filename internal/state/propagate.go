package state

import "fmt"

// Propagate updates a node's state, propagates to all ancestors, and updates
// the root index. This is the single authoritative mutation path — both CLI
// commands and the daemon must use it to stay ADR-024 compliant.
func Propagate(
	nodeAddr string,
	nodeState NodeStatus,
	idx *RootIndex,
	loadNode func(addr string) (*NodeState, error),
	saveNode func(addr string, ns *NodeState) error,
) error {
	// 1. Update the node's own entry in the root index
	if entry, ok := idx.Nodes[nodeAddr]; ok {
		entry.State = nodeState
		idx.Nodes[nodeAddr] = entry
	}

	// 2. Walk up through parents using PropagateUp
	getParentAddr := func(addr string) string {
		if entry, ok := idx.Nodes[addr]; ok {
			return entry.Parent
		}
		return ""
	}

	_, err := PropagateUp(
		nodeAddr,
		nodeState,
		loadNode,
		saveNode,
		getParentAddr,
	)
	if err != nil {
		return fmt.Errorf("propagating state: %w", err)
	}

	// 3. Re-walk ancestors to capture updated states in the root index
	current := nodeAddr
	for {
		entry, ok := idx.Nodes[current]
		if !ok {
			break
		}
		parentAddr := entry.Parent
		if parentAddr == "" {
			break
		}
		parentNS, err := loadNode(parentAddr)
		if err != nil {
			return fmt.Errorf("loading parent state for %q: %w", parentAddr, err)
		}
		if parentEntry, ok := idx.Nodes[parentAddr]; ok {
			parentEntry.State = parentNS.State
			idx.Nodes[parentAddr] = parentEntry
		}
		current = parentAddr
	}

	// 4. Update RootState if we have root entries
	if len(idx.Root) > 0 {
		if rootEntry, ok := idx.Nodes[idx.Root[0]]; ok {
			idx.RootState = rootEntry.State
		}
	}

	return nil
}
