package pipeline

import (
	"strings"
	"testing"
	"time"

	"github.com/dorkusprime/wolfcastle/internal/state"
)

func TestBuildPlanningContext_BasicScope(t *testing.T) {
	ns := state.NewNodeState("orch-001", "Build Cache Layer", state.NodeOrchestrator)
	ns.Scope = "Implement a TTL-based cache layer for the prompt repository."

	ctx := BuildPlanningContext("project/orch-001", ns, "initial")

	if !strings.Contains(ctx, "**Orchestrator:** project/orch-001") {
		t.Error("context should include orchestrator address")
	}
	if !strings.Contains(ctx, "**Planning Trigger:** initial") {
		t.Error("context should include planning trigger")
	}
	if !strings.Contains(ctx, "## Scope") {
		t.Error("context should include scope section")
	}
	if !strings.Contains(ctx, "TTL-based cache layer") {
		t.Error("context should include scope content")
	}
}

func TestBuildPlanningContext_PendingScope(t *testing.T) {
	ns := state.NewNodeState("orch-002", "Orch", state.NodeOrchestrator)
	ns.PendingScope = []string{
		"Add Redis support",
		"Add Memcached support",
		"Add LRU eviction",
		"Add TTL eviction",
		"Add size-based eviction",
		"Add disk-backed cache",
		"Add distributed locking",
	}

	ctx := BuildPlanningContext("project/orch-002", ns, "scope_update")

	if !strings.Contains(ctx, "## Pending Scope") {
		t.Error("context should include pending scope section")
	}
	// First 5 should appear
	for _, item := range ns.PendingScope[:5] {
		if !strings.Contains(ctx, item) {
			t.Errorf("context should include pending scope item %q", item)
		}
	}
	// Items beyond 5 should be truncated
	if strings.Contains(ctx, "Add disk-backed cache") {
		t.Error("6th pending scope item should be truncated")
	}
	if strings.Contains(ctx, "Add distributed locking") {
		t.Error("7th pending scope item should be truncated")
	}
	if !strings.Contains(ctx, "(2 additional items truncated)") {
		t.Error("context should show truncation count")
	}
}

func TestBuildPlanningContext_PendingScope_ExactlyFive(t *testing.T) {
	ns := state.NewNodeState("orch-003", "Orch", state.NodeOrchestrator)
	ns.PendingScope = []string{"a", "b", "c", "d", "e"}

	ctx := BuildPlanningContext("project/orch-003", ns, "scope_update")

	for _, item := range ns.PendingScope {
		if !strings.Contains(ctx, item) {
			t.Errorf("context should include pending scope item %q", item)
		}
	}
	if strings.Contains(ctx, "additional items truncated") {
		t.Error("exactly 5 items should not trigger truncation message")
	}
}

func TestBuildPlanningContext_SuccessCriteria(t *testing.T) {
	ns := state.NewNodeState("orch-004", "Orch", state.NodeOrchestrator)
	ns.SuccessCriteria = []string{
		"All tests pass with -race",
		"Cache hit rate above 90%",
		"No goroutine leaks",
	}

	ctx := BuildPlanningContext("project/orch-004", ns, "replan")

	if !strings.Contains(ctx, "## Success Criteria") {
		t.Error("context should include success criteria section")
	}
	for _, c := range ns.SuccessCriteria {
		if !strings.Contains(ctx, c) {
			t.Errorf("context should include criterion %q", c)
		}
	}
}

func TestBuildPlanningContext_ChildrenState(t *testing.T) {
	ns := state.NewNodeState("orch-005", "Orch", state.NodeOrchestrator)
	ns.Children = []state.ChildRef{
		{ID: "leaf-001", Address: "project/orch-005/leaf-001", State: state.StatusComplete},
		{ID: "leaf-002", Address: "project/orch-005/leaf-002", State: state.StatusInProgress},
		{ID: "leaf-003", Address: "project/orch-005/leaf-003", State: state.StatusBlocked},
	}

	ctx := BuildPlanningContext("project/orch-005", ns, "child_complete")

	if !strings.Contains(ctx, "## Children") {
		t.Error("context should include children section")
	}
	if !strings.Contains(ctx, "**leaf-001** (project/orch-005/leaf-001): complete") {
		t.Error("context should render completed child with state")
	}
	if !strings.Contains(ctx, "**leaf-002** (project/orch-005/leaf-002): in_progress") {
		t.Error("context should render in-progress child")
	}
	if !strings.Contains(ctx, "**leaf-003** (project/orch-005/leaf-003): blocked") {
		t.Error("context should render blocked child")
	}
}

func TestBuildPlanningContext_TaskState(t *testing.T) {
	ns := state.NewNodeState("leaf-010", "Leaf", state.NodeLeaf)
	ns.Tasks = []state.Task{
		{ID: "t-1", Description: "Write cache struct", State: state.StatusComplete},
		{ID: "t-2", Description: "Add TTL logic", State: state.StatusInProgress},
		{ID: "t-3", Description: "Wire into app", State: state.StatusBlocked, BlockedReason: "waiting on config"},
		{ID: "t-4", Description: "Write tests", State: state.StatusNotStarted},
	}

	ctx := BuildPlanningContext("project/leaf-010", ns, "task_update")

	if !strings.Contains(ctx, "## Tasks") {
		t.Error("context should include tasks section")
	}
	if !strings.Contains(ctx, "✓ t-1: Write cache struct") {
		t.Error("completed task should have check marker")
	}
	if !strings.Contains(ctx, "→ t-2: Add TTL logic") {
		t.Error("in-progress task should have arrow marker")
	}
	if !strings.Contains(ctx, "✖ t-3: Wire into app (blocked: waiting on config)") {
		t.Error("blocked task should have X marker and reason")
	}
	if !strings.Contains(ctx, "○ t-4: Write tests") {
		t.Error("not-started task should have circle marker")
	}
}

func TestBuildPlanningContext_PlanningHistory(t *testing.T) {
	ns := state.NewNodeState("orch-006", "Orch", state.NodeOrchestrator)
	base := time.Date(2026, 3, 15, 10, 0, 0, 0, time.UTC)
	ns.PlanningHistory = []state.PlanningPass{
		{Timestamp: base, Trigger: "initial", Summary: "Created 3 children"},
		{Timestamp: base.Add(1 * time.Hour), Trigger: "child_complete", Summary: "Merged leaf-001 results"},
		{Timestamp: base.Add(2 * time.Hour), Trigger: "child_blocked", Summary: "Replanned leaf-002"},
		{Timestamp: base.Add(3 * time.Hour), Trigger: "scope_update", Summary: "Added Redis support"},
		{Timestamp: base.Add(4 * time.Hour), Trigger: "child_complete", Summary: "Final review pass"},
	}

	ctx := BuildPlanningContext("project/orch-006", ns, "replan")

	if !strings.Contains(ctx, "## Planning History") {
		t.Error("context should include planning history section")
	}
	// Only last 3 should appear
	if strings.Contains(ctx, "Created 3 children") {
		t.Error("oldest planning pass should be truncated")
	}
	if strings.Contains(ctx, "Merged leaf-001 results") {
		t.Error("second-oldest planning pass should be truncated")
	}
	if !strings.Contains(ctx, "Replanned leaf-002") {
		t.Error("third-to-last pass should appear")
	}
	if !strings.Contains(ctx, "Added Redis support") {
		t.Error("second-to-last pass should appear")
	}
	if !strings.Contains(ctx, "Final review pass") {
		t.Error("most recent pass should appear")
	}
}

func TestBuildPlanningContext_OpenGaps(t *testing.T) {
	ns := state.NewNodeState("orch-007", "Orch", state.NodeOrchestrator)
	ns.Audit.Gaps = []state.Gap{
		{ID: "gap-001", Description: "Missing error handling in cache.Get", Status: state.GapOpen},
		{ID: "gap-002", Description: "No test for expiry edge case", Status: state.GapFixed},
		{ID: "gap-003", Description: "Race condition in eviction", Status: state.GapOpen},
	}

	ctx := BuildPlanningContext("project/orch-007", ns, "audit_failed")

	if !strings.Contains(ctx, "## Open Audit Gaps") {
		t.Error("context should include open audit gaps section")
	}
	if !strings.Contains(ctx, "gap-001: Missing error handling in cache.Get") {
		t.Error("open gap should appear")
	}
	if !strings.Contains(ctx, "gap-003: Race condition in eviction") {
		t.Error("second open gap should appear")
	}
	if strings.Contains(ctx, "gap-002") {
		t.Error("fixed gap should not appear in open gaps section")
	}
}

func TestBuildPlanningContext_OpenGaps_AllClosed(t *testing.T) {
	ns := state.NewNodeState("orch-008", "Orch", state.NodeOrchestrator)
	ns.Audit.Gaps = []state.Gap{
		{ID: "gap-001", Description: "Already fixed", Status: state.GapFixed},
	}

	ctx := BuildPlanningContext("project/orch-008", ns, "audit_passed")

	if strings.Contains(ctx, "## Open Audit Gaps") {
		t.Error("no open gaps section when all gaps are closed")
	}
}

func TestBuildPlanningContext_EmptyState(t *testing.T) {
	ns := state.NewNodeState("orch-009", "Minimal Orch", state.NodeOrchestrator)

	ctx := BuildPlanningContext("project/orch-009", ns, "initial")

	// Header lines should always be present
	if !strings.Contains(ctx, "**Orchestrator:** project/orch-009") {
		t.Error("context should always include orchestrator address")
	}
	if !strings.Contains(ctx, "**Planning Trigger:** initial") {
		t.Error("context should always include trigger")
	}

	// No optional sections should appear
	for _, section := range []string{
		"## Scope",
		"## Pending Scope",
		"## Success Criteria",
		"## Children",
		"## Tasks",
		"## Planning History",
		"## Open Audit Gaps",
		"## Linked Specs",
	} {
		if strings.Contains(ctx, section) {
			t.Errorf("empty state should not render section %q", section)
		}
	}
}
