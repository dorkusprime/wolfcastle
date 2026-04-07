package detail

import (
	"charm.land/bubbles/v2/viewport"
	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"

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
// active sub-view. Phase 1 implements only the dashboard; other modes show
// placeholder text until their phases land.
type DetailModel struct {
	mode      DetailMode
	dashboard DashboardModel
	// Phase 2+: nodeDetail, taskDetail, logView
	// Phase 4: inbox
	viewport viewport.Model
	width    int
	height   int
	focused  bool
}

var placeholderStyle = lipgloss.NewStyle().
	Foreground(tui.ColorDimWhite).
	Italic(true)

const placeholderText = "Coming in the next phase."

// NewDetailModel creates a DetailModel starting in dashboard mode.
func NewDetailModel() DetailModel {
	vp := viewport.New()
	return DetailModel{
		mode:      ModeDashboard,
		dashboard: NewDashboardModel(),
		viewport:  vp,
	}
}

// Update routes messages to the active sub-view.
func (m DetailModel) Update(msg tea.Msg) (DetailModel, tea.Cmd) {
	switch m.mode {
	case ModeDashboard:
		return m.updateDashboard(msg)
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

func (m DetailModel) updatePlaceholder(msg tea.Msg) (DetailModel, tea.Cmd) {
	switch msg.(type) {
	case tea.KeyPressMsg:
		var cmd tea.Cmd
		m.viewport, cmd = m.viewport.Update(msg)
		return m, cmd
	}
	return m, nil
}

// SetSize propagates dimensions to the dashboard and viewport.
func (m *DetailModel) SetSize(width, height int) {
	m.width = width
	m.height = height
	m.dashboard.SetSize(width, height)
	m.viewport.SetWidth(width)
	m.viewport.SetHeight(height)
}

// SetFocused marks whether this container currently holds keyboard focus.
func (m *DetailModel) SetFocused(focused bool) {
	m.focused = focused
}

// SetMode switches the active sub-view. When switching to a placeholder mode,
// the viewport content is set to the placeholder string.
func (m *DetailModel) SetMode(mode DetailMode) {
	m.mode = mode
	if mode != ModeDashboard {
		m.viewport.SetContent(placeholderStyle.Render(placeholderText))
	}
}

// View renders the active sub-view.
func (m DetailModel) View() string {
	if m.mode == ModeDashboard {
		return m.dashboard.View()
	}
	return m.viewport.View()
}
