package validate

import (
	"os"
	"path/filepath"
	"runtime"
	"testing"

	"github.com/dorkusprime/wolfcastle/internal/state"
)

// ── FixWithVerification: ApplyDeterministicFixes error via chmod ────

func TestFixWithVerification_ApplyFixesError_ReadOnlyStateFiles(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("permission test not reliable on Windows")
	}
	t.Parallel()

	dir := t.TempDir()
	idx := state.NewRootIndex()

	// Create a node with a missing audit task. Triggers MISSING_AUDIT_TASK fix.
	leafDir := filepath.Join(dir, "perm-node")
	_ = os.MkdirAll(leafDir, 0755)
	ns := state.NewNodeState("perm-node", "Perm", state.NodeLeaf)
	ns.Tasks = []state.Task{
		{ID: "task-0001", Description: "work", State: state.StatusNotStarted},
	}
	_ = state.SaveNodeState(filepath.Join(leafDir, "state.json"), ns)

	idx.Root = []string{"perm-node"}
	idx.Nodes["perm-node"] = state.IndexEntry{
		Name: "Perm", Type: state.NodeLeaf, State: state.StatusNotStarted, Address: "perm-node",
	}
	idxPath := filepath.Join(dir, "state.json")
	_ = state.SaveRootIndex(idxPath, idx)

	// Lock the leaf directory so SaveNodeState fails during fix.
	_ = os.Chmod(leafDir, 0555)
	t.Cleanup(func() { _ = os.Chmod(leafDir, 0755) })

	_, _, err := FixWithVerification(dir, idxPath, DefaultNodeLoader(dir))
	if err == nil {
		t.Error("expected error when state file is read-only during fix")
	}
}
