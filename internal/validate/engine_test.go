package validate

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/dorkusprime/wolfcastle/internal/state"
)

func setupTestTree(t *testing.T) (string, *state.RootIndex) {
	t.Helper()
	dir := t.TempDir()

	idx := state.NewRootIndex()
	idx.RootID = "test"
	idx.RootName = "Test"
	idx.RootState = state.StatusNotStarted
	idx.Root = []string{"leaf-a"}

	// Create a valid leaf node
	leafDir := filepath.Join(dir, "leaf-a")
	os.MkdirAll(leafDir, 0755)
	ns := state.NewNodeState("leaf-a", "Leaf A", state.NodeLeaf)
	ns.Tasks = []state.Task{
		{ID: "task-1", Description: "do work", State: state.StatusNotStarted},
		{ID: "audit", Description: "audit", State: state.StatusNotStarted, IsAudit: true},
	}
	state.SaveNodeState(filepath.Join(leafDir, "state.json"), ns)

	idx.Nodes["leaf-a"] = state.IndexEntry{
		Name:    "Leaf A",
		Type:    state.NodeLeaf,
		State:   state.StatusNotStarted,
		Address: "leaf-a",
	}

	return dir, idx
}

func TestValidateAll_HealthyTree(t *testing.T) {
	t.Parallel()
	dir, idx := setupTestTree(t)
	engine := NewEngine(dir, DefaultNodeLoader(dir))
	report := engine.ValidateAll(idx)

	if report.HasErrors() {
		for _, issue := range report.Issues {
			t.Errorf("unexpected issue: [%s] %s: %s", issue.Category, issue.Node, issue.Description)
		}
	}
}

func TestValidateAll_DetectsDanglingRef(t *testing.T) {
	t.Parallel()
	dir, idx := setupTestTree(t)

	// Add a node to the index that doesn't exist on disk
	idx.Nodes["missing-node"] = state.IndexEntry{
		Name: "Missing", Type: state.NodeLeaf, State: state.StatusNotStarted, Address: "missing-node",
	}

	engine := NewEngine(dir, DefaultNodeLoader(dir))
	report := engine.ValidateAll(idx)

	found := false
	for _, issue := range report.Issues {
		if issue.Category == CatRootIndexDanglingRef && issue.Node == "missing-node" {
			found = true
		}
	}
	if !found {
		t.Error("expected ROOTINDEX_DANGLING_REF issue for missing-node")
	}
}

func TestValidateAll_DetectsMissingEntry(t *testing.T) {
	t.Parallel()
	dir, idx := setupTestTree(t)

	// Create a node on disk that's not in the index
	orphanDir := filepath.Join(dir, "orphan-node")
	os.MkdirAll(orphanDir, 0755)
	ns := state.NewNodeState("orphan-node", "Orphan", state.NodeLeaf)
	state.SaveNodeState(filepath.Join(orphanDir, "state.json"), ns)

	engine := NewEngine(dir, DefaultNodeLoader(dir))
	report := engine.ValidateAll(idx)

	found := false
	for _, issue := range report.Issues {
		if issue.Category == CatRootIndexMissingEntry && issue.Node == "orphan-node" {
			found = true
		}
	}
	if !found {
		t.Error("expected ROOTINDEX_MISSING_ENTRY for orphan-node")
	}
}

func TestValidateAll_DetectsMissingAuditTask(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	idx := state.NewRootIndex()

	// Create a leaf without audit task
	leafDir := filepath.Join(dir, "no-audit")
	os.MkdirAll(leafDir, 0755)
	ns := state.NewNodeState("no-audit", "No Audit", state.NodeLeaf)
	ns.Tasks = []state.Task{
		{ID: "task-1", Description: "work", State: state.StatusNotStarted},
	}
	state.SaveNodeState(filepath.Join(leafDir, "state.json"), ns)

	idx.Nodes["no-audit"] = state.IndexEntry{
		Name: "No Audit", Type: state.NodeLeaf, State: state.StatusNotStarted, Address: "no-audit",
	}

	engine := NewEngine(dir, DefaultNodeLoader(dir))
	report := engine.ValidateAll(idx)

	found := false
	for _, issue := range report.Issues {
		if issue.Category == CatMissingAuditTask {
			found = true
		}
	}
	if !found {
		t.Error("expected MISSING_AUDIT_TASK issue")
	}
}

func TestValidateAll_DetectsAuditNotLast(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	idx := state.NewRootIndex()

	leafDir := filepath.Join(dir, "bad-order")
	os.MkdirAll(leafDir, 0755)
	ns := state.NewNodeState("bad-order", "Bad Order", state.NodeLeaf)
	ns.Tasks = []state.Task{
		{ID: "audit", Description: "audit", State: state.StatusNotStarted, IsAudit: true},
		{ID: "task-1", Description: "work", State: state.StatusNotStarted},
	}
	state.SaveNodeState(filepath.Join(leafDir, "state.json"), ns)

	idx.Nodes["bad-order"] = state.IndexEntry{
		Name: "Bad Order", Type: state.NodeLeaf, State: state.StatusNotStarted, Address: "bad-order",
	}

	engine := NewEngine(dir, DefaultNodeLoader(dir))
	report := engine.ValidateAll(idx)

	found := false
	for _, issue := range report.Issues {
		if issue.Category == CatAuditNotLast {
			found = true
		}
	}
	if !found {
		t.Error("expected AUDIT_NOT_LAST issue")
	}
}

func TestValidateAll_DetectsMultipleAuditTasks(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	idx := state.NewRootIndex()

	leafDir := filepath.Join(dir, "multi-audit")
	os.MkdirAll(leafDir, 0755)
	ns := state.NewNodeState("multi-audit", "Multi Audit", state.NodeLeaf)
	ns.Tasks = []state.Task{
		{ID: "task-1", Description: "work", State: state.StatusNotStarted},
		{ID: "audit-1", Description: "audit 1", State: state.StatusNotStarted, IsAudit: true},
		{ID: "audit-2", Description: "audit 2", State: state.StatusNotStarted, IsAudit: true},
	}
	state.SaveNodeState(filepath.Join(leafDir, "state.json"), ns)

	idx.Nodes["multi-audit"] = state.IndexEntry{
		Name: "Multi Audit", Type: state.NodeLeaf, State: state.StatusNotStarted, Address: "multi-audit",
	}

	engine := NewEngine(dir, DefaultNodeLoader(dir))
	report := engine.ValidateAll(idx)

	found := false
	for _, issue := range report.Issues {
		if issue.Category == CatMultipleAuditTasks {
			found = true
		}
	}
	if !found {
		t.Error("expected MULTIPLE_AUDIT_TASKS issue")
	}
}

func TestValidateAll_DetectsCompleteWithIncomplete(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	idx := state.NewRootIndex()

	leafDir := filepath.Join(dir, "bad-complete")
	os.MkdirAll(leafDir, 0755)
	ns := state.NewNodeState("bad-complete", "Bad Complete", state.NodeLeaf)
	ns.State = state.StatusComplete
	ns.Tasks = []state.Task{
		{ID: "task-1", Description: "work", State: state.StatusComplete},
		{ID: "task-2", Description: "more work", State: state.StatusNotStarted},
		{ID: "audit", Description: "audit", State: state.StatusNotStarted, IsAudit: true},
	}
	state.SaveNodeState(filepath.Join(leafDir, "state.json"), ns)

	idx.Nodes["bad-complete"] = state.IndexEntry{
		Name: "Bad Complete", Type: state.NodeLeaf, State: state.StatusComplete, Address: "bad-complete",
	}

	engine := NewEngine(dir, DefaultNodeLoader(dir))
	report := engine.ValidateAll(idx)

	found := false
	for _, issue := range report.Issues {
		if issue.Category == CatCompleteWithIncomplete {
			found = true
		}
	}
	if !found {
		t.Error("expected INVALID_TRANSITION_COMPLETE_WITH_INCOMPLETE issue")
	}
}

func TestValidateAll_DetectsBlockedWithoutReason(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	idx := state.NewRootIndex()

	leafDir := filepath.Join(dir, "no-reason")
	os.MkdirAll(leafDir, 0755)
	ns := state.NewNodeState("no-reason", "No Reason", state.NodeLeaf)
	ns.State = state.StatusBlocked
	ns.Tasks = []state.Task{
		{ID: "task-1", Description: "work", State: state.StatusBlocked, BlockedReason: ""},
		{ID: "audit", Description: "audit", State: state.StatusNotStarted, IsAudit: true},
	}
	state.SaveNodeState(filepath.Join(leafDir, "state.json"), ns)

	idx.Nodes["no-reason"] = state.IndexEntry{
		Name: "No Reason", Type: state.NodeLeaf, State: state.StatusBlocked, Address: "no-reason",
	}

	engine := NewEngine(dir, DefaultNodeLoader(dir))
	report := engine.ValidateAll(idx)

	found := false
	for _, issue := range report.Issues {
		if issue.Category == CatBlockedWithoutReason {
			found = true
		}
	}
	if !found {
		t.Error("expected INVALID_TRANSITION_BLOCKED_WITHOUT_REASON issue")
	}
}

func TestValidateAll_DetectsNegativeFailureCount(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	idx := state.NewRootIndex()

	leafDir := filepath.Join(dir, "neg-fail")
	os.MkdirAll(leafDir, 0755)
	ns := state.NewNodeState("neg-fail", "Neg Fail", state.NodeLeaf)
	ns.Tasks = []state.Task{
		{ID: "task-1", Description: "work", State: state.StatusNotStarted, FailureCount: -3},
		{ID: "audit", Description: "audit", State: state.StatusNotStarted, IsAudit: true},
	}
	state.SaveNodeState(filepath.Join(leafDir, "state.json"), ns)

	idx.Nodes["neg-fail"] = state.IndexEntry{
		Name: "Neg Fail", Type: state.NodeLeaf, State: state.StatusNotStarted, Address: "neg-fail",
	}

	engine := NewEngine(dir, DefaultNodeLoader(dir))
	report := engine.ValidateAll(idx)

	found := false
	for _, issue := range report.Issues {
		if issue.Category == CatNegativeFailureCount {
			found = true
		}
	}
	if !found {
		t.Error("expected NEGATIVE_FAILURE_COUNT issue")
	}
}

func TestValidateAll_DetectsMissingRequiredField(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	idx := state.NewRootIndex()

	leafDir := filepath.Join(dir, "empty-fields")
	os.MkdirAll(leafDir, 0755)
	ns := &state.NodeState{
		Version: 1,
		ID:      "",
		Name:    "",
		Type:    state.NodeLeaf,
		State:   state.StatusNotStarted,
	}
	state.SaveNodeState(filepath.Join(leafDir, "state.json"), ns)

	idx.Nodes["empty-fields"] = state.IndexEntry{
		Name: "Empty", Type: state.NodeLeaf, State: state.StatusNotStarted, Address: "empty-fields",
	}

	engine := NewEngine(dir, DefaultNodeLoader(dir))
	report := engine.ValidateAll(idx)

	found := false
	for _, issue := range report.Issues {
		if issue.Category == CatMissingRequiredField {
			found = true
		}
	}
	if !found {
		t.Error("expected MISSING_REQUIRED_FIELD issue")
	}
}

func TestValidateAll_DetectsMultipleInProgress(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	idx := state.NewRootIndex()

	for _, name := range []string{"leaf-a", "leaf-b"} {
		leafDir := filepath.Join(dir, name)
		os.MkdirAll(leafDir, 0755)
		ns := state.NewNodeState(name, name, state.NodeLeaf)
		ns.State = state.StatusInProgress
		ns.Tasks = []state.Task{
			{ID: "task-1", Description: "work", State: state.StatusInProgress},
			{ID: "audit", Description: "audit", State: state.StatusNotStarted, IsAudit: true},
		}
		state.SaveNodeState(filepath.Join(leafDir, "state.json"), ns)
		idx.Nodes[name] = state.IndexEntry{
			Name: name, Type: state.NodeLeaf, State: state.StatusInProgress, Address: name,
		}
	}

	engine := NewEngine(dir, DefaultNodeLoader(dir))
	report := engine.ValidateAll(idx)

	found := false
	for _, issue := range report.Issues {
		if issue.Category == CatMultipleInProgress {
			found = true
		}
	}
	if !found {
		t.Error("expected MULTIPLE_IN_PROGRESS issue")
	}
}

func TestValidateAll_DetectsDepthMismatch(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	idx := state.NewRootIndex()

	// Parent with depth 3
	parentDir := filepath.Join(dir, "parent")
	os.MkdirAll(parentDir, 0755)
	parentNS := state.NewNodeState("parent", "Parent", state.NodeOrchestrator)
	parentNS.DecompositionDepth = 3
	parentNS.Children = []state.ChildRef{{ID: "child", Address: "parent/child", State: state.StatusNotStarted}}
	state.SaveNodeState(filepath.Join(parentDir, "state.json"), parentNS)

	// Child with depth 1 (less than parent — invalid)
	childDir := filepath.Join(dir, "parent", "child")
	os.MkdirAll(childDir, 0755)
	childNS := state.NewNodeState("child", "Child", state.NodeLeaf)
	childNS.DecompositionDepth = 1
	childNS.Tasks = []state.Task{
		{ID: "task-1", Description: "work", State: state.StatusNotStarted},
		{ID: "audit", Description: "audit", State: state.StatusNotStarted, IsAudit: true},
	}
	state.SaveNodeState(filepath.Join(childDir, "state.json"), childNS)

	idx.Nodes["parent"] = state.IndexEntry{
		Name: "Parent", Type: state.NodeOrchestrator, State: state.StatusNotStarted, Address: "parent",
		Children: []string{"parent/child"},
	}
	idx.Nodes["parent/child"] = state.IndexEntry{
		Name: "Child", Type: state.NodeLeaf, State: state.StatusNotStarted, Address: "parent/child",
		Parent: "parent",
	}

	engine := NewEngine(dir, DefaultNodeLoader(dir))
	report := engine.ValidateAll(idx)

	found := false
	for _, issue := range report.Issues {
		if issue.Category == CatDepthMismatch {
			found = true
		}
	}
	if !found {
		t.Error("expected DEPTH_MISMATCH issue")
	}
}

func TestValidateAll_DetectsPropagationMismatch(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	idx := state.NewRootIndex()

	// Create orchestrator claiming complete but child is not_started
	orchDir := filepath.Join(dir, "orch")
	os.MkdirAll(orchDir, 0755)
	orchNS := state.NewNodeState("orch", "Orch", state.NodeOrchestrator)
	orchNS.State = state.StatusComplete
	orchNS.Children = []state.ChildRef{{ID: "child", Address: "orch/child", State: state.StatusNotStarted}}
	state.SaveNodeState(filepath.Join(orchDir, "state.json"), orchNS)

	childDir := filepath.Join(dir, "orch", "child")
	os.MkdirAll(childDir, 0755)
	childNS := state.NewNodeState("child", "Child", state.NodeLeaf)
	childNS.Tasks = []state.Task{
		{ID: "task-1", Description: "work", State: state.StatusNotStarted},
		{ID: "audit", Description: "audit", State: state.StatusNotStarted, IsAudit: true},
	}
	state.SaveNodeState(filepath.Join(childDir, "state.json"), childNS)

	idx.Nodes["orch"] = state.IndexEntry{
		Name: "Orch", Type: state.NodeOrchestrator, State: state.StatusComplete, Address: "orch",
		Children: []string{"orch/child"},
	}
	idx.Nodes["orch/child"] = state.IndexEntry{
		Name: "Child", Type: state.NodeLeaf, State: state.StatusNotStarted, Address: "orch/child",
		Parent: "orch",
	}

	engine := NewEngine(dir, DefaultNodeLoader(dir))
	report := engine.ValidateAll(idx)

	found := false
	for _, issue := range report.Issues {
		if issue.Category == CatPropagationMismatch {
			found = true
		}
	}
	if !found {
		t.Error("expected PROPAGATION_MISMATCH issue")
	}
}

func TestValidateStartup_OnlyRunsSubset(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	idx := state.NewRootIndex()

	// Create node with orphan definition (not in startup subset)
	leafDir := filepath.Join(dir, "leaf")
	os.MkdirAll(leafDir, 0755)
	ns := state.NewNodeState("leaf", "Leaf", state.NodeLeaf)
	ns.Tasks = []state.Task{
		{ID: "task-1", Description: "work", State: state.StatusNotStarted},
		{ID: "audit", Description: "audit", State: state.StatusNotStarted, IsAudit: true},
	}
	state.SaveNodeState(filepath.Join(leafDir, "state.json"), ns)
	idx.Nodes["leaf"] = state.IndexEntry{
		Name: "Leaf", Type: state.NodeLeaf, State: state.StatusNotStarted, Address: "leaf",
	}

	// Write an orphan .md file — ORPHAN_DEFINITION is not in startup subset
	os.WriteFile(filepath.Join(dir, "nonexistent", "readme.md"), []byte("orphan"), 0644)

	engine := NewEngine(dir, DefaultNodeLoader(dir))
	report := engine.ValidateStartup(idx)

	for _, issue := range report.Issues {
		if issue.Category == CatOrphanDefinition {
			t.Error("ORPHAN_DEFINITION should not appear in startup validation")
		}
	}
}

func TestNormalizeStateValue(t *testing.T) {
	t.Parallel()

	tests := []struct {
		input    string
		expected state.NodeStatus
		ok       bool
	}{
		{"complete", state.StatusComplete, true},
		{"completed", state.StatusComplete, true},
		{"done", state.StatusComplete, true},
		{"not_started", state.StatusNotStarted, true},
		{"not-started", state.StatusNotStarted, true},
		{"pending", state.StatusNotStarted, true},
		{"todo", state.StatusNotStarted, true},
		{"in_progress", state.StatusInProgress, true},
		{"in-progress", state.StatusInProgress, true},
		{"started", state.StatusInProgress, true},
		{"doing", state.StatusInProgress, true},
		{"blocked", state.StatusBlocked, true},
		{"stuck", state.StatusBlocked, true},
		{"invalid", "", false},
		{"", "", false},
	}

	for _, tt := range tests {
		result, ok := NormalizeStateValue(tt.input)
		if ok != tt.ok || result != tt.expected {
			t.Errorf("NormalizeStateValue(%q) = (%q, %v), want (%q, %v)", tt.input, result, ok, tt.expected, tt.ok)
		}
	}
}

func TestApplyDeterministicFixes_MissingAudit(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	idx := state.NewRootIndex()

	leafDir := filepath.Join(dir, "leaf")
	os.MkdirAll(leafDir, 0755)
	ns := state.NewNodeState("leaf", "Leaf", state.NodeLeaf)
	ns.Tasks = []state.Task{
		{ID: "task-1", Description: "work", State: state.StatusNotStarted},
	}
	statePath := filepath.Join(leafDir, "state.json")
	state.SaveNodeState(statePath, ns)

	idx.Nodes["leaf"] = state.IndexEntry{
		Name: "Leaf", Type: state.NodeLeaf, State: state.StatusNotStarted, Address: "leaf",
	}

	issues := []Issue{{
		Severity: SeverityError, Category: CatMissingAuditTask, Node: "leaf",
		CanAutoFix: true, FixType: FixDeterministic,
	}}

	idxPath := filepath.Join(dir, "state.json")
	state.SaveRootIndex(idxPath, idx)

	fixes, _, err := ApplyDeterministicFixes(idx, issues, dir, idxPath)
	if err != nil {
		t.Fatal(err)
	}
	if len(fixes) != 1 {
		t.Fatalf("expected 1 fix, got %d", len(fixes))
	}

	// Verify audit task was added
	loaded, err := state.LoadNodeState(statePath)
	if err != nil {
		t.Fatal(err)
	}
	if len(loaded.Tasks) != 2 {
		t.Errorf("expected 2 tasks, got %d", len(loaded.Tasks))
	}
	lastTask := loaded.Tasks[len(loaded.Tasks)-1]
	if !lastTask.IsAudit {
		t.Error("expected last task to have IsAudit=true")
	}
}

func TestValidateAll_DetectsInvalidAuditStatus(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	idx := state.NewRootIndex()

	leafDir := filepath.Join(dir, "bad-audit")
	os.MkdirAll(leafDir, 0755)
	ns := state.NewNodeState("bad-audit", "Bad Audit", state.NodeLeaf)
	ns.Audit.Status = "garbage"
	ns.Tasks = []state.Task{
		{ID: "task-1", Description: "work", State: state.StatusNotStarted},
		{ID: "audit", Description: "audit", State: state.StatusNotStarted, IsAudit: true},
	}
	state.SaveNodeState(filepath.Join(leafDir, "state.json"), ns)

	idx.Nodes["bad-audit"] = state.IndexEntry{
		Name: "Bad Audit", Type: state.NodeLeaf, State: state.StatusNotStarted, Address: "bad-audit",
	}

	engine := NewEngine(dir, DefaultNodeLoader(dir))
	report := engine.ValidateAll(idx)

	found := false
	for _, issue := range report.Issues {
		if issue.Category == CatInvalidAuditStatus {
			found = true
		}
	}
	if !found {
		t.Error("expected INVALID_AUDIT_STATUS issue")
	}
}

func TestValidateAll_DetectsAuditStatusTaskMismatch(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	idx := state.NewRootIndex()

	leafDir := filepath.Join(dir, "mismatch")
	os.MkdirAll(leafDir, 0755)
	ns := state.NewNodeState("mismatch", "Mismatch", state.NodeLeaf)
	ns.State = state.StatusInProgress
	ns.Audit.Status = state.AuditPassed // wrong — should be in_progress
	ns.Tasks = []state.Task{
		{ID: "task-1", Description: "work", State: state.StatusInProgress},
		{ID: "audit", Description: "audit", State: state.StatusNotStarted, IsAudit: true},
	}
	state.SaveNodeState(filepath.Join(leafDir, "state.json"), ns)

	idx.Nodes["mismatch"] = state.IndexEntry{
		Name: "Mismatch", Type: state.NodeLeaf, State: state.StatusInProgress, Address: "mismatch",
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
		t.Error("expected AUDIT_STATUS_TASK_MISMATCH issue")
	}
}

func TestValidateAll_DetectsInvalidAuditGap(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	idx := state.NewRootIndex()

	leafDir := filepath.Join(dir, "bad-gap")
	os.MkdirAll(leafDir, 0755)
	ns := state.NewNodeState("bad-gap", "Bad Gap", state.NodeLeaf)
	ns.Audit.Gaps = []state.Gap{
		{ID: "", Description: "", Status: state.GapOpen}, // missing ID and description
	}
	ns.Tasks = []state.Task{
		{ID: "task-1", Description: "work", State: state.StatusNotStarted},
		{ID: "audit", Description: "audit", State: state.StatusNotStarted, IsAudit: true},
	}
	state.SaveNodeState(filepath.Join(leafDir, "state.json"), ns)

	idx.Nodes["bad-gap"] = state.IndexEntry{
		Name: "Bad Gap", Type: state.NodeLeaf, State: state.StatusNotStarted, Address: "bad-gap",
	}

	engine := NewEngine(dir, DefaultNodeLoader(dir))
	report := engine.ValidateAll(idx)

	found := false
	for _, issue := range report.Issues {
		if issue.Category == CatInvalidAuditGap {
			found = true
		}
	}
	if !found {
		t.Error("expected INVALID_AUDIT_GAP issue")
	}
}

func TestValidateAll_DetectsStaleGapMetadata(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	idx := state.NewRootIndex()

	leafDir := filepath.Join(dir, "stale-gap")
	os.MkdirAll(leafDir, 0755)
	ns := state.NewNodeState("stale-gap", "Stale Gap", state.NodeLeaf)
	ns.Audit.Gaps = []state.Gap{
		{ID: "gap-1", Description: "test gap", Status: state.GapOpen, FixedBy: "should-not-be-here"},
	}
	ns.Tasks = []state.Task{
		{ID: "task-1", Description: "work", State: state.StatusNotStarted},
		{ID: "audit", Description: "audit", State: state.StatusNotStarted, IsAudit: true},
	}
	state.SaveNodeState(filepath.Join(leafDir, "state.json"), ns)

	idx.Nodes["stale-gap"] = state.IndexEntry{
		Name: "Stale Gap", Type: state.NodeLeaf, State: state.StatusNotStarted, Address: "stale-gap",
	}

	engine := NewEngine(dir, DefaultNodeLoader(dir))
	report := engine.ValidateAll(idx)

	found := false
	for _, issue := range report.Issues {
		if issue.Category == CatInvalidAuditGap && strings.Contains(issue.Description, "stale") {
			found = true
		}
	}
	if !found {
		t.Error("expected INVALID_AUDIT_GAP issue for stale fixed_by metadata")
	}
}

func TestValidateAll_DetectsInvalidAuditEscalation(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	idx := state.NewRootIndex()

	leafDir := filepath.Join(dir, "bad-esc")
	os.MkdirAll(leafDir, 0755)
	ns := state.NewNodeState("bad-esc", "Bad Escalation", state.NodeLeaf)
	ns.Audit.Escalations = []state.Escalation{
		{ID: "", Description: "", SourceNode: ""}, // all empty
	}
	ns.Tasks = []state.Task{
		{ID: "task-1", Description: "work", State: state.StatusNotStarted},
		{ID: "audit", Description: "audit", State: state.StatusNotStarted, IsAudit: true},
	}
	state.SaveNodeState(filepath.Join(leafDir, "state.json"), ns)

	idx.Nodes["bad-esc"] = state.IndexEntry{
		Name: "Bad Escalation", Type: state.NodeLeaf, State: state.StatusNotStarted, Address: "bad-esc",
	}

	engine := NewEngine(dir, DefaultNodeLoader(dir))
	report := engine.ValidateAll(idx)

	found := false
	for _, issue := range report.Issues {
		if issue.Category == CatInvalidAuditEscalation {
			found = true
		}
	}
	if !found {
		t.Error("expected INVALID_AUDIT_ESCALATION issue")
	}
}

func TestValidateAll_DetectsInvalidAuditScope(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	idx := state.NewRootIndex()

	leafDir := filepath.Join(dir, "bad-scope")
	os.MkdirAll(leafDir, 0755)
	ns := state.NewNodeState("bad-scope", "Bad Scope", state.NodeLeaf)
	ns.Audit.Scope = &state.AuditScope{Description: ""} // empty description
	ns.Tasks = []state.Task{
		{ID: "task-1", Description: "work", State: state.StatusNotStarted},
		{ID: "audit", Description: "audit", State: state.StatusNotStarted, IsAudit: true},
	}
	state.SaveNodeState(filepath.Join(leafDir, "state.json"), ns)

	idx.Nodes["bad-scope"] = state.IndexEntry{
		Name: "Bad Scope", Type: state.NodeLeaf, State: state.StatusNotStarted, Address: "bad-scope",
	}

	engine := NewEngine(dir, DefaultNodeLoader(dir))
	report := engine.ValidateAll(idx)

	found := false
	for _, issue := range report.Issues {
		if issue.Category == CatInvalidAuditScope {
			found = true
		}
	}
	if !found {
		t.Error("expected INVALID_AUDIT_SCOPE issue")
	}
}

func TestApplyDeterministicFixes_NegativeFailureCount(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	idx := state.NewRootIndex()

	leafDir := filepath.Join(dir, "leaf")
	os.MkdirAll(leafDir, 0755)
	ns := state.NewNodeState("leaf", "Leaf", state.NodeLeaf)
	ns.Tasks = []state.Task{
		{ID: "task-1", Description: "work", State: state.StatusNotStarted, FailureCount: -5},
		{ID: "audit", Description: "audit", State: state.StatusNotStarted, IsAudit: true},
	}
	statePath := filepath.Join(leafDir, "state.json")
	state.SaveNodeState(statePath, ns)

	idx.Nodes["leaf"] = state.IndexEntry{
		Name: "Leaf", Type: state.NodeLeaf, State: state.StatusNotStarted, Address: "leaf",
	}

	issues := []Issue{{
		Severity: SeverityError, Category: CatNegativeFailureCount, Node: "leaf",
		CanAutoFix: true, FixType: FixDeterministic,
	}}

	idxPath := filepath.Join(dir, "state.json")
	state.SaveRootIndex(idxPath, idx)

	fixes, _, err := ApplyDeterministicFixes(idx, issues, dir, idxPath)
	if err != nil {
		t.Fatal(err)
	}
	if len(fixes) != 1 {
		t.Fatalf("expected 1 fix, got %d", len(fixes))
	}

	loaded, err := state.LoadNodeState(statePath)
	if err != nil {
		t.Fatal(err)
	}
	if loaded.Tasks[0].FailureCount != 0 {
		t.Errorf("expected failure count 0, got %d", loaded.Tasks[0].FailureCount)
	}
}
