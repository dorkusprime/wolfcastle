package app

import (
	tea "charm.land/bubbletea/v2"

	"github.com/dorkusprime/wolfcastle/internal/daemon"
	"github.com/dorkusprime/wolfcastle/internal/instance"
	"github.com/dorkusprime/wolfcastle/internal/state"
)

// reconcileActiveInstance is the single reconciliation point for all
// instance-list changes. Every event that modifies m.instances (daemon
// status discovery, registry watcher updates, daemon start/stop) calls
// this method instead of doing its own ad-hoc index bookkeeping.
//
// It handles three cases:
//
//  1. The previously-active instance still exists in the new list but
//     may have moved to a different index (reordering). Update the
//     index, no switch needed.
//
//  2. The previously-active instance disappeared (stopped, stale entry
//     pruned). Pick the best replacement: CWD match first, then first
//     available. Trigger switchInstance to load data.
//
//  3. This is the first time we see instances (cold start). Same as
//     case 2.
//
// Returns any tea.Cmd that needs to be batched into the caller's
// response (typically a switchInstance command, or nil).
func (m *TUIModel) reconcileActiveInstance(newInstances []instance.Entry) tea.Cmd {
	// Snapshot what we were looking at.
	var prevPID int
	var prevWorktree string
	if m.activeInstanceIndex < len(m.instances) {
		prev := m.instances[m.activeInstanceIndex]
		prevPID = prev.PID
		prevWorktree = prev.Worktree
	}

	m.instances = newInstances

	if len(newInstances) == 0 {
		// All instances gone. Reset to CWD context so the tree shows
		// the local project rather than stale data from a dead process.
		// This covers both explicit stops (DaemonStoppedMsg) and
		// external kills detected by the poll tick.
		if m.worktreeDir != m.originalCWD {
			m.worktreeDir = m.originalCWD
			wolfDir := m.originalCWD + "/.wolfcastle"
			m.store = storeFromWolfcastleDir(wolfDir)
			m.daemonRepo = daemon.NewRepository(wolfDir)

			m.tree.Reset()
			m.detail.Reset()
			m.detail.SwitchToDashboard()
			m.prevIndex = nil
			m.prevNodes = make(map[string]*state.NodeState)

			m.stopAndDrainWatcher()
			m.watcher = newWatcherFor(m.store, m.daemonRepo, m.watcherEvents)
		}
		m.activeInstanceIndex = 0
		m.entryState = StateCold
		m.header.SetInstances(m.instances, 0)
		return m.handleRefresh()
	}
	m.header.SetInstances(m.instances, m.activeInstanceIndex)

	// Case 1: re-locate the active instance by PID+worktree.
	for i, inst := range newInstances {
		if inst.PID == prevPID && inst.Worktree == prevWorktree {
			m.activeInstanceIndex = i
			m.header.SetInstances(m.instances, m.activeInstanceIndex)
			return nil
		}
	}

	// Case 2/3: active instance gone or first discovery. Pick the best
	// replacement and switch to it. Use originalCWD for matching since
	// worktreeDir gets mutated by switchInstance.
	target := 0
	for i, inst := range newInstances {
		if inst.Worktree == m.originalCWD {
			target = i
			break
		}
	}

	m.activeInstanceIndex = target
	m.header.SetInstances(m.instances, m.activeInstanceIndex)

	if m.switching {
		return nil
	}
	return m.switchInstance(newInstances[target])
}
