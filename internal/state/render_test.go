package state

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func TestRenderContext_BasicFields(t *testing.T) {
	t.Parallel()
	task := Task{
		ID:          "task-0001",
		Description: "Implement JWT validation",
		State:       StatusInProgress,
	}

	result := task.RenderContext("project/auth", "")

	if !strings.Contains(result, "**Task:** project/auth/task-0001") {
		t.Error("expected task address")
	}
	if !strings.Contains(result, "**Description:** Implement JWT validation") {
		t.Error("expected description")
	}
	if !strings.Contains(result, "**Task State:** in_progress") {
		t.Error("expected task state")
	}
}

func TestRenderContext_TaskType(t *testing.T) {
	t.Parallel()
	task := Task{
		ID:       "task-0001",
		State:    StatusInProgress,
		TaskType: "implementation",
	}

	result := task.RenderContext("node", "")

	if !strings.Contains(result, "**Task Type:** implementation") {
		t.Error("expected task type")
	}
}

func TestRenderContext_TaskTypeOmittedWhenEmpty(t *testing.T) {
	t.Parallel()
	task := Task{ID: "task-0001", State: StatusInProgress}

	result := task.RenderContext("node", "")

	if strings.Contains(result, "**Task Type:**") {
		t.Error("task type should be omitted when empty")
	}
}

func TestRenderContext_Body(t *testing.T) {
	t.Parallel()
	task := Task{
		ID:    "task-0001",
		State: StatusInProgress,
		Body:  "Detailed instructions for the task.",
	}

	result := task.RenderContext("node", "")

	if !strings.Contains(result, "## Task Details") {
		t.Error("expected task details section")
	}
	if !strings.Contains(result, "Detailed instructions for the task.") {
		t.Error("expected body content")
	}
}

func TestRenderContext_Integration(t *testing.T) {
	t.Parallel()
	task := Task{
		ID:          "task-0001",
		State:       StatusInProgress,
		Integration: "Must integrate with the auth middleware.",
	}

	result := task.RenderContext("node", "")

	if !strings.Contains(result, "## Integration") {
		t.Error("expected integration section")
	}
	if !strings.Contains(result, "Must integrate with the auth middleware.") {
		t.Error("expected integration content")
	}
}

func TestRenderContext_Deliverables(t *testing.T) {
	t.Parallel()
	task := Task{
		ID:           "task-0001",
		State:        StatusInProgress,
		Deliverables: []string{"internal/auth/jwt.go", "internal/auth/jwt_test.go"},
	}

	result := task.RenderContext("node", "")

	if !strings.Contains(result, "**Deliverables:**") {
		t.Error("expected deliverables section")
	}
	if !strings.Contains(result, "- `internal/auth/jwt.go`") {
		t.Error("expected first deliverable")
	}
	if !strings.Contains(result, "- `internal/auth/jwt_test.go`") {
		t.Error("expected second deliverable")
	}
}

func TestRenderContext_AcceptanceCriteria(t *testing.T) {
	t.Parallel()
	task := Task{
		ID:                 "task-0001",
		State:              StatusInProgress,
		AcceptanceCriteria: []string{"All tests pass", "No lint errors"},
	}

	result := task.RenderContext("node", "")

	if !strings.Contains(result, "**Acceptance Criteria:**") {
		t.Error("expected acceptance criteria section")
	}
	if !strings.Contains(result, "- All tests pass") {
		t.Error("expected first criterion")
	}
	if !strings.Contains(result, "- No lint errors") {
		t.Error("expected second criterion")
	}
}

func TestRenderContext_Constraints(t *testing.T) {
	t.Parallel()
	task := Task{
		ID:          "task-0001",
		State:       StatusInProgress,
		Constraints: []string{"No external dependencies", "Must be backward compatible"},
	}

	result := task.RenderContext("node", "")

	if !strings.Contains(result, "**Constraints:**") {
		t.Error("expected constraints section")
	}
	if !strings.Contains(result, "- No external dependencies") {
		t.Error("expected first constraint")
	}
}

func TestRenderContext_References(t *testing.T) {
	t.Parallel()
	task := Task{
		ID:         "task-0001",
		State:      StatusInProgress,
		References: []string{"docs/api-spec.txt", "docs/design.txt"},
	}

	result := task.RenderContext("node", "")

	if !strings.Contains(result, "**Reference Material:**") {
		t.Error("expected reference material section")
	}
	if !strings.Contains(result, "- `docs/api-spec.txt`") {
		t.Error("expected first reference")
	}
}

func TestRenderContext_ReferencesInlineMdContent(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	specPath := filepath.Join(dir, "spec.md")
	_ = os.WriteFile(specPath, []byte("# API Spec\n\nEndpoints listed here."), 0644)

	task := Task{
		ID:         "task-0001",
		State:      StatusInProgress,
		References: []string{specPath},
	}

	result := task.RenderContext("node", "")

	if !strings.Contains(result, "### Reference: "+specPath) {
		t.Error("expected inlined reference header")
	}
	if !strings.Contains(result, "# API Spec") {
		t.Error("expected inlined spec content")
	}
}

func TestRenderContext_ReferencesSkipLargeMdFiles(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	specPath := filepath.Join(dir, "huge.md")
	_ = os.WriteFile(specPath, []byte(strings.Repeat("x", 9000)), 0644)

	task := Task{
		ID:         "task-0001",
		State:      StatusInProgress,
		References: []string{specPath},
	}

	result := task.RenderContext("node", "")

	if strings.Contains(result, "### Reference:") {
		t.Error("large files should not be inlined")
	}
}

func TestRenderContext_FailureCount(t *testing.T) {
	t.Parallel()
	task := Task{
		ID:           "task-0001",
		State:        StatusInProgress,
		FailureCount: 7,
	}

	result := task.RenderContext("node", "")

	if !strings.Contains(result, "**Failure Count:** 7") {
		t.Error("expected failure count")
	}
}

func TestRenderContext_FailureCountOmittedWhenZero(t *testing.T) {
	t.Parallel()
	task := Task{ID: "task-0001", State: StatusInProgress}

	result := task.RenderContext("node", "")

	if strings.Contains(result, "**Failure Count:**") {
		t.Error("failure count should be omitted when zero")
	}
}

func TestRenderContext_LastFailureType_NoTerminalMarker(t *testing.T) {
	t.Parallel()
	task := Task{
		ID:              "task-0001",
		State:           StatusInProgress,
		FailureCount:    2,
		LastFailureType: "no_terminal_marker",
	}

	result := task.RenderContext("node", "")

	if !strings.Contains(result, "## Previous Attempt Failed") {
		t.Error("expected previous attempt failed section")
	}
	if !strings.Contains(result, "did not emit a terminal marker") {
		t.Error("expected no_terminal_marker explanation")
	}
}

func TestRenderContext_LastFailureType_NoProgress(t *testing.T) {
	t.Parallel()
	task := Task{
		ID:              "task-0001",
		State:           StatusInProgress,
		FailureCount:    1,
		LastFailureType: "no_progress",
	}

	result := task.RenderContext("node", "")

	if !strings.Contains(result, "no git changes were detected") {
		t.Error("expected no_progress explanation")
	}
}

func TestRenderContext_LastFailureType_Custom(t *testing.T) {
	t.Parallel()
	task := Task{
		ID:              "task-0001",
		State:           StatusInProgress,
		FailureCount:    1,
		LastFailureType: "timeout",
	}

	result := task.RenderContext("node", "")

	if !strings.Contains(result, "failed with reason: timeout") {
		t.Error("expected custom failure type")
	}
}

func TestRenderContext_NoFailureSection_WhenCountZero(t *testing.T) {
	t.Parallel()
	task := Task{
		ID:              "task-0001",
		State:           StatusInProgress,
		LastFailureType: "no_progress", // stale field, count is zero
	}

	result := task.RenderContext("node", "")

	if strings.Contains(result, "## Previous Attempt Failed") {
		t.Error("failure section should not appear when count is zero")
	}
}

func TestRenderContext_NodeDirMdFile(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	_ = os.WriteFile(filepath.Join(dir, "task-0001.md"), []byte("# Task Markdown\n\nExtra context here."), 0644)

	task := Task{
		ID:    "task-0001",
		State: StatusInProgress,
	}

	result := task.RenderContext("node", dir)

	if !strings.Contains(result, "# Task Markdown") {
		t.Error("expected task markdown content")
	}
	if !strings.Contains(result, "Extra context here.") {
		t.Error("expected task markdown body")
	}
}

func TestRenderContext_NodeDirMdFileMissing(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()

	task := Task{
		ID:    "task-0001",
		State: StatusInProgress,
	}

	result := task.RenderContext("node", dir)

	// Should still render without error
	if !strings.Contains(result, "**Task:** node/task-0001") {
		t.Error("expected task address even without .md file")
	}
}

func TestRenderContext_EmptyNodeDir(t *testing.T) {
	t.Parallel()
	task := Task{
		ID:    "task-0001",
		State: StatusInProgress,
	}

	result := task.RenderContext("node", "")

	if !strings.Contains(result, "**Task:** node/task-0001") {
		t.Error("expected task address with empty nodeDir")
	}
}

func TestRenderContext_AllOptionalFieldsEmpty(t *testing.T) {
	t.Parallel()
	task := Task{
		ID:          "task-0001",
		Description: "Minimal task",
		State:       StatusNotStarted,
	}

	result := task.RenderContext("proj", "")

	// Should contain only basic fields
	if !strings.Contains(result, "**Task:** proj/task-0001") {
		t.Error("expected task address")
	}
	if !strings.Contains(result, "**Task State:** not_started") {
		t.Error("expected task state")
	}

	// Should omit all optional sections
	for _, section := range []string{
		"**Task Type:**",
		"## Task Details",
		"## Integration",
		"**Deliverables:**",
		"**Acceptance Criteria:**",
		"**Constraints:**",
		"**Reference Material:**",
		"**Failure Count:**",
		"## Previous Attempt Failed",
	} {
		if strings.Contains(result, section) {
			t.Errorf("should not contain %q for minimal task", section)
		}
	}
}

func TestRenderContext_FullTask(t *testing.T) {
	t.Parallel()
	task := Task{
		ID:                 "task-0003",
		Description:        "Build the widget",
		State:              StatusInProgress,
		TaskType:           "implementation",
		Class:              "backend",
		Body:               "Build the widget using the factory pattern.",
		Integration:        "Connects to the widget store.",
		Deliverables:       []string{"pkg/widget.go"},
		AcceptanceCriteria: []string{"Unit tests pass"},
		Constraints:        []string{"No reflection"},
		References:         []string{"docs/widget-rfc.txt"},
		FailureCount:       2,
		LastFailureType:    "no_progress",
	}

	result := task.RenderContext("myproj/widgets", "")

	expected := []string{
		"**Task:** myproj/widgets/task-0003",
		"**Description:** Build the widget",
		"**Task Type:** implementation",
		"## Task Details",
		"Build the widget using the factory pattern.",
		"## Integration",
		"Connects to the widget store.",
		"- `pkg/widget.go`",
		"- Unit tests pass",
		"- No reflection",
		"- `docs/widget-rfc.txt`",
		"**Task State:** in_progress",
		"**Failure Count:** 2",
		"## Previous Attempt Failed",
		"no git changes were detected",
	}
	for _, s := range expected {
		if !strings.Contains(result, s) {
			t.Errorf("expected output to contain %q", s)
		}
	}
}

// --- AuditState.RenderContext tests ---

func TestAuditRenderContext_Empty(t *testing.T) {
	t.Parallel()
	audit := AuditState{}
	if audit.RenderContext() != "" {
		t.Error("empty audit state should return empty string")
	}
}

func TestAuditRenderContext_EmptyBreadcrumbsNilScope(t *testing.T) {
	t.Parallel()
	audit := AuditState{Breadcrumbs: []Breadcrumb{}}
	if audit.RenderContext() != "" {
		t.Error("empty breadcrumbs with nil scope should return empty string")
	}
}

func TestAuditRenderContext_BreadcrumbsOnly(t *testing.T) {
	t.Parallel()
	ts := time.Date(2026, 3, 15, 14, 30, 0, 0, time.UTC)
	audit := AuditState{
		Breadcrumbs: []Breadcrumb{
			{Timestamp: ts, Task: "task-0001", Text: "Added JWT validation"},
		},
	}

	result := audit.RenderContext()

	if !strings.Contains(result, "## Recent Breadcrumbs") {
		t.Error("expected breadcrumbs header")
	}
	if !strings.Contains(result, "- [2026-03-15T14:30Z] task-0001: Added JWT validation") {
		t.Error("expected formatted breadcrumb entry")
	}
	if strings.Contains(result, "## Audit Scope") {
		t.Error("scope section should not appear when scope is nil")
	}
}

func TestAuditRenderContext_ScopeOnly(t *testing.T) {
	t.Parallel()
	audit := AuditState{
		Scope: &AuditScope{Description: "Verify auth middleware"},
	}

	result := audit.RenderContext()

	if strings.Contains(result, "## Recent Breadcrumbs") {
		t.Error("breadcrumbs section should not appear when empty")
	}
	if !strings.Contains(result, "## Audit Scope") {
		t.Error("expected audit scope header")
	}
	if !strings.Contains(result, "Verify auth middleware") {
		t.Error("expected scope description")
	}
}

func TestAuditRenderContext_BreadcrumbsAndScope(t *testing.T) {
	t.Parallel()
	ts := time.Date(2026, 1, 10, 8, 0, 0, 0, time.UTC)
	audit := AuditState{
		Breadcrumbs: []Breadcrumb{
			{Timestamp: ts, Task: "task-0002", Text: "Refactored handler"},
		},
		Scope: &AuditScope{Description: "Check error handling"},
	}

	result := audit.RenderContext()

	if !strings.Contains(result, "## Recent Breadcrumbs") {
		t.Error("expected breadcrumbs header")
	}
	if !strings.Contains(result, "## Audit Scope") {
		t.Error("expected audit scope header")
	}
	// Breadcrumbs should come before scope
	bcIdx := strings.Index(result, "## Recent Breadcrumbs")
	scIdx := strings.Index(result, "## Audit Scope")
	if bcIdx >= scIdx {
		t.Error("breadcrumbs section should appear before scope section")
	}
}

func TestAuditRenderContext_LimitToLast10Breadcrumbs(t *testing.T) {
	t.Parallel()
	var crumbs []Breadcrumb
	base := time.Date(2026, 3, 1, 0, 0, 0, 0, time.UTC)
	for i := 0; i < 15; i++ {
		crumbs = append(crumbs, Breadcrumb{
			Timestamp: base.Add(time.Duration(i) * time.Hour),
			Task:      "task-0001",
			Text:      fmt.Sprintf("breadcrumb-%02d", i),
		})
	}
	audit := AuditState{Breadcrumbs: crumbs}

	result := audit.RenderContext()

	// First 5 (indices 0-4) should be excluded
	for i := 0; i < 5; i++ {
		marker := fmt.Sprintf("breadcrumb-%02d", i)
		if strings.Contains(result, marker) {
			t.Errorf("breadcrumb %d should have been trimmed", i)
		}
	}
	// Last 10 (indices 5-14) should be present
	for i := 5; i < 15; i++ {
		marker := fmt.Sprintf("breadcrumb-%02d", i)
		if !strings.Contains(result, marker) {
			t.Errorf("breadcrumb %d should be present", i)
		}
	}
}

func TestAuditRenderContext_Exactly10Breadcrumbs(t *testing.T) {
	t.Parallel()
	var crumbs []Breadcrumb
	base := time.Date(2026, 3, 1, 0, 0, 0, 0, time.UTC)
	for i := 0; i < 10; i++ {
		crumbs = append(crumbs, Breadcrumb{
			Timestamp: base.Add(time.Duration(i) * time.Hour),
			Task:      "task-0001",
			Text:      fmt.Sprintf("crumb-%02d", i),
		})
	}
	audit := AuditState{Breadcrumbs: crumbs}

	result := audit.RenderContext()

	// All 10 should be present
	for i := 0; i < 10; i++ {
		marker := fmt.Sprintf("crumb-%02d", i)
		if !strings.Contains(result, marker) {
			t.Errorf("breadcrumb %d should be present", i)
		}
	}
}
