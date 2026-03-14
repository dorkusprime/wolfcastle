package cmd

import (
	"os"
	"path/filepath"
	"runtime"
	"testing"
)

func TestCopyDir(t *testing.T) {
	src := filepath.Join(t.TempDir(), "src")
	dst := filepath.Join(t.TempDir(), "dst")

	os.MkdirAll(src, 0755)
	os.WriteFile(filepath.Join(src, "file1.txt"), []byte("hello"), 0644)

	sub := filepath.Join(src, "sub")
	os.MkdirAll(sub, 0755)
	os.WriteFile(filepath.Join(sub, "file2.txt"), []byte("world"), 0644)

	if err := copyDir(src, dst); err != nil {
		t.Fatalf("copyDir failed: %v", err)
	}

	// Verify files copied
	if _, err := os.Stat(filepath.Join(dst, "file1.txt")); os.IsNotExist(err) {
		t.Error("file1.txt not copied")
	}
	if _, err := os.Stat(filepath.Join(dst, "sub", "file2.txt")); os.IsNotExist(err) {
		t.Error("sub/file2.txt not copied")
	}

	data, _ := os.ReadFile(filepath.Join(dst, "file1.txt"))
	if string(data) != "hello" {
		t.Errorf("unexpected content: %s", string(data))
	}
}

func TestCanSymlink(t *testing.T) {
	result := canSymlink()
	if runtime.GOOS == "windows" {
		if result {
			t.Error("canSymlink should return false on Windows")
		}
	} else {
		if !result {
			t.Error("canSymlink should return true on non-Windows")
		}
	}
}

func TestEnsureSkillSource(t *testing.T) {
	dir := filepath.Join(t.TempDir(), "skills")

	if err := ensureSkillSource(dir); err != nil {
		t.Fatalf("ensureSkillSource failed: %v", err)
	}

	// Should have created wolfcastle.md
	skillFile := filepath.Join(dir, "wolfcastle.md")
	if _, err := os.Stat(skillFile); os.IsNotExist(err) {
		t.Error("wolfcastle.md should be created")
	}

	// Calling again should not overwrite
	data1, _ := os.ReadFile(skillFile)
	ensureSkillSource(dir)
	data2, _ := os.ReadFile(skillFile)
	if string(data1) != string(data2) {
		t.Error("calling ensureSkillSource twice should not change file")
	}
}
