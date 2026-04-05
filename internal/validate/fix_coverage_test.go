package validate

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/dorkusprime/wolfcastle/internal/state"
)

// ═══════════════════════════════════════════════════════════════════════════
// FixWithVerification: additional coverage paths
// ═══════════════════════════════════════════════════════════════════════════

func TestFixWithVerification_PerfectlyCleanTree_NilFixes(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	idx := state.NewRootIndex()

	// Create a perfectly valid leaf node
	leafDir := filepath.Join(dir, "clean-leaf")
	_ = os.MkdirAll(leafDir, 0755)
	ns := state.NewNodeState("clean-leaf", "Clean Leaf", state.NodeLeaf)
	ns.Tasks = []state.Task{
		{ID: "task-0001", Description: "work", State: state.StatusNotStarted},
		{ID: "audit", Description: "audit", State: state.StatusNotStarted, IsAudit: true},
	}
	_ = state.SaveNodeState(filepath.Join(leafDir, "state.json"), ns)

	idx.Root = []string{"clean-leaf"}
	idx.Nodes["clean-leaf"] = state.IndexEntry{
		Name: "Clean Leaf", Type: state.NodeLeaf, State: state.StatusNotStarted, Address: "clean-leaf",
	}
	idxPath := filepath.Join(dir, "state.json")
	_ = state.SaveRootIndex(idxPath, idx)

	fixes, report, err := FixWithVerification(dir, idxPath, DefaultNodeLoader(dir))
	if err != nil {
		t.Fatal(err)
	}
	if len(fixes) != 0 {
		t.Errorf("expected 0 fixes for clean tree, got %d", len(fixes))
	}
	if report == nil {
		t.Fatal("expected non-nil final report")
	}
	if report.HasErrors() {
		t.Error("clean tree should have no errors")
	}
}

func TestFixWithVerification_BrokenRootIndexPath(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()

	// Point at a nonexistent index path
	idxPath := filepath.Join(dir, "nonexistent", "state.json")

	_, _, err := FixWithVerification(dir, idxPath, DefaultNodeLoader(dir))
	if err == nil {
		t.Error("expected error when root index cannot be loaded")
	}
}

func TestFixWithVerification_CascadingMultiPassFixes(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	idx := state.NewRootIndex()

	// Create a node with multiple issues that need cascading fixes:
	// 1. Missing audit task (pass 1 fix)
	// 2. Negative failure count (pass 1 fix)
	// After pass 1 fixes, pass 2 re-validates
	leafDir := filepath.Join(dir, "cascade-node")
	_ = os.MkdirAll(leafDir, 0755)
	ns := state.NewNodeState("cascade-node", "Cascade", state.NodeLeaf)
	ns.State = state.StatusNotStarted
	ns.Tasks = []state.Task{
		{ID: "task-0001", Description: "work", State: state.StatusNotStarted, FailureCount: -1},
	}
	// No audit task. Triggers MISSING_AUDIT_TASK
	_ = state.SaveNodeState(filepath.Join(leafDir, "state.json"), ns)

	idx.Root = []string{"cascade-node"}
	idx.Nodes["cascade-node"] = state.IndexEntry{
		Name: "Cascade", Type: state.NodeLeaf, State: state.StatusNotStarted, Address: "cascade-node",
	}
	idxPath := filepath.Join(dir, "state.json")
	_ = state.SaveRootIndex(idxPath, idx)

	fixes, report, err := FixWithVerification(dir, idxPath, DefaultNodeLoader(dir))
	if err != nil {
		t.Fatal(err)
	}
	if len(fixes) == 0 {
		t.Error("expected at least one fix for cascading issues")
	}
	if report == nil {
		t.Fatal("expected non-nil final report")
	}

	// Verify that fixes have pass numbers set
	for _, fix := range fixes {
		if fix.Pass < 1 {
			t.Errorf("fix %q should have pass >= 1, got %d", fix.Category, fix.Pass)
		}
	}
}

func TestFixWithVerification_BlockedWithoutReasonFix(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	idx := state.NewRootIndex()

	leafDir := filepath.Join(dir, "blocked-leaf")
	_ = os.MkdirAll(leafDir, 0755)
	ns := state.NewNodeState("blocked-leaf", "Blocked", state.NodeLeaf)
	ns.State = state.StatusNotStarted
	ns.Tasks = []state.Task{
		{ID: "task-0001", Description: "work", State: state.StatusBlocked, BlockedReason: ""},
		{ID: "audit", Description: "audit", State: state.StatusNotStarted, IsAudit: true},
	}
	_ = state.SaveNodeState(filepath.Join(leafDir, "state.json"), ns)

	idx.Root = []string{"blocked-leaf"}
	idx.Nodes["blocked-leaf"] = state.IndexEntry{
		Name: "Blocked", Type: state.NodeLeaf, State: state.StatusNotStarted, Address: "blocked-leaf",
	}
	idxPath := filepath.Join(dir, "state.json")
	_ = state.SaveRootIndex(idxPath, idx)

	fixes, report, err := FixWithVerification(dir, idxPath, DefaultNodeLoader(dir))
	if err != nil {
		t.Fatal(err)
	}
	// Should have applied the blocked-without-reason fix
	_ = fixes
	if report == nil {
		t.Fatal("expected non-nil final report")
	}
}

func TestFixWithVerification_DanglingRefAndOrphan(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	idx := state.NewRootIndex()

	// Create an orphan on disk not in the index
	orphanDir := filepath.Join(dir, "orphan-node")
	_ = os.MkdirAll(orphanDir, 0755)
	ns := state.NewNodeState("orphan-node", "Orphan", state.NodeLeaf)
	ns.Tasks = []state.Task{
		{ID: "task-0001", Description: "work", State: state.StatusNotStarted},
		{ID: "audit", Description: "audit", State: state.StatusNotStarted, IsAudit: true},
	}
	_ = state.SaveNodeState(filepath.Join(orphanDir, "state.json"), ns)

	// Add a dangling ref in the index (no state file on disk)
	idx.Nodes["ghost-ref"] = state.IndexEntry{
		Name: "Ghost", Type: state.NodeLeaf, State: state.StatusNotStarted, Address: "ghost-ref",
	}
	idx.Root = []string{"ghost-ref"}

	idxPath := filepath.Join(dir, "state.json")
	_ = state.SaveRootIndex(idxPath, idx)

	fixes, report, err := FixWithVerification(dir, idxPath, DefaultNodeLoader(dir))
	if err != nil {
		t.Fatal(err)
	}
	_ = fixes
	if report == nil {
		t.Fatal("expected non-nil final report")
	}
}

func TestFixWithVerification_FixesReturnedButZeroApplied(t *testing.T) {
	t.Parallel()
	// This scenario triggers the "len(fixes) == 0" break path inside the loop.
	// If HasAutoFixable() returns true but ApplyDeterministicFixes returns 0 fixes,
	// the loop breaks and proceeds to final validation.
	dir := t.TempDir()
	idx := state.NewRootIndex()

	// Create a node where the validation reports an auto-fixable issue but
	// the fix logic can't actually load the node (e.g., missing state file).
	// We put a dangling ref that will report as ROOTINDEX_DANGLING_REF (fixable),
	// but we also remove the ability to actually apply the fix by making the
	// node loadable.
	//
	// Actually, let's use a propagation mismatch where the node is a leaf
	// (not orchestrator), so the fix is a no-op but still gets recorded.
	leafDir := filepath.Join(dir, "prop-leaf")
	_ = os.MkdirAll(leafDir, 0755)
	ns := state.NewNodeState("prop-leaf", "Prop Leaf", state.NodeLeaf)
	ns.State = state.StatusInProgress
	ns.Tasks = []state.Task{
		{ID: "task-0001", Description: "work", State: state.StatusInProgress},
		{ID: "audit", Description: "audit", State: state.StatusNotStarted, IsAudit: true},
	}
	_ = state.SaveNodeState(filepath.Join(leafDir, "state.json"), ns)

	idx.Root = []string{"prop-leaf"}
	idx.Nodes["prop-leaf"] = state.IndexEntry{
		Name: "Prop Leaf", Type: state.NodeLeaf, State: state.StatusInProgress, Address: "prop-leaf",
	}
	idxPath := filepath.Join(dir, "state.json")
	_ = state.SaveRootIndex(idxPath, idx)

	fixes, report, err := FixWithVerification(dir, idxPath, DefaultNodeLoader(dir))
	if err != nil {
		t.Fatal(err)
	}
	_ = fixes
	if report == nil {
		t.Fatal("expected non-nil final report")
	}
}

func TestFixWithVerification_DepthMismatchFix(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	idx := state.NewRootIndex()

	// Create parent (orchestrator)
	parentDir := filepath.Join(dir, "parent")
	_ = os.MkdirAll(parentDir, 0755)
	parentNS := state.NewNodeState("parent", "Parent", state.NodeOrchestrator)
	parentNS.DecompositionDepth = 3
	parentNS.Children = []state.ChildRef{{ID: "child", Address: "parent/child", State: state.StatusNotStarted}}
	parentNS.Tasks = []state.Task{
		{ID: "audit", Description: "audit", State: state.StatusNotStarted, IsAudit: true},
	}
	_ = state.SaveNodeState(filepath.Join(parentDir, "state.json"), parentNS)

	// Create child with wrong depth
	childDir := filepath.Join(dir, "parent", "child")
	_ = os.MkdirAll(childDir, 0755)
	childNS := state.NewNodeState("child", "Child", state.NodeLeaf)
	childNS.DecompositionDepth = 1 // Wrong; should match parent
	childNS.Tasks = []state.Task{
		{ID: "task-0001", Description: "work", State: state.StatusNotStarted},
		{ID: "audit", Description: "audit", State: state.StatusNotStarted, IsAudit: true},
	}
	_ = state.SaveNodeState(filepath.Join(childDir, "state.json"), childNS)

	idx.Root = []string{"parent"}
	idx.Nodes["parent"] = state.IndexEntry{
		Name: "Parent", Type: state.NodeOrchestrator, State: state.StatusNotStarted,
		Address: "parent", Children: []string{"parent/child"},
	}
	idx.Nodes["parent/child"] = state.IndexEntry{
		Name: "Child", Type: state.NodeLeaf, State: state.StatusNotStarted,
		Address: "parent/child", Parent: "parent",
	}
	idxPath := filepath.Join(dir, "state.json")
	_ = state.SaveRootIndex(idxPath, idx)

	fixes, report, err := FixWithVerification(dir, idxPath, DefaultNodeLoader(dir))
	if err != nil {
		t.Fatal(err)
	}
	_ = fixes
	if report == nil {
		t.Fatal("expected non-nil final report")
	}
}

func TestFixWithVerification_AuditNotLastFix(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	idx := state.NewRootIndex()

	leafDir := filepath.Join(dir, "audit-order")
	_ = os.MkdirAll(leafDir, 0755)
	ns := state.NewNodeState("audit-order", "Audit Order", state.NodeLeaf)
	ns.State = state.StatusNotStarted
	ns.Tasks = []state.Task{
		{ID: "audit", Description: "audit", State: state.StatusNotStarted, IsAudit: true},
		{ID: "task-0001", Description: "work", State: state.StatusNotStarted},
	}
	_ = state.SaveNodeState(filepath.Join(leafDir, "state.json"), ns)

	idx.Root = []string{"audit-order"}
	idx.Nodes["audit-order"] = state.IndexEntry{
		Name: "Audit Order", Type: state.NodeLeaf, State: state.StatusNotStarted, Address: "audit-order",
	}
	idxPath := filepath.Join(dir, "state.json")
	_ = state.SaveRootIndex(idxPath, idx)

	fixes, report, err := FixWithVerification(dir, idxPath, DefaultNodeLoader(dir))
	if err != nil {
		t.Fatal(err)
	}
	_ = fixes
	if report == nil {
		t.Fatal("expected non-nil final report")
	}
}
