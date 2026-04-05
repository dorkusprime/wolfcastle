package tierfs

import (
	"os"
	"path/filepath"
	"runtime"
	"testing"
)

func TestAtomicWriteFile_Success(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	path := filepath.Join(dir, "output.txt")

	if err := atomicWriteFile(path, []byte("hello world")); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	got, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if string(got) != "hello world" {
		t.Errorf("got %q, want %q", got, "hello world")
	}
}

func TestAtomicWriteFile_CreateTempFails(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("chmod not reliable on Windows")
	}
	t.Parallel()

	dir := t.TempDir()
	// Make dir read-only so CreateTemp fails.
	_ = os.Chmod(dir, 0555)
	t.Cleanup(func() { _ = os.Chmod(dir, 0755) })

	err := atomicWriteFile(filepath.Join(dir, "file.txt"), []byte("data"))
	if err == nil {
		t.Error("expected error when directory is read-only")
	}
}

func TestAtomicWriteFile_RenameTargetIsDir(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	target := filepath.Join(dir, "file.txt")
	// Place a directory where the destination file should be.
	if err := os.MkdirAll(target, 0755); err != nil {
		t.Fatal(err)
	}

	err := atomicWriteFile(target, []byte("data"))
	if err == nil {
		t.Error("expected rename error when target is a directory")
	}
}

func TestAtomicWriteFile_OverwriteExisting(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	path := filepath.Join(dir, "file.txt")

	// Write initial content.
	if err := atomicWriteFile(path, []byte("v1")); err != nil {
		t.Fatal(err)
	}
	// Overwrite with new content.
	if err := atomicWriteFile(path, []byte("v2")); err != nil {
		t.Fatal(err)
	}

	got, _ := os.ReadFile(path)
	if string(got) != "v2" {
		t.Errorf("expected v2, got %q", got)
	}
}
