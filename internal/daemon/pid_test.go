package daemon

import (
	"os"
	"testing"
)

func TestIsProcessRunning_CurrentProcess(t *testing.T) {
	t.Parallel()
	if !IsProcessRunning(os.Getpid()) {
		t.Error("current process should be running")
	}
}

func TestIsProcessRunning_DeadProcess(t *testing.T) {
	t.Parallel()
	// PID 99999999 is extremely unlikely to be running
	if IsProcessRunning(99999999) {
		t.Error("PID 99999999 should not be running")
	}
}
