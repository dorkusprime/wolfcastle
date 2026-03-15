package validate

import (
	"os"
	"path/filepath"
	"runtime"
	"testing"

	"github.com/dorkusprime/wolfcastle/internal/state"
)

func TestCheckOrphanedStateFiles_UnreadableDir(t *testing.T) {
	t.Parallel()
	if runtime.GOOS == "windows" {
		t.Skip("chmod 0000 has no effect on Windows")
	}
	dir := t.TempDir()
	idx := state.NewRootIndex()

	// Create a subdirectory that can't be read
	sub := filepath.Join(dir, "locked")
	_ = os.MkdirAll(sub, 0755)
	_ = os.WriteFile(filepath.Join(sub, "state.json"), []byte(`{"id":"x"}`), 0644)
	_ = os.Chmod(sub, 0000)
	defer func() { _ = os.Chmod(sub, 0755) }()

	engine := NewEngine(dir, DefaultNodeLoader(dir))
	report := &Report{}
	engine.checkOrphanedStateFiles(idx, report)
	// Should not panic; unreadable directories are skipped gracefully
}

func TestCheckOrphanedDefinitions_UnreadableDir(t *testing.T) {
	t.Parallel()
	if runtime.GOOS == "windows" {
		t.Skip("chmod 0000 has no effect on Windows")
	}
	dir := t.TempDir()
	idx := state.NewRootIndex()

	sub := filepath.Join(dir, "locked")
	_ = os.MkdirAll(sub, 0755)
	_ = os.WriteFile(filepath.Join(sub, "definition.md"), []byte("test"), 0644)
	_ = os.Chmod(sub, 0000)
	defer func() { _ = os.Chmod(sub, 0755) }()

	engine := NewEngine(dir, DefaultNodeLoader(dir))
	report := &Report{}
	engine.checkOrphanedDefinitions(idx, report)
	// Should not panic
}

func TestValidateStartup_WithWolfcastleDir(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	wolfDir := filepath.Join(dir, ".wolfcastle")
	_ = os.MkdirAll(wolfDir, 0755)

	idx := state.NewRootIndex()
	leafDir := filepath.Join(dir, "leaf")
	_ = os.MkdirAll(leafDir, 0755)
	ns := state.NewNodeState("leaf", "Leaf", state.NodeLeaf)
	ns.Tasks = []state.Task{
		{ID: "task-1", Description: "work", State: state.StatusNotStarted},
		{ID: "audit", Description: "audit", State: state.StatusNotStarted, IsAudit: true},
	}
	_ = state.SaveNodeState(filepath.Join(leafDir, "state.json"), ns)

	idx.Root = []string{"leaf"}
	idx.Nodes["leaf"] = state.IndexEntry{
		Name: "Leaf", Type: state.NodeLeaf, State: state.StatusNotStarted, Address: "leaf",
	}

	engine := NewEngine(dir, DefaultNodeLoader(dir), wolfDir)
	report := engine.ValidateStartup(idx)
	if report == nil {
		t.Fatal("expected non-nil report from ValidateStartup")
	}
}
