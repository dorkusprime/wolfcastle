package validate

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/dorkusprime/wolfcastle/internal/state"
)

// ── helpers ─────────────────────────────────────────────────────────────

// saveLeaf creates a leaf node on disk and returns its state path.
func saveLeaf(t *testing.T, dir, addr string, ns *state.NodeState) string {
	t.Helper()
	leafDir := filepath.Join(dir, filepath.FromSlash(addr))
	if err := os.MkdirAll(leafDir, 0755); err != nil {
		t.Fatal(err)
	}
	p := filepath.Join(leafDir, "state.json")
	if err := state.SaveNodeState(p, ns); err != nil {
		t.Fatal(err)
	}
	return p
}

// saveIndex writes the root index and returns its path.
func saveIndex(t *testing.T, dir string, idx *state.RootIndex) string {
	t.Helper()
	p := filepath.Join(dir, "state.json")
	if err := state.SaveRootIndex(p, idx); err != nil {
		t.Fatal(err)
	}
	return p
}

// findFix returns the first fix with the given category, or nil.
func findFix(fixes []FixResult, category string) *FixResult {
	for i := range fixes {
		if fixes[i].Category == category {
			return &fixes[i]
		}
	}
	return nil
}

// ── Report.Counts and Report.HasErrors ──────────────────────────────────

func TestReport_Counts(t *testing.T) {
	t.Parallel()
	r := &Report{
		Issues: []Issue{
			{Severity: SeverityError},
			{Severity: SeverityError},
			{Severity: SeverityWarning},
			{Severity: SeverityInfo},
			{Severity: SeverityWarning},
		},
	}
	r.Counts()
	if r.Errors != 2 {
		t.Errorf("expected 2 errors, got %d", r.Errors)
	}
	if r.Warnings != 2 {
		t.Errorf("expected 2 warnings, got %d", r.Warnings)
	}
}

func TestReport_Counts_Empty(t *testing.T) {
	t.Parallel()
	r := &Report{}
	r.Counts()
	if r.Errors != 0 || r.Warnings != 0 {
		t.Errorf("expected 0/0, got %d/%d", r.Errors, r.Warnings)
	}
}

func TestReport_HasErrors_True(t *testing.T) {
	t.Parallel()
	r := &Report{Issues: []Issue{{Severity: SeverityError}}}
	if !r.HasErrors() {
		t.Error("expected HasErrors=true")
	}
}

func TestReport_HasErrors_False(t *testing.T) {
	t.Parallel()
	r := &Report{Issues: []Issue{{Severity: SeverityWarning}}}
	if r.HasErrors() {
		t.Error("expected HasErrors=false with only warnings")
	}
}

func TestReport_HasErrors_Empty(t *testing.T) {
	t.Parallel()
	r := &Report{}
	if r.HasErrors() {
		t.Error("expected HasErrors=false on empty report")
	}
}

// ── Fix: CatRootIndexDanglingRef ────────────────────────────────────────

func TestFix_DanglingRef(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	idx := state.NewRootIndex()

	// Add a real leaf so the index is valid, plus a dangling entry
	ns := state.NewNodeState("leaf-a", "Leaf A", state.NodeLeaf)
	ns.Tasks = []state.Task{
		{ID: "t1", Description: "work", State: state.StatusNotStarted},
		{ID: "audit", Description: "audit", State: state.StatusNotStarted, IsAudit: true},
	}
	saveLeaf(t, dir, "leaf-a", ns)

	idx.Root = []string{"leaf-a", "dangling"}
	idx.Nodes["leaf-a"] = state.IndexEntry{
		Name: "Leaf A", Type: state.NodeLeaf, State: state.StatusNotStarted, Address: "leaf-a",
	}
	idx.Nodes["dangling"] = state.IndexEntry{
		Name: "Dangling", Type: state.NodeLeaf, State: state.StatusNotStarted, Address: "dangling",
	}
	// Also set dangling as a child of leaf-a to test child cleanup
	entry := idx.Nodes["leaf-a"]
	entry.Children = []string{"dangling"}
	idx.Nodes["leaf-a"] = entry

	idxPath := saveIndex(t, dir, idx)

	issues := []Issue{{
		Severity: SeverityError, Category: CatRootIndexDanglingRef, Node: "dangling",
		CanAutoFix: true, FixType: FixDeterministic,
	}}

	fixes, _, err := ApplyDeterministicFixes(idx, issues, dir, idxPath)
	if err != nil {
		t.Fatal(err)
	}
	if findFix(fixes, CatRootIndexDanglingRef) == nil {
		t.Fatal("expected a dangling ref fix")
	}

	// Verify removal from index
	if _, ok := idx.Nodes["dangling"]; ok {
		t.Error("dangling node should have been removed from index")
	}

	// Verify removal from root list
	for _, r := range idx.Root {
		if r == "dangling" {
			t.Error("dangling node should have been removed from root list")
		}
	}

	// Verify removal from parent children
	for _, child := range idx.Nodes["leaf-a"].Children {
		if child == "dangling" {
			t.Error("dangling node should have been removed from parent children")
		}
	}
}

// ── Fix: CatRootIndexMissingEntry ───────────────────────────────────────

func TestFix_MissingEntry_TopLevel(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	idx := state.NewRootIndex()

	// Node on disk but not in index — top-level (no parent)
	ns := state.NewNodeState("orphan", "Orphan Node", state.NodeLeaf)
	ns.State = state.StatusInProgress
	ns.DecompositionDepth = 2
	saveLeaf(t, dir, "orphan", ns)

	idxPath := saveIndex(t, dir, idx)

	issues := []Issue{{
		Severity: SeverityError, Category: CatRootIndexMissingEntry, Node: "orphan",
		CanAutoFix: true, FixType: FixDeterministic,
	}}

	fixes, _, err := ApplyDeterministicFixes(idx, issues, dir, idxPath)
	if err != nil {
		t.Fatal(err)
	}
	if findFix(fixes, CatRootIndexMissingEntry) == nil {
		t.Fatal("expected missing entry fix")
	}

	entry, ok := idx.Nodes["orphan"]
	if !ok {
		t.Fatal("orphan should now be in index")
	}
	if entry.Name != "Orphan Node" {
		t.Errorf("expected name 'Orphan Node', got %q", entry.Name)
	}
	if entry.State != state.StatusInProgress {
		t.Errorf("expected state in_progress, got %s", entry.State)
	}
	if entry.DecompositionDepth != 2 {
		t.Errorf("expected depth 2, got %d", entry.DecompositionDepth)
	}
	if entry.Parent != "" {
		t.Errorf("expected no parent, got %q", entry.Parent)
	}

	// Top-level should be in root list
	foundInRoot := false
	for _, r := range idx.Root {
		if r == "orphan" {
			foundInRoot = true
		}
	}
	if !foundInRoot {
		t.Error("orphan should be in root list")
	}
}

func TestFix_MissingEntry_Nested(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	idx := state.NewRootIndex()

	// Create parent in index
	parentNS := state.NewNodeState("parent", "Parent", state.NodeOrchestrator)
	saveLeaf(t, dir, "parent", parentNS)
	idx.Nodes["parent"] = state.IndexEntry{
		Name: "Parent", Type: state.NodeOrchestrator, State: state.StatusNotStarted, Address: "parent",
	}

	// Child on disk but not in index
	childNS := state.NewNodeState("child", "Child", state.NodeLeaf)
	saveLeaf(t, dir, "parent/child", childNS)

	idxPath := saveIndex(t, dir, idx)

	issues := []Issue{{
		Severity: SeverityError, Category: CatRootIndexMissingEntry, Node: "parent/child",
		CanAutoFix: true, FixType: FixDeterministic,
	}}

	fixes, _, err := ApplyDeterministicFixes(idx, issues, dir, idxPath)
	if err != nil {
		t.Fatal(err)
	}
	if findFix(fixes, CatRootIndexMissingEntry) == nil {
		t.Fatal("expected missing entry fix")
	}

	entry, ok := idx.Nodes["parent/child"]
	if !ok {
		t.Fatal("child should now be in index")
	}
	if entry.Parent != "parent" {
		t.Errorf("expected parent 'parent', got %q", entry.Parent)
	}

	// Check parent now has child listed
	parentEntry := idx.Nodes["parent"]
	found := false
	for _, c := range parentEntry.Children {
		if c == "parent/child" {
			found = true
		}
	}
	if !found {
		t.Error("parent should list parent/child as a child")
	}
}

// ── Fix: CatPropagationMismatch ─────────────────────────────────────────

func TestFix_PropagationMismatch(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	idx := state.NewRootIndex()

	// Orchestrator with wrong state — children say not_started
	orchNS := state.NewNodeState("orch", "Orch", state.NodeOrchestrator)
	orchNS.State = state.StatusComplete // wrong
	orchNS.Children = []state.ChildRef{
		{ID: "c1", Address: "orch/c1", State: state.StatusNotStarted},
	}
	saveLeaf(t, dir, "orch", orchNS)

	idx.Nodes["orch"] = state.IndexEntry{
		Name: "Orch", Type: state.NodeOrchestrator, State: state.StatusComplete, Address: "orch",
	}

	idxPath := saveIndex(t, dir, idx)

	issues := []Issue{{
		Severity: SeverityError, Category: CatPropagationMismatch, Node: "orch",
		CanAutoFix: true, FixType: FixDeterministic,
	}}

	fixes, _, err := ApplyDeterministicFixes(idx, issues, dir, idxPath)
	if err != nil {
		t.Fatal(err)
	}
	if findFix(fixes, CatPropagationMismatch) == nil {
		t.Fatal("expected propagation fix")
	}

	// Verify disk state was updated
	loaded, err := state.LoadNodeState(filepath.Join(dir, "orch", "state.json"))
	if err != nil {
		t.Fatal(err)
	}
	if loaded.State != state.StatusNotStarted {
		t.Errorf("expected not_started (recomputed), got %s", loaded.State)
	}

	// Verify index was updated
	if idx.Nodes["orch"].State != state.StatusNotStarted {
		t.Errorf("index should reflect recomputed state, got %s", idx.Nodes["orch"].State)
	}
}

// ── Fix: CatAuditNotLast ────────────────────────────────────────────────

func TestFix_AuditNotLast(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	idx := state.NewRootIndex()

	ns := state.NewNodeState("leaf", "Leaf", state.NodeLeaf)
	ns.Tasks = []state.Task{
		{ID: "audit", Description: "audit", State: state.StatusNotStarted, IsAudit: true},
		{ID: "task-1", Description: "work", State: state.StatusNotStarted},
		{ID: "task-2", Description: "more work", State: state.StatusNotStarted},
	}
	statePath := saveLeaf(t, dir, "leaf", ns)

	idx.Nodes["leaf"] = state.IndexEntry{
		Name: "Leaf", Type: state.NodeLeaf, State: state.StatusNotStarted, Address: "leaf",
	}
	idxPath := saveIndex(t, dir, idx)

	issues := []Issue{{
		Severity: SeverityError, Category: CatAuditNotLast, Node: "leaf",
		CanAutoFix: true, FixType: FixDeterministic,
	}}

	fixes, _, err := ApplyDeterministicFixes(idx, issues, dir, idxPath)
	if err != nil {
		t.Fatal(err)
	}
	if findFix(fixes, CatAuditNotLast) == nil {
		t.Fatal("expected audit-not-last fix")
	}

	loaded, err := state.LoadNodeState(statePath)
	if err != nil {
		t.Fatal(err)
	}
	last := loaded.Tasks[len(loaded.Tasks)-1]
	if !last.IsAudit {
		t.Error("audit task should now be last")
	}
	if last.ID != "audit" {
		t.Errorf("expected audit task ID 'audit', got %q", last.ID)
	}
	// Non-audit tasks should precede
	if loaded.Tasks[0].ID != "task-1" || loaded.Tasks[1].ID != "task-2" {
		t.Error("non-audit tasks should be in original order before audit")
	}
}

// ── Fix: CatInvalidStateValue ───────────────────────────────────────────

func TestFix_InvalidStateValue(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	idx := state.NewRootIndex()

	ns := state.NewNodeState("leaf", "Leaf", state.NodeLeaf)
	ns.State = "completed" // typo — should normalize to "complete"
	ns.Tasks = []state.Task{
		{ID: "audit", Description: "audit", State: state.StatusNotStarted, IsAudit: true},
	}
	statePath := saveLeaf(t, dir, "leaf", ns)

	idx.Nodes["leaf"] = state.IndexEntry{
		Name: "Leaf", Type: state.NodeLeaf, State: "completed", Address: "leaf",
	}
	idxPath := saveIndex(t, dir, idx)

	issues := []Issue{{
		Severity: SeverityError, Category: CatInvalidStateValue, Node: "leaf",
		CanAutoFix: true, FixType: FixDeterministic,
	}}

	fixes, _, err := ApplyDeterministicFixes(idx, issues, dir, idxPath)
	if err != nil {
		t.Fatal(err)
	}
	if findFix(fixes, CatInvalidStateValue) == nil {
		t.Fatal("expected invalid state value fix")
	}

	loaded, err := state.LoadNodeState(statePath)
	if err != nil {
		t.Fatal(err)
	}
	if loaded.State != state.StatusComplete {
		t.Errorf("expected complete, got %s", loaded.State)
	}
	if idx.Nodes["leaf"].State != state.StatusComplete {
		t.Errorf("index should have normalized state, got %s", idx.Nodes["leaf"].State)
	}
}

func TestFix_InvalidStateValue_NonNormalizable(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	idx := state.NewRootIndex()

	ns := state.NewNodeState("leaf", "Leaf", state.NodeLeaf)
	ns.State = "gibberish" // cannot be normalized
	ns.Tasks = []state.Task{
		{ID: "audit", Description: "audit", State: state.StatusNotStarted, IsAudit: true},
	}
	saveLeaf(t, dir, "leaf", ns)

	idx.Nodes["leaf"] = state.IndexEntry{
		Name: "Leaf", Type: state.NodeLeaf, State: "gibberish", Address: "leaf",
	}
	idxPath := saveIndex(t, dir, idx)

	// Non-normalizable: CanAutoFix=false, so fix should be skipped
	issues := []Issue{{
		Severity: SeverityError, Category: CatInvalidStateValue, Node: "leaf",
		CanAutoFix: false, FixType: FixModelAssisted,
	}}

	fixes, _, err := ApplyDeterministicFixes(idx, issues, dir, idxPath)
	if err != nil {
		t.Fatal(err)
	}
	if findFix(fixes, CatInvalidStateValue) != nil {
		t.Error("should not fix non-normalizable state value")
	}
}

// ── Fix: CatBlockedWithoutReason ────────────────────────────────────────

func TestFix_BlockedWithoutReason(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	idx := state.NewRootIndex()

	ns := state.NewNodeState("leaf", "Leaf", state.NodeLeaf)
	ns.State = state.StatusBlocked
	ns.Tasks = []state.Task{
		{ID: "t1", Description: "work", State: state.StatusBlocked, BlockedReason: ""},
		{ID: "audit", Description: "audit", State: state.StatusNotStarted, IsAudit: true},
	}
	statePath := saveLeaf(t, dir, "leaf", ns)

	idx.Nodes["leaf"] = state.IndexEntry{
		Name: "Leaf", Type: state.NodeLeaf, State: state.StatusBlocked, Address: "leaf",
	}
	idxPath := saveIndex(t, dir, idx)

	issues := []Issue{{
		Severity: SeverityError, Category: CatBlockedWithoutReason, Node: "leaf",
		CanAutoFix: true, FixType: FixDeterministic,
	}}

	fixes, _, err := ApplyDeterministicFixes(idx, issues, dir, idxPath)
	if err != nil {
		t.Fatal(err)
	}
	if findFix(fixes, CatBlockedWithoutReason) == nil {
		t.Fatal("expected blocked-without-reason fix")
	}

	loaded, err := state.LoadNodeState(statePath)
	if err != nil {
		t.Fatal(err)
	}
	if loaded.Tasks[0].BlockedReason == "" {
		t.Error("blocked reason should have been populated")
	}
	if !strings.Contains(loaded.Tasks[0].BlockedReason, "auto-fixed") {
		t.Error("blocked reason should mention auto-fixed")
	}
}

// ── Fix: CatDepthMismatch ───────────────────────────────────────────────

func TestFix_DepthMismatch(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	idx := state.NewRootIndex()

	parentNS := state.NewNodeState("parent", "Parent", state.NodeOrchestrator)
	parentNS.DecompositionDepth = 5
	parentNS.Children = []state.ChildRef{{ID: "child", Address: "parent/child", State: state.StatusNotStarted}}
	saveLeaf(t, dir, "parent", parentNS)

	childNS := state.NewNodeState("child", "Child", state.NodeLeaf)
	childNS.DecompositionDepth = 1 // too low
	childNS.Tasks = []state.Task{
		{ID: "audit", Description: "audit", State: state.StatusNotStarted, IsAudit: true},
	}
	statePath := saveLeaf(t, dir, "parent/child", childNS)

	idx.Nodes["parent"] = state.IndexEntry{
		Name: "Parent", Type: state.NodeOrchestrator, State: state.StatusNotStarted, Address: "parent",
		Children: []string{"parent/child"},
	}
	idx.Nodes["parent/child"] = state.IndexEntry{
		Name: "Child", Type: state.NodeLeaf, State: state.StatusNotStarted, Address: "parent/child",
		Parent: "parent",
	}
	idxPath := saveIndex(t, dir, idx)

	issues := []Issue{{
		Severity: SeverityError, Category: CatDepthMismatch, Node: "parent/child",
		CanAutoFix: true, FixType: FixDeterministic,
	}}

	fixes, _, err := ApplyDeterministicFixes(idx, issues, dir, idxPath)
	if err != nil {
		t.Fatal(err)
	}
	if findFix(fixes, CatDepthMismatch) == nil {
		t.Fatal("expected depth mismatch fix")
	}

	loaded, err := state.LoadNodeState(statePath)
	if err != nil {
		t.Fatal(err)
	}
	if loaded.DecompositionDepth != 5 {
		t.Errorf("expected depth 5, got %d", loaded.DecompositionDepth)
	}
}

// ── Fix: CatMissingRequiredField ────────────────────────────────────────

func TestFix_MissingRequiredField(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	idx := state.NewRootIndex()

	// All required fields empty
	ns := &state.NodeState{Version: 1}
	statePath := saveLeaf(t, dir, "empty", ns)

	idx.Nodes["empty"] = state.IndexEntry{
		Name: "Empty", Type: state.NodeLeaf, State: state.StatusNotStarted, Address: "empty",
	}
	idxPath := saveIndex(t, dir, idx)

	issues := []Issue{{
		Severity: SeverityError, Category: CatMissingRequiredField, Node: "empty",
		CanAutoFix: true, FixType: FixDeterministic,
	}}

	fixes, _, err := ApplyDeterministicFixes(idx, issues, dir, idxPath)
	if err != nil {
		t.Fatal(err)
	}
	if findFix(fixes, CatMissingRequiredField) == nil {
		t.Fatal("expected missing required field fix")
	}

	loaded, err := state.LoadNodeState(statePath)
	if err != nil {
		t.Fatal(err)
	}
	if loaded.ID != "empty" {
		t.Errorf("expected ID 'empty', got %q", loaded.ID)
	}
	if loaded.Name != "empty" {
		t.Errorf("expected Name 'empty', got %q", loaded.Name)
	}
	if loaded.Type != state.NodeLeaf {
		t.Errorf("expected type leaf, got %s", loaded.Type)
	}
	if loaded.State != state.StatusNotStarted {
		t.Errorf("expected state not_started, got %s", loaded.State)
	}
}

// ── Fix: CatInvalidAuditStatus ──────────────────────────────────────────

func TestFix_InvalidAuditStatus(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	idx := state.NewRootIndex()

	ns := state.NewNodeState("leaf", "Leaf", state.NodeLeaf)
	ns.Audit.Status = "garbage"
	ns.Tasks = []state.Task{
		{ID: "audit", Description: "audit", State: state.StatusNotStarted, IsAudit: true},
	}
	statePath := saveLeaf(t, dir, "leaf", ns)

	idx.Nodes["leaf"] = state.IndexEntry{
		Name: "Leaf", Type: state.NodeLeaf, State: state.StatusNotStarted, Address: "leaf",
	}
	idxPath := saveIndex(t, dir, idx)

	issues := []Issue{{
		Severity: SeverityError, Category: CatInvalidAuditStatus, Node: "leaf",
		CanAutoFix: true, FixType: FixDeterministic,
	}}

	fixes, _, err := ApplyDeterministicFixes(idx, issues, dir, idxPath)
	if err != nil {
		t.Fatal(err)
	}
	if findFix(fixes, CatInvalidAuditStatus) == nil {
		t.Fatal("expected invalid audit status fix")
	}

	loaded, err := state.LoadNodeState(statePath)
	if err != nil {
		t.Fatal(err)
	}
	if loaded.Audit.Status != state.AuditPending {
		t.Errorf("expected audit status 'pending', got %q", loaded.Audit.Status)
	}
}

// ── Fix: CatAuditStatusTaskMismatch ─────────────────────────────────────

func TestFix_AuditStatusTaskMismatch(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	idx := state.NewRootIndex()

	ns := state.NewNodeState("leaf", "Leaf", state.NodeLeaf)
	ns.State = state.StatusInProgress
	ns.Audit.Status = state.AuditPassed // wrong — should be in_progress
	ns.Tasks = []state.Task{
		{ID: "t1", Description: "work", State: state.StatusInProgress},
		{ID: "audit", Description: "audit", State: state.StatusNotStarted, IsAudit: true},
	}
	statePath := saveLeaf(t, dir, "leaf", ns)

	idx.Nodes["leaf"] = state.IndexEntry{
		Name: "Leaf", Type: state.NodeLeaf, State: state.StatusInProgress, Address: "leaf",
	}
	idxPath := saveIndex(t, dir, idx)

	issues := []Issue{{
		Severity: SeverityWarning, Category: CatAuditStatusTaskMismatch, Node: "leaf",
		CanAutoFix: true, FixType: FixDeterministic,
	}}

	fixes, _, err := ApplyDeterministicFixes(idx, issues, dir, idxPath)
	if err != nil {
		t.Fatal(err)
	}
	if findFix(fixes, CatAuditStatusTaskMismatch) == nil {
		t.Fatal("expected audit status task mismatch fix")
	}

	loaded, err := state.LoadNodeState(statePath)
	if err != nil {
		t.Fatal(err)
	}
	if loaded.Audit.Status != state.AuditInProgress {
		t.Errorf("expected audit status in_progress, got %q", loaded.Audit.Status)
	}
}

// ── Fix: CatInvalidAuditGap ────────────────────────────────────────────

func TestFix_InvalidAuditGap_ClearsStaleMetadata(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	idx := state.NewRootIndex()

	now := time.Now()
	ns := state.NewNodeState("leaf", "Leaf", state.NodeLeaf)
	ns.Audit.Gaps = []state.Gap{
		{ID: "gap-1", Description: "open gap", Status: state.GapOpen, FixedBy: "someone", FixedAt: &now},
		{ID: "gap-2", Description: "fixed gap", Status: state.GapFixed, FixedBy: "someone"},
	}
	ns.Tasks = []state.Task{
		{ID: "audit", Description: "audit", State: state.StatusNotStarted, IsAudit: true},
	}
	statePath := saveLeaf(t, dir, "leaf", ns)

	idx.Nodes["leaf"] = state.IndexEntry{
		Name: "Leaf", Type: state.NodeLeaf, State: state.StatusNotStarted, Address: "leaf",
	}
	idxPath := saveIndex(t, dir, idx)

	issues := []Issue{{
		Severity: SeverityWarning, Category: CatInvalidAuditGap, Node: "leaf",
		CanAutoFix: true, FixType: FixDeterministic,
	}}

	fixes, _, err := ApplyDeterministicFixes(idx, issues, dir, idxPath)
	if err != nil {
		t.Fatal(err)
	}
	if findFix(fixes, CatInvalidAuditGap) == nil {
		t.Fatal("expected invalid audit gap fix")
	}

	loaded, err := state.LoadNodeState(statePath)
	if err != nil {
		t.Fatal(err)
	}
	// Open gap should have metadata cleared
	if loaded.Audit.Gaps[0].FixedBy != "" {
		t.Error("open gap FixedBy should have been cleared")
	}
	if loaded.Audit.Gaps[0].FixedAt != nil {
		t.Error("open gap FixedAt should have been cleared")
	}
	// Fixed gap should be untouched
	if loaded.Audit.Gaps[1].FixedBy != "someone" {
		t.Error("fixed gap FixedBy should be preserved")
	}
}

// ── Fix: CatOrphanDefinition ────────────────────────────────────────────

func TestFix_OrphanDefinition(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	idx := state.NewRootIndex()
	idxPath := saveIndex(t, dir, idx)

	issues := []Issue{{
		Severity: SeverityWarning, Category: CatOrphanDefinition, Node: "some-orphan",
		CanAutoFix: true, FixType: FixDeterministic,
	}}

	fixes, _, err := ApplyDeterministicFixes(idx, issues, dir, idxPath)
	if err != nil {
		t.Fatal(err)
	}
	fix := findFix(fixes, CatOrphanDefinition)
	if fix == nil {
		t.Fatal("expected orphan definition fix result")
	}
	if !strings.Contains(fix.Description, "no auto-fix") {
		t.Errorf("expected no-op description, got %q", fix.Description)
	}
}

// ── Fix: CatStalePIDFile ────────────────────────────────────────────────

func TestFix_StalePIDFile(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	wolfcastleDir := t.TempDir()
	idx := state.NewRootIndex()
	idxPath := saveIndex(t, dir, idx)

	// Create a stale PID file
	pidPath := filepath.Join(wolfcastleDir, "wolfcastle.pid")
	_ = os.WriteFile(pidPath, []byte("99999999"), 0644)

	issues := []Issue{{
		Severity: SeverityWarning, Category: CatStalePIDFile,
		CanAutoFix: true, FixType: FixDeterministic,
	}}

	fixes, _, err := ApplyDeterministicFixes(idx, issues, dir, idxPath, wolfcastleDir)
	if err != nil {
		t.Fatal(err)
	}
	if findFix(fixes, CatStalePIDFile) == nil {
		t.Fatal("expected stale PID file fix")
	}

	if _, err := os.Stat(pidPath); !os.IsNotExist(err) {
		t.Error("PID file should have been removed")
	}
}

func TestFix_StalePIDFile_NoWolfcastleDir(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	idx := state.NewRootIndex()
	idxPath := saveIndex(t, dir, idx)

	issues := []Issue{{
		Severity: SeverityWarning, Category: CatStalePIDFile,
		CanAutoFix: true, FixType: FixDeterministic,
	}}

	// No wolfcastleDir passed — fix should silently do nothing
	fixes, _, err := ApplyDeterministicFixes(idx, issues, dir, idxPath)
	if err != nil {
		t.Fatal(err)
	}
	if findFix(fixes, CatStalePIDFile) != nil {
		t.Error("should not produce a fix when wolfcastleDir is empty")
	}
}

// ── Fix: CatStaleStopFile ───────────────────────────────────────────────

func TestFix_StaleStopFile(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	wolfcastleDir := t.TempDir()
	idx := state.NewRootIndex()
	idxPath := saveIndex(t, dir, idx)

	stopPath := filepath.Join(wolfcastleDir, "stop")
	_ = os.WriteFile(stopPath, []byte(""), 0644)

	issues := []Issue{{
		Severity: SeverityWarning, Category: CatStaleStopFile,
		CanAutoFix: true, FixType: FixDeterministic,
	}}

	fixes, _, err := ApplyDeterministicFixes(idx, issues, dir, idxPath, wolfcastleDir)
	if err != nil {
		t.Fatal(err)
	}
	if findFix(fixes, CatStaleStopFile) == nil {
		t.Fatal("expected stale stop file fix")
	}

	if _, err := os.Stat(stopPath); !os.IsNotExist(err) {
		t.Error("stop file should have been removed")
	}
}

func TestFix_StaleStopFile_NoWolfcastleDir(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	idx := state.NewRootIndex()
	idxPath := saveIndex(t, dir, idx)

	issues := []Issue{{
		Severity: SeverityWarning, Category: CatStaleStopFile,
		CanAutoFix: true, FixType: FixDeterministic,
	}}

	fixes, _, err := ApplyDeterministicFixes(idx, issues, dir, idxPath)
	if err != nil {
		t.Fatal(err)
	}
	if findFix(fixes, CatStaleStopFile) != nil {
		t.Error("should not produce a fix when wolfcastleDir is empty")
	}
}

// ── Fix: Non-deterministic issues are skipped ───────────────────────────

func TestFix_SkipsNonDeterministic(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	idx := state.NewRootIndex()
	idxPath := saveIndex(t, dir, idx)

	issues := []Issue{
		{Severity: SeverityError, Category: CatOrphanState, Node: "x", CanAutoFix: false, FixType: FixModelAssisted},
		{Severity: SeverityError, Category: CatCompleteWithIncomplete, Node: "y", CanAutoFix: false, FixType: FixModelAssisted},
	}

	fixes, _, err := ApplyDeterministicFixes(idx, issues, dir, idxPath)
	if err != nil {
		t.Fatal(err)
	}
	if len(fixes) != 0 {
		t.Errorf("expected no fixes for non-deterministic issues, got %d", len(fixes))
	}
}

// ── Detection: STALE_PID_FILE ───────────────────────────────────────────

func TestDetect_StalePIDFile(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	wolfcastleDir := t.TempDir()
	idx := state.NewRootIndex()

	// Create PID file with a dead PID
	pidPath := filepath.Join(wolfcastleDir, "wolfcastle.pid")
	_ = os.WriteFile(pidPath, []byte("99999999"), 0644)

	engine := NewEngine(dir, DefaultNodeLoader(dir), wolfcastleDir)
	report := engine.ValidateAll(idx)

	found := false
	for _, issue := range report.Issues {
		if issue.Category == CatStalePIDFile {
			found = true
		}
	}
	if !found {
		t.Error("expected STALE_PID_FILE issue")
	}
}

func TestDetect_StalePIDFile_NoPIDFile(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	wolfcastleDir := t.TempDir()
	idx := state.NewRootIndex()

	// No PID file — should not report
	engine := NewEngine(dir, DefaultNodeLoader(dir), wolfcastleDir)
	report := engine.ValidateAll(idx)

	for _, issue := range report.Issues {
		if issue.Category == CatStalePIDFile {
			t.Error("should not report STALE_PID_FILE when no PID file exists")
		}
	}
}

// ── Detection: STALE_STOP_FILE ──────────────────────────────────────────

func TestDetect_StaleStopFile(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	wolfcastleDir := t.TempDir()
	idx := state.NewRootIndex()

	stopPath := filepath.Join(wolfcastleDir, "stop")
	_ = os.WriteFile(stopPath, []byte(""), 0644)

	engine := NewEngine(dir, DefaultNodeLoader(dir), wolfcastleDir)
	report := engine.ValidateAll(idx)

	found := false
	for _, issue := range report.Issues {
		if issue.Category == CatStaleStopFile {
			found = true
		}
	}
	if !found {
		t.Error("expected STALE_STOP_FILE issue")
	}
}

// ── Detection: isDaemonAlive edge cases ─────────────────────────────────

func TestIsDaemonAlive_NoWolfcastleDir(t *testing.T) {
	t.Parallel()
	engine := NewEngine(t.TempDir(), DefaultNodeLoader(t.TempDir()))
	// wolfcastleDir is "" — should return false
	if engine.isDaemonAlive() {
		t.Error("expected false when wolfcastleDir is empty")
	}
}

func TestIsDaemonAlive_NoPIDFile(t *testing.T) {
	t.Parallel()
	wolfcastleDir := t.TempDir()
	engine := NewEngine(t.TempDir(), DefaultNodeLoader(t.TempDir()), wolfcastleDir)
	if engine.isDaemonAlive() {
		t.Error("expected false when PID file does not exist")
	}
}

func TestIsDaemonAlive_EmptyPIDFile(t *testing.T) {
	t.Parallel()
	wolfcastleDir := t.TempDir()
	_ = os.WriteFile(filepath.Join(wolfcastleDir, "wolfcastle.pid"), []byte(""), 0644)
	engine := NewEngine(t.TempDir(), DefaultNodeLoader(t.TempDir()), wolfcastleDir)
	if engine.isDaemonAlive() {
		t.Error("expected false for empty PID file")
	}
}

func TestIsDaemonAlive_NonNumericPID(t *testing.T) {
	t.Parallel()
	wolfcastleDir := t.TempDir()
	_ = os.WriteFile(filepath.Join(wolfcastleDir, "wolfcastle.pid"), []byte("not-a-number"), 0644)
	engine := NewEngine(t.TempDir(), DefaultNodeLoader(t.TempDir()), wolfcastleDir)
	if engine.isDaemonAlive() {
		t.Error("expected false for non-numeric PID")
	}
}

func TestIsDaemonAlive_DeadProcess(t *testing.T) {
	t.Parallel()
	wolfcastleDir := t.TempDir()
	// Use a very large PID unlikely to be alive
	_ = os.WriteFile(filepath.Join(wolfcastleDir, "wolfcastle.pid"), []byte("99999999"), 0644)
	engine := NewEngine(t.TempDir(), DefaultNodeLoader(t.TempDir()), wolfcastleDir)
	if engine.isDaemonAlive() {
		t.Error("expected false for dead process")
	}
}

func TestIsDaemonAlive_LiveProcess(t *testing.T) {
	t.Parallel()
	wolfcastleDir := t.TempDir()
	// Use our own PID — we know we're alive
	_ = os.WriteFile(filepath.Join(wolfcastleDir, "wolfcastle.pid"),
		[]byte(fmt.Sprintf("%d", os.Getpid())), 0644)

	engine := NewEngine(t.TempDir(), DefaultNodeLoader(t.TempDir()), wolfcastleDir)
	if !engine.isDaemonAlive() {
		t.Error("expected true for our own PID (we are alive)")
	}
}

// ── Detection: ORPHAN_DEFINITION ────────────────────────────────────────

func TestDetect_OrphanDefinition(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	idx := state.NewRootIndex()

	// Create a directory with a .md file but no node in index
	orphanDir := filepath.Join(dir, "orphan-def")
	_ = os.MkdirAll(orphanDir, 0755)
	_ = os.WriteFile(filepath.Join(orphanDir, "definition.md"), []byte("# Orphan"), 0644)

	engine := NewEngine(dir, DefaultNodeLoader(dir))
	report := engine.ValidateAll(idx)

	found := false
	for _, issue := range report.Issues {
		if issue.Category == CatOrphanDefinition {
			found = true
		}
	}
	if !found {
		t.Error("expected ORPHAN_DEFINITION issue")
	}
}

// ── Detection: ORPHAN_STATE ─────────────────────────────────────────────

func TestDetect_OrphanState(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	idx := state.NewRootIndex()

	// Create parent and child nodes
	parentNS := state.NewNodeState("parent", "Parent", state.NodeOrchestrator)
	parentNS.Children = []state.ChildRef{{ID: "child", Address: "parent/child", State: state.StatusNotStarted}}
	saveLeaf(t, dir, "parent", parentNS)

	childNS := state.NewNodeState("child", "Child", state.NodeLeaf)
	childNS.Tasks = []state.Task{
		{ID: "audit", Description: "audit", State: state.StatusNotStarted, IsAudit: true},
	}
	saveLeaf(t, dir, "parent/child", childNS)

	// Index: child has parent "parent" but parent does NOT list child in children
	idx.Nodes["parent"] = state.IndexEntry{
		Name: "Parent", Type: state.NodeOrchestrator, State: state.StatusNotStarted, Address: "parent",
		Children: []string{}, // child missing from parent's children list
	}
	idx.Nodes["parent/child"] = state.IndexEntry{
		Name: "Child", Type: state.NodeLeaf, State: state.StatusNotStarted, Address: "parent/child",
		Parent: "parent",
	}

	engine := NewEngine(dir, DefaultNodeLoader(dir))
	report := engine.ValidateAll(idx)

	found := false
	for _, issue := range report.Issues {
		if issue.Category == CatOrphanState {
			found = true
		}
	}
	if !found {
		t.Error("expected ORPHAN_STATE issue")
	}
}

// ── Detection: INVALID_STATE_VALUE ──────────────────────────────────────

func TestDetect_InvalidStateValue(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	idx := state.NewRootIndex()

	ns := state.NewNodeState("leaf", "Leaf", state.NodeLeaf)
	ns.State = "gibberish"
	ns.Tasks = []state.Task{
		{ID: "audit", Description: "audit", State: state.StatusNotStarted, IsAudit: true},
	}
	saveLeaf(t, dir, "leaf", ns)

	idx.Nodes["leaf"] = state.IndexEntry{
		Name: "Leaf", Type: state.NodeLeaf, State: "gibberish", Address: "leaf",
	}

	engine := NewEngine(dir, DefaultNodeLoader(dir))
	report := engine.ValidateAll(idx)

	found := false
	for _, issue := range report.Issues {
		if issue.Category == CatInvalidStateValue {
			found = true
			// Non-normalizable should be model-assisted
			if issue.FixType != FixModelAssisted {
				t.Errorf("expected model-assisted fix type for gibberish, got %s", issue.FixType)
			}
			if issue.CanAutoFix {
				t.Error("expected CanAutoFix=false for non-normalizable state")
			}
		}
	}
	if !found {
		t.Error("expected INVALID_STATE_VALUE issue")
	}
}

func TestDetect_InvalidStateValue_Normalizable(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	idx := state.NewRootIndex()

	ns := state.NewNodeState("leaf", "Leaf", state.NodeLeaf)
	ns.State = "completed" // typo but normalizable
	ns.Tasks = []state.Task{
		{ID: "audit", Description: "audit", State: state.StatusNotStarted, IsAudit: true},
	}
	saveLeaf(t, dir, "leaf", ns)

	idx.Nodes["leaf"] = state.IndexEntry{
		Name: "Leaf", Type: state.NodeLeaf, State: "completed", Address: "leaf",
	}

	engine := NewEngine(dir, DefaultNodeLoader(dir))
	report := engine.ValidateAll(idx)

	found := false
	for _, issue := range report.Issues {
		if issue.Category == CatInvalidStateValue {
			found = true
			if issue.FixType != FixDeterministic {
				t.Errorf("expected deterministic fix type for normalizable typo, got %s", issue.FixType)
			}
			if !issue.CanAutoFix {
				t.Error("expected CanAutoFix=true for normalizable state")
			}
		}
	}
	if !found {
		t.Error("expected INVALID_STATE_VALUE issue for 'completed'")
	}
}

// ── Detection: INVALID_AUDIT_GAP with invalid status ────────────────────

func TestDetect_InvalidAuditGap_BadStatus(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	idx := state.NewRootIndex()

	ns := state.NewNodeState("leaf", "Leaf", state.NodeLeaf)
	ns.Audit.Gaps = []state.Gap{
		{ID: "gap-1", Description: "valid fields", Status: "invalid_status"},
	}
	ns.Tasks = []state.Task{
		{ID: "audit", Description: "audit", State: state.StatusNotStarted, IsAudit: true},
	}
	saveLeaf(t, dir, "leaf", ns)

	idx.Nodes["leaf"] = state.IndexEntry{
		Name: "Leaf", Type: state.NodeLeaf, State: state.StatusNotStarted, Address: "leaf",
	}

	engine := NewEngine(dir, DefaultNodeLoader(dir))
	report := engine.ValidateAll(idx)

	found := false
	for _, issue := range report.Issues {
		if issue.Category == CatInvalidAuditGap && strings.Contains(issue.Description, "invalid status") {
			found = true
		}
	}
	if !found {
		t.Error("expected INVALID_AUDIT_GAP issue for invalid gap status")
	}
}

// ── Detection: STALE_IN_PROGRESS ────────────────────────────────────────

func TestDetect_StaleInProgress(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	wolfcastleDir := t.TempDir()
	idx := state.NewRootIndex()

	// Single in-progress task, no daemon alive
	ns := state.NewNodeState("leaf", "Leaf", state.NodeLeaf)
	ns.State = state.StatusInProgress
	ns.Tasks = []state.Task{
		{ID: "t1", Description: "work", State: state.StatusInProgress},
		{ID: "audit", Description: "audit", State: state.StatusNotStarted, IsAudit: true},
	}
	saveLeaf(t, dir, "leaf", ns)

	idx.Nodes["leaf"] = state.IndexEntry{
		Name: "Leaf", Type: state.NodeLeaf, State: state.StatusInProgress, Address: "leaf",
	}

	engine := NewEngine(dir, DefaultNodeLoader(dir), wolfcastleDir)
	report := engine.ValidateAll(idx)

	found := false
	for _, issue := range report.Issues {
		if issue.Category == CatStaleInProgress {
			found = true
		}
	}
	if !found {
		t.Error("expected STALE_IN_PROGRESS issue")
	}
}

// ── Detection: COMPLETE_WITH_INCOMPLETE for orchestrators ───────────────

func TestDetect_CompleteWithIncomplete_Orchestrator(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	idx := state.NewRootIndex()

	orchNS := state.NewNodeState("orch", "Orch", state.NodeOrchestrator)
	orchNS.State = state.StatusComplete
	orchNS.Children = []state.ChildRef{
		{ID: "c1", Address: "orch/c1", State: state.StatusComplete},
		{ID: "c2", Address: "orch/c2", State: state.StatusNotStarted},
	}
	saveLeaf(t, dir, "orch", orchNS)

	idx.Nodes["orch"] = state.IndexEntry{
		Name: "Orch", Type: state.NodeOrchestrator, State: state.StatusComplete, Address: "orch",
	}

	engine := NewEngine(dir, DefaultNodeLoader(dir))
	report := engine.ValidateAll(idx)

	found := false
	for _, issue := range report.Issues {
		if issue.Category == CatCompleteWithIncomplete && strings.Contains(issue.Description, "Orchestrator") {
			found = true
		}
	}
	if !found {
		t.Error("expected COMPLETE_WITH_INCOMPLETE issue for orchestrator")
	}
}

// ── expectedAuditStatus coverage ────────────────────────────────────────

func TestDetect_AuditStatusMismatch_NotStarted(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	idx := state.NewRootIndex()

	ns := state.NewNodeState("leaf", "Leaf", state.NodeLeaf)
	ns.State = state.StatusNotStarted
	ns.Audit.Status = state.AuditInProgress // wrong
	ns.Tasks = []state.Task{
		{ID: "audit", Description: "audit", State: state.StatusNotStarted, IsAudit: true},
	}
	saveLeaf(t, dir, "leaf", ns)

	idx.Nodes["leaf"] = state.IndexEntry{
		Name: "Leaf", Type: state.NodeLeaf, State: state.StatusNotStarted, Address: "leaf",
	}

	engine := NewEngine(dir, DefaultNodeLoader(dir))
	report := engine.ValidateAll(idx)

	found := false
	for _, issue := range report.Issues {
		if issue.Category == CatAuditStatusTaskMismatch {
			found = true
		}
	}
	if !found {
		t.Error("expected AUDIT_STATUS_TASK_MISMATCH for not_started node with in_progress audit status")
	}
}

func TestDetect_AuditStatusMismatch_Blocked(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	idx := state.NewRootIndex()

	ns := state.NewNodeState("leaf", "Leaf", state.NodeLeaf)
	ns.State = state.StatusBlocked
	ns.Audit.Status = state.AuditPassed // wrong, should be failed
	ns.Tasks = []state.Task{
		{ID: "t1", Description: "work", State: state.StatusBlocked, BlockedReason: "reason"},
		{ID: "audit", Description: "audit", State: state.StatusNotStarted, IsAudit: true},
	}
	saveLeaf(t, dir, "leaf", ns)

	idx.Nodes["leaf"] = state.IndexEntry{
		Name: "Leaf", Type: state.NodeLeaf, State: state.StatusBlocked, Address: "leaf",
	}

	engine := NewEngine(dir, DefaultNodeLoader(dir))
	report := engine.ValidateAll(idx)

	found := false
	for _, issue := range report.Issues {
		if issue.Category == CatAuditStatusTaskMismatch {
			found = true
		}
	}
	if !found {
		t.Error("expected AUDIT_STATUS_TASK_MISMATCH for blocked node")
	}
}

func TestDetect_AuditStatusMismatch_CompleteWithOpenGaps(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	idx := state.NewRootIndex()

	ns := state.NewNodeState("leaf", "Leaf", state.NodeLeaf)
	ns.State = state.StatusComplete
	ns.Audit.Status = state.AuditPassed // wrong — has open gaps, should be failed
	ns.Audit.Gaps = []state.Gap{
		{ID: "g1", Description: "open gap", Status: state.GapOpen},
	}
	ns.Tasks = []state.Task{
		{ID: "t1", Description: "work", State: state.StatusComplete},
		{ID: "audit", Description: "audit", State: state.StatusComplete, IsAudit: true},
	}
	saveLeaf(t, dir, "leaf", ns)

	idx.Nodes["leaf"] = state.IndexEntry{
		Name: "Leaf", Type: state.NodeLeaf, State: state.StatusComplete, Address: "leaf",
	}

	engine := NewEngine(dir, DefaultNodeLoader(dir))
	report := engine.ValidateAll(idx)

	found := false
	for _, issue := range report.Issues {
		if issue.Category == CatAuditStatusTaskMismatch {
			found = true
		}
	}
	if !found {
		t.Error("expected AUDIT_STATUS_TASK_MISMATCH for complete node with open gaps")
	}
}

func TestDetect_AuditStatusMismatch_CompleteNoGaps(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	idx := state.NewRootIndex()

	ns := state.NewNodeState("leaf", "Leaf", state.NodeLeaf)
	ns.State = state.StatusComplete
	ns.Audit.Status = state.AuditFailed // wrong — no open gaps, should be passed
	ns.Tasks = []state.Task{
		{ID: "t1", Description: "work", State: state.StatusComplete},
		{ID: "audit", Description: "audit", State: state.StatusComplete, IsAudit: true},
	}
	saveLeaf(t, dir, "leaf", ns)

	idx.Nodes["leaf"] = state.IndexEntry{
		Name: "Leaf", Type: state.NodeLeaf, State: state.StatusComplete, Address: "leaf",
	}

	engine := NewEngine(dir, DefaultNodeLoader(dir))
	report := engine.ValidateAll(idx)

	found := false
	for _, issue := range report.Issues {
		if issue.Category == CatAuditStatusTaskMismatch {
			found = true
		}
	}
	if !found {
		t.Error("expected AUDIT_STATUS_TASK_MISMATCH for complete node with no gaps but failed audit")
	}
}

// ── Detection: PropagationMismatch index vs node ────────────────────────

func TestDetect_PropagationMismatch_IndexVsNode(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	idx := state.NewRootIndex()

	// Node has in_progress but index says not_started
	ns := state.NewNodeState("leaf", "Leaf", state.NodeLeaf)
	ns.State = state.StatusInProgress
	ns.Tasks = []state.Task{
		{ID: "t1", Description: "work", State: state.StatusInProgress},
		{ID: "audit", Description: "audit", State: state.StatusNotStarted, IsAudit: true},
	}
	saveLeaf(t, dir, "leaf", ns)

	idx.Nodes["leaf"] = state.IndexEntry{
		Name: "Leaf", Type: state.NodeLeaf, State: state.StatusNotStarted, Address: "leaf",
	}

	engine := NewEngine(dir, DefaultNodeLoader(dir))
	report := engine.ValidateAll(idx)

	found := false
	for _, issue := range report.Issues {
		if issue.Category == CatPropagationMismatch && strings.Contains(issue.Description, "Index says") {
			found = true
		}
	}
	if !found {
		t.Error("expected PROPAGATION_MISMATCH for index/node state divergence")
	}
}

// ── Verify .md at root level is ignored ─────────────────────────────────

func TestDetect_OrphanDefinition_RootMdIgnored(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	idx := state.NewRootIndex()

	// .md in root dir should be ignored (addr == ".")
	_ = os.WriteFile(filepath.Join(dir, "README.md"), []byte("root readme"), 0644)

	engine := NewEngine(dir, DefaultNodeLoader(dir))
	report := engine.ValidateAll(idx)

	for _, issue := range report.Issues {
		if issue.Category == CatOrphanDefinition && issue.Node == "." {
			t.Error("root-level .md files should not be flagged as orphan definitions")
		}
	}
}

// ── Engine with wolfcastleDir not set ───────────────────────────────────

func TestDetect_StalePIDFile_NoWolfcastleDir(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	idx := state.NewRootIndex()

	// No wolfcastleDir — stale PID/stop should not be checked
	engine := NewEngine(dir, DefaultNodeLoader(dir))
	report := engine.ValidateAll(idx)

	for _, issue := range report.Issues {
		if issue.Category == CatStalePIDFile || issue.Category == CatStaleStopFile {
			t.Error("should not check PID/stop files without wolfcastleDir")
		}
	}
}

// ── Engine: invalid address in index is skipped ─────────────────────────

func TestValidate_SkipsInvalidAddressInIndex(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	idx := state.NewRootIndex()

	// Add a valid leaf
	ns := state.NewNodeState("leaf", "Leaf", state.NodeLeaf)
	ns.Tasks = []state.Task{
		{ID: "audit", Description: "audit", State: state.StatusNotStarted, IsAudit: true},
	}
	saveLeaf(t, dir, "leaf", ns)
	idx.Nodes["leaf"] = state.IndexEntry{
		Name: "Leaf", Type: state.NodeLeaf, State: state.StatusNotStarted, Address: "leaf",
	}

	// Add an entry with an invalid address (contains uppercase — invalid slug)
	idx.Nodes["INVALID ADDRESS"] = state.IndexEntry{
		Name: "Bad", Type: state.NodeLeaf, State: state.StatusNotStarted, Address: "INVALID ADDRESS",
	}

	engine := NewEngine(dir, DefaultNodeLoader(dir))
	report := engine.ValidateAll(idx)

	// Should not panic or crash; the invalid address is silently skipped
	_ = report
}

// ── DefaultNodeLoader: invalid address returns error ────────────────────

func TestDefaultNodeLoader_InvalidAddress(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	loader := DefaultNodeLoader(dir)
	_, err := loader("INVALID ADDRESS")
	if err == nil {
		t.Error("expected error for invalid address")
	}
}

// ── Fix: loadOrCached errors (node missing from disk) ───────────────────

func TestFix_LoadError_ContinuesGracefully(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	idx := state.NewRootIndex()

	// Node is in index but the state file is missing from disk
	idx.Nodes["missing"] = state.IndexEntry{
		Name: "Missing", Type: state.NodeLeaf, State: state.StatusNotStarted, Address: "missing",
	}
	idxPath := saveIndex(t, dir, idx)

	// All these categories try to loadOrCached and should gracefully continue
	categories := []string{
		CatPropagationMismatch,
		CatAuditNotLast,
		CatInvalidStateValue,
		CatBlockedWithoutReason,
		CatMissingRequiredField,
		CatInvalidAuditStatus,
		CatAuditStatusTaskMismatch,
		CatInvalidAuditGap,
	}

	for _, cat := range categories {
		issues := []Issue{{
			Severity: SeverityError, Category: cat, Node: "missing",
			CanAutoFix: true, FixType: FixDeterministic,
		}}

		fixes, _, err := ApplyDeterministicFixes(idx, issues, dir, idxPath)
		if err != nil {
			t.Fatalf("category %s: unexpected error: %v", cat, err)
		}
		// Some categories produce a fix even on load error (e.g., PropagationMismatch
		// doesn't continue on load error for all branches), but none should panic
		_ = fixes
	}
}

func TestFix_LoadError_DepthMismatch(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	idx := state.NewRootIndex()

	// Node with parent, but node file missing from disk
	idx.Nodes["parent"] = state.IndexEntry{
		Name: "Parent", Type: state.NodeOrchestrator, State: state.StatusNotStarted, Address: "parent",
	}
	idx.Nodes["parent/missing"] = state.IndexEntry{
		Name: "Missing", Type: state.NodeLeaf, State: state.StatusNotStarted, Address: "parent/missing",
		Parent: "parent",
	}
	idxPath := saveIndex(t, dir, idx)

	issues := []Issue{{
		Severity: SeverityError, Category: CatDepthMismatch, Node: "parent/missing",
		CanAutoFix: true, FixType: FixDeterministic,
	}}

	fixes, _, err := ApplyDeterministicFixes(idx, issues, dir, idxPath)
	if err != nil {
		t.Fatal(err)
	}
	// Should not produce a fix since the node can't be loaded
	if findFix(fixes, CatDepthMismatch) != nil {
		t.Error("should not fix depth mismatch when node can't be loaded")
	}
}

func TestFix_LoadError_MissingEntry_InvalidAddress(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	idx := state.NewRootIndex()
	idxPath := saveIndex(t, dir, idx)

	// Issue with an address that won't parse
	issues := []Issue{{
		Severity: SeverityError, Category: CatRootIndexMissingEntry, Node: "INVALID ADDRESS",
		CanAutoFix: true, FixType: FixDeterministic,
	}}

	fixes, _, err := ApplyDeterministicFixes(idx, issues, dir, idxPath)
	if err != nil {
		t.Fatal(err)
	}
	if findFix(fixes, CatRootIndexMissingEntry) != nil {
		t.Error("should not fix missing entry with invalid address")
	}
}

// ── PropagationMismatch fix: leaf (non-orchestrator) ────────────────────

// ── Fix: cache hit in loadOrCached (two fixes on same node) ─────────────

func TestFix_CacheHit_TwoFixesSameNode(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	idx := state.NewRootIndex()

	ns := state.NewNodeState("leaf", "Leaf", state.NodeLeaf)
	ns.State = state.StatusBlocked
	ns.Audit.Status = "garbage"
	ns.Tasks = []state.Task{
		{ID: "t1", Description: "work", State: state.StatusBlocked, BlockedReason: ""},
		{ID: "audit", Description: "audit", State: state.StatusNotStarted, IsAudit: true},
	}
	saveLeaf(t, dir, "leaf", ns)

	idx.Nodes["leaf"] = state.IndexEntry{
		Name: "Leaf", Type: state.NodeLeaf, State: state.StatusBlocked, Address: "leaf",
	}
	idxPath := saveIndex(t, dir, idx)

	// Two fixes on the same node — the second should hit the cache
	issues := []Issue{
		{Severity: SeverityError, Category: CatBlockedWithoutReason, Node: "leaf", CanAutoFix: true, FixType: FixDeterministic},
		{Severity: SeverityError, Category: CatInvalidAuditStatus, Node: "leaf", CanAutoFix: true, FixType: FixDeterministic},
	}

	fixes, _, err := ApplyDeterministicFixes(idx, issues, dir, idxPath)
	if err != nil {
		t.Fatal(err)
	}
	if len(fixes) != 2 {
		t.Fatalf("expected 2 fixes, got %d", len(fixes))
	}
	if findFix(fixes, CatBlockedWithoutReason) == nil {
		t.Error("expected blocked-without-reason fix")
	}
	if findFix(fixes, CatInvalidAuditStatus) == nil {
		t.Error("expected invalid audit status fix")
	}
}

// ── PropagationMismatch fix: leaf (non-orchestrator) ────────────────────

func TestFix_PropagationMismatch_LeafOnlyUpdatesIndex(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	idx := state.NewRootIndex()

	// A leaf node where index and node disagree on state
	ns := state.NewNodeState("leaf", "Leaf", state.NodeLeaf)
	ns.State = state.StatusInProgress
	ns.Tasks = []state.Task{
		{ID: "t1", Description: "work", State: state.StatusInProgress},
		{ID: "audit", Description: "audit", State: state.StatusNotStarted, IsAudit: true},
	}
	saveLeaf(t, dir, "leaf", ns)

	idx.Nodes["leaf"] = state.IndexEntry{
		Name: "Leaf", Type: state.NodeLeaf, State: state.StatusNotStarted, Address: "leaf",
	}
	idxPath := saveIndex(t, dir, idx)

	issues := []Issue{{
		Severity: SeverityWarning, Category: CatPropagationMismatch, Node: "leaf",
		CanAutoFix: true, FixType: FixDeterministic,
	}}

	fixes, _, err := ApplyDeterministicFixes(idx, issues, dir, idxPath)
	if err != nil {
		t.Fatal(err)
	}
	if findFix(fixes, CatPropagationMismatch) == nil {
		t.Fatal("expected propagation fix")
	}

	// For a leaf, the index should be updated to match the node state
	if idx.Nodes["leaf"].State != state.StatusInProgress {
		t.Errorf("expected index state in_progress, got %s", idx.Nodes["leaf"].State)
	}
}

func TestHasAutoFixable(t *testing.T) {
	t.Parallel()

	t.Run("empty report", func(t *testing.T) {
		r := &Report{}
		if r.HasAutoFixable() {
			t.Error("empty report should not be auto-fixable")
		}
	})

	t.Run("no fixable issues", func(t *testing.T) {
		r := &Report{
			Issues: []Issue{
				{Category: CatOrphanDefinition, CanAutoFix: false},
			},
		}
		if r.HasAutoFixable() {
			t.Error("report with no fixable issues should not be auto-fixable")
		}
	})

	t.Run("has fixable issue", func(t *testing.T) {
		r := &Report{
			Issues: []Issue{
				{Category: CatMissingAuditTask, CanAutoFix: true},
			},
		}
		if !r.HasAutoFixable() {
			t.Error("report with fixable issue should be auto-fixable")
		}
	})
}

func TestFixWithVerification_SinglePass(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	projectsDir := dir

	// Create a leaf missing its audit task
	leaf := state.NewNodeState("leaf", "Leaf", state.NodeLeaf)
	leaf.Tasks = []state.Task{
		{ID: "task-1", Description: "work", State: state.StatusNotStarted},
	}
	saveLeaf(t, dir, "leaf", leaf)

	idx := state.NewRootIndex()
	idx.Root = []string{"leaf"}
	idx.Nodes["leaf"] = state.IndexEntry{
		Name:    "Leaf",
		Type:    state.NodeLeaf,
		State:   state.StatusNotStarted,
		Address: "leaf",
	}
	indexPath := saveIndex(t, dir, idx)

	loader := DefaultNodeLoader(projectsDir)
	fixes, finalReport, err := FixWithVerification(projectsDir, indexPath, loader)
	if err != nil {
		t.Fatal(err)
	}

	if len(fixes) == 0 {
		t.Error("expected at least one fix")
	}

	// Check that the fix has Pass set
	foundAuditFix := false
	for _, fix := range fixes {
		if fix.Category == CatMissingAuditTask {
			foundAuditFix = true
			if fix.Pass != 1 {
				t.Errorf("expected fix on pass 1, got pass %d", fix.Pass)
			}
		}
	}
	if !foundAuditFix {
		t.Error("expected missing audit task fix")
	}

	// Final report should show no critical issues
	if finalReport == nil {
		t.Fatal("expected final report")
	}
}

func TestFixWithVerification_CleanTree(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	projectsDir := dir

	// Create a valid leaf with audit task
	leaf := state.NewNodeState("leaf", "Leaf", state.NodeLeaf)
	leaf.Tasks = []state.Task{
		{ID: "task-1", Description: "work", State: state.StatusNotStarted},
		{ID: "audit", Description: "verify", State: state.StatusNotStarted, IsAudit: true},
	}
	saveLeaf(t, dir, "leaf", leaf)

	idx := state.NewRootIndex()
	idx.Root = []string{"leaf"}
	idx.Nodes["leaf"] = state.IndexEntry{
		Name:    "Leaf",
		Type:    state.NodeLeaf,
		State:   state.StatusNotStarted,
		Address: "leaf",
	}
	indexPath := saveIndex(t, dir, idx)

	loader := DefaultNodeLoader(projectsDir)
	fixes, _, err := FixWithVerification(projectsDir, indexPath, loader)
	if err != nil {
		t.Fatal(err)
	}

	if len(fixes) != 0 {
		t.Errorf("expected no fixes for clean tree, got %d", len(fixes))
	}
}
