package daemon

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestAcquireGlobalLock_CreatesLockFile(t *testing.T) {
	// Use a temp dir as HOME to avoid touching the real ~/.wolfcastle
	tmpHome := t.TempDir()
	t.Setenv("HOME", tmpHome)

	err := AcquireGlobalLock("/repo", "/repo/worktree")
	if err != nil {
		t.Fatalf("AcquireGlobalLock: %v", err)
	}
	defer ReleaseGlobalLock()

	lock, err := ReadGlobalLock()
	if err != nil {
		t.Fatalf("ReadGlobalLock: %v", err)
	}
	if lock.PID != os.Getpid() {
		t.Errorf("expected PID %d, got %d", os.Getpid(), lock.PID)
	}
	if lock.Repo != "/repo" {
		t.Errorf("expected repo /repo, got %s", lock.Repo)
	}
	if lock.Worktree != "/repo/worktree" {
		t.Errorf("expected worktree /repo/worktree, got %s", lock.Worktree)
	}
	if lock.Started.IsZero() {
		t.Error("started should be set")
	}
}

func TestAcquireGlobalLock_FailsWhenAlreadyRunning(t *testing.T) {
	tmpHome := t.TempDir()
	t.Setenv("HOME", tmpHome)

	// Write a lock file with the current PID (simulating a running daemon)
	lockDir := filepath.Join(tmpHome, ".wolfcastle")
	_ = os.MkdirAll(lockDir, 0755)
	lock := GlobalLock{
		PID:      os.Getpid(), // Current process is definitely alive
		Repo:     "/other/repo",
		Worktree: "/other/repo/feat",
		Started:  time.Now().UTC(),
	}
	data, _ := json.MarshalIndent(lock, "", "  ")
	_ = os.WriteFile(filepath.Join(lockDir, "daemon.lock"), data, 0644)

	err := AcquireGlobalLock("/repo", "/repo/worktree")
	if err == nil {
		t.Fatal("expected error when daemon already running")
	}
	if !contains(err.Error(), "daemon already running") {
		t.Errorf("expected 'daemon already running' error, got: %v", err)
	}
	if !contains(err.Error(), "/other/repo/feat") {
		t.Errorf("error should mention the existing worktree, got: %v", err)
	}
}

func TestAcquireGlobalLock_RemovesStaleLock(t *testing.T) {
	tmpHome := t.TempDir()
	t.Setenv("HOME", tmpHome)

	// Write a lock with a PID that doesn't exist
	lockDir := filepath.Join(tmpHome, ".wolfcastle")
	_ = os.MkdirAll(lockDir, 0755)
	lock := GlobalLock{
		PID:      99999999, // Almost certainly not running
		Repo:     "/old/repo",
		Worktree: "/old/repo/old-branch",
		Started:  time.Now().Add(-24 * time.Hour).UTC(),
	}
	data, _ := json.MarshalIndent(lock, "", "  ")
	_ = os.WriteFile(filepath.Join(lockDir, "daemon.lock"), data, 0644)

	// Should succeed because old PID is dead
	err := AcquireGlobalLock("/repo", "/repo/worktree")
	if err != nil {
		t.Fatalf("expected stale lock to be replaced: %v", err)
	}
	defer ReleaseGlobalLock()

	// Verify lock was replaced
	newLock, _ := ReadGlobalLock()
	if newLock.PID != os.Getpid() {
		t.Errorf("lock should have current PID, got %d", newLock.PID)
	}
}

func TestReleaseGlobalLock_OnlyRemovesOwnLock(t *testing.T) {
	tmpHome := t.TempDir()
	t.Setenv("HOME", tmpHome)

	// Write a lock with a different PID
	lockDir := filepath.Join(tmpHome, ".wolfcastle")
	_ = os.MkdirAll(lockDir, 0755)
	lock := GlobalLock{
		PID:      os.Getpid() + 1, // Different process
		Repo:     "/other",
		Worktree: "/other",
		Started:  time.Now().UTC(),
	}
	data, _ := json.MarshalIndent(lock, "", "  ")
	lockPath := filepath.Join(lockDir, "daemon.lock")
	_ = os.WriteFile(lockPath, data, 0644)

	// Release should not remove another process's lock
	ReleaseGlobalLock()

	if _, err := os.Stat(lockPath); os.IsNotExist(err) {
		t.Error("release should not remove another process's lock file")
	}
}

func TestReadGlobalLock_MissingFile(t *testing.T) {
	tmpHome := t.TempDir()
	t.Setenv("HOME", tmpHome)

	_, err := ReadGlobalLock()
	if err == nil {
		t.Error("expected error for missing lock file")
	}
}

func contains(s, substr string) bool {
	return len(s) >= len(substr) && searchString(s, substr)
}

func searchString(s, substr string) bool {
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}
