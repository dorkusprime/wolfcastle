//go:build windows

// Package signals defines the canonical set of OS signals that trigger
// a graceful shutdown across all wolfcastle components.
package signals

import (
	"os"
	"syscall"
)

// Shutdown is the set of signals that trigger graceful shutdown.
// SIGTSTP is not available on Windows.
var Shutdown = []os.Signal{syscall.SIGINT, syscall.SIGTERM}
