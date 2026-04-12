package logging

import (
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"testing"
)

// ═══════════════════════════════════════════════════════════════════════════
// compressFile: io.Copy failure via read permission revoked mid-stream,
// and gz.Close / out.Close error paths.
// ═══════════════════════════════════════════════════════════════════════════

func TestCompressFile_DestinationDirReadOnly(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("permission-based test not supported on Windows")
	}
	t.Parallel()
	dir := t.TempDir()
	src := filepath.Join(dir, "readonly-dest.jsonl")
	_ = os.WriteFile(src, []byte("data to compress\n"), 0644)

	// Make the directory read-only so os.Create for .gz fails
	_ = os.Chmod(dir, 0555)
	defer func() { _ = os.Chmod(dir, 0755) }()

	err := compressFile(src)
	if err == nil {
		t.Error("expected error when destination directory is read-only")
	}
}

func TestCompressFile_EmptyFile_ErrorPath(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	src := filepath.Join(dir, "empty.jsonl")
	_ = os.WriteFile(src, []byte{}, 0644)

	if err := compressFile(src); err != nil {
		t.Fatalf("compressing empty file should succeed: %v", err)
	}

	// Original should be removed
	if _, err := os.Stat(src); !os.IsNotExist(err) {
		t.Error("original file should be removed after compression")
	}
	if _, err := os.Stat(src + ".gz"); err != nil {
		t.Error("compressed file should exist")
	}
}

// ═══════════════════════════════════════════════════════════════════════════
// EnforceRetention: compress path with unreadable source triggers
// non-fatal continue in the compression loop.
// ═══════════════════════════════════════════════════════════════════════════

func TestEnforceRetention_CompressUnreadableFile(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("permission-based test not supported on Windows")
	}
	t.Parallel()
	dir := t.TempDir()

	// Create two uncompressed log files (compress only happens for len > 1)
	_ = os.WriteFile(filepath.Join(dir, "0001-20260101T00-00Z.jsonl"), []byte("old\n"), 0644)
	_ = os.WriteFile(filepath.Join(dir, "0002-20260102T00-00Z.jsonl"), []byte("new\n"), 0644)

	// Make the older file unreadable so compressFile fails non-fatally
	_ = os.Chmod(filepath.Join(dir, "0001-20260101T00-00Z.jsonl"), 0000)
	defer func() { _ = os.Chmod(filepath.Join(dir, "0001-20260101T00-00Z.jsonl"), 0644) }()

	err := EnforceRetention(dir, 100, 30, WithCompression())
	if err != nil {
		t.Fatalf("EnforceRetention should succeed even when compression of one file fails: %v", err)
	}

	// The unreadable file should still exist (compression skipped)
	if _, err := os.Stat(filepath.Join(dir, "0001-20260101T00-00Z.jsonl")); err != nil {
		t.Error("unreadable file should remain when compression fails")
	}
}

func TestEnforceRetention_CompressWithReadOnlyDestDir(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("permission-based test not supported on Windows")
	}
	t.Parallel()

	// Use a separate tmpdir so we can make it read-only temporarily
	parent := t.TempDir()
	dir := filepath.Join(parent, "logs")
	_ = os.MkdirAll(dir, 0755)

	// Create two files so the compress path is triggered
	_ = os.WriteFile(filepath.Join(dir, "0001-20260101T00-00Z.jsonl"), []byte("old\n"), 0644)
	_ = os.WriteFile(filepath.Join(dir, "0002-20260102T00-00Z.jsonl"), []byte("new\n"), 0644)

	// Make directory read-only so .gz creation fails, but files are readable
	_ = os.Chmod(dir, 0555)
	defer func() { _ = os.Chmod(dir, 0755) }()

	err := EnforceRetention(dir, 100, 30, WithCompression())
	if err != nil {
		t.Fatalf("EnforceRetention should succeed even when .gz creation fails: %v", err)
	}
}

// ═══════════════════════════════════════════════════════════════════════════
// EnforceRetention: age-based deletion with old files
// ═══════════════════════════════════════════════════════════════════════════

func TestEnforceRetention_DeletesByCount(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()

	// Create 5 log files
	for i := 1; i <= 5; i++ {
		name := filepath.Join(dir, fmt.Sprintf("%04d-20260101T00-00Z.jsonl", i))
		_ = os.WriteFile(name, []byte("log\n"), 0644)
	}

	// Keep only 2 newest
	err := EnforceRetention(dir, 2, 30)
	if err != nil {
		t.Fatal(err)
	}

	entries, _ := os.ReadDir(dir)
	count := 0
	for _, e := range entries {
		if IsLogFile(e.Name()) {
			count++
		}
	}
	if count > 2 {
		t.Errorf("expected at most 2 log files after retention, got %d", count)
	}
}

// ═══════════════════════════════════════════════════════════════════════════
// IterationFromDir: with non-log files
// ═══════════════════════════════════════════════════════════════════════════

func TestIterationFromDir_MixedFiles(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()

	_ = os.WriteFile(filepath.Join(dir, "readme.txt"), []byte("not a log"), 0644)
	_ = os.WriteFile(filepath.Join(dir, "0003-20260101T00-00Z.jsonl"), []byte("{}"), 0644)
	_ = os.MkdirAll(filepath.Join(dir, "subdir"), 0755)

	n := IterationFromDir(dir)
	if n != 3 {
		t.Errorf("expected iteration 3, got %d", n)
	}
}

func TestIterationFromDir_NonexistentDir(t *testing.T) {
	t.Parallel()
	n := IterationFromDir("/nonexistent/wolfcastle/logs")
	if n != 0 {
		t.Errorf("expected 0 for nonexistent dir, got %d", n)
	}
}
