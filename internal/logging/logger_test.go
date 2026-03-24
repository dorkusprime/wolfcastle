package logging

import (
	"bytes"
	"compress/gzip"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
	"time"
)

// withFrozenClock replaces nowFunc for the duration of the test and
// restores it on cleanup.
func withFrozenClock(t *testing.T, frozen time.Time) {
	t.Helper()
	original := nowFunc
	nowFunc = func() time.Time { return frozen }
	t.Cleanup(func() { nowFunc = original })
}

// ── Level Tests ─────────────────────────────────────────────────────

func TestParseLevel_ValidStrings(t *testing.T) {
	t.Parallel()
	cases := []struct {
		input string
		want  Level
	}{
		{"debug", LevelDebug},
		{"info", LevelInfo},
		{"warn", LevelWarn},
		{"error", LevelError},
		{"DEBUG", LevelDebug},
		{"Info", LevelInfo},
	}
	for _, tc := range cases {
		got, ok := ParseLevel(tc.input)
		if !ok {
			t.Errorf("ParseLevel(%q) returned ok=false", tc.input)
		}
		if got != tc.want {
			t.Errorf("ParseLevel(%q) = %v, want %v", tc.input, got, tc.want)
		}
	}
}

func TestParseLevel_InvalidString(t *testing.T) {
	t.Parallel()
	_, ok := ParseLevel("verbose")
	if ok {
		t.Error("expected ParseLevel(\"verbose\") to return ok=false")
	}
}

func TestLevel_String(t *testing.T) {
	t.Parallel()
	cases := []struct {
		level Level
		want  string
	}{
		{LevelDebug, "debug"},
		{LevelInfo, "info"},
		{LevelWarn, "warn"},
		{LevelError, "error"},
		{Level(99), "info"}, // unknown falls back to info
	}
	for _, tc := range cases {
		if got := tc.level.String(); got != tc.want {
			t.Errorf("Level(%d).String() = %q, want %q", tc.level, got, tc.want)
		}
	}
}

// ── NewLogger Tests ────────────────────────────────────────────────

func TestNewLogger_CreatesLogDirectory(t *testing.T) {
	t.Parallel()
	dir := filepath.Join(t.TempDir(), "logs", "nested")

	logger, err := NewLogger(dir)
	if err != nil {
		t.Fatal(err)
	}
	defer logger.Close()

	info, err := os.Stat(dir)
	if err != nil {
		t.Fatal("log directory was not created")
	}
	if !info.IsDir() {
		t.Error("expected a directory")
	}
}

// ── StartIteration Tests ──────────────────────────────────────────

func TestStartIteration_CreatesNumberedLogFile(t *testing.T) {
	frozen := time.Date(2026, 3, 14, 12, 30, 0, 0, time.UTC)
	withFrozenClock(t, frozen)
	dir := t.TempDir()

	logger, err := NewLogger(dir)
	if err != nil {
		t.Fatal(err)
	}
	defer logger.Close()

	if err := logger.StartIteration(); err != nil {
		t.Fatal(err)
	}

	entries, _ := os.ReadDir(dir)
	if len(entries) != 1 {
		t.Fatalf("expected 1 log file, got %d", len(entries))
	}

	name := entries[0].Name()
	if name != "0001-20260314T12-30Z.jsonl" {
		t.Errorf("unexpected filename %q", name)
	}
}

func TestStartIteration_IncrementsCounter(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()

	logger, err := NewLogger(dir)
	if err != nil {
		t.Fatal(err)
	}
	defer logger.Close()

	_ = logger.StartIteration()
	if logger.Iteration != 1 {
		t.Errorf("expected iteration=1, got %d", logger.Iteration)
	}

	_ = logger.StartIteration()
	if logger.Iteration != 2 {
		t.Errorf("expected iteration=2, got %d", logger.Iteration)
	}
}

func TestStartIteration_ClosesPreviousFile(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()

	logger, err := NewLogger(dir)
	if err != nil {
		t.Fatal(err)
	}
	defer logger.Close()

	_ = logger.StartIteration()
	_ = logger.Log(map[string]any{"msg": "first"})

	_ = logger.StartIteration()
	_ = logger.Log(map[string]any{"msg": "second"})

	logger.Close()

	entries, _ := os.ReadDir(dir)
	if len(entries) != 2 {
		t.Fatalf("expected 2 log files, got %d", len(entries))
	}
}

// ── Log Tests ─────────────────────────────────────────────────────

func TestLog_WritesNDJSONRecords(t *testing.T) {
	frozen := time.Date(2026, 3, 14, 10, 0, 0, 0, time.UTC)
	withFrozenClock(t, frozen)
	dir := t.TempDir()

	logger, err := NewLogger(dir)
	if err != nil {
		t.Fatal(err)
	}

	defer logger.Close()

	if err := logger.StartIteration(); err != nil {
		t.Fatal(err)
	}

	_ = logger.Log(map[string]any{"type": "test", "message": "hello"})
	_ = logger.Log(map[string]any{"type": "test", "message": "world"}, LevelWarn)
	logger.Close()

	entries, _ := os.ReadDir(dir)
	data, err := os.ReadFile(filepath.Join(dir, entries[0].Name()))
	if err != nil {
		t.Fatal(err)
	}

	lines := strings.Split(strings.TrimSpace(string(data)), "\n")
	if len(lines) != 2 {
		t.Fatalf("expected 2 lines, got %d", len(lines))
	}

	var r1, r2 map[string]any
	_ = json.Unmarshal([]byte(lines[0]), &r1)
	_ = json.Unmarshal([]byte(lines[1]), &r2)

	if r1["message"] != "hello" {
		t.Errorf("expected msg=hello, got %v", r1["message"])
	}
	if r1["level"] != "info" {
		t.Errorf("expected default level=info, got %v", r1["level"])
	}
	if r1["timestamp"] != "2026-03-14T10:00:00Z" {
		t.Errorf("unexpected timestamp: %v", r1["timestamp"])
	}
	if r2["level"] != "warn" {
		t.Errorf("expected level=warn, got %v", r2["level"])
	}
}

func TestLog_ReturnsErrorWhenNoIteration(t *testing.T) {
	t.Parallel()
	logger, err := NewLogger(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	defer logger.Close()

	err = logger.Log(map[string]any{"type": "test"})
	if err == nil || !strings.Contains(err.Error(), "no active iteration") {
		t.Errorf("expected 'no active iteration' error, got: %v", err)
	}
}

func TestLog_DefaultLevelIsInfo(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	logger, err := NewLogger(dir)
	if err != nil {
		t.Fatal(err)
	}

	defer logger.Close()

	_ = logger.StartIteration()
	_ = logger.Log(map[string]any{"message": "test"})
	logger.Close()

	entries, _ := os.ReadDir(dir)
	data, _ := os.ReadFile(filepath.Join(dir, entries[0].Name()))
	var record map[string]any
	_ = json.Unmarshal([]byte(strings.TrimSpace(string(data))), &record)

	if record["level"] != "info" {
		t.Errorf("expected level=info, got %v", record["level"])
	}
}

// ── AssistantWriter Tests ─────────────────────────────────────────

func TestAssistantWriter_WritesAtDebugLevel(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()

	logger, err := NewLogger(dir)
	if err != nil {
		t.Fatal(err)
	}

	defer logger.Close()

	_ = logger.StartIteration()
	w := logger.AssistantWriter()
	if w == nil {
		t.Fatal("expected non-nil writer")
	}

	n, err := w.Write([]byte("hello from assistant"))
	if err != nil {
		t.Fatal(err)
	}
	if n != len("hello from assistant") {
		t.Errorf("expected %d bytes written, got %d", len("hello from assistant"), n)
	}

	logger.Close()

	entries, _ := os.ReadDir(dir)
	data, _ := os.ReadFile(filepath.Join(dir, entries[0].Name()))

	var record map[string]any
	_ = json.Unmarshal([]byte(strings.TrimSpace(string(data))), &record)

	if record["type"] != "assistant" {
		t.Errorf("expected type=assistant, got %v", record["type"])
	}
	if record["level"] != "debug" {
		t.Errorf("expected level=debug for assistant, got %v", record["level"])
	}
	if record["text"] != "hello from assistant" {
		t.Errorf("expected text='hello from assistant', got %v", record["text"])
	}
}

func TestAssistantWriter_ReturnsNilWhenNoIteration(t *testing.T) {
	t.Parallel()
	logger, err := NewLogger(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	defer logger.Close()

	if w := logger.AssistantWriter(); w != nil {
		t.Error("expected nil writer when no iteration is active")
	}
}

// ── Close Tests ───────────────────────────────────────────────────

func TestClose_NilFile_DoesNotPanic(t *testing.T) {
	t.Parallel()
	logger, err := NewLogger(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	logger.Close()
	logger.Close() // double close
}

// ── CurrentLogPath Tests ──────────────────────────────────────────

func TestCurrentLogPath_ReturnsEmptyWhenNoIteration(t *testing.T) {
	t.Parallel()
	logger, err := NewLogger(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	defer logger.Close()

	if got := logger.CurrentLogPath(); got != "" {
		t.Errorf("expected empty path, got %q", got)
	}
}

func TestCurrentLogPath_ReturnsPathDuringIteration(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	logger, err := NewLogger(dir)
	if err != nil {
		t.Fatal(err)
	}
	defer logger.Close()

	_ = logger.StartIteration()
	path := logger.CurrentLogPath()
	if path == "" {
		t.Fatal("expected non-empty path")
	}
	if !strings.HasSuffix(path, ".jsonl") {
		t.Errorf("expected .jsonl suffix, got %q", path)
	}
}

// ── LatestLogFile Tests ───────────────────────────────────────────

func TestLatestLogFile_ReturnsMostRecent(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()

	_ = os.WriteFile(filepath.Join(dir, "0001-20260101T00-00Z.jsonl"), []byte("{}"), 0644)
	_ = os.WriteFile(filepath.Join(dir, "0002-20260102T00-00Z.jsonl"), []byte("{}"), 0644)
	_ = os.WriteFile(filepath.Join(dir, "0003-20260103T00-00Z.jsonl"), []byte("{}"), 0644)

	latest, err := LatestLogFile(dir)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.HasSuffix(latest, "0003-20260103T00-00Z.jsonl") {
		t.Errorf("expected 0003, got %q", latest)
	}
}

func TestLatestLogFile_ConsidersGzFiles(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()

	_ = os.WriteFile(filepath.Join(dir, "0001-20260101T00-00Z.jsonl.gz"), []byte("{}"), 0644)
	_ = os.WriteFile(filepath.Join(dir, "0002-20260102T00-00Z.jsonl"), []byte("{}"), 0644)

	latest, err := LatestLogFile(dir)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.HasSuffix(latest, "0002-20260102T00-00Z.jsonl") {
		t.Errorf("expected uncompressed file, got %q", latest)
	}
}

func TestLatestLogFile_IgnoresNonJSONLFiles(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()

	_ = os.WriteFile(filepath.Join(dir, "0001-20260101T00-00Z.jsonl"), []byte("{}"), 0644)
	_ = os.WriteFile(filepath.Join(dir, "notes.txt"), []byte("text"), 0644)
	_ = os.WriteFile(filepath.Join(dir, "readme.md"), []byte("md"), 0644)

	latest, err := LatestLogFile(dir)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.HasSuffix(latest, ".jsonl") {
		t.Errorf("expected .jsonl file, got %q", latest)
	}
}

func TestLatestLogFile_ErrorsOnEmptyDirectory(t *testing.T) {
	t.Parallel()
	_, err := LatestLogFile(t.TempDir())
	if err == nil || !strings.Contains(err.Error(), "no log files found") {
		t.Errorf("expected 'no log files found' error, got: %v", err)
	}
}

func TestLatestLogFile_ErrorsWhenOnlyNonJSONLFiles(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	_ = os.WriteFile(filepath.Join(dir, "notes.txt"), []byte("text"), 0644)

	_, err := LatestLogFile(dir)
	if err == nil {
		t.Error("expected error when no log files exist")
	}
}

func TestLatestLogFile_ErrorsOnMissingDirectory(t *testing.T) {
	t.Parallel()
	_, err := LatestLogFile(filepath.Join(t.TempDir(), "nonexistent"))
	if err == nil {
		t.Error("expected error for missing directory")
	}
}

// ── EnforceRetention Tests ────────────────────────────────────────

func TestEnforceRetention_DeletesOldFilesByCount(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()

	for i := 1; i <= 5; i++ {
		name := filepath.Join(dir, fmt.Sprintf("%04d-20260101T00-00Z.jsonl", i))
		_ = os.WriteFile(name, []byte("{}"), 0644)
	}

	if err := EnforceRetention(dir, 2, 365); err != nil {
		t.Fatal(err)
	}

	remaining := countLogFiles(dir)
	if remaining != 2 {
		t.Errorf("expected 2 files after retention, got %d", remaining)
	}
}

func TestEnforceRetention_DeletesOldFilesByAge(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()

	oldFile := filepath.Join(dir, "0001-20260101T00-00Z.jsonl")
	_ = os.WriteFile(oldFile, []byte("{}"), 0644)
	oldTime := time.Now().AddDate(0, 0, -60)
	_ = os.Chtimes(oldFile, oldTime, oldTime)

	newFile := filepath.Join(dir, "0002-20260301T00-00Z.jsonl")
	_ = os.WriteFile(newFile, []byte("{}"), 0644)

	if err := EnforceRetention(dir, 100, 30); err != nil {
		t.Fatal(err)
	}

	remaining := countLogFiles(dir)
	if remaining != 1 {
		t.Errorf("expected 1 file after age retention, got %d", remaining)
	}
}

func TestEnforceRetention_MaxFilesZero_DeletesAll(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()

	for i := 1; i <= 3; i++ {
		_ = os.WriteFile(filepath.Join(dir, fmt.Sprintf("%04d-20260101T00-00Z.jsonl", i)), []byte("{}"), 0644)
	}

	if err := EnforceRetention(dir, 0, 365); err != nil {
		t.Fatal(err)
	}

	if remaining := countLogFiles(dir); remaining != 0 {
		t.Errorf("expected 0 files, got %d", remaining)
	}
}

func TestEnforceRetention_MaxAgeDaysZero_DeletesAllByAge(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()

	for i := 1; i <= 3; i++ {
		name := filepath.Join(dir, fmt.Sprintf("%04d-20260101T00-00Z.jsonl", i))
		_ = os.WriteFile(name, []byte("{}"), 0644)
		old := time.Now().Add(-time.Second)
		_ = os.Chtimes(name, old, old)
	}

	if err := EnforceRetention(dir, 100, 0); err != nil {
		t.Fatal(err)
	}

	if remaining := countLogFiles(dir); remaining != 0 {
		t.Errorf("expected 0 files, got %d", remaining)
	}
}

func TestEnforceRetention_IgnoresDirectories(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()

	_ = os.WriteFile(filepath.Join(dir, "0001-20260101T00-00Z.jsonl"), []byte("{}"), 0644)
	_ = os.MkdirAll(filepath.Join(dir, "subdir"), 0755)

	if err := EnforceRetention(dir, 100, 365); err != nil {
		t.Fatal(err)
	}

	info, err := os.Stat(filepath.Join(dir, "subdir"))
	if err != nil || !info.IsDir() {
		t.Error("subdirectory should not be affected by retention")
	}
}

func TestEnforceRetention_CountsGzFiles(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()

	for i := 1; i <= 5; i++ {
		suffix := ".jsonl"
		if i <= 3 {
			suffix = ".jsonl.gz"
		}
		_ = os.WriteFile(filepath.Join(dir, fmt.Sprintf("%04d-20260101T00-00Z%s", i, suffix)), []byte("{}"), 0644)
	}

	if err := EnforceRetention(dir, 3, 365); err != nil {
		t.Fatal(err)
	}

	if remaining := countLogFiles(dir); remaining != 3 {
		t.Errorf("expected 3 files after retention, got %d", remaining)
	}
}

func TestEnforceRetention_ErrorOnMissingDir(t *testing.T) {
	t.Parallel()
	err := EnforceRetention(filepath.Join(t.TempDir(), "nonexistent"), 10, 30)
	if err == nil {
		t.Error("expected error for missing directory")
	}
}

// ── Compression Tests ─────────────────────────────────────────────

func TestEnforceRetention_WithCompression(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()

	content := `{"message":"first"}` + "\n"
	_ = os.WriteFile(filepath.Join(dir, "0001-20260101T00-00Z.jsonl"), []byte(content), 0644)
	_ = os.WriteFile(filepath.Join(dir, "0002-20260102T00-00Z.jsonl"), []byte(content), 0644)
	_ = os.WriteFile(filepath.Join(dir, "0003-20260103T00-00Z.jsonl"), []byte(content), 0644)

	if err := EnforceRetention(dir, 100, 365, WithCompression()); err != nil {
		t.Fatal(err)
	}

	entries, _ := os.ReadDir(dir)
	var gzCount, jsonlCount int
	for _, e := range entries {
		if strings.HasSuffix(e.Name(), ".jsonl.gz") {
			gzCount++
		} else if strings.HasSuffix(e.Name(), ".jsonl") {
			jsonlCount++
		}
	}

	// Newest stays uncompressed, older two get compressed
	if gzCount != 2 {
		t.Errorf("expected 2 .gz files, got %d", gzCount)
	}
	if jsonlCount != 1 {
		t.Errorf("expected 1 uncompressed .jsonl file, got %d", jsonlCount)
	}

	// Verify compressed content is valid gzip
	gzPath := filepath.Join(dir, "0001-20260101T00-00Z.jsonl.gz")
	f, err := os.Open(gzPath)
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = f.Close() }()

	gz, err := gzip.NewReader(f)
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = gz.Close() }()

	decompressed, err := io.ReadAll(gz)
	if err != nil {
		t.Fatal(err)
	}
	if string(decompressed) != content {
		t.Errorf("decompressed content mismatch: got %q", decompressed)
	}
}

func TestEnforceRetention_WithCompression_SingleFile_NotCompressed(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()

	_ = os.WriteFile(filepath.Join(dir, "0001-20260101T00-00Z.jsonl"), []byte("{}"), 0644)

	if err := EnforceRetention(dir, 100, 365, WithCompression()); err != nil {
		t.Fatal(err)
	}

	entries, _ := os.ReadDir(dir)
	if len(entries) != 1 {
		t.Fatalf("expected 1 file, got %d", len(entries))
	}
	if !strings.HasSuffix(entries[0].Name(), ".jsonl") {
		t.Error("single file should not be compressed (it might be active)")
	}
}

func TestCompressFile(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	src := filepath.Join(dir, "test.jsonl")
	content := "line1\nline2\n"
	_ = os.WriteFile(src, []byte(content), 0644)

	if err := compressFile(src); err != nil {
		t.Fatal(err)
	}

	// Original should be removed
	if _, err := os.Stat(src); !os.IsNotExist(err) {
		t.Error("expected original file to be removed")
	}

	// Compressed file should exist
	gz, err := os.Open(src + ".gz")
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = gz.Close() }()

	r, err := gzip.NewReader(gz)
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = r.Close() }()

	decompressed, _ := io.ReadAll(r)
	if string(decompressed) != content {
		t.Errorf("expected %q, got %q", content, decompressed)
	}
}

func TestCompressFile_MissingSrc(t *testing.T) {
	t.Parallel()
	err := compressFile(filepath.Join(t.TempDir(), "nonexistent"))
	if err == nil {
		t.Error("expected error for missing source file")
	}
}

// ── IterationFromDir Tests ────────────────────────────────────────

func TestIterationFromDir_Empty(t *testing.T) {
	t.Parallel()
	if got := IterationFromDir(t.TempDir()); got != 0 {
		t.Errorf("expected 0 for empty dir, got %d", got)
	}
}

func TestIterationFromDir_FindsHighest(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()

	_ = os.WriteFile(filepath.Join(dir, "0003-20260101T00-00Z.jsonl"), []byte("{}"), 0644)
	_ = os.WriteFile(filepath.Join(dir, "0007-20260102T00-00Z.jsonl"), []byte("{}"), 0644)
	_ = os.WriteFile(filepath.Join(dir, "0005-20260103T00-00Z.jsonl.gz"), []byte("{}"), 0644)

	if got := IterationFromDir(dir); got != 7 {
		t.Errorf("expected 7, got %d", got)
	}
}

func TestIterationFromDir_IgnoresNonLogFiles(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()

	_ = os.WriteFile(filepath.Join(dir, "0003-20260101T00-00Z.jsonl"), []byte("{}"), 0644)
	_ = os.WriteFile(filepath.Join(dir, "notes.txt"), []byte("text"), 0644)

	if got := IterationFromDir(dir); got != 3 {
		t.Errorf("expected 3, got %d", got)
	}
}

func TestIterationFromDir_MissingDir(t *testing.T) {
	t.Parallel()
	if got := IterationFromDir(filepath.Join(t.TempDir(), "nope")); got != 0 {
		t.Errorf("expected 0 for missing dir, got %d", got)
	}
}

// ── WatchForNewFiles Tests ────────────────────────────────────────

func TestWatchForNewFiles_DetectsNewFile(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	currentPath := filepath.Join(dir, "0001-20260101T00-00Z.jsonl")
	_ = os.WriteFile(currentPath, []byte("{}"), 0644)

	done := make(chan struct{})

	// Write the new file after a tiny delay
	go func() {
		time.Sleep(50 * time.Millisecond)
		_ = os.WriteFile(filepath.Join(dir, "0002-20260102T00-00Z.jsonl"), []byte("{}"), 0644)
	}()

	result := WatchForNewFiles(dir, currentPath, done, 20*time.Millisecond)
	if !strings.Contains(result, "0002") {
		t.Errorf("expected to detect new file, got %q", result)
	}
}

func TestWatchForNewFiles_StopsOnDone(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	currentPath := filepath.Join(dir, "0001-20260101T00-00Z.jsonl")
	_ = os.WriteFile(currentPath, []byte("{}"), 0644)

	done := make(chan struct{})
	go func() {
		time.Sleep(50 * time.Millisecond)
		close(done)
	}()

	result := WatchForNewFiles(dir, currentPath, done, 20*time.Millisecond)
	if result != "" {
		t.Errorf("expected empty string on done, got %q", result)
	}
}

// ── MultipleIterations Tests ──────────────────────────────────────

func TestMultipleIterations_CreateSeparateFiles(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()

	logger, err := NewLogger(dir)
	if err != nil {
		t.Fatal(err)
	}

	defer logger.Close()

	for i := 0; i < 3; i++ {
		if err := logger.StartIteration(); err != nil {
			t.Fatal(err)
		}
		_ = logger.Log(map[string]any{"iter": i})
	}
	logger.Close()

	if count := countLogFiles(dir); count != 3 {
		t.Errorf("expected 3 log files, got %d", count)
	}
}

// ── isLogFile Tests ───────────────────────────────────────────────

func TestIsLogFile(t *testing.T) {
	t.Parallel()
	cases := []struct {
		name string
		want bool
	}{
		{"0001-20260101T00-00Z.jsonl", true},
		{"0001-20260101T00-00Z.jsonl.gz", true},
		{"notes.txt", false},
		{"data.json", false},
		{".jsonl", true},
		{"foo.jsonl.gz", true},
	}
	for _, tc := range cases {
		if got := isLogFile(tc.name); got != tc.want {
			t.Errorf("isLogFile(%q) = %v, want %v", tc.name, got, tc.want)
		}
	}
}

// ── Integration Test ──────────────────────────────────────────────

func TestFullLifecycle(t *testing.T) {
	frozen := time.Date(2026, 3, 14, 15, 0, 0, 0, time.UTC)
	withFrozenClock(t, frozen)

	dir := t.TempDir()

	logger, err := NewLogger(dir)
	if err != nil {
		t.Fatal(err)
	}

	// Iteration 1
	_ = logger.StartIteration()
	_ = logger.Log(map[string]any{"message": "daemon starting", "type": "daemon_start"}, LevelInfo)
	_ = logger.Log(map[string]any{"message": "skip details", "stage": "expand"}, LevelDebug)
	_ = logger.Log(map[string]any{"message": "stage done", "stage": "execute"}, LevelInfo)
	logger.Close()

	// Verify NDJSON has all 3 records
	entries, _ := os.ReadDir(dir)
	data, _ := os.ReadFile(filepath.Join(dir, entries[0].Name()))
	lines := strings.Split(strings.TrimSpace(string(data)), "\n")
	if len(lines) != 3 {
		t.Fatalf("NDJSON: expected 3 lines, got %d", len(lines))
	}

	// Verify iteration counter
	if logger.Iteration != 1 {
		t.Errorf("expected iteration=1, got %d", logger.Iteration)
	}

	// Iteration 2
	_ = logger.StartIteration()
	_ = logger.Log(map[string]any{"message": "warn event"}, LevelWarn)
	logger.Close()

	if logger.Iteration != 2 {
		t.Errorf("expected iteration=2, got %d", logger.Iteration)
	}

	// IterationFromDir should find 2
	if got := IterationFromDir(dir); got != 2 {
		t.Errorf("IterationFromDir = %d, want 2", got)
	}

	// LatestLogFile should return iteration 2
	latest, err := LatestLogFile(dir)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(latest, "0002") {
		t.Errorf("expected latest to be iteration 2, got %q", latest)
	}
}

// ── Edge Cases ────────────────────────────────────────────────────

func TestNewLogger_ErrorOnInvalidPath(t *testing.T) {
	t.Parallel()
	// Use a path that can't be created (file exists where dir should be)
	dir := t.TempDir()
	blocker := filepath.Join(dir, "blocker")
	_ = os.WriteFile(blocker, []byte("x"), 0644)

	_, err := NewLogger(filepath.Join(blocker, "logs"))
	if err == nil {
		t.Error("expected error when directory can't be created")
	}
}

func TestLog_MarshalError(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	logger, err := NewLogger(dir)
	if err != nil {
		t.Fatal(err)
	}

	defer logger.Close()

	_ = logger.StartIteration()

	// A channel value can't be marshalled to JSON
	err = logger.Log(map[string]any{"bad": make(chan int)})
	if err == nil {
		t.Error("expected marshal error for channel value")
	}
}

func TestAssistantWriter_WriteError(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	logger, err := NewLogger(dir)
	if err != nil {
		t.Fatal(err)
	}

	_ = logger.StartIteration()
	w := logger.AssistantWriter()

	// Close the file to force write errors
	_ = logger.file.Close()

	n, err := w.Write([]byte("should fail"))
	if err == nil {
		t.Error("expected error writing to closed file")
	}
	if n != 0 {
		t.Errorf("expected 0 bytes on error, got %d", n)
	}
	// Reset so Close() doesn't double-close
	logger.file = nil
}

func TestCompressFile_ReadOnlyDst(t *testing.T) {
	t.Parallel()
	if runtime.GOOS == "windows" {
		t.Skip("chmod restrictions have no effect on Windows")
	}
	dir := t.TempDir()
	src := filepath.Join(dir, "test.jsonl")
	_ = os.WriteFile(src, []byte("data"), 0644)

	// Create a read-only directory where .gz can't be written
	roDir := filepath.Join(dir, "readonly")
	_ = os.MkdirAll(roDir, 0755)
	roSrc := filepath.Join(roDir, "test.jsonl")
	_ = os.WriteFile(roSrc, []byte("data"), 0644)
	_ = os.Chmod(roDir, 0555)
	defer func() { _ = os.Chmod(roDir, 0755) }() // cleanup

	err := compressFile(roSrc)
	if err == nil {
		t.Error("expected error when destination can't be created")
	}
}

// ── Trace ID Tests ────────────────────────────────────────────────

func TestStartIterationWithPrefix_SetsTraceID(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	logger, err := NewLogger(dir)
	if err != nil {
		t.Fatal(err)
	}
	defer logger.Close()

	_ = logger.StartIterationWithPrefix("exec")
	if logger.TraceID != "exec-0001" {
		t.Errorf("TraceID = %q, want exec-0001", logger.TraceID)
	}

	_ = logger.StartIterationWithPrefix("intake")
	if logger.TraceID != "intake-0002" {
		t.Errorf("TraceID = %q, want intake-0002", logger.TraceID)
	}
}

func TestStartIteration_SetsDefaultTracePrefix(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	logger, err := NewLogger(dir)
	if err != nil {
		t.Fatal(err)
	}
	defer logger.Close()

	_ = logger.StartIteration()
	if logger.TraceID != "iter-0001" {
		t.Errorf("TraceID = %q, want iter-0001", logger.TraceID)
	}
}

// ── LogIterationStart Tests ───────────────────────────────────────

func TestLogIterationStart_EmitsRecord(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	logger, err := NewLogger(dir)
	if err != nil {
		t.Fatal(err)
	}

	defer logger.Close()

	_ = logger.StartIterationWithPrefix("exec")
	if err := logger.LogIterationStart("execute", "my-project/auth"); err != nil {
		t.Fatalf("LogIterationStart returned error: %v", err)
	}

	// Read back the log file and parse the record.
	logPath := logger.CurrentLogPath()
	data, err := os.ReadFile(logPath)
	if err != nil {
		t.Fatalf("reading log file: %v", err)
	}
	var rec map[string]any
	if err := json.Unmarshal(data, &rec); err != nil {
		t.Fatalf("parsing log record: %v", err)
	}

	if rec["type"] != "iteration_start" {
		t.Errorf("type = %v, want iteration_start", rec["type"])
	}
	if rec["stage"] != "execute" {
		t.Errorf("stage = %v, want execute", rec["stage"])
	}
	if rec["node"] != "my-project/auth" {
		t.Errorf("node = %v, want my-project/auth", rec["node"])
	}
	// iteration is stored as float64 by JSON unmarshal
	if iter, ok := rec["iteration"].(float64); !ok || int(iter) != 1 {
		t.Errorf("iteration = %v, want 1", rec["iteration"])
	}
}

func TestLogIterationStart_OmitsNodeWhenEmpty(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	logger, err := NewLogger(dir)
	if err != nil {
		t.Fatal(err)
	}

	defer logger.Close()

	_ = logger.StartIterationWithPrefix("intake")
	_ = logger.LogIterationStart("intake", "")

	data, err := os.ReadFile(logger.CurrentLogPath())
	if err != nil {
		t.Fatal(err)
	}
	var rec map[string]any
	if err := json.Unmarshal(data, &rec); err != nil {
		t.Fatal(err)
	}
	if _, hasNode := rec["node"]; hasNode {
		t.Error("expected no 'node' field when nodeAddr is empty")
	}
}

func TestLog_IncludesTraceID(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	logger, err := NewLogger(dir)
	if err != nil {
		t.Fatal(err)
	}

	defer logger.Close()

	_ = logger.StartIterationWithPrefix("exec")
	_ = logger.Log(map[string]any{"type": "test"})

	path := logger.CurrentLogPath()
	data, _ := os.ReadFile(path)
	var record map[string]any
	_ = json.Unmarshal(bytes.TrimSpace(data), &record)

	trace, ok := record["trace"].(string)
	if !ok || trace != "exec-0001" {
		t.Errorf("trace = %q, want exec-0001", trace)
	}
}

func TestLog_OmitsTraceWhenEmpty(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	logger, err := NewLogger(dir)
	if err != nil {
		t.Fatal(err)
	}

	logger.TraceID = ""
	defer logger.Close()

	// Manually set up a file without going through StartIteration
	// to test that empty TraceID is omitted
	_ = logger.StartIteration()
	logger.TraceID = "" // clear it after StartIteration sets it
	_ = logger.Log(map[string]any{"type": "test"})

	path := logger.CurrentLogPath()
	data, _ := os.ReadFile(path)
	lines := strings.Split(strings.TrimSpace(string(data)), "\n")
	lastLine := lines[len(lines)-1]
	if strings.Contains(lastLine, `"trace"`) {
		t.Error("expected no trace field when TraceID is empty")
	}
}

// ── Record Shape Tests ────────────────────────────────────────────
//
// These tests verify that each NDJSON record type consumed by the log
// command contains the fields the spec requires. The Logger.Log method
// adds timestamp, level, and trace automatically; these tests confirm
// that the caller-supplied fields survive serialization and that
// Logger.Log injects the envelope fields.

func TestRecordShape_StageStart(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	logger, err := NewLogger(dir)
	if err != nil {
		t.Fatal(err)
	}

	defer logger.Close()
	_ = logger.StartIterationWithPrefix("exec")

	// Emit the record with the fields the execute loop provides.
	_ = logger.Log(map[string]any{
		"type":  "stage_start",
		"stage": "execute",
		"node":  "my-project/auth",
		"task":  "task-0001",
		"model": "sonnet",
	})

	rec := readSingleRecord(t, logger.CurrentLogPath())

	// Spec-required fields.
	requireField(t, rec, "type", "stage_start")
	requireField(t, rec, "stage", "execute")
	requireField(t, rec, "node", "my-project/auth")

	// Envelope fields injected by Logger.Log.
	requireFieldPresent(t, rec, "timestamp")
	requireFieldPresent(t, rec, "level")
	requireFieldPresent(t, rec, "trace")
}

func TestRecordShape_StageComplete(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	logger, err := NewLogger(dir)
	if err != nil {
		t.Fatal(err)
	}

	defer logger.Close()
	_ = logger.StartIterationWithPrefix("exec")

	_ = logger.Log(map[string]any{
		"type":       "stage_complete",
		"stage":      "execute",
		"exit_code":  0,
		"output_len": 512,
	})

	rec := readSingleRecord(t, logger.CurrentLogPath())

	requireField(t, rec, "type", "stage_complete")
	requireField(t, rec, "stage", "execute")
	requireFieldPresent(t, rec, "exit_code")
	requireFieldPresent(t, rec, "timestamp") // needed for duration calculation
}

func TestRecordShape_PlanningStart(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	logger, err := NewLogger(dir)
	if err != nil {
		t.Fatal(err)
	}

	defer logger.Close()
	_ = logger.StartIterationWithPrefix("exec")

	_ = logger.Log(map[string]any{
		"type":    "planning_start",
		"node":    "my-project",
		"trigger": "new_orchestrator",
		"model":   "sonnet",
	})

	rec := readSingleRecord(t, logger.CurrentLogPath())

	requireField(t, rec, "type", "planning_start")
	requireField(t, rec, "node", "my-project")
	requireFieldPresent(t, rec, "timestamp")
}

func TestRecordShape_PlanningComplete(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	logger, err := NewLogger(dir)
	if err != nil {
		t.Fatal(err)
	}

	defer logger.Close()
	_ = logger.StartIterationWithPrefix("exec")

	_ = logger.Log(map[string]any{
		"type":      "planning_complete",
		"node":      "my-project",
		"exit_code": 0,
	})

	rec := readSingleRecord(t, logger.CurrentLogPath())

	requireField(t, rec, "type", "planning_complete")
	requireField(t, rec, "node", "my-project")
	requireFieldPresent(t, rec, "exit_code")
	requireFieldPresent(t, rec, "timestamp")
}

func TestRecordShape_AuditReportWritten(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	logger, err := NewLogger(dir)
	if err != nil {
		t.Fatal(err)
	}

	defer logger.Close()
	_ = logger.StartIterationWithPrefix("exec")

	_ = logger.Log(map[string]any{
		"type": "audit_report_written",
		"node": "my-project/auth",
		"path": ".wolfcastle/system/projects/eng/my-project/auth/audit-2026-03-21.md",
	})

	rec := readSingleRecord(t, logger.CurrentLogPath())

	requireField(t, rec, "type", "audit_report_written")
	requireFieldPresent(t, rec, "path")
	requireFieldPresent(t, rec, "timestamp")
}

// readSingleRecord reads a log file and parses the first NDJSON line.
func readSingleRecord(t *testing.T, path string) map[string]any {
	t.Helper()
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("reading log file: %v", err)
	}
	lines := strings.Split(strings.TrimSpace(string(data)), "\n")
	if len(lines) == 0 {
		t.Fatal("log file is empty")
	}
	var rec map[string]any
	if err := json.Unmarshal([]byte(lines[0]), &rec); err != nil {
		t.Fatalf("parsing log record: %v", err)
	}
	return rec
}

// requireField asserts that rec[key] equals the expected string value.
func requireField(t *testing.T, rec map[string]any, key, want string) {
	t.Helper()
	got, ok := rec[key]
	if !ok {
		t.Errorf("record missing required field %q", key)
		return
	}
	if fmt.Sprint(got) != want {
		t.Errorf("field %q = %v, want %q", key, got, want)
	}
}

// requireFieldPresent asserts that rec[key] exists (any value).
func requireFieldPresent(t *testing.T, rec map[string]any, key string) {
	t.Helper()
	if _, ok := rec[key]; !ok {
		t.Errorf("record missing required field %q", key)
	}
}

// ── Child Tests ──────────────────────────────────────────────────

func TestChild_SharesLogDir(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	parent, err := NewLogger(dir)
	if err != nil {
		t.Fatal(err)
	}
	defer parent.Close()

	child := parent.Child("worker")
	if child.LogDir != parent.LogDir {
		t.Errorf("child.LogDir = %q, want %q", child.LogDir, parent.LogDir)
	}
}

func TestChild_IndependentIteration(t *testing.T) {
	frozen := time.Date(2026, 3, 14, 12, 30, 0, 0, time.UTC)
	withFrozenClock(t, frozen)
	dir := t.TempDir()

	parent, err := NewLogger(dir)
	if err != nil {
		t.Fatal(err)
	}
	defer parent.Close()

	// Advance the parent a few iterations.
	for i := 0; i < 3; i++ {
		if err := parent.StartIteration(); err != nil {
			t.Fatal(err)
		}
	}

	child := parent.Child("worker")
	defer child.Close()

	if child.Iteration != 0 {
		t.Errorf("child.Iteration = %d, want 0", child.Iteration)
	}
	if err := child.StartIteration(); err != nil {
		t.Fatal(err)
	}
	if child.Iteration != 1 {
		t.Errorf("after StartIteration, child.Iteration = %d, want 1", child.Iteration)
	}
	// Parent unchanged.
	if parent.Iteration != 3 {
		t.Errorf("parent.Iteration = %d, want 3", parent.Iteration)
	}
}

func TestChild_IndependentFileHandle(t *testing.T) {
	frozen := time.Date(2026, 3, 14, 12, 30, 0, 0, time.UTC)
	withFrozenClock(t, frozen)
	dir := t.TempDir()

	parent, err := NewLogger(dir)
	if err != nil {
		t.Fatal(err)
	}
	defer parent.Close()

	child := parent.Child("worker")
	defer child.Close()

	if child.file != nil {
		t.Error("child.file should be nil before StartIteration")
	}

	if err := child.StartIteration(); err != nil {
		t.Fatal(err)
	}
	if child.file == nil {
		t.Error("child.file should be non-nil after StartIteration")
	}
	// Parent's file handle should still be nil (never started).
	if parent.file != nil {
		t.Error("parent.file should still be nil")
	}
}

func TestChild_UsesDefaultPrefix(t *testing.T) {
	frozen := time.Date(2026, 3, 14, 12, 30, 0, 0, time.UTC)
	withFrozenClock(t, frozen)
	dir := t.TempDir()

	parent, err := NewLogger(dir)
	if err != nil {
		t.Fatal(err)
	}
	defer parent.Close()

	child := parent.Child("worker")
	defer child.Close()

	if err := child.StartIteration(); err != nil {
		t.Fatal(err)
	}
	if child.TraceID != "worker-0001" {
		t.Errorf("child.TraceID = %q, want %q", child.TraceID, "worker-0001")
	}
}

func TestChild_ParentUnmodified(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	parent, err := NewLogger(dir)
	if err != nil {
		t.Fatal(err)
	}
	defer parent.Close()

	origIter := parent.Iteration
	origTrace := parent.TraceID

	_ = parent.Child("worker")

	if parent.Iteration != origIter {
		t.Errorf("parent.Iteration changed from %d to %d", origIter, parent.Iteration)
	}
	if parent.TraceID != origTrace {
		t.Errorf("parent.TraceID changed from %q to %q", origTrace, parent.TraceID)
	}
}

func TestStartIteration_UsesDefaultPrefix(t *testing.T) {
	frozen := time.Date(2026, 3, 14, 12, 30, 0, 0, time.UTC)
	withFrozenClock(t, frozen)
	dir := t.TempDir()

	logger, err := NewLogger(dir)
	if err != nil {
		t.Fatal(err)
	}
	defer logger.Close()

	// Default prefix for a plain Logger is "iter".
	if err := logger.StartIteration(); err != nil {
		t.Fatal(err)
	}
	if logger.TraceID != "iter-0001" {
		t.Errorf("TraceID = %q, want %q", logger.TraceID, "iter-0001")
	}
}

// ── Helper ────────────────────────────────────────────────────────

func countLogFiles(dir string) int {
	entries, _ := os.ReadDir(dir)
	count := 0
	for _, e := range entries {
		if !e.IsDir() && isLogFile(e.Name()) {
			count++
		}
	}
	return count
}
