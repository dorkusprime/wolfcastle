package daemon

import (
	"fmt"

	"github.com/dorkusprime/wolfcastle/internal/logging"
	"github.com/dorkusprime/wolfcastle/internal/state"
)

// propagateState locks the namespace, re-reads the root index from disk,
// propagates a node's state to all ancestors, and saves everything
// atomically. Re-reading prevents the daemon from clobbering new nodes
// that CLI commands (called by the intake model) added during iteration.
// All reads and writes happen inside a single lock hold, so the
// load/save callbacks use raw I/O (no nested locking).
//
// lg is the logger for the fallback-diagnostic record emitted when the
// index can't be re-read. Callers pass the worker's child logger in
// parallel mode and d.Logger in the sequential path.
func (d *Daemon) propagateState(lg *logging.Logger, nodeAddr string, nodeState state.NodeStatus, idx *state.RootIndex) error {
	return d.Store.WithLock(func() error {
		// Re-read the index from disk to pick up concurrent modifications.
		freshIdx, err := state.LoadRootIndex(d.Store.IndexPath())
		if err != nil {
			// Fall back to the in-memory copy if the file can't be read.
			_ = lg.Log(map[string]any{
				"type":  "propagate_index_read_fallback",
				"node":  nodeAddr,
				"error": err.Error(),
			})
			freshIdx = idx
		}

		// Update the node's state in the fresh index.
		if entry, ok := freshIdx.Nodes[nodeAddr]; ok {
			entry.State = nodeState
			freshIdx.Nodes[nodeAddr] = entry
		}

		loadNode := func(addr string) (*state.NodeState, error) {
			p, err := d.Store.NodePath(addr)
			if err != nil {
				return nil, fmt.Errorf("resolving path for %q: %w", addr, err)
			}
			return state.LoadNodeState(p)
		}

		// Raw SaveNodeState (no lock) since we already hold the namespace lock.
		saveNode := func(addr string, ns *state.NodeState) error {
			p, err := d.Store.NodePath(addr)
			if err != nil {
				return fmt.Errorf("resolving path for %q: %w", addr, err)
			}
			return state.SaveNodeState(p, ns)
		}

		if err := state.Propagate(nodeAddr, nodeState, freshIdx, loadNode, saveNode); err != nil {
			return err
		}

		// Copy propagated state back to the caller's index so subsequent
		// operations in the same iteration see the updated state.
		*idx = *freshIdx

		return state.SaveRootIndex(d.Store.IndexPath(), freshIdx)
	})
}
