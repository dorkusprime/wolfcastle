package search

import (
	"strings"
	"testing"

	tea "charm.land/bubbletea/v2"
)

func TestNewModel(t *testing.T) {
	m := NewModel()
	if m.IsActive() {
		t.Error("new model should not be active")
	}
	if m.Query() != "" {
		t.Errorf("new model query = %q, want empty", m.Query())
	}
	if m.HasMatches() {
		t.Error("new model should have no matches")
	}
}

func TestActivate(t *testing.T) {
	m := NewModel()
	m.Activate(1)
	if !m.IsActive() {
		t.Error("expected active after Activate")
	}
	if m.Query() != "" {
		t.Errorf("query should be empty after Activate, got %q", m.Query())
	}
	if m.paneType != 1 {
		t.Errorf("paneType = %d, want 1", m.paneType)
	}
	if m.current != 0 {
		t.Errorf("current = %d, want 0", m.current)
	}
}

func TestDismiss(t *testing.T) {
	m := NewModel()
	m.Activate(0)
	m.SetMatches([]Match{{Row: 1, Col: 2, Length: 3}})
	m.Dismiss()

	if m.IsActive() {
		t.Error("should not be active after Dismiss")
	}
	if m.Query() != "" {
		t.Errorf("query should be empty after Dismiss, got %q", m.Query())
	}
	if m.HasMatches() {
		t.Error("matches should be cleared after Dismiss")
	}
}

func TestConfirm(t *testing.T) {
	m := NewModel()
	m.Activate(0)
	m.query = "test"
	m.SetMatches([]Match{
		{Row: 0, Col: 0, Length: 4},
		{Row: 1, Col: 5, Length: 4},
	})
	m.Confirm()

	if m.IsActive() {
		t.Error("should not be active after Confirm")
	}
	if !m.HasMatches() {
		t.Error("matches should be preserved after Confirm")
	}
}

func TestConfirmClampsCurrentIndex(t *testing.T) {
	m := NewModel()
	m.Activate(0)
	m.SetMatches([]Match{{Row: 0, Col: 0, Length: 1}})
	m.current = 5 // artificially out of range
	m.Confirm()
	if m.current != 0 {
		t.Errorf("current = %d, want 0 after clamping", m.current)
	}
}

func TestAccessors(t *testing.T) {
	m := NewModel()
	if m.IsActive() {
		t.Error("IsActive should be false on new model")
	}
	if m.HasMatches() {
		t.Error("HasMatches should be false on new model")
	}
	if m.Query() != "" {
		t.Error("Query should be empty on new model")
	}

	m.Activate(0)
	if !m.IsActive() {
		t.Error("IsActive should be true after Activate")
	}

	m.query = "hello"
	if m.Query() != "hello" {
		t.Errorf("Query() = %q, want %q", m.Query(), "hello")
	}
}

func TestSetMatches(t *testing.T) {
	m := NewModel()
	matches := []Match{
		{Row: 0, Col: 0, Length: 3},
		{Row: 2, Col: 1, Length: 3},
	}
	m.SetMatches(matches)
	if !m.HasMatches() {
		t.Error("HasMatches should be true after SetMatches")
	}
	if m.Current() != 0 {
		t.Errorf("Current() = %d, want 0", m.Current())
	}
}

func TestSetMatchesClampsCurrentIndex(t *testing.T) {
	m := NewModel()
	m.SetMatches([]Match{{}, {}, {}})
	m.current = 2
	// Now shrink matches so current is out of range.
	m.SetMatches([]Match{{Row: 0}})
	if m.current != 0 {
		t.Errorf("current = %d, want 0 after clamp", m.current)
	}
}

func TestSetMatchesEmpty(t *testing.T) {
	m := NewModel()
	m.current = 5
	m.SetMatches(nil)
	if m.current != 0 {
		t.Errorf("current = %d, want 0 after empty SetMatches", m.current)
	}
}

func TestNextMatch(t *testing.T) {
	m := NewModel()
	m.SetMatches([]Match{{Row: 0}, {Row: 1}, {Row: 2}})
	m.NextMatch()
	if m.Current() != 1 {
		t.Errorf("after NextMatch: current = %d, want 1", m.Current())
	}
	m.NextMatch()
	if m.Current() != 2 {
		t.Errorf("after second NextMatch: current = %d, want 2", m.Current())
	}
	// Wrap around.
	m.NextMatch()
	if m.Current() != 0 {
		t.Errorf("after wrap NextMatch: current = %d, want 0", m.Current())
	}
}

func TestNextMatchNoMatches(t *testing.T) {
	m := NewModel()
	m.NextMatch() // should not panic
	if m.Current() != 0 {
		t.Errorf("current = %d, want 0", m.Current())
	}
}

func TestPrevMatch(t *testing.T) {
	m := NewModel()
	m.SetMatches([]Match{{Row: 0}, {Row: 1}, {Row: 2}})
	// Wrap backward from 0.
	m.PrevMatch()
	if m.Current() != 2 {
		t.Errorf("after PrevMatch from 0: current = %d, want 2", m.Current())
	}
	m.PrevMatch()
	if m.Current() != 1 {
		t.Errorf("after second PrevMatch: current = %d, want 1", m.Current())
	}
}

func TestPrevMatchNoMatches(t *testing.T) {
	m := NewModel()
	m.PrevMatch() // should not panic
	if m.Current() != 0 {
		t.Errorf("current = %d, want 0", m.Current())
	}
}

func TestCurrentMatch(t *testing.T) {
	m := NewModel()
	m.SetMatches([]Match{
		{Row: 5, Col: 3, Length: 2},
		{Row: 10, Col: 0, Length: 4},
	})
	match, ok := m.CurrentMatch()
	if !ok {
		t.Fatal("expected ok = true")
	}
	if match.Row != 5 || match.Col != 3 || match.Length != 2 {
		t.Errorf("unexpected match: %+v", match)
	}

	m.NextMatch()
	match, ok = m.CurrentMatch()
	if !ok {
		t.Fatal("expected ok = true")
	}
	if match.Row != 10 {
		t.Errorf("Row = %d, want 10", match.Row)
	}
}

func TestCurrentMatchEmpty(t *testing.T) {
	m := NewModel()
	_, ok := m.CurrentMatch()
	if ok {
		t.Error("expected ok = false when no matches")
	}
}

func TestUpdateEscWhenActive(t *testing.T) {
	m := NewModel()
	m.Activate(0)
	m.SetMatches([]Match{{Row: 1}})

	m, _ = m.Update(tea.KeyPressMsg{Code: tea.KeyEscape})
	if m.IsActive() {
		t.Error("expected inactive after Esc")
	}
	if m.HasMatches() {
		t.Error("expected matches cleared after Esc dismiss")
	}
}

func TestUpdateEnterWhenActive(t *testing.T) {
	m := NewModel()
	m.Activate(0)
	m.query = "foo"
	m.SetMatches([]Match{{Row: 0}, {Row: 1}})

	m, _ = m.Update(tea.KeyPressMsg{Code: tea.KeyEnter})
	if m.IsActive() {
		t.Error("expected inactive after Enter")
	}
	if !m.HasMatches() {
		t.Error("expected matches preserved after Enter confirm")
	}
}

func TestUpdateTypingWhenActive(t *testing.T) {
	m := NewModel()
	m.Activate(0)

	// Send a regular key; the textinput should process it.
	m, _ = m.Update(tea.KeyPressMsg{Code: 'a', Text: "a"})
	// The query is synced from the input's value after update.
	if m.query != m.input.Value() {
		t.Errorf("query %q should match input value %q", m.query, m.input.Value())
	}
}

func TestUpdateNWhenInactiveWithMatches(t *testing.T) {
	m := NewModel()
	m.SetMatches([]Match{{Row: 0}, {Row: 1}, {Row: 2}})

	m, _ = m.Update(tea.KeyPressMsg{Code: 'n', Text: "n"})
	if m.Current() != 1 {
		t.Errorf("after n: current = %d, want 1", m.Current())
	}
}

func TestUpdateShiftNWhenInactiveWithMatches(t *testing.T) {
	m := NewModel()
	m.SetMatches([]Match{{Row: 0}, {Row: 1}, {Row: 2}})

	m, _ = m.Update(tea.KeyPressMsg{Code: 'N', Text: "N", Mod: tea.ModShift})
	if m.Current() != 2 {
		t.Errorf("after N: current = %d, want 2", m.Current())
	}
}

func TestUpdateNonKeyMsg(t *testing.T) {
	m := NewModel()
	m.Activate(0)
	// A non-key message should pass through without error.
	m, _ = m.Update(tea.WindowSizeMsg{Width: 80, Height: 24})
	if !m.IsActive() {
		t.Error("non-key msg should not change active state")
	}
}

func TestViewActiveWithMatches(t *testing.T) {
	m := NewModel()
	m.Activate(0)
	m.query = "test"
	m.SetMatches([]Match{{Row: 0}, {Row: 3}})

	v := m.View()
	if !strings.Contains(v, "1/2 matches") {
		t.Errorf("expected match count in view, got: %q", v)
	}
}

func TestViewActiveNoMatches(t *testing.T) {
	m := NewModel()
	m.Activate(0)
	m.query = "nonexistent"

	v := m.View()
	if !strings.Contains(v, "No matches. Adjust your aim.") {
		t.Errorf("expected 'No matches' message, got: %q", v)
	}
}

func TestViewActiveEmptyQuery(t *testing.T) {
	m := NewModel()
	m.Activate(0)

	v := m.View()
	// Should render the input but no match info when query is empty.
	if strings.Contains(v, "matches") {
		t.Errorf("should not show match info with empty query, got: %q", v)
	}
}

func TestViewInactiveNoMatches(t *testing.T) {
	m := NewModel()
	v := m.View()
	if v != "" {
		t.Errorf("expected empty view when inactive with no matches, got: %q", v)
	}
}

func TestPaneType(t *testing.T) {
	m := NewModel()
	if m.PaneType() != 0 {
		t.Errorf("default PaneType = %d, want 0", m.PaneType())
	}
	m.Activate(1)
	if m.PaneType() != 1 {
		t.Errorf("PaneType after Activate(1) = %d, want 1", m.PaneType())
	}
}

func TestViewInactiveWithConfirmedMatches(t *testing.T) {
	m := NewModel()
	m.Activate(0)
	m.query = "foo"
	m.SetMatches([]Match{{Row: 0}, {Row: 1}})
	m.Confirm()

	v := m.View()
	if !strings.Contains(v, "/foo") {
		t.Errorf("expected query in confirmed view, got: %q", v)
	}
	if !strings.Contains(v, "1/2 matches") {
		t.Errorf("expected match count in confirmed view, got: %q", v)
	}
}
