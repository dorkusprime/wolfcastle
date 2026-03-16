package testutil

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/dorkusprime/wolfcastle/internal/state"
)

// ═══════════════════════════════════════════════════════════════════════════
// WriteJSON — creates nested directories
// ═══════════════════════════════════════════════════════════════════════════

func TestWriteJSON_CreatesNestedDirectories(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	path := filepath.Join(dir, "deep", "nested", "dir", "test.json")

	data := map[string]string{"key": "deep"}
	WriteJSON(t, path, data)

	if _, err := os.Stat(path); os.IsNotExist(err) {
		t.Error("file should be created with nested directories")
	}
}

func TestWriteJSON_ComplexObject(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	path := filepath.Join(dir, "complex.json")

	data := map[string]any{
		"name":    "test",
		"numbers": []int{1, 2, 3},
		"nested":  map[string]string{"inner": "value"},
	}
	WriteJSON(t, path, data)

	content, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if len(content) == 0 {
		t.Error("expected non-empty file")
	}
}

// ═══════════════════════════════════════════════════════════════════════════
// ReadJSON — complex types
// ═══════════════════════════════════════════════════════════════════════════

func TestReadJSON_NodeState(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	path := filepath.Join(dir, "node.json")

	original := state.NewNodeState("test-node", "Test Node", state.NodeLeaf)
	original.State = state.StatusInProgress
	original.Tasks = []state.Task{
		{ID: "t1", Description: "do stuff", State: state.StatusNotStarted},
	}
	WriteJSON(t, path, original)

	var loaded state.NodeState
	ReadJSON(t, path, &loaded)

	if loaded.Name != "Test Node" {
		t.Errorf("expected 'Test Node', got %q", loaded.Name)
	}
	if loaded.State != state.StatusInProgress {
		t.Errorf("expected in_progress, got %s", loaded.State)
	}
	if len(loaded.Tasks) != 1 {
		t.Errorf("expected 1 task, got %d", len(loaded.Tasks))
	}
}

// ═══════════════════════════════════════════════════════════════════════════
// SetupWolfcastle — verifies all key directories
// ═══════════════════════════════════════════════════════════════════════════

func TestSetupWolfcastle_AllDirectoriesExist(t *testing.T) {
	t.Parallel()
	wcDir, ns := SetupWolfcastle(t)

	expectedDirs := []string{
		"system/base/prompts",
		"system/base/rules",
		"system/base/audits",
		"system/custom",
		"system/local",
		"archive",
		"docs/decisions",
		"docs/specs",
		"system/logs",
		filepath.Join("system", "projects", ns),
	}
	for _, d := range expectedDirs {
		path := filepath.Join(wcDir, d)
		if _, err := os.Stat(path); os.IsNotExist(err) {
			t.Errorf("expected directory %s to exist", d)
		}
	}
}

func TestSetupWolfcastle_ConfigFilesExist(t *testing.T) {
	t.Parallel()
	wcDir, _ := SetupWolfcastle(t)

	for _, f := range []string{"system/base/config.json", "system/custom/config.json", "system/local/config.json"} {
		if _, err := os.Stat(filepath.Join(wcDir, f)); os.IsNotExist(err) {
			t.Errorf("expected %s to exist", f)
		}
	}
}

func TestSetupWolfcastle_RootIndexExists(t *testing.T) {
	t.Parallel()
	wcDir, ns := SetupWolfcastle(t)

	idxPath := filepath.Join(wcDir, "system", "projects", ns, "state.json")
	if _, err := os.Stat(idxPath); os.IsNotExist(err) {
		t.Error("expected root index to exist")
	}

	var idx state.RootIndex
	ReadJSON(t, idxPath, &idx)
	if idx.Nodes == nil {
		t.Error("root index should have initialized Nodes map")
	}
}

// ═══════════════════════════════════════════════════════════════════════════
// SetupTree — verify tree structure deeply
// ═══════════════════════════════════════════════════════════════════════════

func TestSetupTree_ChildAHasTasks(t *testing.T) {
	t.Parallel()
	wcDir, ns, _ := SetupTree(t)

	childA := LoadNode(t, wcDir, ns, "root-project/child-a")
	if len(childA.Tasks) != 2 {
		t.Errorf("expected 2 tasks on child-a, got %d", len(childA.Tasks))
	}

	hasAudit := false
	for _, task := range childA.Tasks {
		if task.IsAudit {
			hasAudit = true
		}
	}
	if !hasAudit {
		t.Error("child-a should have an audit task")
	}
}

func TestSetupTree_RootIsOrchestrator(t *testing.T) {
	t.Parallel()
	wcDir, ns, _ := SetupTree(t)

	root := LoadNode(t, wcDir, ns, "root-project")
	if root.Type != state.NodeOrchestrator {
		t.Errorf("root should be orchestrator, got %s", root.Type)
	}
	if len(root.Children) != 2 {
		t.Errorf("root should have 2 children, got %d", len(root.Children))
	}
}

func TestSetupTree_IndexHasCorrectParents(t *testing.T) {
	t.Parallel()
	_, _, idx := SetupTree(t)

	if idx.Nodes["root-project/child-a"].Parent != "root-project" {
		t.Error("child-a should have root-project as parent")
	}
	if idx.Nodes["root-project/child-b"].Parent != "root-project" {
		t.Error("child-b should have root-project as parent")
	}
	if idx.Nodes["root-project"].Parent != "" {
		t.Error("root-project should have no parent")
	}
}

// ═══════════════════════════════════════════════════════════════════════════
// SaveNode — verify persistence
// ═══════════════════════════════════════════════════════════════════════════

func TestSaveNode_PreservesAllFields(t *testing.T) {
	t.Parallel()
	wcDir, ns, _ := SetupTree(t)

	node := LoadNode(t, wcDir, ns, "root-project/child-a")
	node.State = state.StatusComplete
	node.Tasks[0].State = state.StatusComplete
	SaveNode(t, wcDir, ns, "root-project/child-a", node)

	reloaded := LoadNode(t, wcDir, ns, "root-project/child-a")
	if reloaded.State != state.StatusComplete {
		t.Errorf("expected complete, got %s", reloaded.State)
	}
	if reloaded.Tasks[0].State != state.StatusComplete {
		t.Errorf("expected task complete, got %s", reloaded.Tasks[0].State)
	}
}
