//go:build !windows

package instance

import (
	"os"
	"syscall"
)

// isProcessRunning checks if a process with the given PID exists
// by sending signal 0 (no-op signal that checks for process existence).
func isProcessRunning(pid int) bool {
	proc, err := os.FindProcess(pid)
	if err != nil {
		return false
	}
	return proc.Signal(syscall.Signal(0)) == nil
}
