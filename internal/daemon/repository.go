package daemon

import (
	"errors"
	"os"
	"path/filepath"

	"github.com/dorkusprime/wolfcastle/internal/instance"
	"github.com/dorkusprime/wolfcastle/internal/tierfs"
)

// DaemonRepository consolidates all filesystem operations the daemon
// performs against its system directory. Every path the daemon touches
// (PID file, stop file, log directory) flows through this struct,
// eliminating scattered filepath.Join constructions.
type DaemonRepository struct {
	systemDir string
}

// NewDaemonRepository creates a DaemonRepository rooted at the given
// wolfcastle directory. All paths are derived from wolfcastleRoot/system.
func NewDaemonRepository(wolfcastleRoot string) *DaemonRepository {
	return &DaemonRepository{
		systemDir: filepath.Join(wolfcastleRoot, tierfs.SystemPrefix),
	}
}

func (r *DaemonRepository) stopPath() string {
	return filepath.Join(r.systemDir, "stop")
}

// HasStopFile reports whether the stop file exists.
func (r *DaemonRepository) HasStopFile() bool {
	_, err := os.Stat(r.stopPath())
	return err == nil
}

// WriteStopFile creates the stop file (empty, 0644).
func (r *DaemonRepository) WriteStopFile() error {
	return os.WriteFile(r.stopPath(), nil, 0644)
}

// RemoveStopFile removes the stop file. Returns nil if the file does not exist.
func (r *DaemonRepository) RemoveStopFile() error {
	err := os.Remove(r.stopPath())
	if errors.Is(err, os.ErrNotExist) {
		return nil
	}
	return err
}

// StopFileExists reports whether the stop file exists on disk.
func (r *DaemonRepository) StopFileExists() bool {
	_, err := os.Stat(r.stopPath())
	return err == nil
}

// IsAlive checks the instance registry for a running daemon whose
// worktree matches this repository's root directory (parent of systemDir).
func (r *DaemonRepository) IsAlive() bool {
	repoDir := filepath.Dir(r.systemDir)
	entry, err := instance.Resolve(repoDir)
	if err != nil {
		return false
	}
	return IsProcessRunning(entry.PID)
}

// HasDrainFile reports whether the drain file exists.
func (r *DaemonRepository) HasDrainFile() bool {
	_, err := os.Stat(r.drainPath())
	return err == nil
}

// WriteDrainFile creates the drain file (empty, 0644).
func (r *DaemonRepository) WriteDrainFile() error {
	return os.WriteFile(r.drainPath(), nil, 0644)
}

// RemoveDrainFile removes the drain file. Returns nil if the file does not exist.
func (r *DaemonRepository) RemoveDrainFile() error {
	err := os.Remove(r.drainPath())
	if errors.Is(err, os.ErrNotExist) {
		return nil
	}
	return err
}

func (r *DaemonRepository) drainPath() string {
	return filepath.Join(r.systemDir, "drain")
}

// LogDir returns the path to the daemon log directory. This is an
// intentional escape hatch: the Logger manages its own file handles,
// rotation, and compression, so it needs the directory path rather
// than a repository method for each log operation.
func (r *DaemonRepository) LogDir() string {
	return filepath.Join(r.systemDir, "logs")
}
