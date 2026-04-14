package app

import (
	"os"
	"testing"

	"github.com/dorkusprime/wolfcastle/internal/instance"
)

// Each scenario exercises one branch of updateTabsFromInstances: a live
// match with the real process, a stale match where the PID is dead, and
// a tab whose worktree is missing from the instance list entirely.

func TestUpdateTabsFromInstances_LivePromotesTab(t *testing.T) {
	t.Parallel()
	m := newColdModel(t)
	dir := m.activeTab().WorktreeDir
	m.activeTab().EntryState = StateCold

	m.updateTabsFromInstances([]instance.Entry{
		// Use the current process pid so isProcessAlive returns true.
		{PID: os.Getpid(), Worktree: dir},
	})

	if m.activeTab().EntryState != StateLive {
		t.Errorf("expected StateLive after match, got %v", m.activeTab().EntryState)
	}
}

func TestUpdateTabsFromInstances_DeadPidDemotesLive(t *testing.T) {
	t.Parallel()
	m := newColdModel(t)
	dir := m.activeTab().WorktreeDir
	m.activeTab().EntryState = StateLive

	// Pid 1 << 30 is effectively guaranteed to be dead on test hosts.
	m.updateTabsFromInstances([]instance.Entry{
		{PID: 1 << 30, Worktree: dir},
	})

	if m.activeTab().EntryState != StateCold {
		t.Errorf("stale instance should demote StateLive to StateCold, got %v",
			m.activeTab().EntryState)
	}
}

func TestUpdateTabsFromInstances_MissingInstanceDemotesLive(t *testing.T) {
	t.Parallel()
	m := newColdModel(t)
	m.activeTab().EntryState = StateLive

	m.updateTabsFromInstances(nil)

	if m.activeTab().EntryState != StateCold {
		t.Errorf("missing instance should demote StateLive to StateCold, got %v",
			m.activeTab().EntryState)
	}
}

func TestUpdateTabsFromInstances_MissingInstanceLeavesColdAlone(t *testing.T) {
	t.Parallel()
	m := newColdModel(t)
	m.activeTab().EntryState = StateCold

	m.updateTabsFromInstances(nil)

	if m.activeTab().EntryState != StateCold {
		t.Errorf("cold tab should stay cold when absent from instances, got %v",
			m.activeTab().EntryState)
	}
}
