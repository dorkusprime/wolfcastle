package cmd

import (
	"os"
	"path/filepath"
	"runtime"
	"testing"
)

// ── spec create — MkdirAll error ────────────────────────────────────

func TestSpecCreate_MkdirAllError_ReadOnly(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("permission test not reliable on Windows")
	}

	oldApp := app
	defer func() { app = oldApp }()

	env := newTestEnv(t)
	app = env.App

	// Remove the docs/specs dir and lock docs/ so MkdirAll fails.
	docsDir := filepath.Join(env.WolfcastleDir, "docs")
	_ = os.RemoveAll(filepath.Join(docsDir, "specs"))
	_ = os.Chmod(docsDir, 0555)
	t.Cleanup(func() { _ = os.Chmod(docsDir, 0755) })

	rootCmd.SetArgs([]string{"spec", "create", "Test Spec"})
	err := rootCmd.Execute()
	if err == nil {
		t.Error("expected error when specs directory cannot be created")
	}
}

// ── spec create — WriteFile error ───────────────────────────────────

func TestSpecCreate_WriteFileError_ReadOnly(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("permission test not reliable on Windows")
	}

	oldApp := app
	defer func() { app = oldApp }()

	env := newTestEnv(t)
	app = env.App

	// Lock the specs directory so WriteFile fails.
	specsDir := filepath.Join(env.WolfcastleDir, "docs", "specs")
	_ = os.Chmod(specsDir, 0555)
	t.Cleanup(func() { _ = os.Chmod(specsDir, 0755) })

	rootCmd.SetArgs([]string{"spec", "create", "Write Fail Spec"})
	err := rootCmd.Execute()
	if err == nil {
		t.Error("expected error when spec file cannot be written")
	}
}

// ── spec create — SaveNodeState error ───────────────────────────────

func TestSpecCreate_SaveNodeStateError_ReadOnly(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("permission test not reliable on Windows")
	}

	oldApp := app
	defer func() { app = oldApp }()

	env := newTestEnv(t)
	app = env.App
	env.createLeafNode(t, "perm-node", "Perm Node")

	// Lock the node directory so SaveNodeState fails after spec is created.
	nodeDir := filepath.Join(env.ProjectsDir, "perm-node")
	_ = os.Chmod(nodeDir, 0555)
	t.Cleanup(func() { _ = os.Chmod(nodeDir, 0755) })

	rootCmd.SetArgs([]string{"spec", "create", "--node", "perm-node", "Linked Fail Spec"})
	err := rootCmd.Execute()

	// Reset the --node flag to avoid polluting subsequent tests that
	// share the package-level specCreateCmd.
	_ = specCreateCmd.Flags().Set("node", "")

	if err == nil {
		t.Error("expected error when SaveNodeState fails for spec create with --node")
	}
}

// ── spec link — SaveNodeState error ─────────────────────────────────

func TestSpecLink_SaveNodeStateError_ReadOnly(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("permission test not reliable on Windows")
	}

	oldApp := app
	defer func() { app = oldApp }()

	env := newTestEnv(t)
	app = env.App
	env.createLeafNode(t, "link-node", "Link Node")

	// Create spec file.
	specsDir := filepath.Join(env.WolfcastleDir, "docs", "specs")
	filename := "2025-01-01T00-00Z-perm-spec.md"
	_ = os.WriteFile(filepath.Join(specsDir, filename), []byte("# Perm Spec\n"), 0644)

	// Lock the node directory.
	nodeDir := filepath.Join(env.ProjectsDir, "link-node")
	_ = os.Chmod(nodeDir, 0555)
	t.Cleanup(func() { _ = os.Chmod(nodeDir, 0755) })

	rootCmd.SetArgs([]string{"spec", "link", "--node", "link-node", filename})
	err := rootCmd.Execute()
	if err == nil {
		t.Error("expected error when SaveNodeState fails for spec link")
	}
}
