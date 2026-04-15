package logging

import (
	"compress/gzip"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
	"time"
)

// ═══════════════════════════════════════════════════════════════════════════
// compressFile: additional error paths
// ═══════════════════════════════════════════════════════════════════════════

func TestCompressFile_ValidContent(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	src := filepath.Join(dir, "big.jsonl")
	// Write substantial content
	content := strings.Repeat(`{"message":"test line","level":"info"}`+"\n", 100)
	_ = os.WriteFile(src, []byte(content), 0644)

	if err := compressFile(src); err != nil {
		t.Fatal(err)
	}

	// Verify original removed
	if _, err := os.Stat(src); !os.IsNotExist(err) {
		t.Error("original should be removed")
	}

	// Verify compressed file is valid and decompresses correctly
	f, err := os.Open(src + ".gz")
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = f.Close() }()

	gz, err := gzip.NewReader(f)
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = gz.Close() }()

	data, err := io.ReadAll(gz)
	if err != nil {
		t.Fatal(err)
	}
	if string(data) != content {
		t.Error("decompressed content mismatch")
	}
}

func TestCompressFile_EmptyFile(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	src := filepath.Join(dir, "empty.jsonl")
	_ = os.WriteFile(src, []byte{}, 0644)

	if err := compressFile(src); err != nil {
		t.Fatal(err)
	}

	if _, err := os.Stat(src); !os.IsNotExist(err) {
		t.Error("original should be removed")
	}
	if _, err := os.Stat(src + ".gz"); err != nil {
		t.Error("compressed file should exist")
	}
}

// ═══════════════════════════════════════════════════════════════════════════
// EnforceRetention: age-based cleanup with frozen clock
// ═══════════════════════════════════════════════════════════════════════════

func TestEnforceRetention_AgeCutoff_WithFrozenClock(t *testing.T) {
	frozen := time.Date(2026, 3, 14, 12, 0, 0, 0, time.UTC)
	withFrozenClock(t, frozen)
	dir := t.TempDir()

	// File from 40 days ago
	oldFile := filepath.Join(dir, "0001-20260201T00-00Z.jsonl")
	_ = os.WriteFile(oldFile, []byte("{}"), 0644)
	oldTime := frozen.AddDate(0, 0, -40)
	_ = os.Chtimes(oldFile, oldTime, oldTime)

	// Recent file
	newFile := filepath.Join(dir, "0002-20260314T00-00Z.jsonl")
	_ = os.WriteFile(newFile, []byte("{}"), 0644)

	if err := EnforceRetention(dir, 100, 30, WithQuietWindow(0)); err != nil {
		t.Fatal(err)
	}

	if _, err := os.Stat(oldFile); !os.IsNotExist(err) {
		t.Error("old file should be deleted by age")
	}
	if _, err := os.Stat(newFile); err != nil {
		t.Error("new file should remain")
	}
}

func TestEnforceRetention_CompressionWithMultipleFiles(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()

	content := `{"level":"info"}` + "\n"
	backdate := time.Now().Add(-2 * time.Minute)
	for i := 1; i <= 5; i++ {
		name := filepath.Join(dir, fmt.Sprintf("%04d-20260101T00-00Z.jsonl", i))
		_ = os.WriteFile(name, []byte(content), 0644)
		_ = os.Chtimes(name, backdate, backdate)
	}

	if err := EnforceRetention(dir, 100, 365, WithCompression(), WithQuietWindow(0)); err != nil {
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

	// Newest stays uncompressed, 4 get compressed
	if gzCount != 4 {
		t.Errorf("expected 4 .gz files, got %d", gzCount)
	}
	if jsonlCount != 1 {
		t.Errorf("expected 1 uncompressed .jsonl, got %d", jsonlCount)
	}
}

func TestEnforceRetention_MixedGzAndPlain(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()

	// Some already compressed, some plain
	_ = os.WriteFile(filepath.Join(dir, "0001-20260101T00-00Z.jsonl.gz"), []byte("compressed"), 0644)
	_ = os.WriteFile(filepath.Join(dir, "0002-20260102T00-00Z.jsonl"), []byte("{}"), 0644)
	_ = os.WriteFile(filepath.Join(dir, "0003-20260103T00-00Z.jsonl"), []byte("{}"), 0644)
	// Back-date the plain files so compression is eligible immediately.
	backdate := time.Now().Add(-2 * time.Minute)
	_ = os.Chtimes(filepath.Join(dir, "0002-20260102T00-00Z.jsonl"), backdate, backdate)
	_ = os.Chtimes(filepath.Join(dir, "0003-20260103T00-00Z.jsonl"), backdate, backdate)

	if err := EnforceRetention(dir, 100, 365, WithCompression(), WithQuietWindow(0)); err != nil {
		t.Fatal(err)
	}

	// 0002 should get compressed, 0003 stays plain (newest)
	if _, err := os.Stat(filepath.Join(dir, "0002-20260102T00-00Z.jsonl.gz")); err != nil {
		t.Error("0002 should be compressed")
	}
	if _, err := os.Stat(filepath.Join(dir, "0003-20260103T00-00Z.jsonl")); err != nil {
		t.Error("0003 (newest) should stay uncompressed")
	}
}

func TestEnforceRetention_CountDeletesOldestFirst(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()

	for i := 1; i <= 10; i++ {
		name := filepath.Join(dir, fmt.Sprintf("%04d-20260101T00-00Z.jsonl", i))
		_ = os.WriteFile(name, []byte("{}"), 0644)
	}

	if err := EnforceRetention(dir, 3, 365, WithQuietWindow(0)); err != nil {
		t.Fatal(err)
	}

	remaining := countLogFiles(dir)
	if remaining != 3 {
		t.Errorf("expected 3 files, got %d", remaining)
	}

	// Verify the 3 newest remain
	for i := 8; i <= 10; i++ {
		name := filepath.Join(dir, fmt.Sprintf("%04d-20260101T00-00Z.jsonl", i))
		if _, err := os.Stat(name); err != nil {
			t.Errorf("expected file %d to remain", i)
		}
	}
}

// ═══════════════════════════════════════════════════════════════════════════
// StartIteration: error path
// ═══════════════════════════════════════════════════════════════════════════

func TestStartIteration_ErrorOnReadOnlyDir(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("chmod restrictions have no effect on Windows")
	}
	t.Parallel()
	dir := t.TempDir()
	logger, err := NewLogger(dir)
	if err != nil {
		t.Fatal(err)
	}
	defer logger.Close()

	// Make directory read-only
	_ = os.Chmod(dir, 0555)
	defer func() { _ = os.Chmod(dir, 0755) }()

	err = logger.StartIteration()
	if err == nil {
		t.Error("expected error creating log file in read-only dir")
	}
}

// ═══════════════════════════════════════════════════════════════════════════
// WatchForNewFiles: additional scenarios
// ═══════════════════════════════════════════════════════════════════════════

func TestWatchForNewFiles_ImmediateNewFile(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	currentPath := filepath.Join(dir, "0001-20260101T00-00Z.jsonl")
	_ = os.WriteFile(currentPath, []byte("{}"), 0644)

	// New file already exists before polling starts
	newPath := filepath.Join(dir, "0002-20260102T00-00Z.jsonl")
	_ = os.WriteFile(newPath, []byte("{}"), 0644)

	done := make(chan struct{})
	result := WatchForNewFiles(dir, currentPath, done, 10*time.Millisecond)
	if !strings.Contains(result, "0002") {
		t.Errorf("expected to detect existing new file, got %q", result)
	}
}

func TestWatchForNewFiles_DoneBeforePoll(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	currentPath := filepath.Join(dir, "0001-20260101T00-00Z.jsonl")
	_ = os.WriteFile(currentPath, []byte("{}"), 0644)

	done := make(chan struct{})
	close(done)

	result := WatchForNewFiles(dir, currentPath, done, 10*time.Millisecond)
	if result != "" {
		t.Errorf("expected empty string on immediate done, got %q", result)
	}
}

// ═══════════════════════════════════════════════════════════════════════════
// EnforceRetention: compression with read-only target (non-fatal)
// ═══════════════════════════════════════════════════════════════════════════

func TestEnforceRetention_CompressionErrorNonFatal(t *testing.T) {
	t.Parallel()
	if runtime.GOOS == "windows" {
		t.Skip("chmod 0000 has no effect on Windows")
	}
	dir := t.TempDir()

	content := `{"level":"info"}` + "\n"
	_ = os.WriteFile(filepath.Join(dir, "0001-20260101T00-00Z.jsonl"), []byte(content), 0644)
	_ = os.WriteFile(filepath.Join(dir, "0002-20260102T00-00Z.jsonl"), []byte(content), 0644)

	// Make the older file unreadable so compression fails (non-fatal)
	_ = os.Chmod(filepath.Join(dir, "0001-20260101T00-00Z.jsonl"), 0000)
	defer func() { _ = os.Chmod(filepath.Join(dir, "0001-20260101T00-00Z.jsonl"), 0644) }()

	// Should not error. Compression failures are non-fatal
	if err := EnforceRetention(dir, 100, 365, WithCompression(), WithQuietWindow(0)); err != nil {
		t.Fatal(err)
	}
}

// ═══════════════════════════════════════════════════════════════════════════
// compressFile: error during gzip write and close
// ═══════════════════════════════════════════════════════════════════════════

func TestCompressFile_DestCannotBeCreated(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	src := filepath.Join(dir, "test.jsonl")
	_ = os.WriteFile(src, []byte("data\n"), 0644)

	// Create a directory named test.jsonl.gz to block creation
	_ = os.MkdirAll(src+".gz", 0755)

	err := compressFile(src)
	if err == nil {
		t.Error("expected error when .gz destination is a directory")
	}
}

func TestCompressFile_OriginalRemovedOnSuccess(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	src := filepath.Join(dir, "remove-me.jsonl")
	content := "line1\nline2\nline3\n"
	_ = os.WriteFile(src, []byte(content), 0644)

	if err := compressFile(src); err != nil {
		t.Fatal(err)
	}

	// Original must be gone
	if _, err := os.Stat(src); !os.IsNotExist(err) {
		t.Error("original file should be removed after successful compression")
	}

	// Compressed file should be readable
	f, err := os.Open(src + ".gz")
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = f.Close() }()

	gz, err := gzip.NewReader(f)
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = gz.Close() }()

	data, _ := io.ReadAll(gz)
	if string(data) != content {
		t.Errorf("decompressed mismatch: got %q", data)
	}
}

// ═══════════════════════════════════════════════════════════════════════════
// EnforceRetention: retention with only gz files
// ═══════════════════════════════════════════════════════════════════════════

func TestEnforceRetention_OnlyGzFiles_NoCompression(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()

	for i := 1; i <= 3; i++ {
		name := filepath.Join(dir, fmt.Sprintf("%04d-20260101T00-00Z.jsonl.gz", i))
		_ = os.WriteFile(name, []byte("compressed"), 0644)
	}

	// No uncompressed files. Compression pass should be a no-op
	if err := EnforceRetention(dir, 100, 365, WithCompression(), WithQuietWindow(0)); err != nil {
		t.Fatal(err)
	}

	if count := countLogFiles(dir); count != 3 {
		t.Errorf("expected 3 files, got %d", count)
	}
}

func TestCompressFile_LargeFileCompression(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	src := filepath.Join(dir, "large.jsonl")
	// Write 10K lines to exercise io.Copy path thoroughly
	var content strings.Builder
	for i := 0; i < 10000; i++ {
		fmt.Fprintf(&content, `{"iteration":%d,"level":"info","message":"test log line with some padding for realism"}`, i)
		content.WriteString("\n")
	}
	_ = os.WriteFile(src, []byte(content.String()), 0644)

	if err := compressFile(src); err != nil {
		t.Fatal(err)
	}

	// Verify compressed file exists and is valid
	f, err := os.Open(src + ".gz")
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = f.Close() }()

	gz, err := gzip.NewReader(f)
	if err != nil {
		t.Fatal(err)
	}
	data, err := io.ReadAll(gz)
	_ = gz.Close()
	if err != nil {
		t.Fatal(err)
	}
	if len(data) != len(content.String()) {
		t.Errorf("expected %d bytes, got %d", len(content.String()), len(data))
	}
}

func TestEnforceRetention_NoUncompressedFiles_SkipsCompression(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()

	// Only .gz files. Compression step should be a no-op
	_ = os.WriteFile(filepath.Join(dir, "0001-20260101T00-00Z.jsonl.gz"), []byte("gz1"), 0644)
	_ = os.WriteFile(filepath.Join(dir, "0002-20260102T00-00Z.jsonl.gz"), []byte("gz2"), 0644)

	if err := EnforceRetention(dir, 100, 365, WithCompression(), WithQuietWindow(0)); err != nil {
		t.Fatal(err)
	}

	entries, _ := os.ReadDir(dir)
	if len(entries) != 2 {
		t.Errorf("expected 2 files, got %d", len(entries))
	}
}

func TestEnforceRetention_SingleUncompressed_NotCompressed(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()

	// Single uncompressed file should NOT be compressed (might be active)
	_ = os.WriteFile(filepath.Join(dir, "0001-20260101T00-00Z.jsonl"), []byte("{}"), 0644)

	if err := EnforceRetention(dir, 100, 365, WithCompression(), WithQuietWindow(0)); err != nil {
		t.Fatal(err)
	}

	entries, _ := os.ReadDir(dir)
	for _, e := range entries {
		if strings.HasSuffix(e.Name(), ".jsonl.gz") {
			t.Error("single file should not be compressed")
		}
	}
}

func TestEnforceRetention_AgeAndCountCombined(t *testing.T) {
	frozen := time.Date(2026, 3, 14, 12, 0, 0, 0, time.UTC)
	withFrozenClock(t, frozen)
	dir := t.TempDir()

	// Create 5 files: 2 old, 3 recent
	for i := 1; i <= 5; i++ {
		name := filepath.Join(dir, fmt.Sprintf("%04d-20260101T00-00Z.jsonl", i))
		_ = os.WriteFile(name, []byte("{}"), 0644)
		if i <= 2 {
			old := frozen.AddDate(0, 0, -60)
			_ = os.Chtimes(name, old, old)
		}
	}

	// maxFiles=10 (won't trigger), maxAgeDays=30 (removes 2 old)
	if err := EnforceRetention(dir, 10, 30, WithQuietWindow(0)); err != nil {
		t.Fatal(err)
	}

	if count := countLogFiles(dir); count != 3 {
		t.Errorf("expected 3 files after age cleanup, got %d", count)
	}
}

// ── StartIteration: previous file leak prevention ──────────────

func TestStartIteration_ClosePreviousBeforeNew(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	logger, err := NewLogger(dir)
	if err != nil {
		t.Fatal(err)
	}

	defer logger.Close()

	// Start 3 iterations rapidly. Should close each previous
	for i := 0; i < 3; i++ {
		if err := logger.StartIteration(); err != nil {
			t.Fatalf("iteration %d: %v", i, err)
		}
		_ = logger.Log(map[string]any{"iter": i})
	}
	logger.Close()

	// Verify 3 separate files
	entries, _ := os.ReadDir(dir)
	logCount := 0
	for _, e := range entries {
		if IsLogFile(e.Name()) {
			logCount++
		}
	}
	if logCount != 3 {
		t.Errorf("expected 3 log files, got %d", logCount)
	}
}

// ── compressFile: simulate errors via special files ──────────────

func TestEnforceRetention_SymlinkInfoError(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()

	// Create a normal file and a broken symlink
	_ = os.WriteFile(filepath.Join(dir, "0001-20260101T00-00Z.jsonl"), []byte("{}"), 0644)

	// Create a symlink to a nonexistent target. Os.DirEntry.Info() will error
	brokenTarget := filepath.Join(dir, "nonexistent_target.jsonl")
	brokenLink := filepath.Join(dir, "0002-20260102T00-00Z.jsonl")
	_ = os.Symlink(brokenTarget, brokenLink)

	// EnforceRetention should handle Info() errors gracefully
	if err := EnforceRetention(dir, 100, 1, WithQuietWindow(0)); err != nil {
		t.Fatal(err)
	}
}

func TestWatchForNewFiles_LatestLogFileError(t *testing.T) {
	t.Parallel()
	// Use an empty dir where LatestLogFile returns error
	dir := t.TempDir()
	currentPath := filepath.Join(dir, "current.jsonl")

	done := make(chan struct{})

	// Write a new file after a brief delay
	go func() {
		time.Sleep(30 * time.Millisecond)
		_ = os.WriteFile(filepath.Join(dir, "0001-20260101T00-00Z.jsonl"), []byte("{}"), 0644)
	}()

	result := WatchForNewFiles(dir, currentPath, done, 10*time.Millisecond)
	if result == "" {
		t.Error("expected to eventually detect a new file")
	}
}

func TestCompressFile_SourceOpenButDstDirGone(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("chmod restrictions have no effect on Windows")
	}
	t.Parallel()
	// Test that compressFile handles errors gracefully when the .gz
	// destination cannot be written (directory removed after Open).
	dir := t.TempDir()
	subDir := filepath.Join(dir, "sub")
	_ = os.MkdirAll(subDir, 0755)
	src := filepath.Join(subDir, "test.jsonl")
	_ = os.WriteFile(src, []byte("data\n"), 0644)

	// Make sub dir read-only so .gz can't be created
	_ = os.Chmod(subDir, 0555)
	defer func() { _ = os.Chmod(subDir, 0755) }()

	err := compressFile(src)
	if err == nil {
		t.Error("expected error when .gz cannot be created")
	}
}

// ── Log: write error path ──────────────────────────────────────

func TestLog_WriteError(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	logger, err := NewLogger(dir)
	if err != nil {
		t.Fatal(err)
	}

	_ = logger.StartIteration()
	// Close the underlying file to force a write error
	_ = logger.file.Close()

	err = logger.Log(map[string]any{"msg": "should fail"})
	if err == nil {
		t.Error("expected write error")
	}
	// Reset to prevent double-close
	logger.file = nil
}
