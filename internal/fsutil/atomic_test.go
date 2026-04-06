package fsutil

import (
	"os"
	"path/filepath"
	"runtime"
	"testing"
)

func TestAtomicWriteFile_Success(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	path := filepath.Join(dir, "test.json")

	if err := AtomicWriteFile(path, []byte(`{"ok":true}`)); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("reading written file: %v", err)
	}
	if string(data) != `{"ok":true}` {
		t.Errorf("expected {\"ok\":true}, got %s", data)
	}
}

func TestAtomicWriteFile_CreatesParentDir(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	path := filepath.Join(dir, "nested", "deep", "test.json")

	if err := AtomicWriteFile(path, []byte("hello")); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("reading written file: %v", err)
	}
	if string(data) != "hello" {
		t.Errorf("expected hello, got %s", data)
	}
}

func TestAtomicWriteFile_Overwrite(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	path := filepath.Join(dir, "test.json")

	if err := AtomicWriteFile(path, []byte("first")); err != nil {
		t.Fatalf("first write: %v", err)
	}
	if err := AtomicWriteFile(path, []byte("second")); err != nil {
		t.Fatalf("second write: %v", err)
	}

	data, _ := os.ReadFile(path)
	if string(data) != "second" {
		t.Errorf("expected second, got %s", data)
	}
}

func TestAtomicWriteFile_NoTempFileLeftOnSuccess(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	path := filepath.Join(dir, "test.json")

	if err := AtomicWriteFile(path, []byte("data")); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	entries, _ := os.ReadDir(dir)
	for _, e := range entries {
		if e.Name() != "test.json" {
			t.Errorf("unexpected file left behind: %s", e.Name())
		}
	}
}

func TestAtomicWriteFile_MkdirAllError(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("chmod restrictions have no effect on Windows")
	}
	t.Parallel()
	dir := t.TempDir()
	_ = os.Chmod(dir, 0o555)
	t.Cleanup(func() { _ = os.Chmod(dir, 0o755) })

	path := filepath.Join(dir, "subdir", "test.json")
	err := AtomicWriteFile(path, []byte("data"))
	if err == nil {
		t.Error("expected error when parent dir is read-only")
	}
}

func TestAtomicWriteFile_CreateTempError(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("chmod restrictions have no effect on Windows")
	}
	t.Parallel()
	dir := t.TempDir()
	// Directory exists but is read-only, so CreateTemp fails.
	_ = os.Chmod(dir, 0o555)
	t.Cleanup(func() { _ = os.Chmod(dir, 0o755) })

	path := filepath.Join(dir, "test.json")
	err := AtomicWriteFile(path, []byte("data"))
	if err == nil {
		t.Error("expected error when dir is read-only")
	}
}

func TestAtomicWriteFile_RenameError(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("chmod restrictions have no effect on Windows")
	}
	t.Parallel()

	// Create a directory where we can write temp files but can't
	// rename over the target (target is in a read-only parent).
	srcDir := t.TempDir()
	targetDir := filepath.Join(srcDir, "readonly")
	_ = os.MkdirAll(targetDir, 0o755)

	// Write a file, then make the directory read-only so rename fails
	// when trying to replace a file in that directory.
	targetPath := filepath.Join(targetDir, "test.json")
	_ = os.WriteFile(targetPath, []byte("old"), 0o644)
	_ = os.Chmod(targetDir, 0o555)
	t.Cleanup(func() { _ = os.Chmod(targetDir, 0o755) })

	err := AtomicWriteFile(targetPath, []byte("new"))
	if err == nil {
		t.Error("expected error when target dir is read-only")
	}

	// Original file should be untouched.
	data, _ := os.ReadFile(targetPath)
	if string(data) != "old" {
		t.Errorf("original file should be unchanged, got %s", data)
	}
}

func TestAtomicWriteFile_EmptyData(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	path := filepath.Join(dir, "empty.json")

	if err := AtomicWriteFile(path, []byte{}); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	data, _ := os.ReadFile(path)
	if len(data) != 0 {
		t.Errorf("expected empty file, got %d bytes", len(data))
	}
}
