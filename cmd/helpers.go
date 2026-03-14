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

	loadNode := func(addr string) (*state.NodeState, error) {
		a, err := tree.ParseAddress(addr)
		if err != nil {
			return nil, fmt.Errorf("parsing address %q: %w", addr, err)
		}
		return state.LoadNodeState(resolver.NodeStatePath(a))
	}

	saveNode := func(addr string, ns *state.NodeState) error {
		a, err := tree.ParseAddress(addr)
		if err != nil {
			return fmt.Errorf("parsing address %q: %w", addr, err)
		}
		return state.SaveNodeState(resolver.NodeStatePath(a), ns)
	}

	if err := state.Propagate(nodeAddr, nodeState, idx, loadNode, saveNode); err != nil {
		return err
	}

	// Save the root index once
	if err := state.SaveRootIndex(resolver.RootIndexPath(), idx); err != nil {
		return fmt.Errorf("saving root index: %w", err)
	}

	return nil
}
