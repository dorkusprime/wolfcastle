package daemon

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"testing"

	"github.com/dorkusprime/wolfcastle/internal/config"
	"github.com/dorkusprime/wolfcastle/internal/state"
)

// ═══════════════════════════════════════════════════════════════════════════
// Bug #2: Root index not clobbered by daemon propagation
// ═══════════════════════════════════════════════════════════════════════════
//
// When the intake model creates new projects (via `wolfcastle project create`)
// during a daemon iteration, the daemon's propagateState must re-read the
// root index from disk before writing, so those new projects are preserved.

func TestPropagateState_PreservesNewNodesAddedDuringIteration(t *testing.T) {
	d := testDaemon(t)
	projDir := d.Store.Dir()

	// Set up initial index with one leaf node.
	idx := state.NewRootIndex()
	idx.Root = []string{"node-a"}
	idx.Nodes["node-a"] = state.IndexEntry{
		Name:    "Node A",
		Type:    state.NodeLeaf,
		State:   state.StatusInProgress,
		Address: "node-a",
	}
	writeJSON(t, filepath.Join(d.Store.Dir(), "state.json"), idx)

	// Create the node-a state file so propagation can load it.
	nsA := state.NewNodeState("node-a", "Node A", state.NodeLeaf)
	nsA.Tasks = []state.Task{
		{ID: "task-0001", Description: "work", State: state.StatusComplete},
	}
	writeJSON(t, filepath.Join(projDir, "node-a", "state.json"), nsA)

	// Simulate the intake model adding a new project to the root index
	// on disk while the daemon holds a stale in-memory copy.
	diskIdx, err := d.Store.ReadIndex()
	if err != nil {
		t.Fatal(err)
	}
	diskIdx.Root = append(diskIdx.Root, "node-b")
	diskIdx.Nodes["node-b"] = state.IndexEntry{
		Name:    "Node B",
		Type:    state.NodeLeaf,
		State:   state.StatusNotStarted,
		Address: "node-b",
	}
	writeJSON(t, filepath.Join(d.Store.Dir(), "state.json"), diskIdx)

	// Now propagate using the daemon's stale in-memory idx (which
	// knows nothing about node-b).
	if err := d.propagateState(d.Logger, "node-a", state.StatusComplete, idx); err != nil {
		t.Fatalf("propagateState error: %v", err)
	}

	// Re-read the index from disk and verify node-b survived.
	final, err := d.Store.ReadIndex()
	if err != nil {
		t.Fatal(err)
	}
	if _, ok := final.Nodes["node-b"]; !ok {
		t.Error("node-b was clobbered by propagation; expected it to survive")
	}
	if _, ok := final.Nodes["node-a"]; !ok {
		t.Error("node-a should still be in the index")
	}

	// The in-memory idx should also have been updated.
	if _, ok := idx.Nodes["node-b"]; !ok {
		t.Error("in-memory index should have been updated with node-b")
	}
}

func TestPropagateState_FallsBackToInMemoryOnDiskError(t *testing.T) {
	d := testDaemon(t)
	projDir := d.Store.Dir()

	// Set up initial index with one leaf node.
	idx := state.NewRootIndex()
	idx.Root = []string{"node-a"}
	idx.Nodes["node-a"] = state.IndexEntry{
		Name:    "Node A",
		Type:    state.NodeLeaf,
		State:   state.StatusInProgress,
		Address: "node-a",
	}
	writeJSON(t, filepath.Join(d.Store.Dir(), "state.json"), idx)

	// Create node-a state file.
	nsA := state.NewNodeState("node-a", "Node A", state.NodeLeaf)
	nsA.Tasks = []state.Task{
		{ID: "task-0001", Description: "work", State: state.StatusComplete},
	}
	writeJSON(t, filepath.Join(projDir, "node-a", "state.json"), nsA)

	// Delete the root index so LoadRootIndex fails on re-read.
	_ = os.Remove(filepath.Join(d.Store.Dir(), "state.json"))

	// propagateState should fall back to the in-memory idx.
	// This will fail to save (directory still exists), but the key
	// behavior is that it doesn't panic or return a load error.
	// The save may or may not succeed depending on the fallback path;
	// we only verify no panic and that the in-memory idx gets the update.
	_ = d.propagateState(d.Logger, "node-a", state.StatusComplete, idx)

	if idx.Nodes["node-a"].State != state.StatusComplete {
		t.Errorf("expected in-memory state to be complete, got %s", idx.Nodes["node-a"].State)
	}
}

// ═══════════════════════════════════════════════════════════════════════════
// Bug #4: WOLFCASTLE_COMPLETE printed exactly once
// ═══════════════════════════════════════════════════════════════════════════
//
// When all tasks are complete, the daemon should print WOLFCASTLE_COMPLETE
// exactly once across multiple RunOnce calls, not on every poll cycle.

func TestRunOnce_CompletePrintedOnce(t *testing.T) {
	d := testDaemon(t)
	d.Config.Git.VerifyBranch = false
	projDir := d.Store.Dir()

	// Set up a fully-complete tree: one leaf, one task, already done.
	idx := state.NewRootIndex()
	idx.Root = []string{"done-node"}
	idx.Nodes["done-node"] = state.IndexEntry{
		Name:    "Done Node",
		Type:    state.NodeLeaf,
		State:   state.StatusComplete,
		Address: "done-node",
	}
	writeJSON(t, filepath.Join(d.Store.Dir(), "state.json"), idx)

	ns := state.NewNodeState("done-node", "Done Node", state.NodeLeaf)
	ns.State = state.StatusComplete
	ns.Tasks = []state.Task{
		{ID: "task-0001", Description: "already done", State: state.StatusComplete},
		{ID: "audit", Description: "audit", State: state.StatusComplete, IsAudit: true},
	}
	writeJSON(t, filepath.Join(projDir, "done-node", "state.json"), ns)

	writePromptFile(t, d.WolfcastleDir, "stages/execute.md")

	// Start a logger iteration so idle_reason records can be written.
	_ = d.Logger.StartIterationWithPrefix("test")

	// Run three iterations; all should return NoWork.
	for i := 0; i < 3; i++ {
		result, err := d.RunOnce(context.Background())
		if err != nil {
			t.Fatalf("iteration %d error: %v", i, err)
		}
		if result != IterationNoWork {
			t.Fatalf("iteration %d: expected NoWork, got %v", i, result)
		}
	}

	d.Logger.Close()

	// Read the log file and count idle_reason records containing WOLFCASTLE_COMPLETE.
	logFiles, _ := filepath.Glob(filepath.Join(d.Logger.LogDir, "*.jsonl"))
	count := 0
	for _, f := range logFiles {
		data, err := os.ReadFile(f)
		if err != nil {
			continue
		}
		for _, line := range bytes.Split(data, []byte("\n")) {
			if bytes.Contains(line, []byte(`"type":"idle_reason"`)) &&
				bytes.Contains(line, []byte("WOLFCASTLE_COMPLETE")) {
				count++
			}
		}
	}

	if count != 1 {
		t.Errorf("expected idle_reason with WOLFCASTLE_COMPLETE exactly once in logs, got %d", count)
	}
}

func TestRunOnce_CompleteResetWhenNewWorkAppears(t *testing.T) {
	d := testDaemon(t)
	d.Config.Git.VerifyBranch = false
	projDir := d.Store.Dir()

	// Start with a complete tree.
	idx := state.NewRootIndex()
	idx.Root = []string{"done-node"}
	idx.Nodes["done-node"] = state.IndexEntry{
		Name:    "Done Node",
		Type:    state.NodeLeaf,
		State:   state.StatusComplete,
		Address: "done-node",
	}
	writeJSON(t, filepath.Join(d.Store.Dir(), "state.json"), idx)

	ns := state.NewNodeState("done-node", "Done Node", state.NodeLeaf)
	ns.State = state.StatusComplete
	ns.Tasks = []state.Task{
		{ID: "task-0001", Description: "done", State: state.StatusComplete},
		{ID: "audit", Description: "audit", State: state.StatusComplete, IsAudit: true},
	}
	writeJSON(t, filepath.Join(projDir, "done-node", "state.json"), ns)

	writePromptFile(t, d.WolfcastleDir, "stages/execute.md")

	// Suppress stdout for this test.
	origStdout := os.Stdout
	_, w, _ := os.Pipe()
	os.Stdout = w

	// First call: prints WOLFCASTLE_COMPLETE, sets lastNoWorkMsg.
	result, err := d.RunOnce(context.Background())
	if err != nil {
		os.Stdout = origStdout
		t.Fatalf("iteration 1 error: %v", err)
	}
	if result != IterationNoWork {
		os.Stdout = origStdout
		t.Fatalf("iteration 1: expected NoWork, got %v", result)
	}
	if d.lastNoWorkMsg != "WOLFCASTLE_COMPLETE" {
		os.Stdout = origStdout
		t.Fatalf("expected lastNoWorkMsg to be WOLFCASTLE_COMPLETE, got %q", d.lastNoWorkMsg)
	}

	// Now add new work to the tree.
	idx2 := state.NewRootIndex()
	idx2.Root = []string{"done-node", "new-node"}
	idx2.Nodes["done-node"] = state.IndexEntry{
		Name:    "Done Node",
		Type:    state.NodeLeaf,
		State:   state.StatusComplete,
		Address: "done-node",
	}
	idx2.Nodes["new-node"] = state.IndexEntry{
		Name:    "New Node",
		Type:    state.NodeLeaf,
		State:   state.StatusNotStarted,
		Address: "new-node",
	}
	writeJSON(t, filepath.Join(d.Store.Dir(), "state.json"), idx2)

	nsNew := state.NewNodeState("new-node", "New Node", state.NodeLeaf)
	nsNew.Tasks = []state.Task{
		{ID: "task-0001", Description: "new work", State: state.StatusNotStarted},
		{ID: "audit", Description: "audit", State: state.StatusNotStarted, IsAudit: true},
	}
	writeJSON(t, filepath.Join(projDir, "new-node", "state.json"), nsNew)

	// Second call: finds work, so lastNoWorkMsg should reset.
	_ = d.Logger.StartIteration()
	result2, err := d.RunOnce(context.Background())
	d.Logger.Close()
	_ = w.Close()
	os.Stdout = origStdout

	if err != nil {
		t.Fatalf("iteration 2 error: %v", err)
	}
	if result2 != IterationDidWork {
		t.Fatalf("iteration 2: expected DidWork, got %v", result2)
	}
	if d.lastNoWorkMsg != "" {
		t.Errorf("lastNoWorkMsg should have been reset when new work appeared, got %q", d.lastNoWorkMsg)
	}
}

// ═══════════════════════════════════════════════════════════════════════════
// Bug #2 (extended): Full iteration round-trip doesn't clobber new nodes
// ═══════════════════════════════════════════════════════════════════════════
//
// A more realistic version: the mock model is a shell script that adds a
// new node to the root index (simulating what the intake model does via
// `wolfcastle project create`), then emits WOLFCASTLE_COMPLETE. After the
// iteration finishes and propagation runs, the new node must still exist.

func TestIntegration_PropagatePreservesNodesAddedDuringModelRun(t *testing.T) {
	d := testDaemon(t)
	d.Config.Git.VerifyBranch = false
	projDir := d.Store.Dir()

	setupLeafNode(t, d, "existing-node", []state.Task{
		{ID: "task-0001", Description: "do the thing", State: state.StatusNotStarted},
		{ID: "audit", Description: "audit", State: state.StatusNotStarted, IsAudit: true},
	})

	// The mock model script adds a new node to the root index on disk
	// before emitting WOLFCASTLE_COMPLETE. This simulates the intake
	// model calling `wolfcastle project create`.
	scriptFile := filepath.Join(t.TempDir(), "add-node.sh")
	rootIndexPath := filepath.Join(d.Store.Dir(), "state.json")
	newNodeDir := filepath.Join(projDir, "injected-node")

	// Build the new node state JSON.
	newNS := state.NewNodeState("injected-node", "Injected Node", state.NodeLeaf)
	newNS.Tasks = []state.Task{
		{ID: "task-0001", Description: "injected work", State: state.StatusNotStarted},
	}
	newNSData, _ := json.MarshalIndent(newNS, "", "  ")

	script := fmt.Sprintf(`#!/bin/sh
# Read stdin (prompt) and discard it
cat > /dev/null

# Add a new node to the root index (simulating intake model)
mkdir -p %q
cat > %q <<'NODEJSON'
%s
NODEJSON

# Read current root index, add the new node
python3 -c "
import json, sys
with open('%s') as f:
    idx = json.load(f)
idx['root'].append('injected-node')
idx['nodes']['injected-node'] = {
    'name': 'Injected Node',
    'type': 'leaf',
    'state': 'not_started',
    'address': 'injected-node'
}
with open('%s', 'w') as f:
    json.dump(idx, f, indent=2)
"

echo "WOLFCASTLE_COMPLETE"
`, newNodeDir, filepath.Join(newNodeDir, "state.json"), string(newNSData),
		rootIndexPath, rootIndexPath)

	if err := os.WriteFile(scriptFile, []byte(script), 0755); err != nil {
		t.Fatal(err)
	}

	d.Config.Models["echo"] = config.ModelDef{
		Command: "sh",
		Args:    []string{scriptFile},
	}
	writePromptFile(t, d.WolfcastleDir, "stages/execute.md")

	_ = d.Logger.StartIteration()
	result, err := d.RunOnce(context.Background())
	d.Logger.Close()
	if err != nil {
		t.Fatalf("iteration error: %v", err)
	}
	if result != IterationDidWork {
		t.Fatalf("expected DidWork, got %v", result)
	}

	// Verify the injected node survived propagation.
	final, err := d.Store.ReadIndex()
	if err != nil {
		t.Fatal(err)
	}
	if _, ok := final.Nodes["injected-node"]; !ok {
		t.Error("injected-node was clobbered by propagation; the re-read logic failed")
	}
	if _, ok := final.Nodes["existing-node"]; !ok {
		t.Error("existing-node should still be present")
	}
}
