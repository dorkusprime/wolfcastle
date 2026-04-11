package app

import (
	"testing"

	"github.com/dorkusprime/wolfcastle/internal/instance"
)

func TestReconcile_RelocatesAfterReorder(t *testing.T) {
	m := newColdModel(t)
	m.instances = []instance.Entry{
		{PID: 1, Worktree: "/a", Branch: "a"},
		{PID: 2, Worktree: "/b", Branch: "b"},
	}
	m.activeInstanceIndex = 1 // watching /b

	// New list has them swapped.
	cmd := m.reconcileActiveInstance([]instance.Entry{
		{PID: 2, Worktree: "/b", Branch: "b"},
		{PID: 1, Worktree: "/a", Branch: "a"},
	})
	if cmd != nil {
		t.Error("reorder should not trigger a switch")
	}
	if m.activeInstanceIndex != 0 {
		t.Errorf("expected active index 0 after reorder, got %d", m.activeInstanceIndex)
	}
}

func TestReconcile_ActiveDisappeared_FallsToCWD(t *testing.T) {
	m := newColdModel(t)
	m.originalCWD = "/cwd"
	m.instances = []instance.Entry{
		{PID: 1, Worktree: "/other", Branch: "other"},
	}
	m.activeInstanceIndex = 0

	// The old instance is gone, a new one matching CWD appeared.
	cmd := m.reconcileActiveInstance([]instance.Entry{
		{PID: 99, Worktree: "/somewhere", Branch: "x"},
		{PID: 100, Worktree: "/cwd", Branch: "main"},
	})
	if cmd == nil {
		t.Error("should trigger switchInstance when active disappeared")
	}
	if m.activeInstanceIndex != 1 {
		t.Errorf("expected CWD match at index 1, got %d", m.activeInstanceIndex)
	}
}

func TestReconcile_ActiveDisappeared_FallsToFirst(t *testing.T) {
	m := newColdModel(t)
	m.worktreeDir = "/cwd-no-match"
	m.instances = []instance.Entry{
		{PID: 1, Worktree: "/old", Branch: "old"},
	}
	m.activeInstanceIndex = 0

	cmd := m.reconcileActiveInstance([]instance.Entry{
		{PID: 50, Worktree: "/alpha", Branch: "alpha"},
		{PID: 51, Worktree: "/beta", Branch: "beta"},
	})
	if cmd == nil {
		t.Error("should trigger switchInstance")
	}
	if m.activeInstanceIndex != 0 {
		t.Errorf("expected fallback to index 0, got %d", m.activeInstanceIndex)
	}
}

func TestReconcile_FirstDiscovery(t *testing.T) {
	m := newColdModel(t)
	// No previous instances.
	m.instances = nil
	m.activeInstanceIndex = 0

	cmd := m.reconcileActiveInstance([]instance.Entry{
		{PID: 10, Worktree: "/new", Branch: "feat"},
	})
	if cmd == nil {
		t.Error("first discovery should trigger switchInstance")
	}
	if m.activeInstanceIndex != 0 {
		t.Errorf("expected index 0, got %d", m.activeInstanceIndex)
	}
}

func TestReconcile_EmptyList_ResetsToOriginalCWD(t *testing.T) {
	m := newColdModel(t)
	m.originalCWD = m.worktreeDir
	m.instances = []instance.Entry{
		{PID: 1, Worktree: "/a", Branch: "a"},
	}
	m.worktreeDir = "/a" // simulates being switched to instance /a
	m.activeInstanceIndex = 0

	cmd := m.reconcileActiveInstance(nil)
	// Should return a handleRefresh cmd to reload CWD data.
	if cmd == nil {
		t.Error("empty list should trigger a CWD refresh")
	}
	if m.worktreeDir != m.originalCWD {
		t.Errorf("expected worktreeDir reset to %s, got %s", m.originalCWD, m.worktreeDir)
	}
	if m.entryState != StateCold {
		t.Error("expected StateCold after all instances gone")
	}
}

func TestReconcile_EmptyList_AlreadyAtCWD(t *testing.T) {
	m := newColdModel(t)
	m.originalCWD = m.worktreeDir
	// worktreeDir already matches originalCWD; the reset block should be skipped
	// but handleRefresh should still run.
	m.instances = []instance.Entry{
		{PID: 1, Worktree: m.worktreeDir, Branch: "main"},
	}
	m.activeInstanceIndex = 0

	cmd := m.reconcileActiveInstance(nil)
	if cmd == nil {
		t.Error("should return handleRefresh even when already at CWD")
	}
	if m.entryState != StateCold {
		t.Error("expected StateCold")
	}
}

func TestReconcile_SkipsDuringSwitching(t *testing.T) {
	m := newColdModel(t)
	m.switching = true
	m.instances = nil

	cmd := m.reconcileActiveInstance([]instance.Entry{
		{PID: 1, Worktree: "/a", Branch: "a"},
	})
	if cmd != nil {
		t.Error("should not trigger switch while already switching")
	}
}
