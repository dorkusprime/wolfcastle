package daemon

import (
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/spf13/cobra"

	"github.com/dorkusprime/wolfcastle/cmd/cmdutil"
	dmn "github.com/dorkusprime/wolfcastle/internal/daemon"
	"github.com/dorkusprime/wolfcastle/internal/instance"
	"github.com/dorkusprime/wolfcastle/internal/tierfs"
)

// setupRegistry creates an isolated instance registry directory and
// sets RegistryDirOverride. The caller must NOT use t.Parallel() since
// RegistryDirOverride is a package-level variable.
func setupRegistry(t *testing.T) string {
	t.Helper()
	raw := t.TempDir()
	regDir, err := filepath.EvalSymlinks(raw)
	if err != nil {
		t.Fatalf("resolving temp dir: %v", err)
	}
	instance.RegistryDirOverride = regDir
	t.Cleanup(func() { instance.RegistryDirOverride = "" })
	return regDir
}

// writeEntry writes an instance.Entry directly to the registry directory,
// bypassing Register (which only writes the current PID). This allows
// tests to plant entries with arbitrary PIDs.
func writeEntry(t *testing.T, regDir string, entry instance.Entry) {
	t.Helper()
	data, err := json.Marshal(entry)
	if err != nil {
		t.Fatalf("marshaling entry: %v", err)
	}
	slug := instance.Slug(entry.Worktree)
	if err := os.WriteFile(filepath.Join(regDir, slug+".json"), data, 0644); err != nil {
		t.Fatalf("writing entry file: %v", err)
	}
}

// resolvedTempDir creates a temp directory and returns its symlink-resolved
// path. On macOS, /tmp is a symlink to /private/var/folders/..., so raw
// TempDir paths won't match what EvalSymlinks produces inside Register/Resolve.
func resolvedTempDir(t *testing.T) string {
	t.Helper()
	raw := t.TempDir()
	resolved, err := filepath.EvalSymlinks(raw)
	if err != nil {
		t.Fatalf("resolving temp dir: %v", err)
	}
	return resolved
}

// ---------------------------------------------------------------------------
// (a) Two instances coexist
// ---------------------------------------------------------------------------

func TestMultiProcess_TwoInstancesCoexist(t *testing.T) {
	regDir := setupRegistry(t)
	base := resolvedTempDir(t)

	// Two separate "worktrees," each with a .wolfcastle/ directory.
	repoA := filepath.Join(base, "repo-alpha")
	repoB := filepath.Join(base, "repo-beta")
	_ = os.MkdirAll(filepath.Join(repoA, ".wolfcastle"), 0755)
	_ = os.MkdirAll(filepath.Join(repoB, ".wolfcastle"), 0755)

	// Start two long-lived subprocesses to act as live daemons.
	sleepA := exec.Command("sleep", "60")
	if err := sleepA.Start(); err != nil {
		t.Fatalf("starting sleep A: %v", err)
	}
	defer func() { _ = sleepA.Process.Kill(); _ = sleepA.Wait() }()

	sleepB := exec.Command("sleep", "60")
	if err := sleepB.Start(); err != nil {
		t.Fatalf("starting sleep B: %v", err)
	}
	defer func() { _ = sleepB.Process.Kill(); _ = sleepB.Wait() }()

	writeEntry(t, regDir, instance.Entry{
		PID:       sleepA.Process.Pid,
		Worktree:  repoA,
		Branch:    "feat/alpha",
		StartedAt: time.Now().UTC(),
	})
	writeEntry(t, regDir, instance.Entry{
		PID:       sleepB.Process.Pid,
		Worktree:  repoB,
		Branch:    "feat/beta",
		StartedAt: time.Now().UTC(),
	})

	// Status from repo-alpha resolves to the alpha instance.
	entryA, err := instance.Resolve(repoA)
	if err != nil {
		t.Fatalf("Resolve(repoA): %v", err)
	}
	if entryA.Branch != "feat/alpha" {
		t.Errorf("repoA resolved to %q, want feat/alpha", entryA.Branch)
	}

	// Status from repo-beta resolves to the beta instance.
	entryB, err := instance.Resolve(repoB)
	if err != nil {
		t.Fatalf("Resolve(repoB): %v", err)
	}
	if entryB.Branch != "feat/beta" {
		t.Errorf("repoB resolved to %q, want feat/beta", entryB.Branch)
	}

	// stop --all should find both.
	all, err := instance.List()
	if err != nil {
		t.Fatalf("List: %v", err)
	}
	if len(all) != 2 {
		t.Errorf("expected 2 instances for stop --all, got %d", len(all))
	}
}

// ---------------------------------------------------------------------------
// (b) CWD routing from subdirectory
// ---------------------------------------------------------------------------

func TestMultiProcess_CWDRoutingFromSubdirectory(t *testing.T) {
	regDir := setupRegistry(t)
	base := resolvedTempDir(t)

	repo := filepath.Join(base, "repo")
	subdir := filepath.Join(repo, "src", "pkg")
	_ = os.MkdirAll(subdir, 0755)

	writeEntry(t, regDir, instance.Entry{
		PID:       os.Getpid(),
		Worktree:  repo,
		Branch:    "main",
		StartedAt: time.Now().UTC(),
	})

	// Resolve from a deep subdirectory should route to the repo instance.
	got, err := instance.Resolve(subdir)
	if err != nil {
		t.Fatalf("Resolve(subdir): %v", err)
	}
	if got.Worktree != repo {
		t.Errorf("expected worktree %q, got %q", repo, got.Worktree)
	}
	if got.Branch != "main" {
		t.Errorf("expected branch main, got %q", got.Branch)
	}
}

// ---------------------------------------------------------------------------
// (c) Boundary check prevents cross-match
// ---------------------------------------------------------------------------

func TestMultiProcess_BoundaryPreventsCrossMatch(t *testing.T) {
	regDir := setupRegistry(t)
	base := resolvedTempDir(t)

	auth := filepath.Join(base, "feat", "auth")
	authV2 := filepath.Join(base, "feat", "auth-v2")
	_ = os.MkdirAll(auth, 0755)
	_ = os.MkdirAll(authV2, 0755)

	writeEntry(t, regDir, instance.Entry{
		PID:       os.Getpid(),
		Worktree:  auth,
		Branch:    "feat/auth",
		StartedAt: time.Now().UTC(),
	})

	// auth-v2 must NOT match the auth instance. The path "auth-v2"
	// shares the prefix "auth" but lacks a separator boundary.
	_, err := instance.Resolve(authV2)
	if err == nil {
		t.Error("auth-v2 should not match auth instance; boundary check failed")
	}
}

// ---------------------------------------------------------------------------
// (d) --instance flag overrides CWD
// ---------------------------------------------------------------------------

func TestMultiProcess_InstanceFlagOverridesCWD(t *testing.T) {
	regDir := setupRegistry(t)
	base := resolvedTempDir(t)

	otherRepo := filepath.Join(base, "other-repo")
	_ = os.MkdirAll(otherRepo, 0755)

	// Start a subprocess so we have a real live PID to stop.
	sleepCmd := exec.Command("sleep", "60")
	if err := sleepCmd.Start(); err != nil {
		t.Fatalf("starting sleep: %v", err)
	}
	defer func() { _ = sleepCmd.Process.Kill(); _ = sleepCmd.Wait() }()

	writeEntry(t, regDir, instance.Entry{
		PID:       sleepCmd.Process.Pid,
		Worktree:  otherRepo,
		Branch:    "feat/other",
		StartedAt: time.Now().UTC(),
	})

	// Resolve using the explicit --instance path. This simulates what
	// the stop command does when --instance is set: it passes the path
	// to Resolve instead of using os.Getwd().
	entry, err := instance.Resolve(otherRepo)
	if err != nil {
		t.Fatalf("Resolve(otherRepo): %v", err)
	}
	if entry.PID != sleepCmd.Process.Pid {
		t.Errorf("expected PID %d, got %d", sleepCmd.Process.Pid, entry.PID)
	}
	if entry.Branch != "feat/other" {
		t.Errorf("expected branch feat/other, got %q", entry.Branch)
	}

	// Verify that stopInstance can target it (sends SIGTERM to the sleep).
	if err := stopInstance(entry, false, false); err != nil {
		t.Fatalf("stopInstance via --instance override: %v", err)
	}
}

// ---------------------------------------------------------------------------
// (e) Tier regeneration
// ---------------------------------------------------------------------------

func TestMultiProcess_EnsureTiers(t *testing.T) {
	base := resolvedTempDir(t)

	wcDir := filepath.Join(base, ".wolfcastle")
	systemDir := filepath.Join(wcDir, tierfs.SystemPrefix)
	customDir := filepath.Join(systemDir, "custom")
	_ = os.MkdirAll(customDir, 0755)

	// Write a tracked config.json in the custom tier (simulating what
	// git checkout brings into a worktree).
	customCfg := map[string]any{"version": 1}
	data, _ := json.Marshal(customCfg)
	_ = os.WriteFile(filepath.Join(customDir, "config.json"), data, 0644)

	// The base tier does NOT exist yet, which is the trigger for
	// ensureTiers to regenerate.
	baseCfg := filepath.Join(systemDir, tierfs.TierNames[0], "config.json")
	if _, err := os.Stat(baseCfg); err == nil {
		t.Fatal("base config should not exist before ensureTiers")
	}

	if err := ensureTiers(wcDir); err != nil {
		t.Fatalf("ensureTiers: %v", err)
	}

	// After regeneration, the base tier config should exist.
	if _, err := os.Stat(baseCfg); err != nil {
		t.Errorf("base config should exist after ensureTiers: %v", err)
	}
}

// ensureTiers must be a no-op when the base tier already exists. This
// covers the early-return branch and prevents an unnecessary call to
// ScaffoldService.Reinit on every start.
func TestEnsureTiers_NoOpWhenBaseTierPresent(t *testing.T) {
	base := resolvedTempDir(t)
	wcDir := filepath.Join(base, ".wolfcastle")
	baseCfgDir := filepath.Join(wcDir, tierfs.SystemPrefix, tierfs.TierNames[0])
	if err := os.MkdirAll(baseCfgDir, 0755); err != nil {
		t.Fatal(err)
	}
	baseCfg := filepath.Join(baseCfgDir, "config.json")
	if err := os.WriteFile(baseCfg, []byte(`{"version":1}`), 0644); err != nil {
		t.Fatal(err)
	}
	mtimeBefore, err := os.Stat(baseCfg)
	if err != nil {
		t.Fatal(err)
	}

	if err := ensureTiers(wcDir); err != nil {
		t.Fatalf("ensureTiers should be a no-op: %v", err)
	}

	mtimeAfter, err := os.Stat(baseCfg)
	if err != nil {
		t.Fatal(err)
	}
	if !mtimeBefore.ModTime().Equal(mtimeAfter.ModTime()) {
		t.Error("base config was rewritten on a no-op call")
	}
}

// ensureTiers must also be a no-op when .wolfcastle doesn't exist at
// all. This is the "user hasn't run init" case — the daemon should
// fall through to the normal config-load error rather than fabricate
// an empty scaffold.
func TestEnsureTiers_NoOpWhenWcDirMissing(t *testing.T) {
	base := resolvedTempDir(t)
	wcDir := filepath.Join(base, ".wolfcastle") // never created
	if err := ensureTiers(wcDir); err != nil {
		t.Errorf("ensureTiers on a missing .wolfcastle should be a no-op, got: %v", err)
	}
	if _, err := os.Stat(wcDir); !os.IsNotExist(err) {
		t.Error("ensureTiers must not create .wolfcastle from nothing")
	}
}

// resolveInstance is a no-op (returns nil) when --instance is unset,
// leaving the App unchanged.
func TestResolveInstance_NoFlagIsNoOp(t *testing.T) {
	cmd := &cobra.Command{}
	cmd.Flags().String("instance", "", "")
	app := &cmdutil.App{}
	if err := resolveInstance(cmd, app); err != nil {
		t.Errorf("expected nil error for empty --instance, got: %v", err)
	}
	if app.Config != nil {
		t.Error("App.Config should remain nil when --instance is unset")
	}
}

// resolveInstance forwards to InitFromDir and surfaces its error when
// --instance points at a path with no .wolfcastle.
func TestResolveInstance_ForwardsInitFromDirError(t *testing.T) {
	tmp := t.TempDir()
	cmd := &cobra.Command{}
	cmd.Flags().String("instance", "", "")
	if err := cmd.Flags().Set("instance", filepath.Join(tmp, "no-such-worktree")); err != nil {
		t.Fatal(err)
	}
	app := &cmdutil.App{}
	if err := resolveInstance(cmd, app); err == nil {
		t.Error("expected error when --instance points at a missing worktree")
	}
}

// resolveInstance with a valid --instance points the App at that
// worktree's .wolfcastle directory.
func TestResolveInstance_PointsAppAtTarget(t *testing.T) {
	tmp := t.TempDir()
	wcDir := filepath.Join(tmp, ".wolfcastle")
	if err := os.MkdirAll(filepath.Join(wcDir, "system", "base"), 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(wcDir, "system", "base", "config.json"), []byte(`{"version":1}`), 0644); err != nil {
		t.Fatal(err)
	}

	cmd := &cobra.Command{}
	cmd.Flags().String("instance", "", "")
	if err := cmd.Flags().Set("instance", tmp); err != nil {
		t.Fatal(err)
	}

	app := &cmdutil.App{}
	if err := resolveInstance(cmd, app); err != nil {
		t.Fatalf("resolveInstance failed: %v", err)
	}
	if app.Config == nil {
		t.Fatal("App.Config should be wired up after a successful resolveInstance")
	}
}

// getDaemonStatus with an explicit repoDir resolves the instance from
// that path instead of os.Getwd, which is the whole reason the
// signature changed from variadic to required.
func TestGetDaemonStatus_ExplicitRepoDir(t *testing.T) {
	regDir := setupRegistry(t)
	base := resolvedTempDir(t)

	repo := filepath.Join(base, "explicit-repo")
	if err := os.MkdirAll(repo, 0755); err != nil {
		t.Fatal(err)
	}

	// Use this test process's PID as the "live" entry. IsProcessRunning
	// returns true for the running test binary so the status string
	// reads "running".
	writeEntry(t, regDir, instance.Entry{
		PID:       os.Getpid(),
		Worktree:  repo,
		Branch:    "main",
		StartedAt: time.Now().UTC(),
	})

	repoObj := dmn.NewDaemonRepository(filepath.Join(repo, ".wolfcastle"))
	status := getDaemonStatus(repoObj, repo)
	if !strings.HasPrefix(status, "running") {
		t.Errorf("expected status to start with 'running', got %q", status)
	}
}

// getDaemonStatus with empty repoDir falls back to os.Getwd. We can't
// easily change CWD without affecting other tests, so just verify the
// function doesn't panic and returns the stopped status when the
// registry has nothing matching the test's actual CWD.
func TestGetDaemonStatus_EmptyRepoDirFallsBackToCWD(t *testing.T) {
	_ = setupRegistry(t) // empty registry
	repo := dmn.NewDaemonRepository(t.TempDir())
	status := getDaemonStatus(repo, "")
	if status != "stopped" {
		t.Errorf("expected 'stopped' for an empty registry, got %q", status)
	}
}

// ---------------------------------------------------------------------------
// (f) stop --all with mixed live/stale
// ---------------------------------------------------------------------------

func TestMultiProcess_StopAllMixedLiveStale(t *testing.T) {
	regDir := setupRegistry(t)
	base := resolvedTempDir(t)

	liveRepo := filepath.Join(base, "live-repo")
	staleRepo := filepath.Join(base, "stale-repo")
	_ = os.MkdirAll(liveRepo, 0755)
	_ = os.MkdirAll(staleRepo, 0755)

	// Start a real subprocess for the "live" entry.
	sleepCmd := exec.Command("sleep", "60")
	if err := sleepCmd.Start(); err != nil {
		t.Fatalf("starting sleep: %v", err)
	}
	defer func() { _ = sleepCmd.Process.Kill(); _ = sleepCmd.Wait() }()

	writeEntry(t, regDir, instance.Entry{
		PID:       sleepCmd.Process.Pid,
		Worktree:  liveRepo,
		Branch:    "feat/live",
		StartedAt: time.Now().UTC(),
	})

	// Plant a stale entry with a PID that does not exist.
	writeEntry(t, regDir, instance.Entry{
		PID:       999999999,
		Worktree:  staleRepo,
		Branch:    "feat/stale",
		StartedAt: time.Now().UTC(),
	})

	// Before stopAllInstances, verify the stale file is on disk.
	staleSlug := instance.Slug(staleRepo)
	stalePath := filepath.Join(regDir, staleSlug+".json")
	if _, err := os.Stat(stalePath); err != nil {
		t.Fatal("stale entry file should exist before stopAll")
	}

	// List() auto-cleans stale entries. After listing, only the live one remains.
	entries, err := instance.List()
	if err != nil {
		t.Fatalf("List: %v", err)
	}
	if len(entries) != 1 {
		t.Fatalf("expected 1 live entry after stale cleanup, got %d", len(entries))
	}
	if entries[0].Worktree != liveRepo {
		t.Errorf("expected live repo %q, got %q", liveRepo, entries[0].Worktree)
	}

	// The stale registry file should have been removed by List().
	if _, err := os.Stat(stalePath); !os.IsNotExist(err) {
		t.Error("stale entry file should have been cleaned by List()")
	}

	// stopAllInstances should send a signal to the live one.
	err = stopAllInstances(false, false)
	if err != nil {
		t.Fatalf("stopAllInstances: %v", err)
	}
}

// ---------------------------------------------------------------------------
// stop --all through the cobra command
// ---------------------------------------------------------------------------

func TestMultiProcess_StopAllCobra(t *testing.T) {
	regDir := setupRegistry(t)
	base := resolvedTempDir(t)

	repo := filepath.Join(base, "repo")
	_ = os.MkdirAll(repo, 0755)

	sleepCmd := exec.Command("sleep", "60")
	if err := sleepCmd.Start(); err != nil {
		t.Fatalf("starting sleep: %v", err)
	}
	defer func() { _ = sleepCmd.Process.Kill(); _ = sleepCmd.Wait() }()

	writeEntry(t, regDir, instance.Entry{
		PID:       sleepCmd.Process.Pid,
		Worktree:  repo,
		Branch:    "main",
		StartedAt: time.Now().UTC(),
	})

	env := newTestEnv(t)
	// Override the registry again since newTestEnv doesn't preserve ours.
	instance.RegistryDirOverride = regDir
	env.RootCmd.SetArgs([]string{"stop", "--all"})
	if err := env.RootCmd.Execute(); err != nil {
		t.Fatalf("stop --all via cobra: %v", err)
	}
}

// ---------------------------------------------------------------------------
// stop --instance via cobra command
// ---------------------------------------------------------------------------

func TestMultiProcess_StopInstanceFlagCobra(t *testing.T) {
	regDir := setupRegistry(t)
	base := resolvedTempDir(t)

	otherRepo := filepath.Join(base, "other-repo")
	_ = os.MkdirAll(otherRepo, 0755)

	sleepCmd := exec.Command("sleep", "60")
	if err := sleepCmd.Start(); err != nil {
		t.Fatalf("starting sleep: %v", err)
	}
	defer func() { _ = sleepCmd.Process.Kill(); _ = sleepCmd.Wait() }()

	writeEntry(t, regDir, instance.Entry{
		PID:       sleepCmd.Process.Pid,
		Worktree:  otherRepo,
		Branch:    "feat/other",
		StartedAt: time.Now().UTC(),
	})

	env := newTestEnv(t)
	instance.RegistryDirOverride = regDir
	env.RootCmd.SetArgs([]string{"stop", "--instance", otherRepo})
	if err := env.RootCmd.Execute(); err != nil {
		t.Fatalf("stop --instance via cobra: %v", err)
	}
}

// ---------------------------------------------------------------------------
// stop --all with empty registry
// ---------------------------------------------------------------------------

func TestMultiProcess_StopAllEmpty(t *testing.T) {
	_ = setupRegistry(t)

	// No entries at all. stopAllInstances should report "no running instances"
	// without error.
	err := stopAllInstances(false, false)
	if err != nil {
		t.Fatalf("stopAllInstances on empty registry should not error: %v", err)
	}
}

// ---------------------------------------------------------------------------
// Resolve does not match unrelated directory
// ---------------------------------------------------------------------------

func TestMultiProcess_ResolveUnrelated(t *testing.T) {
	regDir := setupRegistry(t)
	base := resolvedTempDir(t)

	repo := filepath.Join(base, "repo")
	unrelated := filepath.Join(base, "unrelated")
	_ = os.MkdirAll(repo, 0755)
	_ = os.MkdirAll(unrelated, 0755)

	writeEntry(t, regDir, instance.Entry{
		PID:       os.Getpid(),
		Worktree:  repo,
		Branch:    "main",
		StartedAt: time.Now().UTC(),
	})

	_, err := instance.Resolve(unrelated)
	if err == nil {
		t.Error("Resolve should fail for a directory that is not under any registered worktree")
	}
}

// ---------------------------------------------------------------------------
// Ensure stopInstance cleans stale entry from registry
// ---------------------------------------------------------------------------

func TestMultiProcess_StopInstanceCleansStale(t *testing.T) {
	regDir := setupRegistry(t)

	staleEntry := &instance.Entry{
		PID:       999999999,
		Worktree:  "/fake/stale",
		Branch:    "ghost",
		StartedAt: time.Now().UTC(),
	}
	writeEntry(t, regDir, *staleEntry)

	err := stopInstance(staleEntry, false, false)
	if err == nil {
		t.Fatal("expected error for stale PID")
	}

	// The error message should mention the stale entry was removed.
	want := fmt.Sprintf("pid %d is not running (stale registry entry removed)", staleEntry.PID)
	if err.Error() != want {
		t.Errorf("error = %q, want %q", err.Error(), want)
	}
}
