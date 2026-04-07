package detail

import (
	"fmt"
	"image/color"
	"strings"
	"time"

	"charm.land/bubbles/v2/textinput"
	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"

	"github.com/dorkusprime/wolfcastle/internal/state"
	"github.com/dorkusprime/wolfcastle/internal/tui"
)

// InboxModel displays and manages the inbox view inside the detail pane.
// Items are rendered as a scrollable list with a text input for adding new
// entries. The model never writes to the Store directly; it emits an
// AddInboxItemCmd that the app orchestrator handles.
type InboxModel struct {
	items     []state.InboxItem
	cursor    int
	input     textinput.Model
	inputMode bool
	width     int
	height    int
	focused   bool
	readErr   bool
	scrollTop int
}

// NewInboxModel creates an InboxModel with a configured text input.
func NewInboxModel() InboxModel {
	ti := textinput.New()
	ti.Prompt = "> "
	ti.Placeholder = ""
	ti.CharLimit = 0
	return InboxModel{input: ti}
}

// Update handles messages routed to the inbox sub-view.
func (m InboxModel) Update(msg tea.Msg) (InboxModel, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.KeyPressMsg:
		if m.inputMode {
			return m.updateInputMode(msg)
		}
		return m.updateNormalMode(msg)

	case tui.InboxUpdatedMsg:
		if msg.Inbox != nil {
			m.SetItems(msg.Inbox.Items)
		}
		return m, nil
	}
	return m, nil
}

func (m InboxModel) updateInputMode(msg tea.KeyPressMsg) (InboxModel, tea.Cmd) {
	switch msg.String() {
	case "enter":
		text := strings.TrimSpace(m.input.Value())
		if text == "" {
			m.inputMode = false
			m.input.Blur()
			return m, nil
		}
		m.inputMode = false
		m.input.SetValue("")
		m.input.Blur()
		return m, func() tea.Msg {
			return tui.AddInboxItemCmd{Text: text}
		}
	case "esc":
		m.inputMode = false
		m.input.SetValue("")
		m.input.Blur()
		return m, nil
	default:
		var cmd tea.Cmd
		m.input, cmd = m.input.Update(msg)
		return m, cmd
	}
}

func (m InboxModel) updateNormalMode(msg tea.KeyPressMsg) (InboxModel, tea.Cmd) {
	switch msg.String() {
	case "j", "down":
		if len(m.items) > 0 && m.cursor < len(m.items)-1 {
			m.cursor++
			m.ensureVisible()
		}
		return m, nil
	case "k", "up":
		if m.cursor > 0 {
			m.cursor--
			m.ensureVisible()
		}
		return m, nil
	case "a":
		m.inputMode = true
		m.input.Focus()
		return m, nil
	}
	return m, nil
}

// SetItems replaces the items list and clamps the cursor.
func (m *InboxModel) SetItems(items []state.InboxItem) {
	m.items = items
	m.readErr = false
	if m.cursor >= len(m.items) {
		if len(m.items) > 0 {
			m.cursor = len(m.items) - 1
		} else {
			m.cursor = 0
		}
	}
}

// SetSize updates the rendering dimensions.
func (m *InboxModel) SetSize(width, height int) {
	m.width = width
	m.height = height
}

// SetFocused marks whether this model holds keyboard focus.
func (m *InboxModel) SetFocused(focused bool) {
	m.focused = focused
}

// SetReadError flags an unreadable inbox so the view can show an error state.
func (m *InboxModel) SetReadError(err bool) {
	m.readErr = err
}

// Items returns the current inbox items.
func (m InboxModel) Items() []state.InboxItem {
	return m.items
}

// SelectedText returns the text of the currently selected item, or empty.
func (m InboxModel) SelectedText() string {
	if len(m.items) == 0 || m.cursor >= len(m.items) {
		return ""
	}
	return m.items[m.cursor].Text
}

// SearchContent returns item texts and statuses for search matching.
func (m InboxModel) SearchContent() []string {
	out := make([]string, len(m.items))
	for i, item := range m.items {
		out[i] = item.Text + " " + string(item.Status)
	}
	return out
}

// ensureVisible adjusts scrollTop so the cursor stays in view.
func (m *InboxModel) ensureVisible() {
	visible := m.visibleItemCount()
	if visible <= 0 {
		return
	}
	if m.cursor < m.scrollTop {
		m.scrollTop = m.cursor
	}
	if m.cursor >= m.scrollTop+visible {
		m.scrollTop = m.cursor - visible + 1
	}
}

// visibleItemCount returns how many item lines fit in the viewport, reserving
// space for the header (2 lines), bottom separator + input area (2 lines).
func (m InboxModel) visibleItemCount() int {
	reserved := 4 // header line, blank line, separator, input line
	avail := m.height - reserved
	if avail < 1 {
		return 1
	}
	return avail
}

// View renders the inbox pane.
func (m InboxModel) View() string {
	if m.readErr {
		return m.renderError()
	}

	var b strings.Builder

	// Header
	heading := lipgloss.NewStyle().
		Foreground(tui.ColorWhite).
		Bold(true)
	count := fmt.Sprintf("INBOX  %d items", len(m.items))
	b.WriteString(heading.Render(count))
	b.WriteString("\n")

	if len(m.items) == 0 {
		b.WriteString("\n")
		dim := lipgloss.NewStyle().Foreground(tui.ColorDimWhite).Italic(true)
		b.WriteString(dim.Render("Inbox empty. The silence is temporary."))
		b.WriteString("\n")
	} else {
		b.WriteString("\n")
		visible := m.visibleItemCount()
		end := m.scrollTop + visible
		if end > len(m.items) {
			end = len(m.items)
		}
		for i := m.scrollTop; i < end; i++ {
			b.WriteString(m.renderItem(i))
			b.WriteString("\n")
		}
	}

	// Separator
	sepWidth := m.width
	if sepWidth < 10 {
		sepWidth = 40
	}
	sep := lipgloss.NewStyle().Foreground(tui.ColorDimGray)
	b.WriteString(sep.Render(strings.Repeat("─", sepWidth)))
	b.WriteString("\n")

	// Input area
	if m.inputMode {
		b.WriteString(m.input.View())
	} else {
		hint := lipgloss.NewStyle().Foreground(tui.ColorDimWhite).Italic(true)
		b.WriteString(hint.Render("Press [a] to add an item"))
	}

	return b.String()
}

func (m InboxModel) renderItem(idx int) string {
	item := m.items[idx]
	selected := idx == m.cursor

	// Glyph and color based on status.
	var glyph string
	var glyphColor color.Color
	switch item.Status {
	case state.InboxFiled:
		glyph = "●"
		glyphColor = tui.ColorGreen
	default: // InboxNew or anything else
		glyph = "○"
		glyphColor = tui.ColorDimWhite
	}

	glyphStyle := lipgloss.NewStyle().Foreground(glyphColor)

	// Status label.
	statusLabel := string(item.Status)
	if statusLabel == "" {
		statusLabel = "new"
	}

	// Relative time from timestamp.
	age := inboxRelativeTime(item.Timestamp)

	// Right-side info: status + time.
	rightInfo := fmt.Sprintf("%s  %s", statusLabel, age)

	// Calculate text width: total - glyph(2) - padding(2) - right info.
	textWidth := m.width - 4 - len(rightInfo)
	if textWidth < 10 {
		textWidth = 10
	}

	text := item.Text
	if len(text) > textWidth {
		text = text[:textWidth-1] + "…"
	}

	// Pad text to fill available space.
	if len(text) < textWidth {
		text += strings.Repeat(" ", textWidth-len(text))
	}

	line := fmt.Sprintf("  %s %s  %s", glyphStyle.Render(glyph), text, rightInfo)

	if selected && m.focused {
		selStyle := lipgloss.NewStyle().
			Foreground(tui.ColorWhite).
			Background(tui.ColorDarkGray).
			Bold(true)
		return selStyle.Render(line)
	}

	return line
}

func (m InboxModel) renderError() string {
	heading := lipgloss.NewStyle().
		Foreground(tui.ColorWhite).
		Bold(true)

	var b strings.Builder
	b.WriteString(heading.Render("INBOX"))
	b.WriteString("\n\n")

	errStyle := lipgloss.NewStyle().Foreground(tui.ColorRed)
	b.WriteString(errStyle.Render("Inbox unreadable. Run wolfcastle doctor."))
	return b.String()
}

// relativeTime parses an RFC3339 timestamp and returns a human-friendly
// relative duration like "2m ago" or "3h ago". Falls back to the raw string
// if parsing fails.
func inboxRelativeTime(ts string) string {
	t, err := time.Parse(time.RFC3339, ts)
	if err != nil {
		return ts
	}
	d := time.Since(t)
	switch {
	case d < time.Minute:
		return "just now"
	case d < time.Hour:
		return fmt.Sprintf("%dm ago", int(d.Minutes()))
	case d < 24*time.Hour:
		return fmt.Sprintf("%dh ago", int(d.Hours()))
	default:
		return fmt.Sprintf("%dd ago", int(d.Hours()/24))
	}
}
