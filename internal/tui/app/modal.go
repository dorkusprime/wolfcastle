package app

import (
	"strings"

	"charm.land/bubbles/v2/key"
	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"
	uv "github.com/charmbracelet/ultraviolet"

	"github.com/dorkusprime/wolfcastle/internal/tui"
)

// fillModalBg stamps the overlay background color onto every cell of
// the rendered content that doesn't already have an explicit background.
// This operates at the cell level (via lipgloss Canvas + ultraviolet),
// so ANSI resets between styled spans no longer punch transparent holes.
func fillModalBg(content string, width int) string {
	lines := strings.Split(content, "\n")
	height := len(lines)
	if height == 0 || width == 0 {
		return content
	}

	canvas := lipgloss.NewCanvas(width, height)
	ss := uv.NewStyledString(content)
	canvas.Compose(ss)

	bg := tui.ColorOverlayBg
	bounds := canvas.Bounds()
	for y := bounds.Min.Y; y < bounds.Max.Y; y++ {
		for x := bounds.Min.X; x < bounds.Max.X; x++ {
			c := canvas.CellAt(x, y)
			if c == nil {
				canvas.SetCell(x, y, &uv.Cell{
					Content: " ",
					Width:   1,
					Style:   uv.Style{Bg: bg},
				})
			} else if c.Style.Bg == nil {
				c.Style.Bg = bg
			}
		}
	}

	return canvas.Render()
}

// fillBaseBg stamps the base background (near-black) onto every cell
// of the full-screen view that doesn't already have a background.
// This ensures the TUI is readable on light terminals without
// breaking transparency on dark ones.
func fillBaseBg(content string, width, height int) string {
	if height == 0 || width == 0 {
		return content
	}

	canvas := lipgloss.NewCanvas(width, height)
	ss := uv.NewStyledString(content)
	canvas.Compose(ss)

	bg := tui.ColorBaseBg
	bounds := canvas.Bounds()
	for y := bounds.Min.Y; y < bounds.Max.Y; y++ {
		for x := bounds.Min.X; x < bounds.Max.X; x++ {
			c := canvas.CellAt(x, y)
			if c == nil {
				canvas.SetCell(x, y, &uv.Cell{
					Content: " ",
					Width:   1,
					Style:   uv.Style{Bg: bg},
				})
			} else if c.Style.Bg == nil {
				c.Style.Bg = bg
			}
		}
	}

	return canvas.Render()
}

// ActiveModal tracks which modal overlay (if any) is currently visible.
// Only one modal can be open at a time; the enum enforces this
// structurally rather than relying on runtime checks.
type ActiveModal int

const (
	ModalNone ActiveModal = iota
	ModalInbox
	ModalLog
	ModalDaemon
	ModalNewTab
)

func (m TUIModel) isModalActive() bool {
	return m.activeModal != ModalNone
}

func (m *TUIModel) closeModal() {
	// Restore sub-model dimensions that may have been resized for the
	// modal. propagateSize will fix them on the next frame, but calling
	// it here avoids a single-frame glitch.
	m.activeModal = ModalNone
	m.propagateSize()
}

// updateActiveModal dispatches keypresses to the active modal's sub-model.
// Every branch must return early so keys never leak to tree/detail routing.
func (m TUIModel) updateActiveModal(msg tea.KeyPressMsg) (TUIModel, tea.Cmd) {
	switch m.activeModal {
	case ModalInbox:
		return m.updateInboxModal(msg)
	case ModalLog:
		return m.updateLogModal(msg)
	case ModalDaemon:
		dm, cmd := m.daemonModal.Update(msg)
		m.daemonModal = dm
		if !m.daemonModal.IsActive() {
			m.activeModal = ModalNone
		}
		return m, cmd
	case ModalNewTab:
		return m.updateNewTabModal(msg)
	}
	return m, nil
}

func (m TUIModel) updateNewTabModal(msg tea.KeyPressMsg) (TUIModel, tea.Cmd) {
	picker, cmd := m.tabPicker.Update(msg)
	m.tabPicker = picker
	return m, cmd
}

func (m TUIModel) updateInboxModal(msg tea.KeyPressMsg) (TUIModel, tea.Cmd) {
	tab := m.activeTab()
	if tab == nil {
		return m, nil
	}
	inbox := tab.Detail.InboxModelRef()

	if inbox.IsInputActive() {
		updated, cmd := inbox.Update(msg)
		*inbox = updated
		return m, cmd
	}

	if key.Matches(msg, dismissKey) {
		m.closeModal()
		return m, nil
	}

	updated, cmd := inbox.Update(msg)
	*inbox = updated
	return m, cmd
}

func (m TUIModel) updateLogModal(msg tea.KeyPressMsg) (TUIModel, tea.Cmd) {
	if key.Matches(msg, dismissKey) {
		m.closeModal()
		return m, nil
	}

	tab := m.activeTab()
	if tab == nil {
		return m, nil
	}
	logView := tab.Detail.LogViewModelRef()
	updated, cmd := logView.Update(msg)
	*logView = updated
	return m, cmd
}

// renderActiveModal renders the active modal as a centered overlay that
// replaces the content area. Each modal computes its own preferred
// dimensions, sizes the sub-model accordingly, and wraps the result in
// the standard modal chrome.
func (m TUIModel) renderActiveModal(contentHeight int) string {
	switch m.activeModal {
	case ModalInbox:
		return m.renderInboxModal(contentHeight)
	case ModalLog:
		return m.renderLogModal(contentHeight)
	case ModalDaemon:
		return m.daemonModal.View()
	case ModalNewTab:
		return m.renderNewTabModal(contentHeight)
	}
	return ""
}

func (m TUIModel) renderNewTabModal(contentHeight int) string {
	overlayW := m.width * 60 / 100
	if overlayW < 50 {
		overlayW = 50
	}
	overlayH := contentHeight * 80 / 100
	if overlayH < 15 {
		overlayH = 15
	}
	if overlayH > contentHeight {
		overlayH = contentHeight
	}

	innerW := overlayW - 6
	picker := m.tabPicker
	picker.SetSize(innerW, overlayH-4)
	content := picker.View()
	content = fillModalBg(content, innerW)

	box := tui.ModalOverlayStyle.
		Width(overlayW).
		Height(overlayH).
		Padding(1, 2).
		Render(content)

	return lipgloss.Place(m.width, contentHeight, lipgloss.Center, lipgloss.Center, box)
}

func (m TUIModel) renderInboxModal(contentHeight int) string {
	tab := m.activeTab()
	if tab == nil {
		return ""
	}
	overlayW := m.width * 60 / 100
	if overlayW < 40 {
		overlayW = 40
	}
	overlayH := contentHeight * 80 / 100
	if overlayH < 20 {
		overlayH = 20
	}
	if overlayH > contentHeight {
		overlayH = contentHeight
	}
	innerW := overlayW - 6
	innerH := overlayH - 4
	if innerW < 1 {
		innerW = 1
	}
	if innerH < 1 {
		innerH = 1
	}

	inbox := tab.Detail.InboxModelRef()
	inbox.SetSize(innerW, innerH)
	inbox.SetFocused(true)

	content := inbox.View()

	hint := strings.Repeat(" ", 2) + tui.ModalDimStyle.Render("[Esc] Close")
	content += "\n" + hint
	content = fillModalBg(content, innerW)

	box := tui.ModalOverlayStyle.
		Width(overlayW).
		Height(overlayH).
		Padding(1, 2).
		Render(content)

	return lipgloss.Place(m.width, contentHeight, lipgloss.Center, lipgloss.Center, box)
}

func (m TUIModel) renderLogModal(contentHeight int) string {
	tab := m.activeTab()
	if tab == nil {
		return ""
	}
	overlayW := m.width * 80 / 100
	if overlayW < 60 {
		overlayW = 60
	}
	overlayH := contentHeight * 90 / 100
	if overlayH < 20 {
		overlayH = 20
	}
	if overlayH > contentHeight {
		overlayH = contentHeight
	}
	innerW := overlayW - 6
	innerH := overlayH - 4
	if innerW < 1 {
		innerW = 1
	}
	if innerH < 1 {
		innerH = 1
	}

	logView := tab.Detail.LogViewModelRef()
	logView.SetSize(innerW, innerH)
	logView.SetFocused(true)

	content := logView.View()

	hint := strings.Repeat(" ", 2) + tui.ModalDimStyle.Render("[Esc] Close")
	content += "\n" + hint
	content = fillModalBg(content, innerW)

	box := tui.ModalOverlayStyle.
		Width(overlayW).
		Height(overlayH).
		Padding(1, 2).
		Render(content)

	return lipgloss.Place(m.width, contentHeight, lipgloss.Center, lipgloss.Center, box)
}
