package signals

import (
	"syscall"
	"testing"
)

func TestShutdown_ContainsSIGINTAndSIGTERM(t *testing.T) {
	t.Parallel()

	if len(Shutdown) == 0 {
		t.Fatal("Shutdown signal list is empty")
	}

	hasSIGINT := false
	hasSIGTERM := false
	for _, sig := range Shutdown {
		switch sig {
		case syscall.SIGINT:
			hasSIGINT = true
		case syscall.SIGTERM:
			hasSIGTERM = true
		}
	}

	if !hasSIGINT {
		t.Error("Shutdown should contain SIGINT")
	}
	if !hasSIGTERM {
		t.Error("Shutdown should contain SIGTERM")
	}
}
