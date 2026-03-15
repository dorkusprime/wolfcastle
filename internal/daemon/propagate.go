package daemon

import (
	"fmt"
	"path/filepath"

	"github.com/dorkusprime/wolfcastle/internal/state"
	"github.com/dorkusprime/wolfcastle/internal/tree"
)

// propagateState propagates a node's state to all ancestors and updates the
// root index. This ensures ADR-024 compliance for all daemon mutations.
func (d *Daemon) propagateState(nodeAddr string, nodeState state.NodeStatus, idx *state.RootIndex) error {
	loadNode := func(addr string) (*state.NodeState, error) {
		a, err := tree.ParseAddress(addr)
		if err != nil {
			return nil, fmt.Errorf("parsing address %q: %w", addr, err)
		}
		return state.LoadNodeState(filepath.Join(d.Resolver.ProjectsDir(), filepath.Join(a.Parts...), "state.json"))
	}

	saveNode := func(addr string, ns *state.NodeState) error {
		a, err := tree.ParseAddress(addr)
		if err != nil {
			return fmt.Errorf("parsing address %q: %w", addr, err)
		}
		return state.SaveNodeState(filepath.Join(d.Resolver.ProjectsDir(), filepath.Join(a.Parts...), "state.json"), ns)
	}

	if err := state.Propagate(nodeAddr, nodeState, idx, loadNode, saveNode); err != nil {
		return err
	}

	return state.SaveRootIndex(d.Resolver.RootIndexPath(), idx)
}

// checkInboxState returns whether the inbox has new items (needing intake).
// Returns false if the inbox file doesn't exist or can't be read.
func (d *Daemon) checkInboxState(inboxPath string) (hasNew bool) {
	inboxData, err := state.LoadInbox(inboxPath)
	if err != nil {
		return false
	}
	for _, item := range inboxData.Items {
		if item.Status == "new" {
			hasNew = true
		}
	}
	return
}
