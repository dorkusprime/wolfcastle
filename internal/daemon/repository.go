package daemon

import (
	"errors"
	"os"
	"path/filepath"

	"github.com/dorkusprime/wolfcastle/internal/instance"
	"github.com/dorkusprime/wolfcastle/internal/tierfs"
)

// Repository consolidates all filesystem operations the daemon
// performs against its system directory. Every path the daemon touches
// (PID file, stop file, log directory) flows through this struct,
// eliminating scattered filepath.Join constructions.
type Repository struct {
	systemDir string
}

// NewRepository creates a Repository rooted at the given
// wolfcastle directory. All paths are derived from wolfcastleRoot/system.
func NewRepository(wolfcastleRoot string) *Repository {
	return &Repository{
		systemDir: filepath.Join(wolfcastleRoot, tierfs.SystemPrefix),
	}
}

func (r *Repository) stopPath() string {
	return filepath.Join(r.systemDir, "stop")
}

// HasStopFile reports whether the stop file exists.
func (r *Repository) HasStopFile() bool {
	_, err := os.Stat(r.stopPath())
	return err == nil
}

// WriteStopFile creates the stop file (empty, 0644).
func (r *Repository) WriteStopFile() error {
	return os.WriteFile(r.stopPath(), nil, 0644)
}

// RemoveStopFile removes the stop file. Returns nil if the file does not exist.
func (r *Repository) RemoveStopFile() error {
	err := os.Remove(r.stopPath())
	if errors.Is(err, os.ErrNotExist) {
		return nil
	}
	return err
}

// StopFileExists reports whether the stop file exists on disk.
func (r *Repository) StopFileExists() bool {
	_, err := os.Stat(r.stopPath())
	return err == nil
}

// IsAlive checks the instance registry for a running daemon whose
// worktree matches this repository's root directory (parent of systemDir).
func (r *Repository) IsAlive() bool {
	repoDir := filepath.Dir(r.systemDir)
	entry, err := instance.Resolve(repoDir)
	if err != nil {
		return false
	}
	return IsProcessRunning(entry.PID)
}

// HasDrainFile reports whether the drain file exists.
func (r *Repository) HasDrainFile() bool {
	_, err := os.Stat(r.drainPath())
	return err == nil
}

// WriteDrainFile creates the drain file (empty, 0644).
func (r *Repository) WriteDrainFile() error {
	return os.WriteFile(r.drainPath(), nil, 0644)
}

// RemoveDrainFile removes the drain file. Returns nil if the file does not exist.
func (r *Repository) RemoveDrainFile() error {
	err := os.Remove(r.drainPath())
	if errors.Is(err, os.ErrNotExist) {
		return nil
	}
	return err
}

func (r *Repository) drainPath() string {
	return filepath.Join(r.systemDir, "drain")
}

// LogDir returns the path to the daemon log directory. This is an
// intentional escape hatch: the Logger manages its own file handles,
// rotation, and compression, so it needs the directory path rather
// than a repository method for each log operation.
func (r *Repository) LogDir() string {
	return filepath.Join(r.systemDir, "logs")
}
