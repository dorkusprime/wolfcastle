package app

import (
	"path/filepath"
	"testing"

	"github.com/dorkusprime/wolfcastle/internal/daemon"
	"github.com/dorkusprime/wolfcastle/internal/state"
	"github.com/dorkusprime/wolfcastle/internal/tui"
)

func TestDetectEntryState_NilWhenNoDaemonRepo(t *testing.T) {
	t.Parallel()
	m := newWelcomeModel(t.TempDir())
	if cmd := m.detectEntryState(); cmd != nil {
		t.Error("tab without DaemonRepo should produce nil cmd")
	}
}

func TestDetectEntryState_ReturnsStandingDownForColdTab(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	wolfDir := filepath.Join(dir, ".wolfcastle")

	m := NewTUIModel(dir, "test")
	m.tabs = nil
	m.nextTabID = 0
	tab := m.createTab(dir, state.NewStore(dir, 0), daemon.NewRepository(wolfDir))
	m.activeTabID = tab.ID
	m.width = 120
	m.height = 40
	m.propagateSize()

	cmd := m.detectEntryState()
	if cmd == nil {
		t.Fatal("expected a command when DaemonRepo is present")
	}
	msg := cmd()
	status, ok := msg.(tui.DaemonStatusMsg)
	if !ok {
		t.Fatalf("expected DaemonStatusMsg, got %T", msg)
	}
	if status.IsRunning {
		t.Errorf("cold tab should not report running, got %+v", status)
	}
	if status.Status != "standing down" {
		t.Errorf("expected 'standing down', got %q", status.Status)
	}
	if status.Worktree != dir {
		t.Errorf("expected worktree %q, got %q", dir, status.Worktree)
	}
}
