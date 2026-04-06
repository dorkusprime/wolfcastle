package daemon

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestAcquireLock_CreatesLockFile(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("WOLFCASTLE_LOCK_DIR", dir)

	err := AcquireLock(dir, "/repo/worktree", "main")
	if err != nil {
		t.Fatalf("AcquireLock: %v", err)
	}
	defer ReleaseLock(dir)

	lock, err := ReadLock(dir)
	if err != nil {
		t.Fatalf("ReadLock: %v", err)
	}
	if lock.PID != os.Getpid() {
		t.Errorf("expected PID %d, got %d", os.Getpid(), lock.PID)
	}
	if lock.Worktree != "/repo/worktree" {
		t.Errorf("expected worktree /repo/worktree, got %s", lock.Worktree)
	}
	if lock.Branch != "main" {
		t.Errorf("expected branch main, got %s", lock.Branch)
	}
	if lock.Started.IsZero() {
		t.Error("started should be set")
	}
}

func TestAcquireLock_FailsWhenAlreadyRunning(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("WOLFCASTLE_LOCK_DIR", dir)

	// Write a lock file with the current PID (simulating a running daemon)
	lock := Lock{
		PID:      os.Getpid(), // Current process is definitely alive
		Worktree: "/other/repo/feat",
		Branch:   "feat/thing",
		Started:  time.Now().UTC(),
	}
	data, _ := json.MarshalIndent(lock, "", "  ")
	_ = os.WriteFile(filepath.Join(dir, "daemon.lock"), data, 0644)

	err := AcquireLock(dir, "/repo/worktree", "main")
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

func TestAcquireLock_RemovesStaleLock(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("WOLFCASTLE_LOCK_DIR", dir)

	// Write a lock with a PID that doesn't exist
	lock := Lock{
		PID:      99999999, // Almost certainly not running
		Worktree: "/old/repo/old-branch",
		Branch:   "old-branch",
		Started:  time.Now().Add(-24 * time.Hour).UTC(),
	}
	data, _ := json.MarshalIndent(lock, "", "  ")
	_ = os.WriteFile(filepath.Join(dir, "daemon.lock"), data, 0644)

	// Should succeed because old PID is dead
	err := AcquireLock(dir, "/repo/worktree", "main")
	if err != nil {
		t.Fatalf("expected stale lock to be replaced: %v", err)
	}
	defer ReleaseLock(dir)

	// Verify lock was replaced
	newLock, _ := ReadLock(dir)
	if newLock.PID != os.Getpid() {
		t.Errorf("lock should have current PID, got %d", newLock.PID)
	}
}

func TestReleaseLock_OnlyRemovesOwnLock(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("WOLFCASTLE_LOCK_DIR", dir)

	// Write a lock with a different PID
	lock := Lock{
		PID:      os.Getpid() + 1, // Different process
		Worktree: "/other",
		Branch:   "other",
		Started:  time.Now().UTC(),
	}
	data, _ := json.MarshalIndent(lock, "", "  ")
	lp := filepath.Join(dir, "daemon.lock")
	_ = os.WriteFile(lp, data, 0644)

	// Release should not remove another process's lock
	ReleaseLock(dir)

	if _, err := os.Stat(lp); os.IsNotExist(err) {
		t.Error("release should not remove another process's lock file")
	}
}

func TestReadLock_MissingFile(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("WOLFCASTLE_LOCK_DIR", dir)

	_, err := ReadLock(dir)
	if err == nil {
		t.Error("expected error for missing lock file")
	}
}

func TestReleaseLock_MissingFile(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("WOLFCASTLE_LOCK_DIR", dir)

	// Should not panic or error when no lock file exists.
	ReleaseLock(dir)
}

func TestLockPath_EnvOverride(t *testing.T) {
	t.Setenv("WOLFCASTLE_LOCK_DIR", "/custom/lock/dir")
	got := lockPath("/ignored/wolfcastle/dir")
	want := filepath.Join("/custom/lock/dir", "daemon.lock")
	if got != want {
		t.Errorf("lockPath = %q, want %q", got, want)
	}
}

func TestLockPath_Default(t *testing.T) {
	t.Setenv("WOLFCASTLE_LOCK_DIR", "")
	got := lockPath("/my/wolfcastle")
	want := filepath.Join("/my/wolfcastle", "daemon.lock")
	if got != want {
		t.Errorf("lockPath = %q, want %q", got, want)
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
