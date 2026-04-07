package app

import (
	"fmt"
	"testing"

	tea "charm.land/bubbletea/v2"

	"github.com/dorkusprime/wolfcastle/internal/instance"
	"github.com/dorkusprime/wolfcastle/internal/state"
	"github.com/dorkusprime/wolfcastle/internal/tui"
)

// keyMsg builds a tea.KeyPressMsg whose String() returns the given key string.
// For single printable characters, this is the character itself.
func keyMsg(s string) tea.Msg {
	if len(s) == 1 {
		return tea.KeyPressMsg{Code: rune(s[0]), Text: s}
	}
	// Multi-char keys like "ctrl+c" need special handling.
	return tea.KeyPressMsg{Text: s}
}

// toModel extracts the TUIModel from Update's tea.Model return value.
// TUIModel uses a value receiver on Update, so the returned interface
// wraps a TUIModel value (not a pointer).
func toModel(t *testing.T, m tea.Model) TUIModel {
	t.Helper()
	switch v := m.(type) {
	case TUIModel:
		return v
	case *TUIModel:
		return *v
	default:
		t.Fatalf("unexpected model type %T", m)
		return TUIModel{}
	}
}

// newColdModel returns a TUIModel in StateCold with a minimal store.
func newColdModel(t *testing.T) TUIModel {
	t.Helper()
	dir := t.TempDir()
	store := state.NewStore(dir, 0)
	m := NewTUIModel(store, nil, dir, "1.0.0")
	m.entryState = StateCold
	m.width = 120
	m.height = 40
	m.propagateSize()
	return m
}

// newLiveModel returns a TUIModel in StateLive.
func newLiveModel(t *testing.T) TUIModel {
	t.Helper()
	m := newColdModel(t)
	m.entryState = StateLive
	return m
}

func TestToggleDaemonStartsWhenCold(t *testing.T) {
	m := newColdModel(t)

	result, cmd := m.Update(keyMsg("s"))
	model := toModel(t, result)

	if !model.daemonStarting {
		t.Error("daemonStarting should be true after pressing s in StateCold")
	}
	if cmd == nil {
		t.Error("expected a command to start the daemon")
	}
}

func TestToggleDaemonStopsWhenLive(t *testing.T) {
	m := newLiveModel(t)
	// Give it a known instance so stopCurrentDaemon has a PID.
	m.instances = []instance.Entry{{PID: 99999, Worktree: m.worktreeDir, Branch: "main"}}
	m.activeInstanceIndex = 0

	result, cmd := m.Update(keyMsg("s"))
	model := toModel(t, result)

	if !model.daemonStopping {
		t.Error("daemonStopping should be true after pressing s in StateLive")
	}
	if cmd == nil {
		t.Error("expected a command to stop the daemon")
	}
}

func TestStopAllKey(t *testing.T) {
	m := newLiveModel(t)
	m.instances = []instance.Entry{
		{PID: 99998, Branch: "feat/a"},
		{PID: 99997, Branch: "feat/b"},
	}

	result, cmd := m.Update(keyMsg("S"))
	model := toModel(t, result)

	if !model.daemonStopping {
		t.Error("daemonStopping should be true after pressing S")
	}
	if cmd == nil {
		t.Error("expected a command to stop all daemons")
	}
}

func TestNextInstanceKey(t *testing.T) {
	m := newColdModel(t)
	m.instances = []instance.Entry{
		{PID: 100, Worktree: "/a", Branch: "feat/auth"},
		{PID: 200, Worktree: "/b", Branch: "fix/login"},
	}
	m.activeInstanceIndex = 0

	result, cmd := m.Update(keyMsg(">"))
	model := toModel(t, result)

	if model.activeInstanceIndex != 1 {
		t.Errorf("activeInstanceIndex = %d, want 1", model.activeInstanceIndex)
	}
	if cmd == nil {
		t.Error("expected a command for instance switch")
	}
}

func TestPrevInstanceKey(t *testing.T) {
	m := newColdModel(t)
	m.instances = []instance.Entry{
		{PID: 100, Worktree: "/a", Branch: "feat/auth"},
		{PID: 200, Worktree: "/b", Branch: "fix/login"},
	}
	m.activeInstanceIndex = 1

	result, cmd := m.Update(keyMsg("<"))
	model := toModel(t, result)

	if model.activeInstanceIndex != 0 {
		t.Errorf("activeInstanceIndex = %d, want 0", model.activeInstanceIndex)
	}
	if cmd == nil {
		t.Error("expected a command for instance switch")
	}
}

func TestNextInstanceWraps(t *testing.T) {
	m := newColdModel(t)
	m.instances = []instance.Entry{
		{PID: 100, Worktree: "/a", Branch: "feat/auth"},
		{PID: 200, Worktree: "/b", Branch: "fix/login"},
	}
	m.activeInstanceIndex = 1 // last index

	result, cmd := m.Update(keyMsg(">"))
	model := toModel(t, result)

	if model.activeInstanceIndex != 0 {
		t.Errorf("expected wrap to 0, got %d", model.activeInstanceIndex)
	}
	if cmd == nil {
		t.Error("expected a command for instance switch")
	}
}

func TestNumberKeySelectsInstance(t *testing.T) {
	m := newColdModel(t)
	m.instances = []instance.Entry{
		{PID: 100, Worktree: "/a", Branch: "feat/auth"},
		{PID: 200, Worktree: "/b", Branch: "fix/login"},
		{PID: 300, Worktree: "/c", Branch: "feat/third"},
	}
	m.activeInstanceIndex = 0

	// Press "2" to switch to instance index 1.
	result, cmd := m.Update(keyMsg("2"))
	model := toModel(t, result)

	if model.activeInstanceIndex != 1 {
		t.Errorf("activeInstanceIndex = %d, want 1", model.activeInstanceIndex)
	}
	if cmd == nil {
		t.Error("expected a command for instance switch")
	}
}

func TestNumberKeyOutOfRangeIgnored(t *testing.T) {
	m := newColdModel(t)
	m.instances = []instance.Entry{
		{PID: 100, Worktree: "/a", Branch: "feat/auth"},
	}
	m.activeInstanceIndex = 0

	// Press "5" when only 1 instance exists.
	result, cmd := m.Update(keyMsg("5"))
	model := toModel(t, result)

	if model.activeInstanceIndex != 0 {
		t.Errorf("activeInstanceIndex should remain 0, got %d", model.activeInstanceIndex)
	}
	// No command should be issued (though it may fall through to tree handling).
	// The key point is the instance index didn't change.
	_ = cmd
}

func TestDaemonStartedMsg(t *testing.T) {
	m := newColdModel(t)
	m.daemonStarting = true
	m.header.SetStatusHint("Starting daemon...")

	result, _ := m.Update(tui.DaemonStartedMsg{})
	model := toModel(t, result)

	if model.entryState != StateLive {
		t.Errorf("entryState = %d, want StateLive(%d)", model.entryState, StateLive)
	}
	if model.daemonStarting {
		t.Error("daemonStarting should be cleared")
	}
}

func TestDaemonStartFailedMsg(t *testing.T) {
	m := newColdModel(t)
	m.daemonStarting = true
	m.header.SetStatusHint("Starting daemon...")

	result, _ := m.Update(tui.DaemonStartFailedMsg{Err: fmt.Errorf("lock contention")})
	model := toModel(t, result)

	if model.daemonStarting {
		t.Error("daemonStarting should be cleared")
	}
	if len(model.errors) == 0 {
		t.Fatal("expected an error entry")
	}
	// "lock" in the error triggers the specific message about another daemon.
	if model.errors[0].message != "Another daemon is running in this worktree." {
		t.Errorf("unexpected error message: %q", model.errors[0].message)
	}
}

func TestDaemonStoppedMsg(t *testing.T) {
	m := newLiveModel(t)
	m.daemonStopping = true
	m.header.SetStatusHint("Stopping daemon...")

	result, _ := m.Update(tui.DaemonStoppedMsg{})
	model := toModel(t, result)

	if model.entryState != StateCold {
		t.Errorf("entryState = %d, want StateCold(%d)", model.entryState, StateCold)
	}
	if model.daemonStopping {
		t.Error("daemonStopping should be cleared")
	}
}

func TestDaemonStopFailedMsg(t *testing.T) {
	m := newLiveModel(t)
	m.daemonStopping = true
	m.header.SetStatusHint("Stopping daemon...")

	result, _ := m.Update(tui.DaemonStopFailedMsg{Err: fmt.Errorf("timeout")})
	model := toModel(t, result)

	if model.daemonStopping {
		t.Error("daemonStopping should be cleared")
	}
	if len(model.errors) == 0 {
		t.Fatal("expected an error entry")
	}
	// DaemonStopFailedMsg now passes the error through as-is.
	if model.errors[0].message != "timeout" {
		t.Errorf("error message = %q, want %q", model.errors[0].message, "timeout")
	}
}

func TestInstanceSwitchedMsg(t *testing.T) {
	m := newColdModel(t)
	m.instances = []instance.Entry{
		{PID: 100, Worktree: "/a", Branch: "feat/auth"},
		{PID: 200, Worktree: "/b", Branch: "fix/login"},
	}
	m.activeInstanceIndex = 1

	idx := &state.RootIndex{
		Nodes: map[string]state.IndexEntry{
			"root": {Name: "root", State: state.StatusNotStarted},
		},
	}

	result, _ := m.Update(tui.InstanceSwitchedMsg{
		Index: idx,
		Entry: instance.Entry{PID: 200, Worktree: "/b", Branch: "fix/login"},
	})
	model := toModel(t, result)

	// The tree should have been updated with the new index.
	treeIdx := model.tree.Index()
	if treeIdx == nil {
		t.Fatal("tree index should be set after InstanceSwitchedMsg")
	}
	if _, ok := treeIdx.Nodes["root"]; !ok {
		t.Error("tree index should contain 'root' node")
	}
}
