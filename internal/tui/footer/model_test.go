package footer

import (
	"strings"
	"testing"

	tea "charm.land/bubbletea/v2"

	"github.com/dorkusprime/wolfcastle/internal/tui"
)

func TestNewModel_ZeroValues(t *testing.T) {
	t.Parallel()
	m := NewModel()
	if m.focusedPane != 0 {
		t.Errorf("expected focusedPane 0, got %d", m.focusedPane)
	}
	if m.daemonRunning {
		t.Error("expected daemonRunning false")
	}
	if m.width != 0 {
		t.Errorf("expected width 0, got %d", m.width)
	}
}

func TestView_RendersKeyHints(t *testing.T) {
	t.Parallel()
	m := NewModel()
	m.width = 200 // wide enough for all hints
	v := m.View()
	if !strings.Contains(v, "[q] quit") {
		t.Errorf("expected [q] quit, got: %s", v)
	}
	if !strings.Contains(v, "[Tab] focus") {
		t.Errorf("expected [Tab] focus, got: %s", v)
	}
	if !strings.Contains(v, "[<>] switch") {
		t.Errorf("expected [<>] switch, got: %s", v)
	}
	if !strings.Contains(v, "[?] help") {
		t.Errorf("expected [?] help, got: %s", v)
	}
}

func TestDaemonStatusMsg_Running_ShowsStop(t *testing.T) {
	t.Parallel()
	m := NewModel()
	m.width = 200
	m, _ = m.Update(tui.DaemonStatusMsg{IsRunning: true})
	v := m.View()
	if !strings.Contains(v, "[s] stop") {
		t.Errorf("expected [s] stop when daemon running, got: %s", v)
	}
}

func TestDaemonStatusMsg_NotRunning_ShowsStart(t *testing.T) {
	t.Parallel()
	m := NewModel()
	m.width = 200
	m, _ = m.Update(tui.DaemonStatusMsg{IsRunning: false})
	v := m.View()
	if !strings.Contains(v, "[s] start") {
		t.Errorf("expected [s] start when daemon not running, got: %s", v)
	}
}

func TestSetFocus_UpdatesPane(t *testing.T) {
	t.Parallel()
	m := NewModel()
	m.SetFocus(1)
	if m.focusedPane != 1 {
		t.Errorf("expected focusedPane 1, got %d", m.focusedPane)
	}
	m.SetFocus(0)
	if m.focusedPane != 0 {
		t.Errorf("expected focusedPane 0, got %d", m.focusedPane)
	}
}

func TestSetSize_UpdatesWidth(t *testing.T) {
	t.Parallel()
	m := NewModel()
	m.SetSize(80)
	if m.width != 80 {
		t.Errorf("expected width 80, got %d", m.width)
	}
}

func TestSetDetailMode(t *testing.T) {
	t.Parallel()
	m := NewModel()
	m.SetDetailMode(2)
	if m.detailMode != 2 {
		t.Errorf("expected detailMode 2, got %d", m.detailMode)
	}
}

func TestTruncation_NarrowWidth(t *testing.T) {
	t.Parallel()
	m := NewModel()
	// Set width so narrow that only a few hints fit
	m.width = 25
	v := m.View()
	// Should contain high-priority hints
	if !strings.Contains(v, "[q] quit") {
		t.Errorf("expected [q] quit even at narrow width, got: %s", v)
	}
	// Should NOT contain low-priority hints that don't fit
	if strings.Contains(v, "[R] refresh") {
		t.Errorf("expected [R] refresh to be truncated at width 25, got: %s", v)
	}
}

func TestTruncation_VeryNarrow(t *testing.T) {
	t.Parallel()
	m := NewModel()
	m.width = 10
	v := m.View()
	// At width 10, only "[q] quit" (8 chars) should fit
	if !strings.Contains(v, "[q] quit") {
		t.Errorf("expected [q] quit at width 10, got: %s", v)
	}
	if strings.Contains(v, "[Tab]") {
		t.Errorf("expected Tab hint to be truncated at width 10, got: %s", v)
	}
}

func TestTruncation_ZeroWidth_NoLimit(t *testing.T) {
	t.Parallel()
	m := NewModel()
	m.width = 0 // zero means no limit
	v := m.View()
	// All hints should render
	if !strings.Contains(v, "[R] refresh") {
		t.Errorf("expected all hints with zero width (no limit), got: %s", v)
	}
}

func TestWindowSizeMsg_UpdatesWidth(t *testing.T) {
	t.Parallel()
	m := NewModel()
	m, _ = m.Update(tea.WindowSizeMsg{Width: 120, Height: 40})
	if m.width != 120 {
		t.Errorf("expected width 120, got %d", m.width)
	}
}

func TestUpdate_UnhandledMsg_NoChange(t *testing.T) {
	t.Parallel()
	m := NewModel()
	m.width = 80
	m2, cmd := m.Update(tea.FocusMsg{})
	if cmd != nil {
		t.Error("expected nil cmd for unhandled msg")
	}
	if m2.width != m.width {
		t.Error("model should be unchanged for unhandled msg")
	}
}
