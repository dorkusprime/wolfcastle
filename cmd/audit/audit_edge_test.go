package audit

import (
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/dorkusprime/wolfcastle/internal/state"
)

// ---------------------------------------------------------------------------
// audit show — with scope, result summary, all sections
// ---------------------------------------------------------------------------

func TestShow_WithScopeAndResultSummary(t *testing.T) {
	env := newTestEnv(t)
	env.createLeafNode(t, "my-project", "My Project")

	// Set scope with all fields
	env.RootCmd.SetArgs([]string{"audit", "scope", "--node", "my-project",
		"--description", "full audit",
		"--files", "main.go|util.go",
		"--systems", "auth|db",
		"--criteria", "no injection|validated"})
	_ = env.RootCmd.Execute()

	// Set result summary directly
	ns := env.loadNodeState(t, "my-project")
	ns.Audit.ResultSummary = "All checks passed with minor findings."
	parsed, _ := state.LoadNodeState(filepath.Join(env.ProjectsDir, "my-project", "state.json"))
	_ = parsed // just to avoid unused warning
	saveJSON(t, filepath.Join(env.ProjectsDir, "my-project", "state.json"), ns)

	// Show should display all sections
	env.RootCmd.SetArgs([]string{"audit", "show", "--node", "my-project"})
	if err := env.RootCmd.Execute(); err != nil {
		t.Fatalf("show with full scope failed: %v", err)
	}
}

func TestShow_NonexistentNode(t *testing.T) {
	env := newTestEnv(t)

	env.RootCmd.SetArgs([]string{"audit", "show", "--node", "nonexistent"})
	err := env.RootCmd.Execute()
	if err == nil {
		t.Error("expected error for nonexistent node")
	}
}

func TestShow_NoIdentity(t *testing.T) {
	env := newTestEnv(t)
	env.App.Identity = nil

	env.RootCmd.SetArgs([]string{"audit", "show", "--node", "my-project"})
	err := env.RootCmd.Execute()
	if err == nil {
		t.Error("expected error when identity is nil")
	}
}

// ---------------------------------------------------------------------------
// audit scope — error paths
// ---------------------------------------------------------------------------

func TestScope_NoIdentity(t *testing.T) {
	env := newTestEnv(t)
	env.App.Identity = nil

	env.RootCmd.SetArgs([]string{"audit", "scope", "--node", "x", "--description", "test"})
	err := env.RootCmd.Execute()
	if err == nil {
		t.Error("expected error without identity")
	}
}

func TestScope_NonexistentNode(t *testing.T) {
	env := newTestEnv(t)

	env.RootCmd.SetArgs([]string{"audit", "scope", "--node", "nonexistent", "--description", "test"})
	err := env.RootCmd.Execute()
	if err == nil {
		t.Error("expected error for nonexistent node")
	}
}

func TestScope_AllFields(t *testing.T) {
	env := newTestEnv(t)
	env.createLeafNode(t, "my-project", "My Project")

	env.RootCmd.SetArgs([]string{"audit", "scope", "--node", "my-project",
		"--description", "verify everything",
		"--files", "a.go|b.go|a.go",
		"--systems", "auth|auth|db",
		"--criteria", "c1|c2"})
	if err := env.RootCmd.Execute(); err != nil {
		t.Fatalf("scope all fields failed: %v", err)
	}

	ns := env.loadNodeState(t, "my-project")
	// Check dedup worked
	if len(ns.Audit.Scope.Files) != 2 {
		t.Errorf("expected 2 deduped files, got %d", len(ns.Audit.Scope.Files))
	}
	if len(ns.Audit.Scope.Systems) != 2 {
		t.Errorf("expected 2 deduped systems, got %d", len(ns.Audit.Scope.Systems))
	}
}

// ---------------------------------------------------------------------------
// audit pending — with long descriptions
// ---------------------------------------------------------------------------

func TestPending_WithLongDescription(t *testing.T) {
	env := newTestEnv(t)

	longDesc := "This is a very long description that exceeds eighty characters in length and should be truncated by the display logic."
	batch := &state.Batch{
		ID:     "test",
		Status: state.BatchPending,
		Scopes: []string{"test"},
		Findings: []state.Finding{
			{ID: "f-1", Title: "Long Finding", Status: state.FindingPending, Description: longDesc},
		},
	}
	_ = state.SaveBatch(filepath.Join(env.WolfcastleDir, "audit-state.json"), batch)

	env.RootCmd.SetArgs([]string{"audit", "pending"})
	if err := env.RootCmd.Execute(); err != nil {
		t.Fatalf("pending with long desc failed: %v", err)
	}
}

func TestPending_WithMultilineDescription(t *testing.T) {
	env := newTestEnv(t)

	batch := &state.Batch{
		ID:     "test",
		Status: state.BatchPending,
		Scopes: []string{"test"},
		Findings: []state.Finding{
			{ID: "f-1", Title: "Multi Finding", Status: state.FindingPending,
				Description: "First line of the description.\nSecond line with more details."},
		},
	}
	_ = state.SaveBatch(filepath.Join(env.WolfcastleDir, "audit-state.json"), batch)

	env.RootCmd.SetArgs([]string{"audit", "pending"})
	if err := env.RootCmd.Execute(); err != nil {
		t.Fatalf("pending with multiline desc failed: %v", err)
	}
}

// ---------------------------------------------------------------------------
// audit reject — error paths
// ---------------------------------------------------------------------------

func TestReject_FindingNotFound(t *testing.T) {
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

	env.RootCmd.SetArgs([]string{"audit", "reject", "nonexistent"})
	err := env.RootCmd.Execute()
	if err == nil {
		t.Error("expected error for nonexistent finding")
	}
}

func TestReject_NoPendingForAll(t *testing.T) {
	env := newTestEnv(t)

	batch := &state.Batch{
		ID:     "test",
		Status: state.BatchPending,
		Scopes: []string{"test"},
		Findings: []state.Finding{
			{ID: "f-1", Title: "Already Done", Status: state.FindingApproved},
		},
	}
	_ = state.SaveBatch(filepath.Join(env.WolfcastleDir, "audit-state.json"), batch)

	env.RootCmd.SetArgs([]string{"audit", "reject", "--all"})
	err := env.RootCmd.Execute()
	if err == nil {
		t.Error("expected error when no pending findings for --all")
	}
}

// ---------------------------------------------------------------------------
// audit approve — mixed batch (some approved, some pending)
// ---------------------------------------------------------------------------

func TestApprove_MixedBatch(t *testing.T) {
	env := newTestEnv(t)

	batch := &state.Batch{
		ID:     "test",
		Status: state.BatchPending,
		Scopes: []string{"test"},
		Findings: []state.Finding{
			{ID: "f-1", Title: "Already Rejected", Status: state.FindingRejected},
			{ID: "f-2", Title: "Pending Finding", Status: state.FindingPending},
		},
	}
	_ = state.SaveBatch(filepath.Join(env.WolfcastleDir, "audit-state.json"), batch)

	// Approve just f-2 (f-1 is already decided)
	env.RootCmd.SetArgs([]string{"audit", "approve", "f-2"})
	if err := env.RootCmd.Execute(); err != nil {
		t.Fatalf("approve mixed batch failed: %v", err)
	}

	// Batch should be archived (all decided)
	batchPath := filepath.Join(env.WolfcastleDir, "audit-state.json")
	_, err := os.Stat(batchPath)
	if !os.IsNotExist(err) {
		t.Error("batch should be archived after all findings decided")
	}
}

// ---------------------------------------------------------------------------
// finalizeBatchIfComplete — with DecidedAt set
// ---------------------------------------------------------------------------

func TestFinalizeBatchIfComplete_WithDecidedAt(t *testing.T) {
	env := newTestEnv(t)

	now := time.Now()
	batch := &state.Batch{
		ID:     "test-decided",
		Status: state.BatchPending,
		Scopes: []string{"test"},
		Findings: []state.Finding{
			{ID: "f-1", Title: "Approved", Status: state.FindingApproved, DecidedAt: &now, CreatedNode: "some-node"},
			{ID: "f-2", Title: "Rejected", Status: state.FindingRejected, DecidedAt: &now},
		},
	}
	batchPath := filepath.Join(env.WolfcastleDir, "audit-state.json")
	_ = state.SaveBatch(batchPath, batch)

	err := finalizeBatchIfComplete(env.App, batch, batchPath)
	if err != nil {
		t.Fatalf("finalizeBatchIfComplete with DecidedAt failed: %v", err)
	}

	// Verify history was created
	historyPath := filepath.Join(env.WolfcastleDir, "audit-review-history.json")
	history, err := state.LoadHistory(historyPath)
	if err != nil {
		t.Fatalf("loading history: %v", err)
	}
	if len(history.Entries) != 1 {
		t.Fatalf("expected 1 history entry, got %d", len(history.Entries))
	}
	if history.Entries[0].BatchID != "test-decided" {
		t.Errorf("unexpected batch ID: %s", history.Entries[0].BatchID)
	}
}

// ---------------------------------------------------------------------------
// audit gap — no resolver
// ---------------------------------------------------------------------------

func TestGap_NoIdentity(t *testing.T) {
	env := newTestEnv(t)
	env.App.Identity = nil

	env.RootCmd.SetArgs([]string{"audit", "gap", "--node", "my-project", "gap desc"})
	err := env.RootCmd.Execute()
	if err == nil {
		t.Error("expected error without identity")
	}
}

func TestGap_NonexistentNode(t *testing.T) {
	env := newTestEnv(t)

	env.RootCmd.SetArgs([]string{"audit", "gap", "--node", "nonexistent", "gap desc"})
	err := env.RootCmd.Execute()
	if err == nil {
		t.Error("expected error for nonexistent node")
	}
}

// ---------------------------------------------------------------------------
// audit fix-gap — no resolver
// ---------------------------------------------------------------------------

func TestFixGap_NoIdentity(t *testing.T) {
	env := newTestEnv(t)
	env.App.Identity = nil

	env.RootCmd.SetArgs([]string{"audit", "fix-gap", "--node", "my-project", "gap-1"})
	err := env.RootCmd.Execute()
	if err == nil {
		t.Error("expected error without identity")
	}
}

func TestFixGap_NonexistentNode(t *testing.T) {
	env := newTestEnv(t)

	env.RootCmd.SetArgs([]string{"audit", "fix-gap", "--node", "nonexistent", "gap-1"})
	err := env.RootCmd.Execute()
	if err == nil {
		t.Error("expected error for nonexistent node")
	}
}

// ---------------------------------------------------------------------------
// audit escalate — no resolver
// ---------------------------------------------------------------------------

func TestEscalate_NoIdentity(t *testing.T) {
	env := newTestEnv(t)
	env.App.Identity = nil

	env.RootCmd.SetArgs([]string{"audit", "escalate", "--node", "auth/login", "desc"})
	err := env.RootCmd.Execute()
	if err == nil {
		t.Error("expected error without identity")
	}
}

func TestEscalate_NonexistentParent(t *testing.T) {
	env := newTestEnv(t)

	// Node exists but parent doesn't
	env.createLeafNode(t, "auth/login", "Login")

	env.RootCmd.SetArgs([]string{"audit", "escalate", "--node", "auth/login", "gap found"})
	err := env.RootCmd.Execute()
	if err == nil {
		t.Error("expected error when parent node doesn't exist on disk")
	}
}

// ---------------------------------------------------------------------------
// audit resolve — no resolver
// ---------------------------------------------------------------------------

func TestResolve_NoIdentity(t *testing.T) {
	env := newTestEnv(t)
	env.App.Identity = nil

	env.RootCmd.SetArgs([]string{"audit", "resolve", "--node", "auth", "esc-1"})
	err := env.RootCmd.Execute()
	if err == nil {
		t.Error("expected error without identity")
	}
}

func TestResolve_NonexistentNode(t *testing.T) {
	env := newTestEnv(t)

	env.RootCmd.SetArgs([]string{"audit", "resolve", "--node", "nonexistent", "esc-1"})
	err := env.RootCmd.Execute()
	if err == nil {
		t.Error("expected error for nonexistent node")
	}
}

// ---------------------------------------------------------------------------
// audit breadcrumb — nonexistent node
// ---------------------------------------------------------------------------

func TestBreadcrumb_NonexistentNode(t *testing.T) {
	env := newTestEnv(t)

	env.RootCmd.SetArgs([]string{"audit", "breadcrumb", "--node", "nonexistent", "note"})
	err := env.RootCmd.Execute()
	if err == nil {
		t.Error("expected error for nonexistent node")
	}
}

// ---------------------------------------------------------------------------
// audit reject — JSON output with all
// ---------------------------------------------------------------------------

func TestReject_JSONOutput_All(t *testing.T) {
	env := newTestEnv(t)

	batch := &state.Batch{
		ID:     "test",
		Status: state.BatchPending,
		Scopes: []string{"test"},
		Findings: []state.Finding{
			{ID: "f-1", Title: "First", Status: state.FindingPending},
			{ID: "f-2", Title: "Second", Status: state.FindingPending},
		},
	}
	_ = state.SaveBatch(filepath.Join(env.WolfcastleDir, "audit-state.json"), batch)

	env.App.JSONOutput = true
	defer func() { env.App.JSONOutput = false }()

	env.RootCmd.SetArgs([]string{"audit", "reject", "--all"})
	if err := env.RootCmd.Execute(); err != nil {
		t.Fatalf("reject --all (json) failed: %v", err)
	}
}

// ---------------------------------------------------------------------------
// audit approve — JSON output with --all
// ---------------------------------------------------------------------------

func TestApprove_JSONOutput_All(t *testing.T) {
	env := newTestEnv(t)

	batch := &state.Batch{
		ID:     "test",
		Status: state.BatchPending,
		Scopes: []string{"test"},
		Findings: []state.Finding{
			{ID: "f-1", Title: "Rate Limit Issue", Status: state.FindingPending},
			{ID: "f-2", Title: "Auth Bypass Risk", Status: state.FindingPending},
		},
	}
	_ = state.SaveBatch(filepath.Join(env.WolfcastleDir, "audit-state.json"), batch)

	env.App.JSONOutput = true
	defer func() { env.App.JSONOutput = false }()

	env.RootCmd.SetArgs([]string{"audit", "approve", "--all"})
	if err := env.RootCmd.Execute(); err != nil {
		t.Fatalf("approve --all (json) failed: %v", err)
	}
}

// ---------------------------------------------------------------------------
// audit pending — JSON output with all reviewed
// ---------------------------------------------------------------------------

func TestPending_JSONOutput_AllReviewed(t *testing.T) {
	env := newTestEnv(t)
	env.App.JSONOutput = true
	defer func() { env.App.JSONOutput = false }()

	batch := &state.Batch{
		ID:     "test",
		Status: state.BatchPending,
		Scopes: []string{"test"},
		Findings: []state.Finding{
			{ID: "f-1", Title: "Done", Status: state.FindingApproved},
		},
	}
	_ = state.SaveBatch(filepath.Join(env.WolfcastleDir, "audit-state.json"), batch)

	env.RootCmd.SetArgs([]string{"audit", "pending"})
	if err := env.RootCmd.Execute(); err != nil {
		t.Fatalf("pending JSON all reviewed failed: %v", err)
	}
}

// ---------------------------------------------------------------------------
// parseFindings — numbered bold with description after
// ---------------------------------------------------------------------------

func TestParseFindings_BoldWithDescriptionAfter(t *testing.T) {
	input := `1. **Missing Rate Limiting**: No rate limiting on login endpoint.
More detail here.

2. **SQL Injection** — Parameterized queries missing.
`
	findings := parseFindings(input)
	if len(findings) != 2 {
		t.Fatalf("expected 2 findings, got %d", len(findings))
	}
	if findings[0].Description == "" {
		t.Error("expected description for first finding")
	}
}

func TestParseFindings_HeadingsWithDescription(t *testing.T) {
	input := `## First Issue
Some description of the first issue.
More lines.

## Second Issue
Description of second.
`
	findings := parseFindings(input)
	if len(findings) != 2 {
		t.Fatalf("expected 2 findings, got %d", len(findings))
	}
	if findings[0].Description == "" {
		t.Error("first finding should have description")
	}
	if findings[1].Description == "" {
		t.Error("second finding should have description")
	}
}

// ---------------------------------------------------------------------------
// audit list — no scopes (human output)
// ---------------------------------------------------------------------------

// ---------------------------------------------------------------------------
// audit approve — approve finding with description (covers descContent branch)
// ---------------------------------------------------------------------------

func TestApprove_WithDescription(t *testing.T) {
	env := newTestEnv(t)

	batch := &state.Batch{
		ID:     "test",
		Status: state.BatchPending,
		Scopes: []string{"test"},
		Findings: []state.Finding{
			{ID: "f-1", Title: "Missing Rate Limiting", Status: state.FindingPending,
				Description: "The API has no rate limiting configured."},
		},
	}
	_ = state.SaveBatch(filepath.Join(env.WolfcastleDir, "audit-state.json"), batch)

	env.RootCmd.SetArgs([]string{"audit", "approve", "f-1"})
	if err := env.RootCmd.Execute(); err != nil {
		t.Fatalf("approve with description failed: %v", err)
	}

	// Check description file was created
	descPath := filepath.Join(env.ProjectsDir, "missing-rate-limiting.md")
	data, err := os.ReadFile(descPath)
	if err != nil {
		t.Fatalf("reading description file: %v", err)
	}
	if len(data) == 0 {
		t.Error("description file should not be empty")
	}
}

// ---------------------------------------------------------------------------
// audit approve — approve all with mixed findings including already-decided
// ---------------------------------------------------------------------------

func TestApprove_AllWithMixed(t *testing.T) {
	env := newTestEnv(t)

	batch := &state.Batch{
		ID:     "test",
		Status: state.BatchPending,
		Scopes: []string{"test"},
		Findings: []state.Finding{
			{ID: "f-1", Title: "Already Rejected", Status: state.FindingRejected},
			{ID: "f-2", Title: "Pending One", Status: state.FindingPending},
			{ID: "f-3", Title: "Pending Two", Status: state.FindingPending},
		},
	}
	_ = state.SaveBatch(filepath.Join(env.WolfcastleDir, "audit-state.json"), batch)

	env.RootCmd.SetArgs([]string{"audit", "approve", "--all"})
	if err := env.RootCmd.Execute(); err != nil {
		t.Fatalf("approve --all mixed failed: %v", err)
	}
}

// ---------------------------------------------------------------------------
// audit scope — update existing scope (scope already non-nil)
// ---------------------------------------------------------------------------

func TestScope_UpdateExisting(t *testing.T) {
	env := newTestEnv(t)
	env.createLeafNode(t, "my-project", "My Project")

	// First set description
	env.RootCmd.SetArgs([]string{"audit", "scope", "--node", "my-project", "--description", "initial"})
	_ = env.RootCmd.Execute()

	// Then update files (description should remain)
	env.RootCmd.SetArgs([]string{"audit", "scope", "--node", "my-project", "--files", "new.go"})
	if err := env.RootCmd.Execute(); err != nil {
		t.Fatalf("scope update failed: %v", err)
	}

	ns := env.loadNodeState(t, "my-project")
	if ns.Audit.Scope.Description != "initial" {
		t.Errorf("description should be preserved, got %s", ns.Audit.Scope.Description)
	}
	if len(ns.Audit.Scope.Files) != 1 {
		t.Errorf("expected 1 file, got %d", len(ns.Audit.Scope.Files))
	}
}

// ---------------------------------------------------------------------------
// audit gap — multiple gaps
// ---------------------------------------------------------------------------

func TestGap_Multiple(t *testing.T) {
	env := newTestEnv(t)
	env.createLeafNode(t, "my-project", "My Project")

	env.RootCmd.SetArgs([]string{"audit", "gap", "--node", "my-project", "first gap"})
	_ = env.RootCmd.Execute()
	env.RootCmd.SetArgs([]string{"audit", "gap", "--node", "my-project", "second gap"})
	_ = env.RootCmd.Execute()

	ns := env.loadNodeState(t, "my-project")
	if len(ns.Audit.Gaps) != 2 {
		t.Fatalf("expected 2 gaps, got %d", len(ns.Audit.Gaps))
	}
	if ns.Audit.Gaps[0].ID == ns.Audit.Gaps[1].ID {
		t.Error("gap IDs should be unique")
	}
}

// ---------------------------------------------------------------------------
// audit breadcrumb — with task flag
// ---------------------------------------------------------------------------

func TestBreadcrumb_MultipleBreadcrumbs(t *testing.T) {
	env := newTestEnv(t)
	env.createLeafNode(t, "my-project", "My Project")

	env.RootCmd.SetArgs([]string{"audit", "breadcrumb", "--node", "my-project", "first note"})
	_ = env.RootCmd.Execute()
	env.RootCmd.SetArgs([]string{"audit", "breadcrumb", "--node", "my-project", "second note"})
	_ = env.RootCmd.Execute()

	ns := env.loadNodeState(t, "my-project")
	if len(ns.Audit.Breadcrumbs) != 2 {
		t.Fatalf("expected 2 breadcrumbs, got %d", len(ns.Audit.Breadcrumbs))
	}
}

// ---------------------------------------------------------------------------
// audit show — with all audit data populated
// ---------------------------------------------------------------------------

func TestShow_FullAuditState(t *testing.T) {
	env := newTestEnv(t)
	env.createOrchestratorWithChild(t, "auth", "auth/login")

	// Set scope on child
	env.RootCmd.SetArgs([]string{"audit", "scope", "--node", "auth/login",
		"--description", "verify login", "--files", "login.go", "--systems", "auth", "--criteria", "no bypass"})
	_ = env.RootCmd.Execute()

	// Add breadcrumb
	env.RootCmd.SetArgs([]string{"audit", "breadcrumb", "--node", "auth/login", "reviewed code"})
	_ = env.RootCmd.Execute()

	// Add gap
	env.RootCmd.SetArgs([]string{"audit", "gap", "--node", "auth/login", "missing rate limit"})
	_ = env.RootCmd.Execute()

	// Escalate (creates escalation on parent)
	env.RootCmd.SetArgs([]string{"audit", "escalate", "--node", "auth/login", "architecture concern"})
	_ = env.RootCmd.Execute()

	// Show child (has scope, breadcrumbs, gaps)
	env.RootCmd.SetArgs([]string{"audit", "show", "--node", "auth/login"})
	if err := env.RootCmd.Execute(); err != nil {
		t.Fatalf("show child with full audit state failed: %v", err)
	}

	// Show parent (has escalations)
	env.RootCmd.SetArgs([]string{"audit", "show", "--node", "auth"})
	if err := env.RootCmd.Execute(); err != nil {
		t.Fatalf("show parent with escalations failed: %v", err)
	}
}

// ---------------------------------------------------------------------------
// audit approve — duplicate project (CreateProject error)
// ---------------------------------------------------------------------------

func TestApprove_CreateProjectError(t *testing.T) {
	env := newTestEnv(t)

	// Create a batch where two findings produce the same slug
	batch := &state.Batch{
		ID:     "test",
		Status: state.BatchPending,
		Scopes: []string{"test"},
		Findings: []state.Finding{
			{ID: "f-1", Title: "Auth Issue", Status: state.FindingPending},
			{ID: "f-2", Title: "Auth Issue", Status: state.FindingPending},
		},
	}
	_ = state.SaveBatch(filepath.Join(env.WolfcastleDir, "audit-state.json"), batch)

	// First approve should succeed, second should hit "already exists" path
	env.RootCmd.SetArgs([]string{"audit", "approve", "--all"})
	if err := env.RootCmd.Execute(); err != nil {
		t.Fatalf("approve --all with duplicate slugs failed: %v", err)
	}
}

// ---------------------------------------------------------------------------
// audit approve — LoadBatch error (invalid JSON)
// ---------------------------------------------------------------------------

func TestApprove_BrokenBatchFile(t *testing.T) {
	env := newTestEnv(t)

	batchPath := filepath.Join(env.WolfcastleDir, "audit-state.json")
	_ = os.WriteFile(batchPath, []byte("not valid json"), 0644)

	env.RootCmd.SetArgs([]string{"audit", "approve", "f-1"})
	err := env.RootCmd.Execute()
	if err == nil {
		t.Error("expected error when batch file has invalid JSON")
	}
}

// ---------------------------------------------------------------------------
// audit reject — LoadBatch error (invalid JSON)
// ---------------------------------------------------------------------------

func TestReject_BrokenBatchFile(t *testing.T) {
	env := newTestEnv(t)

	batchPath := filepath.Join(env.WolfcastleDir, "audit-state.json")
	_ = os.WriteFile(batchPath, []byte("not valid json"), 0644)

	env.RootCmd.SetArgs([]string{"audit", "reject", "f-1"})
	err := env.RootCmd.Execute()
	if err == nil {
		t.Error("expected error when batch file has invalid JSON")
	}
}

// ---------------------------------------------------------------------------
// audit pending — LoadBatch error (invalid JSON)
// ---------------------------------------------------------------------------

func TestPending_BrokenBatchFile(t *testing.T) {
	env := newTestEnv(t)

	batchPath := filepath.Join(env.WolfcastleDir, "audit-state.json")
	_ = os.WriteFile(batchPath, []byte("not valid json"), 0644)

	env.RootCmd.SetArgs([]string{"audit", "pending"})
	err := env.RootCmd.Execute()
	if err == nil {
		t.Error("expected error when batch file has invalid JSON")
	}
}

// ---------------------------------------------------------------------------
// audit run — LoadBatch error (invalid JSON in existing batch)
// ---------------------------------------------------------------------------

func TestRunCmd_BrokenBatchFile(t *testing.T) {
	env := newTestEnv(t)

	// Create audit scope files so we get past the "no scopes" check
	baseAudits := filepath.Join(env.WolfcastleDir, "system", "base", "audits")
	_ = os.MkdirAll(baseAudits, 0755)
	_ = os.WriteFile(filepath.Join(baseAudits, "test.md"), []byte("# Test\nTest scope"), 0644)

	// Write invalid JSON to batch file
	batchPath := filepath.Join(env.WolfcastleDir, "audit-state.json")
	_ = os.WriteFile(batchPath, []byte("not valid json"), 0644)

	env.RootCmd.SetArgs([]string{"audit", "run"})
	err := env.RootCmd.Execute()
	if err == nil {
		t.Error("expected error when batch file has invalid JSON")
	}
}

// ---------------------------------------------------------------------------
// audit approve — LoadRootIndex error (broken root index)
// ---------------------------------------------------------------------------

func TestApprove_BrokenRootIndex(t *testing.T) {
	env := newTestEnv(t)

	batch := &state.Batch{
		ID:     "test",
		Status: state.BatchPending,
		Scopes: []string{"test"},
		Findings: []state.Finding{
			{ID: "f-1", Title: "Test", Status: state.FindingPending},
		},
	}
	_ = state.SaveBatch(filepath.Join(env.WolfcastleDir, "audit-state.json"), batch)

	// Break the root index
	_ = os.WriteFile(filepath.Join(env.ProjectsDir, "state.json"), []byte("broken"), 0644)

	env.RootCmd.SetArgs([]string{"audit", "approve", "f-1"})
	err := env.RootCmd.Execute()
	if err == nil {
		t.Error("expected error when root index is broken")
	}
}

// ---------------------------------------------------------------------------
// audit breadcrumb — save error (broken state file)
// ---------------------------------------------------------------------------

func TestBreadcrumb_InvalidNodeAddress(t *testing.T) {
	env := newTestEnv(t)

	env.RootCmd.SetArgs([]string{"audit", "breadcrumb", "--node", "INVALID", "note"})
	err := env.RootCmd.Execute()
	if err == nil {
		t.Error("expected error for invalid node address")
	}
}

// ---------------------------------------------------------------------------
// audit gap — invalid address
// ---------------------------------------------------------------------------

func TestGap_InvalidAddress(t *testing.T) {
	env := newTestEnv(t)

	env.RootCmd.SetArgs([]string{"audit", "gap", "--node", "INVALID", "gap"})
	err := env.RootCmd.Execute()
	if err == nil {
		t.Error("expected error for invalid node address")
	}
}

// ---------------------------------------------------------------------------
// audit fix-gap — invalid address
// ---------------------------------------------------------------------------

func TestFixGap_InvalidAddress(t *testing.T) {
	env := newTestEnv(t)

	env.RootCmd.SetArgs([]string{"audit", "fix-gap", "--node", "INVALID", "gap-1"})
	err := env.RootCmd.Execute()
	if err == nil {
		t.Error("expected error for invalid node address")
	}
}

// ---------------------------------------------------------------------------
// audit escalate — invalid address
// ---------------------------------------------------------------------------

func TestEscalate_InvalidAddress(t *testing.T) {
	env := newTestEnv(t)

	env.RootCmd.SetArgs([]string{"audit", "escalate", "--node", "INVALID", "issue"})
	err := env.RootCmd.Execute()
	if err == nil {
		t.Error("expected error for invalid node address")
	}
}

// ---------------------------------------------------------------------------
// audit resolve — invalid address
// ---------------------------------------------------------------------------

func TestResolve_InvalidAddress(t *testing.T) {
	env := newTestEnv(t)

	env.RootCmd.SetArgs([]string{"audit", "resolve", "--node", "INVALID", "esc-1"})
	err := env.RootCmd.Execute()
	if err == nil {
		t.Error("expected error for invalid node address")
	}
}

// ---------------------------------------------------------------------------
// audit scope — invalid address
// ---------------------------------------------------------------------------

func TestScope_InvalidAddress(t *testing.T) {
	env := newTestEnv(t)

	env.RootCmd.SetArgs([]string{"audit", "scope", "--node", "INVALID", "--description", "test"})
	err := env.RootCmd.Execute()
	if err == nil {
		t.Error("expected error for invalid node address")
	}
}

// ---------------------------------------------------------------------------
// audit show — invalid address
// ---------------------------------------------------------------------------

func TestShow_InvalidAddress(t *testing.T) {
	env := newTestEnv(t)

	env.RootCmd.SetArgs([]string{"audit", "show", "--node", "INVALID"})
	err := env.RootCmd.Execute()
	if err == nil {
		t.Error("expected error for invalid node address")
	}
}

// ---------------------------------------------------------------------------
// finalizeBatchIfComplete — LoadHistory error (invalid JSON)
// ---------------------------------------------------------------------------

func TestFinalizeBatchIfComplete_BrokenHistory(t *testing.T) {
	env := newTestEnv(t)

	batch := &state.Batch{
		ID:     "test",
		Status: state.BatchPending,
		Scopes: []string{"test"},
		Findings: []state.Finding{
			{ID: "f-1", Title: "Done", Status: state.FindingApproved},
		},
	}
	batchPath := filepath.Join(env.WolfcastleDir, "audit-state.json")
	_ = state.SaveBatch(batchPath, batch)

	// Write invalid history file
	historyPath := filepath.Join(env.WolfcastleDir, "audit-review-history.json")
	_ = os.WriteFile(historyPath, []byte("broken json"), 0644)

	err := finalizeBatchIfComplete(env.App, batch, batchPath)
	if err == nil {
		t.Error("expected error when history file has invalid JSON")
	}
}

// ---------------------------------------------------------------------------
// audit approve — approve with no-description finding
// ---------------------------------------------------------------------------

func TestApprove_NoDescriptionFinding(t *testing.T) {
	env := newTestEnv(t)

	batch := &state.Batch{
		ID:     "test",
		Status: state.BatchPending,
		Scopes: []string{"test"},
		Findings: []state.Finding{
			{ID: "f-1", Title: "Simple Finding", Status: state.FindingPending},
		},
	}
	_ = state.SaveBatch(filepath.Join(env.WolfcastleDir, "audit-state.json"), batch)

	env.RootCmd.SetArgs([]string{"audit", "approve", "f-1"})
	if err := env.RootCmd.Execute(); err != nil {
		t.Fatalf("approve no-desc finding failed: %v", err)
	}
}

// ---------------------------------------------------------------------------
// discoverScopes — local tier overrides
// ---------------------------------------------------------------------------

func TestDiscoverScopes_LocalOverride(t *testing.T) {
	env := newTestEnv(t)

	baseAudits := filepath.Join(env.WolfcastleDir, "system", "base", "audits")
	_ = os.MkdirAll(baseAudits, 0755)
	_ = os.WriteFile(filepath.Join(baseAudits, "test.md"), []byte("base test"), 0644)

	localAudits := filepath.Join(env.WolfcastleDir, "system", "local", "audits")
	_ = os.MkdirAll(localAudits, 0755)
	_ = os.WriteFile(filepath.Join(localAudits, "test.md"), []byte("local test"), 0644)

	scopes, err := discoverScopes(env.App)
	if err != nil {
		t.Fatalf("discoverScopes failed: %v", err)
	}
	if len(scopes) != 1 {
		t.Fatalf("expected 1 scope (local overrides base), got %d", len(scopes))
	}
	if scopes[0].PromptFile != filepath.Join(localAudits, "test.md") {
		t.Errorf("expected local prompt file, got %s", scopes[0].PromptFile)
	}
}

// ---------------------------------------------------------------------------
// audit history — LoadHistory error (invalid JSON)
// ---------------------------------------------------------------------------

func TestHistory_BrokenHistoryFile(t *testing.T) {
	env := newTestEnv(t)

	historyPath := filepath.Join(env.WolfcastleDir, "audit-review-history.json")
	_ = os.WriteFile(historyPath, []byte("broken json"), 0644)

	env.RootCmd.SetArgs([]string{"audit", "history"})
	err := env.RootCmd.Execute()
	if err == nil {
		t.Error("expected error when history file has invalid JSON")
	}
}

// ---------------------------------------------------------------------------
// discoverScopes — skip directories and non-.md files
// ---------------------------------------------------------------------------

func TestDiscoverScopes_SkipsDirsAndNonMd(t *testing.T) {
	env := newTestEnv(t)

	baseAudits := filepath.Join(env.WolfcastleDir, "system", "base", "audits")
	_ = os.MkdirAll(baseAudits, 0755)
	_ = os.WriteFile(filepath.Join(baseAudits, "valid.md"), []byte("# Valid\nA valid scope"), 0644)
	_ = os.WriteFile(filepath.Join(baseAudits, "notes.txt"), []byte("not a scope"), 0644)
	_ = os.MkdirAll(filepath.Join(baseAudits, "subdir"), 0755)

	scopes, err := discoverScopes(env.App)
	if err != nil {
		t.Fatalf("discoverScopes failed: %v", err)
	}
	if len(scopes) != 1 {
		t.Fatalf("expected 1 scope (skip dir and .txt), got %d", len(scopes))
	}
}

// ---------------------------------------------------------------------------
// discoverScopes — description from file content (non-heading line)
// ---------------------------------------------------------------------------

func TestDiscoverScopes_DescriptionFromContent(t *testing.T) {
	env := newTestEnv(t)

	baseAudits := filepath.Join(env.WolfcastleDir, "system", "base", "audits")
	_ = os.MkdirAll(baseAudits, 0755)
	_ = os.WriteFile(filepath.Join(baseAudits, "security.md"),
		[]byte("# Security Audit\nCheck for vulnerabilities and security issues"), 0644)

	scopes, err := discoverScopes(env.App)
	if err != nil {
		t.Fatalf("discoverScopes failed: %v", err)
	}
	if len(scopes) != 1 {
		t.Fatalf("expected 1 scope, got %d", len(scopes))
	}
	if scopes[0].Description != "Check for vulnerabilities and security issues" {
		t.Errorf("unexpected description: %s", scopes[0].Description)
	}
}

// ---------------------------------------------------------------------------
// audit run — scope flag with valid scope
// ---------------------------------------------------------------------------

// ---------------------------------------------------------------------------
// audit run --list with scopes (human output for each scope)
// ---------------------------------------------------------------------------

func TestRunCmd_ListFlagWithScopes(t *testing.T) {
	env := newTestEnv(t)

	baseAudits := filepath.Join(env.WolfcastleDir, "system", "base", "audits")
	_ = os.MkdirAll(baseAudits, 0755)
	_ = os.WriteFile(filepath.Join(baseAudits, "security.md"), []byte("# Security\nCheck for vulnerabilities"), 0644)
	_ = os.WriteFile(filepath.Join(baseAudits, "performance.md"), []byte("# Performance\nCheck bottlenecks"), 0644)

	env.RootCmd.SetArgs([]string{"audit", "run", "--list"})
	if err := env.RootCmd.Execute(); err != nil {
		t.Fatalf("audit run --list with scopes failed: %v", err)
	}
}

func TestRunCmd_ScopeFlag(t *testing.T) {
	env := newTestEnv(t)

	baseAudits := filepath.Join(env.WolfcastleDir, "system", "base", "audits")
	_ = os.MkdirAll(baseAudits, 0755)
	_ = os.WriteFile(filepath.Join(baseAudits, "security.md"), []byte("Check security"), 0644)
	_ = os.WriteFile(filepath.Join(baseAudits, "perf.md"), []byte("Check perf"), 0644)

	// Run with --list to see scopes work
	env.RootCmd.SetArgs([]string{"audit", "run", "--list"})
	if err := env.RootCmd.Execute(); err != nil {
		t.Fatalf("audit run --list failed: %v", err)
	}
}

func TestAuditList_NoScopes(t *testing.T) {
	env := newTestEnv(t)

	env.RootCmd.SetArgs([]string{"audit", "list"})
	if err := env.RootCmd.Execute(); err != nil {
		t.Fatalf("audit list (no scopes) failed: %v", err)
	}
}
