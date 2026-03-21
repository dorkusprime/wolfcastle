package logrender

import (
	"bytes"
	"context"
	"fmt"
	"strings"
	"testing"
	"time"
)

// localTS converts a UTC time string to its local HH:MM:SS representation,
// matching what formatTimestamp produces regardless of the test machine's timezone.
func localTS(utc string) string {
	return makeTime(utc).Local().Format("15:04:05")
}

func TestInterleavedRenderer_StageStartComplete(t *testing.T) {
	recs := []Record{
		{Type: "stage_start", Stage: "execute", Node: "my-project", Task: "task-0001", Timestamp: makeTime("2026-03-21T18:01:34Z")},
		{Type: "stage_complete", Stage: "execute", Node: "my-project", Task: "task-0001", Timestamp: makeTime("2026-03-21T18:02:56Z"), ExitCode: intPtr(0)},
	}

	var buf bytes.Buffer
	NewInterleavedRenderer(&buf).Render(context.Background(), feedRecords(recs))

	got := buf.String()
	expectContains(t, got, fmt.Sprintf("%s ▶ [execute] my-project/task-0001", localTS("2026-03-21T18:01:34Z")))
	expectContains(t, got, fmt.Sprintf("%s ✓ [execute] my-project/task-0001 (1m22s)", localTS("2026-03-21T18:02:56Z")))
}

func TestInterleavedRenderer_FailedStage(t *testing.T) {
	recs := []Record{
		{Type: "stage_start", Stage: "execute", Node: "proj", Task: "task-0001", Timestamp: makeTime("2026-03-21T10:00:00Z")},
		{Type: "stage_complete", Stage: "execute", Node: "proj", Task: "task-0001", Timestamp: makeTime("2026-03-21T10:03:41Z"), ExitCode: intPtr(1)},
	}

	var buf bytes.Buffer
	NewInterleavedRenderer(&buf).Render(context.Background(), feedRecords(recs))

	got := buf.String()
	expectContains(t, got, fmt.Sprintf("%s ▶ [execute] proj/task-0001", localTS("2026-03-21T10:00:00Z")))
	expectContains(t, got, fmt.Sprintf("%s ✗ [execute] proj/task-0001 (3m41s)", localTS("2026-03-21T10:03:41Z")))
}

func TestInterleavedRenderer_AssistantText(t *testing.T) {
	recs := []Record{
		{Type: "assistant", Text: "I'll start by reading the file...", Timestamp: makeTime("2026-03-21T18:01:35Z")},
		{Type: "assistant", Text: "Reading the project requirements...", Timestamp: makeTime("2026-03-21T18:01:36Z")},
	}

	var buf bytes.Buffer
	NewInterleavedRenderer(&buf).Render(context.Background(), feedRecords(recs))

	got := buf.String()
	expectContains(t, got, fmt.Sprintf("%s     I'll start by reading the file...", localTS("2026-03-21T18:01:35Z")))
	expectContains(t, got, fmt.Sprintf("%s     Reading the project requirements...", localTS("2026-03-21T18:01:36Z")))
}

func TestInterleavedRenderer_AssistantIndentation(t *testing.T) {
	recs := []Record{
		{Type: "stage_start", Stage: "execute", Node: "proj", Task: "task-0001", Timestamp: makeTime("2026-03-21T18:01:34Z")},
		{Type: "assistant", Text: "thinking...", Timestamp: makeTime("2026-03-21T18:01:35Z")},
	}

	var buf bytes.Buffer
	NewInterleavedRenderer(&buf).Render(context.Background(), feedRecords(recs))

	lines := strings.Split(strings.TrimRight(buf.String(), "\n"), "\n")
	if len(lines) != 2 {
		t.Fatalf("expected 2 lines, got %d: %q", len(lines), buf.String())
	}

	// Assistant line should have 4-space indent after the timestamp.
	if !strings.Contains(lines[1], "     thinking...") {
		t.Errorf("expected 4-space indented text, got: %q", lines[1])
	}
}

func TestInterleavedRenderer_SkipsEmptyAssistant(t *testing.T) {
	recs := []Record{
		{Type: "assistant", Text: "First.", Timestamp: makeTime("2026-03-21T18:01:35Z")},
		{Type: "assistant", Text: "", Timestamp: makeTime("2026-03-21T18:01:36Z")},
		{Type: "assistant", Text: "Third.", Timestamp: makeTime("2026-03-21T18:01:37Z")},
	}

	var buf bytes.Buffer
	NewInterleavedRenderer(&buf).Render(context.Background(), feedRecords(recs))

	got := buf.String()
	lineCount := strings.Count(got, "\n")
	if lineCount != 2 {
		t.Errorf("expected 2 lines (skipping empty), got %d:\n%s", lineCount, got)
	}
}

func TestInterleavedRenderer_PlanningRecords(t *testing.T) {
	recs := []Record{
		{Type: "planning_start", Node: "proj", Timestamp: makeTime("2026-03-21T10:00:00Z")},
		{Type: "planning_complete", Node: "proj", Timestamp: makeTime("2026-03-21T10:00:45Z"), ExitCode: intPtr(0)},
	}

	var buf bytes.Buffer
	NewInterleavedRenderer(&buf).Render(context.Background(), feedRecords(recs))

	got := buf.String()
	expectContains(t, got, fmt.Sprintf("%s ▶ [plan] proj", localTS("2026-03-21T10:00:00Z")))
	expectContains(t, got, fmt.Sprintf("%s ✓ [plan] proj (45s)", localTS("2026-03-21T10:00:45Z")))
}

func TestInterleavedRenderer_AuditReportWritten(t *testing.T) {
	recs := []Record{
		{Type: "stage_start", Stage: "audit", Node: "proj/sub", Timestamp: makeTime("2026-03-21T10:00:00Z")},
		{Type: "stage_complete", Stage: "audit", Node: "proj/sub", Timestamp: makeTime("2026-03-21T10:00:34Z"), ExitCode: intPtr(0)},
		{Type: "audit_report_written", Path: ".wolfcastle/system/projects/proj/sub/audit.md", Timestamp: makeTime("2026-03-21T10:00:35Z")},
	}

	var buf bytes.Buffer
	NewInterleavedRenderer(&buf).Render(context.Background(), feedRecords(recs))

	got := buf.String()
	expectContains(t, got, fmt.Sprintf("%s     report: .wolfcastle/system/projects/proj/sub/audit.md", localTS("2026-03-21T10:00:35Z")))
}

func TestInterleavedRenderer_FiltersDaemonStages(t *testing.T) {
	recs := []Record{
		{Type: "stage_start", Stage: "daemon_start", Timestamp: makeTime("2026-03-21T10:00:00Z")},
		{Type: "stage_complete", Stage: "daemon_start", Timestamp: makeTime("2026-03-21T10:00:01Z"), ExitCode: intPtr(0)},
		{Type: "stage_start", Stage: "execute", Node: "proj", Task: "task-0001", Timestamp: makeTime("2026-03-21T10:00:02Z")},
		{Type: "stage_complete", Stage: "execute", Node: "proj", Task: "task-0001", Timestamp: makeTime("2026-03-21T10:01:00Z"), ExitCode: intPtr(0)},
	}

	var buf bytes.Buffer
	NewInterleavedRenderer(&buf).Render(context.Background(), feedRecords(recs))

	got := buf.String()
	lineCount := strings.Count(got, "\n")
	if lineCount != 2 {
		t.Errorf("expected 2 lines (daemon stages filtered), got %d:\n%s", lineCount, got)
	}
	if strings.Contains(got, "daemon") {
		t.Errorf("daemon stages should be filtered, got:\n%s", got)
	}
}

func TestInterleavedRenderer_SkipsIrrelevantTypes(t *testing.T) {
	recs := []Record{
		{Type: "terminal_marker", Marker: "WOLFCASTLE_COMPLETE", Timestamp: makeTime("2026-03-21T10:00:00Z")},
		{Type: "iteration_start", Node: "proj", Timestamp: makeTime("2026-03-21T10:00:01Z")},
		{Type: "stage_start", Stage: "intake", Node: "proj", Timestamp: makeTime("2026-03-21T10:00:02Z")},
		{Type: "stage_complete", Stage: "intake", Node: "proj", Timestamp: makeTime("2026-03-21T10:00:14Z"), ExitCode: intPtr(0)},
	}

	var buf bytes.Buffer
	NewInterleavedRenderer(&buf).Render(context.Background(), feedRecords(recs))

	got := buf.String()
	lineCount := strings.Count(got, "\n")
	if lineCount != 2 {
		t.Errorf("expected 2 lines (irrelevant types skipped), got %d:\n%s", lineCount, got)
	}
}

func TestInterleavedRenderer_CompleteWithoutStart(t *testing.T) {
	recs := []Record{
		{Type: "stage_complete", Stage: "execute", Node: "proj", Task: "task-0001", Timestamp: makeTime("2026-03-21T10:01:00Z"), ExitCode: intPtr(0)},
	}

	var buf bytes.Buffer
	NewInterleavedRenderer(&buf).Render(context.Background(), feedRecords(recs))

	got := buf.String()
	expectContains(t, got, fmt.Sprintf("%s ✓ [execute] proj/task-0001 (0s)", localTS("2026-03-21T10:01:00Z")))
}

func TestInterleavedRenderer_EmptyChannel(t *testing.T) {
	var buf bytes.Buffer
	NewInterleavedRenderer(&buf).Render(context.Background(), feedRecords(nil))

	if buf.String() != "" {
		t.Errorf("expected empty output, got: %q", buf.String())
	}
}

func TestInterleavedRenderer_ContextCancellation(t *testing.T) {
	ch := make(chan Record, 2)
	ch <- Record{Type: "stage_start", Stage: "execute", Node: "proj", Task: "task-0001", Timestamp: makeTime("2026-03-21T10:00:00Z")}

	ctx, cancel := context.WithCancel(context.Background())

	done := make(chan struct{})
	var buf bytes.Buffer
	go func() {
		NewInterleavedRenderer(&buf).Render(ctx, ch)
		close(done)
	}()

	time.Sleep(50 * time.Millisecond)
	cancel()

	select {
	case <-done:
	case <-time.After(2 * time.Second):
		t.Fatal("Render did not return after context cancellation")
	}

	if !bytes.Contains(buf.Bytes(), []byte("▶ [execute] proj/task-0001")) {
		t.Errorf("expected start line before cancel, got: %q", buf.String())
	}
}

func TestInterleavedRenderer_NilExitCodeTreatedAsSuccess(t *testing.T) {
	recs := []Record{
		{Type: "stage_start", Stage: "intake", Node: "proj", Timestamp: makeTime("2026-03-21T10:00:00Z")},
		{Type: "stage_complete", Stage: "intake", Node: "proj", Timestamp: makeTime("2026-03-21T10:00:05Z")},
	}

	var buf bytes.Buffer
	NewInterleavedRenderer(&buf).Render(context.Background(), feedRecords(recs))

	got := buf.String()
	expectContains(t, got, fmt.Sprintf("%s ✓ [intake] proj (5s)", localTS("2026-03-21T10:00:05Z")))
}

func TestInterleavedRenderer_FullSession(t *testing.T) {
	recs := []Record{
		{Type: "stage_start", Stage: "execute", Node: "donut-stand-website", Task: "site-specification/task-0001", Timestamp: makeTime("2026-03-21T18:01:34Z")},
		{Type: "assistant", Text: "I'll start by creating the site specification document...", Timestamp: makeTime("2026-03-21T18:01:35Z")},
		{Type: "assistant", Text: "Reading the project requirements from the inbox item...", Timestamp: makeTime("2026-03-21T18:01:36Z")},
		{Type: "stage_complete", Stage: "execute", Node: "donut-stand-website", Task: "site-specification/task-0001", Timestamp: makeTime("2026-03-21T18:02:56Z"), ExitCode: intPtr(0)},
		{Type: "stage_start", Stage: "execute", Node: "donut-stand-website", Task: "site-specification/task-0002", Timestamp: makeTime("2026-03-21T18:02:57Z")},
		{Type: "assistant", Text: "Now I need to write the HTML structure...", Timestamp: makeTime("2026-03-21T18:02:58Z")},
		{Type: "stage_complete", Stage: "execute", Node: "donut-stand-website", Task: "site-specification/task-0002", Timestamp: makeTime("2026-03-21T18:04:21Z"), ExitCode: intPtr(0)},
	}

	var buf bytes.Buffer
	NewInterleavedRenderer(&buf).Render(context.Background(), feedRecords(recs))

	got := buf.String()

	// 7 lines: 2 start + 2 complete + 3 assistant
	lineCount := strings.Count(got, "\n")
	if lineCount != 7 {
		t.Errorf("expected 7 lines, got %d:\n%s", lineCount, got)
	}

	// Verify chronological order matches the spec example.
	expectContains(t, got, fmt.Sprintf("%s ▶ [execute] donut-stand-website/site-specification/task-0001", localTS("2026-03-21T18:01:34Z")))
	expectContains(t, got, fmt.Sprintf("%s     I'll start by creating the site specification document...", localTS("2026-03-21T18:01:35Z")))
	expectContains(t, got, fmt.Sprintf("%s     Reading the project requirements from the inbox item...", localTS("2026-03-21T18:01:36Z")))
	expectContains(t, got, fmt.Sprintf("%s ✓ [execute] donut-stand-website/site-specification/task-0001 (1m22s)", localTS("2026-03-21T18:02:56Z")))
	expectContains(t, got, fmt.Sprintf("%s ▶ [execute] donut-stand-website/site-specification/task-0002", localTS("2026-03-21T18:02:57Z")))
	expectContains(t, got, fmt.Sprintf("%s     Now I need to write the HTML structure...", localTS("2026-03-21T18:02:58Z")))
}

// expectContains is a test helper that fails with a descriptive message
// if got does not contain the expected substring.
func expectContains(t *testing.T, got, expected string) {
	t.Helper()
	if !strings.Contains(got, expected) {
		t.Errorf("output missing expected substring\nwant: %s\ngot:\n%s", expected, got)
	}
}

func TestInterleavedRenderer_NodeOnlyAddress(t *testing.T) {
	recs := []Record{
		{Type: "stage_start", Stage: "audit", Node: "my-project/sub-module", Timestamp: makeTime("2026-03-21T10:00:00Z")},
		{Type: "stage_complete", Stage: "audit", Node: "my-project/sub-module", Timestamp: makeTime("2026-03-21T10:00:34Z"), ExitCode: intPtr(0)},
	}

	var buf bytes.Buffer
	NewInterleavedRenderer(&buf).Render(context.Background(), feedRecords(recs))

	got := buf.String()
	expectContains(t, got, fmt.Sprintf("%s ▶ [audit] my-project/sub-module", localTS("2026-03-21T10:00:00Z")))
	expectContains(t, got, fmt.Sprintf("%s ✓ [audit] my-project/sub-module (34s)", localTS("2026-03-21T10:00:34Z")))
}
