package help

import (
	"strings"
	"testing"

	tea "charm.land/bubbletea/v2"
)

func TestHelpContentIncludesDaemonControl(t *testing.T) {
	m := NewModel()
	m.SetSize(120, 80) // tall enough to show all sections without scrolling
	m.Toggle()         // activate to build content

	view := m.View()

	if !strings.Contains(view, "Daemon Control") {
		t.Error("help overlay should include 'Daemon Control' section")
	}

	expectedBindings := []string{
		"start/stop daemon",
		"stop all",
		"switch instance",
		"select instance",
	}
	for _, binding := range expectedBindings {
		if !strings.Contains(view, binding) {
			t.Errorf("help overlay missing binding description: %q", binding)
		}
	}
}

func TestHelpToggleActivatesAndDeactivates(t *testing.T) {
	m := NewModel()
	m.SetSize(120, 40)

	if m.IsActive() {
		t.Error("should start inactive")
	}

	m.Toggle()
	if !m.IsActive() {
		t.Error("should be active after toggle")
	}

	m.Toggle()
	if m.IsActive() {
		t.Error("should be inactive after second toggle")
	}
}

func TestHelpViewEmptyWhenInactive(t *testing.T) {
	m := NewModel()
	m.SetSize(120, 40)

	if m.View() != "" {
		t.Error("inactive help should render empty string")
	}
}

// keyPress builds a printable-character key press.
func keyPress(r rune) tea.KeyPressMsg {
	return tea.KeyPressMsg{Code: r, Text: string(r)}
}

func specialKey(code rune) tea.KeyPressMsg {
	return tea.KeyPressMsg{Code: code}
}

func TestUpdate_InactiveSwallowsKeys(t *testing.T) {
	m := NewModel()
	m.SetSize(120, 40)
	// Inactive: keys should be swallowed without state change.
	m2, cmd := m.Update(keyPress('j'))
	if cmd != nil {
		t.Fatal("expected nil cmd from inactive update")
	}
	if m2.scroll != 0 || m2.active {
		t.Fatal("inactive update should not change state")
	}
}

func TestUpdate_DismissKey(t *testing.T) {
	m := NewModel()
	m.SetSize(120, 40)
	m.Toggle()
	if !m.IsActive() {
		t.Fatal("expected active before dismiss")
	}

	m, _ = m.Update(specialKey(tea.KeyEscape))
	if m.IsActive() {
		t.Fatal("expected inactive after Esc")
	}
}

func TestUpdate_ScrollDownAndUp(t *testing.T) {
	m := NewModel()
	// Tiny size forces a small visible window so scrolling is meaningful.
	m.SetSize(40, 20)
	m.Toggle()

	// Scroll down repeatedly; should clamp at maxScroll.
	for range 50 {
		m, _ = m.Update(keyPress('j'))
	}
	if m.scroll == 0 {
		t.Fatal("expected scroll to advance")
	}
	maxScroll := m.maxScroll()
	if m.scroll != maxScroll {
		t.Fatalf("expected scroll=%d (clamped), got %d", maxScroll, m.scroll)
	}

	// Scroll up repeatedly; should clamp at 0.
	for range 50 {
		m, _ = m.Update(keyPress('k'))
	}
	if m.scroll != 0 {
		t.Fatalf("expected scroll=0 after scrolling up, got %d", m.scroll)
	}
}

func TestUpdate_UnknownKeyAbsorbed(t *testing.T) {
	m := NewModel()
	m.SetSize(120, 40)
	m.Toggle()

	// Random unhandled key should be absorbed (no state change, no cmd).
	m2, cmd := m.Update(keyPress('z'))
	if cmd != nil {
		t.Fatal("expected nil cmd from unknown key")
	}
	if m2.scroll != 0 || !m2.IsActive() {
		t.Fatal("unknown key should not change state")
	}
}

func TestUpdate_WindowSizeRebuildsContent(t *testing.T) {
	m := NewModel()
	// First Toggle builds content at zero size.
	m.Toggle()
	initialLines := m.lines
	if initialLines == 0 {
		t.Fatal("expected content to be built")
	}

	// WindowSizeMsg should rebuild content (lines stays the same since
	// content text doesn't depend on size, but we exercise the path).
	m, _ = m.Update(tea.WindowSizeMsg{Width: 100, Height: 50})
	if m.width != 100 || m.height != 50 {
		t.Fatalf("expected 100x50, got %dx%d", m.width, m.height)
	}
	if m.lines != initialLines {
		t.Fatalf("expected lines=%d after rebuild, got %d", initialLines, m.lines)
	}
}

func TestMaxScroll_ZeroWhenContentFits(t *testing.T) {
	m := NewModel()
	m.SetSize(200, 200) // huge window so all content fits
	m.Toggle()

	if m.maxScroll() != 0 {
		t.Fatalf("expected maxScroll=0 when content fits, got %d", m.maxScroll())
	}
}

func TestMaxScroll_PositiveWhenContentOverflows(t *testing.T) {
	m := NewModel()
	m.SetSize(40, 20) // small window so content overflows
	m.Toggle()

	if m.maxScroll() <= 0 {
		t.Fatalf("expected positive maxScroll, got %d", m.maxScroll())
	}
}

func TestView_ClampsScrollOnRender(t *testing.T) {
	m := NewModel()
	m.SetSize(40, 20)
	m.Toggle()

	// Manually set scroll past the end; View should clamp internally.
	m.scroll = 9999
	view := m.View()
	if view == "" {
		t.Fatal("expected non-empty view when active")
	}
}
