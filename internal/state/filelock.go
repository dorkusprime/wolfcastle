package state

import (
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"syscall"
	"time"
)

// DefaultLockTimeout is the default duration to wait when acquiring a lock.
const DefaultLockTimeout = 5 * time.Second

// ErrLockTimeout is returned when lock acquisition exceeds the timeout.
var ErrLockTimeout = fmt.Errorf("lock acquisition timed out — daemon is currently processing, try again shortly")

// FileLock provides advisory file locking scoped to an engineer namespace.
// The lock file lives at .wolfcastle/projects/{namespace}/.lock and uses
// flock(2) for mutual exclusion between cooperating Wolfcastle processes.
type FileLock struct {
	path    string
	file    *os.File
	timeout time.Duration
}

// NewFileLock creates a FileLock for the given namespace directory.
// The timeout controls how long Acquire will wait before giving up.
func NewFileLock(namespaceDir string, timeout time.Duration) *FileLock {
	return &FileLock{
		path:    filepath.Join(namespaceDir, ".lock"),
		timeout: timeout,
	}
}

// Acquire obtains the advisory lock, blocking up to the configured timeout.
// If a stale lock is detected (the holding process is no longer alive), it is
// cleaned up automatically. The caller must call Release when done.
func (fl *FileLock) Acquire() error {
	dir := filepath.Dir(fl.path)
	if err := os.MkdirAll(dir, 0755); err != nil {
		return fmt.Errorf("creating lock directory: %w", err)
	}

	f, err := os.OpenFile(fl.path, os.O_CREATE|os.O_RDWR, 0644)
	if err != nil {
		return fmt.Errorf("opening lock file: %w", err)
	}

	deadline := time.Now().Add(fl.timeout)
	pollInterval := 50 * time.Millisecond

	for {
		err = syscall.Flock(int(f.Fd()), syscall.LOCK_EX|syscall.LOCK_NB)
		if err == nil {
			// Lock acquired — write our PID for stale-lock detection.
			_ = f.Truncate(0)
			_, _ = f.Seek(0, 0)
			_, _ = fmt.Fprintf(f, "%d\n", os.Getpid())
			_ = f.Sync()
			fl.file = f
			return nil
		}

		// Could not acquire — check for stale lock.
		if fl.tryCleanStaleLock(f) {
			// The stale lock was released; retry immediately.
			continue
		}

		if time.Now().After(deadline) {
			_ = f.Close()
			return ErrLockTimeout
		}
		time.Sleep(pollInterval)
	}
}

// Release drops the advisory lock and closes the file descriptor.
// The lock file is intentionally left on disk so that concurrent processes
// always flock the same inode — removing it would introduce a race where
// two processes open different inodes and bypass mutual exclusion.
// It is safe to call Release on a lock that was never acquired (no-op).
func (fl *FileLock) Release() {
	if fl.file == nil {
		return
	}
	_ = syscall.Flock(int(fl.file.Fd()), syscall.LOCK_UN)
	_ = fl.file.Close()
	fl.file = nil
}

// tryCleanStaleLock reads the PID from the lock file and checks whether that
// process is still alive. If it is not, the lock file is removed so the next
// acquisition attempt can succeed. Returns true if a stale lock was cleaned.
func (fl *FileLock) tryCleanStaleLock(f *os.File) bool {
	_, _ = f.Seek(0, 0)
	buf := make([]byte, 64)
	n, err := f.Read(buf)
	if err != nil || n == 0 {
		return false
	}
	pidStr := strings.TrimSpace(string(buf[:n]))
	pid, err := strconv.Atoi(pidStr)
	if err != nil {
		return false
	}

	// Signal 0 does not send a signal but still performs error checking —
	// it succeeds only if the process exists and we have permission.
	proc, err := os.FindProcess(pid)
	if err != nil {
		// On Unix FindProcess never fails, but guard anyway.
		return false
	}
	err = proc.Signal(syscall.Signal(0))
	if err == nil {
		// Process is alive; lock is not stale.
		return false
	}

	// Process is gone — stale lock. Release flock so we can re-acquire.
	_ = syscall.Flock(int(f.Fd()), syscall.LOCK_UN)
	return true
}

// WithLock is a convenience wrapper that acquires the lock, runs fn, and
// releases the lock regardless of whether fn panics or returns an error.
func (fl *FileLock) WithLock(fn func() error) error {
	if err := fl.Acquire(); err != nil {
		return err
	}
	defer fl.Release()
	return fn()
}
