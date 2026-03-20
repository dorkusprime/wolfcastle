package daemon

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	"github.com/dorkusprime/wolfcastle/internal/state"
	"github.com/dorkusprime/wolfcastle/internal/testutil"
)

func newStatusTestEnv(t *testing.T) *testEnv {
	t.Helper()
	env := newTestEnv(t)
	env.env.WithProject("My Project", testutil.Leaf("my-project"))
	return env
}

func TestStatusCmd_Success(t *testing.T) {
	env := newStatusTestEnv(t)
	env.RootCmd.SetArgs([]string{"status"})
	if err := env.RootCmd.Execute(); err != nil {
		t.Fatalf("status failed: %v", err)
	}
}

func TestStatusCmd_WithScope(t *testing.T) {
	env := newStatusTestEnv(t)
	env.RootCmd.SetArgs([]string{"status", "--node", "my-project"})
	if err := env.RootCmd.Execute(); err != nil {
		t.Fatalf("status --node failed: %v", err)
	}
}

func TestShowAllStatus_NoNamespaces(t *testing.T) {
	env := newTestEnv(t)
	// showAllStatus reads from projects/ dir
	err := showAllStatus(env.App)
	if err != nil {
		t.Fatalf("showAllStatus failed: %v", err)
	}
}

func TestShowTreeStatus_EmptyTree(t *testing.T) {
	env := newTestEnv(t)
	idx := state.NewRootIndex()
	err := showTreeStatus(env.App, idx, "")
	if err != nil {
		t.Fatalf("showTreeStatus failed: %v", err)
	}
}

func TestShowTreeStatus_WithNodes(t *testing.T) {
	env := newStatusTestEnv(t)
	idx, _ := state.LoadRootIndex(filepath.Join(env.ProjectsDir, "state.json"))
	err := showTreeStatus(env.App, idx, "")
	if err != nil {
		t.Fatalf("showTreeStatus failed: %v", err)
	}
}

func TestShowTreeStatus_MultipleNodeStates(t *testing.T) {
	env := newStatusTestEnv(t)

	// Add another node in different states
	idx, _ := state.LoadRootIndex(filepath.Join(env.ProjectsDir, "state.json"))
	idx.Nodes["my-project/child-a"] = state.IndexEntry{
		Name:     "Child A",
		Type:     state.NodeLeaf,
		State:    state.StatusComplete,
		Address:  "my-project/child-a",
		Parent:   "my-project",
		Children: []string{},
	}
	idx.Nodes["my-project/child-b"] = state.IndexEntry{
		Name:     "Child B",
		Type:     state.NodeLeaf,
		State:    state.StatusBlocked,
		Address:  "my-project/child-b",
		Parent:   "my-project",
		Children: []string{},
	}
	idx.Nodes["my-project/child-c"] = state.IndexEntry{
		Name:     "Child C",
		Type:     state.NodeLeaf,
		State:    state.StatusNotStarted,
		Address:  "my-project/child-c",
		Parent:   "my-project",
		Children: []string{},
	}
	idxData, _ := json.MarshalIndent(idx, "", "  ")
	_ = os.WriteFile(filepath.Join(env.ProjectsDir, "state.json"), idxData, 0644)

	// Create node dirs and states for children
	for _, name := range []string{"child-a", "child-b", "child-c"} {
		nodeDir := filepath.Join(env.ProjectsDir, "my-project", name)
		_ = os.MkdirAll(nodeDir, 0755)
		ns := state.NewNodeState(name, "Child", state.NodeLeaf)
		nsData, _ := json.MarshalIndent(ns, "", "  ")
		_ = os.WriteFile(filepath.Join(nodeDir, "state.json"), nsData, 0644)
	}

	// Test human output with scope
	if err := showTreeStatus(env.App, idx, "my-project"); err != nil {
		t.Fatalf("showTreeStatus with multiple states and scope failed: %v", err)
	}

	// Test JSON output
	env.App.JSON = true
	defer func() { env.App.JSON = false }()
	if err := showTreeStatus(env.App, idx, "my-project"); err != nil {
		t.Fatalf("showTreeStatus JSON with multiple states failed: %v", err)
	}
}

func TestShowAllStatus_WithMultipleNamespaces(t *testing.T) {
	env := newStatusTestEnv(t)
	// Create a second namespace
	ns2Dir := filepath.Join(env.WolfcastleDir, "system", "projects", "other-eng")
	_ = os.MkdirAll(ns2Dir, 0755)
	idx2 := state.NewRootIndex()
	idx2.Nodes["other-proj"] = state.IndexEntry{
		Name:    "Other Project",
		Type:    state.NodeLeaf,
		State:   state.StatusComplete,
		Address: "other-proj",
	}
	data, _ := json.MarshalIndent(idx2, "", "  ")
	_ = os.WriteFile(filepath.Join(ns2Dir, "state.json"), data, 0644)

	err := showAllStatus(env.App)
	if err != nil {
		t.Fatalf("showAllStatus with multiple namespaces failed: %v", err)
	}

	// JSON mode
	env.App.JSON = true
	defer func() { env.App.JSON = false }()
	err = showAllStatus(env.App)
	if err != nil {
		t.Fatalf("showAllStatus JSON with multiple namespaces failed: %v", err)
	}
}

func TestCountDescendants(t *testing.T) {
	idx := state.NewRootIndex()
	idx.Nodes["root"] = state.IndexEntry{
		Name: "Root", Type: state.NodeOrchestrator, Children: []string{"root/a", "root/b"},
	}
	idx.Nodes["root/a"] = state.IndexEntry{
		Name: "A", Type: state.NodeOrchestrator, Parent: "root", Children: []string{"root/a/x"},
	}
	idx.Nodes["root/a/x"] = state.IndexEntry{
		Name: "X", Type: state.NodeLeaf, Parent: "root/a",
	}
	idx.Nodes["root/b"] = state.IndexEntry{
		Name: "B", Type: state.NodeLeaf, Parent: "root",
	}

	if got := countDescendants(idx, "root"); got != 3 {
		t.Errorf("expected 3 descendants, got %d", got)
	}
	if got := countDescendants(idx, "root/a"); got != 1 {
		t.Errorf("expected 1 descendant, got %d", got)
	}
	if got := countDescendants(idx, "root/b"); got != 0 {
		t.Errorf("expected 0 descendants for leaf, got %d", got)
	}
	if got := countDescendants(idx, "nonexistent"); got != 0 {
		t.Errorf("expected 0 for missing node, got %d", got)
	}
}

func TestShowTreeStatus_SubtaskIndentation(t *testing.T) {
	env := newStatusTestEnv(t)

	// Add hierarchical tasks to the leaf node
	nodeDir := filepath.Join(env.ProjectsDir, "my-project")
	ns := state.NewNodeState("my-project", "My Project", state.NodeLeaf)
	ns.State = state.StatusInProgress
	ns.Tasks = []state.Task{
		{ID: "task-0001", Description: "Parent task", State: state.StatusInProgress},
		{ID: "task-0001.0001", Description: "First subtask", State: state.StatusComplete},
		{ID: "task-0001.0002", Description: "Second subtask", State: state.StatusNotStarted},
		{ID: "task-0002", Description: "Top-level task", State: state.StatusNotStarted},
		{ID: "audit", Description: "Audit", State: state.StatusNotStarted, IsAudit: true},
	}
	nsData, _ := json.MarshalIndent(ns, "", "  ")
	_ = os.WriteFile(filepath.Join(nodeDir, "state.json"), nsData, 0644)

	idx, _ := state.LoadRootIndex(filepath.Join(env.ProjectsDir, "state.json"))
	// Should not panic and should render hierarchical tasks
	if err := showTreeStatus(env.App, idx, ""); err != nil {
		t.Fatalf("showTreeStatus with subtasks failed: %v", err)
	}
}
