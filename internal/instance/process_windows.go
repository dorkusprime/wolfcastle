//go:build windows

package instance

import "golang.org/x/sys/windows"

// isProcessRunning checks if a process with the given PID exists.
// On Windows, os.FindProcess succeeds for any PID, so we open the
// process handle with PROCESS_QUERY_LIMITED_INFORMATION to verify
// it actually exists.
func isProcessRunning(pid int) bool {
	h, err := windows.OpenProcess(windows.PROCESS_QUERY_LIMITED_INFORMATION, false, uint32(pid))
	if err != nil {
		return false
	}
	_ = windows.CloseHandle(h)
	return true
}
