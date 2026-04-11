package detail

import (
	"strings"
	"testing"
	"time"

	"github.com/dorkusprime/wolfcastle/internal/logrender"
	"github.com/dorkusprime/wolfcastle/internal/state"
	"github.com/dorkusprime/wolfcastle/internal/tui"
)

// ---------------------------------------------------------------------------
// summarizeForActivity
// ---------------------------------------------------------------------------

func TestSummarizeForActivity_StageStart_NoNodeTask(t *testing.T) {
	t.Parallel()
	rec := logrender.Record{Type: "stage_start", Stage: "intake"}
	got := summarizeForActivity(rec)
	if got != "▶ intake" {
		t.Errorf("expected '▶ intake', got %q", got)
	}
}

func TestSummarizeForActivity_StageStart_WithNodeTask(t *testing.T) {
	t.Parallel()
	rec := logrender.Record{Type: "stage_start", Stage: "exec", Node: "alpha", Task: "t-001"}
	got := summarizeForActivity(rec)
	if got != "▶ exec alpha/t-001" {
		t.Errorf("expected '▶ exec alpha/t-001', got %q", got)
	}
}

func TestSummarizeForActivity_StageComplete_NoNodeTask(t *testing.T) {
	t.Parallel()
	exit := 0
	rec := logrender.Record{Type: "stage_complete", Stage: "plan", ExitCode: &exit}
	got := summarizeForActivity(rec)
	if got != "✓ plan (exit=0)" {
		t.Errorf("expected '✓ plan (exit=0)', got %q", got)
	}
}

func TestSummarizeForActivity_StageComplete_WithNodeTask(t *testing.T) {
	t.Parallel()
	exit := 1
	rec := logrender.Record{Type: "stage_complete", Stage: "exec", Node: "beta", Task: "t-002", ExitCode: &exit}
	got := summarizeForActivity(rec)
	if got != "✓ exec beta/t-002 (exit=1)" {
		t.Errorf("expected '✓ exec beta/t-002 (exit=1)', got %q", got)
	}
}

func TestSummarizeForActivity_StageComplete_NilExitCode(t *testing.T) {
	t.Parallel()
	rec := logrender.Record{Type: "stage_complete", Stage: "plan"}
	got := summarizeForActivity(rec)
	if got != "✓ plan" {
		t.Errorf("expected '✓ plan', got %q", got)
	}
}

func TestSummarizeForActivity_StageError(t *testing.T) {
	t.Parallel()
	rec := logrender.Record{Type: "stage_error", Stage: "exec", Node: "gamma", Task: "t-003", Error: "timeout"}
	got := summarizeForActivity(rec)
	if got != "✗ exec gamma/t-003: timeout" {
		t.Errorf("expected '✗ exec gamma/t-003: timeout', got %q", got)
	}
}

func TestSummarizeForActivity_FailureIncrement(t *testing.T) {
	t.Parallel()
	rec := logrender.Record{Type: "failure_increment", Node: "delta", Task: "t-004", Counter: 3}
	got := summarizeForActivity(rec)
	if got != "⚠ delta/t-004 failure #3" {
		t.Errorf("expected '⚠ delta/t-004 failure #3', got %q", got)
	}
}

func TestSummarizeForActivity_AutoBlock(t *testing.T) {
	t.Parallel()
	rec := logrender.Record{Type: "auto_block", Node: "epsilon", Task: "t-005", Reason: "too many failures"}
	got := summarizeForActivity(rec)
	if got != "⛔ blocked epsilon/t-005: too many failures" {
		t.Errorf("expected auto_block summary, got %q", got)
	}
}

func TestSummarizeForActivity_DaemonStart(t *testing.T) {
	t.Parallel()
	rec := logrender.Record{Type: "daemon_start"}
	got := summarizeForActivity(rec)
	if got != "Daemon started" {
		t.Errorf("expected 'Daemon started', got %q", got)
	}
}

func TestSummarizeForActivity_DaemonLifecycle_WithEvent(t *testing.T) {
	t.Parallel()
	rec := logrender.Record{Type: "daemon_lifecycle", Event: "shutdown"}
	got := summarizeForActivity(rec)
	if got != "[lifecycle] shutdown" {
		t.Errorf("expected '[lifecycle] shutdown', got %q", got)
	}
}

func TestSummarizeForActivity_DaemonLifecycle_EmptyEvent(t *testing.T) {
	t.Parallel()
	rec := logrender.Record{Type: "daemon_lifecycle", Event: ""}
	got := summarizeForActivity(rec)
	if got != "" {
		t.Errorf("expected empty string for empty lifecycle event, got %q", got)
	}
}

func TestSummarizeForActivity_Unknown(t *testing.T) {
	t.Parallel()
	rec := logrender.Record{Type: "unknown_thing"}
	got := summarizeForActivity(rec)
	if got != "" {
		t.Errorf("expected empty string for unknown type, got %q", got)
	}
}

// ---------------------------------------------------------------------------
// renderStatusBlock: lastActivity and currentNode paths
// ---------------------------------------------------------------------------

func TestView_StatusBlock_WithLastActivity(t *testing.T) {
	t.Parallel()
	m := NewDashboardModel()
	m.nodeCounts[state.StatusInProgress] = 1
	m.totalNodes = 1
	m.daemonStatus = "on patrol"
	m.lastActivity = time.Now().Add(-2 * time.Minute)
	v := m.View()
	if !strings.Contains(v, "Last activity:") {
		t.Errorf("expected Last activity line, got: %s", v)
	}
}

func TestView_StatusBlock_WithCurrentNode(t *testing.T) {
	t.Parallel()
	m := NewDashboardModel()
	m.nodeCounts[state.StatusInProgress] = 1
	m.totalNodes = 1
	m.currentNode = "auth"
	v := m.View()
	if !strings.Contains(v, "Current: auth") {
		t.Errorf("expected 'Current: auth', got: %s", v)
	}
}

func TestView_StatusBlock_WithCurrentNodeAndTask(t *testing.T) {
	t.Parallel()
	m := NewDashboardModel()
	m.nodeCounts[state.StatusInProgress] = 1
	m.totalNodes = 1
	m.currentNode = "auth"
	m.currentTask = "task-0001"
	v := m.View()
	if !strings.Contains(v, "Current: auth/task-0001") {
		t.Errorf("expected 'Current: auth/task-0001', got: %s", v)
	}
}

// ---------------------------------------------------------------------------
// renderInboxSummary
// ---------------------------------------------------------------------------

func TestView_InboxSummary_CountsCorrectly(t *testing.T) {
	t.Parallel()
	m := NewDashboardModel()
	m.nodeCounts[state.StatusInProgress] = 1
	m.totalNodes = 1
	m.inboxItems = []state.InboxItem{
		{Status: state.InboxNew},
		{Status: state.InboxNew},
		{Status: state.InboxFiled},
		{Status: state.InboxNew},
	}
	v := m.View()
	if !strings.Contains(v, "3 new, 1 filed") {
		t.Errorf("expected '3 new, 1 filed', got: %s", v)
	}
}

// ---------------------------------------------------------------------------
// extractAssistantContent
// ---------------------------------------------------------------------------

func TestExtractAssistantContent_PlainText(t *testing.T) {
	t.Parallel()
	got := extractAssistantContent("hello world")
	if got != "hello world" {
		t.Errorf("expected 'hello world', got %q", got)
	}
}

func TestExtractAssistantContent_Empty(t *testing.T) {
	t.Parallel()
	got := extractAssistantContent("")
	if got != "" {
		t.Errorf("expected empty string, got %q", got)
	}
}

func TestExtractAssistantContent_SystemFrame(t *testing.T) {
	t.Parallel()
	got := extractAssistantContent(`{"type":"system","subtype":"init","cwd":"/tmp"}`)
	if got != "" {
		t.Errorf("expected empty string for system frame, got %q", got)
	}
}

func TestExtractAssistantContent_TextBlock(t *testing.T) {
	t.Parallel()
	got := extractAssistantContent(`{"type":"assistant","message":{"content":[{"type":"text","text":"Reviewing the code"}]}}`)
	if got != "Reviewing the code" {
		t.Errorf("expected 'Reviewing the code', got %q", got)
	}
}

func TestExtractAssistantContent_ThinkingBlock(t *testing.T) {
	t.Parallel()
	got := extractAssistantContent(`{"type":"assistant","message":{"content":[{"type":"thinking","thinking":"Let me consider this"}]}}`)
	if !strings.Contains(got, "[thinking]") || !strings.Contains(got, "Let me consider this") {
		t.Errorf("expected thinking block content, got %q", got)
	}
}

func TestExtractAssistantContent_ToolUseBlock(t *testing.T) {
	t.Parallel()
	got := extractAssistantContent(`{"type":"assistant","message":{"content":[{"type":"tool_use","name":"Read"}]}}`)
	if got != "[tool: Read]" {
		t.Errorf("expected '[tool: Read]', got %q", got)
	}
}

func TestExtractAssistantContent_ToolResultBlock(t *testing.T) {
	t.Parallel()
	got := extractAssistantContent(`{"type":"assistant","message":{"content":[{"type":"tool_result"}]}}`)
	if got != "[tool result]" {
		t.Errorf("expected '[tool result]', got %q", got)
	}
}

func TestExtractAssistantContent_MixedBlocks(t *testing.T) {
	t.Parallel()
	got := extractAssistantContent(`{"type":"assistant","message":{"content":[{"type":"text","text":"hello"},{"type":"tool_use","name":"Edit"},{"type":"tool_result"}]}}`)
	if !strings.Contains(got, "hello") || !strings.Contains(got, "[tool: Edit]") || !strings.Contains(got, "[tool result]") {
		t.Errorf("expected all block types, got %q", got)
	}
	if !strings.Contains(got, " | ") {
		t.Errorf("expected pipe separator, got %q", got)
	}
}

func TestExtractAssistantContent_EmptyContentBlocks(t *testing.T) {
	t.Parallel()
	got := extractAssistantContent(`{"type":"assistant","message":{"content":[{"type":"text","text":""},{"type":"thinking","thinking":""},{"type":"tool_use","name":""}]}}`)
	if got != "" {
		t.Errorf("expected empty string for empty content blocks, got %q", got)
	}
}

func TestExtractAssistantContent_LongPlainText_Truncated(t *testing.T) {
	t.Parallel()
	long := strings.Repeat("x", 300)
	got := extractAssistantContent(long)
	if len(got) > 250 {
		t.Errorf("expected truncation, got length %d", len(got))
	}
	if !strings.HasSuffix(got, "…") {
		t.Errorf("expected ellipsis at end, got %q", got[len(got)-5:])
	}
}

// ---------------------------------------------------------------------------
// truncate (logview)
// ---------------------------------------------------------------------------

func TestTruncate_Short(t *testing.T) {
	t.Parallel()
	got := truncate("hello", 10)
	if got != "hello" {
		t.Errorf("expected 'hello', got %q", got)
	}
}

func TestTruncate_Long(t *testing.T) {
	t.Parallel()
	got := truncate(strings.Repeat("a", 50), 10)
	// 10 bytes of 'a' + 3 bytes for UTF-8 "…" = 13 bytes
	if len(got) != 13 {
		t.Errorf("expected length 13 (10 + 3-byte ellipsis), got %d", len(got))
	}
	if !strings.HasSuffix(got, "…") {
		t.Errorf("expected trailing ellipsis, got %q", got)
	}
}

func TestTruncate_NewlinesReplaced(t *testing.T) {
	t.Parallel()
	got := truncate("line1\nline2\nline3", 100)
	if strings.Contains(got, "\n") {
		t.Errorf("expected newlines replaced with spaces, got %q", got)
	}
}

// ---------------------------------------------------------------------------
// wrapBullet
// ---------------------------------------------------------------------------

func TestWrapBullet_ShortText(t *testing.T) {
	t.Parallel()
	got := wrapBullet("hello world", 80)
	if !strings.HasPrefix(got, "  \u2022 ") {
		t.Errorf("expected bullet prefix, got %q", got)
	}
	if !strings.Contains(got, "hello world") {
		t.Errorf("expected text content, got %q", got)
	}
}

func TestWrapBullet_LongText_Wraps(t *testing.T) {
	t.Parallel()
	long := "This is a very long sentence that should definitely wrap when the width is set to something narrow like forty characters"
	got := wrapBullet(long, 40)
	lines := strings.Split(got, "\n")
	if len(lines) < 2 {
		t.Errorf("expected wrapping to multiple lines, got %d line(s)", len(lines))
	}
	// Continuation lines should use "    " indent, not bullet
	if len(lines) > 1 && !strings.HasPrefix(lines[1], "    ") {
		t.Errorf("continuation line should have 4-space indent, got %q", lines[1])
	}
}

func TestWrapBullet_Empty(t *testing.T) {
	t.Parallel()
	got := wrapBullet("", 80)
	if got != "  \u2022 " {
		t.Errorf("expected just bullet prefix for empty text, got %q", got)
	}
}

func TestWrapBullet_NarrowWidth(t *testing.T) {
	t.Parallel()
	got := wrapBullet("some text", 5)
	// Width < 20 is clamped to 20
	if !strings.Contains(got, "some text") {
		t.Errorf("expected text content even at narrow width, got %q", got)
	}
}

// ---------------------------------------------------------------------------
// inboxRelativeTime: hours and days paths
// ---------------------------------------------------------------------------

func TestInboxRelativeTime_HoursAgo(t *testing.T) {
	t.Parallel()
	ts := time.Now().Add(-3 * time.Hour).UTC().Format(time.RFC3339)
	got := inboxRelativeTime(ts)
	if got != "3h ago" {
		t.Errorf("expected '3h ago', got %q", got)
	}
}

func TestInboxRelativeTime_DaysAgo(t *testing.T) {
	t.Parallel()
	ts := time.Now().Add(-48 * time.Hour).UTC().Format(time.RFC3339)
	got := inboxRelativeTime(ts)
	if got != "2d ago" {
		t.Errorf("expected '2d ago', got %q", got)
	}
}

// ---------------------------------------------------------------------------
// ensureVisible (scroll adjustment)
// ---------------------------------------------------------------------------

func TestEnsureVisible_CursorAboveScrollTop(t *testing.T) {
	t.Parallel()
	m := NewInboxModel()
	m.SetSize(80, 24)
	m.SetItems([]state.InboxItem{
		{Text: "a"}, {Text: "b"}, {Text: "c"}, {Text: "d"}, {Text: "e"},
	})
	m.scrollTop = 3
	m.cursor = 1
	m.ensureVisible()
	if m.scrollTop != 1 {
		t.Errorf("expected scrollTop=1, got %d", m.scrollTop)
	}
}

func TestEnsureVisible_CursorBelowViewport(t *testing.T) {
	t.Parallel()
	m := NewInboxModel()
	m.SetSize(80, 10) // height 10, reserved 4 = 6 visible
	m.SetItems([]state.InboxItem{
		{Text: "a"}, {Text: "b"}, {Text: "c"}, {Text: "d"},
		{Text: "e"}, {Text: "f"}, {Text: "g"}, {Text: "h"},
		{Text: "i"}, {Text: "j"},
	})
	m.scrollTop = 0
	m.cursor = 8
	m.ensureVisible()
	if m.scrollTop <= 0 {
		t.Errorf("expected scrollTop > 0 when cursor is below viewport, got %d", m.scrollTop)
	}
}

// ---------------------------------------------------------------------------
// DetailModel: updateNodeDetail and updateTaskDetail paths
// ---------------------------------------------------------------------------

func TestUpdate_NodeDetailMode_KeyPress(t *testing.T) {
	t.Parallel()
	m := NewModel()
	m.SetSize(80, 24)
	m.SetMode(ModeNodeDetail)
	node := makeOrchestratorNode()
	m.nodeDetail.Load("root", node, nil, false)
	m, _ = m.Update(tui.StateUpdatedMsg{Index: &state.RootIndex{
		Nodes: map[string]state.IndexEntry{"a": {State: state.StatusComplete}},
	}})
	// State messages in node detail mode should not panic
}

func TestUpdate_InboxMsgForwarded_WhenNotInInboxMode(t *testing.T) {
	t.Parallel()
	m := NewModel()
	m.SetSize(80, 24)
	// In dashboard mode, InboxUpdatedMsg should still reach the inbox sub-model
	inbox := &state.InboxFile{
		Items: []state.InboxItem{{Text: "forwarded item", Status: state.InboxNew}},
	}
	m, _ = m.Update(tui.InboxUpdatedMsg{Inbox: inbox})
	if len(m.inbox.items) != 1 {
		t.Errorf("expected inbox to receive forwarded items, got %d", len(m.inbox.items))
	}
}

// ---------------------------------------------------------------------------
// DetailModel.Update: LogLinesMsg forwarding when not in log view mode
// ---------------------------------------------------------------------------

func TestUpdate_LogLinesMsg_ForwardedToLogView_WhenInDashboard(t *testing.T) {
	t.Parallel()
	m := NewModel()
	m.SetSize(80, 24)
	// In dashboard mode, LogLinesMsg should still reach the log view sub-model
	m, _ = m.Update(tui.LogLinesMsg{Lines: []string{
		`{"type":"stage_start","stage":"exec","node":"alpha","timestamp":"2026-01-01T00:00:00Z","level":"info"}`,
	}})
	if len(m.logView.lines) != 1 {
		t.Errorf("expected log view to receive forwarded lines, got %d", len(m.logView.lines))
	}
}

func TestUpdate_NewLogFileMsg_ForwardedToLogView_WhenInDashboard(t *testing.T) {
	t.Parallel()
	m := NewModel()
	m.SetSize(80, 24)
	// Pre-add a line so the separator has something to follow
	m.logView.AppendLines([]string{
		`{"type":"daemon_start","timestamp":"2026-01-01T00:00:00Z","level":"info"}`,
	})
	m, _ = m.Update(tui.NewLogFileMsg{Path: "/tmp/0005-exec.jsonl"})
	if len(m.logView.lines) != 2 {
		t.Errorf("expected 2 lines after NewLogFileMsg forwarding, got %d", len(m.logView.lines))
	}
}

// ---------------------------------------------------------------------------
// updateNodeDetail: non-key messages are no-ops
// ---------------------------------------------------------------------------

func TestUpdate_NodeDetailMode_NonKeyMsg_NoOp(t *testing.T) {
	t.Parallel()
	m := NewModel()
	m.SetSize(80, 24)
	m.SetMode(ModeNodeDetail)
	type customMsg struct{}
	_, cmd := m.Update(customMsg{})
	if cmd != nil {
		t.Error("expected nil cmd for non-key message in node detail mode")
	}
}

func TestUpdate_TaskDetailMode_NonKeyMsg_NoOp(t *testing.T) {
	t.Parallel()
	m := NewModel()
	m.SetSize(80, 24)
	m.SetMode(ModeTaskDetail)
	type customMsg struct{}
	_, cmd := m.Update(customMsg{})
	if cmd != nil {
		t.Error("expected nil cmd for non-key message in task detail mode")
	}
}

// ---------------------------------------------------------------------------
// updateInbox: forwarding InboxUpdatedMsg when in inbox mode
// ---------------------------------------------------------------------------

func TestUpdate_InboxMode_ReceivesInboxMsg(t *testing.T) {
	t.Parallel()
	m := NewModel()
	m.SetSize(80, 24)
	m.SetMode(ModeInbox)
	inbox := &state.InboxFile{
		Items: []state.InboxItem{
			{Text: "one", Status: state.InboxNew},
			{Text: "two", Status: state.InboxFiled},
		},
	}
	m, _ = m.Update(tui.InboxUpdatedMsg{Inbox: inbox})
	if len(m.inbox.items) != 2 {
		t.Errorf("expected 2 inbox items, got %d", len(m.inbox.items))
	}
}
