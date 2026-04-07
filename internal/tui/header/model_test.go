package header

import (
	"strings"
	"testing"

	"github.com/dorkusprime/wolfcastle/internal/instance"
)

func TestSetInstances(t *testing.T) {
	m := NewHeaderModel("1.0.0")

	entries := []instance.Entry{
		{PID: 100, Worktree: "/a", Branch: "feat/auth"},
		{PID: 200, Worktree: "/b", Branch: "fix/login"},
	}
	m.SetInstances(entries, 1)

	if m.instanceCount != 2 {
		t.Errorf("instanceCount = %d, want 2", m.instanceCount)
	}
	if m.activeIndex != 1 {
		t.Errorf("activeIndex = %d, want 1", m.activeIndex)
	}
	if len(m.instances) != 2 {
		t.Errorf("len(instances) = %d, want 2", len(m.instances))
	}
}

func TestViewTabBarWideTerminal(t *testing.T) {
	m := NewHeaderModel("1.0.0")
	m.SetSize(120)
	m.SetInstances([]instance.Entry{
		{PID: 100, Branch: "feat/auth"},
		{PID: 200, Branch: "fix/login"},
	}, 0)

	view := m.View()
	lines := strings.Split(view, "\n")

	// Wide terminal with 2+ instances should produce 3 lines (including tab bar).
	if len(lines) < 3 {
		t.Fatalf("expected at least 3 lines for tab bar, got %d:\n%s", len(lines), view)
	}

	// Tab bar (line 3) should contain both branch names.
	tabLine := lines[2]
	if !strings.Contains(tabLine, "feat/auth") {
		t.Errorf("tab bar missing feat/auth: %q", tabLine)
	}
	if !strings.Contains(tabLine, "fix/login") {
		t.Errorf("tab bar missing fix/login: %q", tabLine)
	}
}

func TestViewNoTabBarNarrowTerminal(t *testing.T) {
	m := NewHeaderModel("1.0.0")
	m.SetSize(80) // <= 100: no tab bar
	m.SetInstances([]instance.Entry{
		{PID: 100, Branch: "feat/auth"},
		{PID: 200, Branch: "fix/login"},
	}, 0)

	view := m.View()
	lines := strings.Split(view, "\n")

	// Narrow terminal should produce exactly 2 lines (no tab bar).
	if len(lines) != 2 {
		t.Errorf("expected 2 lines (no tab bar), got %d:\n%s", len(lines), view)
	}

	// Should still show the instance badge.
	if !strings.Contains(view, "[2 running]") {
		t.Errorf("narrow view should show badge, got: %q", view)
	}
}

func TestActiveInstanceMarker(t *testing.T) {
	m := NewHeaderModel("1.0.0")
	m.SetSize(120)
	m.SetInstances([]instance.Entry{
		{PID: 100, Branch: "feat/auth"},
		{PID: 200, Branch: "fix/login"},
	}, 0)

	view := m.View()
	lines := strings.Split(view, "\n")
	if len(lines) < 3 {
		t.Fatalf("expected 3 lines, got %d", len(lines))
	}
	tabLine := lines[2]

	// Active instance (index 0, feat/auth) should have the ● marker.
	if !strings.Contains(tabLine, "●") {
		t.Errorf("active instance missing ● marker: %q", tabLine)
	}

	// The ● should appear near "feat/auth", not near "fix/login".
	authIdx := strings.Index(tabLine, "feat/auth")
	dotIdx := strings.Index(tabLine, "●")
	loginIdx := strings.Index(tabLine, "fix/login")
	if dotIdx < authIdx || dotIdx > loginIdx {
		t.Errorf("● marker at %d should be between feat/auth(%d) and fix/login(%d)", dotIdx, authIdx, loginIdx)
	}
}

func TestSetStatusHintReplacesStatus(t *testing.T) {
	m := NewHeaderModel("1.0.0")
	m.SetSize(120)

	// Default should show daemon status text.
	viewBefore := m.View()
	if !strings.Contains(viewBefore, "standing down") {
		t.Errorf("default view should contain 'standing down': %q", viewBefore)
	}

	m.SetStatusHint("Starting daemon...")
	viewAfter := m.View()

	if !strings.Contains(viewAfter, "Starting daemon...") {
		t.Errorf("view should contain hint text: %q", viewAfter)
	}
	if strings.Contains(viewAfter, "standing down") {
		t.Errorf("view should NOT contain daemon status when hint is set: %q", viewAfter)
	}

	// Clearing the hint should restore daemon status.
	m.SetStatusHint("")
	viewCleared := m.View()
	if !strings.Contains(viewCleared, "standing down") {
		t.Errorf("cleared hint should restore daemon status: %q", viewCleared)
	}
}
