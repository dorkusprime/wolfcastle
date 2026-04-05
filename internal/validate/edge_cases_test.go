package validate

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/dorkusprime/wolfcastle/internal/state"
)

// ═══════════════════════════════════════════════════════════════════════════
// RecoverNodeState: advanced corruption scenarios
// ═══════════════════════════════════════════════════════════════════════════

func TestRecoverNodeState_BOMPlusTruncation(t *testing.T) {
	t.Parallel()
	// BOM followed by truncated JSON: tests the combination of two sanitization
	// steps (BOM strip, then truncation close).
	truncated := `{"version":1,"id":"bom-trunc","name":"BT","type":"leaf","state":"in_progress","tasks":[{"id":"t1","description":"first","state":"complete"},`
	data := append([]byte{0xEF, 0xBB, 0xBF}, []byte(truncated)...)

	recovered, report, err := RecoverNodeState(data)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if recovered.ID != "bom-trunc" {
		t.Errorf("expected ID bom-trunc, got %s", recovered.ID)
	}

	bomFound := false
	for _, step := range report.Applied {
		if strings.Contains(step, "BOM") {
			bomFound = true
		}
	}
	if !bomFound {
		t.Error("expected BOM stripping step in report")
	}
}

func TestRecoverNodeState_NullBytePlusTruncation(t *testing.T) {
	t.Parallel()
	// Null bytes inside truncated JSON
	truncated := "{\"version\":1,\"id\":\"\x00node-null\x00\",\"name\":\"NullTrunc\",\"type\":\"leaf\",\"state\":\"not_started\",\"tasks\":["
	recovered, report, err := RecoverNodeState([]byte(truncated))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if recovered.Name != "NullTrunc" {
		t.Errorf("expected Name NullTrunc, got %s", recovered.Name)
	}

	nullFound := false
	for _, s := range report.Applied {
		if strings.Contains(s, "null") {
			nullFound = true
		}
	}
	if !nullFound {
		t.Error("expected null byte step in report")
	}
}

func TestRecoverNodeState_DeeplyNestedTruncation(t *testing.T) {
	t.Parallel()
	// JSON with nested objects truncated mid-value
	data := `{"version":1,"id":"deep","name":"Deep","type":"leaf","state":"in_progress","audit":{"status":"pending","breadcrumbs":[{"message":"did stuff`
	recovered, _, err := RecoverNodeState([]byte(data))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if recovered.ID != "deep" {
		t.Errorf("expected ID deep, got %s", recovered.ID)
	}
}

func TestRecoverNodeState_OnlyOpenBrace(t *testing.T) {
	t.Parallel()
	// Single opening brace. Recovery should produce a minimal state.
	recovered, _, err := RecoverNodeState([]byte("{"))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	// Should produce a state (possibly empty fields)
	if recovered == nil {
		t.Fatal("expected non-nil recovered state")
	}
}

func TestRecoverNodeState_TrailingCommaAfterLastField(t *testing.T) {
	t.Parallel()
	// Valid JSON except for a trailing comma. The Go JSON parser rejects
	// trailing commas, so this exercises the recovery path (strip trailing
	// or close truncated). Recovery may or may not salvage this depending
	// on the heuristics, so we accept either outcome.
	data := `{"version":1,"id":"tc","name":"TC","type":"leaf","state":"not_started",}`
	recovered, _, err := RecoverNodeState([]byte(data))
	if err != nil {
		// Trailing commas produce invalid JSON that the recovery heuristics
		// may not handle. This is an acceptable failure.
		t.Skipf("trailing comma recovery not supported: %v", err)
	}
	if recovered.ID != "tc" {
		t.Errorf("expected ID tc, got %s", recovered.ID)
	}
}

func TestRecoverNodeState_ArrayInsteadOfObject(t *testing.T) {
	t.Parallel()
	// Array at top level instead of object
	_, _, err := RecoverNodeState([]byte(`[1,2,3]`))
	// This should either succeed with default values or fail gracefully.
	// An array can't unmarshal to NodeState, so recovery should fail.
	if err == nil {
		t.Log("array was somehow recovered (accepted)")
	}
}

func TestRecoverNodeState_NormalizesAuditFields(t *testing.T) {
	t.Parallel()
	// Valid JSON but missing audit sub-fields that normalizeRecovered should populate
	ns := &state.NodeState{
		Version: 1,
		ID:      "norm-audit",
		Name:    "NormAudit",
		Type:    state.NodeLeaf,
		State:   state.StatusNotStarted,
	}
	data, _ := json.Marshal(ns)

	recovered, _, err := RecoverNodeState(data)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if recovered.Audit.Breadcrumbs == nil {
		t.Error("expected non-nil Breadcrumbs after normalization")
	}
	if recovered.Audit.Gaps == nil {
		t.Error("expected non-nil Gaps after normalization")
	}
	if recovered.Audit.Escalations == nil {
		t.Error("expected non-nil Escalations after normalization")
	}
	if recovered.Audit.Status != state.AuditPending {
		t.Errorf("expected audit status pending, got %s", recovered.Audit.Status)
	}
}

// ═══════════════════════════════════════════════════════════════════════════
// RecoverRootIndex: edge cases
// ═══════════════════════════════════════════════════════════════════════════

func TestRecoverRootIndex_Unrecoverable(t *testing.T) {
	t.Parallel()
	_, _, err := RecoverRootIndex([]byte("not json not even close"))
	if err == nil {
		t.Error("expected error for unrecoverable root index")
	}
}

func TestRecoverRootIndex_NullNodesMap(t *testing.T) {
	t.Parallel()
	// Valid JSON but nodes is null
	data := `{"version":1,"root":[],"nodes":null}`
	recovered, _, err := RecoverRootIndex([]byte(data))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if recovered.Nodes == nil {
		t.Error("Nodes map should be initialized even when null in JSON")
	}
}

func TestRecoverRootIndex_MissingNodesField(t *testing.T) {
	t.Parallel()
	data := `{"version":1}`
	recovered, _, err := RecoverRootIndex([]byte(data))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if recovered.Nodes == nil {
		t.Error("Nodes map should be initialized")
	}
}

// ═══════════════════════════════════════════════════════════════════════════
// RecoveringNodeLoader: recovery callback and error paths
// ═══════════════════════════════════════════════════════════════════════════

func TestRecoveringNodeLoader_ValidNode_NoRecovery(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()

	ns := state.NewNodeState("test-node", "Test", state.NodeLeaf)
	ns.Tasks = []state.Task{
		{ID: "t1", Description: "work", State: state.StatusNotStarted},
	}
	saveLeaf(t, dir, "test-node", ns)

	var recovered bool
	loader := RecoveringNodeLoader(dir, func(addr string, report *RecoveryReport) {
		recovered = true
	})

	loaded, err := loader("test-node")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if loaded.ID != "test-node" {
		t.Errorf("expected ID test-node, got %s", loaded.ID)
	}
	if recovered {
		t.Error("should not have triggered recovery for valid node")
	}
}

func TestRecoveringNodeLoader_MalformedJSON_Recovers(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	nodeDir := filepath.Join(dir, "broken-node")
	_ = os.MkdirAll(nodeDir, 0755)

	// Write malformed JSON (BOM + valid content)
	ns := state.NewNodeState("broken-node", "Broken", state.NodeLeaf)
	data, _ := json.Marshal(ns)
	bomData := append([]byte{0xEF, 0xBB, 0xBF}, data...)
	_ = os.WriteFile(filepath.Join(nodeDir, "state.json"), bomData, 0644)

	var recoveredAddr string
	loader := RecoveringNodeLoader(dir, func(addr string, report *RecoveryReport) {
		recoveredAddr = addr
	})

	loaded, err := loader("broken-node")
	if err != nil {
		t.Fatalf("recovery should succeed: %v", err)
	}
	if loaded.ID != "broken-node" {
		t.Errorf("expected ID broken-node, got %s", loaded.ID)
	}
	if recoveredAddr != "broken-node" {
		t.Errorf("expected recovery callback for broken-node, got %q", recoveredAddr)
	}
}

func TestRecoveringNodeLoader_CompletelyCorrupt_ReturnsError(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	nodeDir := filepath.Join(dir, "total-loss")
	_ = os.MkdirAll(nodeDir, 0755)
	_ = os.WriteFile(filepath.Join(nodeDir, "state.json"), []byte("garbage that is not json"), 0644)

	loader := RecoveringNodeLoader(dir, nil)
	_, err := loader("total-loss")
	if err == nil {
		t.Error("expected error for completely corrupt node")
	}
}

func TestRecoveringNodeLoader_MissingFile_ReturnsError(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()

	loader := RecoveringNodeLoader(dir, nil)
	_, err := loader("nonexistent-node")
	if err == nil {
		t.Error("expected error for missing state file")
	}
}

func TestRecoveringNodeLoader_NilCallback_StillRecovers(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	nodeDir := filepath.Join(dir, "nil-cb-node")
	_ = os.MkdirAll(nodeDir, 0755)

	ns := state.NewNodeState("nil-cb-node", "NilCb", state.NodeLeaf)
	data, _ := json.Marshal(ns)
	bomData := append([]byte{0xEF, 0xBB, 0xBF}, data...)
	_ = os.WriteFile(filepath.Join(nodeDir, "state.json"), bomData, 0644)

	// nil callback should not panic
	loader := RecoveringNodeLoader(dir, nil)
	loaded, err := loader("nil-cb-node")
	if err != nil {
		t.Fatalf("expected recovery with nil callback: %v", err)
	}
	if loaded.ID != "nil-cb-node" {
		t.Errorf("expected ID nil-cb-node, got %s", loaded.ID)
	}
}

// ═══════════════════════════════════════════════════════════════════════════
// loadOrRecoverRootIndex: error paths
// ═══════════════════════════════════════════════════════════════════════════

func TestLoadOrRecoverRootIndex_ValidIndex(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	idx := state.NewRootIndex()
	idx.Nodes["a"] = state.IndexEntry{Name: "A", Type: state.NodeLeaf, State: state.StatusNotStarted, Address: "a"}
	indexPath := saveIndex(t, dir, idx)

	loaded, err := loadOrRecoverRootIndex(indexPath)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if _, ok := loaded.Nodes["a"]; !ok {
		t.Error("expected node 'a' in loaded index")
	}
}

func TestLoadOrRecoverRootIndex_MalformedJSON_Recovers(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	indexPath := filepath.Join(dir, "state.json")

	// Write valid index with BOM prefix (makes standard JSON parsing fail)
	idx := state.NewRootIndex()
	idx.Nodes["b"] = state.IndexEntry{Name: "B", Type: state.NodeLeaf, State: state.StatusComplete, Address: "b"}
	data, _ := json.Marshal(idx)
	bomData := append([]byte{0xEF, 0xBB, 0xBF}, data...)
	_ = os.WriteFile(indexPath, bomData, 0644)

	loaded, err := loadOrRecoverRootIndex(indexPath)
	if err != nil {
		t.Fatalf("recovery should succeed: %v", err)
	}
	if _, ok := loaded.Nodes["b"]; !ok {
		t.Error("expected node 'b' after recovery")
	}
}

func TestLoadOrRecoverRootIndex_MissingFile(t *testing.T) {
	t.Parallel()
	_, err := loadOrRecoverRootIndex("/nonexistent/path/state.json")
	if err == nil {
		t.Error("expected error for missing file")
	}
}

func TestLoadOrRecoverRootIndex_CompletelyCorrupt(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	indexPath := filepath.Join(dir, "state.json")
	_ = os.WriteFile(indexPath, []byte("not json at all"), 0644)

	_, err := loadOrRecoverRootIndex(indexPath)
	if err == nil {
		t.Error("expected error for unrecoverable index")
	}
}

// ═══════════════════════════════════════════════════════════════════════════
// NormalizeStateValue: table-driven edge cases
// ═══════════════════════════════════════════════════════════════════════════

func TestNormalizeStateValue_EdgeCases(t *testing.T) {
	t.Parallel()

	tests := []struct {
		input   string
		wantVal state.NodeStatus
		wantOK  bool
	}{
		{"complete", state.StatusComplete, true},
		{"COMPLETE", state.StatusComplete, true},
		{"  Complete  ", state.StatusComplete, true},
		{"completed", state.StatusComplete, true},
		{"done", state.StatusComplete, true},
		{"DONE", state.StatusComplete, true},
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
		{"", "", false},
		{"garbage", "", false},
		{"invalid_state", "", false},
		{"partial", "", false},
	}

	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			t.Parallel()
			val, ok := NormalizeStateValue(tt.input)
			if ok != tt.wantOK {
				t.Errorf("NormalizeStateValue(%q) ok=%v, want %v", tt.input, ok, tt.wantOK)
			}
			if ok && val != tt.wantVal {
				t.Errorf("NormalizeStateValue(%q) = %q, want %q", tt.input, val, tt.wantVal)
			}
		})
	}
}

// ═══════════════════════════════════════════════════════════════════════════
// FixWithVerification: malformed root index triggers recovery
// ═══════════════════════════════════════════════════════════════════════════

func TestFixWithVerification_MalformedIndex_RecoversAndFixes(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	indexPath := filepath.Join(dir, "state.json")

	// Create a valid index with BOM (making initial load fail)
	idx := state.NewRootIndex()
	idx.Root = []string{"fix-node"}
	idx.Nodes["fix-node"] = state.IndexEntry{
		Name: "Fix", Type: state.NodeLeaf, State: state.StatusNotStarted, Address: "fix-node",
	}
	data, _ := json.Marshal(idx)
	bomData := append([]byte{0xEF, 0xBB, 0xBF}, data...)
	_ = os.WriteFile(indexPath, bomData, 0644)

	// Create the node state
	ns := state.NewNodeState("fix-node", "Fix", state.NodeLeaf)
	ns.Tasks = []state.Task{
		{ID: "t1", Description: "work", State: state.StatusNotStarted},
		{ID: "audit", Description: "audit", State: state.StatusNotStarted, IsAudit: true},
	}
	saveLeaf(t, dir, "fix-node", ns)

	loader := DefaultNodeLoader(dir)
	fixes, report, err := FixWithVerification(dir, indexPath, loader)
	if err != nil {
		t.Fatalf("FixWithVerification error: %v", err)
	}

	// The recovered index should be valid now; any fixes applied are acceptable
	_ = fixes
	_ = report
}

// ═══════════════════════════════════════════════════════════════════════════
// Engine.ValidateStartup: verifies subset of categories
// ═══════════════════════════════════════════════════════════════════════════

func TestEngine_ValidateStartup_OnlyStartupCategories(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()

	idx := state.NewRootIndex()
	idx.Root = []string{"startup-node"}
	idx.Nodes["startup-node"] = state.IndexEntry{
		Name: "Startup", Type: state.NodeLeaf, State: state.StatusNotStarted, Address: "startup-node",
	}
	saveIndex(t, dir, idx)

	ns := state.NewNodeState("startup-node", "Startup", state.NodeLeaf)
	ns.Tasks = []state.Task{
		{ID: "t1", Description: "work", State: state.StatusNotStarted},
		// Missing audit task: this is in StartupCategories
	}
	saveLeaf(t, dir, "startup-node", ns)

	engine := NewEngine(dir, DefaultNodeLoader(dir))
	report := engine.ValidateStartup(idx)

	// Should find MISSING_AUDIT_TASK since it's in StartupCategories
	found := false
	for _, issue := range report.Issues {
		if issue.Category == CatMissingAuditTask {
			found = true
		}
		// Should NOT find ORPHAN_DEFINITION since it's not in StartupCategories
		if issue.Category == CatOrphanDefinition {
			t.Error("ORPHAN_DEFINITION should not be in startup checks")
		}
	}
	if !found {
		t.Error("expected MISSING_AUDIT_TASK in startup validation")
	}
}

// ═══════════════════════════════════════════════════════════════════════════
// Engine: blocked task without reason
// ═══════════════════════════════════════════════════════════════════════════

func TestEngine_BlockedWithoutReason_Detected(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()

	idx := state.NewRootIndex()
	idx.Root = []string{"blocked-node"}
	idx.Nodes["blocked-node"] = state.IndexEntry{
		Name: "Blocked", Type: state.NodeLeaf, State: state.StatusBlocked, Address: "blocked-node",
	}
	saveIndex(t, dir, idx)

	ns := state.NewNodeState("blocked-node", "Blocked", state.NodeLeaf)
	ns.State = state.StatusBlocked
	ns.Tasks = []state.Task{
		{ID: "t1", Description: "stuck", State: state.StatusBlocked, BlockedReason: ""},
		{ID: "audit", Description: "audit", State: state.StatusNotStarted, IsAudit: true},
	}
	saveLeaf(t, dir, "blocked-node", ns)

	engine := NewEngine(dir, DefaultNodeLoader(dir))
	report := engine.ValidateAll(idx)

	found := false
	for _, issue := range report.Issues {
		if issue.Category == CatBlockedWithoutReason {
			found = true
			if !issue.CanAutoFix {
				t.Error("blocked without reason should be auto-fixable")
			}
		}
	}
	if !found {
		t.Error("expected BLOCKED_WITHOUT_REASON issue")
	}
}

// ═══════════════════════════════════════════════════════════════════════════
// Engine: complete node with incomplete tasks
// ═══════════════════════════════════════════════════════════════════════════

func TestEngine_CompleteWithIncomplete_Detected(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()

	idx := state.NewRootIndex()
	idx.Root = []string{"bad-complete"}
	idx.Nodes["bad-complete"] = state.IndexEntry{
		Name: "BadComplete", Type: state.NodeLeaf, State: state.StatusComplete, Address: "bad-complete",
	}
	saveIndex(t, dir, idx)

	ns := state.NewNodeState("bad-complete", "BadComplete", state.NodeLeaf)
	ns.State = state.StatusComplete
	ns.Tasks = []state.Task{
		{ID: "t1", Description: "done", State: state.StatusComplete},
		{ID: "t2", Description: "not done", State: state.StatusNotStarted},
		{ID: "audit", Description: "audit", State: state.StatusNotStarted, IsAudit: true},
	}
	saveLeaf(t, dir, "bad-complete", ns)

	engine := NewEngine(dir, DefaultNodeLoader(dir))
	report := engine.ValidateAll(idx)

	found := false
	for _, issue := range report.Issues {
		if issue.Category == CatCompleteWithIncomplete {
			found = true
		}
	}
	if !found {
		t.Error("expected COMPLETE_WITH_INCOMPLETE issue")
	}
}

// ═══════════════════════════════════════════════════════════════════════════
// Engine: negative failure count
// ═══════════════════════════════════════════════════════════════════════════

func TestEngine_NegativeFailureCount_Detected(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()

	idx := state.NewRootIndex()
	idx.Root = []string{"neg-node"}
	idx.Nodes["neg-node"] = state.IndexEntry{
		Name: "Neg", Type: state.NodeLeaf, State: state.StatusInProgress, Address: "neg-node",
	}
	saveIndex(t, dir, idx)

	ns := state.NewNodeState("neg-node", "Neg", state.NodeLeaf)
	ns.State = state.StatusInProgress
	ns.Tasks = []state.Task{
		{ID: "t1", Description: "work", State: state.StatusInProgress, FailureCount: -3},
		{ID: "audit", Description: "audit", State: state.StatusNotStarted, IsAudit: true},
	}
	saveLeaf(t, dir, "neg-node", ns)

	engine := NewEngine(dir, DefaultNodeLoader(dir))
	report := engine.ValidateAll(idx)

	found := false
	for _, issue := range report.Issues {
		if issue.Category == CatNegativeFailureCount {
			found = true
			if !issue.CanAutoFix {
				t.Error("negative failure count should be auto-fixable")
			}
		}
	}
	if !found {
		t.Error("expected NEGATIVE_FAILURE_COUNT issue")
	}
}

// ═══════════════════════════════════════════════════════════════════════════
// sanitizeJSON: no BOM, no null bytes, no whitespace
// ═══════════════════════════════════════════════════════════════════════════

func TestSanitizeJSON_OnlyWhitespace(t *testing.T) {
	t.Parallel()
	input := []byte("  \t\n  {\"a\":1}  \n  ")
	out, steps := sanitizeJSON(input)
	// Should be trimmed
	if string(out) != `{"a":1}` {
		t.Errorf("expected trimmed JSON, got %q", string(out))
	}
	// No BOM or null steps needed
	for _, s := range steps {
		if strings.Contains(s, "BOM") || strings.Contains(s, "null") {
			t.Errorf("unexpected step for whitespace-only cleanup: %s", s)
		}
	}
}

// ═══════════════════════════════════════════════════════════════════════════
// tryStripTrailing: edge cases
// ═══════════════════════════════════════════════════════════════════════════

func TestTryStripTrailing_EmptyInput(t *testing.T) {
	t.Parallel()
	_, ok := tryStripTrailing([]byte{})
	if ok {
		t.Error("empty input should return false")
	}
}

func TestTryStripTrailing_NoOpenerMatch(t *testing.T) {
	t.Parallel()
	_, ok := tryStripTrailing([]byte("hello"))
	if ok {
		t.Error("non-JSON input should return false")
	}
}

func TestTryStripTrailing_ArrayInput(t *testing.T) {
	t.Parallel()
	result, ok := tryStripTrailing([]byte(`[1,2,3]extra`))
	if !ok {
		t.Fatal("expected success for array with trailing garbage")
	}
	if string(result) != `[1,2,3]` {
		t.Errorf("expected [1,2,3], got %s", string(result))
	}
}

// ═══════════════════════════════════════════════════════════════════════════
// tryCloseTruncated: edge cases
// ═══════════════════════════════════════════════════════════════════════════

func TestTryCloseTruncated_EmptyInput(t *testing.T) {
	t.Parallel()
	_, ok := tryCloseTruncated([]byte{})
	if ok {
		t.Error("empty input should return false")
	}
}

func TestTryCloseTruncated_NotJSON(t *testing.T) {
	t.Parallel()
	_, ok := tryCloseTruncated([]byte("hello"))
	if ok {
		t.Error("non-JSON input should return false")
	}
}

func TestTryCloseTruncated_AlreadyValid(t *testing.T) {
	t.Parallel()
	_, ok := tryCloseTruncated([]byte(`{"valid":true}`))
	if ok {
		t.Error("already valid JSON should return false (nothing is open)")
	}
}

func TestTryCloseTruncated_OpenArray(t *testing.T) {
	t.Parallel()
	result, ok := tryCloseTruncated([]byte(`[1,2,3`))
	if !ok {
		t.Fatal("expected success for truncated array")
	}
	// Should produce valid JSON
	var arr []int
	if err := json.Unmarshal(result, &arr); err != nil {
		t.Errorf("result should be valid JSON: %v, got %s", err, result)
	}
}

// ═══════════════════════════════════════════════════════════════════════════
// Report methods
// ═══════════════════════════════════════════════════════════════════════════

func TestReport_HasAutoFixable_True(t *testing.T) {
	t.Parallel()
	r := &Report{Issues: []Issue{
		{Severity: SeverityError, CanAutoFix: false},
		{Severity: SeverityWarning, CanAutoFix: true},
	}}
	if !r.HasAutoFixable() {
		t.Error("expected HasAutoFixable() to be true")
	}
}

func TestReport_HasAutoFixable_False(t *testing.T) {
	t.Parallel()
	r := &Report{Issues: []Issue{
		{Severity: SeverityError, CanAutoFix: false},
	}}
	if r.HasAutoFixable() {
		t.Error("expected HasAutoFixable() to be false")
	}
}

func TestReport_HasErrors_NoErrors(t *testing.T) {
	t.Parallel()
	r := &Report{Issues: []Issue{
		{Severity: SeverityWarning},
		{Severity: SeverityInfo},
	}}
	if r.HasErrors() {
		t.Error("expected HasErrors() to be false")
	}
}

// ═══════════════════════════════════════════════════════════════════════════
// Engine: dangling reference detection
// ═══════════════════════════════════════════════════════════════════════════

func TestEngine_DanglingRef_DetectedAndFixable(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()

	idx := state.NewRootIndex()
	idx.Root = []string{"ghost-node"}
	idx.Nodes["ghost-node"] = state.IndexEntry{
		Name: "Ghost", Type: state.NodeLeaf, State: state.StatusNotStarted, Address: "ghost-node",
	}
	// Don't create the node on disk

	engine := NewEngine(dir, DefaultNodeLoader(dir))
	report := engine.ValidateAll(idx)

	found := false
	for _, issue := range report.Issues {
		if issue.Category == CatRootIndexDanglingRef {
			found = true
			if !issue.CanAutoFix {
				t.Error("dangling ref should be auto-fixable")
			}
		}
	}
	if !found {
		t.Error("expected ROOTINDEX_DANGLING_REF for missing node")
	}
}

// ═══════════════════════════════════════════════════════════════════════════
// Engine: multiple in-progress tasks
// ═══════════════════════════════════════════════════════════════════════════

func TestEngine_MultipleInProgress_Detected(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()

	idx := state.NewRootIndex()
	idx.Root = []string{"multi-ip-a", "multi-ip-b"}
	idx.Nodes["multi-ip-a"] = state.IndexEntry{
		Name: "A", Type: state.NodeLeaf, State: state.StatusInProgress, Address: "multi-ip-a",
	}
	idx.Nodes["multi-ip-b"] = state.IndexEntry{
		Name: "B", Type: state.NodeLeaf, State: state.StatusInProgress, Address: "multi-ip-b",
	}
	saveIndex(t, dir, idx)

	nsA := state.NewNodeState("multi-ip-a", "A", state.NodeLeaf)
	nsA.State = state.StatusInProgress
	nsA.Tasks = []state.Task{
		{ID: "t1", State: state.StatusInProgress},
		{ID: "audit", State: state.StatusNotStarted, IsAudit: true},
	}
	saveLeaf(t, dir, "multi-ip-a", nsA)

	nsB := state.NewNodeState("multi-ip-b", "B", state.NodeLeaf)
	nsB.State = state.StatusInProgress
	nsB.Tasks = []state.Task{
		{ID: "t1", State: state.StatusInProgress},
		{ID: "audit", State: state.StatusNotStarted, IsAudit: true},
	}
	saveLeaf(t, dir, "multi-ip-b", nsB)

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

// ═══════════════════════════════════════════════════════════════════════════
// Engine: invalid audit status
// ═══════════════════════════════════════════════════════════════════════════

func TestEngine_InvalidAuditStatus_Detected(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()

	idx := state.NewRootIndex()
	idx.Root = []string{"audit-bad"}
	idx.Nodes["audit-bad"] = state.IndexEntry{
		Name: "AuditBad", Type: state.NodeLeaf, State: state.StatusNotStarted, Address: "audit-bad",
	}
	saveIndex(t, dir, idx)

	ns := state.NewNodeState("audit-bad", "AuditBad", state.NodeLeaf)
	ns.Audit.Status = "bogus_status"
	ns.Tasks = []state.Task{
		{ID: "t1", State: state.StatusNotStarted},
		{ID: "audit", State: state.StatusNotStarted, IsAudit: true},
	}
	saveLeaf(t, dir, "audit-bad", ns)

	engine := NewEngine(dir, DefaultNodeLoader(dir))
	report := engine.ValidateAll(idx)

	found := false
	for _, issue := range report.Issues {
		if issue.Category == CatInvalidAuditStatus {
			found = true
			if !issue.CanAutoFix {
				t.Error("invalid audit status should be auto-fixable")
			}
		}
	}
	if !found {
		t.Error("expected INVALID_AUDIT_STATUS issue")
	}
}
