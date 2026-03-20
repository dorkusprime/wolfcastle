//go:build !windows

package daemon

import (
	"os"

	"github.com/dorkusprime/wolfcastle/internal/signals"
)

// shutdownSignals are the signals that trigger daemon shutdown.
var shutdownSignals []os.Signal = signals.Shutdown
