package inbox

import (
	"os"
	"runtime"
	"testing"
)

// ── inbox add — SaveInbox error ─────────────────────────────────────

func TestInboxAdd_SaveInboxError_ReadOnly(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("permission test not reliable on Windows")
	}

	env := newTestEnv(t)

	// Lock the projects dir so SaveInbox (atomicWriteJSON) fails.
	_ = os.Chmod(env.ProjectsDir, 0555)
	t.Cleanup(func() { _ = os.Chmod(env.ProjectsDir, 0755) })

	env.RootCmd.SetArgs([]string{"inbox", "add", "test idea"})
	err := env.RootCmd.Execute()
	if err == nil {
		t.Error("expected error when SaveInbox fails for inbox add")
	}
}

// ── inbox clear — SaveInbox error ───────────────────────────────────

func TestInboxClear_SaveInboxError_ReadOnly(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("permission test not reliable on Windows")
	}

	env := newTestEnv(t)

	// Add an item first (with writable dir).
	env.RootCmd.SetArgs([]string{"inbox", "add", "some item"})
	if err := env.RootCmd.Execute(); err != nil {
		t.Fatal(err)
	}

	// Lock the projects dir so SaveInbox fails during clear.
	_ = os.Chmod(env.ProjectsDir, 0555)
	t.Cleanup(func() { _ = os.Chmod(env.ProjectsDir, 0755) })

	env.RootCmd.SetArgs([]string{"inbox", "clear", "--all"})
	err := env.RootCmd.Execute()
	if err == nil {
		t.Error("expected error when SaveInbox fails for inbox clear")
	}
}
