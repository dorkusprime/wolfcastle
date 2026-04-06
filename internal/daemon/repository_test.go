package daemon_test

import (
	"path/filepath"
	"testing"

	"github.com/dorkusprime/wolfcastle/internal/daemon"
	"github.com/dorkusprime/wolfcastle/internal/testutil"
)

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

func TestDaemonRepository_RemoveStopFile_Idempotent(t *testing.T) {
	t.Parallel()
	env := testutil.NewEnvironment(t)
	repo := daemon.NewDaemonRepository(env.Root)

	// No stop file exists; RemoveStopFile should succeed silently.
	if err := repo.RemoveStopFile(); err != nil {
		t.Errorf("RemoveStopFile on missing file: %v", err)
	}
}
