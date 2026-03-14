//go:build windows

package state

import (
	"fmt"
	"os"
)

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
	proc, err := os.FindProcess(pid)
	if err != nil {
		return err
	}
	// On Windows, Signal(0) is not supported. Assume the process is alive
	// if FindProcess succeeds.
	_ = proc
	return fmt.Errorf("process alive")
}
