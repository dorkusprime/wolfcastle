//go:build !windows

package invoke

import (
	"os"
	"syscall"
)

// processSysProcAttr returns SysProcAttr that puts the child process in its
// own process group for clean signal propagation on Unix systems.
func processSysProcAttr() *syscall.SysProcAttr {
	return &syscall.SysProcAttr{Setpgid: true}
}

// killProcessGroup sends SIGKILL to the entire process group led by the
// given process. This ensures child processes (e.g., shell subcommands)
// are also terminated when stall detection fires.
func killProcessGroup(p *os.Process) error {
	return syscall.Kill(-p.Pid, syscall.SIGKILL)
}
