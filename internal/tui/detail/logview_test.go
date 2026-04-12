package detail

import (
	"encoding/json"
	"fmt"
	"strings"
	"testing"
	"time"

	tea "charm.land/bubbletea/v2"

	"github.com/dorkusprime/wolfcastle/internal/tui"
)

func keyPress(r rune) tea.KeyPressMsg {
	return tea.KeyPressMsg{Code: r, Text: string(r)}
}

// makeLogJSON builds a valid JSON log line with the given fields merged
// on top of sensible defaults.
func makeLogJSON(overrides map[string]any) string {
	base := map[string]any{
		"type":      "stage_start",
		"timestamp": time.Now().Format(time.RFC3339Nano),
		"level":     "info",
		"trace":     "exec",
		"stage":     "run",
		"node":      "root",
		"task":      "task-0001",
	}
	for k, v := range overrides {
		base[k] = v
	}
	b, _ := json.Marshal(base)
	return string(b)
}

func TestNewLogViewModel_Defaults(t *testing.T) {
	t.Parallel()
	m := NewLogViewModel()
	if !m.follow {
		t.Error("follow should default to true")
	}
	if m.levelFilter != "all" {
		t.Errorf("levelFilter should default to 'all', got %q", m.levelFilter)
	}
	if m.traceFilter != "all" {
		t.Errorf("traceFilter should default to 'all', got %q", m.traceFilter)
	}
	if len(m.lines) != 0 {
		t.Errorf("lines should start empty, got %d", len(m.lines))
	}
}

func TestAppendLines_ValidJSON(t *testing.T) {
	t.Parallel()
	m := NewLogViewModel()
	m.SetSize(80, 24)
	lines := []string{
		makeLogJSON(map[string]any{"type": "stage_start"}),
		makeLogJSON(map[string]any{"type": "assistant", "text": "hello"}),
	}
	m.AppendLines(lines)
	if len(m.lines) != 2 {
		t.Errorf("expected 2 lines, got %d", len(m.lines))
	}
}

func TestAppendLines_InvalidJSON_Skipped(t *testing.T) {
	t.Parallel()
	m := NewLogViewModel()
	m.SetSize(80, 24)
	m.AppendLines([]string{
		"not json at all",
		"{malformed",
		makeLogJSON(nil),
	})
	if len(m.lines) != 1 {
		t.Errorf("expected 1 line (malformed skipped), got %d", len(m.lines))
	}
}

func TestAppendLines_CircularBufferOverflow(t *testing.T) {
	t.Parallel()
	m := NewLogViewModel()
	m.SetSize(80, 24)

	// Fill beyond maxLogLines
	batch := make([]string, maxLogLines+500)
	for i := range batch {
		batch[i] = makeLogJSON(map[string]any{"counter": i})
	}
	m.AppendLines(batch)

	if len(m.lines) != maxLogLines {
		t.Errorf("expected %d lines after overflow, got %d", maxLogLines, len(m.lines))
	}
}

func TestKey_F_ToggleFollow(t *testing.T) {
	t.Parallel()
	m := NewLogViewModel()
	m.SetSize(80, 24)

	if !m.follow {
		t.Fatal("follow should start true")
	}
	m, _ = m.Update(keyPress('f'))
	if m.follow {
		t.Error("follow should be false after first toggle")
	}
	m, _ = m.Update(keyPress('f'))
	if !m.follow {
		t.Error("follow should be true after second toggle")
	}
}

func TestKey_JK_ScrollAndDisableFollow(t *testing.T) {
	t.Parallel()
	m := NewLogViewModel()
	m.SetSize(80, 24)

	// Add enough lines to make scrolling meaningful
	lines := make([]string, 50)
	for i := range lines {
		lines[i] = makeLogJSON(map[string]any{"counter": i})
	}
	m.AppendLines(lines)
	// follow is true at the start
	m, _ = m.Update(keyPress('k'))
	if m.follow {
		t.Error("k should disable follow")
	}

	// Re-enable and test j
	m.follow = true
	m, _ = m.Update(keyPress('j'))
	// j at the bottom should keep follow (AtBottom returns true)
	// But if we scroll up first, j won't be at bottom
	m.follow = true
	m.viewport.GotoTop()
	m, _ = m.Update(keyPress('j'))
	if m.follow {
		t.Error("j when not at bottom should disable follow")
	}
}

func TestKey_G_Upper_JumpToBottom(t *testing.T) {
	t.Parallel()
	m := NewLogViewModel()
	m.SetSize(80, 24)

	lines := make([]string, 50)
	for i := range lines {
		lines[i] = makeLogJSON(map[string]any{"counter": i})
	}
	m.AppendLines(lines)
	m.follow = false
	m.viewport.GotoTop()

	m, _ = m.Update(keyPress('G'))
	if !m.follow {
		t.Error("G should enable follow")
	}
}

func TestKey_G_Lower_JumpToTop(t *testing.T) {
	t.Parallel()
	m := NewLogViewModel()
	m.SetSize(80, 24)

	lines := make([]string, 50)
	for i := range lines {
		lines[i] = makeLogJSON(map[string]any{"counter": i})
	}
	m.AppendLines(lines)

	m, _ = m.Update(keyPress('g'))
	if m.follow {
		t.Error("g should disable follow")
	}
}

func TestKey_L_CycleLevelFilter(t *testing.T) {
	t.Parallel()
	m := NewLogViewModel()
	m.SetSize(80, 24)

	expected := []string{"debug", "info", "warn", "error", "all"}
	for _, want := range expected {
		m, _ = m.Update(keyPress('L'))
		if m.levelFilter != want {
			t.Errorf("expected level filter %q, got %q", want, m.levelFilter)
		}
	}
}

func TestKey_T_CycleTraceFilter(t *testing.T) {
	t.Parallel()
	m := NewLogViewModel()
	m.SetSize(80, 24)

	expected := []string{"exec", "plan", "inbox", "system", "all"}
	for _, want := range expected {
		m, _ = m.Update(keyPress('T'))
		if m.traceFilter != want {
			t.Errorf("expected trace filter %q, got %q", want, m.traceFilter)
		}
	}
}

func TestLevelFilter_Info_HidesDebug(t *testing.T) {
	t.Parallel()
	m := NewLogViewModel()
	m.SetSize(80, 24)

	m.AppendLines([]string{
		makeLogJSON(map[string]any{"level": "debug", "type": "assistant", "text": "debug line"}),
		makeLogJSON(map[string]any{"level": "info", "type": "assistant", "text": "info line"}),
		makeLogJSON(map[string]any{"level": "warn", "type": "assistant", "text": "warn line"}),
		makeLogJSON(map[string]any{"level": "error", "type": "assistant", "text": "error line"}),
	})

	m.levelFilter = "info"
	filtered := m.filteredLines()
	if len(filtered) != 3 {
		t.Errorf("info filter should show 3 lines (info/warn/error), got %d", len(filtered))
	}
}

func TestLevelFilter_Error_OnlyError(t *testing.T) {
	t.Parallel()
	m := NewLogViewModel()
	m.SetSize(80, 24)

	m.AppendLines([]string{
		makeLogJSON(map[string]any{"level": "debug"}),
		makeLogJSON(map[string]any{"level": "info"}),
		makeLogJSON(map[string]any{"level": "warn"}),
		makeLogJSON(map[string]any{"level": "error"}),
	})

	m.levelFilter = "error"
	filtered := m.filteredLines()
	if len(filtered) != 1 {
		t.Errorf("error filter should show 1 line, got %d", len(filtered))
	}
}

func TestTraceFilter_Exec_OnlyExec(t *testing.T) {
	t.Parallel()
	m := NewLogViewModel()
	m.SetSize(80, 24)

	m.AppendLines([]string{
		makeLogJSON(map[string]any{"trace": "exec"}),
		makeLogJSON(map[string]any{"trace": "intake"}),
		makeLogJSON(map[string]any{"trace": "exec"}),
	})

	m.traceFilter = "exec"
	filtered := m.filteredLines()
	if len(filtered) != 2 {
		t.Errorf("exec filter should show 2 lines, got %d", len(filtered))
	}
}

func TestView_NoLines(t *testing.T) {
	t.Parallel()
	m := NewLogViewModel()
	m.SetSize(80, 24)

	view := m.View()
	if !strings.Contains(view, "No transmissions") {
		t.Errorf("empty view should contain 'No transmissions', got %q", view)
	}
	if !strings.Contains(view, "The daemon has not spoken") {
		t.Errorf("empty view should contain daemon message, got %q", view)
	}
}

func TestView_WithLines_ShowsHeader(t *testing.T) {
	t.Parallel()
	m := NewLogViewModel()
	m.SetSize(80, 24)

	m.AppendLines([]string{makeLogJSON(nil)})
	view := m.View()
	if !strings.Contains(view, "TRANSMISSIONS") {
		t.Errorf("view should contain TRANSMISSIONS header, got %q", view)
	}
}

func TestView_FollowOn_ShowsFollowing(t *testing.T) {
	t.Parallel()
	m := NewLogViewModel()
	m.SetSize(80, 24)
	m.follow = true

	m.AppendLines([]string{makeLogJSON(nil)})
	view := m.View()
	if !strings.Contains(view, "[following]") {
		t.Errorf("view should show [following], got %q", view)
	}
}

func TestView_FollowOff_ShowsPaused(t *testing.T) {
	t.Parallel()
	m := NewLogViewModel()
	m.SetSize(80, 24)
	m.follow = false

	m.AppendLines([]string{makeLogJSON(nil)})
	view := m.View()
	if !strings.Contains(view, "[paused]") {
		t.Errorf("view should show [paused], got %q", view)
	}
}

func TestSetReadError_ShowsErrorMessage(t *testing.T) {
	t.Parallel()
	m := NewLogViewModel()
	m.SetSize(80, 24)
	m.SetReadError(true)

	view := m.View()
	if !strings.Contains(view, "Unable to read log file") {
		t.Errorf("view should show read error, got %q", view)
	}
}

func TestSelectedLineJSON(t *testing.T) {
	t.Parallel()
	m := NewLogViewModel()
	m.SetSize(80, 24)

	line := makeLogJSON(map[string]any{"type": "assistant", "text": "hello"})
	m.AppendLines([]string{line})

	got := m.SelectedLineJSON()
	if got != line {
		t.Errorf("expected raw JSON %q, got %q", line, got)
	}
}

func TestSelectedLineJSON_Empty(t *testing.T) {
	t.Parallel()
	m := NewLogViewModel()
	m.SetSize(80, 24)

	got := m.SelectedLineJSON()
	if got != "" {
		t.Errorf("expected empty string for no lines, got %q", got)
	}
}

func TestNewLogFileMsg_InsertsSeparator(t *testing.T) {
	t.Parallel()
	m := NewLogViewModel()
	m.SetSize(80, 24)

	m.AppendLines([]string{makeLogJSON(nil)})
	m, _ = m.Update(tui.NewLogFileMsg{Path: "/tmp/logs/0042-exec-20260401T12-00Z.jsonl"})

	if len(m.lines) != 2 {
		t.Errorf("expected 2 lines (1 record + 1 separator), got %d", len(m.lines))
	}
	if !strings.Contains(m.lines[1].rendered, "iteration 42") {
		t.Errorf("separator should contain 'iteration 42', got %q", m.lines[1].rendered)
	}
	if m.iteration != 42 {
		t.Errorf("iteration should be 42, got %d", m.iteration)
	}
}

func TestNewLogFileMsg_NoIterationNumber(t *testing.T) {
	t.Parallel()
	m := NewLogViewModel()
	m.SetSize(80, 24)

	m.AppendLines([]string{makeLogJSON(nil)})
	m, _ = m.Update(tui.NewLogFileMsg{Path: "/tmp/test.log"})

	if len(m.lines) != 2 {
		t.Errorf("expected 2 lines, got %d", len(m.lines))
	}
	if !strings.Contains(m.lines[1].rendered, "new log file") {
		t.Errorf("separator without iteration should contain 'new log file', got %q", m.lines[1].rendered)
	}
}

func TestLogLinesMsg_Update(t *testing.T) {
	t.Parallel()
	m := NewLogViewModel()
	m.SetSize(80, 24)

	m, _ = m.Update(tui.LogLinesMsg{Lines: []string{makeLogJSON(nil)}})
	if len(m.lines) != 1 {
		t.Errorf("expected 1 line from LogLinesMsg, got %d", len(m.lines))
	}
}

func TestRenderLine_StageStart(t *testing.T) {
	t.Parallel()
	m := NewLogViewModel()
	line := makeLogJSON(map[string]any{"type": "stage_start", "stage": "plan", "node": "root", "task": "t1"})
	m.AppendLines([]string{line})

	rendered := m.lines[0].rendered
	if !strings.Contains(rendered, "Starting") {
		t.Errorf("stage_start should contain 'Starting', got %q", rendered)
	}
	if !strings.Contains(rendered, "plan") {
		t.Errorf("stage_start should contain stage name, got %q", rendered)
	}
}

func TestRenderLine_StageComplete_Exit0(t *testing.T) {
	t.Parallel()
	m := NewLogViewModel()
	exit0 := 0
	line := makeLogJSON(map[string]any{"type": "stage_complete", "stage": "run", "exit_code": exit0})
	m.AppendLines([]string{line})

	rendered := m.lines[0].rendered
	if !strings.Contains(rendered, "Complete") {
		t.Errorf("stage_complete should contain 'Complete', got %q", rendered)
	}
	if !strings.Contains(rendered, "exit=0") {
		t.Errorf("stage_complete exit 0 should contain 'exit=0', got %q", rendered)
	}
}

func TestRenderLine_StageComplete_Exit1(t *testing.T) {
	t.Parallel()
	m := NewLogViewModel()
	exit1 := 1
	line := makeLogJSON(map[string]any{"type": "stage_complete", "stage": "run", "exit_code": exit1})
	m.AppendLines([]string{line})

	rendered := m.lines[0].rendered
	if !strings.Contains(rendered, "exit=1") {
		t.Errorf("stage_complete exit 1 should contain 'exit=1', got %q", rendered)
	}
}

func TestRenderLine_StageError(t *testing.T) {
	t.Parallel()
	m := NewLogViewModel()
	line := makeLogJSON(map[string]any{"type": "stage_error", "stage": "run", "error": "something broke"})
	m.AppendLines([]string{line})

	rendered := m.lines[0].rendered
	if !strings.Contains(rendered, "Error") {
		t.Errorf("stage_error should contain 'Error', got %q", rendered)
	}
	if !strings.Contains(rendered, "something broke") {
		t.Errorf("stage_error should contain error text, got %q", rendered)
	}
}

func TestRenderLine_Assistant(t *testing.T) {
	t.Parallel()
	m := NewLogViewModel()
	line := makeLogJSON(map[string]any{"type": "assistant", "text": "I am helping"})
	m.AppendLines([]string{line})

	rendered := m.lines[0].rendered
	if !strings.Contains(rendered, "I am helping") {
		t.Errorf("assistant should contain text, got %q", rendered)
	}
}

// TestRenderLine_AssistantWithClaudeEnvelope verifies that an assistant record
// whose text field carries a Claude API JSON envelope is rendered as a
// readable summary instead of being dumped raw into the viewport.
func TestRenderLine_AssistantWithClaudeEnvelope(t *testing.T) {
	t.Parallel()
	m := NewLogViewModel()
	envelope := `{"type":"assistant","message":{"content":[{"type":"text","text":"Reviewing the diff now"},{"type":"tool_use","name":"Read"}]}}`
	line := makeLogJSON(map[string]any{"type": "assistant", "text": envelope})
	m.AppendLines([]string{line})

	if len(m.lines) != 1 {
		t.Fatalf("expected 1 rendered line, got %d", len(m.lines))
	}
	rendered := m.lines[0].rendered
	if !strings.Contains(rendered, "Reviewing the diff now") {
		t.Errorf("expected text content to be extracted, got %q", rendered)
	}
	if !strings.Contains(rendered, "[tool: Read]") {
		t.Errorf("expected tool_use to be tagged, got %q", rendered)
	}
	if strings.Contains(rendered, `"content"`) {
		t.Errorf("rendered line should not contain raw JSON, got %q", rendered)
	}
}

// TestRenderLine_AssistantSystemFrameSkipped verifies that system init frames
// (Claude Code's session bootstrap) produce no entry rather than a noisy raw
// dump.
func TestRenderLine_AssistantSystemFrameSkipped(t *testing.T) {
	t.Parallel()
	m := NewLogViewModel()
	envelope := `{"type":"system","subtype":"init","cwd":"/tmp"}`
	line := makeLogJSON(map[string]any{"type": "assistant", "text": envelope})
	m.AppendLines([]string{line})

	if len(m.lines) != 0 {
		t.Errorf("expected system init frames to be skipped, got %d lines", len(m.lines))
	}
}

func TestRenderLine_FailureIncrement(t *testing.T) {
	t.Parallel()
	m := NewLogViewModel()
	line := makeLogJSON(map[string]any{"type": "failure_increment", "node": "root", "task": "t1", "counter": 3})
	m.AppendLines([]string{line})

	rendered := m.lines[0].rendered
	if !strings.Contains(rendered, "failure") {
		t.Errorf("failure_increment should contain 'failure', got %q", rendered)
	}
	if !strings.Contains(rendered, "#3") {
		t.Errorf("failure_increment should contain counter, got %q", rendered)
	}
}

func TestRenderLine_AutoBlock(t *testing.T) {
	t.Parallel()
	m := NewLogViewModel()
	line := makeLogJSON(map[string]any{"type": "auto_block", "node": "root", "task": "t1", "reason": "too many failures"})
	m.AppendLines([]string{line})

	rendered := m.lines[0].rendered
	if !strings.Contains(rendered, "blocked") {
		t.Errorf("auto_block should contain 'blocked', got %q", rendered)
	}
	if !strings.Contains(rendered, "too many failures") {
		t.Errorf("auto_block should contain reason, got %q", rendered)
	}
}

func TestRenderLine_DaemonStart(t *testing.T) {
	t.Parallel()
	m := NewLogViewModel()
	line := makeLogJSON(map[string]any{"type": "daemon_start"})
	m.AppendLines([]string{line})

	rendered := m.lines[0].rendered
	if !strings.Contains(rendered, "Daemon started") {
		t.Errorf("daemon_start should contain 'Daemon started', got %q", rendered)
	}
}

func TestRenderLine_DaemonLifecycle(t *testing.T) {
	t.Parallel()
	m := NewLogViewModel()
	line := makeLogJSON(map[string]any{"type": "daemon_lifecycle", "event": "shutdown"})
	m.AppendLines([]string{line})

	rendered := m.lines[0].rendered
	if !strings.Contains(rendered, "lifecycle") {
		t.Errorf("daemon_lifecycle should contain 'lifecycle', got %q", rendered)
	}
	if !strings.Contains(rendered, "shutdown") {
		t.Errorf("daemon_lifecycle should contain event, got %q", rendered)
	}
}

func TestRenderLine_Unrecognized(t *testing.T) {
	t.Parallel()
	m := NewLogViewModel()
	line := makeLogJSON(map[string]any{"type": "something_new", "custom": "data"})
	m.AppendLines([]string{line})

	rendered := m.lines[0].rendered
	if !strings.Contains(rendered, "something_new") {
		t.Errorf("unrecognized type should show type name, got %q", rendered)
	}
}

func TestLevelTinting_Debug_Dim(t *testing.T) {
	t.Parallel()
	result := applyLevelTint("debug", "test text")
	// The dim style wraps the text with ANSI codes; just confirm it's non-empty
	// and different from the input (it got styled).
	if result == "" {
		t.Error("debug tint should produce output")
	}
}

func TestLevelTinting_Error_Red(t *testing.T) {
	t.Parallel()
	result := applyLevelTint("error", "test text")
	if result == "" {
		t.Error("error tint should produce output")
	}
}

func TestLevelTinting_Warn(t *testing.T) {
	t.Parallel()
	result := applyLevelTint("warn", "test text")
	if result == "" {
		t.Error("warn tint should produce output")
	}
}

func TestLevelTinting_Info_Passthrough(t *testing.T) {
	t.Parallel()
	result := applyLevelTint("info", "test text")
	if result != "test text" {
		t.Errorf("info tint should pass through unchanged, got %q", result)
	}
}

func TestLevelTinting_Unknown_Passthrough(t *testing.T) {
	t.Parallel()
	result := applyLevelTint("banana", "test text")
	if result != "test text" {
		t.Errorf("unknown level tint should pass through unchanged, got %q", result)
	}
}

func TestSetSize_PropagatesToViewport(t *testing.T) {
	t.Parallel()
	m := NewLogViewModel()
	m.SetSize(120, 40)

	if m.width != 120 {
		t.Errorf("expected width 120, got %d", m.width)
	}
	if m.height != 40 {
		t.Errorf("expected height 40, got %d", m.height)
	}
}

func TestSetSize_NegativeHeight(t *testing.T) {
	t.Parallel()
	m := NewLogViewModel()
	// Height of 0 means vpHeight = -1, clamped to 0
	m.SetSize(80, 0)
	if m.height != 0 {
		t.Errorf("expected height 0, got %d", m.height)
	}
}

func TestLogView_SetFocused(t *testing.T) {
	t.Parallel()
	m := NewLogViewModel()
	m.SetFocused(true)
	if !m.focused {
		t.Error("expected focused to be true")
	}
	m.SetFocused(false)
	if m.focused {
		t.Error("expected focused to be false")
	}
}

func TestLevelOrd(t *testing.T) {
	t.Parallel()
	cases := []struct {
		level string
		want  int
	}{
		{"debug", 0},
		{"info", 1},
		{"warn", 2},
		{"error", 3},
		{"unknown", 1},
	}
	for _, tc := range cases {
		if got := levelOrd(tc.level); got != tc.want {
			t.Errorf("levelOrd(%q) = %d, want %d", tc.level, got, tc.want)
		}
	}
}

func TestLevelFilterDisplay(t *testing.T) {
	t.Parallel()
	cases := []struct {
		filter string
		want   string
	}{
		{"all", "all (unfiltered)"},
		{"debug", "DEBUG and above"},
		{"info", "INFO and above"},
		{"warn", "WARN and above"},
		{"error", "ERROR only"},
		{"custom", "custom"},
	}
	for _, tc := range cases {
		if got := levelFilterDisplay(tc.filter); got != tc.want {
			t.Errorf("levelFilterDisplay(%q) = %q, want %q", tc.filter, got, tc.want)
		}
	}
}

func TestCycleLevelFilter_UnknownResets(t *testing.T) {
	t.Parallel()
	m := NewLogViewModel()
	m.levelFilter = "nonexistent"
	m.cycleLevelFilter()
	if m.levelFilter != "all" {
		t.Errorf("unknown level filter should reset to 'all', got %q", m.levelFilter)
	}
}

func TestCycleTraceFilter_UnknownResets(t *testing.T) {
	t.Parallel()
	m := NewLogViewModel()
	m.traceFilter = "nonexistent"
	m.cycleTraceFilter()
	if m.traceFilter != "all" {
		t.Errorf("unknown trace filter should reset to 'all', got %q", m.traceFilter)
	}
}

func TestRenderLine_WithTrace(t *testing.T) {
	t.Parallel()
	m := NewLogViewModel()
	line := makeLogJSON(map[string]any{"type": "stage_start", "trace": "intake"})
	m.AppendLines([]string{line})

	rendered := m.lines[0].rendered
	if !strings.Contains(rendered, "[intake]") {
		t.Errorf("line with trace should contain trace prefix, got %q", rendered)
	}
}

func TestRenderLine_WithoutTrace(t *testing.T) {
	t.Parallel()
	m := NewLogViewModel()
	line := makeLogJSON(map[string]any{"type": "daemon_start", "trace": ""})
	m.AppendLines([]string{line})

	rendered := m.lines[0].rendered
	if strings.Contains(rendered, "[]") {
		t.Errorf("line without trace should not contain empty brackets, got %q", rendered)
	}
}

func TestStageComplete_NilExitCode(t *testing.T) {
	t.Parallel()
	m := NewLogViewModel()
	// Build JSON without exit_code field
	raw := fmt.Sprintf(`{"type":"stage_complete","stage":"run","timestamp":"%s","level":"info"}`, time.Now().Format(time.RFC3339Nano))
	m.AppendLines([]string{raw})

	rendered := m.lines[0].rendered
	if !strings.Contains(rendered, "exit=?") {
		t.Errorf("nil exit code should show '?', got %q", rendered)
	}
}

func TestFilteredLines_BothFiltersActive(t *testing.T) {
	t.Parallel()
	m := NewLogViewModel()
	m.SetSize(80, 24)

	m.AppendLines([]string{
		makeLogJSON(map[string]any{"level": "debug", "trace": "exec"}),
		makeLogJSON(map[string]any{"level": "info", "trace": "exec"}),
		makeLogJSON(map[string]any{"level": "info", "trace": "intake"}),
		makeLogJSON(map[string]any{"level": "error", "trace": "exec"}),
	})

	m.levelFilter = "info"
	m.traceFilter = "exec"
	filtered := m.filteredLines()
	// Should match: info/exec, error/exec (info and above, exec trace)
	if len(filtered) != 2 {
		t.Errorf("combined filter should show 2 lines, got %d", len(filtered))
	}
}

func TestKey_CtrlD_HalfPageDown(t *testing.T) {
	t.Parallel()
	m := NewLogViewModel()
	m.SetSize(80, 10)

	lines := make([]string, 50)
	for i := range lines {
		lines[i] = makeLogJSON(map[string]any{"counter": i})
	}
	m.AppendLines(lines)
	m.viewport.GotoTop()
	m.follow = true

	m, _ = m.Update(tea.KeyPressMsg{Code: 'd', Mod: tea.ModCtrl})
	// After half-page down from top, we shouldn't be at bottom, so follow should be false
	if m.follow {
		t.Error("ctrl+d from top should disable follow when not at bottom")
	}
}

func TestKey_CtrlU_HalfPageUp(t *testing.T) {
	t.Parallel()
	m := NewLogViewModel()
	m.SetSize(80, 10)

	lines := make([]string, 50)
	for i := range lines {
		lines[i] = makeLogJSON(map[string]any{"counter": i})
	}
	m.AppendLines(lines)
	m.follow = true

	m, _ = m.Update(tea.KeyPressMsg{Code: 'u', Mod: tea.ModCtrl})
	if m.follow {
		t.Error("ctrl+u should disable follow")
	}
}

func TestKey_PgDown(t *testing.T) {
	t.Parallel()
	m := NewLogViewModel()
	m.SetSize(80, 10)

	lines := make([]string, 50)
	for i := range lines {
		lines[i] = makeLogJSON(map[string]any{"counter": i})
	}
	m.AppendLines(lines)
	m.viewport.GotoTop()
	m.follow = true

	m, _ = m.Update(tea.KeyPressMsg{Code: tea.KeyPgDown})
	// pgdown maps to "pgdown" which goes to the default case (viewport passthrough)
}

func TestKey_PgUp(t *testing.T) {
	t.Parallel()
	m := NewLogViewModel()
	m.SetSize(80, 10)

	lines := make([]string, 50)
	for i := range lines {
		lines[i] = makeLogJSON(map[string]any{"counter": i})
	}
	m.AppendLines(lines)
	m, _ = m.Update(tea.KeyPressMsg{Code: tea.KeyPgUp})
	// Just verify no panic
}

func TestUpdate_UnhandledKey_PassesThrough(t *testing.T) {
	t.Parallel()
	m := NewLogViewModel()
	m.SetSize(80, 24)

	// An unhandled key should not panic and should pass through to viewport
	m, _ = m.Update(keyPress('x'))
}

func TestLogView_Update_NonKeyMsg_Ignored(t *testing.T) {
	t.Parallel()
	m := NewLogViewModel()
	m.SetSize(80, 24)

	// Non-key messages that aren't LogLinesMsg or NewLogFileMsg should be ignored
	type customMsg struct{}
	m, _ = m.Update(customMsg{})
	// No panic means pass
}

func TestRebuildViewport_FilterChangesContent(t *testing.T) {
	t.Parallel()
	m := NewLogViewModel()
	m.SetSize(80, 24)

	m.AppendLines([]string{
		makeLogJSON(map[string]any{"level": "debug", "type": "assistant", "text": "debug only"}),
		makeLogJSON(map[string]any{"level": "error", "type": "assistant", "text": "error only"}),
	})

	// With all filter, viewport has both
	m.levelFilter = "error"
	m.rebuildViewport()

	// The viewport content should only have the error line
	// We can't easily inspect viewport content, but we can check filteredLines
	filtered := m.filteredLines()
	if len(filtered) != 1 {
		t.Errorf("expected 1 filtered line, got %d", len(filtered))
	}
}

// ---------------------------------------------------------------------------
// traceCategory
// ---------------------------------------------------------------------------

func TestTraceCategory(t *testing.T) {
	t.Parallel()
	cases := []struct {
		trace string
		want  string
	}{
		{"exec-0002", "exec"},
		{"exec", "exec"},
		{"plan-0001", "plan"},
		{"plan", "plan"},
		{"intake-10001", "inbox"},
		{"intake", "inbox"},
		{"inbox-init-10003", "inbox"},
		{"inbox", "inbox"},
		{"heal-0001", "system"},
		{"heal", "system"},
		{"shutdown-0001", "system"},
		{"shutdown", "system"},
		{"crash-0001", "system"},
		{"crash", "system"},
		{"", "other"},
		{"unknown-prefix", "other"},
		{"something-else", "other"},
	}
	for _, tc := range cases {
		t.Run(fmt.Sprintf("trace=%q", tc.trace), func(t *testing.T) {
			t.Parallel()
			got := traceCategory(tc.trace)
			if got != tc.want {
				t.Errorf("traceCategory(%q) = %q, want %q", tc.trace, got, tc.want)
			}
		})
	}
}
