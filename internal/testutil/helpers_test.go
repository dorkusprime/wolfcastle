package testutil

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/dorkusprime/wolfcastle/internal/state"
)

func TestWriteJSON_WritesValidJSON(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	path := filepath.Join(dir, "test.json")

	data := map[string]string{"key": "value"}
	WriteJSON(t, path, data)

	if _, err := os.Stat(path); os.IsNotExist(err) {
		t.Error("expected file to be created")
	}
}

func TestReadJSON_ReadsBackWrittenJSON(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	path := filepath.Join(dir, "test.json")

	original := map[string]string{"hello": "world"}
	WriteJSON(t, path, original)

	var loaded map[string]string
	ReadJSON(t, path, &loaded)

	if loaded["hello"] != "world" {
		t.Errorf("expected hello=world, got %v", loaded)
	}
}

func TestSetupWolfcastle_CreatesDirectory(t *testing.T) {
	t.Parallel()
	wcDir, ns := SetupWolfcastle(t)

	if _, err := os.Stat(wcDir); os.IsNotExist(err) {
		t.Error("wolfcastle dir should exist")
	}
	if ns == "" {
		t.Error("namespace should not be empty")
	}

	// Verify key subdirectories
	for _, sub := range []string{"system/base/prompts", "system/logs", "archive"} {
		if _, err := os.Stat(filepath.Join(wcDir, sub)); os.IsNotExist(err) {
			t.Errorf("subdirectory %s should exist", sub)
		}
	}
}

func TestSetupTree_CreatesFullTree(t *testing.T) {
	t.Parallel()
	wcDir, ns, idx := SetupTree(t)

	if len(idx.Nodes) != 3 {
		t.Errorf("expected 3 nodes, got %d", len(idx.Nodes))
	}

	// Verify we can load nodes back
	loaded := LoadNode(t, wcDir, ns, "root-project/child-a")
	if loaded.Name != "Child A" {
		t.Errorf("expected 'Child A', got %q", loaded.Name)
	}

	// Verify we can load root index
	reloaded := LoadRootIndex(t, wcDir, ns)
	if len(reloaded.Nodes) != 3 {
		t.Errorf("expected 3 nodes from reload, got %d", len(reloaded.Nodes))
	}
}

func TestSaveNode_PersistsChanges(t *testing.T) {
	t.Parallel()
	wcDir, ns, _ := SetupTree(t)

	// Load, modify, save, reload
	node := LoadNode(t, wcDir, ns, "root-project/child-a")
	node.State = state.StatusInProgress
	SaveNode(t, wcDir, ns, "root-project/child-a", node)

	reloaded := LoadNode(t, wcDir, ns, "root-project/child-a")
	if reloaded.State != state.StatusInProgress {
		t.Errorf("expected in_progress, got %s", reloaded.State)
	}
}
