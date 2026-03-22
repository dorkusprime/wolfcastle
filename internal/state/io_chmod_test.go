package state

import (
	"os"
	"path/filepath"
	"runtime"
	"testing"
)

// readOnlyDir creates a temporary directory with 0555 permissions,
// restoring 0755 on cleanup so the test framework can remove it.
func readOnlyDir(t *testing.T) string {
	t.Helper()
	dir := t.TempDir()
	if err := os.Chmod(dir, 0555); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = os.Chmod(dir, 0755) })
	return dir
}

func TestAtomicWriteJSON_MkdirAll_ReadOnlyParent(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("permission test not reliable on Windows")
	}
	t.Parallel()

	roDir := readOnlyDir(t)
	// MkdirAll needs to create "sub" inside read-only parent — should fail.
	path := filepath.Join(roDir, "sub", "state.json")

	err := atomicWriteJSON(path, map[string]string{"k": "v"})
	if err == nil {
		t.Error("expected MkdirAll error when parent is read-only")
	}
}

func TestAtomicWriteJSON_CreateTemp_ReadOnlyDir(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("permission test not reliable on Windows")
	}
	t.Parallel()

	// Create the target directory first, then lock it.
	dir := t.TempDir()
	sub := filepath.Join(dir, "locked")
	if err := os.MkdirAll(sub, 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.Chmod(sub, 0555); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = os.Chmod(sub, 0755) })

	path := filepath.Join(sub, "state.json")
	err := atomicWriteJSON(path, map[string]string{"k": "v"})
	if err == nil {
		t.Error("expected CreateTemp error when directory is read-only")
	}
}

func TestAtomicWriteJSON_Rename_TargetIsDir(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("permission test not reliable on Windows")
	}
	t.Parallel()

	dir := t.TempDir()
	// Place a directory where the destination file should be — rename fails.
	target := filepath.Join(dir, "state.json")
	if err := os.MkdirAll(target, 0755); err != nil {
		t.Fatal(err)
	}

	err := atomicWriteJSON(target, map[string]string{"k": "v"})
	if err == nil {
		t.Error("expected rename error when target is a directory")
	}
}

func TestAtomicWriteFile_MkdirAll_ReadOnlyParent(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("permission test not reliable on Windows")
	}
	t.Parallel()

	roDir := readOnlyDir(t)
	path := filepath.Join(roDir, "sub", "file.txt")

	err := atomicWriteFile(path, []byte("data"))
	if err == nil {
		t.Error("expected MkdirAll error when parent is read-only")
	}
}

func TestAtomicWriteFile_CreateTemp_ReadOnlyDir(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("permission test not reliable on Windows")
	}
	t.Parallel()

	dir := t.TempDir()
	sub := filepath.Join(dir, "locked")
	if err := os.MkdirAll(sub, 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.Chmod(sub, 0555); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = os.Chmod(sub, 0755) })

	path := filepath.Join(sub, "file.txt")
	err := atomicWriteFile(path, []byte("data"))
	if err == nil {
		t.Error("expected CreateTemp error when directory is read-only")
	}
}

func TestAtomicWriteFile_Rename_TargetIsDir(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("permission test not reliable on Windows")
	}
	t.Parallel()

	dir := t.TempDir()
	target := filepath.Join(dir, "file.txt")
	if err := os.MkdirAll(target, 0755); err != nil {
		t.Fatal(err)
	}

	err := atomicWriteFile(target, []byte("data"))
	if err == nil {
		t.Error("expected rename error when target is a directory")
	}
}

func TestSaveRootIndex_ReadOnlyDir(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("permission test not reliable on Windows")
	}
	t.Parallel()

	roDir := readOnlyDir(t)
	idx := NewRootIndex()

	err := SaveRootIndex(filepath.Join(roDir, "state.json"), idx)
	if err == nil {
		t.Error("expected error saving root index to read-only directory")
	}
}

func TestSaveNodeState_ReadOnlyDir(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("permission test not reliable on Windows")
	}
	t.Parallel()

	roDir := readOnlyDir(t)
	ns := NewNodeState("test", "Test", NodeLeaf)

	err := SaveNodeState(filepath.Join(roDir, "state.json"), ns)
	if err == nil {
		t.Error("expected error saving node state to read-only directory")
	}
}

func TestSaveInbox_ReadOnlyDir(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("permission test not reliable on Windows")
	}
	t.Parallel()

	roDir := readOnlyDir(t)
	inbox := &InboxFile{Items: []InboxItem{{Text: "test"}}}

	err := SaveInbox(filepath.Join(roDir, "inbox.json"), inbox)
	if err == nil {
		t.Error("expected error saving inbox to read-only directory")
	}
}
