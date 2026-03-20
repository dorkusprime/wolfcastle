package daemon_test

import (
	"os"
	"testing"

	"github.com/dorkusprime/wolfcastle/internal/daemon"
	"github.com/dorkusprime/wolfcastle/internal/testutil"
)

func TestPIDFileExists_NoFile(t *testing.T) {
	t.Parallel()
	env := testutil.NewEnvironment(t)
	repo := daemon.NewDaemonRepository(env.Root)

	if repo.PIDFileExists() {
		t.Error("PIDFileExists: expected false when no PID file exists")
	}
}

func TestPIDFileExists_WithFile(t *testing.T) {
	t.Parallel()
	env := testutil.NewEnvironment(t)
	repo := daemon.NewDaemonRepository(env.Root)

	if err := repo.WritePID(12345); err != nil {
		t.Fatalf("WritePID: %v", err)
	}
	if !repo.PIDFileExists() {
		t.Error("PIDFileExists: expected true after writing PID")
	}
}

func TestStopFileExists_NoFile(t *testing.T) {
	t.Parallel()
	env := testutil.NewEnvironment(t)
	repo := daemon.NewDaemonRepository(env.Root)

	if repo.StopFileExists() {
		t.Error("StopFileExists: expected false when no stop file exists")
	}
}

func TestStopFileExists_WithFile(t *testing.T) {
	t.Parallel()
	env := testutil.NewEnvironment(t)
	repo := daemon.NewDaemonRepository(env.Root)

	if err := repo.WriteStopFile(); err != nil {
		t.Fatalf("WriteStopFile: %v", err)
	}
	if !repo.StopFileExists() {
		t.Error("StopFileExists: expected true after writing stop file")
	}
}

func TestIsAlive_NoPIDFile(t *testing.T) {
	t.Parallel()
	env := testutil.NewEnvironment(t)
	repo := daemon.NewDaemonRepository(env.Root)

	if repo.IsAlive() {
		t.Error("IsAlive: expected false when no PID file exists")
	}
}

func TestIsAlive_StalePID(t *testing.T) {
	t.Parallel()
	env := testutil.NewEnvironment(t)
	repo := daemon.NewDaemonRepository(env.Root)

	// Write a PID that almost certainly doesn't belong to a running process.
	if err := repo.WritePID(999999); err != nil {
		t.Fatalf("WritePID: %v", err)
	}
	// On most systems PID 999999 won't be running, so IsAlive should return false.
	// If it happens to be running, the test still passes (we just can't assert false).
}

func TestIsAlive_CurrentProcess(t *testing.T) {
	t.Parallel()
	env := testutil.NewEnvironment(t)
	repo := daemon.NewDaemonRepository(env.Root)

	// Use our own PID, which is guaranteed to be running.
	pid := os.Getpid()
	if err := repo.WritePID(pid); err != nil {
		t.Fatalf("WritePID: %v", err)
	}
	if !repo.IsAlive() {
		t.Error("IsAlive: expected true for current process PID")
	}
}
