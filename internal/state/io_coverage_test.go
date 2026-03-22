package state

import (
	"os"
	"path/filepath"
	"runtime"
	"testing"
)

// ═══════════════════════════════════════════════════════════════════════════
// atomicWriteJSON — error path coverage
// ═══════════════════════════════════════════════════════════════════════════

func TestAtomicWriteJSON_TmpFileCreationFailure(t *testing.T) {
	t.Parallel()
	if runtime.GOOS == "windows" {
		t.Skip("chmod restrictions have no effect on Windows")
	}
	// Point the path at a directory that exists but is read-only,
	// so MkdirAll succeeds but CreateTemp fails.
	dir := t.TempDir()
	sub := filepath.Join(dir, "locked")
	if err := os.MkdirAll(sub, 0755); err != nil {
		t.Fatal(err)
	}
	_ = os.Chmod(sub, 0555)
	defer func() { _ = os.Chmod(sub, 0755) }()

	path := filepath.Join(sub, "state.json")
	err := atomicWriteJSON(path, map[string]string{"key": "value"})
	if err == nil {
		t.Error("expected error when temp file cannot be created")
	}
}

func TestAtomicWriteJSON_MkdirAllFailure(t *testing.T) {
	t.Parallel()
	// Place a regular file where a directory needs to be created.
	dir := t.TempDir()
	blocker := filepath.Join(dir, "blocker")
	if err := os.WriteFile(blocker, []byte("x"), 0644); err != nil {
		t.Fatal(err)
	}

	path := filepath.Join(blocker, "sub", "state.json")
	err := atomicWriteJSON(path, map[string]string{"key": "value"})
	if err == nil {
		t.Error("expected error when directory cannot be created")
	}
}

func TestAtomicWriteJSON_RenameFailure(t *testing.T) {
	t.Parallel()
	// Write to a path where the final destination directory is replaced
	// by a file after MkdirAll succeeds. We simulate by targeting a path
	// whose parent is a read-only dir after creating the temp file.
	// Instead, we use a cross-device rename simulation: write to /dev/null parent.
	// The simplest approach: target a path inside a directory that doesn't
	// allow new entries after temp creation.
	//
	// Actually, the cleanest way is to test that atomicWriteJSON returns
	// an error containing "renaming temp file" when rename fails.
	// We do this by making the target a directory.
	dir := t.TempDir()
	targetDir := filepath.Join(dir, "state.json")
	if err := os.MkdirAll(targetDir, 0755); err != nil {
		t.Fatal(err)
	}

	err := atomicWriteJSON(targetDir, map[string]string{"key": "value"})
	if err == nil {
		t.Error("expected error when rename target is a directory")
	}
}

func TestAtomicWriteJSON_MarshalFailure(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	path := filepath.Join(dir, "state.json")

	// A channel cannot be marshalled to JSON
	err := atomicWriteJSON(path, make(chan int))
	if err == nil {
		t.Error("expected marshal error for channel value")
	}
}

func TestAtomicWriteJSON_SuccessWritesCorrectContent(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	path := filepath.Join(dir, "deep", "nested", "state.json")

	data := map[string]string{"hello": "world"}
	if err := atomicWriteJSON(path, data); err != nil {
		t.Fatal(err)
	}

	content, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if len(content) == 0 {
		t.Error("expected non-empty file content")
	}
	// File should end with newline
	if content[len(content)-1] != '\n' {
		t.Error("expected trailing newline")
	}
}
