//go:build !windows

// Package signals defines the canonical set of OS signals that trigger
// a graceful shutdown across all wolfcastle components.
package signals

import (
	"os"
	"syscall"
)

// Shutdown is the set of signals that trigger graceful shutdown.
// On Unix, SIGTSTP (Ctrl+Z) is included so foreground mode exits
// cleanly instead of suspending to a stopped background process.
var Shutdown = []os.Signal{syscall.SIGINT, syscall.SIGTERM, syscall.SIGTSTP}
