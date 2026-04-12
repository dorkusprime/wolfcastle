package logging

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// ═══════════════════════════════════════════════════════════════════════════
// compressFile: nonexistent source
// ═══════════════════════════════════════════════════════════════════════════

func TestCompressFile_NonexistentSource(t *testing.T) {
	t.Parallel()
	err := compressFile("/tmp/does-not-exist-wolfcastle-xyz.jsonl")
	if err == nil {
		t.Error("expected error for nonexistent source file")
	}
}

// ═══════════════════════════════════════════════════════════════════════════
// Level: String and ParseLevel coverage
// ═══════════════════════════════════════════════════════════════════════════

func TestLevel_String_Unknown(t *testing.T) {
	t.Parallel()
	l := Level(999)
	if l.String() != "info" {
		t.Errorf("unknown level should default to 'info', got %q", l.String())
	}
}

func TestParseLevel_AllLevels(t *testing.T) {
	t.Parallel()
	cases := []struct {
		input string
		level Level
		ok    bool
	}{
		{"debug", LevelDebug, true},
		{"info", LevelInfo, true},
		{"warn", LevelWarn, true},
		{"error", LevelError, true},
		{"DEBUG", LevelDebug, true},
		{"unknown", 0, false},
	}
	for _, tc := range cases {
		l, ok := ParseLevel(tc.input)
		if ok != tc.ok {
			t.Errorf("ParseLevel(%q): ok=%v, want %v", tc.input, ok, tc.ok)
		}
		if ok && l != tc.level {
			t.Errorf("ParseLevel(%q): level=%d, want %d", tc.input, l, tc.level)
		}
	}
}

// ═══════════════════════════════════════════════════════════════════════════
// Logger: Log with no active iteration
// ═══════════════════════════════════════════════════════════════════════════

func TestLog_NoActiveIteration(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	logger, err := NewLogger(dir)
	if err != nil {
		t.Fatal(err)
	}
	defer logger.Close()

	// Don't call StartIteration. File is nil
	err = logger.Log(map[string]any{"msg": "test"})
	if err == nil {
		t.Error("expected error when logging without active iteration")
	}
}

// ═══════════════════════════════════════════════════════════════════════════
// AssistantWriter: nil and active
// ═══════════════════════════════════════════════════════════════════════════

func TestAssistantWriter_NilWhenNoIteration(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	logger, err := NewLogger(dir)
	if err != nil {
		t.Fatal(err)
	}
	defer logger.Close()

	w := logger.AssistantWriter()
	if w != nil {
		t.Error("expected nil AssistantWriter when no iteration is active")
	}
}

func TestAssistantWriter_WritesNDJSON(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	logger, err := NewLogger(dir)
	if err != nil {
		t.Fatal(err)
	}

	defer logger.Close()

	if err := logger.StartIteration(); err != nil {
		t.Fatal(err)
	}

	w := logger.AssistantWriter()
	if w == nil {
		t.Fatal("expected non-nil AssistantWriter")
	}

	n, err := w.Write([]byte("hello from assistant"))
	if err != nil {
		t.Fatalf("write error: %v", err)
	}
	if n != len("hello from assistant") {
		t.Errorf("expected %d bytes written, got %d", len("hello from assistant"), n)
	}
}

// ═══════════════════════════════════════════════════════════════════════════
// CurrentLogPath
// ═══════════════════════════════════════════════════════════════════════════

func TestCurrentLogPath_NoIteration(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	logger, err := NewLogger(dir)
	if err != nil {
		t.Fatal(err)
	}
	defer logger.Close()

	if p := logger.CurrentLogPath(); p != "" {
		t.Errorf("expected empty path, got %q", p)
	}
}

func TestCurrentLogPath_AfterStartIteration(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	logger, err := NewLogger(dir)
	if err != nil {
		t.Fatal(err)
	}
	defer logger.Close()

	if err := logger.StartIteration(); err != nil {
		t.Fatal(err)
	}

	p := logger.CurrentLogPath()
	if p == "" {
		t.Error("expected non-empty path after starting iteration")
	}
	if !strings.HasSuffix(p, ".jsonl") {
		t.Errorf("expected .jsonl suffix, got %q", p)
	}
}

// ═══════════════════════════════════════════════════════════════════════════
// LatestLogFile
// ═══════════════════════════════════════════════════════════════════════════

func TestLatestLogFile_NoFiles(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	_, err := LatestLogFile(dir)
	if err == nil {
		t.Error("expected error when no log files exist")
	}
}

func TestLatestLogFile_WithGzAndPlain(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	_ = os.WriteFile(filepath.Join(dir, "0001-20260101T00-00Z.jsonl.gz"), []byte("gz"), 0644)
	_ = os.WriteFile(filepath.Join(dir, "0002-20260102T00-00Z.jsonl"), []byte("{}"), 0644)

	latest, err := LatestLogFile(dir)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(latest, "0002") {
		t.Errorf("expected latest to be 0002, got %q", latest)
	}
}

// ═══════════════════════════════════════════════════════════════════════════
// NewLogger: error path
// ═══════════════════════════════════════════════════════════════════════════

func TestNewLogger_DirectoryCreationFails(t *testing.T) {
	t.Parallel()
	// Use a file as the parent to block directory creation
	dir := t.TempDir()
	blocker := filepath.Join(dir, "blocker")
	_ = os.WriteFile(blocker, []byte("x"), 0644)

	_, err := NewLogger(filepath.Join(blocker, "logs"))
	if err == nil {
		t.Error("expected error when log directory cannot be created")
	}
}

// ═══════════════════════════════════════════════════════════════════════════
// Close: idempotent
// ═══════════════════════════════════════════════════════════════════════════

func TestClose_Idempotent(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	logger, err := NewLogger(dir)
	if err != nil {
		t.Fatal(err)
	}

	if err := logger.StartIteration(); err != nil {
		t.Fatal(err)
	}

	logger.Close()
	logger.Close() // Should not panic
}

// ═══════════════════════════════════════════════════════════════════════════
// EnforceRetention: read-only directory (error path)
// ═══════════════════════════════════════════════════════════════════════════

func TestEnforceRetention_UnreadableDirectory(t *testing.T) {
	t.Parallel()
	err := EnforceRetention("/nonexistent/dir/xyz", 10, 30)
	if err == nil {
		t.Error("expected error for nonexistent directory")
	}
}

// ═══════════════════════════════════════════════════════════════════════════
// IsLogFile
// ═══════════════════════════════════════════════════════════════════════════

func TestIsLogFile_ExtendedCases(t *testing.T) {
	t.Parallel()
	cases := []struct {
		name     string
		expected bool
	}{
		{"0001-20260101T00-00Z.jsonl", true},
		{"0001-20260101T00-00Z.jsonl.gz", true},
		{"readme.txt", false},
		{"data.json", false},
		{"something.jsonl.bak", false},
	}
	for _, tc := range cases {
		if got := IsLogFile(tc.name); got != tc.expected {
			t.Errorf("IsLogFile(%q) = %v, want %v", tc.name, got, tc.expected)
		}
	}
}
