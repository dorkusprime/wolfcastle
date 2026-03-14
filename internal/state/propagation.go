package state

import "fmt"

// RecomputeState derives an orchestrator's state from its children.
func RecomputeState(children []ChildRef) NodeStatus {
	if len(children) == 0 {
		return StatusNotStarted
	}

	allNotStarted := true
	allComplete := true
	anyBlocked := false

	for _, c := range children {
		if c.State != StatusNotStarted {
			allNotStarted = false
		}
		if c.State != StatusComplete {
			allComplete = false
		}
		if c.State == StatusBlocked {
			anyBlocked = true
		}
	}

	if allNotStarted {
		return StatusNotStarted
	}
	if allComplete {
		return StatusComplete
	}

	// All non-complete are blocked => blocked
	if anyBlocked {
		allNonCompleteBlocked := true
		for _, c := range children {
			if c.State != StatusComplete && c.State != StatusBlocked {
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

// maxPropagationDepth guards against cycles in the parent chain.
const maxPropagationDepth = 100

// PropagateUp updates parent states up to the root.
// It takes a function that loads and saves parent state given an address.
// Returns the list of addresses that were updated.
func PropagateUp(
	childAddr string,
	childState NodeStatus,
	loadParent func(addr string) (*NodeState, error),
	saveParent func(addr string, ns *NodeState) error,
	getParentAddr func(addr string) string,
) ([]string, error) {
	var updated []string
	current := childAddr
	currentState := childState
	visited := make(map[string]bool)

	for {
		parentAddr := getParentAddr(current)
		if parentAddr == "" {
			break
		}

		if visited[parentAddr] {
			return updated, fmt.Errorf("cycle detected in parent chain at %q", parentAddr)
		}
		visited[parentAddr] = true

		if len(visited) > maxPropagationDepth {
			return updated, fmt.Errorf("parent chain exceeds maximum depth (%d) — possible cycle", maxPropagationDepth)
		}

		parent, err := loadParent(parentAddr)
		if err != nil {
			return updated, err
		}

		// Update the child reference in parent
		for i := range parent.Children {
			if parent.Children[i].Address == current {
				parent.Children[i].State = currentState
				break
			}
		}

		newState := RecomputeState(parent.Children)
		parent.State = newState

		if err := saveParent(parentAddr, parent); err != nil {
			return updated, err
		}
		updated = append(updated, parentAddr)

		current = parentAddr
		currentState = newState
	}

	return updated, nil
}
