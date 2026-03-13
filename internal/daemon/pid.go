package daemon

import (
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"syscall"
)

// WritePID writes the current process PID to a file.
func WritePID(wolfcastleDir string) error {
	path := filepath.Join(wolfcastleDir, "wolfcastle.pid")
	return os.WriteFile(path, []byte(fmt.Sprintf("%d\n", os.Getpid())), 0644)
}

// ReadPID reads the PID from the PID file.
func ReadPID(wolfcastleDir string) (int, error) {
	path := filepath.Join(wolfcastleDir, "wolfcastle.pid")
	data, err := os.ReadFile(path)
	if err != nil {
		return 0, err
	}
	return strconv.Atoi(strings.TrimSpace(string(data)))
}

// RemovePID removes the PID file.
func RemovePID(wolfcastleDir string) {
	os.Remove(filepath.Join(wolfcastleDir, "wolfcastle.pid"))
}

// IsProcessRunning checks if a process with the given PID is still alive.
func IsProcessRunning(pid int) bool {
	process, err := os.FindProcess(pid)
	if err != nil {
		return false
	}
	err = process.Signal(syscall.Signal(0))
	return err == nil
}
