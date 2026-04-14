package app

import (
	"testing"

	"github.com/dorkusprime/wolfcastle/internal/state"
)

func TestNewTab_WelcomeStateWhenNoStore(t *testing.T) {
	t.Parallel()
	tab := newTab(1, "/tmp/demo", nil, nil)
	if tab.EntryState != StateWelcome {
		t.Errorf("expected StateWelcome with nil store, got %v", tab.EntryState)
	}
	if tab.Label != "demo" {
		t.Errorf("expected label 'demo', got %q", tab.Label)
	}
	if tab.Events == nil {
		t.Error("tab should have an event channel")
	}
	if !tab.TreeVisible {
		t.Error("tree should start visible")
	}
	if tab.Focused != PaneTree {
		t.Error("focused pane should default to tree")
	}
}

func TestNewTab_ColdStateWhenStorePresent(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	store := state.NewStore(dir, state.DefaultLockTimeout)
	tab := newTab(2, "/tmp/demo", store, nil)
	if tab.EntryState != StateCold {
		t.Errorf("expected StateCold with store, got %v", tab.EntryState)
	}
}

func TestTab_Reset_ClearsState(t *testing.T) {
	t.Parallel()
	tab := newTab(3, "/tmp/demo", nil, nil)

	// Seed state that Reset should clear.
	tab.PrevIndex = &state.RootIndex{}
	tab.PrevNodes["root"] = &state.NodeState{Name: "root"}
	tab.Errors = []errorEntry{{filename: "x.json", message: "boom"}}

	tab.Reset()

	if tab.PrevIndex != nil {
		t.Error("Reset should nil PrevIndex")
	}
	if len(tab.PrevNodes) != 0 {
		t.Errorf("Reset should empty PrevNodes, got %d entries", len(tab.PrevNodes))
	}
	if tab.Errors != nil {
		t.Errorf("Reset should nil Errors, got %+v", tab.Errors)
	}
}

func TestTab_Stop_NoopWhenNoWatcher(t *testing.T) {
	t.Parallel()
	tab := newTab(4, "/tmp/demo", nil, nil)
	// Should not panic despite nil watcher.
	tab.Stop()
}
