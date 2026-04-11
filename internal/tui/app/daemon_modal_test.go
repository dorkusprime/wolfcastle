package app

import (
	"strings"
	"testing"

	tea "charm.land/bubbletea/v2"

	"github.com/dorkusprime/wolfcastle/internal/tui"
)

func TestDaemonModal_OpenAndClose(t *testing.T) {
	t.Parallel()
	var m DaemonModalModel
	m.SetSize(120, 40)

	if m.IsActive() {
		t.Fatal("modal should start inactive")
	}

	m.Open("start", false, false, 0, "main", "/tmp/wc")
	if !m.IsActive() {
		t.Fatal("modal should be active after Open")
	}
	if m.action != "start" {
		t.Errorf("expected action 'start', got %q", m.action)
	}

	m.Close()
	if m.IsActive() {
		t.Fatal("modal should be inactive after Close")
	}
}

func TestDaemonModal_EnterConfirms(t *testing.T) {
	t.Parallel()
	var m DaemonModalModel
	m.SetSize(120, 40)
	m.Open("stop", true, false, 1234, "main", "/tmp/wc")

	m, cmd := m.Update(tea.KeyPressMsg{Code: tea.KeyEnter})
	if m.IsActive() {
		t.Error("modal should deactivate on Enter")
	}
	if cmd == nil {
		t.Fatal("Enter should produce a command")
	}
	msg := cmd()
	if _, ok := msg.(tui.DaemonConfirmedMsg); !ok {
		t.Errorf("expected DaemonConfirmedMsg, got %T", msg)
	}
}

func TestDaemonModal_EscCancels(t *testing.T) {
	t.Parallel()
	var m DaemonModalModel
	m.SetSize(120, 40)
	m.Open("start", false, false, 0, "main", "/tmp/wc")

	m, cmd := m.Update(tea.KeyPressMsg{Code: tea.KeyEscape})
	if m.IsActive() {
		t.Error("modal should deactivate on Esc")
	}
	if cmd != nil {
		t.Error("Esc should produce no command")
	}
}

func TestDaemonModal_AbsorbsOtherKeys(t *testing.T) {
	t.Parallel()
	var m DaemonModalModel
	m.SetSize(120, 40)
	m.Open("start", false, false, 0, "main", "/tmp/wc")

	m, cmd := m.Update(tea.KeyPressMsg{Code: -1, Text: "j"})
	if !m.IsActive() {
		t.Error("unrecognized key should not close the modal")
	}
	if cmd != nil {
		t.Error("unrecognized key should produce no command")
	}
}

func TestDaemonModal_ViewStartAction(t *testing.T) {
	t.Parallel()
	var m DaemonModalModel
	m.SetSize(120, 40)
	m.Open("start", false, false, 0, "main", "/tmp/wc")

	v := m.View()
	if !strings.Contains(v, "START DAEMON") {
		t.Errorf("expected START DAEMON title, got:\n%s", v)
	}
	if !strings.Contains(v, "Confirm") {
		t.Errorf("expected Confirm in footer, got:\n%s", v)
	}
}

func TestDaemonModal_ViewStopAction(t *testing.T) {
	t.Parallel()
	var m DaemonModalModel
	m.SetSize(120, 40)
	m.Open("stop", true, true, 4567, "feat/tui", "/tmp/wc")

	v := m.View()
	if !strings.Contains(v, "STOP DAEMON") {
		t.Errorf("expected STOP DAEMON title, got:\n%s", v)
	}
	if !strings.Contains(v, "4567") {
		t.Errorf("expected PID in body, got:\n%s", v)
	}
	if !strings.Contains(v, "draining") {
		t.Errorf("expected draining warning, got:\n%s", v)
	}
}

func TestDaemonModal_ViewEmptyWhenInactive(t *testing.T) {
	t.Parallel()
	var m DaemonModalModel
	m.SetSize(120, 40)
	if v := m.View(); v != "" {
		t.Errorf("inactive modal should render empty, got %q", v)
	}
}
