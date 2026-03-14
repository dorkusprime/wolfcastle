package project

import (
	"os"
	"path/filepath"
	"testing"
)

// ── Scaffold error paths ──────────────────────────────────────────

func TestScaffold_DirCreationFailure(t *testing.T) {
	t.Parallel()
	// Place a file where a directory is expected to force MkdirAll to fail
	tmp := t.TempDir()
	blocker := filepath.Join(tmp, ".wolfcastle")
	_ = os.WriteFile(blocker, []byte("block"), 0644)

	err := Scaffold(filepath.Join(blocker, "nested"))
	if err == nil {
		t.Error("expected error when directory creation fails")
	}
}

func TestScaffold_GitignoreWriteFailure(t *testing.T) {
	t.Parallel()
	// Create a read-only dir so file writes fail
	dir := filepath.Join(t.TempDir(), ".wolfcastle")
	if err := os.MkdirAll(dir, 0755); err != nil {
		t.Fatal(err)
	}

	// Create the subdirectories so MkdirAll succeeds
	for _, d := range []string{"base/prompts", "base/rules", "base/audits", "custom", "local", "archive", "docs/decisions", "docs/specs", "logs"} {
		_ = os.MkdirAll(filepath.Join(dir, d), 0755)
	}

	// Make the directory read-only so WriteFile fails
	_ = os.Chmod(dir, 0555)
	defer func() { _ = os.Chmod(dir, 0755) }()

	err := Scaffold(dir)
	if err == nil {
		t.Error("expected error when writing .gitignore to read-only dir")
	}
}

func TestScaffold_ConfigWriteFailure(t *testing.T) {
	t.Parallel()
	tmp := t.TempDir()
	dir := filepath.Join(tmp, ".wolfcastle")

	// Create all dirs but put a directory where config.json should go
	if err := os.MkdirAll(dir, 0755); err != nil {
		t.Fatal(err)
	}
	for _, d := range []string{"base/prompts", "base/rules", "base/audits", "custom", "local", "archive", "docs/decisions", "docs/specs", "logs"} {
		_ = os.MkdirAll(filepath.Join(dir, d), 0755)
	}

	// Create a directory named config.json to block the file write
	_ = os.MkdirAll(filepath.Join(dir, "config.json"), 0755)

	err := Scaffold(dir)
	if err == nil {
		t.Error("expected error when config.json cannot be written")
	}
}

func TestScaffold_LocalConfigWriteFailure(t *testing.T) {
	t.Parallel()
	tmp := t.TempDir()
	dir := filepath.Join(tmp, ".wolfcastle")

	if err := os.MkdirAll(dir, 0755); err != nil {
		t.Fatal(err)
	}
	for _, d := range []string{"base/prompts", "base/rules", "base/audits", "custom", "local", "archive", "docs/decisions", "docs/specs", "logs"} {
		_ = os.MkdirAll(filepath.Join(dir, d), 0755)
	}

	// Create a directory named config.local.json to block the file write
	_ = os.MkdirAll(filepath.Join(dir, "config.local.json"), 0755)

	err := Scaffold(dir)
	if err == nil {
		t.Error("expected error when config.local.json cannot be written")
	}
}

func TestScaffold_NamespaceDirCreationFailure(t *testing.T) {
	t.Parallel()
	tmp := t.TempDir()
	dir := filepath.Join(tmp, ".wolfcastle")

	if err := os.MkdirAll(dir, 0755); err != nil {
		t.Fatal(err)
	}
	for _, d := range []string{"base/prompts", "base/rules", "base/audits", "custom", "local", "archive", "docs/decisions", "docs/specs", "logs"} {
		_ = os.MkdirAll(filepath.Join(dir, d), 0755)
	}

	// Create a file named "projects" to block the namespace dir creation
	_ = os.WriteFile(filepath.Join(dir, "projects"), []byte("block"), 0644)

	err := Scaffold(dir)
	if err == nil {
		t.Error("expected error when namespace directory cannot be created")
	}
}

func TestScaffold_RootIndexWriteFailure(t *testing.T) {
	t.Parallel()
	tmp := t.TempDir()
	dir := filepath.Join(tmp, ".wolfcastle")

	// Do initial scaffold to get the identity, then re-scaffold with blocking
	if err := Scaffold(dir); err != nil {
		t.Fatal(err)
	}

	// Find the namespace directory and make it read-only
	entries, err := os.ReadDir(filepath.Join(dir, "projects"))
	if err != nil {
		t.Fatal(err)
	}
	nsDir := filepath.Join(dir, "projects", entries[0].Name())

	// Remove state.json then make dir read-only so the write fails
	_ = os.Remove(filepath.Join(nsDir, "state.json"))
	_ = os.Chmod(nsDir, 0555)
	defer func() { _ = os.Chmod(nsDir, 0755) }()

	// Remove all base/ to force WriteBasePrompts path
	_ = os.RemoveAll(filepath.Join(dir, "base"))

	// Try rescaffold; the namespace dir is read-only
	// For Scaffold: we need a fresh attempt
	// Use a path where projects dir is a file
	dir2 := filepath.Join(tmp, ".wolfcastle2")
	if err := os.MkdirAll(dir2, 0755); err != nil {
		t.Fatal(err)
	}
	for _, d := range []string{"base/prompts", "base/rules", "base/audits", "custom", "local", "archive", "docs/decisions", "docs/specs", "logs"} {
		_ = os.MkdirAll(filepath.Join(dir2, d), 0755)
	}
	// Block the state.json write: create a dir where state.json should be
	identity := detectIdentity()
	ns := identity["user"].(string) + "-" + identity["machine"].(string)
	stateDir := filepath.Join(dir2, "projects", ns, "state.json")
	_ = os.MkdirAll(stateDir, 0755)

	err = Scaffold(dir2)
	if err == nil {
		t.Error("expected error when root index cannot be written")
	}
}

// ── ReScaffold error paths ────────────────────────────────────────

func TestReScaffold_RemoveBaseFailure(t *testing.T) {
	t.Parallel()
	// If the base dir doesn't exist, RemoveAll should still succeed (no error)
	tmp := t.TempDir()
	dir := filepath.Join(tmp, ".wolfcastle")
	if err := Scaffold(dir); err != nil {
		t.Fatal(err)
	}

	// Make sure base exists, then remove it — should be fine
	_ = os.RemoveAll(filepath.Join(dir, "base"))
	if err := ReScaffold(dir); err != nil {
		t.Fatal("ReScaffold should succeed even if base/ doesn't exist:", err)
	}
}

func TestReScaffold_RemoveBaseError(t *testing.T) {
	t.Parallel()
	// If we pass a completely nonexistent dir, RemoveAll won't error but
	// MkdirAll will fail because the parent doesn't exist
	err := ReScaffold("/nonexistent/wolfcastle/dir")
	if err == nil {
		t.Error("expected error when wolfcastle dir does not exist")
	}
}

func TestReScaffold_LocalConfigWriteFailure(t *testing.T) {
	t.Parallel()
	tmp := t.TempDir()
	dir := filepath.Join(tmp, ".wolfcastle")
	if err := Scaffold(dir); err != nil {
		t.Fatal(err)
	}

	// Replace config.local.json with a directory to block write
	localPath := filepath.Join(dir, "config.local.json")
	_ = os.Remove(localPath)
	_ = os.MkdirAll(localPath, 0755)

	err := ReScaffold(dir)
	if err == nil {
		t.Error("expected error when config.local.json cannot be written")
	}
}

// ── WriteBasePrompts error paths ──────────────────────────────────

func TestWriteBasePrompts_MkdirAllFailure(t *testing.T) {
	t.Parallel()
	tmp := t.TempDir()
	dir := filepath.Join(tmp, ".wolfcastle")

	// Create base as a file to block MkdirAll inside
	_ = os.MkdirAll(dir, 0755)
	_ = os.WriteFile(filepath.Join(dir, "base"), []byte("block"), 0644)

	err := WriteBasePrompts(dir)
	if err == nil {
		t.Error("expected error when base directory creation fails")
	}
}

func TestWriteBasePrompts_WriteFileFailure(t *testing.T) {
	t.Parallel()
	tmp := t.TempDir()
	dir := filepath.Join(tmp, ".wolfcastle")

	// Create base/prompts but make it read-only
	promptsDir := filepath.Join(dir, "base", "prompts")
	_ = os.MkdirAll(promptsDir, 0755)
	_ = os.MkdirAll(filepath.Join(dir, "base", "rules"), 0755)
	_ = os.MkdirAll(filepath.Join(dir, "base", "audits"), 0755)

	_ = os.Chmod(promptsDir, 0555)
	defer func() { _ = os.Chmod(promptsDir, 0755) }()

	err := WriteBasePrompts(dir)
	if err == nil {
		t.Error("expected error when prompt files cannot be written")
	}
}

func TestReScaffold_LocalConfigMarshalWriteFailure(t *testing.T) {
	t.Parallel()
	tmp := t.TempDir()
	dir := filepath.Join(tmp, ".wolfcastle")
	if err := Scaffold(dir); err != nil {
		t.Fatal(err)
	}

	// Create a directory where config.local.json should be written
	// to block the write
	localPath := filepath.Join(dir, "config.local.json")
	_ = os.Remove(localPath)
	_ = os.MkdirAll(localPath, 0755)

	err := ReScaffold(dir)
	if err == nil {
		t.Error("expected error when config.local.json write is blocked")
	}
}

func TestScaffold_BasePromptWriteFailure(t *testing.T) {
	t.Parallel()
	tmp := t.TempDir()
	dir := filepath.Join(tmp, ".wolfcastle")

	// Create all expected directories
	for _, d := range []string{"base/prompts", "base/rules", "base/audits", "custom", "local", "archive", "docs/decisions", "docs/specs", "logs"} {
		_ = os.MkdirAll(filepath.Join(dir, d), 0755)
	}
	// Make base/prompts read-only so WriteBasePrompts fails inside Scaffold
	_ = os.Chmod(filepath.Join(dir, "base", "prompts"), 0555)
	defer func() { _ = os.Chmod(filepath.Join(dir, "base", "prompts"), 0755) }()

	err := Scaffold(dir)
	if err == nil {
		t.Error("expected error when base prompt files cannot be written")
	}
}
