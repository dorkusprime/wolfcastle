//go:build !windows

package invoke

import "syscall"

// processSysProcAttr returns SysProcAttr that puts the child process in its
// own process group for clean signal propagation on Unix systems.
func processSysProcAttr() *syscall.SysProcAttr {
	return &syscall.SysProcAttr{Setpgid: true}
}
