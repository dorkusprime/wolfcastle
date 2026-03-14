package pipeline

import (
	"strings"
	"testing"
	"time"

	"github.com/dorkusprime/wolfcastle/internal/config"
	"github.com/dorkusprime/wolfcastle/internal/state"
)

func TestBuildIterationContext_IncludesNodeInfo(t *testing.T) {
	t.Parallel()
	ns := state.NewNodeState("auth", "Auth Module", state.NodeLeaf)
	ns.State = state.StatusInProgress

	result := BuildIterationContext("project/auth", ns, "task-1")

	if !strings.Contains(result, "**Node:** project/auth") {
		t.Error("expected node address")
	}
	if !strings.Contains(result, "**Node Type:** leaf") {
		t.Error("expected node type")
	}
	if !strings.Contains(result, "**Node State:** in_progress") {
		t.Error("expected node state")
	}
}

func TestBuildIterationContext_IncludesTaskDescription(t *testing.T) {
	t.Parallel()
	ns := state.NewNodeState("auth", "Auth Module", state.NodeLeaf)
	ns.Tasks = []state.Task{
		{ID: "task-1", Description: "Implement JWT validation", State: state.StatusInProgress},
	}

	result := BuildIterationContext("project/auth", ns, "task-1")

	if !strings.Contains(result, "**Task:** project/auth/task-1") {
		t.Error("expected task address")
	}
	if !strings.Contains(result, "**Description:** Implement JWT validation") {
		t.Error("expected task description")
	}
}

func TestBuildIterationContext_BreadcrumbsLimitedTo10(t *testing.T) {
	t.Parallel()
	ns := state.NewNodeState("auth", "Auth Module", state.NodeLeaf)

	// Add 15 breadcrumbs
	for i := 0; i < 15; i++ {
		ns.Audit.Breadcrumbs = append(ns.Audit.Breadcrumbs, state.Breadcrumb{
			Timestamp: time.Date(2026, 1, 1, i, 0, 0, 0, time.UTC),
			Task:      "task-1",
			Text:      strings.Repeat("x", 1) + string(rune('a'+i)),
		})
	}

	result := BuildIterationContext("project/auth", ns, "task-1")

	if !strings.Contains(result, "## Recent Breadcrumbs") {
		t.Error("expected breadcrumbs section")
	}

	// Should only contain the last 10 breadcrumbs (indices 5-14)
	lines := strings.Split(result, "\n")
	breadcrumbLines := 0
	for _, line := range lines {
		if strings.HasPrefix(line, "- [") {
			breadcrumbLines++
		}
	}
	if breadcrumbLines != 10 {
		t.Errorf("expected 10 breadcrumb lines, got %d", breadcrumbLines)
	}
}

func TestBuildIterationContext_IncludesAuditScope(t *testing.T) {
	t.Parallel()
	ns := state.NewNodeState("auth", "Auth Module", state.NodeLeaf)
	ns.Audit.Scope = &state.AuditScope{
		Description: "Verify all auth endpoints",
	}

	result := BuildIterationContext("project/auth", ns, "task-1")

	if !strings.Contains(result, "## Audit Scope") {
		t.Error("expected audit scope section")
	}
	if !strings.Contains(result, "Verify all auth endpoints") {
		t.Error("expected scope description")
	}
}

func TestBuildIterationContext_IncludesSpecs(t *testing.T) {
	t.Parallel()
	ns := state.NewNodeState("auth", "Auth Module", state.NodeLeaf)
	ns.Specs = []string{"spec-auth-flow.md", "spec-jwt-format.md"}

	result := BuildIterationContext("project/auth", ns, "task-1")

	if !strings.Contains(result, "## Linked Specs") {
		t.Error("expected specs section")
	}
	if !strings.Contains(result, "spec-auth-flow.md") {
		t.Error("expected first spec")
	}
	if !strings.Contains(result, "spec-jwt-format.md") {
		t.Error("expected second spec")
	}
}

func TestGenerateScriptReference_ReturnsNonEmpty(t *testing.T) {
	t.Parallel()
	ref := GenerateScriptReference()
	if ref == "" {
		t.Fatal("expected non-empty script reference")
	}
}

func TestGenerateScriptReference_ContainsExpectedCommands(t *testing.T) {
	t.Parallel()
	ref := GenerateScriptReference()

	expected := []string{
		"wolfcastle task add",
		"wolfcastle task claim",
		"wolfcastle task complete",
		"wolfcastle task block",
		"wolfcastle project create",
		"wolfcastle audit breadcrumb",
		"wolfcastle audit escalate",
		"wolfcastle navigate",
		"wolfcastle status",
		"wolfcastle spec create",
		"wolfcastle spec link",
		"wolfcastle spec list",
		"wolfcastle adr create",
		"wolfcastle archive add",
		"wolfcastle inbox add",
		"wolfcastle audit pending",
		"wolfcastle audit approve",
		"wolfcastle audit reject",
		"wolfcastle audit history",
	}

	for _, cmd := range expected {
		if !strings.Contains(ref, cmd) {
			t.Errorf("expected script reference to contain %q", cmd)
		}
	}
}

func TestGenerateScriptReference_ContainsTitle(t *testing.T) {
	t.Parallel()
	ref := GenerateScriptReference()
	if !strings.Contains(ref, "# Wolfcastle Script Reference") {
		t.Error("expected script reference to start with title header")
	}
}

func TestBuildIterationContext_NeedsDecomposition(t *testing.T) {
	t.Parallel()
	ns := state.NewNodeState("auth", "Auth Module", state.NodeLeaf)
	ns.Tasks = []state.Task{
		{
			ID:                 "task-1",
			Description:        "Implement auth",
			State:              state.StatusInProgress,
			FailureCount:       12,
			NeedsDecomposition: true,
		},
	}
	cfg := &config.Config{
		Failure: config.FailureConfig{
			DecompositionThreshold: 10,
			MaxDecompositionDepth:  5,
			HardCap:                50,
		},
	}

	result := BuildIterationContext("project/auth", ns, "task-1", cfg)

	if !strings.Contains(result, "## Failure History") {
		t.Error("expected failure history section")
	}
	if !strings.Contains(result, "Decomposition required.") {
		t.Error("expected decomposition required message")
	}
	if !strings.Contains(result, "Break this leaf into smaller sub-tasks") {
		t.Error("expected decomposition instructions")
	}
	if !strings.Contains(result, "wolfcastle project create --node project/auth") {
		t.Error("expected child node creation command")
	}
	if !strings.Contains(result, "WOLFCASTLE_YIELD") {
		t.Error("expected yield marker")
	}
}

func TestBuildIterationContext_FailureHistoryWithoutDecomposition(t *testing.T) {
	t.Parallel()
	ns := state.NewNodeState("auth", "Auth Module", state.NodeLeaf)
	ns.Tasks = []state.Task{
		{
			ID:                 "task-1",
			Description:        "Implement auth",
			State:              state.StatusInProgress,
			FailureCount:       3,
			NeedsDecomposition: false,
		},
	}
	cfg := &config.Config{
		Failure: config.FailureConfig{
			DecompositionThreshold: 10,
			MaxDecompositionDepth:  5,
			HardCap:                50,
		},
	}

	result := BuildIterationContext("project/auth", ns, "task-1", cfg)

	if !strings.Contains(result, "## Failure History") {
		t.Error("expected failure history section")
	}
	if !strings.Contains(result, "This task has failed 3 times.") {
		t.Error("expected failure count message")
	}
	if strings.Contains(result, "Decomposition required.") {
		t.Error("should NOT contain decomposition message when NeedsDecomposition is false")
	}
}

func TestBuildIterationContext_FailureHistoryNoCfg(t *testing.T) {
	t.Parallel()
	ns := state.NewNodeState("auth", "Auth Module", state.NodeLeaf)
	ns.Tasks = []state.Task{
		{
			ID:           "task-1",
			Description:  "Implement auth",
			State:        state.StatusInProgress,
			FailureCount: 5,
		},
	}

	result := BuildIterationContext("project/auth", ns, "task-1")

	// Without a config, failure history should not appear
	if strings.Contains(result, "## Failure History") {
		t.Error("failure history should not appear without config")
	}
}

func TestBuildIterationContext_IncludesFailureCount(t *testing.T) {
	t.Parallel()
	ns := state.NewNodeState("auth", "Auth Module", state.NodeLeaf)
	ns.Tasks = []state.Task{
		{
			ID:           "task-1",
			Description:  "Implement auth",
			State:        state.StatusInProgress,
			FailureCount: 7,
		},
	}

	result := BuildIterationContext("project/auth", ns, "task-1")

	if !strings.Contains(result, "**Failure Count:** 7") {
		t.Error("expected failure count in task info")
	}
}

func TestBuildIterationContext_OmitsFailureCountWhenZero(t *testing.T) {
	t.Parallel()
	ns := state.NewNodeState("auth", "Auth Module", state.NodeLeaf)
	ns.Tasks = []state.Task{
		{
			ID:          "task-1",
			Description: "Implement auth",
			State:       state.StatusInProgress,
		},
	}

	result := BuildIterationContext("project/auth", ns, "task-1")

	if strings.Contains(result, "**Failure Count:**") {
		t.Error("failure count should be omitted when zero")
	}
}

func TestBuildIterationContext_SummaryRequired_LastIncompleteTask(t *testing.T) {
	t.Parallel()
	ns := state.NewNodeState("auth", "Auth Module", state.NodeLeaf)
	ns.Tasks = []state.Task{
		{ID: "task-1", Description: "First task", State: state.StatusComplete},
		{ID: "task-2", Description: "Second task", State: state.StatusInProgress},
	}

	result := BuildIterationContext("project/auth", ns, "task-2")

	if !strings.Contains(result, "## Summary Required") {
		t.Error("expected summary required section when last incomplete task")
	}
	if !strings.Contains(result, "WOLFCASTLE_SUMMARY") {
		t.Error("expected summary marker instruction")
	}
	if !strings.Contains(result, "WOLFCASTLE_COMPLETE") {
		t.Error("expected complete marker instruction")
	}
}

func TestBuildIterationContext_NoSummaryRequired_OtherTasksIncomplete(t *testing.T) {
	t.Parallel()
	ns := state.NewNodeState("auth", "Auth Module", state.NodeLeaf)
	ns.Tasks = []state.Task{
		{ID: "task-1", Description: "First task", State: state.StatusInProgress},
		{ID: "task-2", Description: "Second task", State: state.StatusNotStarted},
	}

	result := BuildIterationContext("project/auth", ns, "task-1")

	if strings.Contains(result, "## Summary Required") {
		t.Error("summary should not be required when other tasks are incomplete")
	}
}

func TestIsLastIncompleteTask_OnlyTask(t *testing.T) {
	t.Parallel()
	ns := state.NewNodeState("auth", "Auth Module", state.NodeLeaf)
	ns.Tasks = []state.Task{
		{ID: "task-1", Description: "Only task", State: state.StatusInProgress},
	}

	if !isLastIncompleteTask(ns, "task-1") {
		t.Error("single task should be the last incomplete task")
	}
}

func TestIsLastIncompleteTask_AllOthersComplete(t *testing.T) {
	t.Parallel()
	ns := state.NewNodeState("auth", "Auth Module", state.NodeLeaf)
	ns.Tasks = []state.Task{
		{ID: "task-1", Description: "First", State: state.StatusComplete},
		{ID: "task-2", Description: "Second", State: state.StatusComplete},
		{ID: "task-3", Description: "Third", State: state.StatusInProgress},
	}

	if !isLastIncompleteTask(ns, "task-3") {
		t.Error("task-3 should be the last incomplete task")
	}
}

func TestIsLastIncompleteTask_OthersStillIncomplete(t *testing.T) {
	t.Parallel()
	ns := state.NewNodeState("auth", "Auth Module", state.NodeLeaf)
	ns.Tasks = []state.Task{
		{ID: "task-1", Description: "First", State: state.StatusComplete},
		{ID: "task-2", Description: "Second", State: state.StatusNotStarted},
		{ID: "task-3", Description: "Third", State: state.StatusInProgress},
	}

	if isLastIncompleteTask(ns, "task-3") {
		t.Error("task-3 should NOT be last incomplete when task-2 is not_started")
	}
}

func TestIsLastIncompleteTask_EmptyTaskList(t *testing.T) {
	t.Parallel()
	ns := state.NewNodeState("auth", "Auth Module", state.NodeLeaf)

	if !isLastIncompleteTask(ns, "nonexistent") {
		t.Error("empty task list should return true (vacuously)")
	}
}

func TestIsLastIncompleteTask_OtherBlocked(t *testing.T) {
	t.Parallel()
	ns := state.NewNodeState("auth", "Auth Module", state.NodeLeaf)
	ns.Tasks = []state.Task{
		{ID: "task-1", Description: "First", State: state.StatusBlocked},
		{ID: "task-2", Description: "Second", State: state.StatusInProgress},
	}

	if isLastIncompleteTask(ns, "task-2") {
		t.Error("task-2 should NOT be last incomplete when task-1 is blocked (not complete)")
	}
}

func TestBuildIterationContext_TaskNotFound(t *testing.T) {
	t.Parallel()
	ns := state.NewNodeState("auth", "Auth Module", state.NodeLeaf)
	ns.Tasks = []state.Task{
		{ID: "task-1", Description: "Exists", State: state.StatusInProgress},
	}

	result := BuildIterationContext("project/auth", ns, "nonexistent")

	// Should still have node info but no task section
	if !strings.Contains(result, "**Node:** project/auth") {
		t.Error("expected node address even when task not found")
	}
	if strings.Contains(result, "**Task:**") {
		t.Error("should not contain task section for nonexistent task")
	}
}

func TestBuildIterationContext_NoBreadcrumbs(t *testing.T) {
	t.Parallel()
	ns := state.NewNodeState("auth", "Auth Module", state.NodeLeaf)

	result := BuildIterationContext("project/auth", ns, "task-1")

	if strings.Contains(result, "## Recent Breadcrumbs") {
		t.Error("breadcrumbs section should be absent when empty")
	}
}

func TestBuildIterationContext_NoSpecs(t *testing.T) {
	t.Parallel()
	ns := state.NewNodeState("auth", "Auth Module", state.NodeLeaf)

	result := BuildIterationContext("project/auth", ns, "task-1")

	if strings.Contains(result, "## Linked Specs") {
		t.Error("specs section should be absent when empty")
	}
}

func TestBuildIterationContext_NoAuditScope(t *testing.T) {
	t.Parallel()
	ns := state.NewNodeState("auth", "Auth Module", state.NodeLeaf)

	result := BuildIterationContext("project/auth", ns, "task-1")

	if strings.Contains(result, "## Audit Scope") {
		t.Error("audit scope section should be absent when nil")
	}
}
