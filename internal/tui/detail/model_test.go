package detail

import (
	"strings"
	"testing"

	tea "charm.land/bubbletea/v2"

	"github.com/dorkusprime/wolfcastle/internal/state"
	"github.com/dorkusprime/wolfcastle/internal/tui"
)

func TestNewModel_StartsInDashboardMode(t *testing.T) {
	t.Parallel()
	m := NewModel()
	if m.mode != ModeDashboard {
		t.Errorf("expected ModeDashboard (%d), got %d", ModeDashboard, m.mode)
	}
}

func TestSetMode_Dashboard(t *testing.T) {
	t.Parallel()
	m := NewModel()
	m.SetMode(ModeDashboard)
	if m.mode != ModeDashboard {
		t.Errorf("expected ModeDashboard, got %d", m.mode)
	}
	v := m.View()
	// Dashboard view includes the heading from the dashboard sub-model
	if !strings.Contains(v, "MISSION BRIEFING") {
		t.Errorf("expected dashboard view to contain MISSION BRIEFING, got: %s", v)
	}
}

func TestSetMode_NodeDetail_RendersNodeView(t *testing.T) {
	t.Parallel()
	m := NewModel()
	m.SetSize(80, 24)
	m.SetMode(ModeNodeDetail)
	if m.mode != ModeNodeDetail {
		t.Errorf("expected ModeNodeDetail, got %d", m.mode)
	}
	// Without loading data, the node detail view renders an empty viewport.
	_ = m.View()
}

func TestSetMode_LogStream_RendersLogView(t *testing.T) {
	t.Parallel()
	m := NewModel()
	m.SetSize(80, 24)
	m.SwitchToLogView()
	if m.mode != ModeLogStream {
		t.Errorf("expected ModeLogStream, got %d", m.mode)
	}
	v := m.View()
	if !strings.Contains(v, "TRANSMISSIONS") {
		t.Errorf("expected TRANSMISSIONS header in log view, got: %s", v)
	}
}

func TestSetMode_TaskDetail_RendersTaskView(t *testing.T) {
	t.Parallel()
	m := NewModel()
	m.SetSize(80, 24)
	m.SetMode(ModeTaskDetail)
	if m.mode != ModeTaskDetail {
		t.Errorf("expected ModeTaskDetail, got %d", m.mode)
	}
	// Without loading data, the task detail view renders an empty viewport.
	_ = m.View()
}

func TestSetMode_Inbox_ShowsInboxView(t *testing.T) {
	t.Parallel()
	m := NewModel()
	m.SetSize(80, 24)
	m.SetMode(ModeInbox)
	v := m.View()
	if !strings.Contains(v, "INBOX") {
		t.Errorf("expected INBOX header, got: %s", v)
	}
	if !strings.Contains(v, "The silence is temporary") {
		t.Errorf("expected empty inbox message, got: %s", v)
	}
}

func TestUpdate_ForwardsStateUpdatedMsg(t *testing.T) {
	t.Parallel()
	m := NewModel()
	idx := &state.RootIndex{
		Nodes: map[string]state.IndexEntry{
			"a": {State: state.StatusComplete},
			"b": {State: state.StatusInProgress},
		},
	}
	m, _ = m.Update(tui.StateUpdatedMsg{Index: idx})
	if m.dashboard.totalNodes != 2 {
		t.Errorf("expected 2 total nodes, got %d", m.dashboard.totalNodes)
	}
}

func TestUpdate_ForwardsDaemonStatusMsg(t *testing.T) {
	t.Parallel()
	m := NewModel()
	m, _ = m.Update(tui.DaemonStatusMsg{
		Status:    "on patrol",
		Branch:    "main",
		IsRunning: true,
	})
	if m.dashboard.daemonStatus != "on patrol" {
		t.Errorf("expected 'on patrol', got %q", m.dashboard.daemonStatus)
	}
	if !m.dashboard.daemonRunning {
		t.Error("expected daemonRunning to be true")
	}
}

func TestUpdate_ForwardsLogLinesMsg(t *testing.T) {
	t.Parallel()
	m := NewModel()
	m, _ = m.Update(tui.LogLinesMsg{Lines: []string{
		`{"type":"stage_start","stage":"intake","node":"alpha"}`,
		`{"type":"stage_complete","stage":"exec","node":"alpha","exit_code":0}`,
	}})
	if len(m.dashboard.recentActivity) != 2 {
		t.Errorf("expected 2 activity entries, got %d", len(m.dashboard.recentActivity))
	}
}

func TestUpdate_NonDashboardModeIgnoresStateMsg(t *testing.T) {
	t.Parallel()
	m := NewModel()
	m.SetMode(ModeNodeDetail)
	m, _ = m.Update(tui.StateUpdatedMsg{Index: &state.RootIndex{
		Nodes: map[string]state.IndexEntry{"a": {State: state.StatusComplete}},
	}})
	// Dashboard should not have been updated since we're in node detail mode
	if m.dashboard.totalNodes != 0 {
		t.Errorf("expected 0 total nodes in node detail mode, got %d", m.dashboard.totalNodes)
	}
}

func TestUpdate_LogStreamModeHandlesKeyPress(t *testing.T) {
	t.Parallel()
	m := NewModel()
	m.SetSize(80, 24)
	m.SwitchToLogView()
	// Should not panic on key press in log stream mode
	m, _ = m.Update(tea.KeyPressMsg{Code: tea.KeyDown})
	if m.mode != ModeLogStream {
		t.Errorf("expected ModeLogStream, got %d", m.mode)
	}
}

func TestUpdate_DashboardModeHandlesKeyPress(t *testing.T) {
	t.Parallel()
	m := NewModel()
	m.SetSize(80, 24)
	// Should not panic on key press in dashboard mode
	m, _ = m.Update(tea.KeyPressMsg{Code: tea.KeyDown})
	if m.mode != ModeDashboard {
		t.Errorf("expected ModeDashboard, got %d", m.mode)
	}
}

func TestSetSize_PropagatesDimensions(t *testing.T) {
	t.Parallel()
	m := NewModel()
	m.SetSize(100, 50)
	if m.width != 100 || m.height != 50 {
		t.Errorf("expected 100x50, got %dx%d", m.width, m.height)
	}
	if m.dashboard.width != 100 || m.dashboard.height != 50 {
		t.Errorf("expected dashboard 100x50, got %dx%d", m.dashboard.width, m.dashboard.height)
	}
}

func TestSetFocused(t *testing.T) {
	t.Parallel()
	m := NewModel()
	m.SetFocused(true)
	if !m.focused {
		t.Error("expected focused to be true")
	}
	m.SetFocused(false)
	if m.focused {
		t.Error("expected focused to be false")
	}
}

func TestView_DashboardMode(t *testing.T) {
	t.Parallel()
	m := NewModel()
	v := m.View()
	if !strings.Contains(v, "MISSION BRIEFING") {
		t.Errorf("expected MISSION BRIEFING in dashboard view, got: %s", v)
	}
}

func TestView_InboxMode_ShowsInboxView(t *testing.T) {
	t.Parallel()
	m := NewModel()
	m.SetSize(80, 24)
	m.SetMode(ModeInbox)
	v := m.View()
	if !strings.Contains(v, "INBOX") {
		t.Errorf("expected INBOX header, got: %s", v)
	}
}

func TestUpdate_UnhandledMsgReturnsSelf(t *testing.T) {
	t.Parallel()
	m := NewModel()
	m2, cmd := m.Update(tea.FocusMsg{})
	if cmd != nil {
		t.Error("expected nil cmd for unhandled msg")
	}
	if m2.mode != m.mode {
		t.Error("model should be unchanged for unhandled msg")
	}
}

func TestMode_Accessor(t *testing.T) {
	t.Parallel()
	m := NewModel()
	if m.Mode() != ModeDashboard {
		t.Errorf("expected ModeDashboard, got %d", m.Mode())
	}
	m.SetMode(ModeLogStream)
	if m.Mode() != ModeLogStream {
		t.Errorf("expected ModeLogStream, got %d", m.Mode())
	}
}

func TestIsCapturingInput_FalseOutsideInbox(t *testing.T) {
	t.Parallel()
	m := NewModel()
	if m.IsCapturingInput() {
		t.Error("should not be capturing input in dashboard mode")
	}
	m.SetMode(ModeNodeDetail)
	if m.IsCapturingInput() {
		t.Error("should not be capturing input in node detail mode")
	}
}

func TestIsCapturingInput_InboxNotActive(t *testing.T) {
	t.Parallel()
	m := NewModel()
	m.SetMode(ModeInbox)
	// Inbox input is not active by default
	if m.IsCapturingInput() {
		t.Error("inbox input should not be active by default")
	}
}

func TestInboxModelRef(t *testing.T) {
	t.Parallel()
	m := NewModel()
	ref := m.InboxModelRef()
	if ref == nil {
		t.Fatal("InboxModelRef should not return nil")
	}
	// Verify it points to the actual inbox sub-model by mutating through the ref.
	ref.SetReadError(true)
	if !m.inbox.readErr {
		t.Error("InboxModelRef should return a pointer to the actual inbox sub-model")
	}
}

func TestLogViewModelRef(t *testing.T) {
	t.Parallel()
	m := NewModel()
	ref := m.LogViewModelRef()
	if ref == nil {
		t.Fatal("LogViewModelRef should not return nil")
	}
	// Verify it points to the actual log view sub-model.
	ref.SetSize(42, 10)
	if m.logView.width != 42 {
		t.Error("LogViewModelRef should return a pointer to the actual log view sub-model")
	}
}

func TestUpdatePlaceholder_KeyPress(t *testing.T) {
	t.Parallel()
	m := NewModel()
	m.SetSize(80, 24)
	// Set mode to something beyond the known modes to hit the default branch.
	m.mode = Mode(99)
	m, cmd := m.Update(tea.KeyPressMsg{Code: tea.KeyDown})
	if cmd != nil {
		// viewport.Update may or may not produce a cmd; the key thing is no panic.
		_ = cmd
	}
	if m.mode != Mode(99) {
		t.Errorf("mode should remain 99, got %d", m.mode)
	}
}

func TestUpdatePlaceholder_NonKeyMsg(t *testing.T) {
	t.Parallel()
	m := NewModel()
	m.SetSize(80, 24)
	m.mode = Mode(99)
	m, cmd := m.Update(tea.FocusMsg{})
	if cmd != nil {
		t.Error("expected nil cmd for non-key msg in placeholder mode")
	}
}

func TestLoadNodeDetail(t *testing.T) {
	t.Parallel()
	m := NewModel()
	m.SetSize(80, 24)
	node := &state.NodeState{
		Tasks: []state.Task{{ID: "t1", Title: "Task", State: state.StatusComplete}},
	}
	entry := &state.IndexEntry{Name: "Alpha", Type: state.NodeLeaf, State: state.StatusInProgress}
	m.LoadNodeDetail("alpha", node, entry, true)
	if m.mode != ModeNodeDetail {
		t.Errorf("expected ModeNodeDetail, got %d", m.mode)
	}
	if m.nodeDetail.Addr() != "alpha" {
		t.Errorf("expected addr 'alpha', got %q", m.nodeDetail.Addr())
	}
}

func TestLoadTaskDetail(t *testing.T) {
	t.Parallel()
	m := NewModel()
	m.SetSize(80, 24)
	task := &state.Task{ID: "t1", Title: "Build", State: state.StatusInProgress}
	m.LoadTaskDetail("alpha", "t1", task)
	if m.mode != ModeTaskDetail {
		t.Errorf("expected ModeTaskDetail, got %d", m.mode)
	}
	if m.taskDetail.TaskAddr() != "alpha/t1" {
		t.Errorf("expected task addr 'alpha/t1', got %q", m.taskDetail.TaskAddr())
	}
}

func TestSwitchToDashboard(t *testing.T) {
	t.Parallel()
	m := NewModel()
	m.SetMode(ModeLogStream)
	m.SwitchToDashboard()
	if m.mode != ModeDashboard {
		t.Errorf("expected ModeDashboard, got %d", m.mode)
	}
}

func TestReset(t *testing.T) {
	t.Parallel()
	m := NewModel()
	m.SetSize(80, 24)
	m.SetMode(ModeLogStream)
	m.logView.AppendLines([]string{
		`{"type":"daemon_start","timestamp":"2026-01-01T00:00:00Z","level":"info"}`,
	})
	m.Reset()
	if m.mode != ModeDashboard {
		t.Errorf("expected ModeDashboard after Reset, got %d", m.mode)
	}
}

func TestSwitchToInbox(t *testing.T) {
	t.Parallel()
	m := NewModel()
	m.SetFocused(true)
	m.SwitchToInbox()
	if m.mode != ModeInbox {
		t.Errorf("expected ModeInbox, got %d", m.mode)
	}
}

func TestLoadInbox(t *testing.T) {
	t.Parallel()
	m := NewModel()
	items := []state.InboxItem{
		{Text: "first", Status: state.InboxNew},
		{Text: "second", Status: state.InboxFiled},
	}
	m.LoadInbox(items)
	if len(m.inbox.items) != 2 {
		t.Errorf("expected 2 inbox items, got %d", len(m.inbox.items))
	}
}

func TestSetDashboardInbox(t *testing.T) {
	t.Parallel()
	m := NewModel()
	items := []state.InboxItem{
		{Text: "a", Status: state.InboxNew},
	}
	m.SetDashboardInbox(items)
	if len(m.dashboard.inboxItems) != 1 {
		t.Errorf("expected 1 dashboard inbox item, got %d", len(m.dashboard.inboxItems))
	}
}

func TestSetInboxReadError(t *testing.T) {
	t.Parallel()
	m := NewModel()
	m.SetInboxReadError(true)
	if !m.inbox.readErr {
		t.Error("expected inbox readErr to be true")
	}
	m.SetInboxReadError(false)
	if m.inbox.readErr {
		t.Error("expected inbox readErr to be false")
	}
}

func TestSearchContent_NodeDetail(t *testing.T) {
	t.Parallel()
	m := NewModel()
	m.SetSize(80, 24)
	node := &state.NodeState{
		Tasks: []state.Task{{ID: "t1", Title: "Build widgets", State: state.StatusComplete}},
	}
	entry := &state.IndexEntry{Name: "Alpha", Type: state.NodeLeaf, State: state.StatusInProgress}
	m.LoadNodeDetail("alpha", node, entry, false)
	content := m.SearchContent()
	if len(content) == 0 {
		t.Error("expected non-empty SearchContent for node detail")
	}
}

func TestSearchContent_TaskDetail(t *testing.T) {
	t.Parallel()
	m := NewModel()
	m.SetSize(80, 24)
	task := &state.Task{ID: "t1", Title: "Build", State: state.StatusInProgress}
	m.LoadTaskDetail("alpha", "t1", task)
	content := m.SearchContent()
	if len(content) == 0 {
		t.Error("expected non-empty SearchContent for task detail")
	}
}

func TestSearchContent_LogStream(t *testing.T) {
	t.Parallel()
	m := NewModel()
	m.SetSize(80, 24)
	m.SwitchToLogView()
	m.logView.AppendLines([]string{
		`{"type":"stage_start","stage":"exec","node":"alpha","timestamp":"2026-01-01T00:00:00Z","level":"info"}`,
	})
	content := m.SearchContent()
	if len(content) == 0 {
		t.Error("expected non-empty SearchContent for log stream")
	}
}

func TestSearchContent_Inbox(t *testing.T) {
	t.Parallel()
	m := NewModel()
	m.SetSize(80, 24)
	m.SwitchToInbox()
	m.LoadInbox([]state.InboxItem{{Text: "hello", Status: state.InboxNew}})
	content := m.SearchContent()
	if len(content) == 0 {
		t.Error("expected non-empty SearchContent for inbox")
	}
}

func TestSearchContent_Dashboard_ReturnsNil(t *testing.T) {
	t.Parallel()
	m := NewModel()
	content := m.SearchContent()
	if content != nil {
		t.Errorf("expected nil SearchContent for dashboard, got %d items", len(content))
	}
}

func TestCopyTarget_NodeDetail(t *testing.T) {
	t.Parallel()
	m := NewModel()
	m.SetSize(80, 24)
	node := &state.NodeState{}
	entry := &state.IndexEntry{Name: "Alpha", Type: state.NodeLeaf}
	m.LoadNodeDetail("alpha", node, entry, false)
	if m.CopyTarget() != "alpha" {
		t.Errorf("expected copy target 'alpha', got %q", m.CopyTarget())
	}
}

func TestCopyTarget_TaskDetail(t *testing.T) {
	t.Parallel()
	m := NewModel()
	m.SetSize(80, 24)
	task := &state.Task{ID: "t1", Title: "Build"}
	m.LoadTaskDetail("alpha", "t1", task)
	if m.CopyTarget() != "alpha/t1" {
		t.Errorf("expected copy target 'alpha/t1', got %q", m.CopyTarget())
	}
}

func TestCopyTarget_LogStream(t *testing.T) {
	t.Parallel()
	m := NewModel()
	m.SetSize(80, 24)
	m.SwitchToLogView()
	// No lines loaded, so SelectedLineJSON returns empty
	if m.CopyTarget() != "" {
		t.Errorf("expected empty copy target for empty log view, got %q", m.CopyTarget())
	}
}

func TestCopyTarget_Inbox(t *testing.T) {
	t.Parallel()
	m := NewModel()
	m.SetSize(80, 24)
	m.SwitchToInbox()
	// No items loaded, so SelectedText returns empty
	if m.CopyTarget() != "" {
		t.Errorf("expected empty copy target for empty inbox, got %q", m.CopyTarget())
	}
}

func TestCopyTarget_Dashboard_ReturnsEmpty(t *testing.T) {
	t.Parallel()
	m := NewModel()
	if m.CopyTarget() != "" {
		t.Errorf("expected empty copy target for dashboard, got %q", m.CopyTarget())
	}
}

func TestView_PlaceholderMode(t *testing.T) {
	t.Parallel()
	m := NewModel()
	m.SetSize(80, 24)
	m.mode = Mode(99)
	// Should render the viewport's view without panic
	_ = m.View()
}
