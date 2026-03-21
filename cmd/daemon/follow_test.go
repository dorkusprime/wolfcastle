package daemon

import (
	"bytes"
	"compress/gzip"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/dorkusprime/wolfcastle/internal/invoke"
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
