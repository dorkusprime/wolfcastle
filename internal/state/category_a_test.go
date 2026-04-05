package state

import (
	"fmt"
	"os"
	"path/filepath"
	"testing"
	"time"
)

// ═══════════════════════════════════════════════════════════════════════════
// filelock.go: stale lock cleanup triggers retry
// ═══════════════════════════════════════════════════════════════════════════

func TestAcquire_StaleLockCleanupTriggersRetry(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	lockPath := filepath.Join(dir, ".lock")

	// Write a dead PID to the lock file to simulate a stale lock.
	// We need to first acquire the lock, then manipulate the PID.
	f, err := os.OpenFile(lockPath, os.O_CREATE|os.O_RDWR, 0644)
	if err != nil {
		t.Fatal(err)
	}

	// Lock the file, write a dead PID, then unlock so the next Acquire
	// can detect the stale lock via tryCleanStaleLock.
	if err := flockExclusive(int(f.Fd())); err != nil {
		_ = f.Close()
		t.Skipf("flock not supported: %v", err)
	}
	_, _ = fmt.Fprintf(f, "%d\n", 99999999) // PID that doesn't exist
	_ = f.Sync()
	_ = flockUnlock(int(f.Fd()))
	_ = f.Close()

	// Now acquire with a new FileLock. The stale PID should be detected
	// and cleaned up, allowing acquisition to succeed.
	fl := NewFileLock(dir, 2*time.Second)
	if err := fl.Acquire(); err != nil {
		t.Fatalf("expected Acquire to succeed after stale lock cleanup, got: %v", err)
	}
	fl.Release()
}
