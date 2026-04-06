package daemon_test

import (
	"os"
	"testing"

	"github.com/dorkusprime/wolfcastle/internal/daemon"
	"github.com/dorkusprime/wolfcastle/internal/instance"
	"github.com/dorkusprime/wolfcastle/internal/testutil"
)

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

func TestIsAlive_NoRegistration(t *testing.T) {
	env := testutil.NewEnvironment(t)

	// Point instance registry at a temp dir so we don't collide with real state.
	regDir := t.TempDir()
	old := instance.RegistryDirOverride
	instance.RegistryDirOverride = regDir
	defer func() { instance.RegistryDirOverride = old }()

	repo := daemon.NewDaemonRepository(env.Root)

	if repo.IsAlive() {
		t.Error("IsAlive: expected false when no instance is registered")
	}
}

func TestIsAlive_CurrentProcess(t *testing.T) {
	env := testutil.NewEnvironment(t)

	// Point instance registry at a temp dir.
	regDir := t.TempDir()
	old := instance.RegistryDirOverride
	instance.RegistryDirOverride = regDir
	defer func() { instance.RegistryDirOverride = old }()

	// Register the current process for this worktree.
	pid := os.Getpid()
	_ = pid // used implicitly by instance.Register (registers os.Getpid())
	if err := instance.Register(env.Root, "test-branch"); err != nil {
		t.Fatalf("instance.Register: %v", err)
	}
	defer func() { _ = instance.Deregister(env.Root) }()

	repo := daemon.NewDaemonRepository(env.Root)
	if !repo.IsAlive() {
		t.Error("IsAlive: expected true for current process PID")
	}
}
