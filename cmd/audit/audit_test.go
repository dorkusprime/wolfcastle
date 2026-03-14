package audit

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	"github.com/dorkusprime/wolfcastle/cmd/cmdutil"
	"github.com/dorkusprime/wolfcastle/internal/clock"
	"github.com/dorkusprime/wolfcastle/internal/config"

	"github.com/dorkusprime/wolfcastle/internal/state"
	"github.com/dorkusprime/wolfcastle/internal/tree"
	"github.com/spf13/cobra"
)

type testEnv struct {
	WolfcastleDir string
	ProjectsDir   string
	App           *cmdutil.App
	RootCmd       *cobra.Command
}

func newTestEnv(t *testing.T) *testEnv {
	t.Helper()
	tmp := t.TempDir()
	wcDir := filepath.Join(tmp, ".wolfcastle")
	_ = os.MkdirAll(wcDir, 0755)

	cfg := config.Defaults()
	cfg.Identity = &config.IdentityConfig{User: "test", Machine: "dev"}

	ns := "test-dev"
	projDir := filepath.Join(wcDir, "projects", ns)
	_ = os.MkdirAll(projDir, 0755)

	idx := state.NewRootIndex()
	saveJSON(t, filepath.Join(projDir, "state.json"), idx)

	resolver := &tree.Resolver{WolfcastleDir: wcDir, Namespace: ns}
	testApp := &cmdutil.App{
		WolfcastleDir: wcDir,
		Cfg:           cfg,
		Resolver:      resolver,
		Clock:         clock.New(),
	}

	rootCmd := &cobra.Command{Use: "wolfcastle"}
	rootCmd.AddGroup(
		&cobra.Group{ID: "lifecycle", Title: "Lifecycle:"},
		&cobra.Group{ID: "work", Title: "Work Management:"},
		&cobra.Group{ID: "audit", Title: "Auditing:"},
		&cobra.Group{ID: "docs", Title: "Documentation:"},
		&cobra.Group{ID: "diagnostics", Title: "Diagnostics:"},
		&cobra.Group{ID: "integration", Title: "Integration:"},
	)
	Register(testApp, rootCmd)

	return &testEnv{
		WolfcastleDir: wcDir,
		ProjectsDir:   projDir,
		App:           testApp,
		RootCmd:       rootCmd,
	}
}

func saveJSON(t *testing.T, path string, v any) {
	t.Helper()
	data, _ := json.MarshalIndent(v, "", "  ")
	_ = os.MkdirAll(filepath.Dir(path), 0755)
	_ = os.WriteFile(path, data, 0644)
}

func createLeafNode(t *testing.T, env *testEnv, addr, name string) {
	t.Helper()
	parsed, _ := tree.ParseAddress(addr)
	nodeDir := filepath.Join(env.ProjectsDir, filepath.Join(parsed.Parts...))
	_ = os.MkdirAll(nodeDir, 0755)

	ns := state.NewNodeState(parsed.Leaf(), name, state.NodeLeaf)
	ns.Tasks = []state.Task{
		{ID: "audit", Description: "Audit", State: state.StatusNotStarted, IsAudit: true},
	}
	saveJSON(t, filepath.Join(nodeDir, "state.json"), ns)

	idx, _ := state.LoadRootIndex(filepath.Join(env.ProjectsDir, "state.json"))
	idx.Nodes[addr] = state.IndexEntry{
		Name:     name,
		Type:     state.NodeLeaf,
		State:    state.StatusNotStarted,
		Address:  addr,
		Children: []string{},
	}
	_ = state.SaveRootIndex(filepath.Join(env.ProjectsDir, "state.json"), idx)
}

func createOrchestratorWithChild(t *testing.T, env *testEnv, parentAddr, childAddr string) {
	t.Helper()
	// Create parent
	parentParsed, _ := tree.ParseAddress(parentAddr)
	parentDir := filepath.Join(env.ProjectsDir, filepath.Join(parentParsed.Parts...))
	_ = os.MkdirAll(parentDir, 0755)

	parentNs := state.NewNodeState(parentParsed.Leaf(), parentAddr, state.NodeOrchestrator)
	saveJSON(t, filepath.Join(parentDir, "state.json"), parentNs)

	idx, _ := state.LoadRootIndex(filepath.Join(env.ProjectsDir, "state.json"))
	idx.Nodes[parentAddr] = state.IndexEntry{
		Name:     parentAddr,
		Type:     state.NodeOrchestrator,
		State:    state.StatusNotStarted,
		Address:  parentAddr,
		Children: []string{childAddr},
	}

	// Create child
	childParsed, _ := tree.ParseAddress(childAddr)
	childDir := filepath.Join(env.ProjectsDir, filepath.Join(childParsed.Parts...))
	_ = os.MkdirAll(childDir, 0755)

	childNs := state.NewNodeState(childParsed.Leaf(), childAddr, state.NodeLeaf)
	saveJSON(t, filepath.Join(childDir, "state.json"), childNs)

	idx.Nodes[childAddr] = state.IndexEntry{
		Name:     childAddr,
		Type:     state.NodeLeaf,
		State:    state.StatusNotStarted,
		Address:  childAddr,
		Parent:   parentAddr,
		Children: []string{},
	}

	_ = state.SaveRootIndex(filepath.Join(env.ProjectsDir, "state.json"), idx)
}

func loadNodeState(t *testing.T, env *testEnv, addr string) *state.NodeState {
	t.Helper()
	parsed, _ := tree.ParseAddress(addr)
	statePath := filepath.Join(env.ProjectsDir, filepath.Join(parsed.Parts...), "state.json")
	ns, err := state.LoadNodeState(statePath)
	if err != nil {
		t.Fatalf("loading node state for %s: %v", addr, err)
	}
	return ns
}

// ---------------------------------------------------------------------------
// audit breadcrumb
// ---------------------------------------------------------------------------

func TestBreadcrumb_Success(t *testing.T) {
	env := newTestEnv(t)
	createLeafNode(t, env, "my-project", "My Project")

	env.RootCmd.SetArgs([]string{"audit", "breadcrumb", "--node", "my-project", "refactored auth module"})
	if err := env.RootCmd.Execute(); err != nil {
		t.Fatalf("breadcrumb failed: %v", err)
	}

	ns := loadNodeState(t, env, "my-project")
	if len(ns.Audit.Breadcrumbs) != 1 {
		t.Fatalf("expected 1 breadcrumb, got %d", len(ns.Audit.Breadcrumbs))
	}
	if ns.Audit.Breadcrumbs[0].Text != "refactored auth module" {
		t.Errorf("unexpected breadcrumb text: %s", ns.Audit.Breadcrumbs[0].Text)
	}
}

func TestBreadcrumb_EmptyText(t *testing.T) {
	env := newTestEnv(t)
	createLeafNode(t, env, "my-project", "My Project")

	env.RootCmd.SetArgs([]string{"audit", "breadcrumb", "--node", "my-project", "   "})
	err := env.RootCmd.Execute()
	if err == nil {
		t.Error("expected error for empty text")
	}
}

func TestBreadcrumb_NoResolver(t *testing.T) {
	env := newTestEnv(t)
	env.App.Resolver = nil

	env.RootCmd.SetArgs([]string{"audit", "breadcrumb", "--node", "my-project", "text"})
	err := env.RootCmd.Execute()
	if err == nil {
		t.Error("expected error without resolver")
	}
}

// ---------------------------------------------------------------------------
// audit escalate
// ---------------------------------------------------------------------------

func TestEscalate_Success(t *testing.T) {
	env := newTestEnv(t)
	createOrchestratorWithChild(t, env, "auth", "auth/login")

	env.RootCmd.SetArgs([]string{"audit", "escalate", "--node", "auth/login", "missing error handling"})
	if err := env.RootCmd.Execute(); err != nil {
		t.Fatalf("escalate failed: %v", err)
	}

	// Check parent has escalation
	parentNs := loadNodeState(t, env, "auth")
	if len(parentNs.Audit.Escalations) != 1 {
		t.Fatalf("expected 1 escalation, got %d", len(parentNs.Audit.Escalations))
	}
	if parentNs.Audit.Escalations[0].Description != "missing error handling" {
		t.Errorf("unexpected escalation: %s", parentNs.Audit.Escalations[0].Description)
	}
	if parentNs.Audit.Escalations[0].Status != state.EscalationOpen {
		t.Errorf("expected open status, got %s", parentNs.Audit.Escalations[0].Status)
	}
}

func TestEscalate_RootNode(t *testing.T) {
	env := newTestEnv(t)
	createLeafNode(t, env, "my-project", "My Project")

	env.RootCmd.SetArgs([]string{"audit", "escalate", "--node", "my-project", "some gap"})
	err := env.RootCmd.Execute()
	if err == nil {
		t.Error("expected error when escalating from root node")
	}
}

func TestEscalate_EmptyDescription(t *testing.T) {
	env := newTestEnv(t)
	createOrchestratorWithChild(t, env, "auth", "auth/login")

	env.RootCmd.SetArgs([]string{"audit", "escalate", "--node", "auth/login", "   "})
	err := env.RootCmd.Execute()
	if err == nil {
		t.Error("expected error for empty description")
	}
}

// ---------------------------------------------------------------------------
// audit gap
// ---------------------------------------------------------------------------

func TestGap_Success(t *testing.T) {
	env := newTestEnv(t)
	createLeafNode(t, env, "my-project", "My Project")

	env.RootCmd.SetArgs([]string{"audit", "gap", "--node", "my-project", "missing error handling"})
	if err := env.RootCmd.Execute(); err != nil {
		t.Fatalf("gap failed: %v", err)
	}

	ns := loadNodeState(t, env, "my-project")
	if len(ns.Audit.Gaps) != 1 {
		t.Fatalf("expected 1 gap, got %d", len(ns.Audit.Gaps))
	}
	if ns.Audit.Gaps[0].Status != state.GapOpen {
		t.Errorf("expected open, got %s", ns.Audit.Gaps[0].Status)
	}
}

func TestGap_EmptyDescription(t *testing.T) {
	env := newTestEnv(t)
	createLeafNode(t, env, "my-project", "My Project")

	env.RootCmd.SetArgs([]string{"audit", "gap", "--node", "my-project", "  "})
	err := env.RootCmd.Execute()
	if err == nil {
		t.Error("expected error for empty description")
	}
}

// ---------------------------------------------------------------------------
// audit fix-gap
// ---------------------------------------------------------------------------

func TestFixGap_Success(t *testing.T) {
	env := newTestEnv(t)
	createLeafNode(t, env, "my-project", "My Project")

	// Add a gap first
	env.RootCmd.SetArgs([]string{"audit", "gap", "--node", "my-project", "some gap"})
	_ = env.RootCmd.Execute()

	ns := loadNodeState(t, env, "my-project")
	gapID := ns.Audit.Gaps[0].ID

	env.RootCmd.SetArgs([]string{"audit", "fix-gap", "--node", "my-project", gapID})
	if err := env.RootCmd.Execute(); err != nil {
		t.Fatalf("fix-gap failed: %v", err)
	}

	ns = loadNodeState(t, env, "my-project")
	if ns.Audit.Gaps[0].Status != state.GapFixed {
		t.Errorf("expected fixed, got %s", ns.Audit.Gaps[0].Status)
	}
}

func TestFixGap_NotFound(t *testing.T) {
	env := newTestEnv(t)
	createLeafNode(t, env, "my-project", "My Project")

	env.RootCmd.SetArgs([]string{"audit", "fix-gap", "--node", "my-project", "nonexistent-gap"})
	err := env.RootCmd.Execute()
	if err == nil {
		t.Error("expected error for nonexistent gap")
	}
}

func TestFixGap_AlreadyFixed(t *testing.T) {
	env := newTestEnv(t)
	createLeafNode(t, env, "my-project", "My Project")

	env.RootCmd.SetArgs([]string{"audit", "gap", "--node", "my-project", "a gap"})
	_ = env.RootCmd.Execute()

	ns := loadNodeState(t, env, "my-project")
	gapID := ns.Audit.Gaps[0].ID

	// Fix it
	env.RootCmd.SetArgs([]string{"audit", "fix-gap", "--node", "my-project", gapID})
	_ = env.RootCmd.Execute()

	// Try to fix again
	env.RootCmd.SetArgs([]string{"audit", "fix-gap", "--node", "my-project", gapID})
	err := env.RootCmd.Execute()
	if err == nil {
		t.Error("expected error when fixing already-fixed gap")
	}
}

// ---------------------------------------------------------------------------
// audit resolve
// ---------------------------------------------------------------------------

func TestResolve_Success(t *testing.T) {
	env := newTestEnv(t)
	createOrchestratorWithChild(t, env, "auth", "auth/login")

	// Add escalation
	env.RootCmd.SetArgs([]string{"audit", "escalate", "--node", "auth/login", "some issue"})
	_ = env.RootCmd.Execute()

	parentNs := loadNodeState(t, env, "auth")
	escID := parentNs.Audit.Escalations[0].ID

	env.RootCmd.SetArgs([]string{"audit", "resolve", "--node", "auth", escID})
	if err := env.RootCmd.Execute(); err != nil {
		t.Fatalf("resolve failed: %v", err)
	}

	parentNs = loadNodeState(t, env, "auth")
	if parentNs.Audit.Escalations[0].Status != state.EscalationResolved {
		t.Errorf("expected resolved, got %s", parentNs.Audit.Escalations[0].Status)
	}
}

func TestResolve_NotFound(t *testing.T) {
	env := newTestEnv(t)
	createLeafNode(t, env, "my-project", "My Project")

	env.RootCmd.SetArgs([]string{"audit", "resolve", "--node", "my-project", "nonexistent"})
	err := env.RootCmd.Execute()
	if err == nil {
		t.Error("expected error for nonexistent escalation")
	}
}

func TestResolve_AlreadyResolved(t *testing.T) {
	env := newTestEnv(t)
	createOrchestratorWithChild(t, env, "auth", "auth/login")

	env.RootCmd.SetArgs([]string{"audit", "escalate", "--node", "auth/login", "issue"})
	_ = env.RootCmd.Execute()

	parentNs := loadNodeState(t, env, "auth")
	escID := parentNs.Audit.Escalations[0].ID

	env.RootCmd.SetArgs([]string{"audit", "resolve", "--node", "auth", escID})
	_ = env.RootCmd.Execute()

	env.RootCmd.SetArgs([]string{"audit", "resolve", "--node", "auth", escID})
	err := env.RootCmd.Execute()
	if err == nil {
		t.Error("expected error when resolving already-resolved escalation")
	}
}

// ---------------------------------------------------------------------------
// audit show
// ---------------------------------------------------------------------------

func TestShow_Success(t *testing.T) {
	env := newTestEnv(t)
	createLeafNode(t, env, "my-project", "My Project")

	env.RootCmd.SetArgs([]string{"audit", "show", "--node", "my-project"})
	if err := env.RootCmd.Execute(); err != nil {
		t.Fatalf("show failed: %v", err)
	}
}

func TestShow_NoNode(t *testing.T) {
	env := newTestEnv(t)
	env.RootCmd.SetArgs([]string{"audit", "show"})
	err := env.RootCmd.Execute()
	if err == nil {
		t.Error("expected error when --node not provided")
	}
}

// ---------------------------------------------------------------------------
// audit scope
// ---------------------------------------------------------------------------

func TestScope_SetDescription(t *testing.T) {
	env := newTestEnv(t)
	createLeafNode(t, env, "my-project", "My Project")

	env.RootCmd.SetArgs([]string{"audit", "scope", "--node", "my-project", "--description", "verify auth module"})
	if err := env.RootCmd.Execute(); err != nil {
		t.Fatalf("scope failed: %v", err)
	}

	ns := loadNodeState(t, env, "my-project")
	if ns.Audit.Scope == nil {
		t.Fatal("scope should not be nil")
	}
	if ns.Audit.Scope.Description != "verify auth module" {
		t.Errorf("unexpected description: %s", ns.Audit.Scope.Description)
	}
}

func TestScope_SetFiles(t *testing.T) {
	env := newTestEnv(t)
	createLeafNode(t, env, "my-project", "My Project")

	env.RootCmd.SetArgs([]string{"audit", "scope", "--node", "my-project", "--files", "auth.go|login.go"})
	if err := env.RootCmd.Execute(); err != nil {
		t.Fatalf("scope failed: %v", err)
	}

	ns := loadNodeState(t, env, "my-project")
	if len(ns.Audit.Scope.Files) != 2 {
		t.Fatalf("expected 2 files, got %d", len(ns.Audit.Scope.Files))
	}
}

func TestScope_NoFields(t *testing.T) {
	env := newTestEnv(t)
	createLeafNode(t, env, "my-project", "My Project")

	env.RootCmd.SetArgs([]string{"audit", "scope", "--node", "my-project"})
	err := env.RootCmd.Execute()
	if err == nil {
		t.Error("expected error when no scope fields provided")
	}
}

// ---------------------------------------------------------------------------
// audit pending / history
// ---------------------------------------------------------------------------

func TestPending_NoBatch(t *testing.T) {
	env := newTestEnv(t)

	env.RootCmd.SetArgs([]string{"audit", "pending"})
	if err := env.RootCmd.Execute(); err != nil {
		t.Fatalf("pending failed: %v", err)
	}
}

func TestPending_WithBatch(t *testing.T) {
	env := newTestEnv(t)

	batch := &state.Batch{
		ID:     "audit-test",
		Status: state.BatchPending,
		Findings: []state.Finding{
			{ID: "finding-1", Title: "Test Finding", Status: state.FindingPending},
		},
	}
	batchPath := filepath.Join(env.WolfcastleDir, "audit-state.json")
	_ = state.SaveBatch(batchPath, batch)

	env.RootCmd.SetArgs([]string{"audit", "pending"})
	if err := env.RootCmd.Execute(); err != nil {
		t.Fatalf("pending failed: %v", err)
	}
}

func TestHistory_Empty(t *testing.T) {
	env := newTestEnv(t)

	env.RootCmd.SetArgs([]string{"audit", "history"})
	if err := env.RootCmd.Execute(); err != nil {
		t.Fatalf("history failed: %v", err)
	}
}

// ---------------------------------------------------------------------------
// audit reject
// ---------------------------------------------------------------------------

func TestReject_SingleFinding(t *testing.T) {
	env := newTestEnv(t)

	batch := &state.Batch{
		ID:     "audit-test",
		Status: state.BatchPending,
		Scopes: []string{"test"},
		Findings: []state.Finding{
			{ID: "finding-1", Title: "Test Finding", Status: state.FindingPending},
		},
	}
	batchPath := filepath.Join(env.WolfcastleDir, "audit-state.json")
	_ = state.SaveBatch(batchPath, batch)

	env.RootCmd.SetArgs([]string{"audit", "reject", "finding-1"})
	if err := env.RootCmd.Execute(); err != nil {
		t.Fatalf("reject failed: %v", err)
	}

	// Batch should be archived since all findings decided
	_, err := os.Stat(batchPath)
	if !os.IsNotExist(err) {
		t.Error("batch file should be removed after all findings decided")
	}
}

func TestReject_NoBatch(t *testing.T) {
	env := newTestEnv(t)

	env.RootCmd.SetArgs([]string{"audit", "reject", "finding-1"})
	err := env.RootCmd.Execute()
	if err == nil {
		t.Error("expected error when no batch exists")
	}
}

func TestReject_NoArgs(t *testing.T) {
	env := newTestEnv(t)

	batch := &state.Batch{
		ID:     "audit-test",
		Status: state.BatchPending,
		Findings: []state.Finding{
			{ID: "finding-1", Title: "Test", Status: state.FindingPending},
		},
	}
	_ = state.SaveBatch(filepath.Join(env.WolfcastleDir, "audit-state.json"), batch)

	env.RootCmd.SetArgs([]string{"audit", "reject"})
	err := env.RootCmd.Execute()
	if err == nil {
		t.Error("expected error with no args and no --all")
	}
}

func TestReject_All(t *testing.T) {
	env := newTestEnv(t)

	batch := &state.Batch{
		ID:     "audit-test",
		Status: state.BatchPending,
		Scopes: []string{"test"},
		Findings: []state.Finding{
			{ID: "finding-1", Title: "Finding One", Status: state.FindingPending},
			{ID: "finding-2", Title: "Finding Two", Status: state.FindingPending},
		},
	}
	batchPath := filepath.Join(env.WolfcastleDir, "audit-state.json")
	_ = state.SaveBatch(batchPath, batch)

	env.RootCmd.SetArgs([]string{"audit", "reject", "--all"})
	if err := env.RootCmd.Execute(); err != nil {
		t.Fatalf("reject --all failed: %v", err)
	}
}

// ---------------------------------------------------------------------------
// parseFindings
// ---------------------------------------------------------------------------

func TestParseFindings_MarkdownHeadings(t *testing.T) {
	input := `## Authentication Bypass
User tokens are not validated properly.

## SQL Injection Risk
Parameterized queries not used in search endpoint.
`
	findings := parseFindings(input)
	if len(findings) != 2 {
		t.Fatalf("expected 2 findings, got %d", len(findings))
	}
	if findings[0].Title != "Authentication Bypass" {
		t.Errorf("unexpected title: %s", findings[0].Title)
	}
	if findings[1].Title != "SQL Injection Risk" {
		t.Errorf("unexpected title: %s", findings[1].Title)
	}
}

func TestParseFindings_NumberedBold(t *testing.T) {
	input := `1. **Missing Rate Limiting**: No rate limiting on API endpoints.
2. **Stale Dependencies**: Several dependencies have known CVEs.
`
	findings := parseFindings(input)
	if len(findings) != 2 {
		t.Fatalf("expected 2 findings, got %d", len(findings))
	}
	if findings[0].Title != "Missing Rate Limiting" {
		t.Errorf("unexpected title: %s", findings[0].Title)
	}
}

func TestParseFindings_Empty(t *testing.T) {
	findings := parseFindings("")
	if len(findings) != 0 {
		t.Errorf("expected 0 findings, got %d", len(findings))
	}
}

func TestParseFindings_SkipsAuditFindingsHeader(t *testing.T) {
	input := `## Audit Findings

## Real Finding
Description here.
`
	findings := parseFindings(input)
	if len(findings) != 1 {
		t.Fatalf("expected 1 finding (skip 'Audit Findings' header), got %d", len(findings))
	}
	if findings[0].Title != "Real Finding" {
		t.Errorf("unexpected title: %s", findings[0].Title)
	}
}

// ---------------------------------------------------------------------------
// discoverScopes
// ---------------------------------------------------------------------------

func TestDiscoverScopes_FindsScopes(t *testing.T) {
	env := newTestEnv(t)

	// Create audit scope files
	baseAudits := filepath.Join(env.WolfcastleDir, "base", "audits")
	_ = os.MkdirAll(baseAudits, 0755)
	_ = os.WriteFile(filepath.Join(baseAudits, "security.md"), []byte("# Security\nCheck for vulnerabilities"), 0644)
	_ = os.WriteFile(filepath.Join(baseAudits, "performance.md"), []byte("# Performance\nCheck for bottlenecks"), 0644)

	scopes, err := discoverScopes(env.App)
	if err != nil {
		t.Fatalf("discoverScopes failed: %v", err)
	}
	if len(scopes) != 2 {
		t.Fatalf("expected 2 scopes, got %d", len(scopes))
	}
}

func TestDiscoverScopes_TierOverride(t *testing.T) {
	env := newTestEnv(t)

	// Base and custom with same name
	baseAudits := filepath.Join(env.WolfcastleDir, "base", "audits")
	_ = os.MkdirAll(baseAudits, 0755)
	_ = os.WriteFile(filepath.Join(baseAudits, "security.md"), []byte("base security"), 0644)

	customAudits := filepath.Join(env.WolfcastleDir, "custom", "audits")
	_ = os.MkdirAll(customAudits, 0755)
	_ = os.WriteFile(filepath.Join(customAudits, "security.md"), []byte("custom security"), 0644)

	scopes, err := discoverScopes(env.App)
	if err != nil {
		t.Fatalf("discoverScopes failed: %v", err)
	}
	if len(scopes) != 1 {
		t.Fatalf("expected 1 scope (custom overrides base), got %d", len(scopes))
	}
	// The prompt file should be the custom one
	if scopes[0].PromptFile != filepath.Join(customAudits, "security.md") {
		t.Errorf("expected custom prompt file, got %s", scopes[0].PromptFile)
	}
}

func TestDiscoverScopes_NoScopes(t *testing.T) {
	env := newTestEnv(t)
	scopes, err := discoverScopes(env.App)
	if err != nil {
		t.Fatalf("discoverScopes failed: %v", err)
	}
	if len(scopes) != 0 {
		t.Errorf("expected 0 scopes, got %d", len(scopes))
	}
}

// ---------------------------------------------------------------------------
// scope helpers
// ---------------------------------------------------------------------------

func TestSplitPipe(t *testing.T) {
	result := splitPipe("a|b|c")
	if len(result) != 3 {
		t.Fatalf("expected 3 items, got %d", len(result))
	}
	if result[0] != "a" || result[1] != "b" || result[2] != "c" {
		t.Errorf("unexpected result: %v", result)
	}
}

func TestSplitPipe_EmptyParts(t *testing.T) {
	result := splitPipe("a||b| |c")
	if len(result) != 3 {
		t.Fatalf("expected 3 items (empty parts dropped), got %d", len(result))
	}
}

func TestDedup(t *testing.T) {
	result := dedup([]string{"a", "b", "a", "c", "b"})
	if len(result) != 3 {
		t.Fatalf("expected 3 unique items, got %d", len(result))
	}
}

func TestScopeIDs(t *testing.T) {
	scopes := []auditScope{
		{ID: "security"},
		{ID: "performance"},
	}
	ids := scopeIDs(scopes)
	if len(ids) != 2 {
		t.Fatalf("expected 2 ids, got %d", len(ids))
	}
	if ids[0] != "security" || ids[1] != "performance" {
		t.Errorf("unexpected ids: %v", ids)
	}
}
