package detail

import (
	"strings"
	"testing"
	"time"

	tea "charm.land/bubbletea/v2"

	"github.com/dorkusprime/wolfcastle/internal/state"
	"github.com/dorkusprime/wolfcastle/internal/tui"
)

func TestNewInboxModel_InitializesCleanState(t *testing.T) {
	t.Parallel()
	m := NewInboxModel()
	if m.cursor != 0 {
		t.Errorf("cursor should start at 0, got %d", m.cursor)
	}
	if m.inputMode {
		t.Error("inputMode should start false")
	}
	if len(m.items) != 0 {
		t.Errorf("items should be empty, got %d", len(m.items))
	}
}

func TestSetItems_ClampsHighCursor(t *testing.T) {
	t.Parallel()
	m := NewInboxModel()
	m.cursor = 5
	m.SetItems([]state.InboxItem{
		{Text: "only one"},
	})
	if m.cursor != 0 {
		t.Errorf("cursor should clamp to 0, got %d", m.cursor)
	}
}

func TestSetItems_EmptyListResetsCursor(t *testing.T) {
	t.Parallel()
	m := NewInboxModel()
	m.cursor = 3
	m.SetItems(nil)
	if m.cursor != 0 {
		t.Errorf("cursor should be 0 after empty set, got %d", m.cursor)
	}
}

func TestUpdate_JDown_MovesCursor(t *testing.T) {
	t.Parallel()
	m := NewInboxModel()
	m.SetSize(80, 24)
	m.SetItems([]state.InboxItem{
		{Text: "a"}, {Text: "b"}, {Text: "c"},
	})
	m, _ = m.Update(tea.KeyPressMsg{Code: -1, Text: "j"})
	if m.cursor != 1 {
		t.Errorf("expected cursor 1 after j, got %d", m.cursor)
	}
}

func TestUpdate_KUp_MovesCursor(t *testing.T) {
	t.Parallel()
	m := NewInboxModel()
	m.SetSize(80, 24)
	m.SetItems([]state.InboxItem{
		{Text: "a"}, {Text: "b"}, {Text: "c"},
	})
	m.cursor = 2
	m, _ = m.Update(tea.KeyPressMsg{Code: -1, Text: "k"})
	if m.cursor != 1 {
		t.Errorf("expected cursor 1 after k, got %d", m.cursor)
	}
}

func TestUpdate_JAtBottom_DoesNotOverflow(t *testing.T) {
	t.Parallel()
	m := NewInboxModel()
	m.SetSize(80, 24)
	m.SetItems([]state.InboxItem{
		{Text: "a"}, {Text: "b"},
	})
	m.cursor = 1
	m, _ = m.Update(tea.KeyPressMsg{Code: -1, Text: "j"})
	if m.cursor != 1 {
		t.Errorf("cursor should stay at 1, got %d", m.cursor)
	}
}

func TestUpdate_KAtTop_DoesNotUnderflow(t *testing.T) {
	t.Parallel()
	m := NewInboxModel()
	m.SetSize(80, 24)
	m.SetItems([]state.InboxItem{{Text: "a"}})
	m, _ = m.Update(tea.KeyPressMsg{Code: -1, Text: "k"})
	if m.cursor != 0 {
		t.Errorf("cursor should stay at 0, got %d", m.cursor)
	}
}

func TestUpdate_AActivatesInputMode(t *testing.T) {
	t.Parallel()
	m := NewInboxModel()
	m.SetSize(80, 24)
	m, _ = m.Update(tea.KeyPressMsg{Code: -1, Text: "a"})
	if !m.inputMode {
		t.Error("expected inputMode after pressing a")
	}
}

func TestUpdate_EscCancelsInputMode(t *testing.T) {
	t.Parallel()
	m := NewInboxModel()
	m.SetSize(80, 24)
	m.inputMode = true
	m.input.Focus()
	m, _ = m.Update(tea.KeyPressMsg{Code: tea.KeyEscape, Text: "esc"})
	if m.inputMode {
		t.Error("expected inputMode to be false after Esc")
	}
}

func TestUpdate_EnterEmptyInput_CancelsWithoutCmd(t *testing.T) {
	t.Parallel()
	m := NewInboxModel()
	m.SetSize(80, 24)
	m.inputMode = true
	m.input.Focus()
	m, cmd := m.Update(tea.KeyPressMsg{Code: tea.KeyEnter, Text: "enter"})
	if m.inputMode {
		t.Error("inputMode should be false after enter on empty")
	}
	if cmd != nil {
		t.Error("no cmd expected for empty input submit")
	}
}

func TestUpdate_EnterWithText_ReturnsAddCmd(t *testing.T) {
	t.Parallel()
	m := NewInboxModel()
	m.SetSize(80, 24)
	m.inputMode = true
	m.input.Focus()
	m.input.SetValue("Buy milk")
	m, cmd := m.Update(tea.KeyPressMsg{Code: tea.KeyEnter, Text: "enter"})
	if m.inputMode {
		t.Error("inputMode should be false after submit")
	}
	if cmd == nil {
		t.Fatal("expected a cmd from submit")
	}
	msg := cmd()
	addCmd, ok := msg.(tui.AddInboxItemCmd)
	if !ok {
		t.Fatalf("expected AddInboxItemCmd, got %T", msg)
	}
	if addCmd.Text != "Buy milk" {
		t.Errorf("expected 'Buy milk', got %q", addCmd.Text)
	}
}

func TestUpdate_InboxUpdatedMsg_SetsItems(t *testing.T) {
	t.Parallel()
	m := NewInboxModel()
	m.SetSize(80, 24)
	inbox := &state.InboxFile{
		Items: []state.InboxItem{
			{Text: "item one", Status: state.InboxNew},
			{Text: "item two", Status: state.InboxFiled},
		},
	}
	m, _ = m.Update(tui.InboxUpdatedMsg{Inbox: inbox})
	if len(m.items) != 2 {
		t.Errorf("expected 2 items, got %d", len(m.items))
	}
}

func TestSelectedText_ReturnsCorrectItem(t *testing.T) {
	t.Parallel()
	m := NewInboxModel()
	m.SetItems([]state.InboxItem{
		{Text: "first"}, {Text: "second"},
	})
	m.cursor = 1
	if m.SelectedText() != "second" {
		t.Errorf("expected 'second', got %q", m.SelectedText())
	}
}

func TestSelectedText_EmptyReturnsEmpty(t *testing.T) {
	t.Parallel()
	m := NewInboxModel()
	if m.SelectedText() != "" {
		t.Errorf("expected empty, got %q", m.SelectedText())
	}
}

func TestSearchContent_ReturnsTexts(t *testing.T) {
	t.Parallel()
	m := NewInboxModel()
	m.SetItems([]state.InboxItem{
		{Text: "alpha"}, {Text: "beta"},
	})
	content := m.SearchContent()
	if len(content) != 2 {
		t.Fatalf("expected 2 items, got %d", len(content))
	}
	if !strings.Contains(content[0], "alpha") || !strings.Contains(content[1], "beta") {
		t.Errorf("unexpected search content: %v", content)
	}
}

func TestView_EmptyInbox(t *testing.T) {
	t.Parallel()
	m := NewInboxModel()
	m.SetSize(60, 20)
	v := m.View()
	if !strings.Contains(v, "INBOX") {
		t.Error("expected INBOX header")
	}
	if !strings.Contains(v, "0 items") {
		t.Error("expected 0 items count")
	}
	if !strings.Contains(v, "The silence is temporary") {
		t.Error("expected empty inbox message")
	}
	if !strings.Contains(v, "Press [a] to add an item") {
		t.Error("expected add hint")
	}
}

func TestView_WithItems(t *testing.T) {
	t.Parallel()
	m := NewInboxModel()
	m.SetSize(80, 24)
	m.SetFocused(true)
	now := time.Now().UTC().Format(time.RFC3339)
	m.SetItems([]state.InboxItem{
		{Text: "Buy milk", Status: state.InboxNew, Timestamp: now},
		{Text: "API rate limiting", Status: state.InboxFiled, Timestamp: now},
	})
	v := m.View()
	if !strings.Contains(v, "2 items") {
		t.Error("expected 2 items count")
	}
	if !strings.Contains(v, "Buy milk") {
		t.Error("expected 'Buy milk' in view")
	}
	if !strings.Contains(v, "API rate limiting") {
		t.Error("expected 'API rate limiting' in view")
	}
	if !strings.Contains(v, "○") {
		t.Error("expected ○ glyph for new items")
	}
	if !strings.Contains(v, "●") {
		t.Error("expected ● glyph for filed items")
	}
}

func TestView_ReadError(t *testing.T) {
	t.Parallel()
	m := NewInboxModel()
	m.SetSize(60, 20)
	m.SetReadError(true)
	v := m.View()
	if !strings.Contains(v, "Inbox unreadable") {
		t.Error("expected read error message")
	}
}

func TestView_InputMode(t *testing.T) {
	t.Parallel()
	m := NewInboxModel()
	m.SetSize(60, 20)
	m.inputMode = true
	m.input.Focus()
	v := m.View()
	// In input mode, should not show the hint text.
	if strings.Contains(v, "Press [a] to add an item") {
		t.Error("should not show hint in input mode")
	}
}

func TestInboxRelativeTime_JustNow(t *testing.T) {
	t.Parallel()
	ts := time.Now().UTC().Format(time.RFC3339)
	got := inboxRelativeTime(ts)
	if got != "just now" {
		t.Errorf("expected 'just now', got %q", got)
	}
}

func TestInboxRelativeTime_MinutesAgo(t *testing.T) {
	t.Parallel()
	ts := time.Now().Add(-5 * time.Minute).UTC().Format(time.RFC3339)
	got := inboxRelativeTime(ts)
	if !strings.Contains(got, "m ago") {
		t.Errorf("expected '5m ago', got %q", got)
	}
}

func TestInboxRelativeTime_InvalidTimestamp(t *testing.T) {
	t.Parallel()
	got := inboxRelativeTime("not-a-date")
	if got != "not-a-date" {
		t.Errorf("expected raw string fallback, got %q", got)
	}
}
