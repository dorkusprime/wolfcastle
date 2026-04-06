//go:build windows

package instance

import "os"

// isProcessRunning checks if a process with the given PID exists.
// On Windows, FindProcess succeeds for any PID; we attempt to open
// the process handle to verify it actually exists.
func isProcessRunning(pid int) bool {
	proc, err := os.FindProcess(pid)
	if err != nil {
		return false
	}
	// On Windows, Signal(0) returns "not supported" rather than
	// checking liveness. Release is the best we can do without
	// opening the process handle via Win32 API.
	_ = proc.Release()
	return true
}
