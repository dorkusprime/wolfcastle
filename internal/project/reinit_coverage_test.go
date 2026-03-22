package project

import (
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"

	"github.com/dorkusprime/wolfcastle/internal/config"
	"github.com/dorkusprime/wolfcastle/internal/tierfs"
)

// ── Reinit error paths ─────────────────────────────────────────────

func TestReinit_RemoveAllBaseFailure(t *testing.T) {
	t.Parallel()
	if runtime.GOOS == "windows" {
		t.Skip("chmod restrictions have no effect on Windows")
	}
	svc, _, root := newScaffoldService(t)

	if err := svc.Init(testIdentity()); err != nil {
		t.Fatal(err)
	}

	// Make system/ read-only so RemoveAll("system/base") cannot
	// unlink the base entry from its parent.
	sysDir := filepath.Join(root, "system")
	if err := os.Chmod(sysDir, 0555); err != nil {
		t.Fatal(err)
	}
	defer func() { _ = os.Chmod(sysDir, 0755) }()

	err := svc.Reinit()
	if err == nil {
		t.Fatal("expected error when RemoveAll on system/base fails")
	}
	if !strings.Contains(err.Error(), "removing system/base/") {
		t.Errorf("unexpected error: %v", err)
	}
}

func TestReinit_MkdirAllRecreationFailure(t *testing.T) {
	t.Parallel()
	if runtime.GOOS == "windows" {
		t.Skip("chmod restrictions have no effect on Windows")
	}
	svc, _, root := newScaffoldService(t)

	if err := svc.Init(testIdentity()); err != nil {
		t.Fatal(err)
	}

	// Remove base/ entirely so RemoveAll succeeds (no-op on missing path),
	// then make system/ read-only so MkdirAll cannot recreate base/.
	sysDir := filepath.Join(root, "system")
	if err := os.RemoveAll(filepath.Join(sysDir, "base")); err != nil {
		t.Fatal(err)
	}
	if err := os.Chmod(sysDir, 0555); err != nil {
		t.Fatal(err)
	}
	defer func() { _ = os.Chmod(sysDir, 0755) }()

	err := svc.Reinit()
	if err == nil {
		t.Fatal("expected error when MkdirAll fails after RemoveAll")
	}
	if !strings.Contains(err.Error(), "creating system/base/") {
		t.Errorf("unexpected error: %v", err)
	}
}

func TestReinit_WriteBaseConfigFailure(t *testing.T) {
	t.Parallel()
	tmp := t.TempDir()
	root := filepath.Join(tmp, ".wolfcastle")

	// Build a ScaffoldService whose config repo tiers root diverges from
	// the filesystem root. Reinit's RemoveAll and MkdirAll operate on root
	// directly, while WriteBase writes through the tiers path.
	if err := os.MkdirAll(filepath.Join(root, "system"), 0755); err != nil {
		t.Fatal(err)
	}
	badTiersRoot := filepath.Join(tmp, "bad-system")
	if err := os.WriteFile(badTiersRoot, []byte("block"), 0644); err != nil {
		t.Fatal(err)
	}
	tiers := tierfs.New(badTiersRoot)
	cfg := config.NewConfigRepositoryWithTiers(tiers, root)
	pw := &stubPromptWriter{}
	svc := NewScaffoldService(cfg, pw, nil, root)

	// Create the directories Reinit expects to tear down and rebuild.
	for _, d := range []string{
		"system/base/prompts", "system/base/rules", "system/base/audits",
		"system/custom", "system/local",
	} {
		if err := os.MkdirAll(filepath.Join(root, d), 0755); err != nil {
			t.Fatal(err)
		}
	}

	err := svc.Reinit()
	if err == nil {
		t.Fatal("expected error when WriteBase fails via bad tiers root")
	}
	if !strings.Contains(err.Error(), "scaffold:") {
		t.Errorf("unexpected error: %v", err)
	}
}

func TestReinit_WriteBasePromptsFailure(t *testing.T) {
	t.Parallel()
	svc, pw, _ := newScaffoldService(t)

	if err := svc.Init(testIdentity()); err != nil {
		t.Fatal(err)
	}

	// Make the prompt writer fail on the next call.
	pw.err = errSimulated
	pw.called = false

	err := svc.Reinit()
	if err == nil {
		t.Fatal("expected error when writeBasePrompts fails during Reinit")
	}
	if !strings.Contains(err.Error(), "scaffold:") {
		t.Errorf("unexpected error: %v", err)
	}
	if !pw.called {
		t.Error("WriteAllBase should have been invoked")
	}
}

func TestReinit_WriteCustomConfigFailure(t *testing.T) {
	t.Parallel()
	if runtime.GOOS == "windows" {
		t.Skip("chmod restrictions have no effect on Windows")
	}
	svc, _, root := newScaffoldService(t)

	if err := svc.Init(testIdentity()); err != nil {
		t.Fatal(err)
	}

	// Remove custom/config.json so the os.IsNotExist branch triggers,
	// then make custom/ read-only so WriteCustom's WriteFile fails.
	customDir := filepath.Join(root, "system", "custom")
	if err := os.Remove(filepath.Join(customDir, "config.json")); err != nil {
		t.Fatal(err)
	}
	if err := os.Chmod(customDir, 0555); err != nil {
		t.Fatal(err)
	}
	defer func() { _ = os.Chmod(customDir, 0755) }()

	err := svc.Reinit()
	if err == nil {
		t.Fatal("expected error when WriteCustom fails")
	}
	if !strings.Contains(err.Error(), "scaffold:") {
		t.Errorf("unexpected error: %v", err)
	}
}

func TestReinit_InvalidLocalConfigJSON(t *testing.T) {
	t.Parallel()
	svc, _, root := newScaffoldService(t)

	if err := svc.Init(testIdentity()); err != nil {
		t.Fatal(err)
	}

	// Write invalid JSON to local/config.json so Unmarshal fails.
	localPath := filepath.Join(root, "system", "local", "config.json")
	if err := os.WriteFile(localPath, []byte("{invalid"), 0644); err != nil {
		t.Fatal(err)
	}

	err := svc.Reinit()
	if err == nil {
		t.Fatal("expected error when local config contains invalid JSON")
	}
	if !strings.Contains(err.Error(), "not valid JSON") {
		t.Errorf("unexpected error: %v", err)
	}
}

func TestReinit_WriteLocalConfigFailure(t *testing.T) {
	t.Parallel()
	if runtime.GOOS == "windows" {
		t.Skip("chmod restrictions have no effect on Windows")
	}
	svc, _, root := newScaffoldService(t)

	if err := svc.Init(testIdentity()); err != nil {
		t.Fatal(err)
	}

	// Remove local/config.json then make local/ read-only. Directory
	// permissions control file creation; without the existing file,
	// WriteLocal must create a new one and fails on the read-only dir.
	localDir := filepath.Join(root, "system", "local")
	if err := os.Remove(filepath.Join(localDir, "config.json")); err != nil {
		t.Fatal(err)
	}
	if err := os.Chmod(localDir, 0555); err != nil {
		t.Fatal(err)
	}
	defer func() { _ = os.Chmod(localDir, 0755) }()

	err := svc.Reinit()
	if err == nil {
		t.Fatal("expected error when WriteLocal fails")
	}
	if !strings.Contains(err.Error(), "scaffold:") {
		t.Errorf("unexpected error: %v", err)
	}
}

// WriteAuditTaskMD's only uncovered branch (line 362-364) is the
// Templates.ReadFile error return. Because Templates is a compile-time
// embed.FS containing the audit-task.md template, ReadFile cannot fail
// at runtime. The branch is structurally unreachable.

// errSimulated is a reusable sentinel for tests that inject failures.
var errSimulated = errorf("simulated error")

func errorf(msg string) error { return &simErr{msg} }

type simErr struct{ msg string }

func (e *simErr) Error() string { return e.msg }
