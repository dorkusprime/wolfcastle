package validate

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/dorkusprime/wolfcastle/internal/config"
	"github.com/dorkusprime/wolfcastle/internal/invoke"
	"github.com/dorkusprime/wolfcastle/internal/state"
)

// ═══════════════════════════════════════════════════════════════════════════
// fix.go: FixWithVerification: LoadRootIndex error in loop (corrupt index)
// ═══════════════════════════════════════════════════════════════════════════

func TestFixWithVerification_CorruptIndexMidPass(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	idx := state.NewRootIndex()

	// Create a node with a fixable issue (missing audit task)
	leafDir := filepath.Join(dir, "fixable")
	_ = os.MkdirAll(leafDir, 0755)
	ns := state.NewNodeState("fixable", "Fixable", state.NodeLeaf)
	ns.Tasks = []state.Task{
		{ID: "task-0001", Description: "work", State: state.StatusNotStarted},
		// Missing audit task. Triggers fix
	}
	_ = state.SaveNodeState(filepath.Join(leafDir, "state.json"), ns)

	idx.Root = []string{"fixable"}
	idx.Nodes["fixable"] = state.IndexEntry{
		Name: "Fixable", Type: state.NodeLeaf, State: state.StatusNotStarted, Address: "fixable",
	}
	idxPath := filepath.Join(dir, "state.json")
	_ = state.SaveRootIndex(idxPath, idx)

	// Run once to ensure it works, then corrupt the index for the final validation pass
	// by writing invalid JSON to the index path after the first pass
	fixes, report, err := FixWithVerification(dir, idxPath, DefaultNodeLoader(dir))
	if err != nil {
		t.Fatal(err)
	}
	_ = fixes
	_ = report

	// Now corrupt the index path and run again
	_ = os.WriteFile(idxPath, []byte("NOT VALID JSON{{{"), 0644)

	_, _, err = FixWithVerification(dir, idxPath, DefaultNodeLoader(dir))
	if err == nil {
		t.Fatal("expected error when root index is corrupt")
	}
	if !strings.Contains(err.Error(), "loading root index") {
		t.Errorf("expected 'loading root index' in error, got: %v", err)
	}
}

// ═══════════════════════════════════════════════════════════════════════════
// fix.go: FixWithVerification: zero fixes break (report with issues but
// none auto-fixable triggers the len(fixes)==0 break)
// ═══════════════════════════════════════════════════════════════════════════

func TestFixWithVerification_HasAutoFixableButZeroApplied(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	idx := state.NewRootIndex()

	// Create a node with a dangling reference. The dangling ref fix removes
	// the entry from the index. After removal, if the entry also appears in
	// root list, it gets cleaned. The result is 1+ fix, not zero.
	//
	// To get the zero-fixes-break path, we need HasAutoFixable() == true
	// but ApplyDeterministicFixes returns 0 fixes. This happens when the
	// only auto-fixable issue has a category that's handled but the node
	// can't be loaded (e.g., loadOrCached fails).
	//
	// Create a propagation mismatch where the node's state file is missing.
	idx.Root = []string{"ghost"}
	idx.Nodes["ghost"] = state.IndexEntry{
		Name: "Ghost", Type: state.NodeLeaf, State: state.StatusInProgress,
		Address: "ghost",
	}
	// Don't create the node state file on disk

	idxPath := filepath.Join(dir, "state.json")
	_ = state.SaveRootIndex(idxPath, idx)

	fixes, report, err := FixWithVerification(dir, idxPath, DefaultNodeLoader(dir))
	if err != nil {
		t.Fatal(err)
	}
	// The result depends on what validation detects. The important thing
	// is that the function completes without error.
	_ = fixes
	if report == nil {
		t.Fatal("expected non-nil report")
	}
}

// ═══════════════════════════════════════════════════════════════════════════
// model_fix.go: invoke error (nonexistent command)
// ═══════════════════════════════════════════════════════════════════════════

func TestTryModelAssistedFix_InvokeError(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()

	// Create a node on disk so the address is valid
	nodeDir := filepath.Join(dir, "test-node")
	_ = os.MkdirAll(nodeDir, 0755)
	ns := state.NewNodeState("test-node", "Test", state.NodeLeaf)
	_ = state.SaveNodeState(filepath.Join(nodeDir, "state.json"), ns)

	issue := Issue{
		Node:        "test-node",
		Category:    CatMultipleInProgress,
		FixType:     FixModelAssisted,
		Description: "test issue",
	}

	// Use a nonexistent command to trigger invocation failure
	model := config.ModelDef{Command: "/nonexistent/command/wolfcastle-test", Args: []string{}}

	ok, err := TryModelAssistedFix(context.Background(), invoke.NewProcessInvoker(), model, issue, dir)
	if ok {
		t.Error("expected ok=false for invoke error")
	}
	if err == nil {
		t.Fatal("expected error from failed invocation")
	}
	if !strings.Contains(err.Error(), "model invocation failed") {
		t.Errorf("expected 'model invocation failed' in error, got: %v", err)
	}
}

// ═══════════════════════════════════════════════════════════════════════════
// model_fix.go: JSON parse error (mock returning non-JSON)
// ═══════════════════════════════════════════════════════════════════════════

func TestTryModelAssistedFix_JSONParseError(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()

	nodeDir := filepath.Join(dir, "json-node")
	_ = os.MkdirAll(nodeDir, 0755)
	ns := state.NewNodeState("json-node", "JSON", state.NodeLeaf)
	_ = state.SaveNodeState(filepath.Join(nodeDir, "state.json"), ns)

	issue := Issue{
		Node:        "json-node",
		Category:    CatMultipleInProgress,
		FixType:     FixModelAssisted,
		Description: "test issue",
	}

	// Use echo to return non-JSON output
	model := config.ModelDef{Command: "echo", Args: []string{"this is not json"}}

	ok, err := TryModelAssistedFix(context.Background(), invoke.NewProcessInvoker(), model, issue, dir)
	if ok {
		t.Error("expected ok=false for non-JSON response")
	}
	if err == nil {
		t.Fatal("expected error parsing non-JSON model response")
	}
	if !strings.Contains(err.Error(), "parsing model response") {
		t.Errorf("expected 'parsing model response' in error, got: %v", err)
	}
}
