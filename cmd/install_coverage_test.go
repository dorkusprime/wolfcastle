package cmd

import (
	"os"
	"path/filepath"
	"runtime"
	"testing"
)

// ═══════════════════════════════════════════════════════════════════════════
// ensureSkillSource — additional coverage
// ═══════════════════════════════════════════════════════════════════════════

func TestEnsureSkillSource_FileAlreadyExistsWithDifferentContent(t *testing.T) {
	dir := filepath.Join(t.TempDir(), "skills")
	_ = os.MkdirAll(dir, 0755)

	// Write a custom skill file first
	skillFile := filepath.Join(dir, "wolfcastle.md")
	customContent := "# My Custom Skill\n\nDifferent content.\n"
	_ = os.WriteFile(skillFile, []byte(customContent), 0644)

	// ensureSkillSource should NOT overwrite existing file
	if err := ensureSkillSource(dir); err != nil {
		t.Fatalf("ensureSkillSource failed: %v", err)
	}

	data, err := os.ReadFile(skillFile)
	if err != nil {
		t.Fatal(err)
	}
	if string(data) != customContent {
		t.Error("ensureSkillSource should not overwrite existing file with different content")
	}
}

func TestEnsureSkillSource_FileAlreadyExistsEmpty(t *testing.T) {
	dir := filepath.Join(t.TempDir(), "skills")
	_ = os.MkdirAll(dir, 0755)

	// Write an empty skill file
	skillFile := filepath.Join(dir, "wolfcastle.md")
	_ = os.WriteFile(skillFile, []byte(""), 0644)

	// ensureSkillSource should NOT overwrite even an empty existing file
	if err := ensureSkillSource(dir); err != nil {
		t.Fatalf("ensureSkillSource failed: %v", err)
	}

	data, _ := os.ReadFile(skillFile)
	if string(data) != "" {
		t.Error("ensureSkillSource should not overwrite existing empty file")
	}
}

func TestEnsureSkillSource_DirectoryCreationFailure(t *testing.T) {
	// Use a path where the parent is a file, blocking MkdirAll
	dir := t.TempDir()
	blocker := filepath.Join(dir, "blocker")
	_ = os.WriteFile(blocker, []byte("x"), 0644)

	err := ensureSkillSource(filepath.Join(blocker, "skills"))
	if err == nil {
		t.Error("expected error when directory cannot be created")
	}
}

func TestEnsureSkillSource_WriteFailure(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("chmod restrictions have no effect on Windows")
	}
	// Create a read-only directory so the write fails
	dir := filepath.Join(t.TempDir(), "skills")
	_ = os.MkdirAll(dir, 0755)
	// Make directory read-only after creation
	_ = os.Chmod(dir, 0555)
	defer func() { _ = os.Chmod(dir, 0755) }()

	err := ensureSkillSource(dir)
	if err == nil {
		t.Error("expected error when skill file cannot be written")
	}
}
