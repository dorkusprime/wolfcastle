package daemon

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"
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
func (r *DaemonRepository) WritePID(pid int) error {
	return os.WriteFile(r.pidPath(), []byte(fmt.Sprintf("%d\n", pid)), 0644)
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

// LogDir returns the path to the daemon log directory. This is an
// intentional escape hatch: the Logger manages its own file handles,
// rotation, and compression, so it needs the directory path rather
// than a repository method for each log operation.
func (r *DaemonRepository) LogDir() string {
	return filepath.Join(r.systemDir, "logs")
}
