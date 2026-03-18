package daemon

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"time"
)

// GlobalLock represents the daemon lock file at ~/.wolfcastle/daemon.lock.
// One daemon runs globally at a time.
type GlobalLock struct {
	PID      int       `json:"pid"`
	Repo     string    `json:"repo"`
	Worktree string    `json:"worktree"`
	Started  time.Time `json:"started"`
}

// GlobalLockDir overrides the lock directory for testing.
// When empty, defaults to ~/.wolfcastle/.
var GlobalLockDir string

// globalLockPath returns the path to daemon.lock. It checks GlobalLockDir
// first, then WOLFCASTLE_LOCK_DIR, then falls back to ~/.wolfcastle/.
func globalLockPath() (string, error) {
	dir := GlobalLockDir
	if dir == "" {
		dir = os.Getenv("WOLFCASTLE_LOCK_DIR")
	}
	if dir == "" {
		home, err := os.UserHomeDir()
		if err != nil {
			return "", fmt.Errorf("resolving home directory: %w", err)
		}
		dir = filepath.Join(home, ".wolfcastle")
	}
	return filepath.Join(dir, "daemon.lock"), nil
}

// AcquireGlobalLock checks for an existing daemon and creates the lock file.
// Returns an error if another daemon is running.
func AcquireGlobalLock(repoDir, worktreeDir string) error {
	lockPath, err := globalLockPath()
	if err != nil {
		return err
	}

	// Check for existing lock
	if existing, readErr := ReadGlobalLock(); readErr == nil {
		if IsProcessRunning(existing.PID) {
			return fmt.Errorf("daemon already running in %s (PID %d, started %s)",
				existing.Worktree, existing.PID, existing.Started.Format(time.RFC3339))
		}
		// Stale lock, remove it
		_ = os.Remove(lockPath)
	}

	// Create the lock directory if needed
	if err := os.MkdirAll(filepath.Dir(lockPath), 0755); err != nil {
		return fmt.Errorf("creating lock directory: %w", err)
	}

	lock := GlobalLock{
		PID:      os.Getpid(),
		Repo:     repoDir,
		Worktree: worktreeDir,
		Started:  time.Now().UTC(),
	}

	data, err := json.MarshalIndent(lock, "", "  ")
	if err != nil {
		return fmt.Errorf("marshaling lock: %w", err)
	}

	if err := os.WriteFile(lockPath, data, 0644); err != nil {
		return fmt.Errorf("writing lock file: %w", err)
	}

	return nil
}

// ReleaseGlobalLock removes the global lock file. Only removes if the
// lock belongs to the current process (prevents removing another daemon's lock
// in race conditions).
func ReleaseGlobalLock() {
	lockPath, err := globalLockPath()
	if err != nil {
		return
	}
	existing, readErr := ReadGlobalLock()
	if readErr != nil {
		return
	}
	if existing.PID == os.Getpid() {
		_ = os.Remove(lockPath)
	}
}

// ReadGlobalLock reads the global lock file. Returns an error if the
// file doesn't exist or can't be parsed.
func ReadGlobalLock() (*GlobalLock, error) {
	lockPath, err := globalLockPath()
	if err != nil {
		return nil, err
	}
	data, err := os.ReadFile(lockPath)
	if err != nil {
		return nil, err
	}
	var lock GlobalLock
	if err := json.Unmarshal(data, &lock); err != nil {
		return nil, fmt.Errorf("parsing lock file: %w", err)
	}
	return &lock, nil
}
