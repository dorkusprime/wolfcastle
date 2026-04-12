// Package detail implements the right-pane detail views: dashboard, node detail, task detail, log stream, and inbox.
package detail

import (
	"charm.land/bubbles/v2/viewport"
	tea "charm.land/bubbletea/v2"

	"github.com/dorkusprime/wolfcastle/internal/state"
	"github.com/dorkusprime/wolfcastle/internal/tui"
)

// Mode selects which sub-view the detail container renders.
type Mode int

const (
	ModeDashboard Mode = iota
	ModeNodeDetail
	ModeTaskDetail
	ModeLogStream
	ModeInbox
)

// Model is the right-pane container that delegates rendering to the
// active sub-view.
type Model struct {
	mode       Mode
	dashboard  DashboardModel
	nodeDetail NodeModel
	taskDetail TaskModel
	logView    LogViewModel
	inbox      InboxModel
	viewport   viewport.Model
	width      int
	height     int
	focused    bool
}

// NewModel creates a Model starting in dashboard mode.
func NewModel() Model {
	vp := viewport.New()
	return Model{
		mode:       ModeDashboard,
		dashboard:  NewDashboardModel(),
		nodeDetail: NewNodeModel(),
		taskDetail: NewTaskModel(),
		logView:    NewLogViewModel(),
		inbox:      NewInboxModel(),
		viewport:   vp,
	}
}

// Mode returns the current detail mode.
func (m Model) Mode() Mode {
	return m.mode
}

// IsCapturingInput returns true when a sub-view is capturing keyboard
// input (e.g., the inbox text input field is active). The app
// orchestrator should suppress global key bindings in this state.
func (m Model) IsCapturingInput() bool {
	return m.mode == ModeInbox && m.inbox.IsInputActive()
}

// InboxModelRef returns a pointer to the inbox sub-model so the modal
// layer can borrow it for rendering and key routing without moving
// ownership out of the detail container.
func (m *Model) InboxModelRef() *InboxModel {
	return &m.inbox
}

// LogViewModelRef returns a pointer to the log view sub-model so the
// modal layer can borrow it for rendering and key routing.
func (m *Model) LogViewModelRef() *LogViewModel {
	return &m.logView
}

// Update routes messages to the active sub-view.
//
// Some messages are broadcast to background sub-models regardless of
// which mode is active so the data stays fresh when the user switches:
//
//   - InboxUpdatedMsg → inbox sub-model
//   - LogLinesMsg     → log view sub-model (so the watcher's tail-load
//     at startup is captured even when the detail
//     pane is currently in dashboard mode and the
//     user hasn't pressed L yet)
//   - NewLogFileMsg   → log view sub-model (same rationale: rotation
//     events must reach the log view even when it
//     is not the active mode)
func (m Model) Update(msg tea.Msg) (Model, tea.Cmd) {
	if inboxMsg, ok := msg.(tui.InboxUpdatedMsg); ok {
		m.inbox, _ = m.inbox.Update(inboxMsg)
	}
	if _, ok := msg.(tui.LogLinesMsg); ok && m.mode != ModeLogStream {
		m.logView, _ = m.logView.Update(msg)
	}
	if _, ok := msg.(tui.NewLogFileMsg); ok && m.mode != ModeLogStream {
		m.logView, _ = m.logView.Update(msg)
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

func (m Model) updateDashboard(msg tea.Msg) (Model, tea.Cmd) {
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

func (m Model) updateNodeDetail(msg tea.Msg) (Model, tea.Cmd) {
	switch msg.(type) {
	case tea.KeyPressMsg:
		var cmd tea.Cmd
		m.nodeDetail, cmd = m.nodeDetail.Update(msg)
		return m, cmd
	}
	return m, nil
}

func (m Model) updateTaskDetail(msg tea.Msg) (Model, tea.Cmd) {
	switch msg.(type) {
	case tea.KeyPressMsg:
		var cmd tea.Cmd
		m.taskDetail, cmd = m.taskDetail.Update(msg)
		return m, cmd
	}
	return m, nil
}

func (m Model) updateLogView(msg tea.Msg) (Model, tea.Cmd) {
	switch msg.(type) {
	case tea.KeyPressMsg, tui.LogLinesMsg, tui.NewLogFileMsg:
		var cmd tea.Cmd
		m.logView, cmd = m.logView.Update(msg)
		return m, cmd
	}
	return m, nil
}

func (m Model) updateInbox(msg tea.Msg) (Model, tea.Cmd) {
	switch msg.(type) {
	case tea.KeyPressMsg, tui.InboxUpdatedMsg:
		var cmd tea.Cmd
		m.inbox, cmd = m.inbox.Update(msg)
		return m, cmd
	}
	return m, nil
}

func (m Model) updatePlaceholder(msg tea.Msg) (Model, tea.Cmd) {
	switch msg.(type) {
	case tea.KeyPressMsg:
		var cmd tea.Cmd
		m.viewport, cmd = m.viewport.Update(msg)
		return m, cmd
	}
	return m, nil
}

// SetSize propagates dimensions to all sub-models.
func (m *Model) SetSize(width, height int) {
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
func (m *Model) SetFocused(focused bool) {
	m.focused = focused
}

// SetMode switches the active sub-view.
func (m *Model) SetMode(mode Mode) {
	m.mode = mode
}

// LoadNodeDetail switches to node detail mode and populates the view.
func (m *Model) LoadNodeDetail(addr string, node *state.NodeState, entry *state.IndexEntry, isTarget bool) {
	m.mode = ModeNodeDetail
	m.nodeDetail.Load(addr, node, entry, isTarget)
}

// LoadTaskDetail switches to task detail mode and populates the view.
func (m *Model) LoadTaskDetail(addr, taskID string, task *state.Task) {
	m.mode = ModeTaskDetail
	m.taskDetail.Load(addr, taskID, task)
}

// SwitchToLogView switches to log stream mode.
func (m *Model) SwitchToLogView() {
	m.mode = ModeLogStream
}

// SwitchToDashboard switches to dashboard mode.
func (m *Model) SwitchToDashboard() {
	m.mode = ModeDashboard
}

// Reset clears all data from the detail sub-views, used when switching
// instances. The dashboard shows a "loading..." placeholder until fresh
// data arrives.
func (m *Model) Reset() {
	m.dashboard.Reset()
	m.nodeDetail = NewNodeModel()
	m.taskDetail = NewTaskModel()
	m.logView = NewLogViewModel()
	m.inbox = NewInboxModel()
	m.mode = ModeDashboard
	m.SetSize(m.width, m.height)
}

// SwitchToInbox switches to inbox mode.
func (m *Model) SwitchToInbox() {
	m.mode = ModeInbox
	m.inbox.SetFocused(m.focused)
}

// LoadInbox updates the inbox model with fresh data.
func (m *Model) LoadInbox(items []state.InboxItem) {
	m.inbox.SetItems(items)
}

// SetDashboardInbox updates the dashboard's cached inbox items for the
// summary line, independent of the inbox detail view.
func (m *Model) SetDashboardInbox(items []state.InboxItem) {
	m.dashboard.SetInbox(items)
}

// SetInboxReadError flags the inbox as unreadable.
func (m *Model) SetInboxReadError(err bool) {
	m.inbox.SetReadError(err)
}

// SearchContent returns the searchable lines for the current detail mode.
// The app uses this when search is activated on the detail pane.
func (m Model) SearchContent() []string {
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
func (m Model) CopyTarget() string {
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
func (m Model) View() string {
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
