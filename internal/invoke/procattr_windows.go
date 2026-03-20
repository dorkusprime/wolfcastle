//go:build windows

package invoke

import (
	"os"
	"syscall"
)

// processSysProcAttr returns nil on Windows where Setpgid is not available.
// Process group management on Windows would require CREATE_NEW_PROCESS_GROUP
// but is not critical for correctness.
func processSysProcAttr() *syscall.SysProcAttr {
	return nil
}

// killProcessGroup kills the process on Windows. True process group
// termination would require CREATE_NEW_PROCESS_GROUP and
// GenerateConsoleCtrlEvent, but a simple kill is sufficient for
// stall detection purposes.
func killProcessGroup(p *os.Process) error {
	return p.Kill()
}
