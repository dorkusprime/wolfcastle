package notify

import (
	"strings"
	"time"

	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"
)

const (
	maxQueue     = 5
	dismissAfter = 3 * time.Second
	maxWidth     = 50
)

type toast struct {
	text      string
	createdAt time.Time
	dismissed bool
}

// NotificationModel manages a stack of transient notification toasts.
type NotificationModel struct {
	toasts []toast
	width  int
}

// ToastDismissMsg signals that a specific toast should be marked dismissed.
type ToastDismissMsg struct {
	Index int
}

// NewNotificationModel returns a notification model with no active toasts.
func NewNotificationModel() NotificationModel {
	return NotificationModel{}
}

// Push adds a toast to the queue, trims to maxQueue (dropping oldest), and
// returns a tick command that will dismiss the toast after 3 seconds.
func (m *NotificationModel) Push(text string) tea.Cmd {
	t := toast{
		text:      text,
		createdAt: time.Now(),
	}
	m.toasts = append(m.toasts, t)

	// Trim oldest if we exceed the cap.
	if len(m.toasts) > maxQueue {
		m.toasts = m.toasts[len(m.toasts)-maxQueue:]
	}

	idx := len(m.toasts) - 1
	return tea.Tick(dismissAfter, func(time.Time) tea.Msg {
		return ToastDismissMsg{Index: idx}
	})
}

// Update processes messages directed at the notification model.
func (m NotificationModel) Update(msg tea.Msg) (NotificationModel, tea.Cmd) {
	switch msg := msg.(type) {
	case ToastDismissMsg:
		if msg.Index >= 0 && msg.Index < len(m.toasts) {
			m.toasts[msg.Index].dismissed = true
		}
	}
	return m, nil
}

// SetSize updates the available width for rendering.
func (m *NotificationModel) SetSize(width int) {
	m.width = width
}

// HasToasts reports whether any non-dismissed toasts remain.
func (m NotificationModel) HasToasts() bool {
	for _, t := range m.toasts {
		if !t.dismissed {
			return true
		}
	}
	return false
}

// View renders active toasts stacked from the top, right-aligned for overlay
// in the upper-right corner of the detail pane.
func (m NotificationModel) View() string {
	if !m.HasToasts() {
		return ""
	}

	toastStyle := lipgloss.NewStyle().
		Foreground(lipgloss.Color("15")).
		Background(lipgloss.Color("236")).
		BorderLeft(true).
		BorderStyle(lipgloss.ThickBorder()).
		BorderLeftForeground(lipgloss.Color("1")).
		Padding(0, 1).
		MaxWidth(maxWidth)

	var rendered []string
	for _, t := range m.toasts {
		if t.dismissed {
			continue
		}
		rendered = append(rendered, toastStyle.Render(t.text))
	}

	joined := strings.Join(rendered, "\n")

	if m.width > 0 {
		return lipgloss.NewStyle().Width(m.width).Align(lipgloss.Right).Render(joined)
	}
	return joined
}
