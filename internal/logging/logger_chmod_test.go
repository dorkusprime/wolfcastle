package logging

import (
	"os"
	"path/filepath"
	"runtime"
	"testing"
)

// ── compressFile — permission-based error paths ─────────────────────

func TestCompressFile_UnreadableSource_Chmod(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("permission test not reliable on Windows")
	}
	t.Parallel()

	dir := t.TempDir()
	src := filepath.Join(dir, "secret.jsonl")
	_ = os.WriteFile(src, []byte("data\n"), 0644)

	_ = os.Chmod(src, 0000)
	t.Cleanup(func() { _ = os.Chmod(src, 0644) })

	err := compressFile(src)
	if err == nil {
		t.Error("expected error when source file is unreadable")
	}

	// No .gz should be left behind.
	if _, statErr := os.Stat(src + ".gz"); !os.IsNotExist(statErr) {
		t.Error("no .gz file should exist when source is unreadable")
	}
}

func TestCompressFile_UnwritableDestination_Chmod(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("permission test not reliable on Windows")
	}
	t.Parallel()

	dir := t.TempDir()
	src := filepath.Join(dir, "writable.jsonl")
	_ = os.WriteFile(src, []byte("data\n"), 0644)

	// Lock the directory so the .gz file cannot be created.
	_ = os.Chmod(dir, 0555)
	t.Cleanup(func() { _ = os.Chmod(dir, 0755) })

	err := compressFile(src)
	if err == nil {
		t.Error("expected error when destination directory is read-only")
	}
}
