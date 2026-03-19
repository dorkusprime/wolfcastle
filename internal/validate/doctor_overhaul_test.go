package validate

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/dorkusprime/wolfcastle/internal/state"
)

// nodeAddrFromTaskRef
// ═══════════════════════════════════════════════════════════════════════════

func TestNodeAddrFromTaskRef(t *testing.T) {
	t.Parallel()
	tests := []struct {
		ref  string
		want string
	}{
		{"node-a/task-0001", "node-a"},
		{"parent/child/task-0001", "parent/child"},
		{"domain-repo/foundation/tier-resolution/task-0001.0002", "domain-repo/foundation/tier-resolution"},
		{"single", "single"},
	}
	for _, tt := range tests {
		if got := nodeAddrFromTaskRef(tt.ref); got != tt.want {
			t.Errorf("nodeAddrFromTaskRef(%q) = %q, want %q", tt.ref, got, tt.want)
		}
	}
}

// MULTIPLE_IN_PROGRESS detection: per-node issues, deterministic fix type
// ═══════════════════════════════════════════════════════════════════════════

func TestDetect_MultipleInProgress_PerNodeIssues(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	idx := state.NewRootIndex()

	// Two leaves, each with a task in_progress
	for _, name := range []string{"node-a", "node-b"} {
		nodeDir := filepath.Join(dir, name)
		_ = os.MkdirAll(nodeDir, 0755)
		ns := state.NewNodeState(name, name, state.NodeLeaf)
		ns.Tasks = []state.Task{
			{ID: "task-0001", State: state.StatusInProgress},
			{ID: "audit", State: state.StatusNotStarted, IsAudit: true},
		}
		_ = state.SaveNodeState(filepath.Join(nodeDir, "state.json"), ns)
		idx.Root = append(idx.Root, name)
		idx.Nodes[name] = state.IndexEntry{
			Name: name, Type: state.NodeLeaf, State: state.StatusInProgress, Address: name,
		}
	}

	engine := NewEngine(dir, DefaultNodeLoader(dir))
	report := engine.ValidateAll(idx)

	var multiIssues []Issue
	for _, issue := range report.Issues {
		if issue.Category == CatMultipleInProgress {
			multiIssues = append(multiIssues, issue)
		}
	}

	if len(multiIssues) != 2 {
		t.Fatalf("expected 2 MULTIPLE_IN_PROGRESS issues (one per node), got %d", len(multiIssues))
	}
	for _, issue := range multiIssues {
		if issue.Node == "" {
			t.Error("MULTIPLE_IN_PROGRESS issue should have Node set")
		}
		if issue.FixType != FixDeterministic {
			t.Errorf("expected FixDeterministic, got %s", issue.FixType)
		}
		if !issue.CanAutoFix {
			t.Error("expected CanAutoFix=true")
		}
	}
}

// STALE_IN_PROGRESS detection: per-node issues, deterministic fix type
// ═══════════════════════════════════════════════════════════════════════════

func TestDetect_StaleInProgress_PerNodeIssues(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	idx := state.NewRootIndex()

	nodeDir := filepath.Join(dir, "stale-node")
	_ = os.MkdirAll(nodeDir, 0755)
	ns := state.NewNodeState("stale-node", "Stale", state.NodeLeaf)
	ns.Tasks = []state.Task{
		{ID: "task-0001", State: state.StatusInProgress},
		{ID: "audit", State: state.StatusNotStarted, IsAudit: true},
	}
	_ = state.SaveNodeState(filepath.Join(nodeDir, "state.json"), ns)
	idx.Root = []string{"stale-node"}
	idx.Nodes["stale-node"] = state.IndexEntry{
		Name: "Stale", Type: state.NodeLeaf, State: state.StatusInProgress, Address: "stale-node",
	}

	// No daemon alive, no wolfcastleDir → isDaemonAlive returns false
	engine := NewEngine(dir, DefaultNodeLoader(dir))
	report := engine.ValidateAll(idx)

	var staleIssues []Issue
	for _, issue := range report.Issues {
		if issue.Category == CatStaleInProgress {
			staleIssues = append(staleIssues, issue)
		}
	}

	if len(staleIssues) != 1 {
		t.Fatalf("expected 1 STALE_IN_PROGRESS issue, got %d", len(staleIssues))
	}
	if staleIssues[0].Node != "stale-node" {
		t.Errorf("expected Node='stale-node', got %q", staleIssues[0].Node)
	}
	if staleIssues[0].FixType != FixDeterministic {
		t.Errorf("expected FixDeterministic, got %s", staleIssues[0].FixType)
	}
	if !staleIssues[0].CanAutoFix {
		t.Error("expected CanAutoFix=true")
	}
}

// Fix: STALE_IN_PROGRESS resets tasks to not_started
// ═══════════════════════════════════════════════════════════════════════════

func TestFix_StaleInProgress_ResetsToNotStarted(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	idx := state.NewRootIndex()
	indexPath := filepath.Join(dir, "..", "root_index.json")

	nodeDir := filepath.Join(dir, "stale-node")
	_ = os.MkdirAll(nodeDir, 0755)
	ns := state.NewNodeState("stale-node", "Stale", state.NodeLeaf)
	ns.State = state.StatusInProgress
	ns.Tasks = []state.Task{
		{ID: "task-0001", State: state.StatusInProgress},
		{ID: "task-0002", State: state.StatusComplete},
		{ID: "audit", State: state.StatusNotStarted, IsAudit: true},
	}
	_ = state.SaveNodeState(filepath.Join(nodeDir, "state.json"), ns)
	idx.Root = []string{"stale-node"}
	idx.Nodes["stale-node"] = state.IndexEntry{
		Name: "Stale", Type: state.NodeLeaf, State: state.StatusInProgress, Address: "stale-node",
	}
	_ = state.SaveRootIndex(indexPath, idx)

	issues := []Issue{{
		Severity:   SeverityError,
		Category:   CatStaleInProgress,
		Node:       "stale-node",
		CanAutoFix: true,
		FixType:    FixDeterministic,
	}}

	fixes, _, err := ApplyDeterministicFixes(idx, issues, dir, indexPath)
	if err != nil {
		t.Fatalf("ApplyDeterministicFixes: %v", err)
	}
	if len(fixes) != 1 {
		t.Fatalf("expected 1 fix, got %d", len(fixes))
	}

	// Verify the task was reset
	nsAfter, err := state.LoadNodeState(filepath.Join(nodeDir, "state.json"))
	if err != nil {
		t.Fatalf("loading node: %v", err)
	}
	if nsAfter.Tasks[0].State != state.StatusNotStarted {
		t.Errorf("task-0001 should be not_started, got %s", nsAfter.Tasks[0].State)
	}
	if nsAfter.Tasks[1].State != state.StatusComplete {
		t.Errorf("task-0002 should remain complete, got %s", nsAfter.Tasks[1].State)
	}
	if nsAfter.State != state.StatusNotStarted {
		t.Errorf("node state should be not_started, got %s", nsAfter.State)
	}
}

// Fix: MULTIPLE_IN_PROGRESS resets tasks to not_started
// ═══════════════════════════════════════════════════════════════════════════

func TestFix_MultipleInProgress_ResetsToNotStarted(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	idx := state.NewRootIndex()
	indexPath := filepath.Join(dir, "..", "root_index.json")

	for _, name := range []string{"node-a", "node-b"} {
		nodeDir := filepath.Join(dir, name)
		_ = os.MkdirAll(nodeDir, 0755)
		ns := state.NewNodeState(name, name, state.NodeLeaf)
		ns.State = state.StatusInProgress
		ns.Tasks = []state.Task{
			{ID: "task-0001", State: state.StatusInProgress},
		}
		_ = state.SaveNodeState(filepath.Join(nodeDir, "state.json"), ns)
		idx.Root = append(idx.Root, name)
		idx.Nodes[name] = state.IndexEntry{
			Name: name, Type: state.NodeLeaf, State: state.StatusInProgress, Address: name,
		}
	}
	_ = state.SaveRootIndex(indexPath, idx)

	issues := []Issue{
		{Severity: SeverityError, Category: CatMultipleInProgress, Node: "node-a", CanAutoFix: true, FixType: FixDeterministic},
		{Severity: SeverityError, Category: CatMultipleInProgress, Node: "node-b", CanAutoFix: true, FixType: FixDeterministic},
	}

	fixes, _, err := ApplyDeterministicFixes(idx, issues, dir, indexPath)
	if err != nil {
		t.Fatalf("ApplyDeterministicFixes: %v", err)
	}
	if len(fixes) != 2 {
		t.Fatalf("expected 2 fixes, got %d", len(fixes))
	}

	for _, name := range []string{"node-a", "node-b"} {
		nsAfter, _ := state.LoadNodeState(filepath.Join(dir, name, "state.json"))
		if nsAfter.Tasks[0].State != state.StatusNotStarted {
			t.Errorf("%s task should be not_started, got %s", name, nsAfter.Tasks[0].State)
		}
		if nsAfter.State != state.StatusNotStarted {
			t.Errorf("%s state should be not_started, got %s", name, nsAfter.State)
		}
	}

	// Verify index was updated
	for _, name := range []string{"node-a", "node-b"} {
		if idx.Nodes[name].State != state.StatusNotStarted {
			t.Errorf("index %s should be not_started, got %s", name, idx.Nodes[name].State)
		}
	}
}

// INVALID_AUDIT_SCOPE: not flagged when audit is pending
// ═══════════════════════════════════════════════════════════════════════════

func TestAuditScope_NotFlaggedWhenPending(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	idx := state.NewRootIndex()

	nodeDir := filepath.Join(dir, "fresh-node")
	_ = os.MkdirAll(nodeDir, 0755)
	ns := state.NewNodeState("fresh-node", "Fresh", state.NodeLeaf)
	ns.Audit.Scope = &state.AuditScope{Description: ""} // empty, but pending
	ns.Audit.Status = state.AuditPending
	ns.Tasks = []state.Task{
		{ID: "task-0001", State: state.StatusNotStarted},
		{ID: "audit", State: state.StatusNotStarted, IsAudit: true},
	}
	_ = state.SaveNodeState(filepath.Join(nodeDir, "state.json"), ns)
	idx.Root = []string{"fresh-node"}
	idx.Nodes["fresh-node"] = state.IndexEntry{
		Name: "Fresh", Type: state.NodeLeaf, State: state.StatusNotStarted, Address: "fresh-node",
	}

	engine := NewEngine(dir, DefaultNodeLoader(dir))
	report := engine.ValidateAll(idx)

	for _, issue := range report.Issues {
		if issue.Category == CatInvalidAuditScope {
			t.Error("INVALID_AUDIT_SCOPE should not fire for pending audits")
		}
	}
}

func TestAuditScope_FlaggedWhenContentWithNoDescription(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	idx := state.NewRootIndex()

	nodeDir := filepath.Join(dir, "bad-scope")
	_ = os.MkdirAll(nodeDir, 0755)
	ns := state.NewNodeState("bad-scope", "Bad", state.NodeLeaf)
	ns.Audit.Scope = &state.AuditScope{
		Description: "",
		Criteria:    []string{"builds", "tests pass"},
	}
	ns.Audit.Status = state.AuditFailed // completed with content but no description
	ns.Tasks = []state.Task{
		{ID: "task-0001", State: state.StatusComplete},
		{ID: "audit", State: state.StatusComplete, IsAudit: true},
	}
	_ = state.SaveNodeState(filepath.Join(nodeDir, "state.json"), ns)
	idx.Root = []string{"bad-scope"}
	idx.Nodes["bad-scope"] = state.IndexEntry{
		Name: "Bad", Type: state.NodeLeaf, State: state.StatusComplete, Address: "bad-scope",
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
		t.Error("INVALID_AUDIT_SCOPE should fire when scope has criteria but no description after audit completed")
	}
}

func TestAuditScope_NotFlaggedWhenEmptyDefault(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	idx := state.NewRootIndex()

	// Default scope from NewNodeState: empty description, empty slices, any status
	nodeDir := filepath.Join(dir, "default-node")
	_ = os.MkdirAll(nodeDir, 0755)
	ns := state.NewNodeState("default-node", "Default", state.NodeLeaf)
	// ns.Audit.Scope is already set by NewNodeState with empty fields
	ns.Audit.Status = state.AuditFailed // even after audit completed
	ns.Tasks = []state.Task{
		{ID: "task-0001", State: state.StatusComplete},
		{ID: "audit", State: state.StatusComplete, IsAudit: true},
	}
	_ = state.SaveNodeState(filepath.Join(nodeDir, "state.json"), ns)
	idx.Root = []string{"default-node"}
	idx.Nodes["default-node"] = state.IndexEntry{
		Name: "Default", Type: state.NodeLeaf, State: state.StatusComplete, Address: "default-node",
	}

	engine := NewEngine(dir, DefaultNodeLoader(dir))
	report := engine.ValidateAll(idx)

	for _, issue := range report.Issues {
		if issue.Category == CatInvalidAuditScope {
			t.Error("INVALID_AUDIT_SCOPE should not fire for default empty scope")
		}
	}
}

// Orchestrator in-progress tracking (not just leaves)
// ═══════════════════════════════════════════════════════════════════════════

func TestDetect_OrchestratorInProgress_Tracked(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	idx := state.NewRootIndex()

	// Orchestrator with an audit task in_progress
	nodeDir := filepath.Join(dir, "orch")
	_ = os.MkdirAll(nodeDir, 0755)
	ns := state.NewNodeState("orch", "Orchestrator", state.NodeOrchestrator)
	ns.Tasks = []state.Task{
		{ID: "audit", State: state.StatusInProgress, IsAudit: true},
	}
	_ = state.SaveNodeState(filepath.Join(nodeDir, "state.json"), ns)
	idx.Root = []string{"orch"}
	idx.Nodes["orch"] = state.IndexEntry{
		Name: "Orchestrator", Type: state.NodeOrchestrator, State: state.StatusInProgress, Address: "orch",
	}

	engine := NewEngine(dir, DefaultNodeLoader(dir))
	report := engine.ValidateAll(idx)

	// With no daemon alive and an in_progress task, should get STALE_IN_PROGRESS
	found := false
	for _, issue := range report.Issues {
		if issue.Category == CatStaleInProgress && issue.Node == "orch" {
			found = true
		}
	}
	if !found {
		t.Error("orchestrator in_progress task should be detected as STALE_IN_PROGRESS")
	}
}
