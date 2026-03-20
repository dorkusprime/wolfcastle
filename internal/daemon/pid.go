package daemon

import (
	"os"
	"syscall"
)

// IsProcessRunning checks if a process with the given PID is still alive.
func IsProcessRunning(pid int) bool {
	process, err := os.FindProcess(pid)
	if err != nil {
		return false
	}
	err = process.Signal(syscall.Signal(0))
	return err == nil
}
