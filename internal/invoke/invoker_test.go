package invoke

import (
	"bytes"
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/dorkusprime/wolfcastle/internal/config"
)

// echoModel returns a ModelDef that runs "echo" with the given args.
// Because the invoker pipes prompt to stdin, echo ignores it and prints args.
func echoModel(args ...string) config.ModelDef {
	return config.ModelDef{Command: "echo", Args: args}
}

// catModel returns a ModelDef that runs "cat", which copies stdin to stdout.
func catModel() config.ModelDef {
	return config.ModelDef{Command: "cat"}
}

// shModel runs a one-liner shell command.
func shModel(script string) config.ModelDef {
	return config.ModelDef{Command: "sh", Args: []string{"-c", script}}
}

func TestInvokeSimple_BasicOutput_Echo(t *testing.T) {
	result, err := InvokeSimple(context.Background(), echoModel("hello", "world"), "", ".")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	got := strings.TrimSpace(result.Stdout)
	if got != "hello world" {
		t.Errorf("stdout = %q, want %q", got, "hello world")
	}
	if result.ExitCode != 0 {
		t.Errorf("exit code = %d, want 0", result.ExitCode)
	}
}

func TestInvokeSimple_StdinPiped(t *testing.T) {
	result, err := InvokeSimple(context.Background(), catModel(), "prompt text here", ".")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	got := strings.TrimSpace(result.Stdout)
	if got != "prompt text here" {
		t.Errorf("stdout = %q, want %q", got, "prompt text here")
	}
}

func TestInvokeSimple_NonZeroExitCode(t *testing.T) {
	result, err := InvokeSimple(context.Background(), shModel("exit 42"), "", ".")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.ExitCode != 42 {
		t.Errorf("exit code = %d, want 42", result.ExitCode)
	}
}

func TestInvokeSimple_StderrCaptured(t *testing.T) {
	result, err := InvokeSimple(context.Background(), shModel("echo error-text >&2"), "", ".")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !strings.Contains(result.Stderr, "error-text") {
		t.Errorf("stderr = %q, want to contain %q", result.Stderr, "error-text")
	}
}

func TestInvokeSimple_CommandNotFound(t *testing.T) {
	_, err := InvokeSimple(context.Background(), config.ModelDef{Command: "nonexistent-command-xyz"}, "", ".")
	if err == nil {
		t.Fatal("expected error for missing command")
	}
	if !strings.Contains(err.Error(), "invoking") {
		t.Errorf("error = %q, want to contain 'invoking'", err.Error())
	}
}

func TestInvokeSimple_WorkingDirectory(t *testing.T) {
	dir := t.TempDir()
	result, err := InvokeSimple(context.Background(), shModel("pwd"), "", dir)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	// Resolve symlinks on both sides since macOS has /var -> /private/var.
	resolvedDir, _ := filepath.EvalSymlinks(dir)
	got := strings.TrimSpace(result.Stdout)
	resolvedGot, _ := filepath.EvalSymlinks(got)
	if resolvedGot != resolvedDir {
		t.Errorf("pwd = %q (resolved: %q), want %q", got, resolvedGot, resolvedDir)
	}
}

func TestInvokeSimple_ContextCancelled(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	cancel() // cancel immediately
	_, err := InvokeSimple(ctx, shModel("sleep 10"), "", ".")
	if err == nil {
		t.Fatal("expected error for cancelled context")
	}
}

// --- Streaming tests ---

func TestProcessInvoker_Streaming_LogWriterReceivesOutput(t *testing.T) {
	var logBuf bytes.Buffer
	result, err := NewProcessInvoker().Invoke(context.Background(), echoModel("line1"), "", ".", &logBuf, nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !strings.Contains(result.Stdout, "line1") {
		t.Errorf("result.Stdout = %q, want to contain 'line1'", result.Stdout)
	}
	if !strings.Contains(logBuf.String(), "line1") {
		t.Errorf("logWriter = %q, want to contain 'line1'", logBuf.String())
	}
}

func TestProcessInvoker_Streaming_MultipleLines(t *testing.T) {
	var logBuf bytes.Buffer
	model := shModel("echo line1; echo line2; echo line3")
	result, err := NewProcessInvoker().Invoke(context.Background(), model, "", ".", &logBuf, nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	for _, want := range []string{"line1", "line2", "line3"} {
		if !strings.Contains(result.Stdout, want) {
			t.Errorf("result.Stdout missing %q", want)
		}
		if !strings.Contains(logBuf.String(), want) {
			t.Errorf("logWriter missing %q", want)
		}
	}
}

func TestProcessInvoker_Streaming_NilLogWriter(t *testing.T) {
	// With nil logWriter, should still capture output via non-streaming path.
	result, err := NewProcessInvoker().Invoke(context.Background(), echoModel("hello"), "", ".", nil, nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !strings.Contains(result.Stdout, "hello") {
		t.Errorf("result.Stdout = %q, want to contain 'hello'", result.Stdout)
	}
}

func TestProcessInvoker_Streaming_NonZeroExitCode(t *testing.T) {
	var logBuf bytes.Buffer
	result, err := NewProcessInvoker().Invoke(context.Background(), shModel("echo output; exit 7"), "", ".", &logBuf, nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.ExitCode != 7 {
		t.Errorf("exit code = %d, want 7", result.ExitCode)
	}
	if !strings.Contains(result.Stdout, "output") {
		t.Errorf("stdout should still be captured despite non-zero exit")
	}
}

func TestProcessInvoker_Streaming_StderrCaptured(t *testing.T) {
	var logBuf bytes.Buffer
	result, err := NewProcessInvoker().Invoke(context.Background(), shModel("echo ok; echo err >&2"), "", ".", &logBuf, nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !strings.Contains(result.Stderr, "err") {
		t.Errorf("stderr = %q, want to contain 'err'", result.Stderr)
	}
}

// --- Marker detection tests ---

func TestMarkerDetection_Complete(t *testing.T) {
	result, err := InvokeSimple(context.Background(), echoModel("WOLFCASTLE_COMPLETE"), "", ".")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.TerminalMarker != MarkerComplete {
		t.Errorf("TerminalMarker = %v, want MarkerComplete", result.TerminalMarker)
	}
}

func TestMarkerDetection_Yield(t *testing.T) {
	result, err := InvokeSimple(context.Background(), echoModel("WOLFCASTLE_YIELD"), "", ".")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.TerminalMarker != MarkerYield {
		t.Errorf("TerminalMarker = %v, want MarkerYield", result.TerminalMarker)
	}
}

func TestMarkerDetection_Blocked(t *testing.T) {
	result, err := InvokeSimple(context.Background(), shModel("echo 'WOLFCASTLE_BLOCKED: missing dependency'"), "", ".")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.TerminalMarker != MarkerBlocked {
		t.Errorf("TerminalMarker = %v, want MarkerBlocked", result.TerminalMarker)
	}
}

func TestMarkerDetection_Summary(t *testing.T) {
	result, err := InvokeSimple(context.Background(), shModel("echo 'WOLFCASTLE_SUMMARY: Implemented the feature'"), "", ".")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.Summary != "Implemented the feature" {
		t.Errorf("Summary = %q, want %q", result.Summary, "Implemented the feature")
	}
}

func TestMarkerDetection_SummaryWithComplete(t *testing.T) {
	script := `echo "WOLFCASTLE_SUMMARY: Task done successfully"
echo "WOLFCASTLE_COMPLETE"`
	result, err := InvokeSimple(context.Background(), shModel(script), "", ".")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.TerminalMarker != MarkerComplete {
		t.Errorf("TerminalMarker = %v, want MarkerComplete", result.TerminalMarker)
	}
	if result.Summary != "Task done successfully" {
		t.Errorf("Summary = %q, want %q", result.Summary, "Task done successfully")
	}
}

func TestMarkerDetection_FirstTerminalMarkerWins(t *testing.T) {
	script := `echo "WOLFCASTLE_YIELD"
echo "WOLFCASTLE_COMPLETE"`
	result, err := InvokeSimple(context.Background(), shModel(script), "", ".")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.TerminalMarker != MarkerYield {
		t.Errorf("TerminalMarker = %v, want MarkerYield (first wins)", result.TerminalMarker)
	}
}

func TestMarkerDetection_Skip(t *testing.T) {
	result, err := InvokeSimple(context.Background(), echoModel("WOLFCASTLE_SKIP reason here"), "", ".")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.TerminalMarker != MarkerSkip {
		t.Errorf("TerminalMarker = %v, want MarkerSkip", result.TerminalMarker)
	}
}

func TestMarkerDetection_Continue(t *testing.T) {
	result, err := InvokeSimple(context.Background(), echoModel("WOLFCASTLE_CONTINUE"), "", ".")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.TerminalMarker != MarkerContinue {
		t.Errorf("TerminalMarker = %v, want MarkerContinue", result.TerminalMarker)
	}
}

func TestMarkerDetection_NoMarker(t *testing.T) {
	result, err := InvokeSimple(context.Background(), echoModel("just some output"), "", ".")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.TerminalMarker != MarkerNone {
		t.Errorf("TerminalMarker = %v, want MarkerNone", result.TerminalMarker)
	}
}

func TestMarkerDetection_EmptySummaryIgnored(t *testing.T) {
	result, err := InvokeSimple(context.Background(), shModel("echo 'WOLFCASTLE_SUMMARY:'"), "", ".")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.Summary != "" {
		t.Errorf("Summary = %q, want empty for blank summary", result.Summary)
	}
}

func TestMarkerDetection_Streaming(t *testing.T) {
	var logBuf bytes.Buffer
	script := `echo "some output"
echo "WOLFCASTLE_SUMMARY: streamed summary"
echo "WOLFCASTLE_COMPLETE"`
	result, err := NewProcessInvoker().Invoke(context.Background(), shModel(script), "", ".", &logBuf, nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.TerminalMarker != MarkerComplete {
		t.Errorf("TerminalMarker = %v, want MarkerComplete", result.TerminalMarker)
	}
	if result.Summary != "streamed summary" {
		t.Errorf("Summary = %q, want %q", result.Summary, "streamed summary")
	}
}

// --- Marker String() tests ---

func TestMarkerString(t *testing.T) {
	tests := []struct {
		marker Marker
		want   string
	}{
		{MarkerNone, "none"},
		{MarkerComplete, "WOLFCASTLE_COMPLETE"},
		{MarkerYield, "WOLFCASTLE_YIELD"},
		{MarkerBlocked, "WOLFCASTLE_BLOCKED"},
		{MarkerSkip, "WOLFCASTLE_SKIP"},
		{MarkerContinue, "WOLFCASTLE_CONTINUE"},
	}
	for _, tt := range tests {
		if got := tt.marker.String(); got != tt.want {
			t.Errorf("Marker(%d).String() = %q, want %q", tt.marker, got, tt.want)
		}
	}
}

// --- ProcessInvoker tests ---

func TestProcessInvoker_WithLineCallback(t *testing.T) {
	inv := NewProcessInvoker()
	var lines []string
	cb := func(line string) {
		lines = append(lines, line)
	}
	model := shModel("echo alpha; echo beta")
	result, err := inv.Invoke(context.Background(), model, "", ".", nil, cb)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(lines) != 2 {
		t.Fatalf("got %d callback lines, want 2", len(lines))
	}
	if lines[0] != "alpha" {
		t.Errorf("line[0] = %q, want %q", lines[0], "alpha")
	}
	if lines[1] != "beta" {
		t.Errorf("line[1] = %q, want %q", lines[1], "beta")
	}
	if !strings.Contains(result.Stdout, "alpha") || !strings.Contains(result.Stdout, "beta") {
		t.Errorf("stdout should contain both lines")
	}
}

func TestProcessInvoker_WithCmdFactory(t *testing.T) {
	factoryCalled := false
	inv := &ProcessInvoker{
		CmdFactory: func(ctx context.Context, name string, args ...string) *exec.Cmd {
			factoryCalled = true
			return exec.CommandContext(ctx, "echo", "from-factory")
		},
	}
	result, err := inv.Invoke(context.Background(), config.ModelDef{Command: "ignored"}, "", ".", nil, nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !factoryCalled {
		t.Error("CmdFactory was not called")
	}
	if !strings.Contains(result.Stdout, "from-factory") {
		t.Errorf("stdout = %q, want to contain 'from-factory'", result.Stdout)
	}
}

func TestProcessInvoker_StreamingCommandNotFound(t *testing.T) {
	inv := NewProcessInvoker()
	var logBuf bytes.Buffer
	_, err := inv.Invoke(context.Background(), config.ModelDef{Command: "nonexistent-xyz"}, "", ".", &logBuf, nil)
	if err == nil {
		t.Fatal("expected error for missing command in streaming mode")
	}
}

// --- InvokeSimple tests ---

func TestInvokeSimple_BasicOutput(t *testing.T) {
	result, err := InvokeSimple(context.Background(), echoModel("simple"), "", ".")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !strings.Contains(result.Stdout, "simple") {
		t.Errorf("stdout = %q, want to contain 'simple'", result.Stdout)
	}
}

// --- detectMarkers unit tests ---

func TestDetectMarkers_InMiddleOfLine(t *testing.T) {
	r := &Result{}
	detectMarkers("some text WOLFCASTLE_COMPLETE more text", r)
	if r.TerminalMarker != MarkerComplete {
		t.Errorf("TerminalMarker = %v, want MarkerComplete (embedded in text)", r.TerminalMarker)
	}
}

func TestDetectMarkers_SummaryNotOverwritten(t *testing.T) {
	// When multiple WOLFCASTLE_SUMMARY lines exist, the last one wins
	// (detectLineMarker overwrites each time).
	r := &Result{}
	detectMarkers("WOLFCASTLE_SUMMARY: first\nWOLFCASTLE_SUMMARY: second", r)
	if r.Summary != "second" {
		t.Errorf("Summary = %q, want %q (last wins)", r.Summary, "second")
	}
}

func TestDetectLineMarker_NoOp(t *testing.T) {
	r := &Result{}
	detectLineMarker("normal line of text", r)
	if r.TerminalMarker != MarkerNone {
		t.Errorf("TerminalMarker = %v, want MarkerNone", r.TerminalMarker)
	}
	if r.Summary != "" {
		t.Errorf("Summary = %q, want empty", r.Summary)
	}
}

// --- Large output test ---

func TestProcessInvoker_Streaming_LargeOutput(t *testing.T) {
	// Generate output with many lines to test streaming works correctly.
	// Use printf with a fixed string instead of /dev/zero for speed.
	padding := strings.Repeat("A", 100)
	dir := t.TempDir()
	scriptPath := filepath.Join(dir, "gen.sh")
	script := fmt.Sprintf(`#!/bin/sh
i=0
while [ $i -lt 500 ]; do
  printf "line-%%d: %s\n" "$i"
  i=$((i + 1))
done
echo "WOLFCASTLE_COMPLETE"
`, padding)
	if err := os.WriteFile(scriptPath, []byte(script), 0755); err != nil {
		t.Fatal(err)
	}

	var logBuf bytes.Buffer
	result, err := NewProcessInvoker().Invoke(context.Background(), config.ModelDef{Command: "sh", Args: []string{scriptPath}}, "", ".", &logBuf, nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.TerminalMarker != MarkerComplete {
		t.Errorf("TerminalMarker = %v, want MarkerComplete", result.TerminalMarker)
	}
	lineCount := strings.Count(result.Stdout, "\n")
	if lineCount < 500 {
		t.Errorf("line count = %d, want >= 500", lineCount)
	}
}

// --- Empty output test ---

func TestInvokeSimple_EmptyOutput(t *testing.T) {
	result, err := InvokeSimple(context.Background(), shModel("true"), "", ".")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.Stdout != "" {
		t.Errorf("stdout = %q, want empty", result.Stdout)
	}
	if result.ExitCode != 0 {
		t.Errorf("exit code = %d, want 0", result.ExitCode)
	}
	if result.TerminalMarker != MarkerNone {
		t.Errorf("TerminalMarker = %v, want MarkerNone", result.TerminalMarker)
	}
}

// --- Callback with markers test ---

func TestProcessInvoker_CallbackSeesMarkerLines(t *testing.T) {
	inv := NewProcessInvoker()
	var markerLines []string
	cb := func(line string) {
		if strings.HasPrefix(strings.TrimSpace(line), "WOLFCASTLE_") {
			markerLines = append(markerLines, line)
		}
	}
	script := `echo "normal line"
echo "WOLFCASTLE_SUMMARY: test summary"
echo "WOLFCASTLE_COMPLETE"`
	result, err := inv.Invoke(context.Background(), shModel(script), "", ".", nil, cb)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(markerLines) != 2 {
		t.Errorf("got %d marker lines, want 2", len(markerLines))
	}
	if result.TerminalMarker != MarkerComplete {
		t.Errorf("TerminalMarker = %v, want MarkerComplete", result.TerminalMarker)
	}
	if result.Summary != "test summary" {
		t.Errorf("Summary = %q, want %q", result.Summary, "test summary")
	}
}

// --- Both logWriter and callback test ---

func TestProcessInvoker_BothLogWriterAndCallback(t *testing.T) {
	inv := NewProcessInvoker()
	var logBuf bytes.Buffer
	var cbLines []string
	cb := func(line string) {
		cbLines = append(cbLines, line)
	}
	model := shModel("echo hello; echo world")
	result, err := inv.Invoke(context.Background(), model, "", ".", &logBuf, cb)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	// Both should receive the lines.
	if len(cbLines) != 2 {
		t.Errorf("callback got %d lines, want 2", len(cbLines))
	}
	if !strings.Contains(logBuf.String(), "hello") || !strings.Contains(logBuf.String(), "world") {
		t.Errorf("logWriter = %q, should contain both lines", logBuf.String())
	}
	if !strings.Contains(result.Stdout, "hello") || !strings.Contains(result.Stdout, "world") {
		t.Errorf("result.Stdout = %q, should contain both lines", result.Stdout)
	}
}

// --- Process group kill tests ---

func TestProcessGroupKill_NonStreamingCancelKillsChildren(t *testing.T) {
	// Spawn a non-streaming command that starts a background child, then
	// cancel the context. The child should be killed via process group
	// signal, not left as an orphan.
	//
	// Strategy: write the child's PID to a temp file so we can check
	// whether it's still alive after cancellation.
	pidFile := filepath.Join(t.TempDir(), "child.pid")

	// The script: (1) start a background sleep, (2) write its PID,
	// (3) echo "started" so we get output, (4) sleep to keep the
	// leader alive until the cancel arrives.
	script := fmt.Sprintf(
		`sleep 999 & echo $! > %s; echo started; sleep 999`,
		pidFile,
	)

	ctx, cancel := context.WithCancel(context.Background())

	// Cancel after a short delay — enough for the script to start
	// its background child and echo "started".
	go func() {
		time.Sleep(500 * time.Millisecond)
		cancel()
	}()

	inv := NewProcessInvoker()
	_, _ = inv.Invoke(ctx, shModel(script), "", ".", nil, nil)

	// Read the child PID and verify it's dead.
	pidBytes, err := os.ReadFile(pidFile)
	if err != nil {
		t.Fatalf("failed to read child PID file: %v", err)
	}
	pidStr := strings.TrimSpace(string(pidBytes))
	if pidStr == "" {
		t.Fatal("child PID file is empty")
	}

	// Give the OS a moment to reap the child.
	time.Sleep(200 * time.Millisecond)

	// Sending signal 0 checks if the process exists without killing it.
	checkCmd := exec.Command("kill", "-0", pidStr)
	if err := checkCmd.Run(); err == nil {
		// Clean up the orphan we just detected, then fail.
		_ = exec.Command("kill", "-9", pidStr).Run()
		t.Errorf("child process %s is still alive after context cancellation (orphan)", pidStr)
	}
}
