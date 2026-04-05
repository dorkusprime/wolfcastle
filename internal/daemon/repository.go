package daemon

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"syscall"

	"github.com/dorkusprime/wolfcastle/internal/state"
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
		systemDir: filepath.Join(wolfcastleRoot, "system"),
	}
}

func (r *DaemonRepository) pidPath() string {
	return filepath.Join(r.systemDir, "wolfcastle.pid")
}

func (r *DaemonRepository) stopPath() string {
	return filepath.Join(r.systemDir, "stop")
}

// ReadPID reads the daemon PID from the PID file, trimming whitespace
// and converting to int.
func (r *DaemonRepository) ReadPID() (int, error) {
	data, err := os.ReadFile(r.pidPath())
	if err != nil {
		return 0, err
	}
	return strconv.Atoi(strings.TrimSpace(string(data)))
}

// WritePID writes the given PID as a decimal string with a trailing newline.
// Uses atomic write (temp file + rename) so a crash mid-write never leaves
// a truncated PID file.
func (r *DaemonRepository) WritePID(pid int) error {
	return state.AtomicWriteFile(r.pidPath(), []byte(fmt.Sprintf("%d\n", pid)))
}

// RemovePID removes the PID file. Returns nil if the file does not exist.
func (r *DaemonRepository) RemovePID() error {
	err := os.Remove(r.pidPath())
	if errors.Is(err, os.ErrNotExist) {
		return nil
	}
	return err
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

// IsAlive checks whether a daemon process is currently running by reading
// the PID file and sending signal 0. Returns false if the PID file is
// missing, malformed, or the process is dead.
func (r *DaemonRepository) IsAlive() bool {
	pid, err := r.ReadPID()
	if err != nil {
		return false
	}
	proc, err := os.FindProcess(pid)
	if err != nil {
		return false
	}
	return proc.Signal(syscall.Signal(0)) == nil
}

// PIDFileExists reports whether the PID file exists on disk.
func (r *DaemonRepository) PIDFileExists() bool {
	_, err := os.Stat(r.pidPath())
	return err == nil
}

// StopFileExists reports whether the stop file exists on disk.
func (r *DaemonRepository) StopFileExists() bool {
	_, err := os.Stat(r.stopPath())
	return err == nil
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
