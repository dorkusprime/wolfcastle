package daemon

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestLoadParallelStatus_InvalidJSON(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	sysDir := filepath.Join(dir, "system")
	_ = os.MkdirAll(sysDir, 0755)
	_ = os.WriteFile(filepath.Join(sysDir, "parallel-status.json"), []byte("not valid json{{{"), 0644)

	status := LoadParallelStatus(dir)
	if status != nil {
		t.Error("expected nil for invalid JSON, got non-nil")
	}
}

func TestLoadParallelStatus_MissingFile(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()

	status := LoadParallelStatus(dir)
	if status != nil {
		t.Error("expected nil for missing file, got non-nil")
	}
}

func TestLoadParallelStatus_ValidFile(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	sysDir := filepath.Join(dir, "system")
	_ = os.MkdirAll(sysDir, 0755)

	ps := ParallelStatus{
		MaxWorkers: 4,
		Active: []ParallelWorkerEntry{
			{Task: "task-0001", Node: "leaf"},
		},
		UpdatedAt: time.Now().UTC(),
	}
	data, _ := json.Marshal(ps)
	_ = os.WriteFile(filepath.Join(sysDir, "parallel-status.json"), data, 0644)

	result := LoadParallelStatus(dir)
	if result == nil {
		t.Fatal("expected non-nil status")
	}
	if result.MaxWorkers != 4 {
		t.Errorf("expected MaxWorkers=4, got %d", result.MaxWorkers)
	}
	if len(result.Active) != 1 {
		t.Errorf("expected 1 active worker, got %d", len(result.Active))
	}
}

func TestWriteStatusSnapshot_WritesFile(t *testing.T) {
	t.Parallel()
	d := parallelTestDaemon(t, 2)

	dispatcher := NewParallelDispatcher(d, 2)

	// Should not panic on empty state.
	dispatcher.writeStatusSnapshot()

	// Verify file was written.
	statusPath := parallelStatusPath(d.WolfcastleDir)
	if _, err := os.Stat(statusPath); os.IsNotExist(err) {
		t.Error("expected parallel-status.json to be written")
	}
}

func TestRemoveStatusFile_ExistingFile(t *testing.T) {
	t.Parallel()
	d := parallelTestDaemon(t, 1)

	statusPath := parallelStatusPath(d.WolfcastleDir)
	_ = os.MkdirAll(filepath.Dir(statusPath), 0755)
	_ = os.WriteFile(statusPath, []byte("{}"), 0644)

	dispatcher := NewParallelDispatcher(d, 1)
	dispatcher.removeStatusFile()

	if _, err := os.Stat(statusPath); !os.IsNotExist(err) {
		t.Error("expected status file to be removed")
	}
}

func TestRemoveStatusFile_NoFile(t *testing.T) {
	t.Parallel()
	d := parallelTestDaemon(t, 1)

	dispatcher := NewParallelDispatcher(d, 1)
	// Should not panic when file doesn't exist.
	dispatcher.removeStatusFile()
}
