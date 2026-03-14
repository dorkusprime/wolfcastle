//go:build !windows

package state

import (
	"os"
	"syscall"
)

func flockExclusive(fd int) error {
	return syscall.Flock(fd, syscall.LOCK_EX|syscall.LOCK_NB)
}

func flockUnlock(fd int) error {
	return syscall.Flock(fd, syscall.LOCK_UN)
}

func signalProcess(pid int) error {
	proc, err := os.FindProcess(pid)
	if err != nil {
		return err
	}
	return proc.Signal(syscall.Signal(0))
}
