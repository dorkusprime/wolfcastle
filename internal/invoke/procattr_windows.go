//go:build windows

package invoke

import "syscall"

// processSysProcAttr returns nil on Windows where Setpgid is not available.
// Process group management on Windows would require CREATE_NEW_PROCESS_GROUP
// but is not critical for correctness.
func processSysProcAttr() *syscall.SysProcAttr {
	return nil
}

// reclaimForeground is a no-op on Windows. The foreground process group
// concept does not apply the same way.
func reclaimForeground() {}
