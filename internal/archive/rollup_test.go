package archive

import (
	"strings"
	"testing"
	"time"

	"github.com/dorkusprime/wolfcastle/internal/config"
	"github.com/dorkusprime/wolfcastle/internal/state"
)

func TestGenerateEntry_ProducesValidMarkdown(t *testing.T) {
	t.Parallel()
	ns := state.NewNodeState("auth", "Auth Module", state.NodeLeaf)
	ns.State = state.StatusComplete
	cfg := config.Defaults()
	cfg.Identity = &config.IdentityConfig{User: "dev", Machine: "laptop"}

	entry := GenerateEntry("project/auth", ns, cfg, "main", "")

	if entry == nil {
		t.Fatal("expected non-nil entry")
	}
	if !strings.HasSuffix(entry.Filename, "-complete.md") {
		t.Errorf("expected filename ending in -complete.md, got %q", entry.Filename)
	}
	if !strings.Contains(entry.Content, "# Archive: project/auth") {
		t.Error("expected archive header")
	}
}

func TestGenerateEntry_IncludesSummary(t *testing.T) {
	t.Parallel()
	ns := state.NewNodeState("auth", "Auth Module", state.NodeLeaf)
	cfg := config.Defaults()
	cfg.Identity = &config.IdentityConfig{User: "dev", Machine: "laptop"}

	entry := GenerateEntry("project/auth", ns, cfg, "main", "This is the summary text")

	if !strings.Contains(entry.Content, "## Summary") {
		t.Error("expected summary section")
	}
	if !strings.Contains(entry.Content, "This is the summary text") {
		t.Error("expected summary content")
	}
}

func TestGenerateEntry_OmitsSummaryWhenEmpty(t *testing.T) {
	t.Parallel()
	ns := state.NewNodeState("auth", "Auth Module", state.NodeLeaf)
	cfg := config.Defaults()
	cfg.Identity = &config.IdentityConfig{User: "dev", Machine: "laptop"}

	entry := GenerateEntry("project/auth", ns, cfg, "main", "")

	if strings.Contains(entry.Content, "## Summary") {
		t.Error("expected no summary section when empty")
	}
}

func TestGenerateEntry_IncludesBreadcrumbs(t *testing.T) {
	t.Parallel()
	ns := state.NewNodeState("auth", "Auth Module", state.NodeLeaf)
	ns.Audit.Breadcrumbs = []state.Breadcrumb{
		{Timestamp: time.Date(2026, 3, 1, 10, 0, 0, 0, time.UTC), Task: "task-1", Text: "Added JWT middleware"},
		{Timestamp: time.Date(2026, 3, 1, 11, 0, 0, 0, time.UTC), Task: "task-2", Text: "Updated tests"},
	}
	cfg := config.Defaults()
	cfg.Identity = &config.IdentityConfig{User: "dev", Machine: "laptop"}

	entry := GenerateEntry("project/auth", ns, cfg, "main", "")

	if !strings.Contains(entry.Content, "## Breadcrumbs") {
		t.Error("expected breadcrumbs section")
	}
	if !strings.Contains(entry.Content, "Added JWT middleware") {
		t.Error("expected first breadcrumb")
	}
	if !strings.Contains(entry.Content, "Updated tests") {
		t.Error("expected second breadcrumb")
	}
}

func TestGenerateEntry_IncludesAuditGaps(t *testing.T) {
	t.Parallel()
	ns := state.NewNodeState("auth", "Auth Module", state.NodeLeaf)
	ns.Audit.Gaps = []state.Gap{
		{ID: "g1", Description: "Missing error handling", Status: "open"},
		{ID: "g2", Description: "No input validation", Status: "fixed", FixedBy: "task-2"},
	}
	cfg := config.Defaults()
	cfg.Identity = &config.IdentityConfig{User: "dev", Machine: "laptop"}

	entry := GenerateEntry("project/auth", ns, cfg, "main", "")

	if !strings.Contains(entry.Content, "### Gaps") {
		t.Error("expected gaps section")
	}
	if !strings.Contains(entry.Content, "Missing error handling") {
		t.Error("expected open gap")
	}
	if !strings.Contains(entry.Content, "FIXED") {
		t.Error("expected fixed gap status")
	}
	if !strings.Contains(entry.Content, "fixed by task-2") {
		t.Error("expected fixed-by attribution")
	}
}

func TestGenerateEntry_IncludesMetadata(t *testing.T) {
	t.Parallel()
	ns := state.NewNodeState("auth", "Auth Module", state.NodeLeaf)
	cfg := config.Defaults()
	cfg.Identity = &config.IdentityConfig{User: "dev", Machine: "laptop"}

	entry := GenerateEntry("project/auth", ns, cfg, "feature-branch", "")

	if !strings.Contains(entry.Content, "## Metadata") {
		t.Error("expected metadata section")
	}
	if !strings.Contains(entry.Content, "| Node | project/auth |") {
		t.Error("expected node in metadata")
	}
	if !strings.Contains(entry.Content, "| Engineer | dev-laptop |") {
		t.Error("expected engineer in metadata")
	}
	if !strings.Contains(entry.Content, "| Branch | feature-branch |") {
		t.Error("expected branch in metadata")
	}
}

func TestGenerateEntry_NilIdentity(t *testing.T) {
	t.Parallel()
	ns := state.NewNodeState("auth", "Auth Module", state.NodeLeaf)
	cfg := config.Defaults()
	cfg.Identity = nil

	entry := GenerateEntry("project/auth", ns, cfg, "main", "")

	if entry == nil {
		t.Fatal("expected non-nil entry even with nil identity")
	}
	if strings.Contains(entry.Content, "| Engineer |") {
		t.Error("engineer row should be absent when identity is nil")
	}
	// Other sections should still be present
	if !strings.Contains(entry.Content, "## Metadata") {
		t.Error("expected metadata section")
	}
	if !strings.Contains(entry.Content, "| Node | project/auth |") {
		t.Error("expected node in metadata")
	}
}

func TestGenerateEntry_EmptyBranch(t *testing.T) {
	t.Parallel()
	ns := state.NewNodeState("auth", "Auth Module", state.NodeLeaf)
	cfg := config.Defaults()
	cfg.Identity = &config.IdentityConfig{User: "dev", Machine: "laptop"}

	entry := GenerateEntry("project/auth", ns, cfg, "", "")

	if strings.Contains(entry.Content, "| Branch |") {
		t.Error("branch row should be absent when branch is empty")
	}
}

func TestGenerateEntry_Escalations_OpenAndResolved(t *testing.T) {
	t.Parallel()
	ns := state.NewNodeState("auth", "Auth Module", state.NodeLeaf)
	ns.Audit.Escalations = []state.Escalation{
		{
			ID:          "e1",
			Timestamp:   time.Date(2026, 3, 1, 10, 0, 0, 0, time.UTC),
			Description: "API contract mismatch",
			SourceNode:  "project/api",
			Status:      "open",
		},
		{
			ID:          "e2",
			Timestamp:   time.Date(2026, 3, 2, 10, 0, 0, 0, time.UTC),
			Description: "Missing rate limiting",
			SourceNode:  "project/gateway",
			Status:      "resolved",
			ResolvedBy:  "task-5",
		},
	}
	cfg := config.Defaults()
	cfg.Identity = &config.IdentityConfig{User: "dev", Machine: "laptop"}

	entry := GenerateEntry("project/auth", ns, cfg, "main", "")

	if !strings.Contains(entry.Content, "### Escalations") {
		t.Error("expected escalations section")
	}
	if !strings.Contains(entry.Content, "[OPEN] API contract mismatch (from project/api)") {
		t.Error("expected open escalation")
	}
	if !strings.Contains(entry.Content, "[RESOLVED] Missing rate limiting (from project/gateway)") {
		t.Error("expected resolved escalation")
	}
}

func TestGenerateEntry_NoEscalations(t *testing.T) {
	t.Parallel()
	ns := state.NewNodeState("auth", "Auth Module", state.NodeLeaf)
	cfg := config.Defaults()
	cfg.Identity = &config.IdentityConfig{User: "dev", Machine: "laptop"}

	entry := GenerateEntry("project/auth", ns, cfg, "main", "")

	if strings.Contains(entry.Content, "### Escalations") {
		t.Error("escalations section should be absent when empty")
	}
}

func TestGenerateEntry_ScopeWithAllFields(t *testing.T) {
	t.Parallel()
	ns := state.NewNodeState("auth", "Auth Module", state.NodeLeaf)
	ns.Audit.Scope = &state.AuditScope{
		Description: "Verify authentication endpoints",
		Files:       []string{"auth.go", "middleware.go"},
		Systems:     []string{"auth-service", "token-service"},
		Criteria:    []string{"JWT tokens validated", "Rate limiting enforced", "Error codes documented"},
	}
	cfg := config.Defaults()
	cfg.Identity = &config.IdentityConfig{User: "dev", Machine: "laptop"}

	entry := GenerateEntry("project/auth", ns, cfg, "main", "")

	if !strings.Contains(entry.Content, "### Scope") {
		t.Error("expected scope section")
	}
	if !strings.Contains(entry.Content, "Verify authentication endpoints") {
		t.Error("expected scope description")
	}
	if !strings.Contains(entry.Content, "**Criteria:**") {
		t.Error("expected criteria header")
	}
	if !strings.Contains(entry.Content, "- [x] JWT tokens validated") {
		t.Error("expected first criterion")
	}
	if !strings.Contains(entry.Content, "- [x] Rate limiting enforced") {
		t.Error("expected second criterion")
	}
	if !strings.Contains(entry.Content, "- [x] Error codes documented") {
		t.Error("expected third criterion")
	}
}

func TestGenerateEntry_ScopeWithNoCriteria(t *testing.T) {
	t.Parallel()
	ns := state.NewNodeState("auth", "Auth Module", state.NodeLeaf)
	ns.Audit.Scope = &state.AuditScope{
		Description: "Basic scope check",
	}
	cfg := config.Defaults()
	cfg.Identity = &config.IdentityConfig{User: "dev", Machine: "laptop"}

	entry := GenerateEntry("project/auth", ns, cfg, "main", "")

	if !strings.Contains(entry.Content, "### Scope") {
		t.Error("expected scope section")
	}
	if !strings.Contains(entry.Content, "Basic scope check") {
		t.Error("expected scope description")
	}
	if strings.Contains(entry.Content, "**Criteria:**") {
		t.Error("criteria section should be absent when empty")
	}
}

func TestGenerateEntry_ResultSummary(t *testing.T) {
	t.Parallel()
	ns := state.NewNodeState("auth", "Auth Module", state.NodeLeaf)
	ns.Audit.ResultSummary = "All tests passed. Coverage at 95%."
	cfg := config.Defaults()
	cfg.Identity = &config.IdentityConfig{User: "dev", Machine: "laptop"}

	entry := GenerateEntry("project/auth", ns, cfg, "main", "")

	if !strings.Contains(entry.Content, "### Result") {
		t.Error("expected result section")
	}
	if !strings.Contains(entry.Content, "All tests passed. Coverage at 95%.") {
		t.Error("expected result summary content")
	}
}

func TestGenerateEntry_NoResultSummary(t *testing.T) {
	t.Parallel()
	ns := state.NewNodeState("auth", "Auth Module", state.NodeLeaf)
	cfg := config.Defaults()
	cfg.Identity = &config.IdentityConfig{User: "dev", Machine: "laptop"}

	entry := GenerateEntry("project/auth", ns, cfg, "main", "")

	if strings.Contains(entry.Content, "### Result") {
		t.Error("result section should be absent when result summary is empty")
	}
}

func TestGenerateEntry_NoBreadcrumbs(t *testing.T) {
	t.Parallel()
	ns := state.NewNodeState("auth", "Auth Module", state.NodeLeaf)
	cfg := config.Defaults()
	cfg.Identity = &config.IdentityConfig{User: "dev", Machine: "laptop"}

	entry := GenerateEntry("project/auth", ns, cfg, "main", "")

	if !strings.Contains(entry.Content, "## Breadcrumbs") {
		t.Error("breadcrumbs section should always be present")
	}
	if !strings.Contains(entry.Content, "No breadcrumbs recorded.") {
		t.Error("expected 'no breadcrumbs' message")
	}
}

func TestGenerateEntry_AuditStatus(t *testing.T) {
	t.Parallel()
	ns := state.NewNodeState("auth", "Auth Module", state.NodeLeaf)
	ns.Audit.Status = state.AuditPassed
	cfg := config.Defaults()
	cfg.Identity = &config.IdentityConfig{User: "dev", Machine: "laptop"}

	entry := GenerateEntry("project/auth", ns, cfg, "main", "")

	if !strings.Contains(entry.Content, "## Audit") {
		t.Error("expected audit section")
	}
	if !strings.Contains(entry.Content, "**Status:** passed") {
		t.Error("expected audit status")
	}
}

func TestGenerateEntry_FilenameSlugifiesAddress(t *testing.T) {
	t.Parallel()
	ns := state.NewNodeState("deep", "Deep Node", state.NodeLeaf)
	cfg := config.Defaults()
	cfg.Identity = &config.IdentityConfig{User: "dev", Machine: "laptop"}

	entry := GenerateEntry("project/module/deep", ns, cfg, "main", "")

	if !strings.Contains(entry.Filename, "project-module-deep-complete.md") {
		t.Errorf("expected slugified filename, got %q", entry.Filename)
	}
}

func TestGenerateEntry_NoGaps(t *testing.T) {
	t.Parallel()
	ns := state.NewNodeState("auth", "Auth Module", state.NodeLeaf)
	cfg := config.Defaults()
	cfg.Identity = &config.IdentityConfig{User: "dev", Machine: "laptop"}

	entry := GenerateEntry("project/auth", ns, cfg, "main", "")

	if strings.Contains(entry.Content, "### Gaps") {
		t.Error("gaps section should be absent when empty")
	}
}

func TestGenerateEntry_GapOpenWithoutFixedBy(t *testing.T) {
	t.Parallel()
	ns := state.NewNodeState("auth", "Auth Module", state.NodeLeaf)
	ns.Audit.Gaps = []state.Gap{
		{ID: "g1", Description: "Missing validation", Status: "open"},
	}
	cfg := config.Defaults()
	cfg.Identity = &config.IdentityConfig{User: "dev", Machine: "laptop"}

	entry := GenerateEntry("project/auth", ns, cfg, "main", "")

	if !strings.Contains(entry.Content, "[OPEN] Missing validation") {
		t.Error("expected open gap")
	}
	if strings.Contains(entry.Content, "(fixed by") {
		t.Error("should not contain fixed-by for open gap")
	}
}
