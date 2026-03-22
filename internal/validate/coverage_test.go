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
// checkOrphanedStateFiles — direct method tests
// ═══════════════════════════════════════════════════════════════════════════

func TestCheckOrphanedStateFiles_NestedOrphan(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	idx := state.NewRootIndex()

	orphanDir := filepath.Join(dir, "parent", "orphan")
	_ = os.MkdirAll(orphanDir, 0755)
	ns := state.NewNodeState("orphan", "Orphan", state.NodeLeaf)
	_ = state.SaveNodeState(filepath.Join(orphanDir, "state.json"), ns)

	engine := NewEngine(dir, DefaultNodeLoader(dir))
	report := &Report{}
	engine.checkOrphanedStateFiles(idx, report)

	found := false
	for _, issue := range report.Issues {
		if issue.Category == CatRootIndexMissingEntry && issue.Node == "parent/orphan" {
			found = true
		}
	}
	if !found {
		t.Error("expected ROOTINDEX_MISSING_ENTRY for nested orphan state file")
	}
}

func TestCheckOrphanedStateFiles_RootStateSkipped(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	idx := state.NewRootIndex()
	_ = state.SaveRootIndex(filepath.Join(dir, "state.json"), idx)

	engine := NewEngine(dir, DefaultNodeLoader(dir))
	report := &Report{}
	engine.checkOrphanedStateFiles(idx, report)

	if len(report.Issues) > 0 {
		t.Error("root state.json should not be flagged as orphaned")
	}
}

func TestCheckOrphanedStateFiles_KnownNodeNotFlagged(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	idx := state.NewRootIndex()

	leafDir := filepath.Join(dir, "known")
	_ = os.MkdirAll(leafDir, 0755)
	ns := state.NewNodeState("known", "Known", state.NodeLeaf)
	_ = state.SaveNodeState(filepath.Join(leafDir, "state.json"), ns)
	idx.Nodes["known"] = state.IndexEntry{
		Name: "Known", Type: state.NodeLeaf, State: state.StatusNotStarted, Address: "known",
	}

	engine := NewEngine(dir, DefaultNodeLoader(dir))
	report := &Report{}
	engine.checkOrphanedStateFiles(idx, report)

	if len(report.Issues) > 0 {
		t.Error("known nodes should not be flagged")
	}
}

// ═══════════════════════════════════════════════════════════════════════════
// checkOrphanedDefinitions — direct method tests
// ═══════════════════════════════════════════════════════════════════════════

func TestCheckOrphanedDefinitions_NestedOrphanMD(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	idx := state.NewRootIndex()

	orphanDir := filepath.Join(dir, "parent", "ghost")
	_ = os.MkdirAll(orphanDir, 0755)
	_ = os.WriteFile(filepath.Join(orphanDir, "definition.md"), []byte("orphan def"), 0644)

	engine := NewEngine(dir, DefaultNodeLoader(dir))
	report := &Report{}
	engine.checkOrphanedDefinitions(idx, report)

	found := false
	for _, issue := range report.Issues {
		if issue.Category == CatOrphanDefinition {
			found = true
		}
	}
	if !found {
		t.Error("expected ORPHAN_DEFINITION for nested .md file")
	}
}

func TestCheckOrphanedDefinitions_KnownNodeNotFlagged(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	idx := state.NewRootIndex()

	leafDir := filepath.Join(dir, "tracked")
	_ = os.MkdirAll(leafDir, 0755)
	_ = os.WriteFile(filepath.Join(leafDir, "notes.md"), []byte("tracked"), 0644)
	idx.Nodes["tracked"] = state.IndexEntry{
		Name: "Tracked", Type: state.NodeLeaf, State: state.StatusNotStarted, Address: "tracked",
	}

	engine := NewEngine(dir, DefaultNodeLoader(dir))
	report := &Report{}
	engine.checkOrphanedDefinitions(idx, report)

	if len(report.Issues) > 0 {
		t.Error("tracked nodes should not generate orphan definition issues")
	}
}

func TestCheckOrphanedDefinitions_ArchiveSkipped(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	idx := state.NewRootIndex()

	archiveDir := filepath.Join(dir, ".archive", "old-project")
	_ = os.MkdirAll(archiveDir, 0755)
	_ = os.WriteFile(filepath.Join(archiveDir, "old-project.md"), []byte("archived"), 0644)
	_ = os.WriteFile(filepath.Join(archiveDir, "audit.md"), []byte("archived audit"), 0644)

	engine := NewEngine(dir, DefaultNodeLoader(dir))
	report := &Report{}
	engine.checkOrphanedDefinitions(idx, report)

	for _, issue := range report.Issues {
		if issue.Category == CatOrphanDefinition {
			t.Errorf("archived .md files should not produce ORPHAN_DEFINITION, got: %s", issue.Node)
		}
	}
}

// ═══════════════════════════════════════════════════════════════════════════
// FixWithVerification — multi-pass convergence
// ═══════════════════════════════════════════════════════════════════════════

func TestFixWithVerification_MultiplePassesConverge(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	idx := state.NewRootIndex()

	leafDir := filepath.Join(dir, "leaf")
	_ = os.MkdirAll(leafDir, 0755)
	ns := state.NewNodeState("leaf", "Leaf", state.NodeLeaf)
	ns.State = state.StatusNotStarted
	ns.Tasks = []state.Task{
		{ID: "task-0001", Description: "work", State: state.StatusNotStarted, FailureCount: -2},
	}
	_ = state.SaveNodeState(filepath.Join(leafDir, "state.json"), ns)

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

func TestFixWithVerification_NoFixableIssues_Coverage(t *testing.T) {
	t.Parallel()
	dir, idx := setupTestTree(t)
	idxPath := filepath.Join(dir, "state.json")
	_ = state.SaveRootIndex(idxPath, idx)

	fixes, report, err := FixWithVerification(dir, idxPath, DefaultNodeLoader(dir))
	if err != nil {
		t.Fatal(err)
	}
	if len(fixes) != 0 {
		t.Errorf("expected 0 fixes, got %d", len(fixes))
	}
	if report == nil {
		t.Fatal("expected non-nil report")
	}
}

func TestFixWithVerification_WithWolfcastleDirArg(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	wolfDir := filepath.Join(dir, ".wolfcastle")
	_ = os.MkdirAll(wolfDir, 0755)

	idx := state.NewRootIndex()
	leafDir := filepath.Join(dir, "system", "projects", "leaf")
	_ = os.MkdirAll(leafDir, 0755)
	ns := state.NewNodeState("leaf", "Leaf", state.NodeLeaf)
	ns.Tasks = []state.Task{
		{ID: "task-0001", Description: "work", State: state.StatusNotStarted},
		{ID: "audit", Description: "audit", State: state.StatusNotStarted, IsAudit: true},
	}
	_ = state.SaveNodeState(filepath.Join(leafDir, "state.json"), ns)
	idx.Nodes["leaf"] = state.IndexEntry{
		Name: "Leaf", Type: state.NodeLeaf, State: state.StatusNotStarted, Address: "leaf",
	}

	projDir := filepath.Join(dir, "system", "projects")
	idxPath := filepath.Join(projDir, "state.json")
	_ = state.SaveRootIndex(idxPath, idx)

	_, _, err := FixWithVerification(projDir, idxPath, DefaultNodeLoader(projDir), wolfDir)
	if err != nil {
		t.Fatal(err)
	}
}

// ═══════════════════════════════════════════════════════════════════════════
// TryModelAssistedFix — coverage for all code paths
// ═══════════════════════════════════════════════════════════════════════════

func TestTryModelAssistedFix_InvalidModelResponse(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()

	leafDir := filepath.Join(dir, "test-node")
	_ = os.MkdirAll(leafDir, 0755)
	ns := state.NewNodeState("test-node", "Test", state.NodeLeaf)
	_ = state.SaveNodeState(filepath.Join(leafDir, "state.json"), ns)

	issue := Issue{Node: "test-node", Category: CatInvalidStateValue, FixType: FixModelAssisted}
	model := config.ModelDef{Command: "echo", Args: []string{"not json at all"}}
	ok, err := TryModelAssistedFix(context.Background(), invoke.NewProcessInvoker(), model, issue, dir)
	if ok {
		t.Error("expected ok=false for invalid model response")
	}
	if err == nil {
		t.Error("expected parsing error")
	}
}

func TestTryModelAssistedFix_InvalidResolution(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()

	leafDir := filepath.Join(dir, "test-node")
	_ = os.MkdirAll(leafDir, 0755)
	ns := state.NewNodeState("test-node", "Test", state.NodeLeaf)
	_ = state.SaveNodeState(filepath.Join(leafDir, "state.json"), ns)

	issue := Issue{Node: "test-node", Category: CatInvalidStateValue, FixType: FixModelAssisted}
	model := config.ModelDef{Command: "printf", Args: []string{`{"resolution":"garbage","reason":"test"}`}}
	ok, err := TryModelAssistedFix(context.Background(), invoke.NewProcessInvoker(), model, issue, dir)
	if ok {
		t.Error("expected ok=false")
	}
	if err == nil || !strings.Contains(err.Error(), "invalid resolution") {
		t.Errorf("expected 'invalid resolution' error, got: %v", err)
	}
}

func TestTryModelAssistedFix_SuccessfulFix(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()

	leafDir := filepath.Join(dir, "test-node")
	_ = os.MkdirAll(leafDir, 0755)
	ns := state.NewNodeState("test-node", "Test", state.NodeLeaf)
	_ = state.SaveNodeState(filepath.Join(leafDir, "state.json"), ns)

	issue := Issue{Node: "test-node", Category: CatInvalidStateValue, FixType: FixModelAssisted}
	model := config.ModelDef{Command: "printf", Args: []string{`{"resolution":"complete","reason":"all done"}`}}
	ok, err := TryModelAssistedFix(context.Background(), invoke.NewProcessInvoker(), model, issue, dir)
	if err != nil {
		t.Fatal(err)
	}
	if !ok {
		t.Error("expected ok=true")
	}

	loaded, _ := state.LoadNodeState(filepath.Join(leafDir, "state.json"))
	if loaded.State != state.StatusComplete {
		t.Errorf("expected complete, got %s", loaded.State)
	}
}

func TestTryModelAssistedFix_WithWolfcastleDirTemplate(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	wolfDir := filepath.Join(dir, ".wolfcastle")
	promptsDir := filepath.Join(wolfDir, "system", "base", "prompts")
	_ = os.MkdirAll(promptsDir, 0755)
	_ = os.WriteFile(filepath.Join(promptsDir, "doctor.md"),
		[]byte(`Fix {{.Node}}: {{.Description}}`), 0644)

	leafDir := filepath.Join(dir, "test-node")
	_ = os.MkdirAll(leafDir, 0755)
	ns := state.NewNodeState("test-node", "Test", state.NodeLeaf)
	_ = state.SaveNodeState(filepath.Join(leafDir, "state.json"), ns)

	issue := Issue{Node: "test-node", Category: CatInvalidStateValue, FixType: FixModelAssisted, Description: "bad state"}
	model := config.ModelDef{Command: "printf", Args: []string{`{"resolution":"blocked","reason":"stuck"}`}}
	ok, err := TryModelAssistedFix(context.Background(), invoke.NewProcessInvoker(), model, issue, dir, wolfDir)
	if err != nil {
		t.Fatal(err)
	}
	if !ok {
		t.Error("expected ok=true")
	}
}

func TestTryModelAssistedFix_MissingNodeState(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	issue := Issue{Node: "ghost-node", Category: CatInvalidStateValue, FixType: FixModelAssisted}
	model := config.ModelDef{Command: "printf", Args: []string{`{"resolution":"complete","reason":"done"}`}}
	ok, err := TryModelAssistedFix(context.Background(), invoke.NewProcessInvoker(), model, issue, dir)
	if ok {
		t.Error("expected ok=false")
	}
	if err == nil {
		t.Error("expected error for missing node state file")
	}
}

// ═══════════════════════════════════════════════════════════════════════════
// ApplyDeterministicFixes — orphan definition and audit gap
// ═══════════════════════════════════════════════════════════════════════════

func TestApplyDeterministicFixes_OrphanDefinitionEntry(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	idx := state.NewRootIndex()
	idxPath := filepath.Join(dir, "state.json")
	_ = state.SaveRootIndex(idxPath, idx)

	issues := []Issue{{
		Severity: SeverityWarning, Category: CatOrphanDefinition,
		Node: "ghost", CanAutoFix: true, FixType: FixDeterministic,
	}}

	fixes, _, err := ApplyDeterministicFixes(idx, issues, dir, idxPath)
	if err != nil {
		t.Fatal(err)
	}
	if findFix(fixes, CatOrphanDefinition) == nil {
		t.Error("expected orphan definition fix entry")
	}
}

func TestApplyDeterministicFixes_InvalidAuditGapClearsMeta(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	idx := state.NewRootIndex()

	leafDir := filepath.Join(dir, "leaf")
	_ = os.MkdirAll(leafDir, 0755)
	ns := state.NewNodeState("leaf", "Leaf", state.NodeLeaf)
	ns.Audit.Gaps = []state.Gap{
		{ID: "gap-1", Description: "test", Status: state.GapOpen, FixedBy: "stale"},
	}
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
		Severity: SeverityError, Category: CatInvalidAuditGap,
		Node: "leaf", CanAutoFix: true, FixType: FixDeterministic,
	}}

	fixes, _, err := ApplyDeterministicFixes(idx, issues, dir, idxPath)
	if err != nil {
		t.Fatal(err)
	}
	if findFix(fixes, CatInvalidAuditGap) == nil {
		t.Error("expected audit gap fix")
	}

	loaded, _ := state.LoadNodeState(filepath.Join(leafDir, "state.json"))
	if loaded.Audit.Gaps[0].FixedBy != "" {
		t.Error("expected stale FixedBy to be cleared")
	}
}

func TestApplyDeterministicFixes_SkipsNonAutoFixableIssues(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	idx := state.NewRootIndex()
	idxPath := filepath.Join(dir, "state.json")
	_ = state.SaveRootIndex(idxPath, idx)

	issues := []Issue{{
		Severity: SeverityError, Category: CatMultipleInProgress,
		Node: "node", CanAutoFix: false, FixType: FixManual,
	}}

	fixes, _, err := ApplyDeterministicFixes(idx, issues, dir, idxPath)
	if err != nil {
		t.Fatal(err)
	}
	if len(fixes) != 0 {
		t.Errorf("expected 0 fixes, got %d", len(fixes))
	}
}
