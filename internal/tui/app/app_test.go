package app

import (
	"fmt"
	"strings"
	"testing"

	tea "charm.land/bubbletea/v2"

	"github.com/dorkusprime/wolfcastle/internal/instance"
	"github.com/dorkusprime/wolfcastle/internal/state"
	"github.com/dorkusprime/wolfcastle/internal/tui"
	"github.com/dorkusprime/wolfcastle/internal/tui/detail"
	"github.com/dorkusprime/wolfcastle/internal/tui/notify"
	"github.com/dorkusprime/wolfcastle/internal/tui/tree"
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

// ---------------------------------------------------------------------------
// View / renderLayout
// ---------------------------------------------------------------------------

func TestViewReturnsAltScreen(t *testing.T) {
	m := newColdModel(t)
	v := m.View()
	if !v.AltScreen {
		t.Error("View should set AltScreen=true")
	}
	if v.WindowTitle != "WOLFCASTLE" {
		t.Errorf("WindowTitle = %q, want WOLFCASTLE", v.WindowTitle)
	}
}

func TestRenderLayoutTinyTerminal(t *testing.T) {
	m := newColdModel(t)
	m.width = 10
	m.height = 3
	m.propagateSize()
	out := m.renderLayout()
	if !strings.Contains(out, "too small") {
		t.Errorf("tiny terminal should show 'too small' message, got %q", out)
	}
}

func TestRenderLayoutNormalTerminal(t *testing.T) {
	m := newColdModel(t)
	out := m.renderLayout()
	if out == "" {
		t.Error("renderLayout should produce non-empty output")
	}
	// Should contain the header (version string) and footer.
	if !strings.Contains(out, "WOLFCASTLE") {
		t.Error("layout should contain header with WOLFCASTLE title")
	}
}

func TestRenderLayoutWelcomeState(t *testing.T) {
	m := NewTUIModel(nil, nil, "/tmp/test", "1.0.0")
	m.width = 120
	m.height = 40
	m.propagateSize()
	// Model created with nil store should be in welcome state.
	if m.entryState != StateWelcome {
		t.Fatalf("expected StateWelcome, got %d", m.entryState)
	}
	out := m.renderLayout()
	if out == "" {
		t.Error("welcome layout should not be empty")
	}
}

// ---------------------------------------------------------------------------
// renderContent
// ---------------------------------------------------------------------------

func TestRenderContentTreeHidden(t *testing.T) {
	m := newColdModel(t)
	m.treeVisible = false
	m.propagateSize()
	out := m.renderContent(30)
	// Should produce output (detail-only pane).
	if out == "" {
		t.Error("renderContent with tree hidden should not be empty")
	}
}

func TestRenderContentTreeVisible(t *testing.T) {
	m := newColdModel(t)
	m.treeVisible = true
	m.propagateSize()
	out := m.renderContent(30)
	if out == "" {
		t.Error("renderContent with tree visible should not be empty")
	}
}

func TestRenderContentNarrowForcesDetailOnly(t *testing.T) {
	m := newColdModel(t)
	m.treeVisible = true
	m.width = 50 // < 60 forces detail-only
	m.propagateSize()
	out := m.renderContent(20)
	if out == "" {
		t.Error("narrow renderContent should not be empty")
	}
}

// ---------------------------------------------------------------------------
// renderErrorBar
// ---------------------------------------------------------------------------

func TestRenderErrorBarEmpty(t *testing.T) {
	m := newColdModel(t)
	if bar := m.renderErrorBar(); bar != "" {
		t.Errorf("expected empty error bar, got %q", bar)
	}
}

func TestRenderErrorBarOneError(t *testing.T) {
	m := newColdModel(t)
	m.errors = []errorEntry{{filename: "state.json", message: "corrupt"}}
	bar := m.renderErrorBar()
	if !strings.Contains(bar, "state.json") || !strings.Contains(bar, "corrupt") {
		t.Errorf("error bar should contain filename and message: %q", bar)
	}
}

func TestRenderErrorBarThreeErrors(t *testing.T) {
	m := newColdModel(t)
	m.errors = []errorEntry{
		{filename: "a.json", message: "err1"},
		{filename: "b.json", message: "err2"},
		{filename: "c.json", message: "err3"},
	}
	bar := m.renderErrorBar()
	if !strings.Contains(bar, "a.json") || !strings.Contains(bar, "c.json") {
		t.Errorf("all three errors should appear: %q", bar)
	}
	if strings.Contains(bar, "more errors") {
		t.Error("3 errors should not show overflow")
	}
}

func TestRenderErrorBarOverflow(t *testing.T) {
	m := newColdModel(t)
	m.errors = []errorEntry{
		{filename: "a", message: "1"},
		{filename: "b", message: "2"},
		{filename: "c", message: "3"},
		{filename: "d", message: "4"},
		{filename: "e", message: "5"},
	}
	bar := m.renderErrorBar()
	if !strings.Contains(bar, "+2 more errors") {
		t.Errorf("5 errors should show '+2 more errors': %q", bar)
	}
	// Only first 3 should be shown.
	if strings.Contains(bar, "d:") || strings.Contains(bar, "e:") {
		t.Errorf("errors beyond maxShow should be hidden: %q", bar)
	}
}

// ---------------------------------------------------------------------------
// cycleFocus
// ---------------------------------------------------------------------------

func TestCycleFocusTreeToDetail(t *testing.T) {
	m := newColdModel(t)
	m.treeVisible = true
	m.focused = PaneTree
	m.cycleFocus()
	if m.focused != PaneDetail {
		t.Errorf("focused = %d, want PaneDetail(%d)", m.focused, PaneDetail)
	}
}

func TestCycleFocusDetailToTree(t *testing.T) {
	m := newColdModel(t)
	m.treeVisible = true
	m.focused = PaneDetail
	m.cycleFocus()
	if m.focused != PaneTree {
		t.Errorf("focused = %d, want PaneTree(%d)", m.focused, PaneTree)
	}
}

func TestCycleFocusLockedWhenTreeHidden(t *testing.T) {
	m := newColdModel(t)
	m.treeVisible = false
	m.focused = PaneDetail
	m.cycleFocus()
	// Should remain on PaneDetail since tree is hidden.
	if m.focused != PaneDetail {
		t.Errorf("focus should not cycle when tree hidden, got %d", m.focused)
	}
}

// ---------------------------------------------------------------------------
// borderStyle
// ---------------------------------------------------------------------------

func TestBorderStyleFocused(t *testing.T) {
	m := newColdModel(t)
	m.focused = PaneTree
	s := m.borderStyle(PaneTree)
	// FocusedBorderStyle is the one returned when pane matches focused.
	if s.GetBorderStyle() != tui.FocusedBorderStyle.GetBorderStyle() {
		t.Error("focused pane should use FocusedBorderStyle")
	}
}

func TestBorderStyleUnfocused(t *testing.T) {
	m := newColdModel(t)
	m.focused = PaneTree
	s := m.borderStyle(PaneDetail)
	if s.GetBorderStyle() != tui.UnfocusedBorderStyle.GetBorderStyle() {
		t.Error("unfocused pane should use UnfocusedBorderStyle")
	}
}

// ---------------------------------------------------------------------------
// handleCopy
// ---------------------------------------------------------------------------

func TestHandleCopyFromTree(t *testing.T) {
	m := newColdModel(t)
	m.focused = PaneTree
	// Tree has no selection, so copy should return nil.
	cmd := m.handleCopy()
	if cmd != nil {
		t.Error("copy with no tree selection should return nil")
	}
}

func TestHandleCopyFromDetailDashboard(t *testing.T) {
	m := newColdModel(t)
	m.focused = PaneDetail
	// Default mode is ModeDashboard, which goes to default branch (tree addr).
	cmd := m.handleCopy()
	// No selection in tree, so nil.
	if cmd != nil {
		t.Error("copy from dashboard with no selection should return nil")
	}
}

// ---------------------------------------------------------------------------
// loadDetailForSelection
// ---------------------------------------------------------------------------

func TestLoadDetailForSelectionNoSelection(t *testing.T) {
	m := newColdModel(t)
	// No index set, so SelectedRow returns nil.
	m.loadDetailForSelection()
	// Should not panic; detail should remain in dashboard mode.
	if m.detail.Mode() != detail.ModeDashboard {
		t.Errorf("detail mode = %d, want ModeDashboard", m.detail.Mode())
	}
}

func TestLoadDetailForSelectionWithNode(t *testing.T) {
	m := newColdModel(t)
	idx := &state.RootIndex{
		Root: []string{"alpha"},
		Nodes: map[string]state.IndexEntry{
			"alpha": {Name: "alpha", State: state.StatusInProgress, Type: state.NodeLeaf},
		},
	}
	m.tree.SetIndex(idx)
	m.tree.SetSize(40, 20)

	row := m.tree.SelectedRow()
	if row == nil {
		t.Fatal("expected a selected row after SetIndex with one node")
	}
	if row.Addr != "alpha" {
		t.Fatalf("selected addr = %q, want alpha", row.Addr)
	}

	m.loadDetailForSelection()
	if m.detail.Mode() == detail.ModeDashboard {
		t.Error("after loading a node, detail should leave ModeDashboard")
	}
}

// ---------------------------------------------------------------------------
// clearErrorsByFilename
// ---------------------------------------------------------------------------

func TestClearErrorsByFilename(t *testing.T) {
	m := newColdModel(t)
	m.errors = []errorEntry{
		{filename: "state.json", message: "corrupt"},
		{filename: "inbox.json", message: "bad"},
		{filename: "state.json", message: "another"},
	}
	m.clearErrorsByFilename("state.json")
	if len(m.errors) != 1 {
		t.Fatalf("expected 1 error remaining, got %d", len(m.errors))
	}
	if m.errors[0].filename != "inbox.json" {
		t.Errorf("remaining error should be inbox.json, got %q", m.errors[0].filename)
	}
}

func TestClearErrorsByFilenameNoMatch(t *testing.T) {
	m := newColdModel(t)
	m.errors = []errorEntry{
		{filename: "state.json", message: "x"},
	}
	m.clearErrorsByFilename("other.json")
	if len(m.errors) != 1 {
		t.Errorf("no errors should be removed, got %d", len(m.errors))
	}
}

func TestClearErrorsByFilenameAll(t *testing.T) {
	m := newColdModel(t)
	m.errors = []errorEntry{
		{filename: "f.json", message: "a"},
		{filename: "f.json", message: "b"},
	}
	m.clearErrorsByFilename("f.json")
	if len(m.errors) != 0 {
		t.Errorf("all errors should be cleared, got %d", len(m.errors))
	}
}

// ---------------------------------------------------------------------------
// Init
// ---------------------------------------------------------------------------

func TestInitReturnsCommand(t *testing.T) {
	m := newColdModel(t)
	cmd := m.Init()
	if cmd == nil {
		t.Error("Init should return a batch command")
	}
}

func TestInitNilStoreReturnsCommand(t *testing.T) {
	m := NewTUIModel(nil, nil, "/tmp/test", "1.0.0")
	m.width = 80
	m.height = 24
	cmd := m.Init()
	// Even with nil store, detectEntryState should produce a command.
	if cmd == nil {
		t.Error("Init with nil store should still return a command")
	}
}

// ---------------------------------------------------------------------------
// currentTarget
// ---------------------------------------------------------------------------

func TestCurrentTargetReturnsEmpty(t *testing.T) {
	m := newColdModel(t)
	if target := m.currentTarget(); target != "" {
		t.Errorf("currentTarget should return empty, got %q", target)
	}
}

// ---------------------------------------------------------------------------
// renderLayout with errors
// ---------------------------------------------------------------------------

func TestRenderLayoutWithErrors(t *testing.T) {
	m := newColdModel(t)
	m.errors = []errorEntry{
		{filename: "test.json", message: "something went wrong"},
	}
	out := m.renderLayout()
	if !strings.Contains(out, "test.json") {
		t.Errorf("layout with errors should contain error filename: %q", out)
	}
}

// ---------------------------------------------------------------------------
// WindowSizeMsg propagation
// ---------------------------------------------------------------------------

func TestWindowSizeMsgUpdatesModel(t *testing.T) {
	m := newColdModel(t)
	result, cmd := m.Update(tea.WindowSizeMsg{Width: 200, Height: 60})
	model := toModel(t, result)
	if model.width != 200 || model.height != 60 {
		t.Errorf("dimensions = %dx%d, want 200x60", model.width, model.height)
	}
	if cmd != nil {
		t.Error("WindowSizeMsg should return nil command")
	}
}

// ---------------------------------------------------------------------------
// Update: StateUpdatedMsg
// ---------------------------------------------------------------------------

func TestStateUpdatedMsgSetsTree(t *testing.T) {
	m := newColdModel(t)
	idx := &state.RootIndex{
		Root:  []string{"node-a", "node-b"},
		Nodes: map[string]state.IndexEntry{
			"node-a": {Name: "Node A", State: state.StatusComplete},
			"node-b": {Name: "Node B", State: state.StatusInProgress},
		},
	}
	result, cmd := m.Update(tui.StateUpdatedMsg{Index: idx})
	model := toModel(t, result)

	if model.tree.Index() == nil {
		t.Error("tree index should be set after StateUpdatedMsg")
	}
	_ = cmd // may be nil if sub-models return no commands
}

func TestStateUpdatedMsgClearsErrors(t *testing.T) {
	m := newColdModel(t)
	m.errors = []errorEntry{{filename: "state.json", message: "bad"}}
	result, _ := m.Update(tui.StateUpdatedMsg{Index: &state.RootIndex{
		Nodes: map[string]state.IndexEntry{},
	}})
	model := toModel(t, result)
	if len(model.errors) != 0 {
		t.Error("StateUpdatedMsg should clear state.json errors")
	}
}

func TestStateUpdatedMsgDiffDetectsNewNodes(t *testing.T) {
	m := newColdModel(t)
	// Set a previous index with one node.
	m.prevIndex = &state.RootIndex{
		Nodes: map[string]state.IndexEntry{
			"existing": {Name: "existing"},
		},
	}
	// New index adds a node.
	idx := &state.RootIndex{
		Root: []string{"existing", "brandnew"},
		Nodes: map[string]state.IndexEntry{
			"existing": {Name: "existing"},
			"brandnew": {Name: "brandnew"},
		},
	}
	result, cmd := m.Update(tui.StateUpdatedMsg{Index: idx})
	model := toModel(t, result)

	if model.prevIndex == nil {
		t.Error("prevIndex should be updated")
	}
	if cmd == nil {
		t.Error("expected commands including toast notification")
	}
}

// ---------------------------------------------------------------------------
// Update: DaemonStatusMsg
// ---------------------------------------------------------------------------

func TestDaemonStatusMsgRunning(t *testing.T) {
	m := newColdModel(t)
	m.instances = []instance.Entry{{PID: 100, Branch: "main"}}

	result, _ := m.Update(tui.DaemonStatusMsg{
		Status:    "running",
		IsRunning: true,
		PID:       100,
		Branch:    "main",
		Worktree:  "/test",
		Instances: []instance.Entry{{PID: 100, Branch: "main"}},
	})
	model := toModel(t, result)
	if model.entryState != StateLive {
		t.Errorf("entryState = %d, want StateLive", model.entryState)
	}
}

func TestDaemonStatusMsgNotRunning(t *testing.T) {
	m := newLiveModel(t)
	result, _ := m.Update(tui.DaemonStatusMsg{
		Status:    "stopped",
		IsRunning: false,
		Instances: nil,
	})
	model := toModel(t, result)
	if model.entryState != StateCold {
		t.Errorf("entryState = %d, want StateCold", model.entryState)
	}
}

// ---------------------------------------------------------------------------
// Update: NodeUpdatedMsg
// ---------------------------------------------------------------------------

func TestNodeUpdatedMsgCachesNode(t *testing.T) {
	m := newColdModel(t)
	ns := &state.NodeState{Name: "alpha", Type: state.NodeLeaf}
	result, _ := m.Update(tui.NodeUpdatedMsg{Address: "alpha", Node: ns})
	model := toModel(t, result)

	if model.prevNodes["alpha"] == nil {
		t.Error("prevNodes should cache the node after NodeUpdatedMsg")
	}
}

func TestNodeUpdatedMsgDiffToasts(t *testing.T) {
	m := newColdModel(t)
	// Set a previous node with a task in_progress.
	prevNs := &state.NodeState{
		Name: "alpha",
		Tasks: []state.Task{
			{ID: "t1", Title: "Do thing", State: state.StatusInProgress},
		},
	}
	m.prevNodes["alpha"] = prevNs

	// New node has the task complete.
	newNs := &state.NodeState{
		Name: "alpha",
		Tasks: []state.Task{
			{ID: "t1", Title: "Do thing", State: state.StatusComplete},
		},
	}
	result, cmd := m.Update(tui.NodeUpdatedMsg{Address: "alpha", Node: newNs})
	_ = toModel(t, result)
	if cmd == nil {
		t.Error("expected commands from diffNodeForToasts")
	}
}

// ---------------------------------------------------------------------------
// Update: ErrorMsg / ErrorClearedMsg
// ---------------------------------------------------------------------------

func TestErrorMsgAddsError(t *testing.T) {
	m := newColdModel(t)
	result, _ := m.Update(tui.ErrorMsg{Filename: "foo.json", Message: "broken"})
	model := toModel(t, result)
	if len(model.errors) != 1 {
		t.Fatalf("expected 1 error, got %d", len(model.errors))
	}
	if model.errors[0].message != "broken" {
		t.Errorf("error message = %q", model.errors[0].message)
	}
}

func TestErrorClearedMsgRemovesError(t *testing.T) {
	m := newColdModel(t)
	m.errors = []errorEntry{{filename: "foo.json", message: "bad"}}
	result, _ := m.Update(tui.ErrorClearedMsg{Filename: "foo.json"})
	model := toModel(t, result)
	if len(model.errors) != 0 {
		t.Errorf("expected 0 errors after clear, got %d", len(model.errors))
	}
}

// ---------------------------------------------------------------------------
// Update: CopiedMsg
// ---------------------------------------------------------------------------

func TestCopiedMsgForwardsToFooter(t *testing.T) {
	m := newColdModel(t)
	result, _ := m.Update(tui.CopiedMsg{})
	_ = toModel(t, result)
	// Should not panic.
}

// ---------------------------------------------------------------------------
// Update: SpinnerTickMsg
// ---------------------------------------------------------------------------

func TestSpinnerTickMsgAdvancesHeader(t *testing.T) {
	m := newColdModel(t)
	m.header.SetLoading(true)
	result, cmd := m.Update(tui.SpinnerTickMsg{})
	_ = toModel(t, result)
	// When loading, should schedule another tick.
	if cmd == nil {
		t.Error("SpinnerTickMsg while loading should schedule another tick")
	}
}

func TestSpinnerTickMsgNotLoading(t *testing.T) {
	m := newColdModel(t)
	m.header.SetLoading(false)
	result, cmd := m.Update(tui.SpinnerTickMsg{})
	_ = toModel(t, result)
	// When not loading, should NOT schedule another tick (though header cmd may be nil).
	_ = cmd // Just don't panic.
}

// ---------------------------------------------------------------------------
// Update: LogLinesMsg / NewLogFileMsg
// ---------------------------------------------------------------------------

func TestLogLinesMsgForwardsToDetail(t *testing.T) {
	m := newColdModel(t)
	result, _ := m.Update(tui.LogLinesMsg{Lines: []string{"line1", "line2"}})
	_ = toModel(t, result)
}

func TestNewLogFileMsgForwardsToDetail(t *testing.T) {
	m := newColdModel(t)
	result, _ := m.Update(tui.NewLogFileMsg{Path: "/tmp/wolf.log"})
	_ = toModel(t, result)
}

// ---------------------------------------------------------------------------
// Update: InboxUpdatedMsg
// ---------------------------------------------------------------------------

func TestInboxUpdatedMsgUpdatesDetail(t *testing.T) {
	m := newColdModel(t)
	inbox := &state.InboxFile{
		Items: []state.InboxItem{{Text: "hello", Status: state.InboxNew}},
	}
	result, _ := m.Update(tui.InboxUpdatedMsg{Inbox: inbox})
	_ = toModel(t, result)
}

// ---------------------------------------------------------------------------
// Update: AddInboxItemCmd / InboxItemAddedMsg / InboxAddFailedMsg
// ---------------------------------------------------------------------------

func TestAddInboxItemCmdReturnsCommand(t *testing.T) {
	m := newColdModel(t)
	result, cmd := m.Update(tui.AddInboxItemCmd{Text: "test note"})
	_ = toModel(t, result)
	if cmd == nil {
		t.Error("AddInboxItemCmd should produce a write command")
	}
}

func TestInboxItemAddedMsgReloadsInbox(t *testing.T) {
	m := newColdModel(t)
	result, cmd := m.Update(tui.InboxItemAddedMsg{})
	_ = toModel(t, result)
	if cmd == nil {
		t.Error("InboxItemAddedMsg should trigger a loadInbox command")
	}
}

func TestInboxAddFailedMsgAddsError(t *testing.T) {
	m := newColdModel(t)
	result, _ := m.Update(tui.InboxAddFailedMsg{Err: fmt.Errorf("disk full")})
	model := toModel(t, result)
	if len(model.errors) == 0 {
		t.Error("InboxAddFailedMsg should add an error")
	}
}

func TestInboxAddFailedMsgLockError(t *testing.T) {
	m := newColdModel(t)
	result, _ := m.Update(tui.InboxAddFailedMsg{Err: fmt.Errorf("lock contention")})
	model := toModel(t, result)
	if len(model.errors) == 0 {
		t.Fatal("expected an error")
	}
	if !strings.Contains(model.errors[0].message, "lock") {
		t.Errorf("lock error should mention lock: %q", model.errors[0].message)
	}
}

// ---------------------------------------------------------------------------
// Update: ToastMsg
// ---------------------------------------------------------------------------

func TestToastMsgPushesNotification(t *testing.T) {
	m := newColdModel(t)
	result, cmd := m.Update(tui.ToastMsg{Text: "hello"})
	_ = toModel(t, result)
	if cmd == nil {
		t.Error("ToastMsg should return a dismiss timer command")
	}
}

// ---------------------------------------------------------------------------
// Update: ToastDismissMsg
// ---------------------------------------------------------------------------

func TestToastDismissMsgUpdatesNotify(t *testing.T) {
	m := newColdModel(t)
	// Push a toast first so there's something to dismiss.
	m.notify.Push("test toast")
	result, _ := m.Update(notify.ToastDismissMsg{ID: 0})
	_ = toModel(t, result)
}

// ---------------------------------------------------------------------------
// Update: PollTickMsg
// ---------------------------------------------------------------------------

func TestPollTickMsgTriggersRefresh(t *testing.T) {
	m := newColdModel(t)
	result, cmd := m.Update(tui.PollTickMsg{})
	_ = toModel(t, result)
	if cmd == nil {
		t.Error("PollTickMsg should produce poll+detect+tick commands")
	}
}

// ---------------------------------------------------------------------------
// Update: WorktreeGoneMsg
// ---------------------------------------------------------------------------

func TestWorktreeGoneMsgRemovesInstance(t *testing.T) {
	m := newColdModel(t)
	m.instances = []instance.Entry{
		{PID: 100, Worktree: "/a", Branch: "feat/auth"},
		{PID: 200, Worktree: "/b", Branch: "fix/login"},
	}
	m.activeInstanceIndex = 1

	result, _ := m.Update(tui.WorktreeGoneMsg{Entry: instance.Entry{PID: 200, Worktree: "/b"}})
	model := toModel(t, result)

	if len(model.instances) != 1 {
		t.Errorf("instances = %d, want 1", len(model.instances))
	}
	if model.activeInstanceIndex >= len(model.instances) {
		t.Error("activeInstanceIndex should be clamped")
	}
	if len(model.errors) == 0 {
		t.Error("should add an error about the gone worktree")
	}
}

// ---------------------------------------------------------------------------
// Update: InstancesUpdatedMsg
// ---------------------------------------------------------------------------

func TestInstancesUpdatedMsg(t *testing.T) {
	m := newColdModel(t)
	entries := []instance.Entry{
		{PID: 1, Branch: "a"},
		{PID: 2, Branch: "b"},
		{PID: 3, Branch: "c"},
	}
	result, _ := m.Update(tui.InstancesUpdatedMsg{Instances: entries})
	model := toModel(t, result)
	if len(model.instances) != 3 {
		t.Errorf("instances = %d, want 3", len(model.instances))
	}
}

// ---------------------------------------------------------------------------
// Update: LoadNodeMsg (tree fires when expanding a leaf)
// ---------------------------------------------------------------------------

func TestLoadNodeMsgReadsFromStore(t *testing.T) {
	m := newColdModel(t)
	result, cmd := m.Update(tree.LoadNodeMsg{Address: "alpha"})
	_ = toModel(t, result)
	if cmd == nil {
		t.Error("LoadNodeMsg with nil Node should produce a read command")
	}
}

func TestLoadNodeMsgWithNode(t *testing.T) {
	m := newColdModel(t)
	ns := &state.NodeState{Name: "alpha"}
	result, _ := m.Update(tree.LoadNodeMsg{Address: "alpha", Node: ns})
	_ = toModel(t, result)
}

// ---------------------------------------------------------------------------
// Update: key bindings
// ---------------------------------------------------------------------------

func TestQuitKey(t *testing.T) {
	m := newColdModel(t)
	_, cmd := m.Update(keyMsg("q"))
	// tea.Quit returns a special command; we verify cmd is non-nil.
	if cmd == nil {
		t.Error("q should trigger quit command")
	}
}

func TestDashboardKey(t *testing.T) {
	m := newColdModel(t)
	// First switch away from dashboard.
	m.detail.SwitchToLogView()
	if m.detail.Mode() == detail.ModeDashboard {
		t.Fatal("should not be in dashboard after SwitchToLogView")
	}
	result, _ := m.Update(keyMsg("d"))
	model := toModel(t, result)
	if model.detail.Mode() != detail.ModeDashboard {
		t.Errorf("d key should switch to dashboard, mode = %d", model.detail.Mode())
	}
}

func TestToggleTreeKey(t *testing.T) {
	m := newColdModel(t)
	if !m.treeVisible {
		t.Fatal("tree should start visible")
	}
	result, _ := m.Update(keyMsg("t"))
	model := toModel(t, result)
	if model.treeVisible {
		t.Error("t key should toggle tree hidden")
	}

	// Toggle back.
	result2, _ := model.Update(keyMsg("t"))
	model2 := toModel(t, result2)
	if !model2.treeVisible {
		t.Error("t key should toggle tree visible again")
	}
}

func TestCycleFocusKey(t *testing.T) {
	m := newColdModel(t)
	m.treeVisible = true
	m.focused = PaneTree
	result, _ := m.Update(keyMsg("tab"))
	model := toModel(t, result)
	if model.focused != PaneDetail {
		t.Errorf("tab should cycle focus to PaneDetail, got %d", model.focused)
	}
}

func TestEscClearsErrors(t *testing.T) {
	m := newColdModel(t)
	m.errors = []errorEntry{{filename: "x", message: "y"}}
	result, _ := m.Update(keyMsg("esc"))
	model := toModel(t, result)
	if len(model.errors) != 0 {
		t.Errorf("esc should clear errors, got %d", len(model.errors))
	}
}

func TestEscReturnsToDashboard(t *testing.T) {
	m := newColdModel(t)
	m.focused = PaneDetail
	m.detail.SwitchToLogView()
	result, _ := m.Update(keyMsg("esc"))
	model := toModel(t, result)
	if model.detail.Mode() != detail.ModeDashboard {
		t.Errorf("esc in detail non-dashboard should return to dashboard, mode = %d", model.detail.Mode())
	}
}

func TestRefreshKey(t *testing.T) {
	m := newColdModel(t)
	result, cmd := m.Update(keyMsg("R"))
	model := toModel(t, result)
	if !model.header.IsLoading() {
		t.Error("r key should set loading")
	}
	if cmd == nil {
		t.Error("r key should produce refresh commands")
	}
}

func TestHelpToggle(t *testing.T) {
	m := newColdModel(t)
	result, _ := m.Update(keyMsg("?"))
	model := toModel(t, result)
	if !model.help.IsActive() {
		t.Error("? should activate help overlay")
	}
	// Dismiss help.
	result2, _ := model.Update(keyMsg("?"))
	model2 := toModel(t, result2)
	if model2.help.IsActive() {
		t.Error("? again should dismiss help overlay")
	}
}

func TestSearchActivation(t *testing.T) {
	m := newColdModel(t)
	result, _ := m.Update(keyMsg("/"))
	model := toModel(t, result)
	if !model.search.IsActive() {
		t.Error("/ should activate search")
	}
}

// ---------------------------------------------------------------------------
// Update: DaemonStartFailedMsg (generic error)
// ---------------------------------------------------------------------------

func TestDaemonStartFailedGenericError(t *testing.T) {
	m := newColdModel(t)
	m.daemonStarting = true
	result, _ := m.Update(tui.DaemonStartFailedMsg{Err: fmt.Errorf("something weird")})
	model := toModel(t, result)
	if len(model.errors) == 0 {
		t.Fatal("expected an error")
	}
	if !strings.Contains(model.errors[0].message, "Daemon failed to start") {
		t.Errorf("generic error should say 'Daemon failed to start': %q", model.errors[0].message)
	}
}

func TestDaemonStartFailedNotFoundError(t *testing.T) {
	m := newColdModel(t)
	m.daemonStarting = true
	result, _ := m.Update(tui.DaemonStartFailedMsg{Err: fmt.Errorf("config not found")})
	model := toModel(t, result)
	if len(model.errors) == 0 {
		t.Fatal("expected an error")
	}
	if !strings.Contains(model.errors[0].message, "No project found") {
		t.Errorf("not-found error should mention init: %q", model.errors[0].message)
	}
}

// ---------------------------------------------------------------------------
// diffNodeForToasts
// ---------------------------------------------------------------------------

func TestDiffNodeForToastsTaskBlocked(t *testing.T) {
	m := newColdModel(t)
	old := &state.NodeState{
		Tasks: []state.Task{{ID: "t1", State: state.StatusInProgress}},
	}
	new := &state.NodeState{
		Tasks: []state.Task{{ID: "t1", State: state.StatusBlocked}},
	}
	cmds := m.diffNodeForToasts("node-a", old, new)
	if len(cmds) == 0 {
		t.Error("task becoming blocked should produce a toast command")
	}
}

func TestDiffNodeForToastsNewGaps(t *testing.T) {
	m := newColdModel(t)
	old := &state.NodeState{
		Audit: state.AuditState{},
	}
	new := &state.NodeState{
		Audit: state.AuditState{
			Gaps: []state.Gap{
				{ID: "g1", Status: state.GapOpen, Description: "missing coverage"},
			},
		},
	}
	cmds := m.diffNodeForToasts("node-a", old, new)
	if len(cmds) == 0 {
		t.Error("new gap should produce a toast command")
	}
}

func TestDiffNodeForToastsNoChange(t *testing.T) {
	m := newColdModel(t)
	ns := &state.NodeState{
		Tasks: []state.Task{{ID: "t1", State: state.StatusInProgress}},
	}
	cmds := m.diffNodeForToasts("node-a", ns, ns)
	if len(cmds) != 0 {
		t.Errorf("identical nodes should produce 0 toast commands, got %d", len(cmds))
	}
}

// ---------------------------------------------------------------------------
// overlayToasts
// ---------------------------------------------------------------------------

func TestOverlayToastsEmpty(t *testing.T) {
	m := newColdModel(t)
	content := "line1\nline2\nline3"
	result := m.overlayToasts(content, 80)
	// No toasts, so content should be unchanged.
	if result != content {
		t.Errorf("overlayToasts with no toasts should return content unchanged")
	}
}

// ---------------------------------------------------------------------------
// addInboxItem with nil store
// ---------------------------------------------------------------------------

func TestAddInboxItemNilStore(t *testing.T) {
	m := NewTUIModel(nil, nil, "/tmp/test", "1.0.0")
	m.width = 80
	m.height = 24
	cmd := m.addInboxItem("hello")
	if cmd != nil {
		t.Error("addInboxItem with nil store should return nil")
	}
}

// ---------------------------------------------------------------------------
// loadInbox with nil store
// ---------------------------------------------------------------------------

func TestLoadInboxNilStore(t *testing.T) {
	m := NewTUIModel(nil, nil, "/tmp/test", "1.0.0")
	cmd := m.loadInbox()
	if cmd != nil {
		t.Error("loadInbox with nil store should return nil")
	}
}

// ---------------------------------------------------------------------------
// handleRefresh
// ---------------------------------------------------------------------------

func TestHandleRefreshWithStore(t *testing.T) {
	m := newColdModel(t)
	cmd := m.handleRefresh()
	if cmd == nil {
		t.Error("handleRefresh with store should return a command")
	}
}

func TestHandleRefreshNilStore(t *testing.T) {
	m := NewTUIModel(nil, nil, "/tmp/test", "1.0.0")
	m.width = 80
	m.height = 24
	cmd := m.handleRefresh()
	if cmd != nil {
		t.Error("handleRefresh with nil store and nil daemonRepo should return nil")
	}
}
