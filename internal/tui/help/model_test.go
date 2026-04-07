package help

import (
	"strings"
	"testing"

	tea "charm.land/bubbletea/v2"
)

func TestNewHelpOverlayModel(t *testing.T) {
	m := NewHelpOverlayModel()
	if m.IsActive() {
		t.Error("new model should not be active")
	}
}

func TestToggleActivates(t *testing.T) {
	m := NewHelpOverlayModel()
	m.SetSize(120, 40)
	m.Toggle()
	if !m.IsActive() {
		t.Error("expected active after first Toggle")
	}
	if m.scroll != 0 {
		t.Errorf("scroll = %d, want 0 after activation", m.scroll)
	}
}

func TestToggleDeactivates(t *testing.T) {
	m := NewHelpOverlayModel()
	m.Toggle()
	m.Toggle()
	if m.IsActive() {
		t.Error("expected inactive after second Toggle")
	}
}

func TestToggleResetsScroll(t *testing.T) {
	m := NewHelpOverlayModel()
	m.SetSize(120, 40)
	m.Toggle()
	m.scroll = 5
	m.Toggle() // deactivate
	m.Toggle() // reactivate
	if m.scroll != 0 {
		t.Errorf("scroll = %d, want 0 after re-Toggle", m.scroll)
	}
}

func TestIsActive(t *testing.T) {
	m := NewHelpOverlayModel()
	if m.IsActive() {
		t.Error("should start inactive")
	}
	m.Toggle()
	if !m.IsActive() {
		t.Error("should be active after Toggle")
	}
}

func TestSetSize(t *testing.T) {
	m := NewHelpOverlayModel()
	m.SetSize(100, 50)
	if m.width != 100 {
		t.Errorf("width = %d, want 100", m.width)
	}
	if m.height != 50 {
		t.Errorf("height = %d, want 50", m.height)
	}
	// Content should be built.
	if m.content == "" {
		t.Error("content should be populated after SetSize")
	}
}

func TestUpdateDismissWithQuestion(t *testing.T) {
	m := NewHelpOverlayModel()
	m.SetSize(120, 40)
	m.Toggle()

	m, _ = m.Update(tea.KeyPressMsg{Code: '?', Text: "?"})
	if m.IsActive() {
		t.Error("? should dismiss the overlay")
	}
}

func TestUpdateDismissWithEsc(t *testing.T) {
	m := NewHelpOverlayModel()
	m.SetSize(120, 40)
	m.Toggle()

	m, _ = m.Update(tea.KeyPressMsg{Code: tea.KeyEscape})
	if m.IsActive() {
		t.Error("Esc should dismiss the overlay")
	}
}

func TestUpdateScrollDown(t *testing.T) {
	m := NewHelpOverlayModel()
	// Use a small height to ensure scrollable content.
	m.SetSize(80, 25)
	m.Toggle()

	before := m.scroll
	m, _ = m.Update(tea.KeyPressMsg{Code: 'j', Text: "j"})
	if m.scroll <= before && m.maxScroll() > 0 {
		t.Error("j should scroll down when content overflows")
	}
}

func TestUpdateScrollUp(t *testing.T) {
	m := NewHelpOverlayModel()
	m.SetSize(80, 25)
	m.Toggle()
	m.scroll = 3

	m, _ = m.Update(tea.KeyPressMsg{Code: 'k', Text: "k"})
	if m.scroll != 2 {
		t.Errorf("scroll = %d, want 2 after k", m.scroll)
	}
}

func TestUpdateScrollUpClampsAtZero(t *testing.T) {
	m := NewHelpOverlayModel()
	m.SetSize(80, 40)
	m.Toggle()
	m.scroll = 0

	m, _ = m.Update(tea.KeyPressMsg{Code: 'k', Text: "k"})
	if m.scroll != 0 {
		t.Errorf("scroll = %d, want 0 (clamped)", m.scroll)
	}
}

func TestUpdateAbsorbsOtherKeys(t *testing.T) {
	m := NewHelpOverlayModel()
	m.SetSize(120, 40)
	m.Toggle()

	// A random key should be absorbed (overlay stays active, no pass-through).
	m, cmd := m.Update(tea.KeyPressMsg{Code: 'x', Text: "x"})
	if !m.IsActive() {
		t.Error("overlay should remain active for unrecognized keys")
	}
	if cmd != nil {
		t.Error("absorbed keys should produce nil cmd")
	}
}

func TestUpdateInactiveIgnoresKeys(t *testing.T) {
	m := NewHelpOverlayModel()
	m.SetSize(120, 40)

	m, cmd := m.Update(tea.KeyPressMsg{Code: 'j', Text: "j"})
	if m.IsActive() {
		t.Error("should stay inactive")
	}
	if cmd != nil {
		t.Error("inactive overlay should produce nil cmd")
	}
}

func TestUpdateWindowSizeMsg(t *testing.T) {
	m := NewHelpOverlayModel()
	m, _ = m.Update(tea.WindowSizeMsg{Width: 200, Height: 60})
	if m.width != 200 || m.height != 60 {
		t.Errorf("size = %dx%d, want 200x60", m.width, m.height)
	}
}

func TestViewInactive(t *testing.T) {
	m := NewHelpOverlayModel()
	if m.View() != "" {
		t.Error("inactive overlay should render empty string")
	}
}

func TestViewContainsTitle(t *testing.T) {
	m := NewHelpOverlayModel()
	m.SetSize(120, 50)
	m.Toggle()
	v := m.View()
	if !strings.Contains(v, "WOLFCASTLE KEY BINDINGS") {
		t.Error("view should contain title 'WOLFCASTLE KEY BINDINGS'")
	}
}

func TestViewContainsKeyGroups(t *testing.T) {
	m := NewHelpOverlayModel()
	m.SetSize(120, 50)
	m.Toggle()
	v := m.View()
	for _, group := range []string{"Global", "Tree Navigation", "Search"} {
		if !strings.Contains(v, group) {
			t.Errorf("view should contain section %q", group)
		}
	}
}

func TestViewContainsBindings(t *testing.T) {
	m := NewHelpOverlayModel()
	m.SetSize(120, 50)
	m.Toggle()
	v := m.View()
	for _, binding := range []string{"quit", "search", "next match", "expand"} {
		if !strings.Contains(v, binding) {
			t.Errorf("view should contain binding description %q", binding)
		}
	}
}

func TestViewDimensions(t *testing.T) {
	w, h := 120, 50
	m := NewHelpOverlayModel()
	m.SetSize(w, h)
	m.Toggle()
	v := m.View()

	// The full rendered output should be placed in the terminal-sized area.
	lines := strings.Split(v, "\n")
	if len(lines) != h {
		t.Errorf("view lines = %d, want %d (full terminal height)", len(lines), h)
	}
}

func TestScrollClampAtBottom(t *testing.T) {
	m := NewHelpOverlayModel()
	m.SetSize(80, 25) // small viewport forces scrollable content
	m.Toggle()

	// Scroll well past the end.
	for i := 0; i < 100; i++ {
		m, _ = m.Update(tea.KeyPressMsg{Code: 'j', Text: "j"})
	}

	max := m.maxScroll()
	if m.scroll > max {
		t.Errorf("scroll = %d exceeds maxScroll = %d", m.scroll, max)
	}
}

func TestScrollDownClampsAtMax(t *testing.T) {
	m := NewHelpOverlayModel()
	m.SetSize(80, 25)
	m.Toggle()

	max := m.maxScroll()
	m.scroll = max
	m, _ = m.Update(tea.KeyPressMsg{Code: 'j', Text: "j"})
	if m.scroll != max {
		t.Errorf("scroll = %d, should stay at max %d", m.scroll, max)
	}
}

func TestViewCloseHint(t *testing.T) {
	m := NewHelpOverlayModel()
	m.SetSize(120, 50)
	m.Toggle()
	v := m.View()
	if !strings.Contains(v, "Press ? or Esc to close") {
		t.Error("view should contain close hint")
	}
}
