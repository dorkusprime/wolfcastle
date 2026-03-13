package cmd

import (
	"fmt"

	"github.com/dorkusprime/wolfcastle/internal/state"
	"github.com/dorkusprime/wolfcastle/internal/tree"
)

// propagateState walks up from a node to the root, updating each parent's
// child ref state, recomputing the parent state, and updating the root index.
func propagateState(nodeAddr string, nodeState state.NodeStatus) error {
	idx, err := resolver.LoadRootIndex()
	if err != nil {
		return fmt.Errorf("loading root index: %w", err)
	}

	// Update the node's own entry in the root index
	if entry, ok := idx.Nodes[nodeAddr]; ok {
		entry.State = nodeState
		idx.Nodes[nodeAddr] = entry
	}

	// Walk up through parents using PropagateUp
	_, err = state.PropagateUp(
		nodeAddr,
		nodeState,
		func(addr string) (*state.NodeState, error) {
			a := tree.MustParse(addr)
			return state.LoadNodeState(resolver.NodeStatePath(a))
		},
		func(addr string, ns *state.NodeState) error {
			a := tree.MustParse(addr)
			return state.SaveNodeState(resolver.NodeStatePath(a), ns)
		},
		func(addr string) string {
			if entry, ok := idx.Nodes[addr]; ok {
				return entry.Parent
			}
			return ""
		},
	)
	if err != nil {
		return fmt.Errorf("propagating state: %w", err)
	}

	// Update root index entries for all ancestors that were touched
	// Re-walk to capture updated states
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
		a := tree.MustParse(parentAddr)
		parentNS, err := state.LoadNodeState(resolver.NodeStatePath(a))
		if err != nil {
			break
		}
		if parentEntry, ok := idx.Nodes[parentAddr]; ok {
			parentEntry.State = parentNS.State
			idx.Nodes[parentAddr] = parentEntry
		}
		current = parentAddr
	}

	// Save the root index once
	if err := state.SaveRootIndex(resolver.RootIndexPath(), idx); err != nil {
		return fmt.Errorf("saving root index: %w", err)
	}

	return nil
}
