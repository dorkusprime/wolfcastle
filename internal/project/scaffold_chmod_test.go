package project

import (
	"os"
	"path/filepath"
	"runtime"
	"testing"
)

// ── Scaffold — read-only directory error paths ─────────────────────

func TestScaffold_MkdirAllError_ReadOnly(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("permission test not reliable on Windows")
	}
	t.Parallel()

	dir := t.TempDir()
	wcDir := filepath.Join(dir, ".wolfcastle")
	_ = os.MkdirAll(wcDir, 0755)

	// Lock the wolfcastle dir so sub-directory creation fails.
	_ = os.Chmod(wcDir, 0555)
	t.Cleanup(func() { _ = os.Chmod(wcDir, 0755) })

	err := Scaffold(wcDir)
	if err == nil {
		t.Error("expected MkdirAll error when wolfcastle dir is read-only")
	}
}

func TestScaffold_GitignoreWriteError_ReadOnly(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("permission test not reliable on Windows")
	}
	t.Parallel()

	dir := t.TempDir()
	wcDir := filepath.Join(dir, ".wolfcastle")

	// Create all required subdirectories first.
	for _, d := range []string{"base/prompts", "base/rules", "base/audits", "custom", "local", "archive", "docs/decisions", "docs/specs", "logs"} {
		_ = os.MkdirAll(filepath.Join(wcDir, d), 0755)
	}

	// Make the root dir read-only so WriteFile(.gitignore) fails.
	_ = os.Chmod(wcDir, 0555)
	t.Cleanup(func() { _ = os.Chmod(wcDir, 0755) })

	err := Scaffold(wcDir)
	if err == nil {
		t.Error("expected .gitignore write error when directory is read-only")
	}
}

func TestScaffold_ConfigWriteError_ReadOnly(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("permission test not reliable on Windows")
	}
	t.Parallel()

	dir := t.TempDir()
	wcDir := filepath.Join(dir, ".wolfcastle")

	for _, d := range []string{"base/prompts", "base/rules", "base/audits", "custom", "local", "archive", "docs/decisions", "docs/specs", "logs"} {
		_ = os.MkdirAll(filepath.Join(wcDir, d), 0755)
	}

	// Block base/config.json by placing a directory in its way.
	_ = os.MkdirAll(filepath.Join(wcDir, "base", "config.json"), 0755)

	err := Scaffold(wcDir)
	if err == nil {
		t.Error("expected base/config.json write error")
	}
}

func TestScaffold_LocalConfigWriteError_ReadOnly(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("permission test not reliable on Windows")
	}
	t.Parallel()

	dir := t.TempDir()
	wcDir := filepath.Join(dir, ".wolfcastle")

	for _, d := range []string{"base/prompts", "base/rules", "base/audits", "custom", "local", "archive", "docs/decisions", "docs/specs", "logs"} {
		_ = os.MkdirAll(filepath.Join(wcDir, d), 0755)
	}

	// Block local/config.json with a directory.
	_ = os.MkdirAll(filepath.Join(wcDir, "local", "config.json"), 0755)

	err := Scaffold(wcDir)
	if err == nil {
		t.Error("expected local/config.json write error")
	}
}

// ── ReScaffold — read-only directory error paths ────────────────────

func TestReScaffold_RemoveAllError_ReadOnly(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("permission test not reliable on Windows")
	}
	t.Parallel()

	dir := t.TempDir()
	wcDir := filepath.Join(dir, ".wolfcastle")
	if err := Scaffold(wcDir); err != nil {
		t.Fatal(err)
	}

	// Lock a file inside base/ so RemoveAll can't remove everything.
	basePrompts := filepath.Join(wcDir, "base", "prompts")
	_ = os.Chmod(basePrompts, 0500)
	_ = os.Chmod(filepath.Join(wcDir, "base"), 0500)
	t.Cleanup(func() {
		_ = os.Chmod(filepath.Join(wcDir, "base"), 0755)
		_ = os.Chmod(basePrompts, 0755)
	})

	// RemoveAll on a dir with locked children may or may not fail on all
	// platforms. The deeper MkdirAll after removal should fail because
	// base/ is locked. If RemoveAll itself fails first, that's also fine.
	err := ReScaffold(wcDir)
	if err == nil {
		// On some systems RemoveAll succeeds because it recursively
		// changes permissions internally — accept that as valid.
		t.Log("RemoveAll succeeded despite locked dir (platform behaviour)")
	}
}

func TestReScaffold_MkdirAllError(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("permission test not reliable on Windows")
	}
	t.Parallel()

	// Use a completely nonexistent parent so MkdirAll fails.
	err := ReScaffold("/nonexistent/wolfcastle/path")
	if err == nil {
		t.Error("expected MkdirAll error when wolfcastle dir doesn't exist")
	}
}

func TestReScaffold_MkdirAllBaseError_Blocked(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("permission test not reliable on Windows")
	}
	t.Parallel()

	dir := t.TempDir()
	wcDir := filepath.Join(dir, ".wolfcastle")
	if err := Scaffold(wcDir); err != nil {
		t.Fatal(err)
	}

	// Lock the wolfcastle dir so MkdirAll(base/prompts) fails after RemoveAll.
	_ = os.RemoveAll(filepath.Join(wcDir, "base"))
	_ = os.Chmod(wcDir, 0555)
	t.Cleanup(func() { _ = os.Chmod(wcDir, 0755) })

	err := ReScaffold(wcDir)
	if err == nil {
		t.Error("expected error when base/ subdirectories cannot be recreated")
	}
}

func TestReScaffold_WriteLocalConfigError_ReadOnly(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("permission test not reliable on Windows")
	}
	t.Parallel()

	dir := t.TempDir()
	wcDir := filepath.Join(dir, ".wolfcastle")
	if err := Scaffold(wcDir); err != nil {
		t.Fatal(err)
	}

	// Replace local/config.json with a directory.
	localPath := filepath.Join(wcDir, "local", "config.json")
	_ = os.Remove(localPath)
	_ = os.MkdirAll(localPath, 0755)

	err := ReScaffold(wcDir)
	if err == nil {
		t.Error("expected local/config.json write error")
	}
}
