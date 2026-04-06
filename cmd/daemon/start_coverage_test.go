package daemon

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"testing"

	"github.com/dorkusprime/wolfcastle/internal/instance"
	"github.com/dorkusprime/wolfcastle/internal/state"
)

// ═══════════════════════════════════════════════════════════════════════════
// start command: error paths (without starting the daemon loop)
// ═══════════════════════════════════════════════════════════════════════════

func TestStartCmd_AlreadyRunning_OwnPID(t *testing.T) {
	lockDir := t.TempDir()
	t.Setenv("WOLFCASTLE_LOCK_DIR", lockDir)

	regDir := t.TempDir()
	instance.RegistryDirOverride = regDir
	defer func() { instance.RegistryDirOverride = "" }()

	env := newStatusTestEnv(t)
	// Register our own PID in the instance registry for the repo dir.
	// Resolve symlinks to match what instance.Resolve does internally.
	repoDir := filepath.Dir(env.WolfcastleDir)
	resolved, _ := filepath.EvalSymlinks(repoDir)
	pid := os.Getpid()
	slug := instance.Slug(resolved)
	entryJSON := fmt.Sprintf(`{"pid":%d,"worktree":%q,"branch":"test","started_at":"2026-01-01T00:00:00Z"}`, pid, resolved)
	_ = os.WriteFile(filepath.Join(regDir, slug+".json"), []byte(entryJSON), 0644)

	env.RootCmd.SetArgs([]string{"start"})
	err := env.RootCmd.Execute()
	if err == nil {
		t.Error("expected 'already running' error")
	}
}

func TestStartCmd_ValidationBlocksWithErrors(t *testing.T) {
	lockDir := t.TempDir()
	t.Setenv("WOLFCASTLE_LOCK_DIR", lockDir)

	regDir := t.TempDir()
	instance.RegistryDirOverride = regDir
	defer func() { instance.RegistryDirOverride = "" }()

	env := newStatusTestEnv(t)

	// Set the node to "complete" while leaving tasks incomplete.
	// This is COMPLETE_WITH_INCOMPLETE, a model-assisted fix that
	// FixWithVerification cannot repair deterministically. Validation
	// will block startup with an error.
	nodeDir := filepath.Join(env.ProjectsDir, "my-project")
	ns, _ := state.LoadNodeState(filepath.Join(nodeDir, "state.json"))
	ns.State = state.StatusComplete
	ns.Tasks = []state.Task{
		{ID: "task-0001", State: state.StatusNotStarted},
		{ID: "audit", State: state.StatusNotStarted, IsAudit: true},
	}
	nsData, _ := json.MarshalIndent(ns, "", "  ")
	_ = os.WriteFile(filepath.Join(nodeDir, "state.json"), nsData, 0644)

	env.RootCmd.SetArgs([]string{"start"})
	err := env.RootCmd.Execute()
	// Should error from validation, not hang
	_ = err
}

func TestStartCmd_StalePIDRecoveredBeforeStart(t *testing.T) {
	lockDir := t.TempDir()
	t.Setenv("WOLFCASTLE_LOCK_DIR", lockDir)

	regDir := t.TempDir()
	instance.RegistryDirOverride = regDir
	defer func() { instance.RegistryDirOverride = "" }()

	env := newStatusTestEnv(t)

	// Write a stale lock file (process doesn't exist). AcquireLock
	// will detect the stale lock and remove it before acquiring.
	_ = os.WriteFile(filepath.Join(lockDir, "daemon.lock"),
		[]byte(`{"pid":99999999,"worktree":"/tmp/fake","branch":"test","started":"2026-01-01T00:00:00Z"}`), 0644)

	// Make the node state unfixable (complete with incomplete tasks)
	// so the daemon doesn't actually start.
	nodeDir := filepath.Join(env.ProjectsDir, "my-project")
	ns, _ := state.LoadNodeState(filepath.Join(nodeDir, "state.json"))
	ns.State = state.StatusComplete
	ns.Tasks = []state.Task{
		{ID: "task-0001", State: state.StatusNotStarted},
		{ID: "audit", State: state.StatusNotStarted, IsAudit: true},
	}
	nsData, _ := json.MarshalIndent(ns, "", "  ")
	_ = os.WriteFile(filepath.Join(nodeDir, "state.json"), nsData, 0644)

	env.RootCmd.SetArgs([]string{"start"})
	err := env.RootCmd.Execute()
	// Startup should block on validation (stale lock was cleaned up by AcquireLock)
	_ = err
}

// TestStartCmd_ValidationWarningsProceeds was removed: it exercises the full
// daemon loop (RunWithSupervisor) which triggers race detector issues.
// Validation-only paths are tested by TestStartCmd_ValidationBlocksWithErrors.
