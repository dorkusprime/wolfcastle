//go:build windows

package app

import (
	"fmt"
	"syscall"
)

// killProcess is a stub on Windows where syscall.Kill is unavailable.
func killProcess(_ int, _ syscall.Signal) error {
	return fmt.Errorf("sending signals is not supported on Windows")
}

// isProcessAlive is a stub on Windows; always returns false.
func isProcessAlive(_ int) bool {
	return false
}
