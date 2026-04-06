package daemon

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"time"
)

// Lock represents the per-worktree daemon lock file inside .wolfcastle/.
// Only one daemon may run per worktree at a time.
type Lock struct {
	PID      int       `json:"pid"`
	Worktree string    `json:"worktree"`
	Branch   string    `json:"branch"`
	Started  time.Time `json:"started"`
}

// lockPath returns the path to daemon.lock. When WOLFCASTLE_LOCK_DIR is set
// (testing escape hatch), the lock lives there instead of inside wolfcastleDir.
func lockPath(wolfcastleDir string) string {
	if dir := os.Getenv("WOLFCASTLE_LOCK_DIR"); dir != "" {
		return filepath.Join(dir, "daemon.lock")
	}
	return filepath.Join(wolfcastleDir, "daemon.lock")
}

// AcquireLock checks for an existing daemon in this worktree and creates
// the lock file. Returns an error if another daemon is already running
// in the same worktree.
func AcquireLock(wolfcastleDir, worktreeDir, branch string) error {
	lp := lockPath(wolfcastleDir)

	// Check for existing lock
	if existing, readErr := ReadLock(wolfcastleDir); readErr == nil {
		if IsProcessRunning(existing.PID) {
			return fmt.Errorf("daemon already running in %s (PID %d, started %s)",
				existing.Worktree, existing.PID, existing.Started.Format(time.RFC3339))
		}
		// Stale lock, remove it
		_ = os.Remove(lp)
	}

	// Create the lock directory if needed
	if err := os.MkdirAll(filepath.Dir(lp), 0755); err != nil {
		return fmt.Errorf("creating lock directory: %w", err)
	}

	lock := Lock{
		PID:      os.Getpid(),
		Worktree: worktreeDir,
		Branch:   branch,
		Started:  time.Now().UTC(),
	}

	data, err := json.MarshalIndent(lock, "", "  ")
	if err != nil {
		return fmt.Errorf("marshaling lock: %w", err)
	}

	// Atomic write: temp file + rename to avoid partial reads.
	tmp, err := os.CreateTemp(filepath.Dir(lp), ".wolfcastle-tmp-*")
	if err != nil {
		return fmt.Errorf("creating temp lock file: %w", err)
	}
	tmpName := tmp.Name()

	if _, err := tmp.Write(data); err != nil {
		_ = tmp.Close()
		_ = os.Remove(tmpName)
		return fmt.Errorf("writing temp lock file: %w", err)
	}
	if err := tmp.Sync(); err != nil {
		_ = tmp.Close()
		_ = os.Remove(tmpName)
		return fmt.Errorf("syncing temp lock file: %w", err)
	}
	if err := tmp.Close(); err != nil {
		_ = os.Remove(tmpName)
		return fmt.Errorf("closing temp lock file: %w", err)
	}
	if err := os.Rename(tmpName, lp); err != nil {
		_ = os.Remove(tmpName)
		return fmt.Errorf("renaming temp lock file: %w", err)
	}

	return nil
}

// ReleaseLock removes the per-worktree lock file. Only removes if the
// lock belongs to the current process (prevents removing another daemon's lock).
func ReleaseLock(wolfcastleDir string) {
	lp := lockPath(wolfcastleDir)
	existing, readErr := ReadLock(wolfcastleDir)
	if readErr != nil {
		return
	}
	if existing.PID == os.Getpid() {
		_ = os.Remove(lp)
	}
}

// ReadLock reads the per-worktree lock file. Returns an error if the
// file doesn't exist or can't be parsed.
func ReadLock(wolfcastleDir string) (*Lock, error) {
	lp := lockPath(wolfcastleDir)
	data, err := os.ReadFile(lp)
	if err != nil {
		return nil, err
	}
	var lock Lock
	if err := json.Unmarshal(data, &lock); err != nil {
		return nil, fmt.Errorf("parsing lock file: %w", err)
	}
	return &lock, nil
}
