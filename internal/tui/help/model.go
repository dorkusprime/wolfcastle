// Package help renders the scrollable full-screen help overlay listing all key
// bindings grouped by context.
package help

import (
	"fmt"
	"strings"

	"charm.land/bubbles/v2/key"
	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"

	"github.com/dorkusprime/wolfcastle/internal/tui"
)

// Model renders a scrollable help overlay listing key bindings.
type Model struct {
	active  bool
	scroll  int
	width   int
	height  int
	content string // pre-rendered content
	lines   int    // total lines in content
}

// NewModel returns a help overlay ready for use.
func NewModel() Model {
	return Model{}
}

// Update handles messages relevant to the help overlay.
func (m Model) Update(msg tea.Msg) (Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.KeyPressMsg:
		if !m.active {
			return m, nil
		}
		keys := tui.HelpKeyMap
		switch {
		case key.Matches(msg, keys.Dismiss):
			m.active = false
			return m, nil
		case key.Matches(msg, keys.ScrollDown):
			maxScroll := m.maxScroll()
			if m.scroll < maxScroll {
				m.scroll++
			}
			return m, nil
		case key.Matches(msg, keys.ScrollUp):
			if m.scroll > 0 {
				m.scroll--
			}
			return m, nil
		default:
			// Absorb all other keys while the overlay is active.
			return m, nil
		}

	case tea.WindowSizeMsg:
		m.width = msg.Width
		m.height = msg.Height
		m.buildContent()
	}

	return m, nil
}

// Toggle flips the overlay between active and inactive. When activating,
// the scroll position resets and the content is rebuilt.
func (m *Model) Toggle() {
	m.active = !m.active
	if m.active {
		m.scroll = 0
		m.buildContent()
	}
}

// IsActive reports whether the overlay is currently visible.
func (m Model) IsActive() bool {
	return m.active
}

// SetSize updates the terminal dimensions and rebuilds the content.
func (m *Model) SetSize(width, height int) {
	m.width = width
	m.height = height
	m.buildContent()
}

// buildContent generates the formatted help text from the key binding groups.
func (m *Model) buildContent() {
	type binding struct {
		key  string
		desc string
	}
	type section struct {
		title    string
		bindings []binding
	}

	sections := []section{
		{
			title: "Global",
			bindings: []binding{
				{"q", "quit"},
				{"Ctrl+C", "quit"},
				{"d", "dashboard"},
				{"L", "log stream (modal)"},
				{"i", "inbox (modal)"},
				{"t", "toggle tree"},
				{"Tab", "cycle focus"},
				{"R", "refresh"},
				{"?", "help"},
				{"/", "search"},
				{"y", "copy address"},
			},
		},
		{
			title: "Tree Navigation",
			bindings: []binding{
				{"j / \u2193", "move down"},
				{"k / \u2191", "move up"},
				{"Enter / l", "expand / view detail"},
				{"Esc / h", "collapse / back"},
				{"g", "jump to top"},
				{"G", "jump to bottom"},
			},
		},
		{
			title: "Daemon & Tabs",
			bindings: []binding{
				{"s", "start/stop daemon (modal)"},
				{"S", "stop all daemons"},
				{"+", "new tab"},
				{"-", "close tab"},
				{"<", "prev tab"},
				{">", "next tab"},
			},
		},
		{
			title: "Inbox",
			bindings: []binding{
				{"i", "open inbox"},
				{"a", "add item"},
				{"Enter", "submit"},
				{"Esc", "cancel"},
			},
		},
		{
			title: "Log Stream",
			bindings: []binding{
				{"L", "open log stream"},
				{"f", "toggle follow"},
				{"L", "cycle level filter"},
				{"T", "cycle trace filter"},
				{"j / \u2193", "scroll down"},
				{"k / \u2191", "scroll up"},
				{"g", "jump to top"},
				{"G", "jump to bottom"},
				{"Ctrl+D", "half page down"},
				{"Ctrl+U", "half page up"},
			},
		},
		{
			title: "Search",
			bindings: []binding{
				{"/", "start search"},
				{"Enter", "confirm search"},
				{"Esc", "cancel search"},
				{"n", "next match"},
				{"N", "previous match"},
			},
		},
	}

	sectionStyle := lipgloss.NewStyle().Foreground(tui.ColorNeonCyan).Bold(true)
	keyStyle := lipgloss.NewStyle().Foreground(tui.ColorGold)
	descStyle := lipgloss.NewStyle().Foreground(tui.ColorLightGray)

	var b strings.Builder
	for i, sec := range sections {
		if i > 0 {
			b.WriteString("\n")
		}
		b.WriteString(sectionStyle.Render(sec.title))
		b.WriteString("\n")
		for _, kb := range sec.bindings {
			fmt.Fprintf(&b, "  %s%s\n",
				keyStyle.Render(fmt.Sprintf("%-12s", kb.key)),
				descStyle.Render(kb.desc))
		}
	}

	m.content = strings.TrimRight(b.String(), "\n")
	m.lines = strings.Count(m.content, "\n") + 1
}

// View renders the help overlay centered on the terminal.
func (m Model) View() string {
	if !m.active {
		return ""
	}

	// Overlay dimensions: 60% width (min 40), 80% height (min 20).
	overlayW := m.width * 60 / 100
	if overlayW < 40 {
		overlayW = 40
	}
	overlayH := m.height * 80 / 100
	if overlayH < 20 {
		overlayH = 20
	}

	// The border and padding consume space from the inner area.
	// RoundedBorder adds 1 cell per side (2 horizontal, 2 vertical).
	// We add 1 cell horizontal padding on each side for breathing room.
	innerW := overlayW - 4 // 2 border + 2 padding
	if innerW < 10 {
		innerW = 10
	}
	innerH := overlayH - 4 // 2 border + 2 for title/subtitle lines

	titleStyle := tui.HelpTitleStyle.Width(innerW).Align(lipgloss.Center)
	title := titleStyle.Render("WOLFCASTLE KEY BINDINGS")
	subtitle := lipgloss.NewStyle().
		Width(innerW).
		Align(lipgloss.Center).
		Foreground(tui.ColorDimWhite).
		Render("Press ? or Esc to close.")

	headerLines := strings.Count(title, "\n") + 1 + strings.Count(subtitle, "\n") + 1 + 1 // +1 for blank separator
	visibleH := innerH - headerLines
	if visibleH < 1 {
		visibleH = 1
	}

	// Clamp scroll.
	maxScroll := m.lines - visibleH
	if maxScroll < 0 {
		maxScroll = 0
	}
	if m.scroll > maxScroll {
		m.scroll = maxScroll
	}

	// Slice visible lines from the content.
	allLines := strings.Split(m.content, "\n")
	end := m.scroll + visibleH
	if end > len(allLines) {
		end = len(allLines)
	}
	visible := allLines[m.scroll:end]

	// Pad short content so the box stays a consistent size.
	for len(visible) < visibleH {
		visible = append(visible, "")
	}

	body := strings.Join(visible, "\n")

	inner := title + "\n" + subtitle + "\n\n" + body

	box := tui.HelpOverlayStyle.
		Width(overlayW).
		Height(overlayH).
		Padding(0, 1).
		Render(inner)

	return lipgloss.Place(m.width, m.height, lipgloss.Center, lipgloss.Center, box)
}

// maxScroll returns the highest valid scroll offset.
func (m Model) maxScroll() int {
	overlayH := m.height * 80 / 100
	if overlayH < 20 {
		overlayH = 20
	}
	innerH := overlayH - 4

	titleLines := 3 // title + subtitle + blank line
	visibleH := innerH - titleLines
	if visibleH < 1 {
		visibleH = 1
	}

	max := m.lines - visibleH
	if max < 0 {
		return 0
	}
	return max
}
