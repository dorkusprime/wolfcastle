package detail

import (
	"strings"
	"testing"
	"time"

	"github.com/dorkusprime/wolfcastle/internal/state"
	"github.com/dorkusprime/wolfcastle/internal/tui"
)

func TestNewDashboardModel_Defaults(t *testing.T) {
	t.Parallel()
	m := NewDashboardModel()
	if m.daemonStatus != "standing down" {
		t.Errorf("expected 'standing down', got %q", m.daemonStatus)
	}
	if m.nodeCounts == nil {
		t.Error("expected nodeCounts to be initialized")
	}
	if m.auditCounts == nil {
		t.Error("expected auditCounts to be initialized")
	}
	if m.totalNodes != 0 {
		t.Errorf("expected 0 total nodes, got %d", m.totalNodes)
	}
}

func TestStateUpdatedMsg_CountsNodes(t *testing.T) {
	t.Parallel()
	m := NewDashboardModel()
	idx := &state.RootIndex{
		Nodes: map[string]state.IndexEntry{
			"a": {State: state.StatusComplete},
			"b": {State: state.StatusInProgress},
			"c": {State: state.StatusNotStarted},
			"d": {State: state.StatusBlocked},
		},
	}
	m, _ = m.Update(tui.StateUpdatedMsg{Index: idx})
	if m.totalNodes != 4 {
		t.Errorf("expected 4 total nodes, got %d", m.totalNodes)
	}
	if m.nodeCounts[state.StatusComplete] != 1 {
		t.Errorf("expected 1 complete, got %d", m.nodeCounts[state.StatusComplete])
	}
	if m.nodeCounts[state.StatusInProgress] != 1 {
		t.Errorf("expected 1 in progress, got %d", m.nodeCounts[state.StatusInProgress])
	}
	if m.nodeCounts[state.StatusNotStarted] != 1 {
		t.Errorf("expected 1 not started, got %d", m.nodeCounts[state.StatusNotStarted])
	}
	if m.nodeCounts[state.StatusBlocked] != 1 {
		t.Errorf("expected 1 blocked, got %d", m.nodeCounts[state.StatusBlocked])
	}
}

func TestStateUpdatedMsg_SkipsArchived(t *testing.T) {
	t.Parallel()
	m := NewDashboardModel()
	idx := &state.RootIndex{
		Nodes: map[string]state.IndexEntry{
			"a": {State: state.StatusComplete},
			"b": {State: state.StatusComplete, Archived: true},
		},
	}
	m, _ = m.Update(tui.StateUpdatedMsg{Index: idx})
	if m.totalNodes != 1 {
		t.Errorf("expected 1 total node (archived skipped), got %d", m.totalNodes)
	}
}

func TestStateUpdatedMsg_NilIndex_NoPanic(t *testing.T) {
	t.Parallel()
	m := NewDashboardModel()
	m, _ = m.Update(tui.StateUpdatedMsg{Index: nil})
	if m.totalNodes != 0 {
		t.Errorf("expected 0 total nodes with nil index, got %d", m.totalNodes)
	}
}

func TestDaemonStatusMsg_UpdatesFields(t *testing.T) {
	t.Parallel()
	m := NewDashboardModel()
	m, _ = m.Update(tui.DaemonStatusMsg{
		Status:    "on patrol",
		Branch:    "feat/tui",
		IsRunning: true,
	})
	if m.daemonStatus != "on patrol" {
		t.Errorf("expected 'on patrol', got %q", m.daemonStatus)
	}
	if m.branch != "feat/tui" {
		t.Errorf("expected 'feat/tui', got %q", m.branch)
	}
	if !m.daemonRunning {
		t.Error("expected daemonRunning to be true")
	}
}

func TestLogLinesMsg_PushesActivity(t *testing.T) {
	t.Parallel()
	m := NewDashboardModel()
	m, _ = m.Update(tui.LogLinesMsg{Lines: []string{
		`{"type":"stage_start","stage":"intake","node":"alpha"}`,
		`{"type":"stage_complete","stage":"exec","node":"alpha","task":"task-0001","exit_code":0}`,
	}})
	if len(m.recentActivity) != 2 {
		t.Fatalf("expected 2 activity entries, got %d", len(m.recentActivity))
	}
	if !strings.Contains(m.recentActivity[0].text, "intake") {
		t.Errorf("expected first entry to mention intake stage, got %q", m.recentActivity[0].text)
	}
	if !strings.Contains(m.recentActivity[1].text, "exit=0") {
		t.Errorf("expected second entry to include exit code, got %q", m.recentActivity[1].text)
	}
}

func TestLogLinesMsg_CappedAtMax(t *testing.T) {
	t.Parallel()
	m := NewDashboardModel()
	lines := make([]string, 15)
	for i := range lines {
		lines[i] = `{"type":"stage_start","stage":"exec","node":"alpha"}`
	}
	m, _ = m.Update(tui.LogLinesMsg{Lines: lines})
	if len(m.recentActivity) != maxActivity {
		t.Errorf("expected %d activity entries (capped), got %d", maxActivity, len(m.recentActivity))
	}
}

func TestLogLinesMsg_SkipsNoiseAndUnparseable(t *testing.T) {
	t.Parallel()
	m := NewDashboardModel()
	m, _ = m.Update(tui.LogLinesMsg{Lines: []string{
		"alpha",                                       // not JSON, dropped
		`{"type":"assistant","text":"yammering"}`,     // assistant chatter, dropped
		`{"type":"stage_start","stage":"intake"}`,     // kept
		`{"type":"unknown_thing","field":"whatever"}`, // unknown type, dropped
	}})
	if len(m.recentActivity) != 1 {
		t.Errorf("expected 1 activity entry (only the stage_start should survive), got %d", len(m.recentActivity))
	}
}

func TestView_NoNodes_ShowsEmptyMessage(t *testing.T) {
	t.Parallel()
	m := NewDashboardModel()
	v := m.View()
	if !strings.Contains(v, "No targets. Feed the inbox.") {
		t.Errorf("expected empty message, got: %s", v)
	}
	if !strings.Contains(v, "MISSION BRIEFING") {
		t.Errorf("expected heading, got: %s", v)
	}
}

func TestView_AllComplete(t *testing.T) {
	t.Parallel()
	m := NewDashboardModel()
	m.nodeCounts[state.StatusComplete] = 3
	m.totalNodes = 3
	v := m.View()
	if !strings.Contains(v, "All targets eliminated.") {
		t.Errorf("expected all-complete message, got: %s", v)
	}
}

func TestView_AllBlocked(t *testing.T) {
	t.Parallel()
	m := NewDashboardModel()
	m.nodeCounts[state.StatusBlocked] = 5
	m.totalNodes = 5
	v := m.View()
	if !strings.Contains(v, "Blocked on all fronts. Human intervention required.") {
		t.Errorf("expected all-blocked message, got: %s", v)
	}
}

func TestView_Normal_ShowsSections(t *testing.T) {
	t.Parallel()
	m := NewDashboardModel()
	m.nodeCounts[state.StatusComplete] = 2
	m.nodeCounts[state.StatusInProgress] = 1
	m.nodeCounts[state.StatusNotStarted] = 1
	m.totalNodes = 4
	m.daemonStatus = "on patrol"
	m.branch = "main"
	m.daemonRunning = true
	m.uptime = 3*time.Hour + 15*time.Minute

	v := m.View()
	if !strings.Contains(v, "MISSION BRIEFING") {
		t.Errorf("expected MISSION BRIEFING, got: %s", v)
	}
	if !strings.Contains(v, "on patrol") {
		t.Errorf("expected status, got: %s", v)
	}
	if !strings.Contains(v, "main") {
		t.Errorf("expected branch, got: %s", v)
	}
	if !strings.Contains(v, "Progress:") {
		t.Errorf("expected progress section, got: %s", v)
	}
	if !strings.Contains(v, "Audit:") {
		t.Errorf("expected audit section, got: %s", v)
	}
}

func TestView_DaemonNotRunning_NoUptime(t *testing.T) {
	t.Parallel()
	m := NewDashboardModel()
	m.nodeCounts[state.StatusInProgress] = 1
	m.totalNodes = 1
	m.daemonStatus = "standing down"
	m.daemonRunning = false
	v := m.View()
	if !strings.Contains(v, "standing down") {
		t.Errorf("expected 'standing down', got: %s", v)
	}
	if strings.Contains(v, "Uptime:") {
		t.Errorf("expected no uptime when daemon not running, got: %s", v)
	}
}

func TestView_DaemonRunningWithUptime(t *testing.T) {
	t.Parallel()
	m := NewDashboardModel()
	m.nodeCounts[state.StatusInProgress] = 1
	m.totalNodes = 1
	m.daemonStatus = "on patrol"
	m.daemonRunning = true
	m.uptime = 2*time.Hour + 30*time.Minute + 15*time.Second
	v := m.View()
	if !strings.Contains(v, "Uptime: 2h30m15s") {
		t.Errorf("expected uptime in view, got: %s", v)
	}
}

func TestView_NoActivityNoDaemon(t *testing.T) {
	t.Parallel()
	m := NewDashboardModel()
	m.nodeCounts[state.StatusInProgress] = 1
	m.totalNodes = 1
	m.daemonRunning = false
	v := m.View()
	if !strings.Contains(v, "No transmissions. The daemon has not spoken.") {
		t.Errorf("expected no-transmissions message, got: %s", v)
	}
}

func TestView_NoActivityDaemonRunning(t *testing.T) {
	t.Parallel()
	m := NewDashboardModel()
	m.nodeCounts[state.StatusInProgress] = 1
	m.totalNodes = 1
	m.daemonRunning = true
	v := m.View()
	// When daemon is running but no activity, we should NOT see the "not spoken" message
	if strings.Contains(v, "No transmissions. The daemon has not spoken.") {
		t.Errorf("should not show 'not spoken' when daemon is running, got: %s", v)
	}
}

func TestProgressBar_PartialFill(t *testing.T) {
	t.Parallel()
	bar := progressBar(4, 12)
	filled := strings.Count(bar, "█")
	empty := strings.Count(bar, "░")
	if filled != 4 {
		t.Errorf("expected 4 filled chars, got %d", filled)
	}
	if empty != 8 {
		t.Errorf("expected 8 empty chars, got %d", empty)
	}
	if len([]rune(bar)) != progressBarWidth {
		t.Errorf("expected bar width %d, got %d", progressBarWidth, len([]rune(bar)))
	}
}

func TestProgressBar_ZeroFill(t *testing.T) {
	t.Parallel()
	bar := progressBar(0, 12)
	filled := strings.Count(bar, "█")
	empty := strings.Count(bar, "░")
	if filled != 0 {
		t.Errorf("expected 0 filled chars, got %d", filled)
	}
	if empty != progressBarWidth {
		t.Errorf("expected %d empty chars, got %d", progressBarWidth, empty)
	}
}

func TestProgressBar_FullFill(t *testing.T) {
	t.Parallel()
	bar := progressBar(12, 12)
	filled := strings.Count(bar, "█")
	empty := strings.Count(bar, "░")
	if filled != progressBarWidth {
		t.Errorf("expected %d filled chars, got %d", progressBarWidth, filled)
	}
	if empty != 0 {
		t.Errorf("expected 0 empty chars, got %d", empty)
	}
}

func TestProgressBar_ZeroTotal(t *testing.T) {
	t.Parallel()
	bar := progressBar(0, 0)
	empty := strings.Count(bar, "░")
	if empty != progressBarWidth {
		t.Errorf("expected all empty for zero total, got %d empty chars", empty)
	}
}

func TestFormatDuration_Hours(t *testing.T) {
	t.Parallel()
	d := 3*time.Hour + 5*time.Minute + 7*time.Second
	s := formatDuration(d)
	if s != "3h05m07s" {
		t.Errorf("expected '3h05m07s', got %q", s)
	}
}

func TestFormatDuration_Minutes(t *testing.T) {
	t.Parallel()
	d := 42*time.Minute + 3*time.Second
	s := formatDuration(d)
	if s != "42m03s" {
		t.Errorf("expected '42m03s', got %q", s)
	}
}

func TestFormatDuration_Seconds(t *testing.T) {
	t.Parallel()
	d := 15 * time.Second
	s := formatDuration(d)
	if s != "15s" {
		t.Errorf("expected '15s', got %q", s)
	}
}

func TestFormatDuration_Zero(t *testing.T) {
	t.Parallel()
	s := formatDuration(0)
	if s != "0s" {
		t.Errorf("expected '0s', got %q", s)
	}
}

func TestSetSize_StoresDimensions(t *testing.T) {
	t.Parallel()
	m := NewDashboardModel()
	m.SetSize(120, 40)
	if m.width != 120 {
		t.Errorf("expected width 120, got %d", m.width)
	}
	if m.height != 40 {
		t.Errorf("expected height 40, got %d", m.height)
	}
}

func TestView_WithBranch(t *testing.T) {
	t.Parallel()
	m := NewDashboardModel()
	m.nodeCounts[state.StatusInProgress] = 1
	m.totalNodes = 1
	m.branch = "feat/new-thing"
	v := m.View()
	if !strings.Contains(v, "feat/new-thing") {
		t.Errorf("expected branch in view, got: %s", v)
	}
}

func TestView_NoBranch_NoBranchLine(t *testing.T) {
	t.Parallel()
	m := NewDashboardModel()
	m.nodeCounts[state.StatusInProgress] = 1
	m.totalNodes = 1
	m.branch = ""
	v := m.View()
	if strings.Contains(v, "Branch:") {
		t.Errorf("expected no Branch line when branch is empty, got: %s", v)
	}
}

func TestView_ProgressShowsPercentage(t *testing.T) {
	t.Parallel()
	m := NewDashboardModel()
	m.nodeCounts[state.StatusComplete] = 6
	m.nodeCounts[state.StatusInProgress] = 3
	m.nodeCounts[state.StatusNotStarted] = 3
	m.totalNodes = 12
	v := m.View()
	if !strings.Contains(v, "50%") {
		t.Errorf("expected 50%% for 6/12 complete, got: %s", v)
	}
}

func TestView_AuditSection(t *testing.T) {
	t.Parallel()
	m := NewDashboardModel()
	m.nodeCounts[state.StatusInProgress] = 1
	m.totalNodes = 1
	m.auditCounts[state.AuditPassed] = 2
	m.auditCounts[state.AuditInProgress] = 1
	m.auditCounts[state.AuditPending] = 3
	m.openGaps = 1
	m.openEscalations = 2
	v := m.View()
	if !strings.Contains(v, "2 passed") {
		t.Errorf("expected '2 passed' in audit, got: %s", v)
	}
	if !strings.Contains(v, "1 in progress") {
		t.Errorf("expected '1 in progress' in audit, got: %s", v)
	}
	if !strings.Contains(v, "3 pending") {
		t.Errorf("expected '3 pending' in audit, got: %s", v)
	}
	if !strings.Contains(v, "1 open gap(s)") {
		t.Errorf("expected '1 open gap(s)', got: %s", v)
	}
	if !strings.Contains(v, "2 escalation(s)") {
		t.Errorf("expected '2 escalation(s)', got: %s", v)
	}
}

func TestAllComplete_Logic(t *testing.T) {
	t.Parallel()
	m := NewDashboardModel()

	// Zero nodes: not all complete
	if m.allComplete() {
		t.Error("zero nodes should not be allComplete")
	}

	m.totalNodes = 3
	m.nodeCounts[state.StatusComplete] = 2
	if m.allComplete() {
		t.Error("2/3 complete should not be allComplete")
	}

	m.nodeCounts[state.StatusComplete] = 3
	if !m.allComplete() {
		t.Error("3/3 complete should be allComplete")
	}
}

func TestAllBlocked_Logic(t *testing.T) {
	t.Parallel()
	m := NewDashboardModel()

	if m.allBlocked() {
		t.Error("zero nodes should not be allBlocked")
	}

	m.totalNodes = 2
	m.nodeCounts[state.StatusBlocked] = 1
	if m.allBlocked() {
		t.Error("1/2 blocked should not be allBlocked")
	}

	m.nodeCounts[state.StatusBlocked] = 2
	if !m.allBlocked() {
		t.Error("2/2 blocked should be allBlocked")
	}
}

func TestView_ActivityWithTimestamps(t *testing.T) {
	t.Parallel()
	m := NewDashboardModel()
	m.nodeCounts[state.StatusInProgress] = 1
	m.totalNodes = 1
	m.daemonRunning = true
	ts := time.Date(2026, 1, 1, 14, 30, 0, 0, time.UTC)
	m.recentActivity = []activityEntry{
		{timestamp: ts, text: "task completed"},
	}
	v := m.View()
	localHHMM := ts.Local().Format("15:04")
	if !strings.Contains(v, localHHMM) {
		t.Errorf("expected local timestamp %s in activity, got: %s", localHHMM, v)
	}
	if !strings.Contains(v, "task completed") {
		t.Errorf("expected activity text, got: %s", v)
	}
}
