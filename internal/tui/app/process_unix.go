//go:build !windows

package app

import (
	"os"
	"syscall"
)

// killProcess sends the given signal to a process by PID.
func killProcess(pid int, sig syscall.Signal) error {
	return syscall.Kill(pid, sig)
}

// isProcessAlive checks whether a process exists by sending signal 0.
func isProcessAlive(pid int) bool {
	if pid <= 0 {
		return false
	}
	p, err := os.FindProcess(pid)
	if err != nil {
		return false
	}
	return p.Signal(syscall.Signal(0)) == nil
}
