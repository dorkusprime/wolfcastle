package detail

import (
	"strings"
	"testing"

	tea "charm.land/bubbletea/v2"

	"github.com/dorkusprime/wolfcastle/internal/state"
	"github.com/dorkusprime/wolfcastle/internal/tui"
)

func TestNewDetailModel_StartsInDashboardMode(t *testing.T) {
	t.Parallel()
	m := NewDetailModel()
	if m.mode != ModeDashboard {
		t.Errorf("expected ModeDashboard (%d), got %d", ModeDashboard, m.mode)
	}
}

func TestSetMode_Dashboard(t *testing.T) {
	t.Parallel()
	m := NewDetailModel()
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
	m := NewDetailModel()
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
	m := NewDetailModel()
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
	m := NewDetailModel()
	m.SetSize(80, 24)
	m.SetMode(ModeTaskDetail)
	if m.mode != ModeTaskDetail {
		t.Errorf("expected ModeTaskDetail, got %d", m.mode)
	}
	// Without loading data, the task detail view renders an empty viewport.
	_ = m.View()
}

func TestSetMode_Inbox_ShowsPlaceholder(t *testing.T) {
	t.Parallel()
	m := NewDetailModel()
	m.SetSize(80, 24)
	m.SetMode(ModeInbox)
	v := m.View()
	if !strings.Contains(v, "Coming in the next phase.") {
		t.Errorf("expected placeholder text, got: %s", v)
	}
}

func TestUpdate_ForwardsStateUpdatedMsg(t *testing.T) {
	t.Parallel()
	m := NewDetailModel()
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
	m := NewDetailModel()
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
	m := NewDetailModel()
	m, _ = m.Update(tui.LogLinesMsg{Lines: []string{"hello", "world"}})
	if len(m.dashboard.recentActivity) != 2 {
		t.Errorf("expected 2 activity entries, got %d", len(m.dashboard.recentActivity))
	}
}

func TestUpdate_NonDashboardModeIgnoresStateMsg(t *testing.T) {
	t.Parallel()
	m := NewDetailModel()
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
	m := NewDetailModel()
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
	m := NewDetailModel()
	m.SetSize(80, 24)
	// Should not panic on key press in dashboard mode
	m, _ = m.Update(tea.KeyPressMsg{Code: tea.KeyDown})
	if m.mode != ModeDashboard {
		t.Errorf("expected ModeDashboard, got %d", m.mode)
	}
}

func TestSetSize_PropagatesDimensions(t *testing.T) {
	t.Parallel()
	m := NewDetailModel()
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
	m := NewDetailModel()
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
	m := NewDetailModel()
	v := m.View()
	if !strings.Contains(v, "MISSION BRIEFING") {
		t.Errorf("expected MISSION BRIEFING in dashboard view, got: %s", v)
	}
}

func TestView_InboxMode_ShowsPlaceholder(t *testing.T) {
	t.Parallel()
	m := NewDetailModel()
	m.SetSize(80, 24)
	m.SetMode(ModeInbox)
	v := m.View()
	if !strings.Contains(v, "Coming in the next phase.") {
		t.Errorf("expected placeholder in inbox view, got: %s", v)
	}
}

func TestUpdate_UnhandledMsgReturnsSelf(t *testing.T) {
	t.Parallel()
	m := NewDetailModel()
	m2, cmd := m.Update(tea.FocusMsg{})
	if cmd != nil {
		t.Error("expected nil cmd for unhandled msg")
	}
	if m2.mode != m.mode {
		t.Error("model should be unchanged for unhandled msg")
	}
}
