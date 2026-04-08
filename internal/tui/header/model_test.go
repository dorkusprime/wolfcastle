package header

import (
	"strings"
	"testing"

	tea "charm.land/bubbletea/v2"

	"github.com/dorkusprime/wolfcastle/internal/instance"
	"github.com/dorkusprime/wolfcastle/internal/state"
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

	// Padded view with tab bar:
	// blankTop, line1, line2, tabBar, blankBot = 5 lines
	if len(lines) != 5 {
		t.Fatalf("expected 5 lines (padded with tab bar), got %d:\n%s", len(lines), view)
	}

	// Tab bar is at index 3 (blankTop, line1, line2, tabBar).
	tabLine := lines[3]
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

	// Padded view: blankTop, line1, line2, blankBot = 4 lines.
	if len(lines) != 4 {
		t.Errorf("expected 4 lines (padded, no tab bar), got %d:\n%s", len(lines), view)
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
	if len(lines) != 5 {
		t.Fatalf("expected 5 lines, got %d", len(lines))
	}
	// Tab bar is at index 3 (blankTop, line1, line2, tabBar, blankBot).
	tabLine := lines[3]

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

func TestSetLoadingAndIsLoading(t *testing.T) {
	m := NewHeaderModel("1.0.0")
	if m.IsLoading() {
		t.Error("new model should not be loading")
	}

	m.SetLoading(true)
	if !m.IsLoading() {
		t.Error("should be loading after SetLoading(true)")
	}

	m.SetLoading(false)
	if m.IsLoading() {
		t.Error("should not be loading after SetLoading(false)")
	}
}

func TestViewZeroWidth(t *testing.T) {
	m := NewHeaderModel("1.0.0")
	// Width 0 should return empty string.
	if v := m.View(); v != "" {
		t.Errorf("expected empty view for zero width, got %q", v)
	}
}

func TestViewNarrowTerminal(t *testing.T) {
	m := NewHeaderModel("1.0.0")
	m.SetSize(30) // < 40 triggers single-line mode
	view := m.View()
	lines := strings.Split(view, "\n")
	// Padded narrow: blankTop, line1, blankBot = 3 lines.
	if len(lines) != 3 {
		t.Errorf("expected 3 lines for padded narrow terminal, got %d", len(lines))
	}
}

func TestViewContainsVersion(t *testing.T) {
	m := NewHeaderModel("2.3.4")
	m.SetSize(120)
	view := m.View()
	if !strings.Contains(view, "2.3.4") {
		t.Errorf("view should contain version string: %q", view)
	}
}

func TestViewShowsLoadingSpinner(t *testing.T) {
	m := NewHeaderModel("1.0.0")
	m.SetSize(120)
	m.SetLoading(true)
	view := m.View()
	// The spinner character (first frame ⠋) should appear somewhere.
	if !strings.ContainsRune(view, '⠋') {
		t.Errorf("loading view should contain spinner frame: %q", view)
	}
}

func TestUpdateDaemonStatusMsg(t *testing.T) {
	m := NewHeaderModel("1.0.0")
	m.SetSize(120)

	msg := DaemonStatusMsg{
		Status:    "running",
		Branch:    "feat/auth",
		Worktree:  "/home/dev/proj",
		PID:       12345,
		IsRunning: true,
		Instances: []instance.Entry{{PID: 1}, {PID: 2}},
	}
	m, _ = m.Update(msg)

	if m.branch != "feat/auth" {
		t.Errorf("branch = %q, want feat/auth", m.branch)
	}
	if m.instanceCount != 2 {
		t.Errorf("instanceCount = %d, want 2", m.instanceCount)
	}
	view := m.View()
	if !strings.Contains(view, "hunting") {
		t.Errorf("view should mention hunting for running daemon: %q", view)
	}
}

func TestUpdateStateUpdatedMsg(t *testing.T) {
	m := NewHeaderModel("1.0.0")
	m.SetSize(120)

	idx := &state.RootIndex{
		Nodes: map[string]state.IndexEntry{
			"a": {Name: "a", State: state.StatusComplete},
			"b": {Name: "b", State: state.StatusInProgress},
			"c": {Name: "c", State: state.StatusNotStarted},
			"d": {Name: "d", State: state.StatusComplete, Archived: true},
		},
	}
	m, _ = m.Update(StateUpdatedMsg{Index: idx})

	if m.totalNodes != 3 {
		t.Errorf("totalNodes = %d, want 3 (archived excluded)", m.totalNodes)
	}
	if m.nodeCounts[state.StatusComplete] != 1 {
		t.Errorf("complete = %d, want 1", m.nodeCounts[state.StatusComplete])
	}
	view := m.View()
	if !strings.Contains(view, "3 nodes") {
		t.Errorf("view should show node count: %q", view)
	}
}

func TestUpdateStateUpdatedMsgNilIndex(t *testing.T) {
	m := NewHeaderModel("1.0.0")
	m.SetSize(120)
	m, _ = m.Update(StateUpdatedMsg{Index: nil})
	if m.totalNodes != 0 {
		t.Errorf("totalNodes should be 0 for nil index, got %d", m.totalNodes)
	}
}

func TestUpdateInstancesUpdatedMsg(t *testing.T) {
	m := NewHeaderModel("1.0.0")
	m, _ = m.Update(InstancesUpdatedMsg{Instances: []instance.Entry{{PID: 1}, {PID: 2}, {PID: 3}}})
	if m.instanceCount != 3 {
		t.Errorf("instanceCount = %d, want 3", m.instanceCount)
	}
}

func TestUpdateSpinnerTickMsg(t *testing.T) {
	m := NewHeaderModel("1.0.0")
	if m.spinner != 0 {
		t.Fatalf("initial spinner = %d, want 0", m.spinner)
	}
	m, _ = m.Update(SpinnerTickMsg{})
	if m.spinner != 1 {
		t.Errorf("spinner after tick = %d, want 1", m.spinner)
	}
	// Wrap around after all frames.
	for i := 0; i < len(spinnerFrames)-1; i++ {
		m, _ = m.Update(SpinnerTickMsg{})
	}
	if m.spinner != 0 {
		t.Errorf("spinner should wrap to 0, got %d", m.spinner)
	}
}

func TestUpdateWindowSizeMsg(t *testing.T) {
	m := NewHeaderModel("1.0.0")
	m, _ = m.Update(tea.WindowSizeMsg{Width: 200, Height: 50})
	if m.width != 200 {
		t.Errorf("width = %d, want 200", m.width)
	}
}

func TestDaemonStatusString(t *testing.T) {
	tests := []struct {
		name string
		msg  DaemonStatusMsg
		want string
	}{
		{
			name: "empty status",
			msg:  DaemonStatusMsg{},
			want: "status unknown",
		},
		{
			name: "running with worktree",
			msg:  DaemonStatusMsg{Status: "ok", IsRunning: true, PID: 42, Worktree: "/w", Branch: "main"},
			want: "/w (main) hunting (PID 42)",
		},
		{
			name: "draining",
			msg:  DaemonStatusMsg{Status: "ok", IsRunning: true, IsDraining: true, PID: 7, Worktree: "/w"},
			want: "/w draining (PID 7)",
		},
		{
			name: "stale PID",
			msg:  DaemonStatusMsg{Status: "stale", IsRunning: false, PID: 99, Worktree: "/w"},
			want: "/w presumed dead (stale PID 99)",
		},
		{
			name: "standing down no worktree",
			msg:  DaemonStatusMsg{Status: "off"},
			want: "standing down",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := daemonStatusString(tt.msg)
			if got != tt.want {
				t.Errorf("got %q, want %q", got, tt.want)
			}
		})
	}
}

func TestPluralize(t *testing.T) {
	if got := pluralize("gap", 1); got != "gap" {
		t.Errorf("pluralize(gap,1) = %q", got)
	}
	if got := pluralize("gap", 0); got != "gaps" {
		t.Errorf("pluralize(gap,0) = %q", got)
	}
	if got := pluralize("gap", 5); got != "gaps" {
		t.Errorf("pluralize(gap,5) = %q", got)
	}
}

func TestRenderTabBarSingleInstance(t *testing.T) {
	m := NewHeaderModel("1.0.0")
	m.SetSize(120)
	m.SetInstances([]instance.Entry{
		{PID: 100, Branch: "main"},
	}, 0)

	// With only 1 instance, View should NOT produce a tab bar line.
	// Padded: blankTop, line1, line2, blankBot = 4 lines.
	view := m.View()
	lines := strings.Split(view, "\n")
	if len(lines) != 4 {
		t.Errorf("single instance should produce 4 lines (no tab bar), got %d", len(lines))
	}
}

func TestRenderNodeCountsZero(t *testing.T) {
	m := NewHeaderModel("1.0.0")
	m.SetSize(120)
	view := m.View()
	if !strings.Contains(view, "0 nodes") {
		t.Errorf("empty model should show '0 nodes': %q", view)
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
