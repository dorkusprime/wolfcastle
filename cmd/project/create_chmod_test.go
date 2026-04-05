package project

import (
	"os"
	"path/filepath"
	"runtime"
	"testing"

	"github.com/dorkusprime/wolfcastle/internal/state"
)

// ── project create: filesystem error paths via chmod ────────────────

func TestProjectCreate_SaveNodeState_PromotedParent_ReadOnly(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("permission test not reliable on Windows")
	}

	env := newTestEnv(t)

	// Create a leaf node (no non-audit tasks. Eligible for promotion).
	env.RootCmd.SetArgs([]string{"project", "create", "--type", "leaf", "parent-leaf"})
	if err := env.RootCmd.Execute(); err != nil {
		t.Fatal(err)
	}

	// Lock the parent directory so SaveNodeState during promotion fails.
	parentDir := filepath.Join(env.ProjectsDir, "parent-leaf")
	_ = os.Chmod(parentDir, 0555)
	t.Cleanup(func() { _ = os.Chmod(parentDir, 0755) })

	env.RootCmd.SetArgs([]string{"project", "create", "--type", "leaf", "--node", "parent-leaf", "child"})
	err := env.RootCmd.Execute()
	if err == nil {
		t.Error("expected error when SaveNodeState for promoted parent fails")
	}
}

func TestProjectCreate_MkdirAllError_ReadOnly(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("permission test not reliable on Windows")
	}

	env := newTestEnv(t)

	// Lock the projects dir so MkdirAll for the new node directory fails.
	_ = os.Chmod(env.ProjectsDir, 0555)
	t.Cleanup(func() { _ = os.Chmod(env.ProjectsDir, 0755) })

	env.RootCmd.SetArgs([]string{"project", "create", "--type", "leaf", "new-proj"})
	err := env.RootCmd.Execute()
	if err == nil {
		t.Error("expected error when MkdirAll for node directory fails")
	}
}

func TestProjectCreate_SaveNodeStateError_ReadOnly(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("permission test not reliable on Windows")
	}

	env := newTestEnv(t)

	// Pre-create the node directory, then lock it so SaveNodeState fails.
	nodeDir := filepath.Join(env.ProjectsDir, "locked-proj")
	_ = os.MkdirAll(nodeDir, 0755)
	_ = os.Chmod(nodeDir, 0555)
	t.Cleanup(func() { _ = os.Chmod(nodeDir, 0755) })

	env.RootCmd.SetArgs([]string{"project", "create", "--type", "leaf", "locked-proj"})
	err := env.RootCmd.Execute()
	if err == nil {
		t.Error("expected error when SaveNodeState fails for new node")
	}
}

func TestProjectCreate_WriteFileError_ReadOnly(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("permission test not reliable on Windows")
	}

	env := newTestEnv(t)

	// Pre-create the node directory and write a valid state.json,
	// then place a directory where the .md file should go.
	nodeDir := filepath.Join(env.ProjectsDir, "md-block")
	_ = os.MkdirAll(nodeDir, 0755)
	// Place a directory where the markdown file should be written.
	_ = os.MkdirAll(filepath.Join(nodeDir, "md-block.md"), 0755)

	env.RootCmd.SetArgs([]string{"project", "create", "--type", "leaf", "md-block"})
	err := env.RootCmd.Execute()
	if err == nil {
		t.Error("expected error when writing project description fails")
	}
}

func TestProjectCreate_SaveRootIndexError_ReadOnly(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("permission test not reliable on Windows")
	}

	env := newTestEnv(t)

	// Create a successful node first so everything except SaveRootIndex works.
	env.RootCmd.SetArgs([]string{"project", "create", "--type", "leaf", "first-proj"})
	if err := env.RootCmd.Execute(); err != nil {
		t.Fatal(err)
	}

	// Lock the projects dir so the root index can't be rewritten.
	_ = os.Chmod(env.ProjectsDir, 0555)
	t.Cleanup(func() { _ = os.Chmod(env.ProjectsDir, 0755) })

	env.RootCmd.SetArgs([]string{"project", "create", "--type", "leaf", "second-proj"})
	err := env.RootCmd.Execute()
	if err == nil {
		t.Error("expected error when SaveRootIndex fails")
	}
}

func TestProjectCreate_ChildWriteFileError_ReadOnly(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("permission test not reliable on Windows")
	}

	env := newTestEnv(t)

	// Create an orchestrator parent first.
	env.RootCmd.SetArgs([]string{"project", "create", "--type", "orchestrator", "parent-orch"})
	if err := env.RootCmd.Execute(); err != nil {
		t.Fatal(err)
	}

	// Pre-create the child node directory and block the .md file.
	childDir := filepath.Join(env.ProjectsDir, "parent-orch", "child-md")
	_ = os.MkdirAll(childDir, 0755)
	// Place a directory where the markdown file should be written.
	_ = os.MkdirAll(filepath.Join(childDir, "child-md.md"), 0755)

	env.RootCmd.SetArgs([]string{"project", "create", "--type", "leaf", "--node", "parent-orch", "child-md"})
	err := env.RootCmd.Execute()
	if err == nil {
		t.Error("expected error when writing child description fails")
	}
}

// Test that SaveNodeState error for promoted parent uses correct message.
func TestProjectCreate_LoadNodeState_PromotedParent_Missing(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("permission test not reliable on Windows")
	}

	env := newTestEnv(t)

	// Create a root index entry that claims a leaf exists, but no state.json.
	idx, _ := state.LoadRootIndex(filepath.Join(env.ProjectsDir, "state.json"))
	_ = os.MkdirAll(filepath.Join(env.ProjectsDir, "ghost-leaf"), 0755)
	idx.Nodes["ghost-leaf"] = state.IndexEntry{
		Name: "Ghost", Type: state.NodeLeaf,
		State: state.StatusNotStarted, Address: "ghost-leaf",
		Children: []string{},
	}
	idx.Root = append(idx.Root, "ghost-leaf")
	_ = state.SaveRootIndex(filepath.Join(env.ProjectsDir, "state.json"), idx)

	env.RootCmd.SetArgs([]string{"project", "create", "--type", "leaf", "--node", "ghost-leaf", "child"})
	err := env.RootCmd.Execute()
	if err == nil {
		t.Error("expected error when parent state.json is missing")
	}
}
