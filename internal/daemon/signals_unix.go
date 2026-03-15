//go:build !windows

package daemon

import (
	"os"
	"syscall"
)

// shutdownSignals returns the signals that trigger daemon shutdown.
// On Unix, SIGTSTP (Ctrl+Z) is included so foreground mode exits
// cleanly instead of suspending to a stopped background process.
var shutdownSignals = []os.Signal{syscall.SIGINT, syscall.SIGTERM, syscall.SIGTSTP}
