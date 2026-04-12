package app

import (
	"fmt"
	"strings"

	"charm.land/bubbles/v2/key"
	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"

	"github.com/dorkusprime/wolfcastle/internal/tui"
)

// DaemonModalModel is a confirmation dialog shown before starting or
// stopping the daemon. It displays contextual state (branch, PID,
// draining status) and waits for Enter to confirm or Esc to cancel.
type DaemonModalModel struct {
	active     bool
	action     string // "start" or "stop"
	isRunning  bool
	isDraining bool
	pid        int
	branch     string
	worktree   string
	width      int
	height     int
}

// Open populates the modal with current daemon state and activates it.
func (m *DaemonModalModel) Open(action string, isRunning, isDraining bool, pid int, branch, worktree string) {
	m.active = true
	m.action = action
	m.isRunning = isRunning
	m.isDraining = isDraining
	m.pid = pid
	m.branch = branch
	m.worktree = worktree
}

// Close deactivates the modal.
func (m *DaemonModalModel) Close() {
	m.active = false
}

// IsActive returns true when the modal is visible and capturing input.
func (m DaemonModalModel) IsActive() bool {
	return m.active
}

// SetSize stores terminal dimensions for centering.
func (m *DaemonModalModel) SetSize(width, height int) {
	m.width = width
	m.height = height
}

// Update handles keypresses while the modal is active. Enter emits
// DaemonConfirmedMsg; Esc deactivates. All other keys are absorbed.
func (m DaemonModalModel) Update(msg tea.Msg) (DaemonModalModel, tea.Cmd) {
	if !m.active {
		return m, nil
	}
	kp, ok := msg.(tea.KeyPressMsg)
	if !ok {
		return m, nil
	}
	switch {
	case key.Matches(kp, confirmKey):
		m.active = false
		return m, func() tea.Msg { return tui.DaemonConfirmedMsg{} }
	case key.Matches(kp, dismissKey):
		m.active = false
		return m, nil
	}
	// Absorb all other keys.
	return m, nil
}

// View renders the modal as a centered overlay.
func (m DaemonModalModel) View() string {
	if !m.active {
		return ""
	}

	overlayW := m.width * 50 / 100
	if overlayW < 40 {
		overlayW = 40
	}
	overlayH := m.height * 40 / 100
	if overlayH < 12 {
		overlayH = 12
	}
	innerW := overlayW - 4 // border + padding
	if innerW < 1 {
		innerW = 1
	}

	title := tui.ModalTitleStyle.Render(strings.ToUpper(m.action + " DAEMON"))

	var body strings.Builder
	body.WriteString(tui.ModalDimStyle.Render(m.bodyText()))

	footer := tui.ModalAccentStyle.Render("[Enter] Confirm") +
		tui.ModalDimStyle.Render("  ") +
		tui.ModalDimStyle.Render("[Esc] Cancel")

	content := title + "\n\n" +
		lipgloss.NewStyle().Width(innerW).Render(body.String()) +
		"\n\n" + footer
	content = fillModalBg(content, innerW)

	box := tui.ModalOverlayStyle.
		Width(overlayW).
		Height(overlayH).
		Padding(1, 2).
		Render(content)

	return lipgloss.Place(m.width, m.height, lipgloss.Center, lipgloss.Center, box)
}

func (m DaemonModalModel) bodyText() string {
	if m.action == "stop" {
		lines := []string{
			fmt.Sprintf("Stop the daemon running on %s?", m.branch),
		}
		if m.worktree != "" {
			lines = append(lines, fmt.Sprintf("Worktree: %s", m.worktree))
		}
		if m.pid > 0 {
			lines = append(lines, fmt.Sprintf("PID: %d", m.pid))
		}
		if m.isDraining {
			lines = append(lines, "")
			lines = append(lines, "The daemon is currently draining. Stopping it will interrupt in-flight work.")
		}
		return strings.Join(lines, "\n")
	}

	return fmt.Sprintf("Start the daemon for %s?\n\nThis will launch a background process that executes the project pipeline.", m.worktree)
}

// Key bindings for the daemon modal.
var (
	confirmKey = key.NewBinding(key.WithKeys("enter"))
	dismissKey = key.NewBinding(key.WithKeys("esc"))
)
