//go:build windows

package daemon

import (
	"os"
	"syscall"
)

// shutdownSignals returns the signals that trigger daemon shutdown.
// SIGTSTP is not available on Windows.
var shutdownSignals = []os.Signal{syscall.SIGINT, syscall.SIGTERM}
