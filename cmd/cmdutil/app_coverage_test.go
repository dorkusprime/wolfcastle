package cmdutil

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	"github.com/dorkusprime/wolfcastle/internal/state"
	"github.com/dorkusprime/wolfcastle/internal/tree"
)

// ═══════════════════════════════════════════════════════════════════════════
// PropagateState — error paths
// ═══════════════════════════════════════════════════════════════════════════

func TestPropagateState_BrokenRootIndex(t *testing.T) {
	tmp := t.TempDir()
	wcDir := filepath.Join(tmp, ".wolfcastle")
	ns := "test-dev"
	projDir := filepath.Join(wcDir, "projects", ns)
	_ = os.MkdirAll(projDir, 0755)

	// Write invalid JSON as the root index
	_ = os.WriteFile(filepath.Join(projDir, "state.json"), []byte("not json"), 0644)

	resolver := &tree.Resolver{WolfcastleDir: wcDir, Namespace: ns}
	a := &App{
		WolfcastleDir: wcDir,
		Resolver:      resolver,
	}

	err := a.PropagateState("some-node", "in_progress")
	if err == nil {
		t.Error("expected error when root index is broken")
	}
}

func TestPropagateState_BrokenNodeState(t *testing.T) {
	tmp := t.TempDir()
	wcDir := filepath.Join(tmp, ".wolfcastle")
	ns := "test-dev"
	projDir := filepath.Join(wcDir, "projects", ns)
	_ = os.MkdirAll(projDir, 0755)

	// Create valid root index with a node that has a parent
	idx := state.NewRootIndex()
	idx.Nodes["parent"] = state.IndexEntry{
		Name:     "Parent",
		Type:     state.NodeOrchestrator,
		State:    state.StatusNotStarted,
		Address:  "parent",
		Children: []string{"parent/child"},
	}
	idx.Nodes["parent/child"] = state.IndexEntry{
		Name:     "Child",
		Type:     state.NodeLeaf,
		State:    state.StatusNotStarted,
		Address:  "parent/child",
		Parent:   "parent",
		Children: []string{},
	}
	idxData, _ := json.MarshalIndent(idx, "", "  ")
	_ = os.WriteFile(filepath.Join(projDir, "state.json"), idxData, 0644)

	// Create parent dir but write invalid JSON for node state
	parentDir := filepath.Join(projDir, "parent")
	_ = os.MkdirAll(parentDir, 0755)
	_ = os.WriteFile(filepath.Join(parentDir, "state.json"), []byte("broken json"), 0644)

	// Don't create child state file at all
	childDir := filepath.Join(projDir, "parent", "child")
	_ = os.MkdirAll(childDir, 0755)
	_ = os.WriteFile(filepath.Join(childDir, "state.json"), []byte("also broken"), 0644)

	resolver := &tree.Resolver{WolfcastleDir: wcDir, Namespace: ns}
	a := &App{
		WolfcastleDir: wcDir,
		Resolver:      resolver,
	}

	err := a.PropagateState("parent/child", "in_progress")
	if err == nil {
		t.Error("expected error when node state files are broken")
	}
}

func TestPropagateState_InvalidNodeAddress(t *testing.T) {
	tmp := t.TempDir()
	wcDir := filepath.Join(tmp, ".wolfcastle")
	ns := "test-dev"
	projDir := filepath.Join(wcDir, "projects", ns)
	_ = os.MkdirAll(projDir, 0755)

	// Create valid root index with a node
	idx := state.NewRootIndex()
	idx.Nodes["valid-node"] = state.IndexEntry{
		Name:    "Valid",
		Type:    state.NodeLeaf,
		State:   state.StatusNotStarted,
		Address: "valid-node",
	}
	idxData, _ := json.MarshalIndent(idx, "", "  ")
	_ = os.WriteFile(filepath.Join(projDir, "state.json"), idxData, 0644)

	nodeDir := filepath.Join(projDir, "valid-node")
	_ = os.MkdirAll(nodeDir, 0755)
	ns2 := state.NewNodeState("valid-node", "Valid", state.NodeLeaf)
	nsData, _ := json.MarshalIndent(ns2, "", "  ")
	_ = os.WriteFile(filepath.Join(nodeDir, "state.json"), nsData, 0644)

	resolver := &tree.Resolver{WolfcastleDir: wcDir, Namespace: ns}
	a := &App{
		WolfcastleDir: wcDir,
		Resolver:      resolver,
	}

	// Valid node address propagation should work
	err := a.PropagateState("valid-node", "in_progress")
	if err != nil {
		t.Fatalf("PropagateState for valid node failed: %v", err)
	}
}
