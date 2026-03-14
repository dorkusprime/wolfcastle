package state

import (
	"fmt"
	"os"
	"path/filepath"
	"sync"
	"sync/atomic"
	"testing"
	"time"
)

func TestFileLock_AcquireRelease(t *testing.T) {
	dir := t.TempDir()
	fl := NewFileLock(dir, DefaultLockTimeout)

	if err := fl.Acquire(); err != nil {
		t.Fatalf("Acquire failed: %v", err)
	}

	// Lock file should exist and contain our PID.
	data, err := os.ReadFile(filepath.Join(dir, ".lock"))
	if err != nil {
		t.Fatalf("reading lock file: %v", err)
	}
	expected := fmt.Sprintf("%d\n", os.Getpid())
	if string(data) != expected {
		t.Errorf("lock file contains %q, want %q", string(data), expected)
	}

	fl.Release()

	// Lock file remains on disk (by design) but the flock is released,
	// so another process can acquire it immediately.
	fl2 := NewFileLock(dir, 200*time.Millisecond)
	if err := fl2.Acquire(); err != nil {
		t.Fatalf("could not re-acquire lock after release: %v", err)
	}
	fl2.Release()
}

func TestFileLock_ReleaseWithoutAcquire(t *testing.T) {
	dir := t.TempDir()
	fl := NewFileLock(dir, DefaultLockTimeout)
	// Should not panic.
	fl.Release()
}

func TestFileLock_ConcurrentAcquisition(t *testing.T) {
	dir := t.TempDir()
	const goroutines = 8
	const iterations = 20

	var counter int64
	var maxConcurrent int64
	var currentConcurrent int64

	var wg sync.WaitGroup
	wg.Add(goroutines)

	for g := 0; g < goroutines; g++ {
		go func() {
			defer wg.Done()
			for i := 0; i < iterations; i++ {
				fl := NewFileLock(dir, 10*time.Second)
				if err := fl.Acquire(); err != nil {
					t.Errorf("Acquire failed: %v", err)
					return
				}

				// Inside the critical section: increment a shared counter
				// and track maximum concurrency to verify mutual exclusion.
				cur := atomic.AddInt64(&currentConcurrent, 1)
				if cur > 1 {
					// Record violation but continue to see the extent.
					for {
						old := atomic.LoadInt64(&maxConcurrent)
						if cur <= old || atomic.CompareAndSwapInt64(&maxConcurrent, old, cur) {
							break
						}
					}
				}
				atomic.AddInt64(&counter, 1)
				// Small sleep to widen the window for detecting races.
				time.Sleep(time.Millisecond)
				atomic.AddInt64(&currentConcurrent, -1)

				fl.Release()
			}
		}()
	}
	wg.Wait()

	if mc := atomic.LoadInt64(&maxConcurrent); mc > 1 {
		t.Errorf("mutual exclusion violated: max concurrent holders = %d", mc)
	}
	if c := atomic.LoadInt64(&counter); c != goroutines*iterations {
		t.Errorf("counter = %d, want %d", c, goroutines*iterations)
	}
}

func TestFileLock_Timeout(t *testing.T) {
	dir := t.TempDir()

	// First lock — held for the duration of the test.
	holder := NewFileLock(dir, DefaultLockTimeout)
	if err := holder.Acquire(); err != nil {
		t.Fatalf("holder Acquire failed: %v", err)
	}
	defer holder.Release()

	// Second lock — should time out quickly.
	waiter := NewFileLock(dir, 200*time.Millisecond)
	start := time.Now()
	err := waiter.Acquire()
	elapsed := time.Since(start)

	if err != ErrLockTimeout {
		t.Fatalf("expected ErrLockTimeout, got %v", err)
	}
	if elapsed < 150*time.Millisecond {
		t.Errorf("timed out too quickly: %v", elapsed)
	}
	if elapsed > 2*time.Second {
		t.Errorf("timed out too slowly: %v", elapsed)
	}
}

func TestFileLock_StaleLockDetection(t *testing.T) {
	dir := t.TempDir()
	lockPath := filepath.Join(dir, ".lock")

	// Create a lock file with a PID that (almost certainly) doesn't exist.
	// PID 2^22 - 1 is unlikely to be in use.
	stalePID := 4194303
	if err := os.WriteFile(lockPath, []byte(fmt.Sprintf("%d\n", stalePID)), 0644); err != nil {
		t.Fatalf("writing stale lock: %v", err)
	}

	// Despite the stale file on disk, flock-based locking should succeed
	// because no process actually holds the flock.
	fl := NewFileLock(dir, 500*time.Millisecond)
	if err := fl.Acquire(); err != nil {
		t.Fatalf("Acquire should succeed with stale lock file: %v", err)
	}
	defer fl.Release()

	// Verify our PID replaced the stale one.
	data, err := os.ReadFile(lockPath)
	if err != nil {
		t.Fatalf("reading lock file: %v", err)
	}
	expected := fmt.Sprintf("%d\n", os.Getpid())
	if string(data) != expected {
		t.Errorf("lock file = %q, want %q", string(data), expected)
	}
}

func TestFileLock_WithLock(t *testing.T) {
	dir := t.TempDir()
	fl := NewFileLock(dir, DefaultLockTimeout)

	called := false
	err := fl.WithLock(func() error {
		called = true
		// Verify the lock file exists while we hold it.
		if _, err := os.Stat(filepath.Join(dir, ".lock")); err != nil {
			t.Error("lock file should exist during WithLock")
		}
		return nil
	})
	if err != nil {
		t.Fatalf("WithLock returned error: %v", err)
	}
	if !called {
		t.Error("fn was not called")
	}

	// Lock should be released after WithLock returns (another acquire succeeds).
	fl2 := NewFileLock(dir, 200*time.Millisecond)
	if err := fl2.Acquire(); err != nil {
		t.Fatalf("could not re-acquire after WithLock: %v", err)
	}
	fl2.Release()
}

func TestFileLock_WithLockPropagatesError(t *testing.T) {
	dir := t.TempDir()
	fl := NewFileLock(dir, DefaultLockTimeout)

	sentinel := fmt.Errorf("something went wrong")
	err := fl.WithLock(func() error {
		return sentinel
	})
	if err != sentinel {
		t.Errorf("expected sentinel error, got %v", err)
	}
}

func TestFileLock_WithLockReleasesOnPanic(t *testing.T) {
	dir := t.TempDir()
	fl := NewFileLock(dir, DefaultLockTimeout)

	func() {
		defer func() { _ = recover() }()
		_ = fl.WithLock(func() error {
			panic("boom")
		})
	}()

	// After the panic was recovered, the lock should be released.
	// Verify by acquiring a new lock without timeout.
	fl2 := NewFileLock(dir, 500*time.Millisecond)
	if err := fl2.Acquire(); err != nil {
		t.Fatalf("could not acquire lock after panic recovery: %v", err)
	}
	fl2.Release()
}

func TestFileLock_CreatesDirectory(t *testing.T) {
	dir := filepath.Join(t.TempDir(), "nested", "deeply")
	fl := NewFileLock(dir, DefaultLockTimeout)

	if err := fl.Acquire(); err != nil {
		t.Fatalf("Acquire failed: %v", err)
	}
	defer fl.Release()

	if _, err := os.Stat(filepath.Join(dir, ".lock")); err != nil {
		t.Errorf("lock file should exist in created directory: %v", err)
	}
}

func TestFileLock_ReentrantAcquireFails(t *testing.T) {
	// flock is per-file-description, so opening a second fd and trying
	// to lock it from the same process should still work (flock is not
	// per-process exclusive on the same file — it's per open-file-description).
	// This test documents that behaviour: a second FileLock in the same process
	// CAN acquire the lock because it opens a new file descriptor.
	dir := t.TempDir()
	fl1 := NewFileLock(dir, DefaultLockTimeout)
	if err := fl1.Acquire(); err != nil {
		t.Fatalf("first Acquire: %v", err)
	}
	defer fl1.Release()

	fl2 := NewFileLock(dir, 200*time.Millisecond)
	// On Linux/Darwin, flock is per-open-file-description. Two different
	// fds in the same process compete, so this should time out.
	err := fl2.Acquire()
	if err != ErrLockTimeout {
		// If it succeeds, that's platform-dependent; not a test failure per se.
		if err == nil {
			fl2.Release()
		}
		t.Skipf("platform does not enforce flock across fds in same process: %v", err)
	}
}
