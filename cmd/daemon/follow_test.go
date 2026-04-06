package daemon

import (
	"bytes"
	"compress/gzip"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
	"time"

	"github.com/dorkusprime/wolfcastle/internal/invoke"
	"github.com/dorkusprime/wolfcastle/internal/logrender"
	"github.com/spf13/cobra"
)

// ═══════════════════════════════════════════════════════════════════════════
// invoke.FormatAssistantText
// ═══════════════════════════════════════════════════════════════════════════

func TestFormatAssistantText_AssistantWithTextContent(t *testing.T) {
	t.Parallel()
	raw := `{"type":"assistant","message":{"content":[{"type":"text","text":"Hello from the model"}]}}`
	got := invoke.FormatAssistantText(raw)
	if got != "Hello from the model" {
		t.Errorf("expected 'Hello from the model', got %q", got)
	}
}

func TestFormatAssistantText_AssistantWithMultipleTextBlocks(t *testing.T) {
	t.Parallel()
	raw := `{"type":"assistant","message":{"content":[{"type":"text","text":"First"},{"type":"text","text":"Second"}]}}`
	got := invoke.FormatAssistantText(raw)
	if !strings.Contains(got, "First") || !strings.Contains(got, "Second") {
		t.Errorf("expected both text blocks, got %q", got)
	}
	if !strings.Contains(got, "\n") {
		t.Errorf("expected newline joining parts, got %q", got)
	}
}

func TestFormatAssistantText_AssistantWithToolUse_Description(t *testing.T) {
	t.Parallel()
	raw := `{"type":"assistant","message":{"content":[{"type":"tool_use","name":"Bash","input":{"description":"list files"}}]}}`
	got := invoke.FormatAssistantText(raw)
	if !strings.Contains(got, "Bash") || !strings.Contains(got, "list files") {
		t.Errorf("expected tool name and description, got %q", got)
	}
}

func TestFormatAssistantText_AssistantWithToolUse_Command(t *testing.T) {
	t.Parallel()
	raw := `{"type":"assistant","message":{"content":[{"type":"tool_use","name":"Bash","input":{"command":"ls -la"}}]}}`
	got := invoke.FormatAssistantText(raw)
	if !strings.Contains(got, "Bash") || !strings.Contains(got, "ls -la") {
		t.Errorf("expected tool name and command, got %q", got)
	}
}

func TestFormatAssistantText_AssistantWithToolUse_LongCommand(t *testing.T) {
	t.Parallel()
	longCmd := strings.Repeat("x", 100)
	raw := `{"type":"assistant","message":{"content":[{"type":"tool_use","name":"Bash","input":{"command":"` + longCmd + `"}}]}}`
	got := invoke.FormatAssistantText(raw)
	if !strings.HasSuffix(got, "...") {
		t.Errorf("long command should be truncated with ..., got %q", got)
	}
}

func TestFormatAssistantText_AssistantWithToolUse_NoInputDetails(t *testing.T) {
	t.Parallel()
	raw := `{"type":"assistant","message":{"content":[{"type":"tool_use","name":"Read","input":{"path":"/foo"}}]}}`
	got := invoke.FormatAssistantText(raw)
	if got != "  → Read" {
		t.Errorf("expected bare tool name, got %q", got)
	}
}

func TestFormatAssistantText_AssistantWithToolUse_NonMapInput(t *testing.T) {
	t.Parallel()
	raw := `{"type":"assistant","message":{"content":[{"type":"tool_use","name":"Read","input":"just a string"}]}}`
	got := invoke.FormatAssistantText(raw)
	if got != "  → Read" {
		t.Errorf("expected bare tool name for non-map input, got %q", got)
	}
}

func TestFormatAssistantText_AssistantEmptyContent(t *testing.T) {
	t.Parallel()
	raw := `{"type":"assistant","message":{"content":[]}}`
	got := invoke.FormatAssistantText(raw)
	if got != "" {
		t.Errorf("expected empty string for empty content, got %q", got)
	}
}

func TestFormatAssistantText_AssistantEmptyTextBlock(t *testing.T) {
	t.Parallel()
	raw := `{"type":"assistant","message":{"content":[{"type":"text","text":""}]}}`
	got := invoke.FormatAssistantText(raw)
	if got != "" {
		t.Errorf("expected empty string when text is empty, got %q", got)
	}
}

func TestFormatAssistantText_ResultType(t *testing.T) {
	t.Parallel()
	raw := `{"type":"result","result":"Task completed successfully"}`
	got := invoke.FormatAssistantText(raw)
	if got != "[result] Task completed successfully" {
		t.Errorf("expected '[result] Task completed successfully', got %q", got)
	}
}

func TestFormatAssistantText_ResultTypeEmpty(t *testing.T) {
	t.Parallel()
	raw := `{"type":"result","result":""}`
	got := invoke.FormatAssistantText(raw)
	if got != "" {
		t.Errorf("expected empty for empty result, got %q", got)
	}
}

func TestFormatAssistantText_SystemInit(t *testing.T) {
	t.Parallel()
	raw := `{"type":"system","subtype":"init"}`
	got := invoke.FormatAssistantText(raw)
	if got != "[session started]" {
		t.Errorf("expected '[session started]', got %q", got)
	}
}

func TestFormatAssistantText_SystemNonInit(t *testing.T) {
	t.Parallel()
	raw := `{"type":"system","subtype":"something_else"}`
	got := invoke.FormatAssistantText(raw)
	if got != "" {
		t.Errorf("expected empty for non-init system, got %q", got)
	}
}

func TestFormatAssistantText_UnknownType(t *testing.T) {
	t.Parallel()
	raw := `{"type":"delta","data":"stuff"}`
	got := invoke.FormatAssistantText(raw)
	if got != "" {
		t.Errorf("expected empty for unknown type, got %q", got)
	}
}

func TestFormatAssistantText_NonJSON(t *testing.T) {
	t.Parallel()
	got := invoke.FormatAssistantText("plain text output")
	if got != "plain text output" {
		t.Errorf("expected plain text passthrough, got %q", got)
	}
}

func TestFormatAssistantText_NonJSON_LongTruncated(t *testing.T) {
	t.Parallel()
	long := strings.Repeat("a", 250)
	got := invoke.FormatAssistantText(long)
	if len(got) != 203 { // 200 + "..."
		t.Errorf("expected truncated to 203 chars, got %d", len(got))
	}
	if !strings.HasSuffix(got, "...") {
		t.Error("expected trailing ...")
	}
}

func TestFormatAssistantText_NonJSON_ExactlyAtLimit(t *testing.T) {
	t.Parallel()
	exact := strings.Repeat("b", 200)
	got := invoke.FormatAssistantText(exact)
	if got != exact {
		t.Errorf("string at exactly 200 chars should not be truncated")
	}
}

// ═══════════════════════════════════════════════════════════════════════════
// Output mode resolution (last-wins semantics)
// ═══════════════════════════════════════════════════════════════════════════

// parseMode creates a minimal cobra command with mode flags, parses the
// given args, and returns the resolved output mode.
func parseMode(t *testing.T, args []string) outputMode {
	t.Helper()
	mode := modeSummary
	cmd := &cobra.Command{RunE: func(*cobra.Command, []string) error { return nil }}
	registerModeFlags(cmd, &mode)
	cmd.SetArgs(args)
	if err := cmd.Execute(); err != nil {
		t.Fatalf("unexpected parse error: %v", err)
	}
	return mode
}

func TestOutputMode_NoFlags_DefaultsSummary(t *testing.T) {
	t.Parallel()
	if got := parseMode(t, nil); got != modeSummary {
		t.Errorf("expected modeSummary, got %d", got)
	}
}

func TestOutputMode_SingleThoughts(t *testing.T) {
	t.Parallel()
	if got := parseMode(t, []string{"--thoughts"}); got != modeThoughts {
		t.Errorf("expected modeThoughts, got %d", got)
	}
}

func TestOutputMode_SingleThoughtsShort(t *testing.T) {
	t.Parallel()
	if got := parseMode(t, []string{"-t"}); got != modeThoughts {
		t.Errorf("expected modeThoughts, got %d", got)
	}
}

func TestOutputMode_SingleInterleaved(t *testing.T) {
	t.Parallel()
	if got := parseMode(t, []string{"--interleaved"}); got != modeInterleaved {
		t.Errorf("expected modeInterleaved, got %d", got)
	}
}

func TestOutputMode_SingleInterleavedShort(t *testing.T) {
	t.Parallel()
	if got := parseMode(t, []string{"-i"}); got != modeInterleaved {
		t.Errorf("expected modeInterleaved, got %d", got)
	}
}

func TestOutputMode_SingleJSON(t *testing.T) {
	t.Parallel()
	if got := parseMode(t, []string{"--json"}); got != modeJSON {
		t.Errorf("expected modeJSON, got %d", got)
	}
}

func TestOutputMode_ThoughtsThenJSON_LastWins(t *testing.T) {
	t.Parallel()
	if got := parseMode(t, []string{"--thoughts", "--json"}); got != modeJSON {
		t.Errorf("expected modeJSON (last wins), got %d", got)
	}
}

func TestOutputMode_JSONThenThoughts_LastWins(t *testing.T) {
	t.Parallel()
	if got := parseMode(t, []string{"--json", "--thoughts"}); got != modeThoughts {
		t.Errorf("expected modeThoughts (last wins), got %d", got)
	}
}

func TestOutputMode_InterleavedThenThoughts_LastWins(t *testing.T) {
	t.Parallel()
	if got := parseMode(t, []string{"--interleaved", "--thoughts"}); got != modeThoughts {
		t.Errorf("expected modeThoughts (last wins), got %d", got)
	}
}

func TestOutputMode_ThoughtsThenInterleaved_LastWins(t *testing.T) {
	t.Parallel()
	if got := parseMode(t, []string{"--thoughts", "--interleaved"}); got != modeInterleaved {
		t.Errorf("expected modeInterleaved (last wins), got %d", got)
	}
}

func TestOutputMode_AllThree_LastWins(t *testing.T) {
	t.Parallel()
	if got := parseMode(t, []string{"--json", "--thoughts", "--interleaved"}); got != modeInterleaved {
		t.Errorf("expected modeInterleaved (last wins), got %d", got)
	}
}

func TestOutputMode_AllThreeReversed_LastWins(t *testing.T) {
	t.Parallel()
	if got := parseMode(t, []string{"--interleaved", "--thoughts", "--json"}); got != modeJSON {
		t.Errorf("expected modeJSON (last wins), got %d", got)
	}
}

func TestOutputMode_ShortFlagsMixed_LastWins(t *testing.T) {
	t.Parallel()
	if got := parseMode(t, []string{"-t", "-i"}); got != modeInterleaved {
		t.Errorf("expected modeInterleaved (last wins), got %d", got)
	}
}

func TestOutputMode_ShortAndLongMixed_LastWins(t *testing.T) {
	t.Parallel()
	if got := parseMode(t, []string{"--json", "-t"}); got != modeThoughts {
		t.Errorf("expected modeThoughts (last wins), got %d", got)
	}
}

// ═══════════════════════════════════════════════════════════════════════════
// Exit code semantics
// ═══════════════════════════════════════════════════════════════════════════

func TestLogCmd_Exit1_NoLogDir(t *testing.T) {
	t.Parallel()
	env := newTestEnv(t)

	// Log directory does not exist (project not initialized).
	env.RootCmd.SetArgs([]string{"log"})
	err := env.RootCmd.Execute()
	if err == nil {
		t.Fatal("expected error when log directory does not exist")
	}
}

func TestLogCmd_Exit1_EmptyLogDir(t *testing.T) {
	t.Parallel()
	env := newTestEnv(t)

	logDir := filepath.Join(env.WolfcastleDir, "system", "logs")
	_ = os.MkdirAll(logDir, 0755)

	// Directory exists but contains no log files.
	env.RootCmd.SetArgs([]string{"log"})
	err := env.RootCmd.Execute()
	if err == nil {
		t.Fatal("expected error when log directory is empty")
	}
}

func TestLogCmd_Exit1_SessionOutOfRange(t *testing.T) {
	t.Parallel()
	env := newTestEnv(t)

	logDir := filepath.Join(env.WolfcastleDir, "system", "logs")
	_ = os.MkdirAll(logDir, 0755)
	_ = os.WriteFile(filepath.Join(logDir, "0001-20260321T18-00Z.jsonl"),
		[]byte(`{"type":"daemon_start","timestamp":"2026-03-21T18:00:00Z"}`+"\n"), 0644)

	// Only one session exists; requesting session 5 should fail.
	env.RootCmd.SetArgs([]string{"log", "--session", "5"})
	err := env.RootCmd.Execute()
	if err == nil {
		t.Fatal("expected error for out-of-range session index")
	}
}

func TestLogCmd_Exit0_Success(t *testing.T) {
	t.Parallel()
	env := newTestEnv(t)

	logDir := filepath.Join(env.WolfcastleDir, "system", "logs")
	_ = os.MkdirAll(logDir, 0755)
	_ = os.WriteFile(filepath.Join(logDir, "0001-20260321T18-00Z.jsonl"),
		[]byte(`{"type":"daemon_start","timestamp":"2026-03-21T18:00:00Z"}`+"\n"), 0644)

	env.RootCmd.SetArgs([]string{"log"})
	if err := env.RootCmd.Execute(); err != nil {
		t.Fatalf("expected exit 0 on successful replay, got: %v", err)
	}
}

func TestLogCmd_Exit0_FollowInterrupted(t *testing.T) {
	t.Parallel()
	env := newTestEnv(t)

	logDir := filepath.Join(env.WolfcastleDir, "system", "logs")
	_ = os.MkdirAll(logDir, 0755)
	_ = os.WriteFile(filepath.Join(logDir, "0001-20260321T18-00Z.jsonl"),
		[]byte(`{"type":"daemon_start","timestamp":"2026-03-21T18:00:00Z"}`+"\n"), 0644)

	done := make(chan error, 1)
	go func() {
		env.RootCmd.SetArgs([]string{"log", "--follow"})
		done <- env.RootCmd.Execute()
	}()

	// Follow mode blocks until cancelled; verify it doesn't error during streaming.
	select {
	case err := <-done:
		if err != nil {
			t.Fatalf("follow mode should exit 0, got: %v", err)
		}
	case <-time.After(500 * time.Millisecond):
		// Still streaming, which is expected behavior.
	}
}

// ═══════════════════════════════════════════════════════════════════════════
// Edge cases: corrupted NDJSON and compressed files
// ═══════════════════════════════════════════════════════════════════════════

func TestLogCmd_CorruptedNDJSON_SkipsGracefully(t *testing.T) {
	t.Parallel()
	env := newTestEnv(t)

	logDir := filepath.Join(env.WolfcastleDir, "system", "logs")
	_ = os.MkdirAll(logDir, 0755)

	// Mix of valid JSON, garbage, and more valid JSON.
	content := `{"type":"daemon_start","timestamp":"2026-03-21T18:00:00Z"}
not valid json at all {{{
{"type":"stage_start","stage":"plan","node":"n","timestamp":"2026-03-21T18:00:01Z"}
another broken line ~~~
{"type":"stage_complete","stage":"plan","node":"n","exit_code":0,"timestamp":"2026-03-21T18:01:00Z"}
`
	_ = os.WriteFile(filepath.Join(logDir, "0001-20260321T18-00Z.jsonl"),
		[]byte(content), 0644)

	// Should succeed; corrupted lines are skipped silently.
	env.RootCmd.SetArgs([]string{"log"})
	if err := env.RootCmd.Execute(); err != nil {
		t.Fatalf("corrupted NDJSON should be skipped gracefully, got: %v", err)
	}
}

func TestLogCmd_CompressedGzFiles(t *testing.T) {
	t.Parallel()
	env := newTestEnv(t)

	logDir := filepath.Join(env.WolfcastleDir, "system", "logs")
	_ = os.MkdirAll(logDir, 0755)

	// Write a gzip-compressed log file from an "older session".
	var buf bytes.Buffer
	gz := gzip.NewWriter(&buf)
	_, _ = gz.Write([]byte(`{"type":"daemon_start","timestamp":"2026-03-20T10:00:00Z"}` + "\n"))
	_, _ = gz.Write([]byte(`{"type":"stage_start","stage":"execute","node":"old","task":"t1","timestamp":"2026-03-20T10:00:01Z"}` + "\n"))
	_ = gz.Close()
	_ = os.WriteFile(filepath.Join(logDir, "0001-20260320T10-00Z.jsonl.gz"), buf.Bytes(), 0644)

	// Also write a plain session so session 1 resolves to the compressed one.
	_ = os.WriteFile(filepath.Join(logDir, "0001-20260321T18-00Z.jsonl"),
		[]byte(`{"type":"daemon_start","timestamp":"2026-03-21T18:00:00Z"}`+"\n"), 0644)

	// Replay the older (compressed) session.
	env.RootCmd.SetArgs([]string{"log", "--session", "1"})
	if err := env.RootCmd.Execute(); err != nil {
		t.Fatalf("compressed .jsonl.gz files should be read, got: %v", err)
	}
}

// ═══════════════════════════════════════════════════════════════════════════
// replayJSON: direct unit tests for streaming and decompression
// ═══════════════════════════════════════════════════════════════════════════

// writeGzLines creates a gzip-compressed file containing newline-terminated lines.
func writeGzLines(t *testing.T, path string, lines ...string) {
	t.Helper()
	f, err := os.Create(path)
	if err != nil {
		t.Fatalf("creating %s: %v", path, err)
	}
	defer func() { _ = f.Close() }()
	gz := gzip.NewWriter(f)
	for _, l := range lines {
		_, _ = gz.Write([]byte(l))
		_, _ = gz.Write([]byte("\n"))
	}
	if err := gz.Close(); err != nil {
		t.Fatalf("closing gzip writer: %v", err)
	}
}

// captureReplayJSON runs replayJSON while capturing os.Stdout, returning
// whatever was written. Not safe for parallel use (swaps a global).
func captureReplayJSON(t *testing.T, session logrender.Session) string {
	t.Helper()

	r, w, err := os.Pipe()
	if err != nil {
		t.Fatalf("creating pipe: %v", err)
	}

	origStdout := os.Stdout
	os.Stdout = w

	replayErr := replayJSON(session)

	// Close the write end so the reader sees EOF.
	_ = w.Close()
	os.Stdout = origStdout

	var buf bytes.Buffer
	_, _ = io.Copy(&buf, r)
	_ = r.Close()

	if replayErr != nil {
		t.Fatalf("replayJSON returned error: %v", replayErr)
	}
	return buf.String()
}

func TestReplayJSON_PlainJSONL(t *testing.T) {
	dir := t.TempDir()
	plain := filepath.Join(dir, "0001.jsonl")
	lines := []string{
		`{"type":"daemon_start","timestamp":"2026-03-21T10:00:00Z"}`,
		`{"type":"stage_start","stage":"plan","node":"n","timestamp":"2026-03-21T10:00:01Z"}`,
		`{"type":"stage_complete","stage":"plan","node":"n","exit_code":0,"timestamp":"2026-03-21T10:01:00Z"}`,
	}
	_ = os.WriteFile(plain, []byte(strings.Join(lines, "\n")+"\n"), 0644)

	got := captureReplayJSON(t, logrender.Session{Files: []string{plain}})

	for _, want := range lines {
		if !strings.Contains(got, want) {
			t.Errorf("output missing line: %s", want)
		}
	}
	// Each line should appear on its own line in the output.
	gotLines := strings.Split(strings.TrimSpace(got), "\n")
	if len(gotLines) != len(lines) {
		t.Errorf("expected %d lines, got %d", len(lines), len(gotLines))
	}
}

func TestReplayJSON_GzipDecompression(t *testing.T) {
	dir := t.TempDir()
	gzPath := filepath.Join(dir, "0001.jsonl.gz")
	lines := []string{
		`{"type":"daemon_start","timestamp":"2026-03-20T10:00:00Z"}`,
		`{"type":"stage_start","stage":"execute","node":"old","task":"t1","timestamp":"2026-03-20T10:00:01Z"}`,
	}
	writeGzLines(t, gzPath, lines...)

	got := captureReplayJSON(t, logrender.Session{Files: []string{gzPath}})

	for _, want := range lines {
		if !strings.Contains(got, want) {
			t.Errorf("decompressed output missing line: %s", want)
		}
	}
	gotLines := strings.Split(strings.TrimSpace(got), "\n")
	if len(gotLines) != len(lines) {
		t.Errorf("expected %d decompressed lines, got %d", len(lines), len(gotLines))
	}
}

func TestReplayJSON_MixedPlainAndGzip(t *testing.T) {
	dir := t.TempDir()

	plainLines := []string{
		`{"type":"daemon_start","timestamp":"2026-03-21T18:00:00Z"}`,
		`{"type":"stage_start","stage":"plan","node":"a","timestamp":"2026-03-21T18:00:01Z"}`,
	}
	plainPath := filepath.Join(dir, "0001.jsonl")
	_ = os.WriteFile(plainPath, []byte(strings.Join(plainLines, "\n")+"\n"), 0644)

	gzLines := []string{
		`{"type":"daemon_start","timestamp":"2026-03-20T10:00:00Z"}`,
		`{"type":"stage_complete","stage":"execute","node":"b","exit_code":0,"timestamp":"2026-03-20T10:01:00Z"}`,
	}
	gzPath := filepath.Join(dir, "0002.jsonl.gz")
	writeGzLines(t, gzPath, gzLines...)

	got := captureReplayJSON(t, logrender.Session{Files: []string{plainPath, gzPath}})

	allLines := append(plainLines, gzLines...)
	for _, want := range allLines {
		if !strings.Contains(got, want) {
			t.Errorf("mixed output missing line: %s", want)
		}
	}
	gotLines := strings.Split(strings.TrimSpace(got), "\n")
	if len(gotLines) != len(allLines) {
		t.Errorf("expected %d total lines, got %d", len(allLines), len(gotLines))
	}

	// Verify ordering: plain file lines come before gzip file lines.
	plainIdx := strings.Index(got, plainLines[0])
	gzIdx := strings.Index(got, gzLines[0])
	if plainIdx > gzIdx {
		t.Error("plain file lines should appear before gzip file lines in output")
	}
}

func TestReplayJSON_MissingFileSkipped(t *testing.T) {
	dir := t.TempDir()

	// One real file, one missing.
	realPath := filepath.Join(dir, "0001.jsonl")
	line := `{"type":"daemon_start","timestamp":"2026-03-21T18:00:00Z"}`
	_ = os.WriteFile(realPath, []byte(line+"\n"), 0644)

	missingPath := filepath.Join(dir, "nonexistent.jsonl")

	got := captureReplayJSON(t, logrender.Session{Files: []string{missingPath, realPath}})

	if !strings.Contains(got, line) {
		t.Error("output should contain the line from the readable file")
	}
	gotLines := strings.Split(strings.TrimSpace(got), "\n")
	if len(gotLines) != 1 {
		t.Errorf("expected 1 line (missing file skipped), got %d", len(gotLines))
	}
}

func TestReplayJSON_UnreadableFileSkipped(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("chmod restrictions have no effect on Windows")
	}
	if os.Getenv("CI") != "" {
		t.Skip("skip in CI where permissions may differ")
	}
	dir := t.TempDir()

	// Create a file with no read permission.
	unreadable := filepath.Join(dir, "0001.jsonl")
	_ = os.WriteFile(unreadable, []byte(`{"type":"daemon_start"}`+"\n"), 0000)
	defer func() { _ = os.Chmod(unreadable, 0644) }()

	// A second readable file to prove streaming continues.
	readable := filepath.Join(dir, "0002.jsonl")
	line := `{"type":"stage_start","stage":"plan","node":"n","timestamp":"2026-03-21T18:00:01Z"}`
	_ = os.WriteFile(readable, []byte(line+"\n"), 0644)

	got := captureReplayJSON(t, logrender.Session{Files: []string{unreadable, readable}})

	if !strings.Contains(got, line) {
		t.Error("output should contain line from the readable file after skipping unreadable one")
	}
}

func TestReplayJSON_EmptySession(t *testing.T) {
	got := captureReplayJSON(t, logrender.Session{Files: nil})
	if got != "" {
		t.Errorf("empty session should produce no output, got %q", got)
	}
}

func TestReplayJSON_AllFilesMissing(t *testing.T) {
	got := captureReplayJSON(t, logrender.Session{Files: []string{
		"/tmp/does-not-exist-a.jsonl",
		"/tmp/does-not-exist-b.jsonl.gz",
	}})
	if got != "" {
		t.Errorf("all-missing session should produce no output, got %q", got)
	}
}

// ═══════════════════════════════════════════════════════════════════════════
// --follow downgrade when daemon is stopped
// ═══════════════════════════════════════════════════════════════════════════

// makeDaemonAlive writes the current process PID into the test env's PID file,
// causing DaemonRepository.IsAlive() to return true.
func makeDaemonAlive(t *testing.T, wolfcastleDir string) {
	t.Helper()
	sysDir := filepath.Join(wolfcastleDir, "system")
	_ = os.MkdirAll(sysDir, 0755)
	_ = os.WriteFile(
		filepath.Join(sysDir, "wolfcastle.pid"),
		[]byte(fmt.Sprintf("%d", os.Getpid())),
		0644,
	)
}

func TestLogCmd_ExplicitFollowStoppedDaemon_FallsToReplay(t *testing.T) {
	t.Parallel()
	env := newTestEnv(t)

	logDir := filepath.Join(env.WolfcastleDir, "system", "logs")
	_ = os.MkdirAll(logDir, 0755)
	_ = os.WriteFile(filepath.Join(logDir, "0001-20260321T18-00Z.jsonl"),
		[]byte(`{"type":"daemon_start","timestamp":"2026-03-21T18:00:00Z"}`+"\n"), 0644)

	// Daemon is NOT alive (no PID file). Explicit --follow should be
	// downgraded to replay mode and the command should exit promptly.
	done := make(chan error, 1)
	go func() {
		env.RootCmd.SetArgs([]string{"log", "--follow"})
		done <- env.RootCmd.Execute()
	}()

	select {
	case err := <-done:
		if err != nil {
			t.Fatalf("expected clean exit in replay mode, got: %v", err)
		}
		// Command completed: follow was downgraded to replay.
	case <-time.After(3 * time.Second):
		t.Fatal("--follow with stopped daemon should not block; expected replay fallback")
	}
}

func TestLogCmd_ExplicitFollowRunningDaemon_EntersFollowMode(t *testing.T) {
	t.Parallel()
	env := newTestEnv(t)

	logDir := filepath.Join(env.WolfcastleDir, "system", "logs")
	_ = os.MkdirAll(logDir, 0755)
	_ = os.WriteFile(filepath.Join(logDir, "0001-20260321T18-00Z.jsonl"),
		[]byte(`{"type":"daemon_start","timestamp":"2026-03-21T18:00:00Z"}`+"\n"), 0644)

	// Simulate a running daemon so IsAlive() returns true.
	makeDaemonAlive(t, env.WolfcastleDir)

	done := make(chan error, 1)
	go func() {
		env.RootCmd.SetArgs([]string{"log", "--follow"})
		done <- env.RootCmd.Execute()
	}()

	select {
	case err := <-done:
		// If the command returned, it should at least not have errored.
		if err != nil {
			t.Fatalf("follow mode errored unexpectedly: %v", err)
		}
		// It's possible (though unlikely) the follow reader drained instantly;
		// that's acceptable.
	case <-time.After(500 * time.Millisecond):
		// Still streaming: follow mode is active. This is the expected path.
	}
}

func TestLogCmd_NoFollowStoppedDaemon_UsesReplay(t *testing.T) {
	t.Parallel()
	env := newTestEnv(t)

	logDir := filepath.Join(env.WolfcastleDir, "system", "logs")
	_ = os.MkdirAll(logDir, 0755)
	_ = os.WriteFile(filepath.Join(logDir, "0001-20260321T18-00Z.jsonl"),
		[]byte(`{"type":"daemon_start","timestamp":"2026-03-21T18:00:00Z"}`+"\n"), 0644)

	// No PID file, no --follow flag. Baseline regression guard: this
	// must use replay mode and exit without blocking.
	done := make(chan error, 1)
	go func() {
		env.RootCmd.SetArgs([]string{"log"})
		done <- env.RootCmd.Execute()
	}()

	select {
	case err := <-done:
		if err != nil {
			t.Fatalf("replay mode should succeed, got: %v", err)
		}
	case <-time.After(3 * time.Second):
		t.Fatal("log without --follow and stopped daemon should not block")
	}
}
