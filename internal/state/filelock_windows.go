//go:build windows

package state

import "os"

// flockExclusive is a no-op on Windows. File locking on Windows would
// require LockFileEx from the Windows API; for now, advisory locking
// is best-effort on this platform.
func flockExclusive(fd int) error {
	return nil
}

func flockUnlock(fd int) error {
	return nil
}

func signalProcess(pid int) error {
	_, err := os.FindProcess(pid)
	if err != nil {
		return err
	}
	// On Windows, Signal(0) is not supported. FindProcess always succeeds
	// for any PID, so we conservatively assume the process is alive and
	// return nil (no error) to prevent false stale-lock cleanup.
	return nil
}
