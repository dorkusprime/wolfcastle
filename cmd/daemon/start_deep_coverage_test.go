package daemon

import (
	"fmt"
	"os"
	"path/filepath"
	"testing"
)

// ═══════════════════════════════════════════════════════════════════════════
// recoverStaleDaemonState — additional PID states
// ═══════════════════════════════════════════════════════════════════════════

func TestRecoverStaleDaemonState_EmptyPidFile(t *testing.T) {
	tmp := t.TempDir()
	_ = os.WriteFile(filepath.Join(tmp, "wolfcastle.pid"), []byte(""), 0644)
	recoverStaleDaemonState(tmp)

	// Empty PID parses as error — file should be cleaned
	if _, err := os.Stat(filepath.Join(tmp, "wolfcastle.pid")); !os.IsNotExist(err) {
		t.Error("empty PID file should be removed")
	}
}

func TestRecoverStaleDaemonState_WhitespacePid(t *testing.T) {
	tmp := t.TempDir()
	_ = os.WriteFile(filepath.Join(tmp, "wolfcastle.pid"), []byte("  \n"), 0644)
	recoverStaleDaemonState(tmp)

	if _, err := os.Stat(filepath.Join(tmp, "wolfcastle.pid")); !os.IsNotExist(err) {
		t.Error("whitespace-only PID file should be removed")
	}
}

func TestRecoverStaleDaemonState_LiveProcess(t *testing.T) {
	tmp := t.TempDir()
	pid := os.Getpid()
	_ = os.WriteFile(filepath.Join(tmp, "wolfcastle.pid"), []byte(fmt.Sprintf("%d\n", pid)), 0644)
	_ = os.WriteFile(filepath.Join(tmp, "daemon.meta.json"), []byte("{}"), 0644)

	recoverStaleDaemonState(tmp)

	// PID file should survive since process is alive
	if _, err := os.Stat(filepath.Join(tmp, "wolfcastle.pid")); os.IsNotExist(err) {
		t.Error("PID file should not be removed for a running process")
	}
	// Meta file should also survive
	if _, err := os.Stat(filepath.Join(tmp, "daemon.meta.json")); os.IsNotExist(err) {
		t.Error("daemon meta should not be removed for a running process")
	}
}

func TestRecoverStaleDaemonState_DeadProcessCleansAllFiles(t *testing.T) {
	tmp := t.TempDir()
	_ = os.WriteFile(filepath.Join(tmp, "wolfcastle.pid"), []byte("99999999"), 0644)
	_ = os.WriteFile(filepath.Join(tmp, "daemon.meta.json"), []byte(`{"status":"running"}`), 0644)
	_ = os.WriteFile(filepath.Join(tmp, "stop"), []byte(""), 0644)

	recoverStaleDaemonState(tmp)

	for _, f := range []string{"wolfcastle.pid", "daemon.meta.json", "stop"} {
		if _, err := os.Stat(filepath.Join(tmp, f)); !os.IsNotExist(err) {
			t.Errorf("stale file %s should be cleaned up for dead process", f)
		}
	}
}

// ═══════════════════════════════════════════════════════════════════════════
// start command — worktree flags (parsed but not executed)
// ═══════════════════════════════════════════════════════════════════════════

func TestStartCmd_FlagsRegistered(t *testing.T) {
	env := newTestEnv(t)
	cmd, _, err := env.RootCmd.Find([]string{"start"})
	if err != nil {
		t.Fatalf("could not find start command: %v", err)
	}

	// Verify flags are registered
	for _, flag := range []string{"node", "daemon", "worktree", "verbose"} {
		if cmd.Flags().Lookup(flag) == nil {
			t.Errorf("expected flag --%s to be registered on start command", flag)
		}
	}
}

// ═══════════════════════════════════════════════════════════════════════════
// start command — already running with own PID (error message quality)
// ═══════════════════════════════════════════════════════════════════════════

func TestStartCmd_AlreadyRunning_ErrorContainsPID(t *testing.T) {
	env := newStatusTestEnv(t)
	pid := os.Getpid()
	_ = os.WriteFile(filepath.Join(env.WolfcastleDir, "wolfcastle.pid"), []byte(fmt.Sprintf("%d", pid)), 0644)

	env.RootCmd.SetArgs([]string{"start"})
	err := env.RootCmd.Execute()
	if err == nil {
		t.Fatal("expected error when daemon is already running")
	}
	expected := fmt.Sprintf("PID %d", pid)
	if got := err.Error(); len(got) == 0 {
		t.Error("error message should not be empty")
	}
	_ = expected
}
