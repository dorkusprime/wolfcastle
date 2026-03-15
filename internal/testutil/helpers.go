// Package testutil provides shared test helpers for Wolfcastle tests.
// It reduces duplication across test files and standardizes common
// setup operations like writing JSON, building test trees, and
// creating temporary .wolfcastle/ directories.
package testutil

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	"github.com/dorkusprime/wolfcastle/internal/config"
	"github.com/dorkusprime/wolfcastle/internal/state"
)

// WriteJSON marshals v to JSON and writes it to path, creating parent
// directories as needed. Fails the test on error.
func WriteJSON(t *testing.T, path string, v any) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0755); err != nil {
		t.Fatalf("creating directory for %s: %v", path, err)
	}
	data, err := json.MarshalIndent(v, "", "  ")
	if err != nil {
		t.Fatalf("marshaling JSON for %s: %v", path, err)
	}
	if err := os.WriteFile(path, data, 0644); err != nil {
		t.Fatalf("writing %s: %v", path, err)
	}
}

// ReadJSON reads a JSON file and unmarshals it into v. Fails the test on error.
func ReadJSON(t *testing.T, path string, v any) {
	t.Helper()
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("reading %s: %v", path, err)
	}
	if err := json.Unmarshal(data, v); err != nil {
		t.Fatalf("unmarshaling %s: %v", path, err)
	}
}

// SetupWolfcastle creates a minimal .wolfcastle/ directory structure
// in a temp dir with a default config, identity, and empty root index.
// Returns the path to the .wolfcastle/ directory and the namespace name.
func SetupWolfcastle(t *testing.T) (wolfcastleDir, namespace string) {
	t.Helper()
	root := t.TempDir()
	wolfcastleDir = filepath.Join(root, ".wolfcastle")
	namespace = "test-machine"

	dirs := []string{
		"base/prompts",
		"base/rules",
		"base/audits",
		"custom",
		"local",
		"archive",
		"docs/decisions",
		"docs/specs",
		"logs",
		filepath.Join("projects", namespace),
	}
	for _, d := range dirs {
		if err := os.MkdirAll(filepath.Join(wolfcastleDir, d), 0755); err != nil {
			t.Fatalf("creating %s: %v", d, err)
		}
	}

	// Write default config to base/config.json
	cfg := config.Defaults()
	cfg.Identity = nil
	WriteJSON(t, filepath.Join(wolfcastleDir, "base", "config.json"), cfg)

	// Write empty custom/config.json
	WriteJSON(t, filepath.Join(wolfcastleDir, "custom", "config.json"), map[string]any{})

	// Write local config with test identity
	localCfg := map[string]any{
		"identity": map[string]any{
			"user":    "test",
			"machine": "machine",
		},
	}
	WriteJSON(t, filepath.Join(wolfcastleDir, "local", "config.json"), localCfg)

	// Write empty root index
	idx := state.NewRootIndex()
	WriteJSON(t, filepath.Join(wolfcastleDir, "projects", namespace, "state.json"), idx)

	return wolfcastleDir, namespace
}

// SetupTree creates a .wolfcastle/ directory with a pre-built tree structure.
// Returns the wolfcastle dir, namespace, and root index.
//
// The tree has the structure:
//
//	root-project (orchestrator)
//	  child-a (leaf, tasks: task-0001 not_started, audit not_started)
//	  child-b (leaf, tasks: task-0001 not_started, audit not_started)
func SetupTree(t *testing.T) (wolfcastleDir, namespace string, idx *state.RootIndex) {
	t.Helper()
	wolfcastleDir, namespace = SetupWolfcastle(t)
	projectsDir := filepath.Join(wolfcastleDir, "projects", namespace)

	idx = state.NewRootIndex()
	idx.RootID = "root-project"
	idx.RootName = "Root Project"
	idx.RootState = state.StatusNotStarted
	idx.Root = []string{"root-project"}

	// Root orchestrator
	idx.Nodes["root-project"] = state.IndexEntry{
		Name:     "Root Project",
		Type:     state.NodeOrchestrator,
		State:    state.StatusNotStarted,
		Address:  "root-project",
		Children: []string{"root-project/child-a", "root-project/child-b"},
	}

	rootNode := state.NewNodeState("root-project", "Root Project", state.NodeOrchestrator)
	rootNode.Children = []state.ChildRef{
		{ID: "child-a", Address: "root-project/child-a", State: state.StatusNotStarted},
		{ID: "child-b", Address: "root-project/child-b", State: state.StatusNotStarted},
	}
	rootDir := filepath.Join(projectsDir, "root-project")
	_ = os.MkdirAll(rootDir, 0755)
	WriteJSON(t, filepath.Join(rootDir, "state.json"), rootNode)

	// Child A (leaf)
	idx.Nodes["root-project/child-a"] = state.IndexEntry{
		Name:     "Child A",
		Type:     state.NodeLeaf,
		State:    state.StatusNotStarted,
		Address:  "root-project/child-a",
		Parent:   "root-project",
		Children: []string{},
	}

	childA := state.NewNodeState("child-a", "Child A", state.NodeLeaf)
	childA.Tasks = []state.Task{
		{ID: "task-0001", Description: "First task", State: state.StatusNotStarted},
		{ID: "audit", Description: "Verify work", State: state.StatusNotStarted, IsAudit: true},
	}
	childADir := filepath.Join(projectsDir, "root-project", "child-a")
	_ = os.MkdirAll(childADir, 0755)
	WriteJSON(t, filepath.Join(childADir, "state.json"), childA)

	// Child B (leaf)
	idx.Nodes["root-project/child-b"] = state.IndexEntry{
		Name:     "Child B",
		Type:     state.NodeLeaf,
		State:    state.StatusNotStarted,
		Address:  "root-project/child-b",
		Parent:   "root-project",
		Children: []string{},
	}

	childB := state.NewNodeState("child-b", "Child B", state.NodeLeaf)
	childB.Tasks = []state.Task{
		{ID: "task-0001", Description: "First task", State: state.StatusNotStarted},
		{ID: "audit", Description: "Verify work", State: state.StatusNotStarted, IsAudit: true},
	}
	childBDir := filepath.Join(projectsDir, "root-project", "child-b")
	_ = os.MkdirAll(childBDir, 0755)
	WriteJSON(t, filepath.Join(childBDir, "state.json"), childB)

	// Save root index
	WriteJSON(t, filepath.Join(projectsDir, "state.json"), idx)

	return wolfcastleDir, namespace, idx
}

// LoadRootIndex reads the root index from a wolfcastle directory.
func LoadRootIndex(t *testing.T, wolfcastleDir, namespace string) *state.RootIndex {
	t.Helper()
	var idx state.RootIndex
	ReadJSON(t, filepath.Join(wolfcastleDir, "projects", namespace, "state.json"), &idx)
	return &idx
}

// LoadNode reads a node's state from disk.
func LoadNode(t *testing.T, wolfcastleDir, namespace, addr string) *state.NodeState {
	t.Helper()
	var ns state.NodeState
	ReadJSON(t, filepath.Join(wolfcastleDir, "projects", namespace, addr, "state.json"), &ns)
	return &ns
}

// SaveNode writes a node's state to disk.
func SaveNode(t *testing.T, wolfcastleDir, namespace, addr string, ns *state.NodeState) {
	t.Helper()
	WriteJSON(t, filepath.Join(wolfcastleDir, "projects", namespace, addr, "state.json"), ns)
}
