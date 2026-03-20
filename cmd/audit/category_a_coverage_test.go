package audit

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/dorkusprime/wolfcastle/internal/state"
)

// ---------------------------------------------------------------------------
// approve.go: invalid slug from all-punctuation title
// ---------------------------------------------------------------------------

func TestApprove_InvalidSlugTitle_SingleFinding(t *testing.T) {
	env := newTestEnv(t)

	// Title "123 invalid" produces slug "123-invalid" which starts with a digit
	// and fails ValidateSlug
	batch := &state.Batch{
		ID:     "audit-inval",
		Status: state.BatchPending,
		Scopes: []string{"test"},
		Findings: []state.Finding{
			{ID: "f-1", Title: "123 invalid", Status: state.FindingPending},
		},
	}
	_ = state.SaveBatch(filepath.Join(env.WolfcastleDir, "audit-state.json"), batch)

	// Single finding with invalid slug: should return error (not --all mode)
	env.RootCmd.SetArgs([]string{"audit", "approve", "f-1"})
	err := env.RootCmd.Execute()
	if err == nil {
		t.Error("expected error when title produces invalid slug")
	}
}

func TestApprove_InvalidSlugTitle_AllModeSkips(t *testing.T) {
	env := newTestEnv(t)

	// In --all mode, invalid slug findings are skipped (not error)
	batch := &state.Batch{
		ID:     "audit-inval-all",
		Status: state.BatchPending,
		Scopes: []string{"test"},
		Findings: []state.Finding{
			{ID: "f-1", Title: "123 invalid", Status: state.FindingPending},
			{ID: "f-2", Title: "Valid Finding Here", Status: state.FindingPending},
		},
	}
	_ = state.SaveBatch(filepath.Join(env.WolfcastleDir, "audit-state.json"), batch)

	env.RootCmd.SetArgs([]string{"audit", "approve", "--all"})
	err := env.RootCmd.Execute()
	if err != nil {
		t.Fatalf("approve --all should skip invalid slug and proceed: %v", err)
	}
}

// ---------------------------------------------------------------------------
// approve.go: CreateProject error (duplicate slug)
// ---------------------------------------------------------------------------

func TestApprove_DuplicateSlugCreateProjectError(t *testing.T) {
	env := newTestEnv(t)

	// Two findings with the same title in --all mode: the second one hits
	// the CreateProject "already exists" branch which prints an error and
	// continues. Actually, the existing test TestApprove_CreateProjectError
	// covers the "already exists in index" path. Here we exercise the case
	// where CreateProject itself returns an error for a different reason.
	// We pre-create the node so the "already exists" dedup path triggers.
	env.createLeafNode(t, "auth-issue", "Auth Issue")

	batch := &state.Batch{
		ID:     "audit-dup-slug",
		Status: state.BatchPending,
		Scopes: []string{"test"},
		Findings: []state.Finding{
			{ID: "f-1", Title: "Auth Issue", Status: state.FindingPending},
		},
	}
	_ = state.SaveBatch(filepath.Join(env.WolfcastleDir, "audit-state.json"), batch)

	env.RootCmd.SetArgs([]string{"audit", "approve", "f-1"})
	err := env.RootCmd.Execute()
	// Should succeed: "already exists" path marks it approved without creating
	if err != nil {
		t.Fatalf("approve with existing project should succeed: %v", err)
	}
}

// ---------------------------------------------------------------------------
// codebase.go: unknown --scope when other scopes exist
// ---------------------------------------------------------------------------

func TestRunCmd_UnknownScopeWhenOthersExist(t *testing.T) {
	env := newTestEnv(t)

	// Create real scopes so the "unknown" one is checked against a populated map
	baseAudits := filepath.Join(env.WolfcastleDir, "system", "base", "audits")
	_ = os.MkdirAll(baseAudits, 0755)
	_ = os.WriteFile(filepath.Join(baseAudits, "security.md"), []byte("# Security\nSecurity audit"), 0644)
	_ = os.WriteFile(filepath.Join(baseAudits, "performance.md"), []byte("# Perf\nPerformance audit"), 0644)

	env.RootCmd.SetArgs([]string{"audit", "run", "--scope", "nonexistent-scope"})
	err := env.RootCmd.Execute()
	if err == nil {
		t.Error("expected error for unknown scope when valid scopes exist")
	}
}

// ---------------------------------------------------------------------------
// codebase.go: no scopes found for run (exercise the len==0 guard)
// ---------------------------------------------------------------------------

func TestRunCmd_NoScopesFoundForRun(t *testing.T) {
	env := newTestEnv(t)

	// No audit scope files exist at all
	env.RootCmd.SetArgs([]string{"audit", "run"})
	err := env.RootCmd.Execute()
	if err == nil {
		t.Error("expected error when no audit scopes found")
	}
}

// ---------------------------------------------------------------------------
// scope.go: nil scope initialization (node without pre-existing scope)
// ---------------------------------------------------------------------------

func TestScope_NilScopeInitialization(t *testing.T) {
	env := newTestEnv(t)
	env.createLeafNode(t, "fresh-node", "Fresh Node")

	// Node starts with no scope set. Setting description should initialize it.
	env.RootCmd.SetArgs([]string{"audit", "scope", "--node", "fresh-node", "--description", "new audit scope"})
	if err := env.RootCmd.Execute(); err != nil {
		t.Fatalf("scope on fresh node failed: %v", err)
	}

	ns := env.loadNodeState(t, "fresh-node")
	if ns.Audit.Scope == nil {
		t.Fatal("scope should have been initialized")
	}
	if ns.Audit.Scope.Description != "new audit scope" {
		t.Errorf("unexpected description: %s", ns.Audit.Scope.Description)
	}
}

// ---------------------------------------------------------------------------
// show.go: empty --node guard
// ---------------------------------------------------------------------------

func TestShow_EmptyNodeGuard(t *testing.T) {
	env := newTestEnv(t)

	env.RootCmd.SetArgs([]string{"audit", "show", "--node", ""})
	err := env.RootCmd.Execute()
	if err == nil {
		t.Error("expected error when --node is empty for show")
	}
}

// ---------------------------------------------------------------------------
// All commands: empty --node guards (breadcrumb, escalate, fix_gap, gap, resolve)
// ---------------------------------------------------------------------------

func TestBreadcrumb_EmptyNodeGuard(t *testing.T) {
	env := newTestEnv(t)

	env.RootCmd.SetArgs([]string{"audit", "breadcrumb", "--node", "", "text"})
	err := env.RootCmd.Execute()
	if err == nil {
		t.Error("expected error when --node is empty for breadcrumb")
	}
}

func TestEscalate_EmptyNodeGuard(t *testing.T) {
	env := newTestEnv(t)

	env.RootCmd.SetArgs([]string{"audit", "escalate", "--node", "", "description"})
	err := env.RootCmd.Execute()
	if err == nil {
		t.Error("expected error when --node is empty for escalate")
	}
}

func TestFixGap_EmptyNodeGuard(t *testing.T) {
	env := newTestEnv(t)

	env.RootCmd.SetArgs([]string{"audit", "fix-gap", "--node", "", "gap-1"})
	err := env.RootCmd.Execute()
	if err == nil {
		t.Error("expected error when --node is empty for fix-gap")
	}
}

func TestGap_EmptyNodeGuard(t *testing.T) {
	env := newTestEnv(t)

	env.RootCmd.SetArgs([]string{"audit", "gap", "--node", "", "description"})
	err := env.RootCmd.Execute()
	if err == nil {
		t.Error("expected error when --node is empty for gap")
	}
}

func TestResolve_EmptyNodeGuard(t *testing.T) {
	env := newTestEnv(t)

	env.RootCmd.SetArgs([]string{"audit", "resolve", "--node", "", "esc-1"})
	err := env.RootCmd.Execute()
	if err == nil {
		t.Error("expected error when --node is empty for resolve")
	}
}

func TestScope_EmptyNodeGuard(t *testing.T) {
	env := newTestEnv(t)

	env.RootCmd.SetArgs([]string{"audit", "scope", "--node", "", "--description", "test"})
	err := env.RootCmd.Execute()
	if err == nil {
		t.Error("expected error when --node is empty for scope")
	}
}
