package logrender

import (
	"bytes"
	"testing"
	"time"
)

func intPtr(n int) *int { return &n }

func makeTime(s string) time.Time {
	t, err := time.Parse(time.RFC3339, s)
	if err != nil {
		panic(err)
	}
	return t
}

func feedRecords(recs []Record) <-chan Record {
	ch := make(chan Record, len(recs))
	for _, r := range recs {
		ch <- r
	}
	close(ch)
	return ch
}

func TestSummaryRenderer_SingleStage(t *testing.T) {
	recs := []Record{
		{Type: "stage_start", Stage: "execute", Node: "my-project", Task: "task-0001", Timestamp: makeTime("2026-03-21T10:00:00Z")},
		{Type: "stage_complete", Stage: "execute", Node: "my-project", Task: "task-0001", Timestamp: makeTime("2026-03-21T10:01:22Z"), ExitCode: intPtr(0)},
	}

	var buf bytes.Buffer
	NewSummaryRenderer(&buf).Replay(feedRecords(recs))

	expected := "✓ [execute] my-project/task-0001 (1m22s)\n"
	if buf.String() != expected {
		t.Errorf("got:\n%s\nwant:\n%s", buf.String(), expected)
	}
}

func TestSummaryRenderer_FailedStage(t *testing.T) {
	recs := []Record{
		{Type: "stage_start", Stage: "execute", Node: "proj", Task: "task-0001", Timestamp: makeTime("2026-03-21T10:00:00Z")},
		{Type: "stage_complete", Stage: "execute", Node: "proj", Task: "task-0001", Timestamp: makeTime("2026-03-21T10:03:41Z"), ExitCode: intPtr(1)},
	}

	var buf bytes.Buffer
	NewSummaryRenderer(&buf).Replay(feedRecords(recs))

	expected := "✗ [execute] proj/task-0001 (3m41s)\n"
	if buf.String() != expected {
		t.Errorf("got:\n%s\nwant:\n%s", buf.String(), expected)
	}
}

func TestSummaryRenderer_MultipleStagesAligned(t *testing.T) {
	recs := []Record{
		{Type: "stage_start", Stage: "intake", Node: "donut-stand", Timestamp: makeTime("2026-03-21T10:00:00Z")},
		{Type: "stage_complete", Stage: "intake", Node: "donut-stand", Timestamp: makeTime("2026-03-21T10:00:12Z"), ExitCode: intPtr(0)},
		{Type: "stage_start", Stage: "execute", Node: "donut-stand", Task: "task-0001", Timestamp: makeTime("2026-03-21T10:00:13Z")},
		{Type: "stage_complete", Stage: "execute", Node: "donut-stand", Task: "task-0001", Timestamp: makeTime("2026-03-21T10:01:35Z"), ExitCode: intPtr(0)},
	}

	var buf bytes.Buffer
	NewSummaryRenderer(&buf).Replay(feedRecords(recs))

	// [intake] is 8 chars, [execute] is 9 chars, so intake gets 1 space pad.
	// "donut-stand" is 11 chars, "donut-stand/task-0001" is 21 chars, so
	// first address gets 10 spaces pad.
	lines := buf.String()
	if lines == "" {
		t.Fatal("no output")
	}

	// Verify column alignment: both duration parens should start at the same column.
	parts := bytes.Split(buf.Bytes(), []byte("\n"))
	if len(parts) < 3 { // 2 lines + trailing newline
		t.Fatalf("expected 2 output lines, got %d", len(parts)-1)
	}

	col0 := bytes.Index(parts[0], []byte("("))
	col1 := bytes.Index(parts[1], []byte("("))
	if col0 != col1 {
		t.Errorf("duration columns not aligned: line 0 at %d, line 1 at %d\n%s", col0, col1, lines)
	}
}

func TestSummaryRenderer_PlanningRecords(t *testing.T) {
	recs := []Record{
		{Type: "planning_start", Node: "proj", Timestamp: makeTime("2026-03-21T10:00:00Z")},
		{Type: "planning_complete", Node: "proj", Timestamp: makeTime("2026-03-21T10:00:45Z"), ExitCode: intPtr(0)},
	}

	var buf bytes.Buffer
	NewSummaryRenderer(&buf).Replay(feedRecords(recs))

	expected := "✓ [plan] proj (45s)\n"
	if buf.String() != expected {
		t.Errorf("got:\n%s\nwant:\n%s", buf.String(), expected)
	}
}

func TestSummaryRenderer_AuditReportWritten(t *testing.T) {
	recs := []Record{
		{Type: "stage_start", Stage: "audit", Node: "proj/sub", Timestamp: makeTime("2026-03-21T10:00:00Z")},
		{Type: "stage_complete", Stage: "audit", Node: "proj/sub", Timestamp: makeTime("2026-03-21T10:00:34Z"), ExitCode: intPtr(0)},
		{Type: "audit_report_written", Path: ".wolfcastle/system/projects/proj/sub/audit-2026-03-21T10-00.md"},
	}

	var buf bytes.Buffer
	NewSummaryRenderer(&buf).Replay(feedRecords(recs))

	expected := "✓ [audit] proj/sub (34s)\n  report: .wolfcastle/system/projects/proj/sub/audit-2026-03-21T10-00.md\n"
	if buf.String() != expected {
		t.Errorf("got:\n%s\nwant:\n%s", buf.String(), expected)
	}
}

func TestSummaryRenderer_SkipsIrrelevantTypes(t *testing.T) {
	recs := []Record{
		{Type: "terminal_marker", Marker: "WOLFCASTLE_COMPLETE"},
		{Type: "assistant", Text: "I'll start by reading the file..."},
		{Type: "iteration_start", Node: "proj"},
		{Type: "stage_start", Stage: "execute", Node: "proj", Task: "task-0001", Timestamp: makeTime("2026-03-21T10:00:00Z")},
		{Type: "stage_complete", Stage: "execute", Node: "proj", Task: "task-0001", Timestamp: makeTime("2026-03-21T10:00:58Z"), ExitCode: intPtr(0)},
	}

	var buf bytes.Buffer
	NewSummaryRenderer(&buf).Replay(feedRecords(recs))

	expected := "✓ [execute] proj/task-0001 (58s)\n"
	if buf.String() != expected {
		t.Errorf("got:\n%s\nwant:\n%s", buf.String(), expected)
	}
}

func TestSummaryRenderer_FiltersDaemonStages(t *testing.T) {
	recs := []Record{
		{Type: "stage_start", Stage: "daemon_start", Node: "", Timestamp: makeTime("2026-03-21T10:00:00Z")},
		{Type: "stage_complete", Stage: "daemon_start", Node: "", Timestamp: makeTime("2026-03-21T10:00:01Z"), ExitCode: intPtr(0)},
		{Type: "stage_start", Stage: "daemon_stop", Node: "", Timestamp: makeTime("2026-03-21T10:05:00Z")},
		{Type: "stage_complete", Stage: "daemon_stop", Node: "", Timestamp: makeTime("2026-03-21T10:05:01Z"), ExitCode: intPtr(0)},
		{Type: "stage_start", Stage: "execute", Node: "proj", Task: "task-0001", Timestamp: makeTime("2026-03-21T10:00:02Z")},
		{Type: "stage_complete", Stage: "execute", Node: "proj", Task: "task-0001", Timestamp: makeTime("2026-03-21T10:01:00Z"), ExitCode: intPtr(0)},
	}

	var buf bytes.Buffer
	NewSummaryRenderer(&buf).Replay(feedRecords(recs))

	expected := "✓ [execute] proj/task-0001 (58s)\n"
	if buf.String() != expected {
		t.Errorf("got:\n%s\nwant:\n%s", buf.String(), expected)
	}
}

func TestSummaryRenderer_NilExitCodeTreatedAsSuccess(t *testing.T) {
	recs := []Record{
		{Type: "stage_start", Stage: "intake", Node: "proj", Timestamp: makeTime("2026-03-21T10:00:00Z")},
		{Type: "stage_complete", Stage: "intake", Node: "proj", Timestamp: makeTime("2026-03-21T10:00:05Z")},
	}

	var buf bytes.Buffer
	NewSummaryRenderer(&buf).Replay(feedRecords(recs))

	expected := "✓ [intake] proj (5s)\n"
	if buf.String() != expected {
		t.Errorf("got:\n%s\nwant:\n%s", buf.String(), expected)
	}
}

func TestSummaryRenderer_EmptyChannel(t *testing.T) {
	var buf bytes.Buffer
	NewSummaryRenderer(&buf).Replay(feedRecords(nil))

	if buf.String() != "" {
		t.Errorf("expected empty output for no records, got: %q", buf.String())
	}
}

func TestSummaryRenderer_CompleteWithoutStart(t *testing.T) {
	// A stage_complete with no matching start should still render with 0s duration.
	recs := []Record{
		{Type: "stage_complete", Stage: "execute", Node: "proj", Task: "task-0001", Timestamp: makeTime("2026-03-21T10:01:00Z"), ExitCode: intPtr(0)},
	}

	var buf bytes.Buffer
	NewSummaryRenderer(&buf).Replay(feedRecords(recs))

	expected := "✓ [execute] proj/task-0001 (0s)\n"
	if buf.String() != expected {
		t.Errorf("got:\n%s\nwant:\n%s", buf.String(), expected)
	}
}

func TestSummaryRenderer_NodeOnlyAddress(t *testing.T) {
	recs := []Record{
		{Type: "stage_start", Stage: "audit", Node: "my-project/sub-module", Timestamp: makeTime("2026-03-21T10:00:00Z")},
		{Type: "stage_complete", Stage: "audit", Node: "my-project/sub-module", Timestamp: makeTime("2026-03-21T10:00:34Z"), ExitCode: intPtr(0)},
	}

	var buf bytes.Buffer
	NewSummaryRenderer(&buf).Replay(feedRecords(recs))

	expected := "✓ [audit] my-project/sub-module (34s)\n"
	if buf.String() != expected {
		t.Errorf("got:\n%s\nwant:\n%s", buf.String(), expected)
	}
}

func TestSummaryRenderer_FullSession(t *testing.T) {
	// Simulates the spec example with multiple stage types.
	recs := []Record{
		{Type: "stage_start", Stage: "intake", Node: "donut-stand-website", Timestamp: makeTime("2026-03-21T18:01:00Z")},
		{Type: "stage_complete", Stage: "intake", Node: "donut-stand-website", Timestamp: makeTime("2026-03-21T18:01:12Z"), ExitCode: intPtr(0)},
		{Type: "planning_start", Node: "donut-stand-website", Timestamp: makeTime("2026-03-21T18:01:13Z")},
		{Type: "planning_complete", Node: "donut-stand-website", Timestamp: makeTime("2026-03-21T18:01:58Z"), ExitCode: intPtr(0)},
		{Type: "stage_start", Stage: "execute", Node: "donut-stand-website", Task: "site-specification/task-0001", Timestamp: makeTime("2026-03-21T18:02:00Z")},
		{Type: "stage_complete", Stage: "execute", Node: "donut-stand-website", Task: "site-specification/task-0001", Timestamp: makeTime("2026-03-21T18:03:22Z"), ExitCode: intPtr(0)},
		{Type: "stage_start", Stage: "execute", Node: "donut-stand-website", Task: "site-specification/task-0002", Timestamp: makeTime("2026-03-21T18:03:23Z")},
		{Type: "stage_complete", Stage: "execute", Node: "donut-stand-website", Task: "site-specification/task-0002", Timestamp: makeTime("2026-03-21T18:04:21Z"), ExitCode: intPtr(0)},
		{Type: "stage_start", Stage: "audit", Node: "donut-stand-website", Task: "site-specification", Timestamp: makeTime("2026-03-21T18:04:22Z")},
		{Type: "stage_complete", Stage: "audit", Node: "donut-stand-website", Task: "site-specification", Timestamp: makeTime("2026-03-21T18:04:56Z"), ExitCode: intPtr(0)},
		{Type: "audit_report_written", Path: ".wolfcastle/system/projects/donut-stand-website/audit-2026-03-21T18-08.md"},
		{Type: "stage_start", Stage: "execute", Node: "donut-stand-website", Task: "project-foundation/task-0001", Timestamp: makeTime("2026-03-21T18:04:57Z")},
		{Type: "stage_complete", Stage: "execute", Node: "donut-stand-website", Task: "project-foundation/task-0001", Timestamp: makeTime("2026-03-21T18:07:02Z"), ExitCode: intPtr(0)},
	}

	var buf bytes.Buffer
	NewSummaryRenderer(&buf).Replay(feedRecords(recs))

	// Verify line count (6 stage lines + 1 report line).
	got := buf.String()
	lineCount := 0
	for _, c := range got {
		if c == '\n' {
			lineCount++
		}
	}
	if lineCount != 7 {
		t.Errorf("expected 7 lines, got %d:\n%s", lineCount, got)
	}

	// Spot-check content.
	if !bytes.Contains(buf.Bytes(), []byte("[intake]")) {
		t.Error("missing [intake] line")
	}
	if !bytes.Contains(buf.Bytes(), []byte("[plan]")) {
		t.Error("missing [plan] line")
	}
	if !bytes.Contains(buf.Bytes(), []byte("report: .wolfcastle/system/projects/donut-stand-website/audit-2026-03-21T18-08.md")) {
		t.Error("missing audit report line")
	}
	if !bytes.Contains(buf.Bytes(), []byte("project-foundation/task-0001")) {
		t.Error("missing final execute line")
	}
}
