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
	maxWidth     = 60
)

type toast struct {
	id        int
	text      string
	createdAt time.Time
	dismissed bool
}

// NotificationModel manages a stack of transient notification toasts.
type NotificationModel struct {
	toasts []toast
	nextID int
	width  int
}

// ToastDismissMsg signals that a specific toast should be dismissed by ID.
type ToastDismissMsg struct {
	ID int
}

// NewNotificationModel returns a notification model with no active toasts.
func NewNotificationModel() NotificationModel {
	return NotificationModel{}
}

// Push adds a toast to the queue, trims to maxQueue (dropping oldest), and
// returns a tick command that will dismiss the toast after 3 seconds.
// If the text contains a colon, the part after the colon is front-truncated
// so the label stays intact and the most specific part of the value is visible.
func (m *NotificationModel) Push(text string) tea.Cmd {
	limit := maxWidth - 5 // account for padding/border
	if len(text) > limit {
		if idx := strings.Index(text, ": "); idx >= 0 && idx < limit {
			prefix := text[:idx+2]
			value := text[idx+2:]
			valueLimit := limit - len(prefix)
			if valueLimit > 3 && len(value) > valueLimit {
				value = "..." + value[len(value)-valueLimit+3:]
			}
			text = prefix + value
		} else {
			text = "..." + text[len(text)-limit+3:]
		}
	}

	id := m.nextID
	m.nextID++
	t := toast{
		id:        id,
		text:      text,
		createdAt: time.Now(),
	}
	m.toasts = append(m.toasts, t)

	// Trim oldest if we exceed the cap.
	if len(m.toasts) > maxQueue {
		m.toasts = m.toasts[len(m.toasts)-maxQueue:]
	}

	return tea.Tick(dismissAfter, func(time.Time) tea.Msg {
		return ToastDismissMsg{ID: id}
	})
}

// Update processes messages directed at the notification model.
func (m NotificationModel) Update(msg tea.Msg) (NotificationModel, tea.Cmd) {
	switch msg := msg.(type) {
	case ToastDismissMsg:
		for i := range m.toasts {
			if m.toasts[i].id == msg.ID {
				m.toasts[i].dismissed = true
				break
			}
		}
		// Prune dismissed toasts from the front.
		for len(m.toasts) > 0 && m.toasts[0].dismissed {
			m.toasts = m.toasts[1:]
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
		BorderForeground(lipgloss.Color("1")).
		PaddingLeft(1).
		PaddingRight(1).
		MaxWidth(maxWidth)

	var rendered []string
	for _, t := range m.toasts {
		if t.dismissed {
			continue
		}
		rendered = append(rendered, toastStyle.Render(t.text))
	}

	return strings.Join(rendered, "\n")
}
