package state

import (
	"os"
	"path/filepath"
	"testing"
)

func TestSaveAndLoadRootIndex_Roundtrip(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	path := filepath.Join(dir, "state.json")

	idx := NewRootIndex()
	idx.RootID = "my-project"
	idx.RootName = "My Project"
	idx.RootState = StatusNotStarted
	idx.Root = []string{"leaf-a"}
	idx.Nodes["leaf-a"] = IndexEntry{
		Name:    "Leaf A",
		Type:    NodeLeaf,
		State:   StatusNotStarted,
		Address: "leaf-a",
	}

	if err := SaveRootIndex(path, idx); err != nil {
		t.Fatal(err)
	}

	loaded, err := LoadRootIndex(path)
	if err != nil {
		t.Fatal(err)
	}

	if loaded.RootID != "my-project" {
		t.Errorf("expected root_id 'my-project', got %q", loaded.RootID)
	}
	if loaded.RootName != "My Project" {
		t.Errorf("expected root_name 'My Project', got %q", loaded.RootName)
	}
	if len(loaded.Root) != 1 || loaded.Root[0] != "leaf-a" {
		t.Errorf("expected root [leaf-a], got %v", loaded.Root)
	}
	if len(loaded.Nodes) != 1 {
		t.Fatalf("expected 1 node, got %d", len(loaded.Nodes))
	}
	entry := loaded.Nodes["leaf-a"]
	if entry.Name != "Leaf A" {
		t.Errorf("expected node name 'Leaf A', got %q", entry.Name)
	}
}

func TestSaveAndLoadNodeState_Roundtrip(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	path := filepath.Join(dir, "state.json")

	ns := NewNodeState("leaf-1", "Test Leaf", NodeLeaf)
	ns.Tasks = []Task{
		{ID: "task-1", Description: "do work", State: StatusNotStarted},
		{ID: "task-2", Description: "more work", State: StatusInProgress, FailureCount: 2},
	}

	if err := SaveNodeState(path, ns); err != nil {
		t.Fatal(err)
	}

	loaded, err := LoadNodeState(path)
	if err != nil {
		t.Fatal(err)
	}

	if loaded.ID != "leaf-1" {
		t.Errorf("expected id 'leaf-1', got %q", loaded.ID)
	}
	if loaded.Name != "Test Leaf" {
		t.Errorf("expected name 'Test Leaf', got %q", loaded.Name)
	}
	if len(loaded.Tasks) != 2 {
		t.Fatalf("expected 2 tasks, got %d", len(loaded.Tasks))
	}
	if loaded.Tasks[1].FailureCount != 2 {
		t.Errorf("expected failure_count 2, got %d", loaded.Tasks[1].FailureCount)
	}
}

func TestAtomicWrite_NoTempFileLeftBehind(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	path := filepath.Join(dir, "state.json")

	ns := NewNodeState("n1", "Node", NodeLeaf)
	if err := SaveNodeState(path, ns); err != nil {
		t.Fatal(err)
	}

	// Check no temp files remain
	entries, err := os.ReadDir(dir)
	if err != nil {
		t.Fatal(err)
	}
	for _, e := range entries {
		if e.Name() != "state.json" {
			t.Errorf("unexpected file left behind: %s", e.Name())
		}
	}
}

func TestLoadRootIndex_MissingFile(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	path := filepath.Join(dir, "nonexistent.json")

	_, err := LoadRootIndex(path)
	if err == nil {
		t.Error("expected error for missing file")
	}
}

func TestLoadNodeState_MissingFile(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	path := filepath.Join(dir, "nonexistent.json")

	_, err := LoadNodeState(path)
	if err == nil {
		t.Error("expected error for missing file")
	}
}

func TestLoadRootIndex_InvalidJSON(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	path := filepath.Join(dir, "bad.json")
	if err := os.WriteFile(path, []byte("not json"), 0644); err != nil {
		t.Fatal(err)
	}

	_, err := LoadRootIndex(path)
	if err == nil {
		t.Error("expected error for invalid JSON")
	}
}

func TestLoadNodeState_InvalidJSON(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	path := filepath.Join(dir, "bad.json")
	if err := os.WriteFile(path, []byte("{broken"), 0644); err != nil {
		t.Fatal(err)
	}

	_, err := LoadNodeState(path)
	if err == nil {
		t.Error("expected error for invalid JSON")
	}
}

func TestSaveRootIndex_CreatesDirectories(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	path := filepath.Join(dir, "sub", "dir", "state.json")

	idx := NewRootIndex()
	if err := SaveRootIndex(path, idx); err != nil {
		t.Fatal(err)
	}

	if _, err := os.Stat(path); err != nil {
		t.Errorf("file should exist: %v", err)
	}
}

func TestLoadRootIndex_InitializesNilNodesMap(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	path := filepath.Join(dir, "state.json")

	// Write JSON with no "nodes" key
	if err := os.WriteFile(path, []byte(`{"version":1}`), 0644); err != nil {
		t.Fatal(err)
	}

	loaded, err := LoadRootIndex(path)
	if err != nil {
		t.Fatal(err)
	}
	if loaded.Nodes == nil {
		t.Error("Nodes map should be initialized even when missing from JSON")
	}
}
