package state

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"
)

// DefaultLockTimeout is the default duration to wait when acquiring a lock.
const DefaultLockTimeout = 5 * time.Second

// ErrLockTimeout is returned when lock acquisition exceeds the configured timeout,
// typically because the daemon is mid-iteration.
var ErrLockTimeout = errors.New("lock acquisition timed out — daemon is currently processing, try again shortly")

// FileLock provides advisory file locking scoped to an engineer namespace.
// The lock file lives at .wolfcastle/projects/{namespace}/.lock and uses
// flock(2) on Unix for mutual exclusion between cooperating Wolfcastle
// processes. On Windows, locking is best-effort (no-op).
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
		err = flockExclusive(int(f.Fd()))
		if err == nil {
			_ = f.Truncate(0)
			_, _ = f.Seek(0, 0)
			_, _ = fmt.Fprintf(f, "%d\n", os.Getpid())
			_ = f.Sync()
			fl.file = f
			return nil
		}

		if fl.tryCleanStaleLock(f) {
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
// It is safe to call Release on a lock that was never acquired (no-op).
func (fl *FileLock) Release() {
	if fl.file == nil {
		return
	}
	_ = flockUnlock(int(fl.file.Fd()))
	_ = fl.file.Close()
	fl.file = nil
}

// tryCleanStaleLock reads the PID from the lock file and checks whether that
// process is still alive. If it is not, the flock is released so the next
// acquisition attempt can succeed.
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

	err = signalProcess(pid)
	if err == nil {
		return false
	}

	_ = flockUnlock(int(f.Fd()))
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
