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
