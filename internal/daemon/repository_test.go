package daemon_test

import (
	"path/filepath"
	"testing"

	"github.com/dorkusprime/wolfcastle/internal/daemon"
	"github.com/dorkusprime/wolfcastle/internal/testutil"
	"github.com/dorkusprime/wolfcastle/internal/validate"
)

// Compile-time check: *DaemonRepository must satisfy validate.PIDChecker.
var _ validate.PIDChecker = (*daemon.DaemonRepository)(nil)

func TestDaemonRepository_PIDLifecycle(t *testing.T) {
	t.Parallel()
	env := testutil.NewEnvironment(t)
	repo := daemon.NewDaemonRepository(env.Root)

	const pid = 42

	if err := repo.WritePID(pid); err != nil {
		t.Fatalf("WritePID: %v", err)
	}

	got, err := repo.ReadPID()
	if err != nil {
		t.Fatalf("ReadPID: %v", err)
	}
	if got != pid {
		t.Errorf("ReadPID = %d, want %d", got, pid)
	}

	if err := repo.RemovePID(); err != nil {
		t.Fatalf("RemovePID: %v", err)
	}

	_, err = repo.ReadPID()
	if err == nil {
		t.Error("ReadPID after RemovePID: expected error, got nil")
	}
}

func TestDaemonRepository_StopFile(t *testing.T) {
	t.Parallel()
	env := testutil.NewEnvironment(t)
	repo := daemon.NewDaemonRepository(env.Root)

	if repo.HasStopFile() {
		t.Error("HasStopFile: expected false before write")
	}

	if err := repo.WriteStopFile(); err != nil {
		t.Fatalf("WriteStopFile: %v", err)
	}

	if !repo.HasStopFile() {
		t.Error("HasStopFile: expected true after write")
	}

	if err := repo.RemoveStopFile(); err != nil {
		t.Fatalf("RemoveStopFile: %v", err)
	}

	if repo.HasStopFile() {
		t.Error("HasStopFile: expected false after remove")
	}
}

func TestDaemonRepository_LogDir(t *testing.T) {
	t.Parallel()
	env := testutil.NewEnvironment(t)
	repo := daemon.NewDaemonRepository(env.Root)

	want := filepath.Join(env.Root, "system", "logs")
	if got := repo.LogDir(); got != want {
		t.Errorf("LogDir = %q, want %q", got, want)
	}
}

func TestDaemonRepository_ReadPID_Missing(t *testing.T) {
	t.Parallel()
	env := testutil.NewEnvironment(t)
	repo := daemon.NewDaemonRepository(env.Root)

	_, err := repo.ReadPID()
	if err == nil {
		t.Error("ReadPID on empty repo: expected error, got nil")
	}
}

func TestDaemonRepository_RemovePID_Idempotent(t *testing.T) {
	t.Parallel()
	env := testutil.NewEnvironment(t)
	repo := daemon.NewDaemonRepository(env.Root)

	// No PID file exists; RemovePID should succeed silently.
	if err := repo.RemovePID(); err != nil {
		t.Errorf("RemovePID on missing file: %v", err)
	}
}

func TestDaemonRepository_RemoveStopFile_Idempotent(t *testing.T) {
	t.Parallel()
	env := testutil.NewEnvironment(t)
	repo := daemon.NewDaemonRepository(env.Root)

	// No stop file exists; RemoveStopFile should succeed silently.
	if err := repo.RemoveStopFile(); err != nil {
		t.Errorf("RemoveStopFile on missing file: %v", err)
	}
}
