package validate

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/dorkusprime/wolfcastle/internal/daemon"
	"github.com/dorkusprime/wolfcastle/internal/state"
)

func TestValidateAll_DetectsOrphanState(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	idx := state.NewRootIndex()

	// Create parent that doesn't list child
	parentDir := filepath.Join(dir, "parent")
	_ = os.MkdirAll(parentDir, 0755)
	parentNS := state.NewNodeState("parent", "Parent", state.NodeOrchestrator)
	parentNS.Children = []state.ChildRef{} // empty children
	_ = state.SaveNodeState(filepath.Join(parentDir, "state.json"), parentNS)

	childDir := filepath.Join(dir, "parent", "child")
	_ = os.MkdirAll(childDir, 0755)
	childNS := state.NewNodeState("child", "Child", state.NodeLeaf)
	childNS.Tasks = []state.Task{
		{ID: "task-0001", Description: "work", State: state.StatusNotStarted},
		{ID: "audit", Description: "audit", State: state.StatusNotStarted, IsAudit: true},
	}
	_ = state.SaveNodeState(filepath.Join(childDir, "state.json"), childNS)

	idx.Nodes["parent"] = state.IndexEntry{
		Name: "Parent", Type: state.NodeOrchestrator, State: state.StatusNotStarted,
		Address: "parent", Children: []string{"parent/child"},
	}
	idx.Nodes["parent/child"] = state.IndexEntry{
		Name: "Child", Type: state.NodeLeaf, State: state.StatusNotStarted,
		Address: "parent/child", Parent: "parent",
	}
	// Simulate orphan: parent index entry says children=["parent/child"],
	// but the actual parent entry in Nodes has no child listed
	parentEntry := idx.Nodes["parent"]
	parentEntry.Children = []string{} // mismatch
	idx.Nodes["parent"] = parentEntry

	engine := NewEngine(dir, DefaultNodeLoader(dir))
	report := engine.ValidateAll(idx)

	found := false
	for _, issue := range report.Issues {
		if issue.Category == CatOrphanState {
			found = true
		}
	}
	if !found {
		t.Error("expected ORPHAN_STATE issue when parent doesn't list child")
	}
}

func TestValidateAll_DetectsOrphanDefinition(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	idx := state.NewRootIndex()

	// Create a .md file in a directory not tracked by the index
	orphanDir := filepath.Join(dir, "ghost-node")
	_ = os.MkdirAll(orphanDir, 0755)
	_ = os.WriteFile(filepath.Join(orphanDir, "readme.md"), []byte("orphan"), 0644)

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

func TestValidateAll_DetectsCompleteOrchestratorWithIncompleteChildren(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	idx := state.NewRootIndex()

	orchDir := filepath.Join(dir, "orch")
	_ = os.MkdirAll(orchDir, 0755)
	orchNS := state.NewNodeState("orch", "Orch", state.NodeOrchestrator)
	orchNS.State = state.StatusComplete
	orchNS.Children = []state.ChildRef{
		{ID: "child", Address: "orch/child", State: state.StatusNotStarted},
	}
	_ = state.SaveNodeState(filepath.Join(orchDir, "state.json"), orchNS)

	childDir := filepath.Join(dir, "orch", "child")
	_ = os.MkdirAll(childDir, 0755)
	childNS := state.NewNodeState("child", "Child", state.NodeLeaf)
	childNS.Tasks = []state.Task{
		{ID: "task-0001", Description: "work", State: state.StatusNotStarted},
		{ID: "audit", Description: "audit", State: state.StatusNotStarted, IsAudit: true},
	}
	_ = state.SaveNodeState(filepath.Join(childDir, "state.json"), childNS)

	idx.Nodes["orch"] = state.IndexEntry{
		Name: "Orch", Type: state.NodeOrchestrator, State: state.StatusComplete,
		Address: "orch", Children: []string{"orch/child"},
	}
	idx.Nodes["orch/child"] = state.IndexEntry{
		Name: "Child", Type: state.NodeLeaf, State: state.StatusNotStarted,
		Address: "orch/child", Parent: "orch",
	}

	engine := NewEngine(dir, DefaultNodeLoader(dir))
	report := engine.ValidateAll(idx)

	found := false
	for _, issue := range report.Issues {
		if issue.Category == CatCompleteWithIncomplete && issue.Node == "orch" {
			found = true
		}
	}
	if !found {
		t.Error("expected COMPLETE_WITH_INCOMPLETE for orchestrator")
	}
}

func TestValidateAll_DetectsInvalidGapStatus(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	idx := state.NewRootIndex()

	leafDir := filepath.Join(dir, "bad-gap-status")
	_ = os.MkdirAll(leafDir, 0755)
	ns := state.NewNodeState("bad-gap-status", "Bad Gap Status", state.NodeLeaf)
	ns.Audit.Gaps = []state.Gap{
		{ID: "gap-1", Description: "test", Status: "invalid-status"},
	}
	ns.Tasks = []state.Task{
		{ID: "task-0001", Description: "work", State: state.StatusNotStarted},
		{ID: "audit", Description: "audit", State: state.StatusNotStarted, IsAudit: true},
	}
	_ = state.SaveNodeState(filepath.Join(leafDir, "state.json"), ns)

	idx.Nodes["bad-gap-status"] = state.IndexEntry{
		Name: "Bad Gap Status", Type: state.NodeLeaf, State: state.StatusNotStarted,
		Address: "bad-gap-status",
	}

	engine := NewEngine(dir, DefaultNodeLoader(dir))
	report := engine.ValidateAll(idx)

	found := false
	for _, issue := range report.Issues {
		if issue.Category == CatInvalidAuditGap && issue.Node == "bad-gap-status" {
			found = true
		}
	}
	if !found {
		t.Error("expected INVALID_AUDIT_GAP for invalid gap status")
	}
}

func TestValidateAll_DetectsInvalidStateValue(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	idx := state.NewRootIndex()

	leafDir := filepath.Join(dir, "bad-state")
	_ = os.MkdirAll(leafDir, 0755)
	ns := state.NewNodeState("bad-state", "Bad State", state.NodeLeaf)
	ns.State = "garbage"
	ns.Tasks = []state.Task{
		{ID: "task-0001", Description: "work", State: state.StatusNotStarted},
		{ID: "audit", Description: "audit", State: state.StatusNotStarted, IsAudit: true},
	}
	_ = state.SaveNodeState(filepath.Join(leafDir, "state.json"), ns)

	idx.Nodes["bad-state"] = state.IndexEntry{
		Name: "Bad State", Type: state.NodeLeaf, State: state.StatusNotStarted,
		Address: "bad-state",
	}

	engine := NewEngine(dir, DefaultNodeLoader(dir))
	report := engine.ValidateAll(idx)

	found := false
	for _, issue := range report.Issues {
		if issue.Category == CatInvalidStateValue {
			found = true
		}
	}
	if !found {
		t.Error("expected INVALID_STATE_VALUE issue")
	}
}

// TestValidateAll_DetectsStalePIDFile removed: PID files replaced by instance registry.

func TestValidateAll_DetectsStaleStopFile(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	wDir := t.TempDir()
	idx := state.NewRootIndex()

	_ = os.MkdirAll(filepath.Join(wDir, "system"), 0755)
	_ = os.WriteFile(filepath.Join(wDir, "system", "stop"), []byte(""), 0644)

	engine := NewEngine(dir, DefaultNodeLoader(dir), daemon.NewDaemonRepository(wDir))
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

func TestFixWithVerification_MultiPassFix(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()

	// Create a leaf with missing audit task
	leafDir := filepath.Join(dir, "leaf")
	_ = os.MkdirAll(leafDir, 0755)
	ns := state.NewNodeState("leaf", "Leaf", state.NodeLeaf)
	ns.Tasks = []state.Task{
		{ID: "task-0001", Description: "work", State: state.StatusNotStarted},
	}
	_ = state.SaveNodeState(filepath.Join(leafDir, "state.json"), ns)

	idx := state.NewRootIndex()
	idx.Nodes["leaf"] = state.IndexEntry{
		Name: "Leaf", Type: state.NodeLeaf, State: state.StatusNotStarted, Address: "leaf",
	}
	idxPath := filepath.Join(dir, "state.json")
	_ = state.SaveRootIndex(idxPath, idx)

	fixes, report, err := FixWithVerification(dir, idxPath, DefaultNodeLoader(dir))
	if err != nil {
		t.Fatal(err)
	}
	if len(fixes) == 0 {
		t.Error("expected at least one fix")
	}
	if report == nil {
		t.Fatal("expected non-nil final report")
	}
}

func TestFixWithVerification_NoIssues(t *testing.T) {
	t.Parallel()
	dir, idx := setupTestTree(t)
	idxPath := filepath.Join(dir, "state.json")
	_ = state.SaveRootIndex(idxPath, idx)

	fixes, report, err := FixWithVerification(dir, idxPath, DefaultNodeLoader(dir))
	if err != nil {
		t.Fatal(err)
	}
	if len(fixes) != 0 {
		t.Errorf("expected no fixes for healthy tree, got %d", len(fixes))
	}
	if report.HasErrors() {
		t.Error("healthy tree should have no errors")
	}
}

func TestExpectedAuditStatus_BlockedReturnsEmpty(t *testing.T) {
	t.Parallel()
	ns := state.NewNodeState("test", "Test", state.NodeLeaf)
	ns.State = state.StatusBlocked
	result := expectedAuditStatus(ns)
	if result != state.AuditFailed {
		t.Errorf("expected AuditFailed for blocked node, got %q", result)
	}
}

func TestExpectedAuditStatus_CompleteWithOpenGaps(t *testing.T) {
	t.Parallel()
	ns := state.NewNodeState("test", "Test", state.NodeLeaf)
	ns.State = state.StatusComplete
	ns.Audit.Gaps = []state.Gap{
		{ID: "gap-1", Description: "open gap", Status: state.GapOpen},
	}
	result := expectedAuditStatus(ns)
	if result != state.AuditFailed {
		t.Errorf("expected AuditFailed for complete node with open gaps, got %q", result)
	}
}

func TestExpectedAuditStatus_CompleteWithNoGaps(t *testing.T) {
	t.Parallel()
	ns := state.NewNodeState("test", "Test", state.NodeLeaf)
	ns.State = state.StatusComplete
	result := expectedAuditStatus(ns)
	if result != state.AuditPassed {
		t.Errorf("expected AuditPassed for complete node with no gaps, got %q", result)
	}
}

// TestApplyDeterministicFixes_StalePIDFile removed: PID files replaced by instance registry.

func TestApplyDeterministicFixes_StaleStopFile(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	wDir := t.TempDir()
	idx := state.NewRootIndex()
	idxPath := filepath.Join(dir, "state.json")
	_ = state.SaveRootIndex(idxPath, idx)

	_ = os.MkdirAll(filepath.Join(wDir, "system"), 0755)
	stopPath := filepath.Join(wDir, "system", "stop")
	_ = os.WriteFile(stopPath, []byte(""), 0644)

	issues := []Issue{{
		Severity: SeverityWarning, Category: CatStaleStopFile,
		CanAutoFix: true, FixType: FixDeterministic,
	}}

	fixes, _, err := ApplyDeterministicFixes(idx, issues, dir, idxPath, daemon.NewDaemonRepository(wDir))
	if err != nil {
		t.Fatal(err)
	}
	if len(fixes) != 1 {
		t.Fatalf("expected 1 fix, got %d", len(fixes))
	}

	if _, err := os.Stat(stopPath); !os.IsNotExist(err) {
		t.Error("stop file should have been removed")
	}
}

func TestApplyDeterministicFixes_InvalidStateValue(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	idx := state.NewRootIndex()

	leafDir := filepath.Join(dir, "leaf")
	_ = os.MkdirAll(leafDir, 0755)
	ns := state.NewNodeState("leaf", "Leaf", state.NodeLeaf)
	ns.State = "done" // normalizable to "complete"
	ns.Tasks = []state.Task{
		{ID: "task-0001", Description: "work", State: state.StatusComplete},
		{ID: "audit", Description: "audit", State: state.StatusComplete, IsAudit: true},
	}
	_ = state.SaveNodeState(filepath.Join(leafDir, "state.json"), ns)

	idx.Nodes["leaf"] = state.IndexEntry{
		Name: "Leaf", Type: state.NodeLeaf, State: state.StatusNotStarted, Address: "leaf",
	}
	idxPath := filepath.Join(dir, "state.json")
	_ = state.SaveRootIndex(idxPath, idx)

	issues := []Issue{{
		Severity: SeverityError, Category: CatInvalidStateValue, Node: "leaf",
		CanAutoFix: true, FixType: FixDeterministic,
	}}

	fixes, _, err := ApplyDeterministicFixes(idx, issues, dir, idxPath)
	if err != nil {
		t.Fatal(err)
	}
	if len(fixes) != 1 {
		t.Fatalf("expected 1 fix, got %d", len(fixes))
	}

	loaded, _ := state.LoadNodeState(filepath.Join(leafDir, "state.json"))
	if loaded.State != state.StatusComplete {
		t.Errorf("expected state complete, got %q", loaded.State)
	}
}

func TestApplyDeterministicFixes_BlockedWithoutReason(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	idx := state.NewRootIndex()

	leafDir := filepath.Join(dir, "leaf")
	_ = os.MkdirAll(leafDir, 0755)
	ns := state.NewNodeState("leaf", "Leaf", state.NodeLeaf)
	ns.State = state.StatusBlocked
	ns.Tasks = []state.Task{
		{ID: "task-0001", Description: "work", State: state.StatusBlocked, BlockedReason: ""},
		{ID: "audit", Description: "audit", State: state.StatusNotStarted, IsAudit: true},
	}
	_ = state.SaveNodeState(filepath.Join(leafDir, "state.json"), ns)

	idx.Nodes["leaf"] = state.IndexEntry{
		Name: "Leaf", Type: state.NodeLeaf, State: state.StatusBlocked, Address: "leaf",
	}
	idxPath := filepath.Join(dir, "state.json")
	_ = state.SaveRootIndex(idxPath, idx)

	issues := []Issue{{
		Severity: SeverityError, Category: CatBlockedWithoutReason, Node: "leaf",
		CanAutoFix: true, FixType: FixDeterministic,
	}}

	fixes, _, err := ApplyDeterministicFixes(idx, issues, dir, idxPath)
	if err != nil {
		t.Fatal(err)
	}
	if len(fixes) != 1 {
		t.Fatalf("expected 1 fix, got %d", len(fixes))
	}

	loaded, _ := state.LoadNodeState(filepath.Join(leafDir, "state.json"))
	if loaded.Tasks[0].BlockedReason == "" {
		t.Error("expected blocked reason to be populated")
	}
}

func TestApplyDeterministicFixes_MissingRequiredField(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	idx := state.NewRootIndex()

	leafDir := filepath.Join(dir, "leaf")
	_ = os.MkdirAll(leafDir, 0755)
	ns := &state.NodeState{Version: 1, Type: state.NodeLeaf}
	_ = state.SaveNodeState(filepath.Join(leafDir, "state.json"), ns)

	idx.Nodes["leaf"] = state.IndexEntry{
		Name: "Leaf", Type: state.NodeLeaf, State: state.StatusNotStarted, Address: "leaf",
	}
	idxPath := filepath.Join(dir, "state.json")
	_ = state.SaveRootIndex(idxPath, idx)

	issues := []Issue{{
		Severity: SeverityError, Category: CatMissingRequiredField, Node: "leaf",
		CanAutoFix: true, FixType: FixDeterministic,
	}}

	fixes, _, err := ApplyDeterministicFixes(idx, issues, dir, idxPath)
	if err != nil {
		t.Fatal(err)
	}
	if len(fixes) != 1 {
		t.Fatalf("expected 1 fix, got %d", len(fixes))
	}

	loaded, _ := state.LoadNodeState(filepath.Join(leafDir, "state.json"))
	if loaded.ID == "" || loaded.Name == "" {
		t.Error("expected ID and Name to be populated")
	}
}

func TestApplyDeterministicFixes_InvalidAuditStatus(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	idx := state.NewRootIndex()

	leafDir := filepath.Join(dir, "leaf")
	_ = os.MkdirAll(leafDir, 0755)
	ns := state.NewNodeState("leaf", "Leaf", state.NodeLeaf)
	ns.Audit.Status = "garbage"
	ns.Tasks = []state.Task{
		{ID: "audit", Description: "audit", State: state.StatusNotStarted, IsAudit: true},
	}
	_ = state.SaveNodeState(filepath.Join(leafDir, "state.json"), ns)

	idx.Nodes["leaf"] = state.IndexEntry{
		Name: "Leaf", Type: state.NodeLeaf, State: state.StatusNotStarted, Address: "leaf",
	}
	idxPath := filepath.Join(dir, "state.json")
	_ = state.SaveRootIndex(idxPath, idx)

	issues := []Issue{{
		Severity: SeverityError, Category: CatInvalidAuditStatus, Node: "leaf",
		CanAutoFix: true, FixType: FixDeterministic,
	}}

	fixes, _, err := ApplyDeterministicFixes(idx, issues, dir, idxPath)
	if err != nil {
		t.Fatal(err)
	}
	if len(fixes) != 1 {
		t.Fatalf("expected 1 fix, got %d", len(fixes))
	}

	loaded, _ := state.LoadNodeState(filepath.Join(leafDir, "state.json"))
	if loaded.Audit.Status != state.AuditPending {
		t.Errorf("expected audit status pending, got %q", loaded.Audit.Status)
	}
}

func TestApplyDeterministicFixes_AuditStatusTaskMismatch(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	idx := state.NewRootIndex()

	leafDir := filepath.Join(dir, "leaf")
	_ = os.MkdirAll(leafDir, 0755)
	ns := state.NewNodeState("leaf", "Leaf", state.NodeLeaf)
	ns.State = state.StatusInProgress
	ns.Audit.Status = state.AuditPassed // wrong for in_progress
	ns.Tasks = []state.Task{
		{ID: "task-0001", Description: "work", State: state.StatusInProgress},
		{ID: "audit", Description: "audit", State: state.StatusNotStarted, IsAudit: true},
	}
	_ = state.SaveNodeState(filepath.Join(leafDir, "state.json"), ns)

	idx.Nodes["leaf"] = state.IndexEntry{
		Name: "Leaf", Type: state.NodeLeaf, State: state.StatusInProgress, Address: "leaf",
	}
	idxPath := filepath.Join(dir, "state.json")
	_ = state.SaveRootIndex(idxPath, idx)

	issues := []Issue{{
		Severity: SeverityWarning, Category: CatAuditStatusTaskMismatch, Node: "leaf",
		CanAutoFix: true, FixType: FixDeterministic,
	}}

	fixes, _, err := ApplyDeterministicFixes(idx, issues, dir, idxPath)
	if err != nil {
		t.Fatal(err)
	}
	if len(fixes) != 1 {
		t.Fatalf("expected 1 fix, got %d", len(fixes))
	}

	loaded, _ := state.LoadNodeState(filepath.Join(leafDir, "state.json"))
	if loaded.Audit.Status != state.AuditInProgress {
		t.Errorf("expected audit status in_progress, got %q", loaded.Audit.Status)
	}
}

func TestApplyDeterministicFixes_SkipsNonDeterministicFixes(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	idx := state.NewRootIndex()
	idxPath := filepath.Join(dir, "state.json")
	_ = state.SaveRootIndex(idxPath, idx)

	issues := []Issue{
		{Severity: SeverityError, Category: CatMultipleInProgress, CanAutoFix: false, FixType: FixModelAssisted},
		{Severity: SeverityWarning, Category: CatOrphanDefinition, Node: "test", CanAutoFix: false, FixType: FixManual},
	}

	fixes, _, err := ApplyDeterministicFixes(idx, issues, dir, idxPath)
	if err != nil {
		t.Fatal(err)
	}
	if len(fixes) != 0 {
		t.Errorf("expected 0 fixes for non-deterministic issues, got %d", len(fixes))
	}
}

func TestApplyDeterministicFixes_DanglingRef(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	idx := state.NewRootIndex()
	idx.Root = []string{"missing"}
	idx.Nodes["missing"] = state.IndexEntry{
		Name: "Missing", Type: state.NodeLeaf, State: state.StatusNotStarted, Address: "missing",
	}
	idxPath := filepath.Join(dir, "state.json")
	_ = state.SaveRootIndex(idxPath, idx)

	issues := []Issue{{
		Severity: SeverityError, Category: CatRootIndexDanglingRef, Node: "missing",
		CanAutoFix: true, FixType: FixDeterministic,
	}}

	fixes, _, err := ApplyDeterministicFixes(idx, issues, dir, idxPath)
	if err != nil {
		t.Fatal(err)
	}
	if len(fixes) != 1 {
		t.Fatalf("expected 1 fix, got %d", len(fixes))
	}
	if _, ok := idx.Nodes["missing"]; ok {
		t.Error("dangling node should have been removed from index")
	}
	if len(idx.Root) != 0 {
		t.Errorf("expected empty root after removing dangling ref, got %v", idx.Root)
	}
}

func TestFixWithVerification_ErrorOnMissingIndex(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	missingPath := filepath.Join(dir, "nonexistent", "state.json")

	_, _, err := FixWithVerification(dir, missingPath, DefaultNodeLoader(dir))
	if err == nil {
		t.Error("expected error when index file doesn't exist")
	}
}

func TestApplyDeterministicFixes_DepthMismatch(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	idx := state.NewRootIndex()

	parentDir := filepath.Join(dir, "parent")
	_ = os.MkdirAll(parentDir, 0755)
	parentNS := state.NewNodeState("parent", "Parent", state.NodeOrchestrator)
	parentNS.DecompositionDepth = 3
	_ = state.SaveNodeState(filepath.Join(parentDir, "state.json"), parentNS)

	childDir := filepath.Join(dir, "parent", "child")
	_ = os.MkdirAll(childDir, 0755)
	childNS := state.NewNodeState("child", "Child", state.NodeLeaf)
	childNS.DecompositionDepth = 1
	childNS.Tasks = []state.Task{
		{ID: "audit", Description: "audit", State: state.StatusNotStarted, IsAudit: true},
	}
	_ = state.SaveNodeState(filepath.Join(childDir, "state.json"), childNS)

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

	issues := []Issue{{
		Severity: SeverityError, Category: CatDepthMismatch, Node: "parent/child",
		CanAutoFix: true, FixType: FixDeterministic,
	}}

	fixes, _, err := ApplyDeterministicFixes(idx, issues, dir, idxPath)
	if err != nil {
		t.Fatal(err)
	}
	if len(fixes) != 1 {
		t.Fatalf("expected 1 fix, got %d", len(fixes))
	}

	loaded, _ := state.LoadNodeState(filepath.Join(childDir, "state.json"))
	if loaded.DecompositionDepth != 3 {
		t.Errorf("expected depth 3, got %d", loaded.DecompositionDepth)
	}
}
