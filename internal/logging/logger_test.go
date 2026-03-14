package logging

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

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

func TestStartIteration_CreatesNumberedLogFile(t *testing.T) {
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

	entries, _ := os.ReadDir(dir)
	if len(entries) != 1 {
		t.Fatalf("expected 1 log file, got %d", len(entries))
	}

	name := entries[0].Name()
	if !strings.HasPrefix(name, "0001-") {
		t.Errorf("expected file starting with 0001-, got %q", name)
	}
	if !strings.HasSuffix(name, ".jsonl") {
		t.Errorf("expected .jsonl suffix, got %q", name)
	}
}

func TestLog_WritesNDJSONRecords(t *testing.T) {
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

	if err := logger.Log(map[string]any{"type": "test", "msg": "hello"}); err != nil {
		t.Fatal(err)
	}
	if err := logger.Log(map[string]any{"type": "test", "msg": "world"}); err != nil {
		t.Fatal(err)
	}

	logger.Close()

	// Read the file
	entries, _ := os.ReadDir(dir)
	data, err := os.ReadFile(filepath.Join(dir, entries[0].Name()))
	if err != nil {
		t.Fatal(err)
	}

	lines := strings.Split(strings.TrimSpace(string(data)), "\n")
	if len(lines) != 2 {
		t.Fatalf("expected 2 lines, got %d", len(lines))
	}

	var record map[string]any
	if err := json.Unmarshal([]byte(lines[0]), &record); err != nil {
		t.Fatal(err)
	}
	if record["msg"] != "hello" {
		t.Errorf("expected msg=hello, got %v", record["msg"])
	}
	if _, ok := record["timestamp"]; !ok {
		t.Error("expected timestamp field")
	}
}

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
		logger.Log(map[string]any{"iter": i})
	}
	logger.Close()

	entries, _ := os.ReadDir(dir)
	count := 0
	for _, e := range entries {
		if strings.HasSuffix(e.Name(), ".jsonl") {
			count++
		}
	}
	if count != 3 {
		t.Errorf("expected 3 log files, got %d", count)
	}
}

func TestLatestLogFile_ReturnsMostRecent(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()

	// Create files with predictable names
	os.WriteFile(filepath.Join(dir, "0001-20260101T00-00Z.jsonl"), []byte("{}"), 0644)
	os.WriteFile(filepath.Join(dir, "0002-20260102T00-00Z.jsonl"), []byte("{}"), 0644)
	os.WriteFile(filepath.Join(dir, "0003-20260103T00-00Z.jsonl"), []byte("{}"), 0644)

	latest, err := LatestLogFile(dir)
	if err != nil {
		t.Fatal(err)
	}

	if !strings.HasSuffix(latest, "0003-20260103T00-00Z.jsonl") {
		t.Errorf("expected latest to be 0003, got %q", latest)
	}
}

func TestEnforceRetention_DeletesOldFilesByCount(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()

	for i := 1; i <= 5; i++ {
		name := filepath.Join(dir, fmt.Sprintf("000%d-20260101T00-00Z.jsonl", i))
		os.WriteFile(name, []byte("{}"), 0644)
	}

	if err := EnforceRetention(dir, 2, 365); err != nil {
		t.Fatal(err)
	}

	entries, _ := os.ReadDir(dir)
	count := 0
	for _, e := range entries {
		if strings.HasSuffix(e.Name(), ".jsonl") {
			count++
		}
	}
	if count != 2 {
		t.Errorf("expected 2 files after retention, got %d", count)
	}
}

func TestEnforceRetention_DeletesOldFilesByAge(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()

	// Create old file
	oldFile := filepath.Join(dir, "0001-20260101T00-00Z.jsonl")
	os.WriteFile(oldFile, []byte("{}"), 0644)
	// Set modification time to 60 days ago
	oldTime := time.Now().AddDate(0, 0, -60)
	os.Chtimes(oldFile, oldTime, oldTime)

	// Create recent file
	newFile := filepath.Join(dir, "0002-20260301T00-00Z.jsonl")
	os.WriteFile(newFile, []byte("{}"), 0644)

	if err := EnforceRetention(dir, 100, 30); err != nil {
		t.Fatal(err)
	}

	entries, _ := os.ReadDir(dir)
	count := 0
	for _, e := range entries {
		if strings.HasSuffix(e.Name(), ".jsonl") {
			count++
		}
	}
	if count != 1 {
		t.Errorf("expected 1 file after age retention, got %d", count)
	}
}

func TestAssistantWriter_WritesNDJSONWithAssistantType(t *testing.T) {
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
	data, err := os.ReadFile(filepath.Join(dir, entries[0].Name()))
	if err != nil {
		t.Fatal(err)
	}

	lines := strings.Split(strings.TrimSpace(string(data)), "\n")
	if len(lines) != 1 {
		t.Fatalf("expected 1 line, got %d", len(lines))
	}

	var record map[string]any
	if err := json.Unmarshal([]byte(lines[0]), &record); err != nil {
		t.Fatal(err)
	}
	if record["type"] != "assistant" {
		t.Errorf("expected type=assistant, got %v", record["type"])
	}
	if record["text"] != "hello from assistant" {
		t.Errorf("expected text='hello from assistant', got %v", record["text"])
	}
	if _, ok := record["timestamp"]; !ok {
		t.Error("expected timestamp field in assistant record")
	}
}

func TestAssistantWriter_ReturnsNilWhenNoIteration(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()

	logger, err := NewLogger(dir)
	if err != nil {
		t.Fatal(err)
	}
	defer logger.Close()

	w := logger.AssistantWriter()
	if w != nil {
		t.Error("expected nil writer when no iteration is active")
	}
}

func TestLog_ReturnsErrorWhenNoIteration(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()

	logger, err := NewLogger(dir)
	if err != nil {
		t.Fatal(err)
	}
	defer logger.Close()

	err = logger.Log(map[string]any{"type": "test"})
	if err == nil {
		t.Error("expected error when logging without active iteration")
	}
	if !strings.Contains(err.Error(), "no active iteration") {
		t.Errorf("expected 'no active iteration' error, got: %v", err)
	}
}

func TestLatestLogFile_IgnoresNonJSONLFiles(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()

	// Create non-jsonl files that sort after jsonl files
	os.WriteFile(filepath.Join(dir, "0001-20260101T00-00Z.jsonl"), []byte("{}"), 0644)
	os.WriteFile(filepath.Join(dir, "notes.txt"), []byte("text"), 0644)
	os.WriteFile(filepath.Join(dir, "0002-20260102T00-00Z.log"), []byte("log"), 0644)
	os.WriteFile(filepath.Join(dir, "readme.md"), []byte("md"), 0644)

	latest, err := LatestLogFile(dir)
	if err != nil {
		t.Fatal(err)
	}

	if !strings.HasSuffix(latest, "0001-20260101T00-00Z.jsonl") {
		t.Errorf("expected the only jsonl file, got %q", latest)
	}
}

func TestLatestLogFile_ErrorsOnEmptyDirectory(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()

	_, err := LatestLogFile(dir)
	if err == nil {
		t.Error("expected error for empty directory")
	}
	if !strings.Contains(err.Error(), "no log files found") {
		t.Errorf("expected 'no log files found' error, got: %v", err)
	}
}

func TestLatestLogFile_ErrorsWhenOnlyNonJSONLFiles(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()

	os.WriteFile(filepath.Join(dir, "notes.txt"), []byte("text"), 0644)
	os.WriteFile(filepath.Join(dir, "data.json"), []byte("{}"), 0644)

	_, err := LatestLogFile(dir)
	if err == nil {
		t.Error("expected error when no .jsonl files exist")
	}
}

func TestEnforceRetention_MaxFilesZero_DeletesAll(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()

	for i := 1; i <= 3; i++ {
		name := filepath.Join(dir, fmt.Sprintf("000%d-20260101T00-00Z.jsonl", i))
		os.WriteFile(name, []byte("{}"), 0644)
	}

	if err := EnforceRetention(dir, 0, 365); err != nil {
		t.Fatal(err)
	}

	entries, _ := os.ReadDir(dir)
	count := 0
	for _, e := range entries {
		if strings.HasSuffix(e.Name(), ".jsonl") {
			count++
		}
	}
	if count != 0 {
		t.Errorf("expected 0 files after maxFiles=0 retention, got %d", count)
	}
}

func TestEnforceRetention_MaxAgeDaysZero_DeletesAllByAge(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()

	// Create files — all will be "old" since maxAgeDays=0 means cutoff is now
	for i := 1; i <= 3; i++ {
		name := filepath.Join(dir, fmt.Sprintf("000%d-20260101T00-00Z.jsonl", i))
		os.WriteFile(name, []byte("{}"), 0644)
		// Set mod time to 1 second ago to be before cutoff
		old := time.Now().Add(-time.Second)
		os.Chtimes(name, old, old)
	}

	if err := EnforceRetention(dir, 100, 0); err != nil {
		t.Fatal(err)
	}

	entries, _ := os.ReadDir(dir)
	count := 0
	for _, e := range entries {
		if strings.HasSuffix(e.Name(), ".jsonl") {
			count++
		}
	}
	if count != 0 {
		t.Errorf("expected 0 files after maxAgeDays=0 retention, got %d", count)
	}
}

func TestEnforceRetention_IgnoresDirectories(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()

	os.WriteFile(filepath.Join(dir, "0001-20260101T00-00Z.jsonl"), []byte("{}"), 0644)
	os.MkdirAll(filepath.Join(dir, "subdir"), 0755)

	if err := EnforceRetention(dir, 100, 365); err != nil {
		t.Fatal(err)
	}

	// Directory should still exist
	info, err := os.Stat(filepath.Join(dir, "subdir"))
	if err != nil {
		t.Fatal("subdirectory should not be affected by retention")
	}
	if !info.IsDir() {
		t.Error("expected subdirectory to remain")
	}
}

func TestClose_NilFile_DoesNotPanic(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()

	logger, err := NewLogger(dir)
	if err != nil {
		t.Fatal(err)
	}

	// Close without starting iteration should not panic
	logger.Close()
	// Double close should also not panic
	logger.Close()
}

func TestStartIteration_IncrementsCounter(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()

	logger, err := NewLogger(dir)
	if err != nil {
		t.Fatal(err)
	}
	defer logger.Close()

	logger.StartIteration()
	if logger.Iteration != 1 {
		t.Errorf("expected iteration=1, got %d", logger.Iteration)
	}

	logger.StartIteration()
	if logger.Iteration != 2 {
		t.Errorf("expected iteration=2, got %d", logger.Iteration)
	}
}
