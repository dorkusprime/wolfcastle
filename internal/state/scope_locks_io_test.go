package state

import (
	"os"
	"path/filepath"
	"testing"
	"time"
)

// ---------------------------------------------------------------------------
// LoadScopeLocks
// ---------------------------------------------------------------------------

func TestLoadScopeLocks_NonExistentFile(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	path := filepath.Join(dir, "no-such-file.json")

	tbl, err := LoadScopeLocks(path)
	if err != nil {
		t.Fatalf("expected nil error, got %v", err)
	}
	if tbl == nil {
		t.Fatal("expected non-nil ScopeLockTable")
	}
	if tbl.Version != 1 {
		t.Errorf("expected version 1, got %d", tbl.Version)
	}
	if tbl.Locks == nil || len(tbl.Locks) != 0 {
		t.Errorf("expected empty initialized locks map")
	}
}

func TestLoadScopeLocks_ValidFile(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	path := filepath.Join(dir, "scope-locks.json")

	content := `{
  "version": 1,
  "locks": {
    "my-node": {
      "task": "proj/my-node/task-0001",
      "node": "proj/my-node",
      "acquired_at": "2026-03-23T12:00:00Z",
      "pid": 12345
    }
  }
}`
	if err := os.WriteFile(path, []byte(content), 0644); err != nil {
		t.Fatal(err)
	}

	tbl, err := LoadScopeLocks(path)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(tbl.Locks) != 1 {
		t.Fatalf("expected 1 lock, got %d", len(tbl.Locks))
	}
	lock := tbl.Locks["my-node"]
	if lock.Task != "proj/my-node/task-0001" {
		t.Errorf("expected task 'proj/my-node/task-0001', got %q", lock.Task)
	}
	if lock.PID != 12345 {
		t.Errorf("expected PID 12345, got %d", lock.PID)
	}
}

func TestLoadScopeLocks_MalformedJSON(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	path := filepath.Join(dir, "bad.json")

	if err := os.WriteFile(path, []byte("{{broken}"), 0644); err != nil {
		t.Fatal(err)
	}

	_, err := LoadScopeLocks(path)
	if err == nil {
		t.Error("expected error for malformed JSON")
	}
}

func TestLoadScopeLocks_ReadError(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	// Reading a directory triggers a non-NotExist read error.
	_, err := LoadScopeLocks(dir)
	if err == nil {
		t.Error("expected error when reading a directory")
	}
}

func TestLoadScopeLocks_NilLocksMap(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	path := filepath.Join(dir, "scope-locks.json")

	// JSON with null locks field
	if err := os.WriteFile(path, []byte(`{"version":1,"locks":null}`), 0644); err != nil {
		t.Fatal(err)
	}

	tbl, err := LoadScopeLocks(path)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if tbl.Locks == nil {
		t.Error("Locks map should be initialized even when JSON has null")
	}
}

// ---------------------------------------------------------------------------
// SaveScopeLocks
// ---------------------------------------------------------------------------

func TestSaveScopeLocks_CreatesDirectories(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	path := filepath.Join(dir, "nested", "deep", "scope-locks.json")

	tbl := NewScopeLockTable()
	if err := SaveScopeLocks(path, tbl); err != nil {
		t.Fatalf("expected SaveScopeLocks to create directories, got error: %v", err)
	}

	if _, err := os.Stat(path); err != nil {
		t.Errorf("file should exist: %v", err)
	}
}

func TestSaveScopeLocks_Roundtrip(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	path := filepath.Join(dir, "scope-locks.json")

	acq := time.Date(2026, 3, 23, 12, 0, 0, 0, time.UTC)
	original := &ScopeLockTable{
		Version: 1,
		Locks: map[string]ScopeLock{
			"node-a": {Task: "proj/node-a/task-0001", Node: "proj/node-a", AcquiredAt: acq, PID: 100},
			"node-b": {Task: "proj/node-b/task-0002", Node: "proj/node-b", AcquiredAt: acq, PID: 200},
		},
	}

	if err := SaveScopeLocks(path, original); err != nil {
		t.Fatal(err)
	}

	loaded, err := LoadScopeLocks(path)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(loaded.Locks) != 2 {
		t.Fatalf("expected 2 locks, got %d", len(loaded.Locks))
	}
	if loaded.Locks["node-a"].PID != 100 {
		t.Errorf("expected PID 100, got %d", loaded.Locks["node-a"].PID)
	}
	if loaded.Locks["node-b"].Task != "proj/node-b/task-0002" {
		t.Errorf("expected task address, got %q", loaded.Locks["node-b"].Task)
	}
}
