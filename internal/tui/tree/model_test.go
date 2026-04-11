package tree

import (
	"testing"
	"time"

	tea "charm.land/bubbletea/v2"

	"github.com/dorkusprime/wolfcastle/internal/state"
)

// keyPress builds a tea.KeyPressMsg for the given printable rune.
func keyPress(r rune) tea.KeyPressMsg {
	return tea.KeyPressMsg{Code: r, Text: string(r)}
}

// specialKey builds a tea.KeyPressMsg for a special (non-printable) key.
func specialKey(code rune) tea.KeyPressMsg {
	return tea.KeyPressMsg{Code: code}
}

func TestNewModel(t *testing.T) {
	m := NewModel()

	if m.nodes == nil {
		t.Error("nodes map should be initialized")
	}
	if m.cacheExpiry == nil {
		t.Error("cacheExpiry map should be initialized")
	}
	if m.expanded == nil {
		t.Error("expanded map should be initialized")
	}
	if len(m.flatList) != 0 {
		t.Errorf("flatList should be empty, got %d entries", len(m.flatList))
	}
}

func simpleIndex() *state.RootIndex {
	return &state.RootIndex{
		Root: []string{"alpha", "beta", "gamma"},
		Nodes: map[string]state.IndexEntry{
			"alpha": {Name: "Alpha", Type: state.NodeLeaf, State: state.StatusNotStarted, DecompositionDepth: 0},
			"beta":  {Name: "Beta", Type: state.NodeLeaf, State: state.StatusInProgress, DecompositionDepth: 0},
			"gamma": {Name: "Gamma", Type: state.NodeLeaf, State: state.StatusComplete, DecompositionDepth: 0},
		},
	}
}

func orchestratorIndex() *state.RootIndex {
	return &state.RootIndex{
		Root: []string{"parent"},
		Nodes: map[string]state.IndexEntry{
			"parent": {
				Name:               "Parent",
				Type:               state.NodeOrchestrator,
				State:              state.StatusInProgress,
				DecompositionDepth: 0,
				Children:           []string{"child-a", "child-b"},
			},
			"child-a": {
				Name:               "Child A",
				Type:               state.NodeLeaf,
				State:              state.StatusComplete,
				DecompositionDepth: 1,
				Parent:             "parent",
			},
			"child-b": {
				Name:               "Child B",
				Type:               state.NodeLeaf,
				State:              state.StatusNotStarted,
				DecompositionDepth: 1,
				Parent:             "parent",
			},
		},
	}
}

func TestSetIndex_SimpleFlatList(t *testing.T) {
	m := NewModel()
	m.SetIndex(simpleIndex())

	if len(m.flatList) != 3 {
		t.Fatalf("flatList length = %d, want 3", len(m.flatList))
	}

	names := []string{"Alpha", "Beta", "Gamma"}
	for i, want := range names {
		if m.flatList[i].Name != want {
			t.Errorf("flatList[%d].Name = %q, want %q", i, m.flatList[i].Name, want)
		}
	}
}

func TestSetIndex_OrchestratorCollapsed(t *testing.T) {
	m := NewModel()
	m.SetIndex(orchestratorIndex())

	// Orchestrator collapsed: only the parent should appear.
	if len(m.flatList) != 1 {
		t.Fatalf("flatList length = %d, want 1 (children hidden)", len(m.flatList))
	}
	if !m.flatList[0].Expandable {
		t.Error("orchestrator row should be expandable")
	}
	if m.flatList[0].IsExpanded {
		t.Error("orchestrator should not be expanded initially")
	}
}

func TestSetIndex_OrchestratorExpanded(t *testing.T) {
	m := NewModel()
	m.expanded["parent"] = true
	m.SetIndex(orchestratorIndex())

	if len(m.flatList) != 3 {
		t.Fatalf("flatList length = %d, want 3 (parent + 2 children)", len(m.flatList))
	}
	if m.flatList[0].Depth != 0 {
		t.Errorf("parent depth = %d, want 0", m.flatList[0].Depth)
	}
	if m.flatList[1].Depth != 1 {
		t.Errorf("child-a depth = %d, want 1", m.flatList[1].Depth)
	}
}

func TestCursorDown(t *testing.T) {
	m := NewModel()
	m.SetFocused(true)
	m.SetIndex(simpleIndex())
	m.SetSize(80, 20)

	m, _ = m.Update(keyPress('j'))
	if m.cursor != 1 {
		t.Errorf("cursor = %d, want 1 after j", m.cursor)
	}
}

func TestCursorDown_ClampsAtBottom(t *testing.T) {
	m := NewModel()
	m.SetFocused(true)
	m.SetIndex(simpleIndex())
	m.SetSize(80, 20)

	// Press j enough times to reach the end and beyond.
	for i := 0; i < 10; i++ {
		m, _ = m.Update(keyPress('j'))
	}
	if m.cursor != 2 {
		t.Errorf("cursor = %d, want 2 (clamped at last item)", m.cursor)
	}
}

func TestCursorUp(t *testing.T) {
	m := NewModel()
	m.SetFocused(true)
	m.SetIndex(simpleIndex())
	m.SetSize(80, 20)
	m.cursor = 2

	m, _ = m.Update(keyPress('k'))
	if m.cursor != 1 {
		t.Errorf("cursor = %d, want 1 after k", m.cursor)
	}
}

func TestCursorUp_ClampsAt0(t *testing.T) {
	m := NewModel()
	m.SetFocused(true)
	m.SetIndex(simpleIndex())
	m.SetSize(80, 20)

	m, _ = m.Update(keyPress('k'))
	if m.cursor != 0 {
		t.Errorf("cursor = %d, want 0 (clamped)", m.cursor)
	}
}

func TestJumpTop(t *testing.T) {
	m := NewModel()
	m.SetFocused(true)
	m.SetIndex(simpleIndex())
	m.SetSize(80, 20)
	m.cursor = 2

	m, _ = m.Update(keyPress('g'))
	if m.cursor != 0 {
		t.Errorf("cursor = %d, want 0 after g", m.cursor)
	}
}

func TestJumpBottom(t *testing.T) {
	m := NewModel()
	m.SetFocused(true)
	m.SetIndex(simpleIndex())
	m.SetSize(80, 20)

	m, _ = m.Update(keyPress('G'))
	want := len(m.flatList) - 1
	if m.cursor != want {
		t.Errorf("cursor = %d, want %d after G", m.cursor, want)
	}
}

func TestExpandOrchestrator(t *testing.T) {
	m := NewModel()
	m.SetFocused(true)
	m.SetIndex(orchestratorIndex())
	m.SetSize(80, 20)

	// Cursor is on the orchestrator. Press enter to expand.
	m, _ = m.Update(specialKey(tea.KeyEnter))

	if len(m.flatList) != 3 {
		t.Errorf("flatList length = %d, want 3 after expand", len(m.flatList))
	}
	if !m.flatList[0].IsExpanded {
		t.Error("parent should be expanded after enter")
	}
}

func TestCollapseOrchestrator(t *testing.T) {
	m := NewModel()
	m.SetFocused(true)
	m.expanded["parent"] = true
	m.SetIndex(orchestratorIndex())
	m.SetSize(80, 20)

	if len(m.flatList) != 3 {
		t.Fatalf("expected 3 rows when expanded, got %d", len(m.flatList))
	}

	// Press esc to collapse.
	m, _ = m.Update(specialKey(tea.KeyEscape))

	if len(m.flatList) != 1 {
		t.Errorf("flatList length = %d, want 1 after collapse", len(m.flatList))
	}
}

func TestCollapse_AlreadyCollapsed_JumpsToParent(t *testing.T) {
	m := NewModel()
	m.SetFocused(true)
	m.expanded["parent"] = true
	m.SetIndex(orchestratorIndex())
	m.SetSize(80, 20)

	// Move cursor to child-a (index 1).
	m.cursor = 1

	// Press h (collapse) on an already-collapsed leaf: should jump to parent.
	m, _ = m.Update(keyPress('h'))
	if m.cursor != 0 {
		t.Errorf("cursor = %d, want 0 (should jump to parent)", m.cursor)
	}
}

func TestExpandLeaf_FiresLoadNodeCmd(t *testing.T) {
	m := NewModel()
	m.SetFocused(true)
	m.SetIndex(simpleIndex())
	m.SetSize(80, 20)

	// Cursor on alpha (a leaf, no cached node state).
	m, cmd := m.Update(specialKey(tea.KeyEnter))

	if cmd == nil {
		t.Fatal("expand on uncached leaf should return a command")
	}

	msg := cmd()
	loadMsg, ok := msg.(LoadNodeMsg)
	if !ok {
		t.Fatalf("expected LoadNodeMsg, got %T", msg)
	}
	if loadMsg.Address != "alpha" {
		t.Errorf("LoadNodeMsg.Address = %q, want %q", loadMsg.Address, "alpha")
	}
}

func TestExpandLeaf_CachedNode_NoCmd(t *testing.T) {
	m := NewModel()
	m.SetFocused(true)
	m.SetIndex(simpleIndex())
	m.SetSize(80, 20)

	// Pre-cache a node.
	m.nodes["alpha"] = &state.NodeState{
		Tasks: []state.Task{
			{ID: "t1", Title: "Task One", State: state.StatusComplete},
		},
	}

	m, cmd := m.Update(specialKey(tea.KeyEnter))
	if cmd != nil {
		t.Error("expand on cached leaf should not fire a command")
	}

	// Tasks should appear in the flat list now.
	found := false
	for _, row := range m.flatList {
		if row.IsTask && row.Name == "Task One" {
			found = true
			break
		}
	}
	if !found {
		t.Error("tasks should appear in flatList after expanding a cached leaf")
	}
}

func TestLoadNodeMsg_TasksAppear(t *testing.T) {
	m := NewModel()
	m.SetFocused(true)
	m.SetIndex(simpleIndex())
	m.SetSize(80, 20)

	// Expand alpha first.
	m.expanded["alpha"] = true
	m.buildFlatList()

	// Receive loaded node state.
	ns := &state.NodeState{
		Tasks: []state.Task{
			{ID: "t1", Title: "First Task", State: state.StatusInProgress},
			{ID: "t2", Title: "Second Task", State: state.StatusNotStarted},
		},
	}
	m, _ = m.Update(LoadNodeMsg{Address: "alpha", Node: ns})

	taskCount := 0
	for _, row := range m.flatList {
		if row.IsTask {
			taskCount++
		}
	}
	if taskCount != 2 {
		t.Errorf("expected 2 task rows, got %d", taskCount)
	}
}

func TestLoadNodeMsg_Error_Ignored(t *testing.T) {
	m := NewModel()
	m.SetFocused(true)
	m.SetIndex(simpleIndex())
	m.SetSize(80, 20)

	before := len(m.flatList)
	m, _ = m.Update(LoadNodeMsg{Address: "alpha", Err: errTest})
	after := len(m.flatList)

	if before != after {
		t.Errorf("error LoadNodeMsg should not change flatList: %d -> %d", before, after)
	}
}

var errTest = &testError{}

type testError struct{}

func (e *testError) Error() string { return "test error" }

func TestSetFocused(t *testing.T) {
	m := NewModel()
	m.SetFocused(true)
	if !m.focused {
		t.Error("SetFocused(true) should set focused")
	}
	m.SetFocused(false)
	if m.focused {
		t.Error("SetFocused(false) should clear focused")
	}
}

func TestSetCurrentTarget(t *testing.T) {
	m := NewModel()
	m.SetCurrentTarget("some/addr")
	if m.currentTarget != "some/addr" {
		t.Errorf("currentTarget = %q, want %q", m.currentTarget, "some/addr")
	}
}

func TestSelectedAddr(t *testing.T) {
	m := NewModel()
	m.SetIndex(simpleIndex())

	addr := m.SelectedAddr()
	if addr != "alpha" {
		t.Errorf("SelectedAddr = %q, want %q", addr, "alpha")
	}

	m.cursor = 2
	addr = m.SelectedAddr()
	if addr != "gamma" {
		t.Errorf("SelectedAddr = %q, want %q", addr, "gamma")
	}
}

func TestSelectedAddr_Empty(t *testing.T) {
	m := NewModel()
	addr := m.SelectedAddr()
	if addr != "" {
		t.Errorf("SelectedAddr on empty list = %q, want empty", addr)
	}
}

func TestScrollIntoCursor(t *testing.T) {
	m := NewModel()
	m.SetIndex(simpleIndex())
	m.SetSize(80, 2) // only 2 visible rows

	// Move cursor beyond viewport.
	m.cursor = 2
	m.scrollIntoCursor()

	if m.scrollTop != 1 {
		t.Errorf("scrollTop = %d, want 1 (cursor at 2 with height 2)", m.scrollTop)
	}

	// Move cursor above viewport.
	m.cursor = 0
	m.scrollIntoCursor()
	if m.scrollTop != 0 {
		t.Errorf("scrollTop = %d, want 0", m.scrollTop)
	}
}

func TestScrollIntoCursor_ZeroHeight(t *testing.T) {
	m := NewModel()
	m.SetIndex(simpleIndex())
	m.SetSize(80, 0)

	// Should not panic.
	m.cursor = 2
	m.scrollIntoCursor()
}

func TestUnfocused_IgnoresKeys(t *testing.T) {
	m := NewModel()
	m.SetFocused(false)
	m.SetIndex(simpleIndex())
	m.SetSize(80, 20)

	m, _ = m.Update(keyPress('j'))
	if m.cursor != 0 {
		t.Errorf("unfocused model should ignore key presses, cursor = %d", m.cursor)
	}
}

func TestStateUpdatedMsg_RebuildsFlatList(t *testing.T) {
	m := NewModel()
	m.SetIndex(simpleIndex())

	if len(m.flatList) != 3 {
		t.Fatalf("initial flatList len = %d, want 3", len(m.flatList))
	}

	// Update with a different index.
	newIdx := &state.RootIndex{
		Root: []string{"only"},
		Nodes: map[string]state.IndexEntry{
			"only": {Name: "Only One", Type: state.NodeLeaf, State: state.StatusComplete},
		},
	}
	m, _ = m.Update(StateUpdatedMsg{Index: newIdx})

	if len(m.flatList) != 1 {
		t.Errorf("after StateUpdatedMsg flatList len = %d, want 1", len(m.flatList))
	}
}

func TestNodeUpdatedMsg_UpdatesCache(t *testing.T) {
	m := NewModel()
	m.SetIndex(simpleIndex())
	m.expanded["alpha"] = true

	ns := &state.NodeState{
		Tasks: []state.Task{
			{ID: "t1", Title: "Updated Task", State: state.StatusComplete},
		},
	}
	m, _ = m.Update(NodeUpdatedMsg{Address: "alpha", Node: ns})

	if m.nodes["alpha"] != ns {
		t.Error("NodeUpdatedMsg should cache the node state")
	}
	// Expanded nodes must NOT have a cache expiry timer; otherwise their
	// tasks would vanish after 30s of polling, looking like an unexpected
	// collapse.
	if _, ok := m.cacheExpiry["alpha"]; ok {
		t.Error("NodeUpdatedMsg should not set cache expiry for expanded nodes")
	}
}

// TestNodeUpdatedMsg_DoesNotSetCacheExpiryWhenCollapsed is the
// regression for the cache-eviction removal. The eager prefetch
// path populates the cache for every leaf at TUI startup so search
// can walk task content even when the leaf is collapsed; if the
// 30-second eviction timer were still set on collapsed entries,
// the eager-loaded state would silently disappear after the next
// poll tick and the active-task display would go stale again.
func TestNodeUpdatedMsg_DoesNotSetCacheExpiryWhenCollapsed(t *testing.T) {
	m := NewModel()
	m.SetIndex(simpleIndex())
	// Deliberately NOT expanded.

	ns := &state.NodeState{
		Tasks: []state.Task{
			{ID: "t1", Title: "Task", State: state.StatusComplete},
		},
	}
	m, _ = m.Update(NodeUpdatedMsg{Address: "alpha", Node: ns})

	if _, ok := m.cacheExpiry["alpha"]; ok {
		t.Error("NodeUpdatedMsg must not set a cache expiry timer for collapsed nodes; the watcher's per-leaf subscription keeps the cache fresh and eager prefetch needs the entries to persist")
	}
	if _, ok := m.nodes["alpha"]; !ok {
		t.Error("NodeUpdatedMsg should still cache the node state for collapsed nodes")
	}
}

// TestHandleCollapse_DoesNotSetCacheExpiry is the regression for
// the same eviction removal on the collapse path. Collapsing a
// previously-expanded leaf should NOT start an eviction timer.
func TestHandleCollapse_DoesNotSetCacheExpiry(t *testing.T) {
	m := NewModel()
	m.SetFocused(true)
	m.SetIndex(orchestratorIndex())
	m.SetSize(80, 20)

	// Expand the orchestrator first so a collapse is possible.
	m.expanded["parent"] = true
	m.buildFlatList()

	// Find parent's row index and put the cursor on it.
	for i, row := range m.flatList {
		if row.Addr == "parent" {
			m.cursor = i
			break
		}
	}

	// Collapse via the handler.
	m, _ = m.handleCollapse()

	if _, ok := m.cacheExpiry["parent"]; ok {
		t.Error("handleCollapse must not set a cache expiry timer; the cache is now permanent for the session")
	}
	if m.expanded["parent"] {
		t.Error("handleCollapse should mark parent as collapsed")
	}
}

func TestExpandToggle_AlreadyExpanded(t *testing.T) {
	m := NewModel()
	m.SetFocused(true)
	m.SetIndex(orchestratorIndex())
	m.SetSize(80, 20)

	// Expand.
	m, _ = m.Update(specialKey(tea.KeyEnter))
	if len(m.flatList) != 3 {
		t.Fatalf("after expand: %d rows, want 3", len(m.flatList))
	}

	// Enter again on an already-expanded node should collapse it.
	m, _ = m.Update(specialKey(tea.KeyEnter))
	if len(m.flatList) != 1 {
		t.Errorf("enter on expanded should collapse: %d rows, want 1", len(m.flatList))
	}
}

func TestExpandTask_Noop(t *testing.T) {
	m := NewModel()
	m.SetFocused(true)
	m.SetIndex(simpleIndex())
	m.SetSize(80, 20)

	// Cache a node with tasks.
	m.nodes["alpha"] = &state.NodeState{
		Tasks: []state.Task{{ID: "t1", Title: "T", State: state.StatusComplete}},
	}
	m.expanded["alpha"] = true
	m.buildFlatList()

	// Move cursor to the task row.
	m.cursor = 1 // should be the task
	if !m.flatList[1].IsTask {
		t.Fatal("expected row 1 to be a task")
	}

	before := len(m.flatList)
	m, cmd := m.Update(specialKey(tea.KeyEnter))
	if cmd != nil {
		t.Error("enter on task should not produce a command")
	}
	if len(m.flatList) != before {
		t.Error("enter on task should not change flatList")
	}
}

func TestBuildFlatList_NilIndex(t *testing.T) {
	m := NewModel()
	m.buildFlatList()

	if m.flatList != nil {
		t.Errorf("flatList should be nil with nil index, got len %d", len(m.flatList))
	}
}

func TestClampCursor_EmptyList(t *testing.T) {
	m := NewModel()
	m.cursor = 5
	m.clampCursor()
	if m.cursor != 0 {
		t.Errorf("cursor should clamp to 0 on empty list, got %d", m.cursor)
	}
}

func TestClampCursor_BeyondEnd(t *testing.T) {
	m := NewModel()
	m.SetIndex(simpleIndex())
	m.cursor = 100
	m.clampCursor()
	if m.cursor != 2 {
		t.Errorf("cursor should clamp to last index, got %d", m.cursor)
	}
}

func TestClampCursor_Negative(t *testing.T) {
	m := NewModel()
	m.SetIndex(simpleIndex())
	m.cursor = -5
	m.clampCursor()
	if m.cursor != 0 {
		t.Errorf("cursor should clamp to 0, got %d", m.cursor)
	}
}

func TestFlatList_Accessor(t *testing.T) {
	m := NewModel()
	m.SetIndex(simpleIndex())

	fl := m.FlatList()
	if len(fl) != 3 {
		t.Errorf("FlatList() len = %d, want 3", len(fl))
	}
}

func TestSetSize(t *testing.T) {
	m := NewModel()
	m.SetSize(60, 25)
	if m.width != 60 {
		t.Errorf("width = %d, want 60", m.width)
	}
	if m.height != 25 {
		t.Errorf("height = %d, want 25", m.height)
	}
}

func TestParentOf_TaskRow(t *testing.T) {
	m := NewModel()
	m.SetIndex(simpleIndex())
	m.expanded["alpha"] = true
	m.nodes["alpha"] = &state.NodeState{
		Tasks: []state.Task{{ID: "t1", Title: "T1", State: state.StatusComplete}},
	}
	m.buildFlatList()

	// The task row addr is "alpha/t1", parent should be "alpha" at index 0.
	idx := m.parentOf("alpha/t1")
	if idx != 0 {
		t.Errorf("parentOf(alpha/t1) = %d, want 0", idx)
	}
}

func TestParentOf_NilIndex(t *testing.T) {
	m := NewModel()
	idx := m.parentOf("anything")
	if idx != -1 {
		t.Errorf("parentOf with nil index = %d, want -1", idx)
	}
}

func TestHandleCollapse_EmptyList(t *testing.T) {
	m := NewModel()
	m.SetFocused(true)

	// Should not panic.
	m, _ = m.Update(specialKey(tea.KeyEscape))
}

func TestHandleExpand_EmptyList(t *testing.T) {
	m := NewModel()
	m.SetFocused(true)

	// Should not panic.
	m, cmd := m.Update(specialKey(tea.KeyEnter))
	if cmd != nil {
		t.Error("expand on empty list should not produce a command")
	}
}

func TestSetSearchAddresses(t *testing.T) {
	m := NewModel()
	m.SetIndex(simpleIndex())

	literal := map[string]bool{"alpha": true, "alpha/beta": true}
	ancestor := map[string]bool{"root": true}
	m.SetSearchAddresses(literal, ancestor)

	if !m.searchLiteral["alpha"] || !m.searchLiteral["alpha/beta"] {
		t.Error("SetSearchAddresses should store the provided literal map")
	}
	if !m.searchAncestor["root"] {
		t.Error("SetSearchAddresses should store the provided ancestor map")
	}
	if m.searchLiteral["alpha/gamma"] {
		t.Error("alpha/gamma should not be a literal match")
	}

	m.SetSearchAddresses(nil, nil)
	if m.searchLiteral != nil || m.searchAncestor != nil {
		t.Error("SetSearchAddresses(nil, nil) should clear both maps")
	}
	if m.HasSearchHighlights() {
		t.Error("HasSearchHighlights should report false after clear")
	}
}

func TestSetCursor(t *testing.T) {
	m := NewModel()
	m.SetIndex(simpleIndex())
	m.SetSize(80, 20)

	m.SetCursor(2)
	if m.cursor != 2 {
		t.Errorf("cursor = %d, want 2", m.cursor)
	}

	// Beyond bounds should clamp.
	m.SetCursor(100)
	if m.cursor != 2 {
		t.Errorf("cursor = %d, want 2 (clamped)", m.cursor)
	}

	m.SetCursor(-1)
	if m.cursor != 0 {
		t.Errorf("cursor = %d, want 0 (clamped)", m.cursor)
	}
}

func TestCleanCache_RemovesExpired(t *testing.T) {
	m := NewModel()
	m.SetIndex(simpleIndex())

	// Add a node with an already-expired cache entry.
	m.nodes["alpha"] = &state.NodeState{}
	m.cacheExpiry["alpha"] = time.Now().Add(-1 * time.Second)

	// Add a node with a fresh cache entry.
	m.nodes["beta"] = &state.NodeState{}
	m.cacheExpiry["beta"] = time.Now().Add(30 * time.Second)

	m.CleanCache()

	if _, ok := m.nodes["alpha"]; ok {
		t.Error("expired node should have been evicted")
	}
	if _, ok := m.cacheExpiry["alpha"]; ok {
		t.Error("expired cache expiry entry should have been removed")
	}
	if _, ok := m.nodes["beta"]; !ok {
		t.Error("fresh node should still be cached")
	}
}

func TestCleanCache_EmptyIsNoop(t *testing.T) {
	m := NewModel()
	// Should not panic on empty maps.
	m.CleanCache()
}

// TestCollapse_DoesNotSetCacheExpiry replaces the original test that
// asserted the opposite. The eviction-on-collapse behavior has been
// removed: the watcher's per-leaf fsnotify subscription keeps every
// cached entry fresh, so eviction is no longer necessary, and the
// eager-prefetch path needs collapsed entries to persist so search
// can walk task content even when the leaf is folded.
func TestCollapse_DoesNotSetCacheExpiry(t *testing.T) {
	m := NewModel()
	m.SetFocused(true)
	m.expanded["parent"] = true
	m.SetIndex(orchestratorIndex())
	m.SetSize(80, 20)

	m, _ = m.Update(specialKey(tea.KeyEscape))

	if _, ok := m.cacheExpiry["parent"]; ok {
		t.Error("collapse must not set a cache expiry timer; entries are kept until session end and refreshed by the watcher")
	}
	if m.expanded["parent"] {
		t.Error("collapse should mark the node as collapsed")
	}
}
