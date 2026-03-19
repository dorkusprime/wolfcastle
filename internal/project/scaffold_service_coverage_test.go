package project

import (
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/dorkusprime/wolfcastle/internal/config"
	"github.com/dorkusprime/wolfcastle/internal/tierfs"
)

// ── Init error paths ────────────────────────────────────────────────

func TestScaffoldService_Init_MkdirAllFailure(t *testing.T) {
	t.Parallel()
	// Place a file where the root is expected so MkdirAll fails on
	// the first directory it tries to create.
	tmp := t.TempDir()
	blocker := filepath.Join(tmp, ".wolfcastle")
	if err := os.WriteFile(blocker, []byte("block"), 0644); err != nil {
		t.Fatal(err)
	}

	tiers := tierfs.New(filepath.Join(blocker, "system"))
	cfg := config.NewConfigRepositoryWithTiers(tiers, blocker)
	pw := &stubPromptWriter{}
	svc := NewScaffoldService(cfg, pw, nil, blocker)

	err := svc.Init(testIdentity())
	if err == nil {
		t.Fatal("expected error when MkdirAll fails")
	}
	if !strings.Contains(err.Error(), "scaffold: creating directory") {
		t.Errorf("unexpected error message: %v", err)
	}
}

func TestScaffoldService_Init_GitignoreWriteFailure(t *testing.T) {
	t.Parallel()
	svc, _, root := newScaffoldService(t)

	// Pre-create all directories so MkdirAll succeeds, then make root
	// read-only so .gitignore WriteFile fails.
	dirs := []string{
		"system/base/prompts", "system/base/rules", "system/base/audits",
		"system/custom", "system/local", "system/projects", "system/logs",
		"archive", "artifacts", "docs/decisions", "docs/specs",
	}
	for _, d := range dirs {
		if err := os.MkdirAll(filepath.Join(root, d), 0755); err != nil {
			t.Fatal(err)
		}
	}
	if err := os.Chmod(root, 0555); err != nil {
		t.Fatal(err)
	}
	defer func() { _ = os.Chmod(root, 0755) }()

	err := svc.Init(testIdentity())
	if err == nil {
		t.Fatal("expected error when .gitignore cannot be written")
	}
	if !strings.Contains(err.Error(), ".gitignore") {
		t.Errorf("unexpected error message: %v", err)
	}
}

func TestScaffoldService_Init_WriteBaseConfigFailure(t *testing.T) {
	t.Parallel()
	tmp := t.TempDir()
	root := filepath.Join(tmp, ".wolfcastle")

	// Create all dirs, then put a directory where base/config.json
	// should be written so WriteBase fails.
	dirs := []string{
		"system/base/prompts", "system/base/rules", "system/base/audits",
		"system/custom", "system/local", "system/projects", "system/logs",
		"archive", "artifacts", "docs/decisions", "docs/specs",
	}
	for _, d := range dirs {
		if err := os.MkdirAll(filepath.Join(root, d), 0755); err != nil {
			t.Fatal(err)
		}
	}
	if err := os.MkdirAll(filepath.Join(root, "system", "base", "config.json"), 0755); err != nil {
		t.Fatal(err)
	}

	tiers := tierfs.New(filepath.Join(root, "system"))
	cfg := config.NewConfigRepositoryWithTiers(tiers, root)
	pw := &stubPromptWriter{}
	svc := NewScaffoldService(cfg, pw, nil, root)

	err := svc.Init(testIdentity())
	if err == nil {
		t.Fatal("expected error when WriteBase fails")
	}
	if !strings.Contains(err.Error(), "scaffold:") {
		t.Errorf("unexpected error message: %v", err)
	}
}

func TestScaffoldService_Init_WriteCustomConfigFailure(t *testing.T) {
	t.Parallel()
	tmp := t.TempDir()
	root := filepath.Join(tmp, ".wolfcastle")

	dirs := []string{
		"system/base/prompts", "system/base/rules", "system/base/audits",
		"system/custom", "system/local", "system/projects", "system/logs",
		"archive", "artifacts", "docs/decisions", "docs/specs",
	}
	for _, d := range dirs {
		if err := os.MkdirAll(filepath.Join(root, d), 0755); err != nil {
			t.Fatal(err)
		}
	}
	// Block custom/config.json with a directory
	if err := os.MkdirAll(filepath.Join(root, "system", "custom", "config.json"), 0755); err != nil {
		t.Fatal(err)
	}

	tiers := tierfs.New(filepath.Join(root, "system"))
	cfg := config.NewConfigRepositoryWithTiers(tiers, root)
	pw := &stubPromptWriter{}
	svc := NewScaffoldService(cfg, pw, nil, root)

	err := svc.Init(testIdentity())
	if err == nil {
		t.Fatal("expected error when WriteCustom fails")
	}
	if !strings.Contains(err.Error(), "scaffold:") {
		t.Errorf("unexpected error message: %v", err)
	}
}

func TestScaffoldService_Init_WriteLocalConfigFailure(t *testing.T) {
	t.Parallel()
	tmp := t.TempDir()
	root := filepath.Join(tmp, ".wolfcastle")

	dirs := []string{
		"system/base/prompts", "system/base/rules", "system/base/audits",
		"system/custom", "system/local", "system/projects", "system/logs",
		"archive", "artifacts", "docs/decisions", "docs/specs",
	}
	for _, d := range dirs {
		if err := os.MkdirAll(filepath.Join(root, d), 0755); err != nil {
			t.Fatal(err)
		}
	}
	// Block local/config.json with a directory
	if err := os.MkdirAll(filepath.Join(root, "system", "local", "config.json"), 0755); err != nil {
		t.Fatal(err)
	}

	tiers := tierfs.New(filepath.Join(root, "system"))
	cfg := config.NewConfigRepositoryWithTiers(tiers, root)
	pw := &stubPromptWriter{}
	svc := NewScaffoldService(cfg, pw, nil, root)

	err := svc.Init(testIdentity())
	if err == nil {
		t.Fatal("expected error when WriteLocal fails")
	}
	if !strings.Contains(err.Error(), "scaffold:") {
		t.Errorf("unexpected error message: %v", err)
	}
}

func TestScaffoldService_Init_NamespaceDirFailure(t *testing.T) {
	t.Parallel()
	svc, _, root := newScaffoldService(t)

	// Let Init create all standard dirs, then block the namespace
	// subdirectory. The projects dir itself is created by Init's loop,
	// so we block the child by placing a file at the namespace path.
	id := testIdentity()
	nsDir := id.ProjectsDir(root)
	if err := os.MkdirAll(filepath.Dir(nsDir), 0755); err != nil {
		t.Fatal(err)
	}
	// Place a file where the namespace directory would be created.
	if err := os.WriteFile(nsDir, []byte("block"), 0644); err != nil {
		t.Fatal(err)
	}

	err := svc.Init(id)
	if err == nil {
		t.Fatal("expected error when namespace directory creation fails")
	}
	if !strings.Contains(err.Error(), "namespace") {
		t.Errorf("unexpected error message: %v", err)
	}
}

func TestScaffoldService_Init_RootIndexWriteFailure(t *testing.T) {
	t.Parallel()
	svc, _, root := newScaffoldService(t)

	// Pre-create everything, then block state.json with a directory.
	id := testIdentity()
	nsDir := id.ProjectsDir(root)
	if err := os.MkdirAll(nsDir, 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(filepath.Join(nsDir, "state.json"), 0755); err != nil {
		t.Fatal(err)
	}

	err := svc.Init(id)
	if err == nil {
		t.Fatal("expected error when root index cannot be written")
	}
	if !strings.Contains(err.Error(), "root index") {
		t.Errorf("unexpected error message: %v", err)
	}
}

func TestScaffoldService_Init_WriteBasePromptsFailure(t *testing.T) {
	t.Parallel()
	svc, pw, _ := newScaffoldService(t)
	pw.err = errors.New("simulated prompt write error")

	err := svc.Init(testIdentity())
	if err == nil {
		t.Fatal("expected error when writeBasePrompts fails")
	}
	if !strings.Contains(err.Error(), "scaffold:") {
		t.Errorf("unexpected error message: %v", err)
	}
	if !pw.called {
		t.Error("WriteAllBase should have been called even though it returned error")
	}
}

// ── writeBasePrompts via Init ───────────────────────────────────────

func TestScaffoldService_WriteBasePrompts_PromptsErrorPropagation(t *testing.T) {
	t.Parallel()
	svc, pw, _ := newScaffoldService(t)

	sentinel := errors.New("write-all-base broke")
	pw.err = sentinel

	err := svc.Init(testIdentity())
	if err == nil {
		t.Fatal("expected error to propagate from WriteAllBase")
	}
	if !errors.Is(err, sentinel) {
		t.Errorf("expected sentinel error in chain, got: %v", err)
	}
}
