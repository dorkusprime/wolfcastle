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
	env.App.JSONOutput = true
	defer func() { env.App.JSONOutput = false }()
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
	env.App.JSONOutput = true
	defer func() { env.App.JSONOutput = false }()
	err = showAllStatus(env.App)
	if err != nil {
		t.Fatalf("showAllStatus JSON with multiple namespaces failed: %v", err)
	}
}
