package state

import (
	"fmt"
	"os"
	"path/filepath"
	"testing"
	"time"
)

// ═══════════════════════════════════════════════════════════════════════════
// tryCleanStaleLock: various PID states
// ═══════════════════════════════════════════════════════════════════════════

func TestTryCleanStaleLock_EmptyFile(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	lockPath := filepath.Join(dir, ".lock")

	// Create an empty lock file
	f, err := os.Create(lockPath)
	if err != nil {
		t.Fatal(err)
	}

	fl := &FileLock{path: lockPath}
	result := fl.tryCleanStaleLock(f)
	_ = f.Close()

	if result {
		t.Error("empty lock file should not be cleaned (no PID to check)")
	}
}

func TestTryCleanStaleLock_InvalidPID(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	lockPath := filepath.Join(dir, ".lock")

	f, err := os.OpenFile(lockPath, os.O_CREATE|os.O_RDWR, 0644)
	if err != nil {
		t.Fatal(err)
	}
	_, _ = f.WriteString("not-a-number\n")
	_ = f.Sync()

	fl := &FileLock{path: lockPath}
	result := fl.tryCleanStaleLock(f)
	_ = f.Close()

	if result {
		t.Error("non-numeric PID should not trigger cleanup")
	}
}

func TestTryCleanStaleLock_LiveProcess(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	lockPath := filepath.Join(dir, ".lock")

	f, err := os.OpenFile(lockPath, os.O_CREATE|os.O_RDWR, 0644)
	if err != nil {
		t.Fatal(err)
	}
	// Write our own PID. This process is alive
	pid := os.Getpid()
	_, _ = fmt.Fprintf(f, "%d\n", pid)
	_ = f.Sync()

	fl := &FileLock{path: lockPath}
	result := fl.tryCleanStaleLock(f)
	_ = f.Close()

	if result {
		t.Error("live process PID should not be cleaned")
	}
}

func TestTryCleanStaleLock_DeadProcess(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	lockPath := filepath.Join(dir, ".lock")

	f, err := os.OpenFile(lockPath, os.O_CREATE|os.O_RDWR, 0644)
	if err != nil {
		t.Fatal(err)
	}
	// Write a very high PID that almost certainly doesn't exist
	_, _ = f.Write([]byte("99999999\n"))
	_ = f.Sync()

	fl := &FileLock{path: lockPath}
	result := fl.tryCleanStaleLock(f)
	_ = f.Close()

	if !result {
		t.Error("dead process PID should trigger cleanup")
	}
}

// ═══════════════════════════════════════════════════════════════════════════
// Acquire / Release: basic lifecycle
// ═══════════════════════════════════════════════════════════════════════════

func TestAcquireRelease_BasicCycle(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	fl := NewFileLock(dir, DefaultLockTimeout)

	if err := fl.Acquire(); err != nil {
		t.Fatalf("Acquire failed: %v", err)
	}
	fl.Release()

	// Should be able to acquire again after release
	if err := fl.Acquire(); err != nil {
		t.Fatalf("second Acquire failed: %v", err)
	}
	fl.Release()
}

func TestRelease_Noop_WhenNotAcquired(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	fl := NewFileLock(dir, DefaultLockTimeout)

	// Should not panic
	fl.Release()
}

func TestAcquire_Timeout_WhenHeld(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()

	holder := NewFileLock(dir, DefaultLockTimeout)
	if err := holder.Acquire(); err != nil {
		t.Fatalf("holder Acquire: %v", err)
	}
	defer holder.Release()

	// Attempt with very short timeout
	contender := NewFileLock(dir, 100*time.Millisecond)
	err := contender.Acquire()
	if err != ErrLockTimeout {
		t.Errorf("expected ErrLockTimeout, got %v", err)
	}
}

// ═══════════════════════════════════════════════════════════════════════════
// WithLock: success path
// ═══════════════════════════════════════════════════════════════════════════

func TestWithLock_Success(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	fl := NewFileLock(dir, DefaultLockTimeout)

	called := false
	err := fl.WithLock(func() error {
		called = true
		return nil
	})
	if err != nil {
		t.Fatalf("WithLock error: %v", err)
	}
	if !called {
		t.Error("fn should have been called")
	}

	// Lock should be released. Verify by acquiring again
	fl2 := NewFileLock(dir, 200*time.Millisecond)
	if err := fl2.Acquire(); err != nil {
		t.Fatalf("could not acquire after WithLock: %v", err)
	}
	fl2.Release()
}

// ═══════════════════════════════════════════════════════════════════════════
// Acquire: writes PID to lock file
// ═══════════════════════════════════════════════════════════════════════════

func TestAcquire_WritesPIDToLockFile(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	fl := NewFileLock(dir, DefaultLockTimeout)

	if err := fl.Acquire(); err != nil {
		t.Fatal(err)
	}
	defer fl.Release()

	// Read the lock file content
	data, err := os.ReadFile(filepath.Join(dir, ".lock"))
	if err != nil {
		t.Fatal(err)
	}

	if len(data) == 0 {
		t.Error("lock file should contain PID")
	}
}

// ═══════════════════════════════════════════════════════════════════════════
// signalProcess: live and dead process
// ═══════════════════════════════════════════════════════════════════════════

func TestSignalProcess_LiveProcess(t *testing.T) {
	t.Parallel()
	// PID 1 is always alive
	err := signalProcess(1)
	if err != nil {
		// On some systems, signaling PID 1 may fail with EPERM, which is expected
		t.Logf("signalProcess(1) returned: %v (acceptable)", err)
	}
}

func TestSignalProcess_DeadProcess(t *testing.T) {
	t.Parallel()
	err := signalProcess(99999999)
	if err == nil {
		t.Error("expected error for dead process PID")
	}
}
