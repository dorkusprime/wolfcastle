package audit

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/dorkusprime/wolfcastle/internal/state"
)

func TestBreadcrumb_JSONOutput(t *testing.T) {
	env := newTestEnv(t)
	env.App.JSONOutput = true
	defer func() { env.App.JSONOutput = false }()

	createLeafNode(t, env, "my-project", "My Project")

	env.RootCmd.SetArgs([]string{"audit", "breadcrumb", "--node", "my-project", "did something"})
	if err := env.RootCmd.Execute(); err != nil {
		t.Fatalf("breadcrumb (json) failed: %v", err)
	}
}

func TestEscalate_JSONOutput(t *testing.T) {
	env := newTestEnv(t)
	env.App.JSONOutput = true
	defer func() { env.App.JSONOutput = false }()

	createOrchestratorWithChild(t, env, "auth", "auth/login")

	env.RootCmd.SetArgs([]string{"audit", "escalate", "--node", "auth/login", "gap found"})
	if err := env.RootCmd.Execute(); err != nil {
		t.Fatalf("escalate (json) failed: %v", err)
	}
}

func TestGap_JSONOutput(t *testing.T) {
	env := newTestEnv(t)
	env.App.JSONOutput = true
	defer func() { env.App.JSONOutput = false }()

	createLeafNode(t, env, "my-project", "My Project")

	env.RootCmd.SetArgs([]string{"audit", "gap", "--node", "my-project", "some gap"})
	if err := env.RootCmd.Execute(); err != nil {
		t.Fatalf("gap (json) failed: %v", err)
	}
}

func TestShow_JSONOutput(t *testing.T) {
	env := newTestEnv(t)
	env.App.JSONOutput = true
	defer func() { env.App.JSONOutput = false }()

	createLeafNode(t, env, "my-project", "My Project")

	env.RootCmd.SetArgs([]string{"audit", "show", "--node", "my-project"})
	if err := env.RootCmd.Execute(); err != nil {
		t.Fatalf("show (json) failed: %v", err)
	}
}

func TestScope_JSONOutput(t *testing.T) {
	env := newTestEnv(t)
	env.App.JSONOutput = true
	defer func() { env.App.JSONOutput = false }()

	createLeafNode(t, env, "my-project", "My Project")

	env.RootCmd.SetArgs([]string{"audit", "scope", "--node", "my-project", "--description", "test"})
	if err := env.RootCmd.Execute(); err != nil {
		t.Fatalf("scope (json) failed: %v", err)
	}
}

func TestPending_JSONOutput_NoBatch(t *testing.T) {
	env := newTestEnv(t)
	env.App.JSONOutput = true
	defer func() { env.App.JSONOutput = false }()

	env.RootCmd.SetArgs([]string{"audit", "pending"})
	if err := env.RootCmd.Execute(); err != nil {
		t.Fatalf("pending (json) failed: %v", err)
	}
}

func TestPending_JSONOutput_WithBatch(t *testing.T) {
	env := newTestEnv(t)
	env.App.JSONOutput = true
	defer func() { env.App.JSONOutput = false }()

	batch := &state.Batch{
		ID:     "test",
		Status: state.BatchPending,
		Scopes: []string{"security"},
		Findings: []state.Finding{
			{ID: "f-1", Title: "Finding", Status: state.FindingPending, Description: "details"},
		},
	}
	_ = state.SaveBatch(filepath.Join(env.WolfcastleDir, "audit-state.json"), batch)

	env.RootCmd.SetArgs([]string{"audit", "pending"})
	if err := env.RootCmd.Execute(); err != nil {
		t.Fatalf("pending (json) failed: %v", err)
	}
}

func TestHistory_JSONOutput(t *testing.T) {
	env := newTestEnv(t)
	env.App.JSONOutput = true
	defer func() { env.App.JSONOutput = false }()

	env.RootCmd.SetArgs([]string{"audit", "history"})
	if err := env.RootCmd.Execute(); err != nil {
		t.Fatalf("history (json) failed: %v", err)
	}
}

func TestReject_JSONOutput(t *testing.T) {
	env := newTestEnv(t)

	batch := &state.Batch{
		ID:     "test",
		Status: state.BatchPending,
		Scopes: []string{"test"},
		Findings: []state.Finding{
			{ID: "f-1", Title: "Finding", Status: state.FindingPending},
		},
	}
	_ = state.SaveBatch(filepath.Join(env.WolfcastleDir, "audit-state.json"), batch)

	env.App.JSONOutput = true
	defer func() { env.App.JSONOutput = false }()

	env.RootCmd.SetArgs([]string{"audit", "reject", "f-1"})
	if err := env.RootCmd.Execute(); err != nil {
		t.Fatalf("reject (json) failed: %v", err)
	}
}

func TestFixGap_JSONOutput(t *testing.T) {
	env := newTestEnv(t)
	createLeafNode(t, env, "my-project", "My Project")

	env.RootCmd.SetArgs([]string{"audit", "gap", "--node", "my-project", "test gap"})
	_ = env.RootCmd.Execute()

	ns := loadNodeState(t, env, "my-project")
	gapID := ns.Audit.Gaps[0].ID

	env.App.JSONOutput = true
	defer func() { env.App.JSONOutput = false }()

	env.RootCmd.SetArgs([]string{"audit", "fix-gap", "--node", "my-project", gapID})
	if err := env.RootCmd.Execute(); err != nil {
		t.Fatalf("fix-gap (json) failed: %v", err)
	}
}

func TestResolve_JSONOutput(t *testing.T) {
	env := newTestEnv(t)
	createOrchestratorWithChild(t, env, "auth", "auth/login")

	env.RootCmd.SetArgs([]string{"audit", "escalate", "--node", "auth/login", "issue"})
	_ = env.RootCmd.Execute()

	parentNs := loadNodeState(t, env, "auth")
	escID := parentNs.Audit.Escalations[0].ID

	env.App.JSONOutput = true
	defer func() { env.App.JSONOutput = false }()

	env.RootCmd.SetArgs([]string{"audit", "resolve", "--node", "auth", escID})
	if err := env.RootCmd.Execute(); err != nil {
		t.Fatalf("resolve (json) failed: %v", err)
	}
}

func TestScope_SetCriteria(t *testing.T) {
	env := newTestEnv(t)
	createLeafNode(t, env, "my-project", "My Project")

	env.RootCmd.SetArgs([]string{"audit", "scope", "--node", "my-project",
		"--criteria", "no SQL injection|input validation"})
	if err := env.RootCmd.Execute(); err != nil {
		t.Fatalf("scope with criteria failed: %v", err)
	}

	ns := loadNodeState(t, env, "my-project")
	if len(ns.Audit.Scope.Criteria) != 2 {
		t.Fatalf("expected 2 criteria, got %d", len(ns.Audit.Scope.Criteria))
	}
}

func TestScope_SetSystems(t *testing.T) {
	env := newTestEnv(t)
	createLeafNode(t, env, "my-project", "My Project")

	env.RootCmd.SetArgs([]string{"audit", "scope", "--node", "my-project",
		"--systems", "auth|session|database"})
	if err := env.RootCmd.Execute(); err != nil {
		t.Fatalf("scope with systems failed: %v", err)
	}

	ns := loadNodeState(t, env, "my-project")
	if len(ns.Audit.Scope.Systems) != 3 {
		t.Fatalf("expected 3 systems, got %d", len(ns.Audit.Scope.Systems))
	}
}

func TestAuditList_WithScopes(t *testing.T) {
	env := newTestEnv(t)

	baseAudits := filepath.Join(env.WolfcastleDir, "system", "base", "audits")
	_ = os.MkdirAll(baseAudits, 0755)
	_ = os.WriteFile(filepath.Join(baseAudits, "security.md"), []byte("Check security"), 0644)

	env.RootCmd.SetArgs([]string{"audit", "list"})
	if err := env.RootCmd.Execute(); err != nil {
		t.Fatalf("audit list failed: %v", err)
	}
}

func TestAuditList_JSONOutput(t *testing.T) {
	env := newTestEnv(t)
	env.App.JSONOutput = true
	defer func() { env.App.JSONOutput = false }()

	env.RootCmd.SetArgs([]string{"audit", "list"})
	if err := env.RootCmd.Execute(); err != nil {
		t.Fatalf("audit list (json) failed: %v", err)
	}
}

func TestPending_AllReviewed(t *testing.T) {
	env := newTestEnv(t)

	batch := &state.Batch{
		ID:     "test",
		Status: state.BatchPending,
		Scopes: []string{"test"},
		Findings: []state.Finding{
			{ID: "f-1", Title: "Finding", Status: state.FindingApproved},
		},
	}
	_ = state.SaveBatch(filepath.Join(env.WolfcastleDir, "audit-state.json"), batch)

	env.RootCmd.SetArgs([]string{"audit", "pending"})
	if err := env.RootCmd.Execute(); err != nil {
		t.Fatalf("pending (all reviewed) failed: %v", err)
	}
}

func TestShow_WithBreadcrumbsGapsEscalations(t *testing.T) {
	env := newTestEnv(t)
	createOrchestratorWithChild(t, env, "auth", "auth/login")

	// Add breadcrumb to child
	env.RootCmd.SetArgs([]string{"audit", "breadcrumb", "--node", "auth/login", "made progress"})
	_ = env.RootCmd.Execute()

	// Add gap to child
	env.RootCmd.SetArgs([]string{"audit", "gap", "--node", "auth/login", "missing tests"})
	_ = env.RootCmd.Execute()

	// Escalate from child
	env.RootCmd.SetArgs([]string{"audit", "escalate", "--node", "auth/login", "need input"})
	_ = env.RootCmd.Execute()

	// Show parent (has escalation)
	env.RootCmd.SetArgs([]string{"audit", "show", "--node", "auth"})
	if err := env.RootCmd.Execute(); err != nil {
		t.Fatalf("show failed: %v", err)
	}

	// Show child (has breadcrumbs and gaps)
	env.RootCmd.SetArgs([]string{"audit", "show", "--node", "auth/login"})
	if err := env.RootCmd.Execute(); err != nil {
		t.Fatalf("show child failed: %v", err)
	}
}
