package header

import (
	"strings"
	"testing"

	tea "charm.land/bubbletea/v2"

	"github.com/dorkusprime/wolfcastle/internal/instance"
	"github.com/dorkusprime/wolfcastle/internal/state"
)

func TestNewHeaderModel(t *testing.T) {
	m := NewHeaderModel("1.2.3")

	if m.version != "1.2.3" {
		t.Errorf("version = %q, want %q", m.version, "1.2.3")
	}
	if m.daemonStatus != "standing down" {
		t.Errorf("daemonStatus = %q, want %q", m.daemonStatus, "standing down")
	}
	if m.nodeCounts == nil {
		t.Fatal("nodeCounts should be initialized")
	}
	if m.auditCounts == nil {
		t.Fatal("auditCounts should be initialized")
	}
	if m.totalNodes != 0 {
		t.Errorf("totalNodes = %d, want 0", m.totalNodes)
	}
}

func TestDaemonStatusMsg_Running(t *testing.T) {
	m := NewHeaderModel("0.1.0")
	m, _ = m.Update(DaemonStatusMsg{
		Status:    "running",
		PID:       42,
		IsRunning: true,
	})

	if m.daemonStatus != "hunting (PID 42)" {
		t.Errorf("daemonStatus = %q, want %q", m.daemonStatus, "hunting (PID 42)")
	}
}

func TestDaemonStatusMsg_Draining(t *testing.T) {
	m := NewHeaderModel("0.1.0")
	m, _ = m.Update(DaemonStatusMsg{
		Status:     "draining",
		PID:        99,
		IsRunning:  true,
		IsDraining: true,
	})

	if m.daemonStatus != "draining (PID 99)" {
		t.Errorf("daemonStatus = %q, want %q", m.daemonStatus, "draining (PID 99)")
	}
}

func TestDaemonStatusMsg_StalePID(t *testing.T) {
	m := NewHeaderModel("0.1.0")
	m, _ = m.Update(DaemonStatusMsg{
		Status:    "stale",
		PID:       77,
		IsRunning: false,
	})

	want := "presumed dead (stale PID 77)"
	if m.daemonStatus != want {
		t.Errorf("daemonStatus = %q, want %q", m.daemonStatus, want)
	}
}

func TestDaemonStatusMsg_NotRunningPID0(t *testing.T) {
	m := NewHeaderModel("0.1.0")
	m, _ = m.Update(DaemonStatusMsg{
		Status:    "stopped",
		PID:       0,
		IsRunning: false,
	})

	if m.daemonStatus != "standing down" {
		t.Errorf("daemonStatus = %q, want %q", m.daemonStatus, "standing down")
	}
}

func TestDaemonStatusMsg_EmptyStatus(t *testing.T) {
	m := NewHeaderModel("0.1.0")
	m, _ = m.Update(DaemonStatusMsg{})

	if m.daemonStatus != "status unknown" {
		t.Errorf("daemonStatus = %q, want %q", m.daemonStatus, "status unknown")
	}
}

func TestDaemonStatusMsg_InstanceCount(t *testing.T) {
	m := NewHeaderModel("0.1.0")
	m, _ = m.Update(DaemonStatusMsg{
		Status:    "running",
		PID:       1,
		IsRunning: true,
		Instances: []instance.Entry{{PID: 1}, {PID: 2}, {PID: 3}},
	})

	if m.instanceCount != 3 {
		t.Errorf("instanceCount = %d, want 3", m.instanceCount)
	}
}

func TestStateUpdatedMsg_NodeCounts(t *testing.T) {
	idx := &state.RootIndex{
		Nodes: map[string]state.IndexEntry{
			"a": {State: state.StatusComplete},
			"b": {State: state.StatusInProgress},
			"c": {State: state.StatusNotStarted},
			"d": {State: state.StatusBlocked},
			"e": {State: state.StatusComplete, Archived: true},
		},
	}

	m := NewHeaderModel("0.1.0")
	m, _ = m.Update(StateUpdatedMsg{Index: idx})

	if m.totalNodes != 4 {
		t.Errorf("totalNodes = %d, want 4 (archived should be skipped)", m.totalNodes)
	}
	if m.nodeCounts[state.StatusComplete] != 1 {
		t.Errorf("complete = %d, want 1", m.nodeCounts[state.StatusComplete])
	}
	if m.nodeCounts[state.StatusInProgress] != 1 {
		t.Errorf("in_progress = %d, want 1", m.nodeCounts[state.StatusInProgress])
	}
	if m.nodeCounts[state.StatusNotStarted] != 1 {
		t.Errorf("not_started = %d, want 1", m.nodeCounts[state.StatusNotStarted])
	}
	if m.nodeCounts[state.StatusBlocked] != 1 {
		t.Errorf("blocked = %d, want 1", m.nodeCounts[state.StatusBlocked])
	}
}

func TestStateUpdatedMsg_NilIndex(t *testing.T) {
	m := NewHeaderModel("0.1.0")
	m, _ = m.Update(StateUpdatedMsg{Index: nil})

	if m.totalNodes != 0 {
		t.Errorf("totalNodes = %d, want 0 with nil index", m.totalNodes)
	}
}

func TestInstancesUpdatedMsg(t *testing.T) {
	m := NewHeaderModel("0.1.0")
	m, _ = m.Update(InstancesUpdatedMsg{
		Instances: []instance.Entry{{PID: 10}, {PID: 20}},
	})

	if m.instanceCount != 2 {
		t.Errorf("instanceCount = %d, want 2", m.instanceCount)
	}
}

func TestSpinnerTickMsg(t *testing.T) {
	m := NewHeaderModel("0.1.0")

	if m.spinner != 0 {
		t.Fatalf("initial spinner = %d, want 0", m.spinner)
	}

	m, _ = m.Update(SpinnerTickMsg{})
	if m.spinner != 1 {
		t.Errorf("after 1 tick spinner = %d, want 1", m.spinner)
	}

	// Advance to wrap around.
	for i := 0; i < len(spinnerFrames)-1; i++ {
		m, _ = m.Update(SpinnerTickMsg{})
	}
	if m.spinner != 0 {
		t.Errorf("after full cycle spinner = %d, want 0", m.spinner)
	}
}

func TestWindowSizeMsg(t *testing.T) {
	m := NewHeaderModel("0.1.0")
	m, _ = m.Update(tea.WindowSizeMsg{Width: 120, Height: 40})

	if m.width != 120 {
		t.Errorf("width = %d, want 120", m.width)
	}
}

func TestSetSize(t *testing.T) {
	m := NewHeaderModel("0.1.0")
	m.SetSize(100)

	if m.width != 100 {
		t.Errorf("width = %d, want 100", m.width)
	}
}

func TestView_TwoLines_At80Columns(t *testing.T) {
	m := NewHeaderModel("0.5.0")
	m.width = 80

	v := m.View()
	lines := strings.Split(v, "\n")
	if len(lines) != 2 {
		t.Errorf("expected 2 lines at 80 cols, got %d", len(lines))
	}
	if !strings.Contains(v, "WOLFCASTLE") {
		t.Error("view should contain WOLFCASTLE")
	}
	if !strings.Contains(v, "v0.5.0") {
		t.Error("view should contain version")
	}
	if !strings.Contains(v, "standing down") {
		t.Error("view should contain daemon status")
	}
}

func TestView_SingleLine_NarrowTerminal(t *testing.T) {
	m := NewHeaderModel("0.5.0")
	m.width = 39

	v := m.View()
	if strings.Contains(v, "\n") {
		t.Error("narrow terminal (< 40 cols) should produce a single line")
	}
}

func TestView_ZeroWidth(t *testing.T) {
	m := NewHeaderModel("0.5.0")
	m.width = 0

	v := m.View()
	if v != "" {
		t.Errorf("zero width should produce empty string, got %q", v)
	}
}

func TestView_ZeroNodes(t *testing.T) {
	m := NewHeaderModel("0.5.0")
	m.width = 80

	v := m.View()
	if !strings.Contains(v, "0 nodes") {
		t.Error("view should contain '0 nodes' when no data loaded")
	}
}

func TestView_MixedStatuses(t *testing.T) {
	m := NewHeaderModel("0.5.0")
	m.width = 80
	m.totalNodes = 5
	m.nodeCounts = map[state.NodeStatus]int{
		state.StatusComplete:   2,
		state.StatusInProgress: 3,
		state.StatusNotStarted: 0,
		state.StatusBlocked:    0,
	}

	v := m.View()
	if !strings.Contains(v, "5 nodes:") {
		t.Error("view should contain '5 nodes:'")
	}
	if !strings.Contains(v, "2") {
		t.Error("view should contain complete count")
	}
	if !strings.Contains(v, "3") {
		t.Error("view should contain in_progress count")
	}
	// Zero counts should be omitted from the rendered text.
	// We can't easily check for omission of specific glyphs due to styling,
	// but we verify the non-zero ones appear.
}

func TestView_NoAuditData(t *testing.T) {
	m := NewHeaderModel("0.5.0")
	m.width = 80

	v := m.View()
	if !strings.Contains(v, "Audit: no data") {
		t.Error("view should contain 'Audit: no data'")
	}
}

func TestView_AuditData(t *testing.T) {
	m := NewHeaderModel("0.5.0")
	m.width = 80
	m.auditCounts = map[state.AuditStatus]int{
		state.AuditPassed: 5,
	}
	m.openGaps = 2

	v := m.View()
	if !strings.Contains(v, "5 passed") {
		t.Error("view should contain '5 passed'")
	}
	if !strings.Contains(v, "2 gaps") {
		t.Error("view should contain '2 gaps'")
	}
}

func TestPluralize(t *testing.T) {
	tests := []struct {
		word  string
		count int
		want  string
	}{
		{"gap", 1, "gap"},
		{"gap", 2, "gaps"},
		{"gap", 0, "gaps"},
		{"escalation", 1, "escalation"},
		{"escalation", 3, "escalations"},
	}

	for _, tt := range tests {
		got := pluralize(tt.word, tt.count)
		if got != tt.want {
			t.Errorf("pluralize(%q, %d) = %q, want %q", tt.word, tt.count, got, tt.want)
		}
	}
}

func TestView_InstanceBadge(t *testing.T) {
	m := NewHeaderModel("0.5.0")
	m.width = 80
	m.instanceCount = 3
	m.daemonStatus = "hunting (PID 1)"

	v := m.View()
	if !strings.Contains(v, "[3 running]") {
		t.Error("view should contain '[3 running]' when instanceCount > 1")
	}
}

func TestView_InstanceBadge_SingleInstance(t *testing.T) {
	m := NewHeaderModel("0.5.0")
	m.width = 80
	m.instanceCount = 1
	m.daemonStatus = "hunting (PID 1)"

	v := m.View()
	if strings.Contains(v, "running]") {
		t.Error("view should NOT contain instance badge when count <= 1")
	}
}

func TestView_AuditEscalations(t *testing.T) {
	m := NewHeaderModel("0.5.0")
	m.width = 80
	m.openEscalations = 1

	v := m.View()
	if !strings.Contains(v, "1 escalation") {
		t.Error("view should contain '1 escalation'")
	}
	// Singular, not "escalations"
	if strings.Contains(v, "1 escalations") {
		t.Error("should use singular for 1 escalation")
	}
}

func TestComposeLine_GapHandling(t *testing.T) {
	// When left + right exceed width, gap should be at least 1.
	base := strings.Builder{}
	_ = base // just testing the function
	style := func() string { return "" }
	_ = style

	// composeLine is unexported but we can test it indirectly through View.
	m := NewHeaderModel("0.5.0")
	m.width = 10 // very narrow
	v := m.View()
	// Should not panic and should produce some output.
	if v == "" {
		t.Error("view at width=10 should produce output")
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
			name: "running",
			msg:  DaemonStatusMsg{Status: "running", IsRunning: true, PID: 42},
			want: "hunting (PID 42)",
		},
		{
			name: "draining",
			msg:  DaemonStatusMsg{Status: "draining", IsRunning: true, IsDraining: true, PID: 99},
			want: "draining (PID 99)",
		},
		{
			name: "stale PID",
			msg:  DaemonStatusMsg{Status: "stale", IsRunning: false, PID: 77},
			want: "presumed dead (stale PID 77)",
		},
		{
			name: "not running PID 0",
			msg:  DaemonStatusMsg{Status: "stopped", IsRunning: false, PID: 0},
			want: "standing down",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := daemonStatusString(tt.msg)
			if got != tt.want {
				t.Errorf("daemonStatusString() = %q, want %q", got, tt.want)
			}
		})
	}
}

func TestUpdate_ReturnsNilCmd(t *testing.T) {
	m := NewHeaderModel("0.1.0")
	_, cmd := m.Update(SpinnerTickMsg{})
	if cmd != nil {
		t.Error("Update should return nil cmd for all header messages")
	}
}

func TestView_SpinnerVisible_WhenLoading(t *testing.T) {
	m := NewHeaderModel("0.5.0")
	m.width = 80
	m.loading = true

	v := m.View()
	// The spinner frame at index 0 is '⠋'.
	if !strings.ContainsRune(v, spinnerFrames[0]) {
		t.Error("view should contain spinner glyph when loading is true")
	}
}

func TestView_SpinnerHidden_WhenNotLoading(t *testing.T) {
	m := NewHeaderModel("0.5.0")
	m.width = 80
	m.loading = false

	v := m.View()
	for _, frame := range spinnerFrames {
		if strings.ContainsRune(v, frame) {
			t.Errorf("view should not contain spinner glyph %c when not loading", frame)
			break
		}
	}
}

func TestStateUpdatedMsg_ResetsCountsOnSecondUpdate(t *testing.T) {
	m := NewHeaderModel("0.1.0")

	// First update with 3 complete nodes.
	idx1 := &state.RootIndex{
		Nodes: map[string]state.IndexEntry{
			"a": {State: state.StatusComplete},
			"b": {State: state.StatusComplete},
			"c": {State: state.StatusComplete},
		},
	}
	m, _ = m.Update(StateUpdatedMsg{Index: idx1})
	if m.totalNodes != 3 {
		t.Fatalf("after first update totalNodes = %d, want 3", m.totalNodes)
	}

	// Second update with 1 node: counts should reset, not accumulate.
	idx2 := &state.RootIndex{
		Nodes: map[string]state.IndexEntry{
			"x": {State: state.StatusBlocked},
		},
	}
	m, _ = m.Update(StateUpdatedMsg{Index: idx2})
	if m.totalNodes != 1 {
		t.Errorf("after second update totalNodes = %d, want 1", m.totalNodes)
	}
	if m.nodeCounts[state.StatusComplete] != 0 {
		t.Errorf("complete count should reset to 0, got %d", m.nodeCounts[state.StatusComplete])
	}
}

func TestSetLoading(t *testing.T) {
	m := NewHeaderModel("1.0.0")
	m.SetSize(80)

	m.SetLoading(true)
	view := m.View()
	// When loading, the spinner frame should appear in the header.
	found := false
	for _, frame := range spinnerFrames {
		if strings.ContainsRune(view, frame) {
			found = true
			break
		}
	}
	if !found {
		t.Error("header should contain a spinner frame when loading is true")
	}

	m.SetLoading(false)
	view = m.View()
	found = false
	for _, frame := range spinnerFrames {
		if strings.ContainsRune(view, frame) {
			found = true
			break
		}
	}
	if found {
		t.Error("header should not contain a spinner frame when loading is false")
	}
}
