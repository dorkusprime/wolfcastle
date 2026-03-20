package state

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func TestGenerateAuditReport_PassedWithNoGaps(t *testing.T) {
	t.Parallel()
	now := time.Date(2026, 3, 20, 14, 30, 0, 0, time.UTC)
	audit := AuditState{
		Status:        AuditPassed,
		CompletedAt:   &now,
		ResultSummary: "All checks passed. Code coverage at 95%.",
		Scope: &AuditScope{
			Description: "Full codebase review",
			Files:       []string{"internal/auth/", "internal/api/"},
			Criteria:    []string{"No hardcoded secrets", "Error handling present"},
		},
		Gaps:        []Gap{},
		Escalations: []Escalation{},
		Breadcrumbs: []Breadcrumb{},
	}

	report := GenerateAuditReport(audit, "my-project/auth", "auth")

	assertContains(t, report, "# Audit Report: auth")
	assertContains(t, report, "**Node:** `my-project/auth`")
	assertContains(t, report, "**Verdict:** PASSED")
	assertContains(t, report, "**Completed:** 2026-03-20T14:30:00Z")
	assertContains(t, report, "All checks passed")
	assertContains(t, report, "Full codebase review")
	assertContains(t, report, "internal/auth/, internal/api/")
	assertContains(t, report, "- No hardcoded secrets")
	assertNotContains(t, report, "## Findings")
	assertNotContains(t, report, "## Escalations")
}

func TestGenerateAuditReport_FailedWithGapsAndEscalations(t *testing.T) {
	t.Parallel()
	started := time.Date(2026, 3, 19, 10, 0, 0, 0, time.UTC)
	fixedAt := time.Date(2026, 3, 19, 11, 0, 0, 0, time.UTC)
	audit := AuditState{
		Status:        AuditFailed,
		StartedAt:     &started,
		ResultSummary: "Two issues found, one remediated.",
		Scope: &AuditScope{
			Description: "Security review",
			Systems:     []string{"authentication", "authorization"},
		},
		Gaps: []Gap{
			{
				ID:          "gap-001",
				Description: "SQL injection in login handler",
				Source:      "internal/auth/login.go:42",
				Status:      GapFixed,
				FixedBy:     "task-0002",
				FixedAt:     &fixedAt,
			},
			{
				ID:          "gap-002",
				Description: "Missing rate limiting on API endpoints",
				Source:      "internal/api/middleware.go",
				Status:      GapOpen,
			},
		},
		Escalations: []Escalation{
			{
				ID:          "esc-001",
				Description: "Architecture-level auth redesign needed",
				SourceNode:  "my-project/auth",
				Status:      EscalationOpen,
			},
		},
		Breadcrumbs: []Breadcrumb{
			{
				Timestamp: started,
				Task:      "task-0001",
				Text:      "Started security audit",
			},
		},
	}

	report := GenerateAuditReport(audit, "my-project", "my-project")

	assertContains(t, report, "**Verdict:** FAILED")
	assertContains(t, report, "**Started:** 2026-03-19T10:00:00Z")
	assertContains(t, report, "## Findings")
	assertContains(t, report, "2 total (1 remediated, 1 open)")
	assertContains(t, report, "### Open")
	assertContains(t, report, "gap-002")
	assertContains(t, report, "Missing rate limiting")
	assertContains(t, report, "### Remediated")
	assertContains(t, report, "gap-001")
	assertContains(t, report, "(fixed by task-0002)")
	assertContains(t, report, "## Escalations")
	assertContains(t, report, "esc-001")
	assertContains(t, report, "[OPEN]")
	assertContains(t, report, "## Audit Trail")
	assertContains(t, report, "Started security audit")
}

func TestGenerateAuditReport_MinimalState(t *testing.T) {
	t.Parallel()
	audit := AuditState{
		Status: AuditPending,
	}

	report := GenerateAuditReport(audit, "root", "root")

	assertContains(t, report, "# Audit Report: root")
	assertContains(t, report, "**Verdict:** PENDING")
	assertNotContains(t, report, "## Scope")
	assertNotContains(t, report, "## Summary")
	assertNotContains(t, report, "## Findings")
}

func TestGenerateAuditReport_InProgressVerdict(t *testing.T) {
	t.Parallel()
	audit := AuditState{Status: AuditInProgress}
	report := GenerateAuditReport(audit, "node", "node")
	assertContains(t, report, "**Verdict:** IN PROGRESS")
}

func TestWriteAuditReport_WritesFile(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	nodeAddr := "root/child"
	ts := time.Date(2026, 3, 20, 14, 30, 0, 0, time.UTC)
	audit := AuditState{
		Status:        AuditPassed,
		ResultSummary: "Everything looks good.",
	}

	path, err := WriteAuditReport(dir, nodeAddr, audit, "child", ts)
	if err != nil {
		t.Fatalf("WriteAuditReport: %v", err)
	}

	wantPath := filepath.Join(dir, "root", "child", "audit-2026-03-20T14-30.md")
	if path != wantPath {
		t.Errorf("path = %q, want %q", path, wantPath)
	}

	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("reading report: %v", err)
	}
	content := string(data)
	assertContains(t, content, "# Audit Report: child")
	assertContains(t, content, "**Verdict:** PASSED")
}

func TestWriteAuditReport_CreatesDirectory(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	ts := time.Date(2026, 1, 15, 9, 0, 0, 0, time.UTC)

	path, err := WriteAuditReport(dir, "deep/nested/node", AuditState{Status: AuditPending}, "node", ts)
	if err != nil {
		t.Fatalf("WriteAuditReport: %v", err)
	}

	if _, err := os.Stat(path); err != nil {
		t.Errorf("report file does not exist: %v", err)
	}
}

func TestLatestAuditReport_FindsMostRecent(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	nodeDir := filepath.Join(dir, "my-node")
	if err := os.MkdirAll(nodeDir, 0755); err != nil {
		t.Fatal(err)
	}

	// Write two reports with different timestamps
	for _, name := range []string{"audit-2026-03-18T10-00.md", "audit-2026-03-20T14-30.md", "audit-2026-03-19T12-00.md"} {
		if err := os.WriteFile(filepath.Join(nodeDir, name), []byte("report"), 0644); err != nil {
			t.Fatal(err)
		}
	}

	got := LatestAuditReport(dir, "my-node")
	want := filepath.Join(nodeDir, "audit-2026-03-20T14-30.md")
	if got != want {
		t.Errorf("LatestAuditReport = %q, want %q", got, want)
	}
}

func TestLatestAuditReport_EmptyDir(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	if err := os.MkdirAll(filepath.Join(dir, "empty"), 0755); err != nil {
		t.Fatal(err)
	}

	got := LatestAuditReport(dir, "empty")
	if got != "" {
		t.Errorf("LatestAuditReport on empty dir = %q, want empty", got)
	}
}

func TestLatestAuditReport_NonexistentDir(t *testing.T) {
	t.Parallel()
	got := LatestAuditReport("/nonexistent", "missing")
	if got != "" {
		t.Errorf("LatestAuditReport on missing dir = %q, want empty", got)
	}
}

func TestLatestAuditReport_IgnoresNonAuditFiles(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	nodeDir := filepath.Join(dir, "my-node")
	if err := os.MkdirAll(nodeDir, 0755); err != nil {
		t.Fatal(err)
	}

	// Write non-audit files
	for _, name := range []string{"state.json", "notes.md", "audit-report.txt"} {
		if err := os.WriteFile(filepath.Join(nodeDir, name), []byte("data"), 0644); err != nil {
			t.Fatal(err)
		}
	}

	got := LatestAuditReport(dir, "my-node")
	if got != "" {
		t.Errorf("LatestAuditReport = %q, want empty (no audit-*.md files)", got)
	}
}

func TestCountGaps(t *testing.T) {
	t.Parallel()
	gaps := []Gap{
		{Status: GapOpen},
		{Status: GapFixed},
		{Status: GapOpen},
		{Status: GapFixed},
		{Status: GapFixed},
	}
	open, fixed := countGaps(gaps)
	if open != 2 {
		t.Errorf("open = %d, want 2", open)
	}
	if fixed != 3 {
		t.Errorf("fixed = %d, want 3", fixed)
	}
}

func TestVerdictLabel(t *testing.T) {
	t.Parallel()
	tests := []struct {
		status AuditStatus
		want   string
	}{
		{AuditPassed, "PASSED"},
		{AuditFailed, "FAILED"},
		{AuditInProgress, "IN PROGRESS"},
		{AuditPending, "PENDING"},
		{AuditStatus("unknown"), "PENDING"},
	}
	for _, tt := range tests {
		got := verdictLabel(tt.status)
		if got != tt.want {
			t.Errorf("verdictLabel(%q) = %q, want %q", tt.status, got, tt.want)
		}
	}
}

func TestGenerateAuditReport_ScopeWithSystems(t *testing.T) {
	t.Parallel()
	audit := AuditState{
		Status: AuditPassed,
		Scope: &AuditScope{
			Description: "System integration review",
			Systems:     []string{"database", "cache"},
		},
	}

	report := GenerateAuditReport(audit, "node", "node")
	assertContains(t, report, "**Systems:** database, cache")
}

func TestGenerateAuditReport_ResolvedEscalation(t *testing.T) {
	t.Parallel()
	audit := AuditState{
		Status: AuditPassed,
		Escalations: []Escalation{
			{
				ID:          "esc-001",
				Description: "Fixed the thing",
				SourceNode:  "child-node",
				Status:      EscalationResolved,
			},
		},
	}

	report := GenerateAuditReport(audit, "parent", "parent")
	assertContains(t, report, "[RESOLVED]")
}

func assertContains(t *testing.T, s, substr string) {
	t.Helper()
	if !strings.Contains(s, substr) {
		t.Errorf("expected report to contain %q, but it did not.\nReport:\n%s", substr, s)
	}
}

func assertNotContains(t *testing.T, s, substr string) {
	t.Helper()
	if strings.Contains(s, substr) {
		t.Errorf("expected report NOT to contain %q, but it did.\nReport:\n%s", substr, s)
	}
}
