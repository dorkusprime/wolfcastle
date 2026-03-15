package daemon

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"testing"

	"github.com/dorkusprime/wolfcastle/internal/config"
	"github.com/dorkusprime/wolfcastle/internal/state"
)

// TestMultiIteration_YieldThenComplete simulates a model that yields on
// the first invocation and completes on the second. This exercises:
// - Task claim on iteration 1
// - YIELD leaves task in_progress
// - Iteration 2 resumes without re-claiming
// - COMPLETE transitions task to complete
func TestMultiIteration_YieldThenComplete(t *testing.T) {
	d := testDaemon(t)
	d.Config.Git.VerifyBranch = false
	d.Config.Daemon.MaxIterations = 5

	// Set up a leaf node with one task
	projDir := d.Resolver.ProjectsDir()
	nodeDir := filepath.Join(projDir, "test-node")
	_ = os.MkdirAll(nodeDir, 0755)

	ns := state.NewNodeState("test-node", "Test Node", state.NodeLeaf)
	ns.Tasks = []state.Task{
		{ID: "task-1", Description: "test task", State: state.StatusNotStarted},
		{ID: "audit", Description: "audit", State: state.StatusNotStarted, IsAudit: true},
	}
	data, _ := json.MarshalIndent(ns, "", "  ")
	_ = os.WriteFile(filepath.Join(nodeDir, "state.json"), data, 0644)

	idx := state.NewRootIndex()
	idx.Root = []string{"test-node"}
	idx.Nodes["test-node"] = state.IndexEntry{
		Name:    "Test Node",
		Type:    state.NodeLeaf,
		State:   state.StatusNotStarted,
		Address: "test-node",
	}
	idxData, _ := json.MarshalIndent(idx, "", "  ")
	_ = os.WriteFile(filepath.Join(projDir, "state.json"), idxData, 0644)

	// Use a counter file to vary output per invocation.
	// First call: YIELD. Second call: COMPLETE.
	counterFile := filepath.Join(t.TempDir(), "counter")
	_ = os.WriteFile(counterFile, []byte("0"), 0644)

	d.Config.Models["echo"] = config.ModelDef{
		Command: "sh",
		Args: []string{"-c", fmt.Sprintf(
			`n=$(cat %s); echo $((n+1)) > %s; if [ "$n" = "0" ]; then echo WOLFCASTLE_YIELD; else echo WOLFCASTLE_COMPLETE; fi`,
			counterFile, counterFile,
		)},
	}
	writePromptFile(t, d.WolfcastleDir, "execute.md")

	// Run iteration 1: should claim and yield
	_ = d.Logger.StartIteration()
	result1, err1 := d.RunOnce(context.Background())
	d.Logger.Close()
	if err1 != nil {
		t.Fatalf("iteration 1 error: %v", err1)
	}
	if result1 != IterationDidWork {
		t.Fatalf("iteration 1: expected DidWork, got %v", result1)
	}

	// Verify task is in_progress after yield
	ns1, _ := state.LoadNodeState(filepath.Join(nodeDir, "state.json"))
	if ns1.Tasks[0].State != state.StatusInProgress {
		t.Errorf("after yield: expected in_progress, got %s", ns1.Tasks[0].State)
	}

	// Run iteration 2: should resume (no re-claim) and complete
	_ = d.Logger.StartIteration()
	result2, err2 := d.RunOnce(context.Background())
	d.Logger.Close()
	if err2 != nil {
		t.Fatalf("iteration 2 error: %v", err2)
	}
	if result2 != IterationDidWork {
		t.Fatalf("iteration 2: expected DidWork, got %v", result2)
	}

	// Verify task is complete
	ns2, _ := state.LoadNodeState(filepath.Join(nodeDir, "state.json"))
	if ns2.Tasks[0].State != state.StatusComplete {
		t.Errorf("after complete: expected complete, got %s", ns2.Tasks[0].State)
	}
}

// TestMultiIteration_NoFalseMarkerFromPromptEcho verifies that marker
// names mentioned inside larger text (prompt echo, instructions) don't
// trigger false terminal marker detection. Only a standalone line matches.
func TestMultiIteration_NoFalseMarkerFromPromptEcho(t *testing.T) {
	d := testDaemon(t)
	d.Config.Git.VerifyBranch = false

	projDir := d.Resolver.ProjectsDir()
	nodeDir := filepath.Join(projDir, "test-node")
	_ = os.MkdirAll(nodeDir, 0755)

	ns := state.NewNodeState("test-node", "Test Node", state.NodeLeaf)
	ns.Tasks = []state.Task{
		{ID: "task-1", Description: "test task", State: state.StatusNotStarted},
		{ID: "audit", Description: "audit", State: state.StatusNotStarted, IsAudit: true},
	}
	data, _ := json.MarshalIndent(ns, "", "  ")
	_ = os.WriteFile(filepath.Join(nodeDir, "state.json"), data, 0644)

	idx := state.NewRootIndex()
	idx.Root = []string{"test-node"}
	idx.Nodes["test-node"] = state.IndexEntry{
		Name:    "Test Node",
		Type:    state.NodeLeaf,
		State:   state.StatusNotStarted,
		Address: "test-node",
	}
	idxData, _ := json.MarshalIndent(idx, "", "  ")
	_ = os.WriteFile(filepath.Join(projDir, "state.json"), idxData, 0644)

	// Output contains marker names embedded in sentences (prompt echo).
	// Only the standalone WOLFCASTLE_COMPLETE on the last line should match.
	d.Config.Models["echo"] = config.ModelDef{
		Command: "sh",
		Args:    []string{"-c", `printf 'When done, emit WOLFCASTLE_COMPLETE on its own line.\nIf stuck, emit WOLFCASTLE_YIELD.\nI finished the work.\nWOLFCASTLE_COMPLETE\n'`},
	}
	writePromptFile(t, d.WolfcastleDir, "execute.md")

	_ = d.Logger.StartIteration()
	result, err := d.RunOnce(context.Background())
	d.Logger.Close()
	if err != nil {
		t.Fatalf("iteration error: %v", err)
	}
	if result != IterationDidWork {
		t.Fatalf("expected DidWork, got %v", result)
	}

	ns1, _ := state.LoadNodeState(filepath.Join(nodeDir, "state.json"))
	if ns1.Tasks[0].State != state.StatusComplete {
		t.Errorf("expected complete (standalone marker on last line), got %s", ns1.Tasks[0].State)
	}
}

// TestScanTerminalMarker verifies the line-by-line marker scanner.
func TestScanTerminalMarker(t *testing.T) {
	tests := []struct {
		name   string
		input  string
		expect string
	}{
		{"standalone complete", "WOLFCASTLE_COMPLETE", "WOLFCASTLE_COMPLETE"},
		{"standalone yield", "WOLFCASTLE_YIELD", "WOLFCASTLE_YIELD"},
		{"standalone blocked", "WOLFCASTLE_BLOCKED", "WOLFCASTLE_BLOCKED"},
		{"with leading text", "some text WOLFCASTLE_COMPLETE", "WOLFCASTLE_COMPLETE"},
		{"embedded in sentence", "emit WOLFCASTLE_COMPLETE on its own line", ""},
		{"embedded in JSON", `{"text":"WOLFCASTLE_COMPLETE"}`, ""},
		{"no marker", "just regular output", ""},
		{"empty", "", ""},
		{"multiline with marker last", "line1\nline2\nWOLFCASTLE_COMPLETE", "WOLFCASTLE_COMPLETE"},
		{"multiline embedded only", "use WOLFCASTLE_YIELD when pausing\nnormal output", ""},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := scanTerminalMarker(tt.input)
			if got != tt.expect {
				t.Errorf("scanTerminalMarker(%q) = %q, want %q", tt.input, got, tt.expect)
			}
		})
	}
}
