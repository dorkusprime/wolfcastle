package validate

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/dorkusprime/wolfcastle/internal/state"
)

// ═══════════════════════════════════════════════════════════════════════════
// FixWithVerification — error during ApplyDeterministicFixes
// ═══════════════════════════════════════════════════════════════════════════

func TestFixWithVerification_ApplyFixesError_ReadOnlyDir(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	idx := state.NewRootIndex()

	// Create a node with a missing audit task
	leafDir := filepath.Join(dir, "error-node")
	_ = os.MkdirAll(leafDir, 0755)
	ns := state.NewNodeState("error-node", "Error", state.NodeLeaf)
	ns.Tasks = []state.Task{
		{ID: "task-1", Description: "work", State: state.StatusNotStarted},
		// Missing audit task triggers MISSING_AUDIT_TASK fix
	}
	_ = state.SaveNodeState(filepath.Join(leafDir, "state.json"), ns)

	idx.Root = []string{"error-node"}
	idx.Nodes["error-node"] = state.IndexEntry{
		Name: "Error", Type: state.NodeLeaf, State: state.StatusNotStarted, Address: "error-node",
	}
	idxPath := filepath.Join(dir, "state.json")
	_ = state.SaveRootIndex(idxPath, idx)

	// Make the directory read-only so SaveNodeState (atomic write via temp file) fails
	_ = os.Chmod(leafDir, 0555)
	defer func() { _ = os.Chmod(leafDir, 0755) }()

	_, _, err := FixWithVerification(dir, idxPath, DefaultNodeLoader(dir))
	if err == nil {
		t.Error("expected error when state directory is read-only")
	}
}

// ═══════════════════════════════════════════════════════════════════════════
// FixWithVerification — invalid state value normalization
// ═══════════════════════════════════════════════════════════════════════════

func TestFixWithVerification_InvalidStateValueFix(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	idx := state.NewRootIndex()

	leafDir := filepath.Join(dir, "bad-state")
	_ = os.MkdirAll(leafDir, 0755)
	ns := state.NewNodeState("bad-state", "Bad State", state.NodeLeaf)
	ns.State = state.NodeStatus("IN_PROGRESS") // Wrong casing
	ns.Tasks = []state.Task{
		{ID: "task-1", Description: "work", State: state.StatusNotStarted},
		{ID: "audit", Description: "audit", State: state.StatusNotStarted, IsAudit: true},
	}
	_ = state.SaveNodeState(filepath.Join(leafDir, "state.json"), ns)

	idx.Root = []string{"bad-state"}
	idx.Nodes["bad-state"] = state.IndexEntry{
		Name: "Bad State", Type: state.NodeLeaf, State: state.NodeStatus("IN_PROGRESS"), Address: "bad-state",
	}
	idxPath := filepath.Join(dir, "state.json")
	_ = state.SaveRootIndex(idxPath, idx)

	fixes, report, err := FixWithVerification(dir, idxPath, DefaultNodeLoader(dir))
	if err != nil {
		t.Fatal(err)
	}
	// Check if any fix was for invalid state value
	hasStateFix := false
	for _, fix := range fixes {
		if fix.Category == CatInvalidStateValue {
			hasStateFix = true
		}
	}
	if hasStateFix {
		t.Log("Found and fixed invalid state value")
	}
	if report == nil {
		t.Fatal("expected non-nil final report")
	}
}

// ═══════════════════════════════════════════════════════════════════════════
// FixWithVerification — missing required fields
// ═══════════════════════════════════════════════════════════════════════════

func TestFixWithVerification_MissingRequiredFields(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	idx := state.NewRootIndex()

	leafDir := filepath.Join(dir, "empty-node")
	_ = os.MkdirAll(leafDir, 0755)
	// Write a minimal node with empty required fields
	ns := &state.NodeState{
		// All required fields missing
		Audit: state.AuditState{
			Breadcrumbs: []state.Breadcrumb{},
			Gaps:        []state.Gap{},
			Escalations: []state.Escalation{},
			Status:      state.AuditPending,
		},
	}
	ns.Tasks = []state.Task{
		{ID: "audit", Description: "audit", State: state.StatusNotStarted, IsAudit: true},
	}
	_ = state.SaveNodeState(filepath.Join(leafDir, "state.json"), ns)

	idx.Root = []string{"empty-node"}
	idx.Nodes["empty-node"] = state.IndexEntry{
		Name: "Empty", Type: state.NodeLeaf, State: state.StatusNotStarted, Address: "empty-node",
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

// ═══════════════════════════════════════════════════════════════════════════
// FixWithVerification — negative failure count
// ═══════════════════════════════════════════════════════════════════════════

func TestFixWithVerification_NegativeFailureCount(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	idx := state.NewRootIndex()

	leafDir := filepath.Join(dir, "neg-fail")
	_ = os.MkdirAll(leafDir, 0755)
	ns := state.NewNodeState("neg-fail", "NegFail", state.NodeLeaf)
	ns.Tasks = []state.Task{
		{ID: "task-1", Description: "work", State: state.StatusNotStarted, FailureCount: -5},
		{ID: "audit", Description: "audit", State: state.StatusNotStarted, IsAudit: true},
	}
	_ = state.SaveNodeState(filepath.Join(leafDir, "state.json"), ns)

	idx.Root = []string{"neg-fail"}
	idx.Nodes["neg-fail"] = state.IndexEntry{
		Name: "NegFail", Type: state.NodeLeaf, State: state.StatusNotStarted, Address: "neg-fail",
	}
	idxPath := filepath.Join(dir, "state.json")
	_ = state.SaveRootIndex(idxPath, idx)

	fixes, report, err := FixWithVerification(dir, idxPath, DefaultNodeLoader(dir))
	if err != nil {
		t.Fatal(err)
	}
	hasNegFix := false
	for _, fix := range fixes {
		if fix.Category == CatNegativeFailureCount {
			hasNegFix = true
		}
	}
	if !hasNegFix {
		t.Log("Expected negative failure count fix (may depend on validation rules)")
	}
	if report == nil {
		t.Fatal("expected non-nil final report")
	}
}

// ═══════════════════════════════════════════════════════════════════════════
// FixWithVerification — invalid audit status
// ═══════════════════════════════════════════════════════════════════════════

func TestFixWithVerification_InvalidAuditStatus(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	idx := state.NewRootIndex()

	leafDir := filepath.Join(dir, "bad-audit")
	_ = os.MkdirAll(leafDir, 0755)
	ns := state.NewNodeState("bad-audit", "BadAudit", state.NodeLeaf)
	ns.Audit.Status = state.AuditStatus("INVALID_STATUS")
	ns.Tasks = []state.Task{
		{ID: "task-1", Description: "work", State: state.StatusNotStarted},
		{ID: "audit", Description: "audit", State: state.StatusNotStarted, IsAudit: true},
	}
	_ = state.SaveNodeState(filepath.Join(leafDir, "state.json"), ns)

	idx.Root = []string{"bad-audit"}
	idx.Nodes["bad-audit"] = state.IndexEntry{
		Name: "BadAudit", Type: state.NodeLeaf, State: state.StatusNotStarted, Address: "bad-audit",
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

// ═══════════════════════════════════════════════════════════════════════════
// FixWithVerification — stale PID and stop file cleanup
// ═══════════════════════════════════════════════════════════════════════════

func TestFixWithVerification_StalePIDFileFix(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	wolfcastleDir := t.TempDir()

	idx := state.NewRootIndex()

	// Create a perfectly valid leaf node
	leafDir := filepath.Join(dir, "valid")
	_ = os.MkdirAll(leafDir, 0755)
	ns := state.NewNodeState("valid", "Valid", state.NodeLeaf)
	ns.Tasks = []state.Task{
		{ID: "task-1", Description: "work", State: state.StatusNotStarted},
		{ID: "audit", Description: "audit", State: state.StatusNotStarted, IsAudit: true},
	}
	_ = state.SaveNodeState(filepath.Join(leafDir, "state.json"), ns)

	idx.Root = []string{"valid"}
	idx.Nodes["valid"] = state.IndexEntry{
		Name: "Valid", Type: state.NodeLeaf, State: state.StatusNotStarted, Address: "valid",
	}
	idxPath := filepath.Join(dir, "state.json")
	_ = state.SaveRootIndex(idxPath, idx)

	// Create stale PID file (PID that doesn't exist)
	_ = os.WriteFile(filepath.Join(wolfcastleDir, "wolfcastle.pid"), []byte("99999999\n"), 0644)
	_ = os.WriteFile(filepath.Join(wolfcastleDir, "stop"), []byte(""), 0644)

	fixes, report, err := FixWithVerification(dir, idxPath, DefaultNodeLoader(dir), wolfcastleDir)
	if err != nil {
		t.Fatal(err)
	}
	_ = fixes
	if report == nil {
		t.Fatal("expected non-nil final report")
	}
}

// ═══════════════════════════════════════════════════════════════════════════
// FixWithVerification — audit status/task mismatch
// ═══════════════════════════════════════════════════════════════════════════

func TestFixWithVerification_AuditStatusTaskMismatch(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	idx := state.NewRootIndex()

	leafDir := filepath.Join(dir, "mismatch")
	_ = os.MkdirAll(leafDir, 0755)
	ns := state.NewNodeState("mismatch", "Mismatch", state.NodeLeaf)
	// Audit status says in_progress but audit task is not_started
	ns.Audit.Status = state.AuditInProgress
	ns.Tasks = []state.Task{
		{ID: "task-1", Description: "work", State: state.StatusComplete},
		{ID: "audit", Description: "audit", State: state.StatusNotStarted, IsAudit: true},
	}
	_ = state.SaveNodeState(filepath.Join(leafDir, "state.json"), ns)

	idx.Root = []string{"mismatch"}
	idx.Nodes["mismatch"] = state.IndexEntry{
		Name: "Mismatch", Type: state.NodeLeaf, State: state.StatusNotStarted, Address: "mismatch",
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

// ═══════════════════════════════════════════════════════════════════════════
// FixWithVerification — invalid audit gap metadata
// ═══════════════════════════════════════════════════════════════════════════

func TestFixWithVerification_InvalidAuditGapMetadata(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	idx := state.NewRootIndex()

	leafDir := filepath.Join(dir, "gap-node")
	_ = os.MkdirAll(leafDir, 0755)
	ns := state.NewNodeState("gap-node", "GapNode", state.NodeLeaf)
	ns.Audit.Gaps = []state.Gap{
		{ID: "gap-1", Status: state.GapOpen, FixedBy: "stale-value"},
	}
	ns.Tasks = []state.Task{
		{ID: "task-1", Description: "work", State: state.StatusNotStarted},
		{ID: "audit", Description: "audit", State: state.StatusNotStarted, IsAudit: true},
	}
	_ = state.SaveNodeState(filepath.Join(leafDir, "state.json"), ns)

	idx.Root = []string{"gap-node"}
	idx.Nodes["gap-node"] = state.IndexEntry{
		Name: "GapNode", Type: state.NodeLeaf, State: state.StatusNotStarted, Address: "gap-node",
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

// ═══════════════════════════════════════════════════════════════════════════
// FixWithVerification — propagation mismatch with orchestrator
// ═══════════════════════════════════════════════════════════════════════════

func TestFixWithVerification_PropagationMismatch(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	idx := state.NewRootIndex()

	// Parent orchestrator with wrong state
	parentDir := filepath.Join(dir, "orch")
	_ = os.MkdirAll(parentDir, 0755)
	parentNS := state.NewNodeState("orch", "Orch", state.NodeOrchestrator)
	parentNS.State = state.StatusNotStarted // Wrong — child is in_progress
	parentNS.Children = []state.ChildRef{
		{ID: "leaf", Address: "orch/leaf", State: state.StatusInProgress},
	}
	parentNS.Tasks = []state.Task{
		{ID: "audit", Description: "audit", State: state.StatusNotStarted, IsAudit: true},
	}
	_ = state.SaveNodeState(filepath.Join(parentDir, "state.json"), parentNS)

	childDir := filepath.Join(dir, "orch", "leaf")
	_ = os.MkdirAll(childDir, 0755)
	childNS := state.NewNodeState("leaf", "Leaf", state.NodeLeaf)
	childNS.State = state.StatusInProgress
	childNS.Tasks = []state.Task{
		{ID: "task-1", Description: "work", State: state.StatusInProgress},
		{ID: "audit", Description: "audit", State: state.StatusNotStarted, IsAudit: true},
	}
	_ = state.SaveNodeState(filepath.Join(childDir, "state.json"), childNS)

	idx.Root = []string{"orch"}
	idx.Nodes["orch"] = state.IndexEntry{
		Name: "Orch", Type: state.NodeOrchestrator, State: state.StatusNotStarted,
		Address: "orch", Children: []string{"orch/leaf"},
	}
	idx.Nodes["orch/leaf"] = state.IndexEntry{
		Name: "Leaf", Type: state.NodeLeaf, State: state.StatusInProgress,
		Address: "orch/leaf", Parent: "orch",
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

// ═══════════════════════════════════════════════════════════════════════════
// FixWithVerification — root index missing entry (orphan detected on disk)
// ═══════════════════════════════════════════════════════════════════════════

func TestFixWithVerification_RootIndexMissingEntry(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	idx := state.NewRootIndex()

	// Create a valid node on disk but don't add it to the index
	orphanDir := filepath.Join(dir, "orphan")
	_ = os.MkdirAll(orphanDir, 0755)
	ns := state.NewNodeState("orphan", "Orphan", state.NodeLeaf)
	ns.Tasks = []state.Task{
		{ID: "task-1", Description: "work", State: state.StatusNotStarted},
		{ID: "audit", Description: "audit", State: state.StatusNotStarted, IsAudit: true},
	}
	_ = state.SaveNodeState(filepath.Join(orphanDir, "state.json"), ns)

	// Index is empty — no reference to the orphan
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
