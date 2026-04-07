package help

import (
	"strings"
	"testing"
)

func TestHelpContentIncludesDaemonControl(t *testing.T) {
	m := NewHelpOverlayModel()
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
	m := NewHelpOverlayModel()
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
	m := NewHelpOverlayModel()
	m.SetSize(120, 40)

	if m.View() != "" {
		t.Error("inactive help should render empty string")
	}
}
