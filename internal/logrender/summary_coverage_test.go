package logrender

import (
	"bytes"
	"context"
	"strings"
	"testing"
	"time"
)

// ---------------------------------------------------------------------------
// Replay: stage_start + stage_complete (exercises writeAligned with columns)
// ---------------------------------------------------------------------------

func TestReplayStageStartComplete(t *testing.T) {
	t0 := makeTime("2026-01-01T00:00:00Z")
	t1 := t0.Add(82 * time.Second)

	got := replayLines(
		Record{Type: "stage_start", Stage: "execute", Node: "auth", Task: "task-0001", Timestamp: t0},
		Record{Type: "stage_complete", Stage: "execute", Node: "auth", Task: "task-0001", Timestamp: t1, ExitCode: intPtr(0)},
	)

	if !strings.Contains(got, "✓") {
		t.Errorf("expected success glyph ✓ in output:\n%s", got)
	}
	if !strings.Contains(got, "[execute]") {
		t.Errorf("expected [execute] label in output:\n%s", got)
	}
	if !strings.Contains(got, "auth/task-0001") {
		t.Errorf("expected auth/task-0001 address in output:\n%s", got)
	}
	if !strings.Contains(got, "(1m22s)") {
		t.Errorf("expected (1m22s) duration in output:\n%s", got)
	}
}

func TestReplayStageCompleteFailure(t *testing.T) {
	t0 := makeTime("2026-01-01T00:00:00Z")
	t1 := t0.Add(5 * time.Second)

	got := replayLines(
		Record{Type: "stage_start", Stage: "execute", Node: "db", Timestamp: t0},
		Record{Type: "stage_complete", Stage: "execute", Node: "db", Timestamp: t1, ExitCode: intPtr(1)},
	)

	if !strings.Contains(got, "✗") {
		t.Errorf("expected failure glyph ✗ in output:\n%s", got)
	}
}

func TestReplayStageCompleteWithDurationMS(t *testing.T) {
	ms := int64(45000)
	got := replayLines(
		Record{Type: "stage_start", Stage: "audit", Node: "core"},
		Record{Type: "stage_complete", Stage: "audit", Node: "core", ExitCode: intPtr(0), DurationMS: &ms},
	)

	if !strings.Contains(got, "(45s)") {
		t.Errorf("expected (45s) from DurationMS in output:\n%s", got)
	}
}

// ---------------------------------------------------------------------------
// Replay: planning_start + planning_complete
// ---------------------------------------------------------------------------

func TestReplayPlanningStartComplete(t *testing.T) {
	t0 := makeTime("2026-01-01T00:00:00Z")
	t1 := t0.Add(30 * time.Second)

	got := replayLines(
		Record{Type: "planning_start", Node: "auth", Task: "task-0001", Timestamp: t0},
		Record{Type: "planning_complete", Node: "auth", Task: "task-0001", Timestamp: t1, ExitCode: intPtr(0)},
	)

	if !strings.Contains(got, "[plan]") {
		t.Errorf("expected [plan] label in output:\n%s", got)
	}
	if !strings.Contains(got, "(30s)") {
		t.Errorf("expected (30s) duration in output:\n%s", got)
	}
}

// ---------------------------------------------------------------------------
// Replay: audit_report_written
// ---------------------------------------------------------------------------

func TestReplayAuditReportWritten(t *testing.T) {
	got := replayLines(Record{
		Type: "audit_report_written",
		Path: "/tmp/report.md",
	})

	if !strings.Contains(got, "report: /tmp/report.md") {
		t.Errorf("expected report path in output:\n%s", got)
	}
}

// ---------------------------------------------------------------------------
// Replay: skipped stages (daemon_start, daemon_stop)
// ---------------------------------------------------------------------------

func TestReplaySkippedStages(t *testing.T) {
	got := replayLines(
		Record{Type: "stage_start", Stage: "daemon_start", Node: "root"},
		Record{Type: "stage_complete", Stage: "daemon_start", Node: "root", ExitCode: intPtr(0)},
		Record{Type: "stage_start", Stage: "daemon_stop", Node: "root"},
		Record{Type: "stage_complete", Stage: "daemon_stop", Node: "root", ExitCode: intPtr(0)},
	)

	if got != "" {
		t.Errorf("expected empty output for skipped stages, got:\n%s", got)
	}
}

// ---------------------------------------------------------------------------
// Replay: mixed lines exercise writeAligned column padding
// ---------------------------------------------------------------------------

func TestReplayAlignedColumns(t *testing.T) {
	t0 := makeTime("2026-01-01T00:00:00Z")

	got := replayLines(
		Record{Type: "stage_start", Stage: "execute", Node: "auth", Task: "task-0001", Timestamp: t0},
		Record{Type: "stage_complete", Stage: "execute", Node: "auth", Task: "task-0001", Timestamp: t0.Add(10 * time.Second), ExitCode: intPtr(0)},
		Record{Type: "stage_start", Stage: "audit", Node: "db", Timestamp: t0},
		Record{Type: "stage_complete", Stage: "audit", Node: "db", Timestamp: t0.Add(120 * time.Second), ExitCode: intPtr(0)},
		Record{Type: "audit_report_written", Path: "/tmp/r.md"},
	)

	lines := strings.Split(strings.TrimRight(got, "\n"), "\n")
	if len(lines) < 3 {
		t.Fatalf("expected at least 3 output lines, got %d:\n%s", len(lines), got)
	}
	// The two stage lines should be padded to equal column widths.
	// Just verify both stage lines are present and the report line appears.
	if !strings.Contains(got, "[execute]") || !strings.Contains(got, "[audit]") {
		t.Errorf("expected both [execute] and [audit] labels:\n%s", got)
	}
	if !strings.Contains(got, "report: /tmp/r.md") {
		t.Errorf("expected report line:\n%s", got)
	}
}

// ---------------------------------------------------------------------------
// Replay: stage_complete without a matching start (no start time)
// ---------------------------------------------------------------------------

func TestReplayCompleteWithoutStart(t *testing.T) {
	got := replayLines(
		Record{Type: "stage_complete", Stage: "execute", Node: "orphan", ExitCode: intPtr(0)},
	)
	// Should still render with (0s) since no start time and no DurationMS.
	if !strings.Contains(got, "(0s)") {
		t.Errorf("expected (0s) for orphan complete, got:\n%s", got)
	}
}

// ---------------------------------------------------------------------------
// Follow: drains records and respects context cancellation
// ---------------------------------------------------------------------------

func TestFollowDrainsChannel(t *testing.T) {
	var buf bytes.Buffer
	sr := NewSummaryRenderer(&buf)

	ch := make(chan Record, 3)
	ch <- Record{Type: "self_heal", Text: "healed"}
	ch <- Record{Type: "idle_reason", Text: "waiting"}
	ch <- Record{Type: "archive_event", Text: "archived node"}
	close(ch)

	sr.Follow(context.Background(), ch)

	got := buf.String()
	if !strings.Contains(got, "healed") {
		t.Errorf("expected 'healed' in Follow output:\n%s", got)
	}
	if !strings.Contains(got, "waiting") {
		t.Errorf("expected 'waiting' in Follow output:\n%s", got)
	}
	if !strings.Contains(got, "archived node") {
		t.Errorf("expected 'archived node' in Follow output:\n%s", got)
	}
}

func TestFollowRespectsContextCancellation(t *testing.T) {
	var buf bytes.Buffer
	sr := NewSummaryRenderer(&buf)

	ctx, cancel := context.WithCancel(context.Background())
	cancel() // cancel immediately

	ch := make(chan Record) // unbuffered, never written to

	done := make(chan struct{})
	go func() {
		sr.Follow(ctx, ch)
		close(done)
	}()

	select {
	case <-done:
		// Follow returned as expected.
	case <-time.After(2 * time.Second):
		t.Fatal("Follow did not return after context cancellation")
	}
}

// ---------------------------------------------------------------------------
// Follow: stage_start and stage_complete in follow mode
// ---------------------------------------------------------------------------

func TestFollowStageStartComplete(t *testing.T) {
	var buf bytes.Buffer
	sr := NewSummaryRenderer(&buf)

	t0 := makeTime("2026-01-01T00:00:00Z")
	t1 := t0.Add(15 * time.Second)

	ch := make(chan Record, 2)
	ch <- Record{Type: "stage_start", Stage: "execute", Node: "auth", Task: "task-0001", Timestamp: t0}
	ch <- Record{Type: "stage_complete", Stage: "execute", Node: "auth", Task: "task-0001", Timestamp: t1, ExitCode: intPtr(0)}
	close(ch)

	sr.Follow(context.Background(), ch)
	got := buf.String()

	if !strings.Contains(got, "▶ [execute] auth/task-0001") {
		t.Errorf("expected start line in Follow output:\n%s", got)
	}
	if !strings.Contains(got, "✓ [execute] auth/task-0001 (15s)") {
		t.Errorf("expected complete line in Follow output:\n%s", got)
	}
}

func TestFollowPlanningStartComplete(t *testing.T) {
	var buf bytes.Buffer
	sr := NewSummaryRenderer(&buf)

	t0 := makeTime("2026-01-01T00:00:00Z")
	t1 := t0.Add(60 * time.Second)

	ch := make(chan Record, 2)
	ch <- Record{Type: "planning_start", Node: "auth", Task: "task-0001", Timestamp: t0}
	ch <- Record{Type: "planning_complete", Node: "auth", Task: "task-0001", Timestamp: t1, ExitCode: intPtr(0)}
	close(ch)

	sr.Follow(context.Background(), ch)
	got := buf.String()

	if !strings.Contains(got, "▶ [plan] auth/task-0001") {
		t.Errorf("expected plan start line:\n%s", got)
	}
	if !strings.Contains(got, "✓ [plan] auth/task-0001 (1m)") {
		t.Errorf("expected plan complete line:\n%s", got)
	}
}

func TestFollowAuditReportWritten(t *testing.T) {
	got := followLine(Record{Type: "audit_report_written", Path: "/tmp/audit.md"})
	want := "  report: /tmp/audit.md\n"
	if got != want {
		t.Errorf("Follow audit_report_written:\ngot:  %q\nwant: %q", got, want)
	}
}

func TestFollowDaemonLifecycleStandingDown(t *testing.T) {
	got := followLine(Record{Type: "daemon_lifecycle", Event: "standing_down", Reason: "done"})
	want := "=== Wolfcastle standing down (done) ===\n"
	if got != want {
		t.Errorf("Follow standing_down:\ngot:  %q\nwant: %q", got, want)
	}
}

func TestFollowDaemonLifecycleDefault(t *testing.T) {
	got := followLine(Record{Type: "daemon_lifecycle", Event: "drain", Text: "draining"})
	want := "=== draining ===\n"
	if got != want {
		t.Errorf("Follow default lifecycle:\ngot:  %q\nwant: %q", got, want)
	}
}

func TestFollowIterationHeaderPlan(t *testing.T) {
	got := followLine(Record{Type: "iteration_header", Kind: "plan", Iteration: 3, Text: "auth/task-0001"})
	want := "--- Planning 3: auth/task-0001 ---\n"
	if got != want {
		t.Errorf("Follow plan iteration:\ngot:  %q\nwant: %q", got, want)
	}
}

func TestFollowInboxEvent(t *testing.T) {
	got := followLine(Record{Type: "inbox_event", Text: "new inbox item"})
	want := "  new inbox item\n"
	if got != want {
		t.Errorf("Follow inbox_event:\ngot:  %q\nwant: %q", got, want)
	}
}

func TestFollowTaskEvent(t *testing.T) {
	got := followLine(Record{Type: "task_event", Text: "task completed"})
	want := "  task completed\n"
	if got != want {
		t.Errorf("Follow task_event:\ngot:  %q\nwant: %q", got, want)
	}
}

func TestFollowArchiveEvent(t *testing.T) {
	got := followLine(Record{Type: "archive_event", Text: "archived stuff"})
	want := "archived stuff\n"
	if got != want {
		t.Errorf("Follow archive_event:\ngot:  %q\nwant: %q", got, want)
	}
}

func TestFollowSpecEvent(t *testing.T) {
	got := followLine(Record{Type: "spec_event", Text: "loaded spec"})
	want := "  loaded spec\n"
	if got != want {
		t.Errorf("Follow spec_event:\ngot:  %q\nwant: %q", got, want)
	}
}

func TestFollowSkippedStages(t *testing.T) {
	got := followLine(Record{Type: "stage_start", Stage: "daemon_start", Node: "root"})
	if got != "" {
		t.Errorf("expected empty output for skipped stage_start, got: %q", got)
	}

	got = followLine(Record{Type: "stage_complete", Stage: "daemon_stop", Node: "root", ExitCode: intPtr(0)})
	if got != "" {
		t.Errorf("expected empty output for skipped stage_complete, got: %q", got)
	}
}

// ---------------------------------------------------------------------------
// writeAligned: empty input
// ---------------------------------------------------------------------------

func TestWriteAlignedEmpty(t *testing.T) {
	got := replayLines() // no records
	if got != "" {
		t.Errorf("expected empty output for no records, got: %q", got)
	}
}

// ---------------------------------------------------------------------------
// nodeAddress: node-only vs node+task
// ---------------------------------------------------------------------------

func TestReplayNodeOnlyAddress(t *testing.T) {
	t0 := makeTime("2026-01-01T00:00:00Z")
	got := replayLines(
		Record{Type: "stage_start", Stage: "execute", Node: "auth", Timestamp: t0},
		Record{Type: "stage_complete", Stage: "execute", Node: "auth", Timestamp: t0.Add(5 * time.Second), ExitCode: intPtr(0)},
	)
	// Should show "auth" without a trailing slash.
	if !strings.Contains(got, "auth") {
		t.Errorf("expected 'auth' in output:\n%s", got)
	}
	if strings.Contains(got, "auth/") {
		t.Errorf("did not expect 'auth/' for node-only address:\n%s", got)
	}
}

// ---------------------------------------------------------------------------
// Unrecognized record types are silently skipped
// ---------------------------------------------------------------------------

func TestReplayUnknownTypeSkipped(t *testing.T) {
	got := replayLines(Record{Type: "something_new", Text: "future record"})
	if got != "" {
		t.Errorf("expected empty output for unknown type, got: %q", got)
	}
}

func TestFollowUnknownTypeSkipped(t *testing.T) {
	got := followLine(Record{Type: "something_new", Text: "future record"})
	if got != "" {
		t.Errorf("expected empty output for unknown type in follow, got: %q", got)
	}
}
