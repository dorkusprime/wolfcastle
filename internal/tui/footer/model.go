// Package footer renders the bottom status bar showing context-sensitive
// key hints and daemon status.
package footer

import (
	"strings"

	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"

	"github.com/dorkusprime/wolfcastle/internal/tui"
)

// FooterModel renders a single-line key-hint bar at the bottom of the TUI.
type FooterModel struct {
	focusedPane   int // 0=PaneTree, 1=PaneDetail
	detailMode    int // 0=Dashboard, 1=NodeDetail, 2=TaskDetail, 3=LogStream, 4=Inbox
	daemonRunning bool
	width         int
}

// NewFooterModel returns a zero-value footer ready for use.
func NewFooterModel() FooterModel {
	return FooterModel{}
}

// Update handles messages relevant to the footer.
func (m FooterModel) Update(msg tea.Msg) (FooterModel, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.width = msg.Width

	case tui.DaemonStatusMsg:
		m.daemonRunning = msg.IsRunning
	}
	return m, nil
}

// SetFocus updates which pane is focused.
func (m *FooterModel) SetFocus(pane int) {
	m.focusedPane = pane
}

// SetDetailMode updates the current detail-pane mode.
func (m *FooterModel) SetDetailMode(mode int) {
	m.detailMode = mode
}

// SetSize updates the available width for rendering.
func (m *FooterModel) SetSize(width int) {
	m.width = width
}

// View renders the footer as a single line of key hints.
func (m FooterModel) View() string {
	style := lipgloss.NewStyle().Foreground(tui.ColorDimWhite)

	// Build hints in priority order (highest priority first).
	// When the line is too wide we drop from the tail.
	type hint struct {
		key  string
		desc string
	}

	hints := []hint{
		{"q", "quit"},
		{"Tab", "focus"},
		{"d", "dashboard"},
	}

	// Mode-specific bindings
	if m.daemonRunning {
		hints = append(hints, hint{"s", "stop"})
	} else {
		hints = append(hints, hint{"s", "start"})
	}
	hints = append(hints,
		hint{"<>", "switch"},
		hint{"i", "inbox"},
		hint{"L", "logs"},
		hint{"t", "tree"},
		hint{"/", "search"},
		hint{"y", "copy"},
	)

	// Lower priority
	hints = append(hints, hint{"?", "help"})
	hints = append(hints, hint{"R", "refresh"})

	// Render hints as "[key] desc" separated by spaces, truncating from the
	// right when the line would exceed the available width.
	rendered := make([]string, 0, len(hints))
	total := 0
	for _, h := range hints {
		piece := "[" + h.key + "] " + h.desc
		needed := len(piece)
		if total > 0 {
			needed += 2 // account for "  " separator
		}
		if m.width > 0 && total+needed > m.width {
			break
		}
		rendered = append(rendered, piece)
		total += needed
	}

	return style.Render(strings.Join(rendered, "  "))
}
