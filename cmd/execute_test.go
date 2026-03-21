package cmd

import (
	"bytes"
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/dorkusprime/wolfcastle/internal/logrender"
)

// fixtureRecords returns a representative set of NDJSON records covering
// every type the InterleavedRenderer handles: stage start/complete (with
// success and failure exit codes), planning start/complete, assistant text,
// audit report paths, daemon stages (which should be filtered), and
// irrelevant types (which should be silently dropped).
func fixtureRecords() []map[string]any {
	return []map[string]any{
		{"type": "stage_start", "stage": "daemon_start", "timestamp": "2026-03-21T18:00:00Z"},
		{"type": "stage_complete", "stage": "daemon_start", "timestamp": "2026-03-21T18:00:01Z", "exit_code": 0},
		{"type": "stage_start", "stage": "execute", "node": "proj", "task": "task-0001", "timestamp": "2026-03-21T18:01:00Z"},
		{"type": "assistant", "text": "Reading the project requirements...", "timestamp": "2026-03-21T18:01:05Z"},
		{"type": "assistant", "text": "", "timestamp": "2026-03-21T18:01:06Z"},
		{"type": "assistant", "text": "Implementing the solution now.", "timestamp": "2026-03-21T18:01:10Z"},
		{"type": "stage_complete", "stage": "execute", "node": "proj", "task": "task-0001", "timestamp": "2026-03-21T18:02:22Z", "exit_code": 0},
		{"type": "terminal_marker", "marker": "WOLFCASTLE_COMPLETE", "timestamp": "2026-03-21T18:02:23Z"},
		{"type": "planning_start", "node": "proj", "timestamp": "2026-03-21T18:03:00Z"},
		{"type": "planning_complete", "node": "proj", "timestamp": "2026-03-21T18:03:45Z", "exit_code": 0},
		{"type": "stage_start", "stage": "audit", "node": "proj/sub", "timestamp": "2026-03-21T18:04:00Z"},
		{"type": "stage_complete", "stage": "audit", "node": "proj/sub", "timestamp": "2026-03-21T18:04:34Z", "exit_code": 0},
		{"type": "audit_report_written", "path": ".wolfcastle/system/projects/proj/sub/audit.md", "timestamp": "2026-03-21T18:04:35Z"},
		{"type": "stage_start", "stage": "execute", "node": "proj", "task": "task-0002", "timestamp": "2026-03-21T18:05:00Z"},
		{"type": "assistant", "text": "This task is blocked by a missing dependency.", "timestamp": "2026-03-21T18:05:10Z"},
		{"type": "stage_complete", "stage": "execute", "node": "proj", "task": "task-0002", "timestamp": "2026-03-21T18:05:30Z", "exit_code": 1},
		{"type": "stage_start", "stage": "daemon_stop", "timestamp": "2026-03-21T18:06:00Z"},
		{"type": "stage_complete", "stage": "daemon_stop", "timestamp": "2026-03-21T18:06:01Z", "exit_code": 0},
	}
}

// writeNDJSON marshals records as newline-delimited JSON into path.
func writeNDJSON(t *testing.T, path string, records []map[string]any) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0755); err != nil {
		t.Fatalf("creating log dir: %v", err)
	}
	var buf bytes.Buffer
	for _, rec := range records {
		line, err := json.Marshal(rec)
		if err != nil {
			t.Fatalf("marshalling record: %v", err)
		}
		buf.Write(line)
		buf.WriteByte('\n')
	}
	if err := os.WriteFile(path, buf.Bytes(), 0644); err != nil {
		t.Fatalf("writing NDJSON file: %v", err)
	}
}

// renderViaReplay reads NDJSON files through the ReplayReader and renders
// them with InterleavedRenderer, returning the rendered output. This is
// the code path taken by "wolfcastle log --interleaved" in replay mode.
func renderViaReplay(files []string) string {
	reader := logrender.NewReplayReader(files)
	records := reader.Records()

	var buf bytes.Buffer
	ir := logrender.NewInterleavedRenderer(&buf)
	ir.Render(context.Background(), records)
	return buf.String()
}

// renderViaFollow reads NDJSON files through the FollowReader and renders
// them with InterleavedRenderer, returning the rendered output. This is
// the code path taken by "wolfcastle execute" and "wolfcastle intake".
func renderViaFollow(t *testing.T, logDir string) string {
	t.Helper()

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	// Use a short poll interval so the test doesn't wait long.
	reader := logrender.NewFollowReader(logDir, 10*time.Millisecond)
	records := reader.Records(ctx)

	var buf bytes.Buffer
	ir := logrender.NewInterleavedRenderer(&buf)

	// The FollowReader runs indefinitely until context cancellation.
	// We drain records on a goroutine and cancel after a generous pause
	// to let the reader discover and consume all file content.
	done := make(chan struct{})
	go func() {
		defer close(done)
		ir.Render(ctx, records)
	}()

	// Give the reader enough poll cycles to consume everything.
	time.Sleep(150 * time.Millisecond)
	cancel()
	<-done

	return buf.String()
}

// TestRenderParity_SingleFile verifies that FollowReader and ReplayReader
// produce identical InterleavedRenderer output for a single NDJSON file.
func TestRenderParity_SingleFile(t *testing.T) {
	logDir := t.TempDir()
	logFile := filepath.Join(logDir, "0001-20260321T18-00Z.jsonl")
	writeNDJSON(t, logFile, fixtureRecords())

	replay := renderViaReplay([]string{logFile})
	follow := renderViaFollow(t, logDir)

	if replay != follow {
		t.Errorf("output mismatch between replay and follow paths\n--- replay ---\n%s\n--- follow ---\n%s", replay, follow)
	}

	// Sanity check: output is non-empty and contains expected content.
	if replay == "" {
		t.Fatal("both paths produced empty output")
	}
	if !strings.Contains(replay, "▶ [execute] proj/task-0001") {
		t.Error("output missing expected stage start line")
	}
	if !strings.Contains(replay, "✓ [execute] proj/task-0001") {
		t.Error("output missing expected success completion line")
	}
	if !strings.Contains(replay, "✗ [execute] proj/task-0002") {
		t.Error("output missing expected failure completion line")
	}
	if !strings.Contains(replay, "Reading the project requirements...") {
		t.Error("output missing expected assistant text")
	}
	if !strings.Contains(replay, "report: .wolfcastle/system/projects/proj/sub/audit.md") {
		t.Error("output missing expected audit report line")
	}
	if strings.Contains(replay, "daemon") {
		t.Error("daemon_start/daemon_stop stages should be filtered")
	}
	if strings.Contains(replay, "WOLFCASTLE_COMPLETE") {
		t.Error("terminal_marker records should be silently dropped")
	}
}

// TestRenderParity_MultipleFiles verifies parity when records span
// multiple iteration files, as happens in real daemon runs.
func TestRenderParity_MultipleFiles(t *testing.T) {
	logDir := t.TempDir()

	first := []map[string]any{
		{"type": "stage_start", "stage": "execute", "node": "proj", "task": "task-0001", "timestamp": "2026-03-21T18:01:00Z"},
		{"type": "assistant", "text": "Working on the first task.", "timestamp": "2026-03-21T18:01:05Z"},
		{"type": "stage_complete", "stage": "execute", "node": "proj", "task": "task-0001", "timestamp": "2026-03-21T18:02:00Z", "exit_code": 0},
	}
	second := []map[string]any{
		{"type": "stage_start", "stage": "execute", "node": "proj", "task": "task-0002", "timestamp": "2026-03-21T18:03:00Z"},
		{"type": "assistant", "text": "Now handling the second task.", "timestamp": "2026-03-21T18:03:10Z"},
		{"type": "stage_complete", "stage": "execute", "node": "proj", "task": "task-0002", "timestamp": "2026-03-21T18:04:00Z", "exit_code": 0},
	}

	file1 := filepath.Join(logDir, "0001-20260321T18-01Z.jsonl")
	file2 := filepath.Join(logDir, "0002-20260321T18-03Z.jsonl")
	writeNDJSON(t, file1, first)
	writeNDJSON(t, file2, second)

	replay := renderViaReplay([]string{file1, file2})
	follow := renderViaFollow(t, logDir)

	if replay != follow {
		t.Errorf("multi-file output mismatch\n--- replay ---\n%s\n--- follow ---\n%s", replay, follow)
	}

	// Both iterations should appear in chronological order.
	lines := strings.Split(strings.TrimRight(replay, "\n"), "\n")
	if len(lines) != 6 {
		t.Errorf("expected 6 lines (2 starts + 2 completes + 2 assistant), got %d:\n%s", len(lines), replay)
	}
}

// TestRenderParity_EmptyFile verifies both paths handle an empty log file
// gracefully, producing no output.
func TestRenderParity_EmptyFile(t *testing.T) {
	logDir := t.TempDir()
	logFile := filepath.Join(logDir, "0001-20260321T18-00Z.jsonl")
	if err := os.WriteFile(logFile, []byte{}, 0644); err != nil {
		t.Fatal(err)
	}

	replay := renderViaReplay([]string{logFile})
	follow := renderViaFollow(t, logDir)

	if replay != follow {
		t.Errorf("empty-file output mismatch\n--- replay ---\n%q\n--- follow ---\n%q", replay, follow)
	}
	if replay != "" {
		t.Errorf("expected empty output for empty file, got: %q", replay)
	}
}

// TestRenderParity_MalformedLines verifies both paths skip malformed JSON
// lines identically, rendering only the valid records.
func TestRenderParity_MalformedLines(t *testing.T) {
	logDir := t.TempDir()
	logFile := filepath.Join(logDir, "0001-20260321T18-00Z.jsonl")

	// Mix valid records with garbage lines.
	content := strings.Join([]string{
		`{"type":"stage_start","stage":"execute","node":"proj","task":"task-0001","timestamp":"2026-03-21T18:01:00Z"}`,
		`this is not json`,
		`{"broken json`,
		`{"type":"stage_complete","stage":"execute","node":"proj","task":"task-0001","timestamp":"2026-03-21T18:02:00Z","exit_code":0}`,
		"",
	}, "\n")

	if err := os.WriteFile(logFile, []byte(content), 0644); err != nil {
		t.Fatal(err)
	}

	replay := renderViaReplay([]string{logFile})
	follow := renderViaFollow(t, logDir)

	if replay != follow {
		t.Errorf("malformed-lines output mismatch\n--- replay ---\n%s\n--- follow ---\n%s", replay, follow)
	}

	lines := strings.Split(strings.TrimRight(replay, "\n"), "\n")
	if len(lines) != 2 {
		t.Errorf("expected 2 lines (malformed skipped), got %d:\n%s", len(lines), replay)
	}
}

// TestRenderParity_PlanningRecords verifies planning_start/planning_complete
// records render identically through both paths.
func TestRenderParity_PlanningRecords(t *testing.T) {
	logDir := t.TempDir()
	logFile := filepath.Join(logDir, "0001-20260321T18-00Z.jsonl")

	records := []map[string]any{
		{"type": "planning_start", "node": "proj", "timestamp": "2026-03-21T10:00:00Z"},
		{"type": "assistant", "text": "Evaluating the project tree...", "timestamp": "2026-03-21T10:00:20Z"},
		{"type": "planning_complete", "node": "proj", "timestamp": "2026-03-21T10:00:45Z", "exit_code": 0},
	}
	writeNDJSON(t, logFile, records)

	replay := renderViaReplay([]string{logFile})
	follow := renderViaFollow(t, logDir)

	if replay != follow {
		t.Errorf("planning output mismatch\n--- replay ---\n%s\n--- follow ---\n%s", replay, follow)
	}

	if !strings.Contains(replay, "▶ [plan] proj") {
		t.Error("output missing planning start")
	}
	if !strings.Contains(replay, "✓ [plan] proj (45s)") {
		t.Error("output missing planning complete with duration")
	}
}
