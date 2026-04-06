package daemon

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/dorkusprime/wolfcastle/internal/instance"
)

// ---------------------------------------------------------------------------
// stopInstance
// ---------------------------------------------------------------------------

func TestStopInstance_StalePID(t *testing.T) {
	dir := t.TempDir()
	instance.RegistryDirOverride = dir
	t.Cleanup(func() { instance.RegistryDirOverride = "" })

	// Write a registry entry so Deregister has something to clean.
	entry := &instance.Entry{
		PID:       999999999,
		Worktree:  "/fake/stale",
		Branch:    "old-branch",
		StartedAt: time.Now().UTC(),
	}
	slug := instance.Slug(entry.Worktree)
	data, _ := json.Marshal(entry)
	_ = os.WriteFile(filepath.Join(dir, slug+".json"), data, 0644)

	err := stopInstance(entry, false, false)
	if err == nil {
		t.Fatal("expected error for stale PID")
	}
	if got := err.Error(); got != "pid 999999999 is not running (stale registry entry removed)" {
		t.Errorf("unexpected error: %s", got)
	}
}

func TestStopInstance_StalePID_JSONMode(t *testing.T) {
	entry := &instance.Entry{
		PID:       999999999,
		Worktree:  "/fake/stale",
		Branch:    "old-branch",
		StartedAt: time.Now().UTC(),
	}

	err := stopInstance(entry, true, true)
	if err == nil {
		t.Fatal("expected error for stale PID in JSON mode")
	}
}

// ---------------------------------------------------------------------------
// stopAllInstances
// ---------------------------------------------------------------------------

func TestStopAllInstances_Empty(t *testing.T) {
	dir := t.TempDir()
	instance.RegistryDirOverride = dir
	t.Cleanup(func() { instance.RegistryDirOverride = "" })
	_ = os.MkdirAll(dir, 0755)

	err := stopAllInstances(false, false)
	if err != nil {
		t.Fatalf("expected no error for empty registry: %v", err)
	}
}

func TestStopAllInstances_AllStale(t *testing.T) {
	dir := t.TempDir()
	instance.RegistryDirOverride = dir
	t.Cleanup(func() { instance.RegistryDirOverride = "" })

	// Write two stale entries. Both will fail stopInstance, exercising
	// the error-continue path.
	for i, wt := range []string{"/fake/stale1", "/fake/stale2"} {
		entry := instance.Entry{
			PID:       999999990 + i,
			Worktree:  wt,
			Branch:    "stale",
			StartedAt: time.Now().UTC(),
		}
		data, _ := json.Marshal(entry)
		_ = os.WriteFile(filepath.Join(dir, instance.Slug(wt)+".json"), data, 0644)
	}

	// List() filters dead PIDs, so stopAllInstances will see an empty list.
	// We need entries with live PIDs for stopAllInstances to try to stop them.
	// Use our own PID so they appear live, but stopInstance will send SIGTERM
	// to our own process. Instead, test stopAllInstances through the cobra
	// command with --all.
	//
	// Since List() auto-cleans stale entries, this test effectively verifies
	// the empty-after-cleanup path.
	err := stopAllInstances(false, false)
	if err != nil {
		t.Fatalf("expected no error: %v", err)
	}
}

func TestStopAllInstances_JSONMode_Empty(t *testing.T) {
	dir := t.TempDir()
	instance.RegistryDirOverride = dir
	t.Cleanup(func() { instance.RegistryDirOverride = "" })
	_ = os.MkdirAll(dir, 0755)

	err := stopAllInstances(false, true)
	if err != nil {
		t.Fatalf("expected no error: %v", err)
	}
}

// ---------------------------------------------------------------------------
// newStopCmd: cobra flag paths
// ---------------------------------------------------------------------------

func TestStopCmd_AllFlagEmptyRegistry(t *testing.T) {
	env := newTestEnv(t)
	dir := t.TempDir()
	instance.RegistryDirOverride = dir
	t.Cleanup(func() { instance.RegistryDirOverride = "" })
	_ = os.MkdirAll(dir, 0755)

	env.RootCmd.SetArgs([]string{"stop", "--all"})
	if err := env.RootCmd.Execute(); err != nil {
		t.Fatalf("stop --all empty: %v", err)
	}
}

func TestStopCmd_AllFlagJSON(t *testing.T) {
	env := newTestEnv(t)
	env.App.JSON = true
	dir := t.TempDir()
	instance.RegistryDirOverride = dir
	t.Cleanup(func() { instance.RegistryDirOverride = "" })
	_ = os.MkdirAll(dir, 0755)

	env.RootCmd.SetArgs([]string{"stop", "--all"})
	if err := env.RootCmd.Execute(); err != nil {
		t.Fatalf("stop --all json: %v", err)
	}
}

func TestStopCmd_DrainFlag(t *testing.T) {
	env := newTestEnv(t)

	env.RootCmd.SetArgs([]string{"stop", "--drain"})
	if err := env.RootCmd.Execute(); err != nil {
		t.Fatalf("stop --drain: %v", err)
	}
}

func TestStopCmd_DrainFlagJSON(t *testing.T) {
	env := newTestEnv(t)
	env.App.JSON = true

	env.RootCmd.SetArgs([]string{"stop", "--drain"})
	if err := env.RootCmd.Execute(); err != nil {
		t.Fatalf("stop --drain json: %v", err)
	}
}

func TestStopCmd_ForceFlagNoInstance(t *testing.T) {
	env := newTestEnv(t)
	dir := t.TempDir()
	instance.RegistryDirOverride = dir
	t.Cleanup(func() { instance.RegistryDirOverride = "" })
	_ = os.MkdirAll(dir, 0755)

	env.RootCmd.SetArgs([]string{"stop", "--force"})
	err := env.RootCmd.Execute()
	if err == nil {
		t.Fatal("expected error when no instance found with --force")
	}
}

func TestStopCmd_NoInstanceForCwd(t *testing.T) {
	env := newTestEnv(t)
	dir := t.TempDir()
	instance.RegistryDirOverride = dir
	t.Cleanup(func() { instance.RegistryDirOverride = "" })
	_ = os.MkdirAll(dir, 0755)

	env.RootCmd.SetArgs([]string{"stop"})
	err := env.RootCmd.Execute()
	if err == nil {
		t.Fatal("expected error when no instance found")
	}
}
