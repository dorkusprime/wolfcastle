package archive

import (
	"strings"
	"testing"
	"time"

	"github.com/dorkusprime/wolfcastle/internal/clock"
	"github.com/dorkusprime/wolfcastle/internal/config"
	"github.com/dorkusprime/wolfcastle/internal/state"
)

// ═══════════════════════════════════════════════════════════════════════════
// resolveOptionalClock
// ═══════════════════════════════════════════════════════════════════════════

func TestResolveOptionalClock_NilSlice(t *testing.T) {
	t.Parallel()
	clk := resolveOptionalClock(nil)
	if clk == nil {
		t.Error("expected non-nil clock for nil slice")
	}
	// Should be real clock — just verify it returns a time
	now := clk.Now()
	if now.IsZero() {
		t.Error("expected non-zero time from real clock")
	}
}

func TestResolveOptionalClock_EmptySlice(t *testing.T) {
	t.Parallel()
	clk := resolveOptionalClock([]clock.Clock{})
	if clk == nil {
		t.Error("expected non-nil clock for empty slice")
	}
}

func TestResolveOptionalClock_NilClockInSlice(t *testing.T) {
	t.Parallel()
	// Pass a nil clock value in the slice
	clk := resolveOptionalClock([]clock.Clock{nil})
	if clk == nil {
		t.Error("expected non-nil fallback clock when nil value in slice")
	}
}

func TestResolveOptionalClock_ValidClock(t *testing.T) {
	t.Parallel()
	fc := fakeClock{}
	clk := resolveOptionalClock([]clock.Clock{fc})
	if clk == nil {
		t.Fatal("expected non-nil clock")
	}
	if !clk.Now().Equal(fc.Now()) {
		t.Error("expected the provided clock to be returned")
	}
}

// ═══════════════════════════════════════════════════════════════════════════
// collapseHyphens
// ═══════════════════════════════════════════════════════════════════════════

func TestCollapseHyphens_NoHyphens(t *testing.T) {
	t.Parallel()
	if got := collapseHyphens("abc"); got != "abc" {
		t.Errorf("expected abc, got %q", got)
	}
}

func TestCollapseHyphens_SingleHyphen(t *testing.T) {
	t.Parallel()
	if got := collapseHyphens("a-b"); got != "a-b" {
		t.Errorf("expected a-b, got %q", got)
	}
}

func TestCollapseHyphens_MultipleConsecutive(t *testing.T) {
	t.Parallel()
	if got := collapseHyphens("a---b--c"); got != "a-b-c" {
		t.Errorf("expected a-b-c, got %q", got)
	}
}

func TestCollapseHyphens_AllHyphens(t *testing.T) {
	t.Parallel()
	if got := collapseHyphens("---"); got != "-" {
		t.Errorf("expected -, got %q", got)
	}
}

func TestCollapseHyphens_EmptyString(t *testing.T) {
	t.Parallel()
	if got := collapseHyphens(""); got != "" {
		t.Errorf("expected empty, got %q", got)
	}
}

func TestCollapseHyphens_LeadingTrailing(t *testing.T) {
	t.Parallel()
	if got := collapseHyphens("--a--b--"); got != "-a-b-" {
		t.Errorf("expected -a-b-, got %q", got)
	}
}

// ═══════════════════════════════════════════════════════════════════════════
// GenerateEntry — long slug truncation
// ═══════════════════════════════════════════════════════════════════════════

func TestGenerateEntry_LongSlugTruncation(t *testing.T) {
	t.Parallel()
	ns := state.NewNodeState("deep", "Deep Node", state.NodeLeaf)
	cfg := config.Defaults()
	cfg.Identity = &config.IdentityConfig{User: "dev", Machine: "laptop"}

	// Create an address longer than 80 chars after slugification
	longAddr := "project/" + strings.Repeat("very-long-segment/", 6) + "final"
	entry := GenerateEntry(longAddr, ns, cfg, "main", "", fakeClock{})

	slug := strings.TrimSuffix(entry.Filename, ".md")
	// Remove the timestamp prefix
	parts := strings.SplitN(slug, "-", 5)
	if len(parts) >= 5 {
		slugPart := parts[4]
		if len(slugPart) > 80 {
			t.Errorf("slug should be truncated to <=80 chars, got %d: %q", len(slugPart), slugPart)
		}
	}
}

func TestGenerateEntry_LongSlugNoHyphenBoundary(t *testing.T) {
	t.Parallel()
	ns := state.NewNodeState("deep", "Deep Node", state.NodeLeaf)
	cfg := config.Defaults()
	cfg.Identity = &config.IdentityConfig{User: "dev", Machine: "laptop"}

	// Create an address with no hyphens but very long
	longAddr := strings.Repeat("a", 100)
	entry := GenerateEntry(longAddr, ns, cfg, "main", "", fakeClock{})

	if entry == nil {
		t.Fatal("expected non-nil entry for long address")
	}
	if !strings.HasSuffix(entry.Filename, ".md") {
		t.Error("expected .md suffix")
	}
}

func TestGenerateEntry_CompletedAtTimestamp(t *testing.T) {
	t.Parallel()
	ns := state.NewNodeState("auth", "Auth", state.NodeLeaf)
	completedAt := time.Date(2026, 2, 15, 10, 30, 0, 0, time.UTC)
	ns.Audit.CompletedAt = &completedAt
	cfg := config.Defaults()
	cfg.Identity = &config.IdentityConfig{User: "dev", Machine: "laptop"}

	entry := GenerateEntry("project/auth", ns, cfg, "main", "", fakeClock{})

	if !strings.Contains(entry.Content, "2026-02-15T10:30Z") {
		t.Error("expected completedAt timestamp in metadata")
	}
}

func TestGenerateEntry_ConsecutiveHyphensInAddress(t *testing.T) {
	t.Parallel()
	ns := state.NewNodeState("deep", "Deep Node", state.NodeLeaf)
	cfg := config.Defaults()
	cfg.Identity = &config.IdentityConfig{User: "dev", Machine: "laptop"}

	// Address with segments that produce consecutive hyphens
	entry := GenerateEntry("project//double", ns, cfg, "main", "", fakeClock{})

	if entry == nil {
		t.Fatal("expected non-nil entry")
	}
	// The filename slug should collapse consecutive hyphens
	if strings.Contains(entry.Filename, "--") {
		t.Errorf("filename should not contain consecutive hyphens: %q", entry.Filename)
	}
}

// ═══════════════════════════════════════════════════════════════════════════
// GenerateEntry — various node shapes
// ═══════════════════════════════════════════════════════════════════════════

func TestGenerateEntry_OrchestratorNode(t *testing.T) {
	t.Parallel()
	ns := state.NewNodeState("orch", "Orchestrator", state.NodeOrchestrator)
	ns.State = state.StatusComplete
	ns.Children = []state.ChildRef{
		{ID: "child-1", Address: "orch/child-1", State: state.StatusComplete},
	}
	cfg := config.Defaults()
	cfg.Identity = &config.IdentityConfig{User: "dev", Machine: "laptop"}

	entry := GenerateEntry("project/orch", ns, cfg, "main", "Orchestrator complete", fakeClock{})

	if entry == nil {
		t.Fatal("expected non-nil entry for orchestrator")
	}
	if !strings.Contains(entry.Content, "# Archive: project/orch") {
		t.Error("expected archive header for orchestrator")
	}
	if !strings.Contains(entry.Content, "## Summary") {
		t.Error("expected summary section")
	}
}

func TestGenerateEntry_AllAuditFields(t *testing.T) {
	t.Parallel()
	ns := state.NewNodeState("full", "Full Node", state.NodeLeaf)
	ns.State = state.StatusComplete
	ns.Audit.Status = state.AuditPassed
	ns.Audit.ResultSummary = "Everything checks out"
	ns.Audit.Scope = &state.AuditScope{
		Description: "Full coverage",
		Files:       []string{"main.go"},
		Systems:     []string{"api"},
		Criteria:    []string{"Tests pass", "Lint clean"},
	}
	ns.Audit.Breadcrumbs = []state.Breadcrumb{
		{Timestamp: time.Date(2026, 3, 1, 10, 0, 0, 0, time.UTC), Task: "task-0001", Text: "First step"},
	}
	ns.Audit.Gaps = []state.Gap{
		{ID: "g1", Description: "Missing edge case", Status: state.GapFixed, FixedBy: "task-0002"},
	}
	ns.Audit.Escalations = []state.Escalation{
		{ID: "e1", Description: "API mismatch", SourceNode: "other", Status: state.EscalationResolved, ResolvedBy: "task-0003"},
	}
	completedAt := time.Date(2026, 3, 10, 14, 0, 0, 0, time.UTC)
	ns.Audit.CompletedAt = &completedAt

	cfg := config.Defaults()
	cfg.Identity = &config.IdentityConfig{User: "dev", Machine: "laptop"}

	entry := GenerateEntry("project/full", ns, cfg, "feature", "All done", fakeClock{})

	checks := []string{
		"# Archive: project/full",
		"## Summary",
		"All done",
		"## Breadcrumbs",
		"First step",
		"## Audit",
		"**Status:** passed",
		"### Scope",
		"Full coverage",
		"**Criteria:**",
		"Tests pass",
		"### Gaps",
		"FIXED",
		"fixed by task-0002",
		"### Escalations",
		"RESOLVED",
		"### Result",
		"Everything checks out",
		"## Metadata",
		"| Engineer | dev-laptop |",
		"| Branch | feature |",
		"2026-03-10T14:00Z",
	}
	for _, check := range checks {
		if !strings.Contains(entry.Content, check) {
			t.Errorf("expected content to contain %q", check)
		}
	}
}

// ═══════════════════════════════════════════════════════════════════════════
// fakeClock helper
// ═══════════════════════════════════════════════════════════════════════════

type fakeClock struct{}

func (fakeClock) Now() time.Time {
	return time.Date(2026, 3, 14, 12, 0, 0, 0, time.UTC)
}
