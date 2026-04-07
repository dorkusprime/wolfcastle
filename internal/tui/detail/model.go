package detail

import (
	"charm.land/bubbles/v2/viewport"
	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"

	"github.com/dorkusprime/wolfcastle/internal/state"
	"github.com/dorkusprime/wolfcastle/internal/tui"
)

// DetailMode selects which sub-view the detail container renders.
type DetailMode int

const (
	ModeDashboard DetailMode = iota
	ModeNodeDetail
	ModeTaskDetail
	ModeLogStream
	ModeInbox
)

// DetailModel is the right-pane container that delegates rendering to the
// active sub-view.
type DetailModel struct {
	mode       DetailMode
	dashboard  DashboardModel
	nodeDetail NodeDetailModel
	taskDetail TaskDetailModel
	logView    LogViewModel
	inbox      InboxModel
	viewport   viewport.Model
	width      int
	height     int
	focused    bool
}

var placeholderStyle = lipgloss.NewStyle().
	Foreground(tui.ColorDimWhite).
	Italic(true)

const placeholderText = "Coming in the next phase."

// NewDetailModel creates a DetailModel starting in dashboard mode.
func NewDetailModel() DetailModel {
	vp := viewport.New()
	return DetailModel{
		mode:       ModeDashboard,
		dashboard:  NewDashboardModel(),
		nodeDetail: NewNodeDetailModel(),
		taskDetail: NewTaskDetailModel(),
		logView:    NewLogViewModel(),
		inbox:      NewInboxModel(),
		viewport:   vp,
	}
}

// Mode returns the current detail mode.
func (m DetailModel) Mode() DetailMode {
	return m.mode
}

// IsCapturingInput returns true when a sub-view is capturing keyboard
// input (e.g., the inbox text input field is active). The app
// orchestrator should suppress global key bindings in this state.
func (m DetailModel) IsCapturingInput() bool {
	return m.mode == ModeInbox && m.inbox.IsInputActive()
}

// Update routes messages to the active sub-view.
func (m DetailModel) Update(msg tea.Msg) (DetailModel, tea.Cmd) {
	// InboxUpdatedMsg is always forwarded to the inbox sub-model regardless of
	// which mode is active, so the data stays fresh when the user switches.
	if inboxMsg, ok := msg.(tui.InboxUpdatedMsg); ok {
		m.inbox, _ = m.inbox.Update(inboxMsg)
	}

	switch m.mode {
	case ModeDashboard:
		return m.updateDashboard(msg)
	case ModeNodeDetail:
		return m.updateNodeDetail(msg)
	case ModeTaskDetail:
		return m.updateTaskDetail(msg)
	case ModeLogStream:
		return m.updateLogView(msg)
	case ModeInbox:
		return m.updateInbox(msg)
	default:
		return m.updatePlaceholder(msg)
	}
}

func (m DetailModel) updateDashboard(msg tea.Msg) (DetailModel, tea.Cmd) {
	switch msg.(type) {
	case tui.StateUpdatedMsg, tui.DaemonStatusMsg, tui.LogLinesMsg:
		var cmd tea.Cmd
		m.dashboard, cmd = m.dashboard.Update(msg)
		return m, cmd
	case tea.KeyPressMsg:
		var cmd tea.Cmd
		m.dashboard, cmd = m.dashboard.Update(msg)
		return m, cmd
	}
	return m, nil
}

func (m DetailModel) updateNodeDetail(msg tea.Msg) (DetailModel, tea.Cmd) {
	switch msg.(type) {
	case tea.KeyPressMsg:
		var cmd tea.Cmd
		m.nodeDetail, cmd = m.nodeDetail.Update(msg)
		return m, cmd
	}
	return m, nil
}

func (m DetailModel) updateTaskDetail(msg tea.Msg) (DetailModel, tea.Cmd) {
	switch msg.(type) {
	case tea.KeyPressMsg:
		var cmd tea.Cmd
		m.taskDetail, cmd = m.taskDetail.Update(msg)
		return m, cmd
	}
	return m, nil
}

func (m DetailModel) updateLogView(msg tea.Msg) (DetailModel, tea.Cmd) {
	switch msg.(type) {
	case tea.KeyPressMsg, tui.LogLinesMsg, tui.NewLogFileMsg:
		var cmd tea.Cmd
		m.logView, cmd = m.logView.Update(msg)
		return m, cmd
	}
	return m, nil
}

func (m DetailModel) updateInbox(msg tea.Msg) (DetailModel, tea.Cmd) {
	switch msg.(type) {
	case tea.KeyPressMsg, tui.InboxUpdatedMsg:
		var cmd tea.Cmd
		m.inbox, cmd = m.inbox.Update(msg)
		return m, cmd
	}
	return m, nil
}

func (m DetailModel) updatePlaceholder(msg tea.Msg) (DetailModel, tea.Cmd) {
	switch msg.(type) {
	case tea.KeyPressMsg:
		var cmd tea.Cmd
		m.viewport, cmd = m.viewport.Update(msg)
		return m, cmd
	}
	return m, nil
}

// SetSize propagates dimensions to all sub-models.
func (m *DetailModel) SetSize(width, height int) {
	m.width = width
	m.height = height
	m.dashboard.SetSize(width, height)
	m.nodeDetail.SetSize(width, height)
	m.taskDetail.SetSize(width, height)
	m.logView.SetSize(width, height)
	m.inbox.SetSize(width, height)
	m.viewport.SetWidth(width)
	m.viewport.SetHeight(height)
}

// SetFocused marks whether this container currently holds keyboard focus.
func (m *DetailModel) SetFocused(focused bool) {
	m.focused = focused
}

// SetMode switches the active sub-view.
func (m *DetailModel) SetMode(mode DetailMode) {
	m.mode = mode
}

// LoadNodeDetail switches to node detail mode and populates the view.
func (m *DetailModel) LoadNodeDetail(addr string, node *state.NodeState, entry *state.IndexEntry, isTarget bool) {
	m.mode = ModeNodeDetail
	m.nodeDetail.Load(addr, node, entry, isTarget)
}

// LoadTaskDetail switches to task detail mode and populates the view.
func (m *DetailModel) LoadTaskDetail(addr, taskID string, task *state.Task) {
	m.mode = ModeTaskDetail
	m.taskDetail.Load(addr, taskID, task)
}

// SwitchToLogView switches to log stream mode.
func (m *DetailModel) SwitchToLogView() {
	m.mode = ModeLogStream
}

// SwitchToDashboard switches to dashboard mode.
func (m *DetailModel) SwitchToDashboard() {
	m.mode = ModeDashboard
}

// SwitchToInbox switches to inbox mode.
func (m *DetailModel) SwitchToInbox() {
	m.mode = ModeInbox
	m.inbox.SetFocused(m.focused)
}

// LoadInbox updates the inbox model with fresh data.
func (m *DetailModel) LoadInbox(items []state.InboxItem) {
	m.inbox.SetItems(items)
}

// SetDashboardInbox updates the dashboard's cached inbox items for the
// summary line, independent of the inbox detail view.
func (m *DetailModel) SetDashboardInbox(items []state.InboxItem) {
	m.dashboard.SetInbox(items)
}

// SetInboxReadError flags the inbox as unreadable.
func (m *DetailModel) SetInboxReadError(err bool) {
	m.inbox.SetReadError(err)
}

// SearchContent returns the searchable lines for the current detail mode.
// The app uses this when search is activated on the detail pane.
func (m DetailModel) SearchContent() []string {
	switch m.mode {
	case ModeNodeDetail:
		return m.nodeDetail.SearchContent()
	case ModeTaskDetail:
		return m.taskDetail.SearchContent()
	case ModeLogStream:
		return m.logView.SearchContent()
	case ModeInbox:
		return m.inbox.SearchContent()
	default:
		return nil
	}
}

// CopyTarget returns the appropriate copy text for the current mode.
func (m DetailModel) CopyTarget() string {
	switch m.mode {
	case ModeNodeDetail:
		return m.nodeDetail.Addr()
	case ModeTaskDetail:
		return m.taskDetail.TaskAddr()
	case ModeLogStream:
		return m.logView.SelectedLineJSON()
	case ModeInbox:
		return m.inbox.SelectedText()
	default:
		return ""
	}
}

// View renders the active sub-view.
func (m DetailModel) View() string {
	switch m.mode {
	case ModeDashboard:
		return m.dashboard.View()
	case ModeNodeDetail:
		return m.nodeDetail.View()
	case ModeTaskDetail:
		return m.taskDetail.View()
	case ModeLogStream:
		return m.logView.View()
	case ModeInbox:
		return m.inbox.View()
	default:
		return m.viewport.View()
	}
}
