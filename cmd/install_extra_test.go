package cmd

import (
	"os"
	"path/filepath"
	"runtime"
	"testing"
)

func TestCopyDir_UnreadableFileInSubdir(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("chmod 0000 has no effect on Windows")
	}
	src := filepath.Join(t.TempDir(), "src")
	dst := filepath.Join(t.TempDir(), "dst")

	sub := filepath.Join(src, "sub")
	_ = os.MkdirAll(sub, 0755)
	secret := filepath.Join(sub, "secret.txt")
	_ = os.WriteFile(secret, []byte("hidden"), 0644)
	_ = os.Chmod(secret, 0000)
	defer func() { _ = os.Chmod(secret, 0644) }()

	err := copyDir(src, dst)
	if err == nil {
		t.Error("expected error when source file in subdir is unreadable")
	}
}

func TestCopyDir_ReadOnlyDestination(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("chmod 0000 has no effect on Windows")
	}
	src := filepath.Join(t.TempDir(), "src")
	_ = os.MkdirAll(src, 0755)
	_ = os.WriteFile(filepath.Join(src, "file.txt"), []byte("data"), 0644)

	dst := filepath.Join(t.TempDir(), "dst")
	_ = os.MkdirAll(dst, 0755)
	_ = os.Chmod(dst, 0555)
	defer func() { _ = os.Chmod(dst, 0755) }()

	err := copyDir(src, dst)
	if err == nil {
		t.Error("expected error when destination is read-only")
	}
}
