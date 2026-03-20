package daemon

import (
	"fmt"
	"path/filepath"

	"github.com/dorkusprime/wolfcastle/internal/state"
	"github.com/dorkusprime/wolfcastle/internal/tree"
)

// propagateState locks the namespace, re-reads the root index from disk,
// propagates a node's state to all ancestors, and saves everything
// atomically. Re-reading prevents the daemon from clobbering new nodes
// that CLI commands (called by the intake model) added during iteration.
// All reads and writes happen inside a single lock hold, so the
// load/save callbacks use raw I/O (no nested locking).
func (d *Daemon) propagateState(nodeAddr string, nodeState state.NodeStatus, idx *state.RootIndex) error {
	return d.Store.WithLock(func() error {
		// Re-read the index from disk to pick up concurrent modifications.
		freshIdx, err := state.LoadRootIndex(filepath.Join(d.Store.Dir(), "state.json"))
		if err != nil {
			// Fall back to the in-memory copy if the file can't be read.
			freshIdx = idx
		}

		// Update the node's state in the fresh index.
		if entry, ok := freshIdx.Nodes[nodeAddr]; ok {
			entry.State = nodeState
			freshIdx.Nodes[nodeAddr] = entry
		}

		loadNode := func(addr string) (*state.NodeState, error) {
			a, err := tree.ParseAddress(addr)
			if err != nil {
				return nil, fmt.Errorf("parsing address %q: %w", addr, err)
			}
			return state.LoadNodeState(filepath.Join(d.Store.Dir(), filepath.Join(a.Parts...), "state.json"))
		}

		// Raw SaveNodeState (no lock) since we already hold the namespace lock.
		saveNode := func(addr string, ns *state.NodeState) error {
			a, err := tree.ParseAddress(addr)
			if err != nil {
				return fmt.Errorf("parsing address %q: %w", addr, err)
			}
			return state.SaveNodeState(filepath.Join(d.Store.Dir(), filepath.Join(a.Parts...), "state.json"), ns)
		}

		if err := state.Propagate(nodeAddr, nodeState, freshIdx, loadNode, saveNode); err != nil {
			return err
		}

		// Copy propagated state back to the caller's index so subsequent
		// operations in the same iteration see the updated state.
		*idx = *freshIdx

		return state.SaveRootIndex(filepath.Join(d.Store.Dir(), "state.json"), freshIdx)
	})
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
