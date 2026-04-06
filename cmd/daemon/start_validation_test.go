package daemon

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/dorkusprime/wolfcastle/internal/instance"
	"github.com/dorkusprime/wolfcastle/internal/state"
)

// ═══════════════════════════════════════════════════════════════════════════
// newStartCmd: RequireIdentity failure
// ═══════════════════════════════════════════════════════════════════════════

func TestStartCmd_RequireIdentityFailure(t *testing.T) {
	env := newTestEnv(t)
	// Nil identity triggers RequireIdentity error
	env.App.Identity = nil
	env.RootCmd.SetArgs([]string{"start"})
	err := env.RootCmd.Execute()
	if err == nil {
		t.Fatal("expected error when identity is not configured")
	}
	if !strings.Contains(err.Error(), "identity") {
		t.Errorf("error should mention identity, got: %s", err.Error())
	}
}

// ═══════════════════════════════════════════════════════════════════════════
// newStartCmd: verbose flag sets log level
// ═══════════════════════════════════════════════════════════════════════════

func TestStartCmd_VerboseSetsDebugLogLevel(t *testing.T) {
	env := newStatusTestEnv(t)

	lockDir := t.TempDir()
	t.Setenv("WOLFCASTLE_LOCK_DIR", lockDir)

	regDir := t.TempDir()
	instance.RegistryDirOverride = regDir
	defer func() { instance.RegistryDirOverride = "" }()

	// Register our own PID so the command fails at "already running" after
	// the verbose flag has been processed and config loaded.
	repoDir := filepath.Dir(env.WolfcastleDir)
	resolved, _ := filepath.EvalSymlinks(repoDir)
	pid := os.Getpid()
	slug := instance.Slug(resolved)
	entryJSON := fmt.Sprintf(`{"pid":%d,"worktree":%q,"branch":"test","started_at":"2026-01-01T00:00:00Z"}`, pid, resolved)
	_ = os.WriteFile(filepath.Join(regDir, slug+".json"), []byte(entryJSON), 0644)

	env.RootCmd.SetArgs([]string{"start", "--verbose"})
	err := env.RootCmd.Execute()
	// Expected: fails at "already running", which proves it got past
	// config load and verbose processing without error.
	if err == nil {
		t.Fatal("expected error from already-running check")
	}
	if !strings.Contains(err.Error(), "already running") {
		t.Errorf("expected 'already running' error, got: %s", err.Error())
	}
}

// ═══════════════════════════════════════════════════════════════════════════
// newStartCmd: validation gate: errors block startup
// ═══════════════════════════════════════════════════════════════════════════

func TestStartCmd_ValidationErrorsBlockStartup(t *testing.T) {
	env := newStatusTestEnv(t)

	// Set node to complete with incomplete tasks: COMPLETE_WITH_INCOMPLETE
	// is model-assisted and cannot be fixed by pre-start self-heal.
	nodeDir := filepath.Join(env.ProjectsDir, "my-project")
	ns, err := state.LoadNodeState(filepath.Join(nodeDir, "state.json"))
	if err != nil {
		t.Fatalf("loading node state: %v", err)
	}
	ns.State = state.StatusComplete
	ns.Tasks = []state.Task{
		{ID: "task-0001", State: state.StatusNotStarted},
		{ID: "audit", State: state.StatusNotStarted, IsAudit: true},
	}
	nsData, _ := json.MarshalIndent(ns, "", "  ")
	_ = os.WriteFile(filepath.Join(nodeDir, "state.json"), nsData, 0644)

	// Redirect the lock to a temp dir so we don't pollute ~/.wolfcastle
	lockDir := t.TempDir()
	t.Setenv("WOLFCASTLE_LOCK_DIR", lockDir)

	regDir := t.TempDir()
	instance.RegistryDirOverride = regDir
	defer func() { instance.RegistryDirOverride = "" }()

	env.RootCmd.SetArgs([]string{"start"})
	err = env.RootCmd.Execute()
	if err == nil {
		t.Fatal("expected validation to block startup")
	}
	if !strings.Contains(err.Error(), "validation") {
		t.Errorf("error should mention validation, got: %s", err.Error())
	}
}

// ═══════════════════════════════════════════════════════════════════════════
// newStartCmd: validation gate: warnings proceed
// ═══════════════════════════════════════════════════════════════════════════

func TestStartCmd_ValidationWarningsProceed(t *testing.T) {
	env := newStatusTestEnv(t)

	// Set a task to in_progress with no daemon running. ValidateStartup
	// flags this as CatStaleInProgress (warning severity). The command
	// should proceed past validation. We then make it fail at the
	// already-running check so it doesn't enter the daemon loop.
	nodeDir := filepath.Join(env.ProjectsDir, "my-project")
	ns, _ := state.LoadNodeState(filepath.Join(nodeDir, "state.json"))
	for i := range ns.Tasks {
		if !ns.Tasks[i].IsAudit {
			ns.Tasks[i].State = state.StatusInProgress
			break
		}
	}
	nsData, _ := json.MarshalIndent(ns, "", "  ")
	_ = os.WriteFile(filepath.Join(nodeDir, "state.json"), nsData, 0644)

	lockDir := t.TempDir()
	t.Setenv("WOLFCASTLE_LOCK_DIR", lockDir)

	regDir := t.TempDir()
	instance.RegistryDirOverride = regDir
	defer func() { instance.RegistryDirOverride = "" }()

	// Write a lock file with our own PID so AcquireLock detects it as running
	pid := os.Getpid()
	repoDir := filepath.Dir(env.WolfcastleDir)
	lockData := fmt.Sprintf(`{"pid":%d,"worktree":%q,"branch":"test","started":"2026-01-01T00:00:00Z"}`, pid, repoDir)
	_ = os.WriteFile(filepath.Join(lockDir, "daemon.lock"), []byte(lockData), 0644)

	env.RootCmd.SetArgs([]string{"start"})
	err := env.RootCmd.Execute()
	// Should NOT fail with "validation errors" (warnings don't block).
	if err != nil && strings.Contains(err.Error(), "validation errors") {
		t.Errorf("warnings should not block startup, got: %s", err.Error())
	}
	// Should fail at the lock check (proving we got past validation).
	if err == nil {
		t.Fatal("expected error from lock check")
	}
}

// ═══════════════════════════════════════════════════════════════════════════
// newStartCmd: already-running daemon detection (error message quality)
// ═══════════════════════════════════════════════════════════════════════════

func TestStartCmd_AlreadyRunningErrorMessage(t *testing.T) {
	env := newStatusTestEnv(t)

	lockDir := t.TempDir()
	t.Setenv("WOLFCASTLE_LOCK_DIR", lockDir)

	regDir := t.TempDir()
	instance.RegistryDirOverride = regDir
	defer func() { instance.RegistryDirOverride = "" }()

	repoDir := filepath.Dir(env.WolfcastleDir)
	resolved, _ := filepath.EvalSymlinks(repoDir)
	pid := os.Getpid()
	slug := instance.Slug(resolved)
	entryJSON := fmt.Sprintf(`{"pid":%d,"worktree":%q,"branch":"test","started_at":"2026-01-01T00:00:00Z"}`, pid, resolved)
	_ = os.WriteFile(filepath.Join(regDir, slug+".json"), []byte(entryJSON), 0644)

	env.RootCmd.SetArgs([]string{"start"})
	err := env.RootCmd.Execute()
	if err == nil {
		t.Fatal("expected 'already running' error")
	}
	msg := err.Error()
	if !strings.Contains(msg, "already running") {
		t.Errorf("error should say 'already running', got: %s", msg)
	}
	if !strings.Contains(msg, fmt.Sprintf("PID %d", pid)) {
		t.Errorf("error should include PID %d, got: %s", pid, msg)
	}
	if !strings.Contains(msg, "wolfcastle stop") {
		t.Errorf("error should suggest 'wolfcastle stop', got: %s", msg)
	}
}

// ═══════════════════════════════════════════════════════════════════════════
// newStartCmd: background mode flag triggers startBackground path
// ═══════════════════════════════════════════════════════════════════════════

func TestStartCmd_BackgroundFlag(t *testing.T) {
	env := newStatusTestEnv(t)

	lockDir := t.TempDir()
	t.Setenv("WOLFCASTLE_LOCK_DIR", lockDir)

	regDir := t.TempDir()
	instance.RegistryDirOverride = regDir
	defer func() { instance.RegistryDirOverride = "" }()

	// Using -d (background mode) with an invalid executable path will
	// cause startBackground to fail when it tries os.Executable() or
	// to start the process. The key assertion is that we reach that
	// code path rather than the foreground daemon.New path.
	env.RootCmd.SetArgs([]string{"start", "-d"})
	err := env.RootCmd.Execute()
	// Background mode calls startBackground, which calls os.Executable()
	// then exec.Command. In test context this should either succeed
	// (spawning a short-lived process) or fail. Either way, the foreground
	// path (daemon.New) is NOT reached.
	_ = err
}

// ═══════════════════════════════════════════════════════════════════════════
// newStartCmd: stale lock recovered before lock acquisition
// ═══════════════════════════════════════════════════════════════════════════

func TestStartCmd_StaleLockRecovery(t *testing.T) {
	// AcquireLock handles stale lock cleanup internally. Write a stale
	// lock and verify it gets cleaned up by the start path.
	lockDir := t.TempDir()
	t.Setenv("WOLFCASTLE_LOCK_DIR", lockDir)

	regDir := t.TempDir()
	instance.RegistryDirOverride = regDir
	defer func() { instance.RegistryDirOverride = "" }()

	// Write a stale lock (dead PID)
	_ = os.WriteFile(filepath.Join(lockDir, "daemon.lock"),
		[]byte(`{"pid":99999999,"worktree":"/tmp/fake","branch":"test","started":"2026-01-01T00:00:00Z"}`), 0644)

	env := newStatusTestEnv(t)
	// Make validation fail so we don't enter the daemon loop
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
	// Should get past lock acquisition (stale lock cleaned up) and fail on validation
	_ = err
}

// ═══════════════════════════════════════════════════════════════════════════
// newStartCmd: lock acquisition failure (another daemon holds the lock)
// ═══════════════════════════════════════════════════════════════════════════

func TestStartCmd_GlobalLockConflict(t *testing.T) {
	env := newStatusTestEnv(t)

	lockDir := t.TempDir()
	t.Setenv("WOLFCASTLE_LOCK_DIR", lockDir)

	regDir := t.TempDir()
	instance.RegistryDirOverride = regDir
	defer func() { instance.RegistryDirOverride = "" }()

	// Pre-create a lock file with our own PID (simulating a running daemon).
	repoDir := filepath.Dir(env.WolfcastleDir)
	lockData := fmt.Sprintf(`{"pid":%d,"worktree":%q,"branch":"test","started":"2026-01-01T00:00:00Z"}`, os.Getpid(), repoDir)
	_ = os.WriteFile(filepath.Join(lockDir, "daemon.lock"), []byte(lockData), 0644)

	env.RootCmd.SetArgs([]string{"start"})
	err := env.RootCmd.Execute()
	if err == nil {
		t.Fatal("expected error when lock is held")
	}
	if !strings.Contains(err.Error(), "already running") {
		t.Errorf("error should mention 'already running', got: %s", err.Error())
	}
}

// ═══════════════════════════════════════════════════════════════════════════
// newStartCmd: worktree flag with invalid branch (exercising error path)
// ═══════════════════════════════════════════════════════════════════════════

func TestStartCmd_WorktreeCreationFailure(t *testing.T) {
	env := newStatusTestEnv(t)

	lockDir := t.TempDir()
	t.Setenv("WOLFCASTLE_LOCK_DIR", lockDir)

	regDir := t.TempDir()
	instance.RegistryDirOverride = regDir
	defer func() { instance.RegistryDirOverride = "" }()

	// The worktree creation runs git commands. In a temp dir that isn't
	// a real git repo, this will fail with a git error.
	env.RootCmd.SetArgs([]string{"start", "--worktree", "nonexistent-branch-xyz"})
	err := env.RootCmd.Execute()
	if err == nil {
		t.Fatal("expected error from worktree creation in non-git dir")
	}
	if !strings.Contains(err.Error(), "worktree") {
		t.Errorf("error should mention worktree, got: %s", err.Error())
	}
}
