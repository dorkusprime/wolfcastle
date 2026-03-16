package daemon

import (
	"os"
	"path/filepath"
	"testing"
)

func TestWritePID_CreatesPIDFile(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	_ = os.MkdirAll(filepath.Join(dir, "system"), 0755)
	if err := WritePID(dir); err != nil {
		t.Fatal(err)
	}

	pid, err := ReadPID(dir)
	if err != nil {
		t.Fatal(err)
	}
	if pid != os.Getpid() {
		t.Errorf("expected PID %d, got %d", os.Getpid(), pid)
	}
}

func TestReadPID_MissingFile(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	_, err := ReadPID(dir)
	if err == nil {
		t.Error("expected error when PID file does not exist")
	}
}

func TestReadPID_InvalidContent(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	_ = os.MkdirAll(filepath.Join(dir, "system"), 0755)
	_ = os.WriteFile(filepath.Join(dir, "system", "wolfcastle.pid"), []byte("not-a-number"), 0644)

	_, err := ReadPID(dir)
	if err == nil {
		t.Error("expected error for invalid PID content")
	}
}

func TestRemovePID_RemovesFile(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	_ = os.MkdirAll(filepath.Join(dir, "system"), 0755)
	_ = WritePID(dir)

	RemovePID(dir)

	pidPath := filepath.Join(dir, "system", "wolfcastle.pid")
	if _, err := os.Stat(pidPath); !os.IsNotExist(err) {
		t.Error("PID file should be removed")
	}
}

func TestRemovePID_NoOpOnMissingFile(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	// Should not panic or error
	RemovePID(dir)
}

func TestIsProcessRunning_CurrentProcess(t *testing.T) {
	t.Parallel()
	if !IsProcessRunning(os.Getpid()) {
		t.Error("current process should be running")
	}
}

func TestIsProcessRunning_DeadProcess(t *testing.T) {
	t.Parallel()
	// PID 99999999 is extremely unlikely to be running
	if IsProcessRunning(99999999) {
		t.Error("PID 99999999 should not be running")
	}
}
