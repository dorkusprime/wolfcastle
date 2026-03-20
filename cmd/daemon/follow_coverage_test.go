package daemon

import (
	"context"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/dorkusprime/wolfcastle/internal/logging"
)

// ═══════════════════════════════════════════════════════════════════════════
// newLogCmd — flag and RunE path coverage
// ═══════════════════════════════════════════════════════════════════════════

func TestLogCmd_InvalidLevel(t *testing.T) {
	t.Parallel()
	env := newTestEnv(t)

	logDir := filepath.Join(env.WolfcastleDir, "system", "logs")
	_ = os.MkdirAll(logDir, 0755)
	_ = os.WriteFile(filepath.Join(logDir, "0001-test.jsonl"),
		[]byte(`{"level":"info","type":"daemon_start","scope":"test"}`+"\n"), 0644)

	env.RootCmd.SetArgs([]string{"log", "--level", "bogus"})
	err := env.RootCmd.Execute()
	if err == nil {
		t.Fatal("expected error for invalid --level value")
	}
}

func TestLogCmd_ValidLevelDebug(t *testing.T) {
	t.Parallel()
	env := newTestEnv(t)

	logDir := filepath.Join(env.WolfcastleDir, "system", "logs")
	_ = os.MkdirAll(logDir, 0755)
	_ = os.WriteFile(filepath.Join(logDir, "0001-test.jsonl"),
		[]byte(`{"level":"debug","type":"daemon_start","scope":"test"}`+"\n"), 0644)

	// Non-follow mode with --level debug should show the log and exit.
	env.RootCmd.SetArgs([]string{"log", "--level", "debug"})
	if err := env.RootCmd.Execute(); err != nil {
		t.Fatalf("log --level debug failed: %v", err)
	}
}

func TestLogCmd_ValidLevelWarn(t *testing.T) {
	t.Parallel()
	env := newTestEnv(t)

	logDir := filepath.Join(env.WolfcastleDir, "system", "logs")
	_ = os.MkdirAll(logDir, 0755)
	_ = os.WriteFile(filepath.Join(logDir, "0001-test.jsonl"),
		[]byte(`{"level":"warn","type":"stage_error","stage":"x","error":"boom"}`+"\n"), 0644)

	env.RootCmd.SetArgs([]string{"log", "--level", "warn"})
	if err := env.RootCmd.Execute(); err != nil {
		t.Fatalf("log --level warn failed: %v", err)
	}
}

func TestLogCmd_FollowWithLevel(t *testing.T) {
	t.Parallel()
	env := newTestEnv(t)

	logDir := filepath.Join(env.WolfcastleDir, "system", "logs")
	_ = os.MkdirAll(logDir, 0755)
	_ = os.WriteFile(filepath.Join(logDir, "0001-test.jsonl"),
		[]byte(`{"level":"error","type":"stage_error","stage":"x","error":"oops"}`+"\n"), 0644)

	// --follow with --level triggers the followLogs path with a non-default level.
	done := make(chan error, 1)
	go func() {
		env.RootCmd.SetArgs([]string{"log", "--follow", "--level", "error", "--lines", "5"})
		done <- env.RootCmd.Execute()
	}()

	select {
	case err := <-done:
		if err != nil {
			t.Fatalf("log --follow --level error failed: %v", err)
		}
	case <-time.After(1 * time.Second):
		// Expected: still streaming.
	}
}

func TestLogCmd_NoLogDir(t *testing.T) {
	t.Parallel()
	env := newTestEnv(t)

	// Log dir exists but is empty — showRecentLogs prints "No logs yet."
	logDir := filepath.Join(env.WolfcastleDir, "system", "logs")
	_ = os.MkdirAll(logDir, 0755)

	env.RootCmd.SetArgs([]string{"log"})
	if err := env.RootCmd.Execute(); err != nil {
		t.Fatalf("log with empty dir failed: %v", err)
	}
}

// ═══════════════════════════════════════════════════════════════════════════
// followLogs — file rotation and wait-for-logs paths
// ═══════════════════════════════════════════════════════════════════════════

func TestFollowLogs_FileRotation(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()

	// Start with one log file.
	first := filepath.Join(dir, "0001-20260316T00-00Z.jsonl")
	_ = os.WriteFile(first, []byte(
		`{"level":"info","type":"daemon_start","scope":"test"}`+"\n",
	), 0644)

	// Reset offsets so the historical lines path fires.
	clearOffset(first)

	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan error, 1)
	go func() {
		done <- followLogs(ctx, dir, 5, logging.LevelDebug)
	}()

	// Give followLogs time to latch onto the first file,
	// then introduce a second (newer) file to trigger rotation.
	time.Sleep(700 * time.Millisecond)

	second := filepath.Join(dir, "0002-20260316T01-00Z.jsonl")
	_ = os.WriteFile(second, []byte(
		`{"level":"info","type":"daemon_stop","reason":"test"}`+"\n",
	), 0644)

	// Let followLogs detect the new file.
	time.Sleep(700 * time.Millisecond)
	cancel()

	select {
	case err := <-done:
		if err != nil {
			t.Fatalf("followLogs file rotation: %v", err)
		}
	case <-time.After(3 * time.Second):
		t.Fatal("followLogs did not exit after cancel")
	}
}

func TestFollowLogs_WaitForLogsMessage(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	// Empty dir — no log files. followLogs should print the wait message,
	// then exit when context cancels.

	ctx, cancel := context.WithTimeout(context.Background(), 2500*time.Millisecond)
	defer cancel()

	err := followLogs(ctx, dir, 10, logging.LevelInfo)
	if err != nil {
		t.Fatalf("followLogs wait-for-logs: %v", err)
	}
}

func TestFollowLogs_ContextCancelBeforeLatest(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()

	// Cancel immediately — the top-of-loop select should catch it.
	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	err := followLogs(ctx, dir, 10, logging.LevelInfo)
	if err != nil {
		t.Fatalf("followLogs immediate cancel: %v", err)
	}
}

func TestFollowLogs_ZeroLines(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()

	// lines=0 means no historical lines shown on initial attach.
	logLine := `{"level":"info","type":"daemon_start","scope":"test"}` + "\n"
	_ = os.WriteFile(filepath.Join(dir, "0001-20260316T00-00Z.jsonl"), []byte(logLine), 0644)

	ctx, cancel := context.WithTimeout(context.Background(), 400*time.Millisecond)
	defer cancel()

	err := followLogs(ctx, dir, 0, logging.LevelInfo)
	if err != nil {
		t.Fatalf("followLogs zero lines: %v", err)
	}
}

// ═══════════════════════════════════════════════════════════════════════════
// showRecentLogs — direct calls
// ═══════════════════════════════════════════════════════════════════════════

func TestShowRecentLogs_NoFiles(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()

	err := showRecentLogs(dir, 10, logging.LevelInfo)
	if err != nil {
		t.Fatalf("showRecentLogs with no files should succeed: %v", err)
	}
}

func TestShowRecentLogs_WithFile(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()

	logLine := `{"level":"info","type":"daemon_start","scope":"full"}` + "\n"
	_ = os.WriteFile(filepath.Join(dir, "0001-20260316T00-00Z.jsonl"), []byte(logLine), 0644)

	err := showRecentLogs(dir, 10, logging.LevelDebug)
	if err != nil {
		t.Fatalf("showRecentLogs failed: %v", err)
	}
}

// ═══════════════════════════════════════════════════════════════════════════
// tailFileStreaming — remaining edge cases
// ═══════════════════════════════════════════════════════════════════════════

func TestTailFileStreaming_OffsetBeyondFileSize(t *testing.T) {
	t.Parallel()
	tmp := t.TempDir()
	logFile := filepath.Join(tmp, "small.jsonl")
	_ = os.WriteFile(logFile, []byte(`{"level":"info","type":"daemon_start","scope":"x"}`+"\n"), 0644)

	// Set offset well past the file size — should return nil with no output.
	setOffset(logFile, 99999)
	defer clearOffset(logFile)

	err := tailFileStreaming(logFile, logging.LevelDebug)
	if err != nil {
		t.Fatalf("tailFileStreaming with offset past EOF: %v", err)
	}
}

func TestTailFileStreaming_ZeroOffset(t *testing.T) {
	t.Parallel()
	tmp := t.TempDir()
	logFile := filepath.Join(tmp, "zero.jsonl")
	content := `{"level":"debug","type":"daemon_start","scope":"x"}` + "\n" +
		`{"level":"info","type":"daemon_stop","reason":"done"}` + "\n"
	_ = os.WriteFile(logFile, []byte(content), 0644)

	clearOffset(logFile)

	err := tailFileStreaming(logFile, logging.LevelDebug)
	if err != nil {
		t.Fatalf("tailFileStreaming from zero offset: %v", err)
	}

	off := getOffset(logFile)
	if off == 0 {
		t.Error("offset should have advanced")
	}
}

func TestTailFileStreaming_LevelFiltering(t *testing.T) {
	t.Parallel()
	tmp := t.TempDir()
	logFile := filepath.Join(tmp, "levels.jsonl")
	content := `{"level":"debug","type":"daemon_start","scope":"x"}` + "\n" +
		`{"level":"error","type":"stage_error","stage":"x","error":"fail"}` + "\n"
	_ = os.WriteFile(logFile, []byte(content), 0644)

	clearOffset(logFile)

	// With minLevel=error, the debug line is silently dropped.
	err := tailFileStreaming(logFile, logging.LevelError)
	if err != nil {
		t.Fatalf("tailFileStreaming level filter: %v", err)
	}
}

// ═══════════════════════════════════════════════════════════════════════════
// showHistoricalLines — level filtering
// ═══════════════════════════════════════════════════════════════════════════

func TestShowHistoricalLines_LevelFilter(t *testing.T) {
	t.Parallel()
	tmp := t.TempDir()
	logFile := filepath.Join(tmp, "filtered.jsonl")
	content := `{"level":"debug","type":"daemon_start","scope":"x"}` + "\n" +
		`{"level":"error","type":"stage_error","stage":"x","error":"critical"}` + "\n"
	_ = os.WriteFile(logFile, []byte(content), 0644)

	clearOffset(logFile)

	// Only error-level lines should format (debug is silently dropped).
	showHistoricalLines(logFile, 10, logging.LevelError)

	off := getOffset(logFile)
	if off == 0 {
		t.Error("offset should be set after historical lines")
	}
}
