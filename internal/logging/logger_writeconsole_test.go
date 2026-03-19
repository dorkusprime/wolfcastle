package logging

import (
	"bytes"
	"strings"
	"testing"
)

// ═══════════════════════════════════════════════════════════════════════════
// writeConsole — suppressed types
// ═══════════════════════════════════════════════════════════════════════════

func TestWriteConsole_SuppressedTypes(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	logger, err := NewLogger(dir)
	if err != nil {
		t.Fatal(err)
	}
	defer logger.Close()

	var buf bytes.Buffer
	logger.Console = &buf
	logger.ConsoleLevel = LevelDebug

	if err := logger.StartIteration(); err != nil {
		t.Fatal(err)
	}

	for _, typ := range []string{"terminal_marker", "no_terminal_marker", "deliverable_unchanged"} {
		buf.Reset()
		_ = logger.Log(map[string]any{"type": typ, "message": "should not appear"}, LevelInfo)
		if buf.Len() > 0 {
			t.Errorf("type %q should be suppressed on console, got %q", typ, buf.String())
		}
	}
}

// ═══════════════════════════════════════════════════════════════════════════
// writeConsole — failure_increment gets custom format
// ═══════════════════════════════════════════════════════════════════════════

func TestWriteConsole_FailureIncrement(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	logger, err := NewLogger(dir)
	if err != nil {
		t.Fatal(err)
	}
	defer logger.Close()

	var buf bytes.Buffer
	logger.Console = &buf
	logger.ConsoleLevel = LevelDebug

	if err := logger.StartIteration(); err != nil {
		t.Fatal(err)
	}

	// The count field comes through as float64 from JSON round-trip, but we
	// set it directly as int here. The format string uses %d, so passing a
	// non-int will produce "0" for count.
	_ = logger.Log(map[string]any{"type": "failure_increment", "count": 3}, LevelWarn)
	output := buf.String()
	if !strings.Contains(output, "Failed") {
		t.Errorf("expected 'Failed' in output, got %q", output)
	}
	if !strings.Contains(output, "attempts") {
		t.Errorf("expected 'attempts' in output, got %q", output)
	}
}

func TestWriteConsole_FailureIncrement_ZeroCount(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	logger, err := NewLogger(dir)
	if err != nil {
		t.Fatal(err)
	}
	defer logger.Close()

	var buf bytes.Buffer
	logger.Console = &buf
	logger.ConsoleLevel = LevelDebug

	if err := logger.StartIteration(); err != nil {
		t.Fatal(err)
	}

	// count not set as int — type assertion yields zero value
	_ = logger.Log(map[string]any{"type": "failure_increment"}, LevelWarn)
	output := buf.String()
	if !strings.Contains(output, "Failed (0 attempts)") {
		t.Errorf("expected 'Failed (0 attempts)', got %q", output)
	}
}

// ═══════════════════════════════════════════════════════════════════════════
// writeConsole — consoleMessages lookup (known type:stage pairs)
// ═══════════════════════════════════════════════════════════════════════════

func TestWriteConsole_ConsoleMessages(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	logger, err := NewLogger(dir)
	if err != nil {
		t.Fatal(err)
	}
	defer logger.Close()

	var buf bytes.Buffer
	logger.Console = &buf
	logger.ConsoleLevel = LevelDebug

	if err := logger.StartIteration(); err != nil {
		t.Fatal(err)
	}

	cases := []struct {
		typ      string
		stage    string
		expected string
	}{
		{"stage_start", "execute", "Executing..."},
		{"stage_complete", "execute", "Done."},
		{"stage_start", "intake", "Processing inbox..."},
		{"stage_complete", "intake", "Intake complete."},
	}

	for _, tc := range cases {
		buf.Reset()
		_ = logger.Log(map[string]any{"type": tc.typ, "stage": tc.stage}, LevelInfo)
		output := buf.String()
		if !strings.Contains(output, tc.expected) {
			t.Errorf("type=%q stage=%q: expected %q in output, got %q", tc.typ, tc.stage, tc.expected, output)
		}
	}
}

// ═══════════════════════════════════════════════════════════════════════════
// writeConsole — stage present but not in consoleMessages map
// falls through to default [LEVEL] stage: msg format
// ═══════════════════════════════════════════════════════════════════════════

func TestWriteConsole_UnknownTypeWithStage(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	logger, err := NewLogger(dir)
	if err != nil {
		t.Fatal(err)
	}
	defer logger.Close()

	var buf bytes.Buffer
	logger.Console = &buf
	logger.ConsoleLevel = LevelDebug

	if err := logger.StartIteration(); err != nil {
		t.Fatal(err)
	}

	_ = logger.Log(map[string]any{
		"type":    "custom_event",
		"stage":   "audit",
		"message": "something happened",
	}, LevelWarn)

	output := buf.String()
	if !strings.Contains(output, "[WARN]") {
		t.Errorf("expected [WARN] prefix, got %q", output)
	}
	if !strings.Contains(output, "audit") {
		t.Errorf("expected stage name, got %q", output)
	}
	if !strings.Contains(output, "something happened") {
		t.Errorf("expected message, got %q", output)
	}
}

// ═══════════════════════════════════════════════════════════════════════════
// writeConsole — no message field, falls back to type
// ═══════════════════════════════════════════════════════════════════════════

func TestWriteConsole_NoMessageFallsBackToType(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	logger, err := NewLogger(dir)
	if err != nil {
		t.Fatal(err)
	}
	defer logger.Close()

	var buf bytes.Buffer
	logger.Console = &buf
	logger.ConsoleLevel = LevelDebug

	if err := logger.StartIteration(); err != nil {
		t.Fatal(err)
	}

	_ = logger.Log(map[string]any{"type": "heartbeat"}, LevelInfo)
	output := buf.String()
	if !strings.Contains(output, "heartbeat") {
		t.Errorf("expected type as fallback message, got %q", output)
	}
}

// ═══════════════════════════════════════════════════════════════════════════
// writeConsole — no stage, no message, no type: bare [LEVEL] output
// ═══════════════════════════════════════════════════════════════════════════

// ═══════════════════════════════════════════════════════════════════════════
// writeConsole — consoleMessages with empty string value triggers suppression
// ═══════════════════════════════════════════════════════════════════════════

func TestWriteConsole_ConsoleMessageEmptyStringSuppresses(t *testing.T) {
	// The empty-string branch in consoleMessages suppresses output.
	// Inject a temporary entry to exercise it.
	consoleMessages["test_suppress:phantom"] = ""
	t.Cleanup(func() { delete(consoleMessages, "test_suppress:phantom") })

	dir := t.TempDir()
	logger, err := NewLogger(dir)
	if err != nil {
		t.Fatal(err)
	}
	defer logger.Close()

	var buf bytes.Buffer
	logger.Console = &buf
	logger.ConsoleLevel = LevelDebug

	if err := logger.StartIteration(); err != nil {
		t.Fatal(err)
	}

	_ = logger.Log(map[string]any{"type": "test_suppress", "stage": "phantom"}, LevelInfo)
	if buf.Len() > 0 {
		t.Errorf("empty consoleMessages value should suppress output, got %q", buf.String())
	}
}

func TestWriteConsole_BareRecord(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	logger, err := NewLogger(dir)
	if err != nil {
		t.Fatal(err)
	}
	defer logger.Close()

	var buf bytes.Buffer
	logger.Console = &buf
	logger.ConsoleLevel = LevelDebug

	if err := logger.StartIteration(); err != nil {
		t.Fatal(err)
	}

	_ = logger.Log(map[string]any{"data": "something"}, LevelError)
	output := buf.String()
	if !strings.Contains(output, "[ERROR]") {
		t.Errorf("expected [ERROR] prefix, got %q", output)
	}
}
