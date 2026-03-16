package state

import (
	"testing"
	"time"

	"github.com/dorkusprime/wolfcastle/internal/clock"
)

// --- TaskClaim additional coverage ---

func TestTaskClaim_ErrorsForCompleteTask(t *testing.T) {
	t.Parallel()
	ns := newLeafWithTasks(
		Task{ID: "task-0001", Description: "done", State: StatusComplete},
	)
	err := TaskClaim(ns, "task-0001")
	if err == nil {
		t.Error("expected error when claiming a complete task")
	}
}

func TestTaskClaim_ErrorsForBlockedTask(t *testing.T) {
	t.Parallel()
	ns := newLeafWithTasks(
		Task{ID: "task-0001", Description: "stuck", State: StatusBlocked},
	)
	err := TaskClaim(ns, "task-0001")
	if err == nil {
		t.Error("expected error when claiming a blocked task")
	}
}

func TestTaskClaim_SyncsAuditLifecycle(t *testing.T) {
	t.Parallel()
	ns := newLeafWithTasks(
		Task{ID: "task-0001", Description: "test", State: StatusNotStarted},
	)
	if err := TaskClaim(ns, "task-0001"); err != nil {
		t.Fatal(err)
	}
	if ns.Audit.Status != AuditInProgress {
		t.Errorf("expected audit in_progress after claim, got %s", ns.Audit.Status)
	}
}

// --- TaskBlock additional coverage ---

func TestTaskBlock_FailsOnMissingTask(t *testing.T) {
	t.Parallel()
	ns := newLeafWithTasks()
	err := TaskBlock(ns, "task-99", "reason")
	if err == nil {
		t.Error("expected error for missing task")
	}
}

func TestTaskBlock_DoesNotBlockNodeWithRemainingNotStartedTasks(t *testing.T) {
	t.Parallel()
	ns := newLeafWithTasks(
		Task{ID: "task-0001", Description: "first", State: StatusInProgress},
		Task{ID: "task-0002", Description: "second", State: StatusNotStarted},
	)
	ns.State = StatusInProgress

	if err := TaskBlock(ns, "task-0001", "stuck"); err != nil {
		t.Fatal(err)
	}
	if ns.State == StatusBlocked {
		t.Error("node should not be blocked when not_started tasks remain")
	}
}

// --- TaskComplete additional coverage ---

func TestTaskComplete_FailsOnNotStartedTask(t *testing.T) {
	t.Parallel()
	ns := newLeafWithTasks(
		Task{ID: "task-0001", Description: "test", State: StatusNotStarted},
	)
	err := TaskComplete(ns, "task-0001")
	if err == nil {
		t.Error("expected error when completing not_started task")
	}
}

func TestTaskComplete_IdempotentOnBlockedTask(t *testing.T) {
	t.Parallel()
	ns := newLeafWithTasks(
		Task{ID: "task-0001", Description: "test", State: StatusBlocked, BlockedReason: "stuck"},
	)
	err := TaskComplete(ns, "task-0001")
	if err != nil {
		t.Errorf("completing a blocked task should be a no-op, got: %v", err)
	}
	if ns.Tasks[0].State != StatusBlocked {
		t.Errorf("task should remain blocked, got %s", ns.Tasks[0].State)
	}
}

func TestTaskComplete_FailsOnMissingTask(t *testing.T) {
	t.Parallel()
	ns := newLeafWithTasks()
	err := TaskComplete(ns, "task-99")
	if err == nil {
		t.Error("expected error for missing task")
	}
}

func TestTaskComplete_SyncsAuditLifecycle(t *testing.T) {
	t.Parallel()
	ns := newLeafWithTasks(
		Task{ID: "task-0001", Description: "test", State: StatusInProgress},
	)
	ns.State = StatusInProgress
	if err := TaskComplete(ns, "task-0001"); err != nil {
		t.Fatal(err)
	}
	if ns.State != StatusComplete {
		t.Errorf("expected complete, got %s", ns.State)
	}
	if ns.Audit.Status != AuditPassed {
		t.Errorf("expected audit passed after all tasks complete, got %s", ns.Audit.Status)
	}
}

// --- TaskUnblock additional coverage ---

func TestTaskUnblock_FailsOnNotStartedTask(t *testing.T) {
	t.Parallel()
	ns := newLeafWithTasks(
		Task{ID: "task-0001", Description: "test", State: StatusNotStarted},
	)
	err := TaskUnblock(ns, "task-0001")
	if err == nil {
		t.Error("expected error when unblocking not_started task")
	}
}

func TestTaskUnblock_FailsOnMissingTask(t *testing.T) {
	t.Parallel()
	ns := newLeafWithTasks()
	err := TaskUnblock(ns, "task-99")
	if err == nil {
		t.Error("expected error for missing task")
	}
}

func TestTaskUnblock_SyncsAuditLifecycle(t *testing.T) {
	t.Parallel()
	ns := newLeafWithTasks(
		Task{ID: "task-0001", Description: "test", State: StatusBlocked, FailureCount: 2},
	)
	ns.State = StatusBlocked
	if err := TaskUnblock(ns, "task-0001"); err != nil {
		t.Fatal(err)
	}
	if ns.Audit.Status != AuditInProgress {
		t.Errorf("expected audit in_progress after unblock, got %s", ns.Audit.Status)
	}
}

// --- SetNeedsDecomposition ---

func TestSetNeedsDecomposition_SetsFlag(t *testing.T) {
	t.Parallel()
	ns := newLeafWithTasks(
		Task{ID: "task-0001", Description: "test", State: StatusInProgress},
	)
	SetNeedsDecomposition(ns, "task-0001", true)
	if !ns.Tasks[0].NeedsDecomposition {
		t.Error("expected NeedsDecomposition to be true")
	}
}

func TestSetNeedsDecomposition_ClearsFlag(t *testing.T) {
	t.Parallel()
	ns := newLeafWithTasks(
		Task{ID: "task-0001", Description: "test", State: StatusInProgress, NeedsDecomposition: true},
	)
	SetNeedsDecomposition(ns, "task-0001", false)
	if ns.Tasks[0].NeedsDecomposition {
		t.Error("expected NeedsDecomposition to be false")
	}
}

func TestSetNeedsDecomposition_NoopForUnknownTask(t *testing.T) {
	t.Parallel()
	ns := newLeafWithTasks(
		Task{ID: "task-0001", Description: "test", State: StatusInProgress},
	)
	// Should not panic for unknown task
	SetNeedsDecomposition(ns, "task-99", true)
	if ns.Tasks[0].NeedsDecomposition {
		t.Error("should not have modified existing task")
	}
}

// --- AddEscalation ---

func TestAddEscalation_CreatesOnParent(t *testing.T) {
	t.Parallel()
	parent := NewNodeState("orch-1", "Orchestrator", NodeOrchestrator)
	fixed := time.Date(2026, 3, 14, 12, 0, 0, 0, time.UTC)
	clk := clock.NewFixed(fixed)

	AddEscalation(parent, "leaf-1", "missing error handling", "gap-1", clk)

	if len(parent.Audit.Escalations) != 1 {
		t.Fatalf("expected 1 escalation, got %d", len(parent.Audit.Escalations))
	}
	esc := parent.Audit.Escalations[0]
	if esc.SourceNode != "leaf-1" {
		t.Errorf("expected source_node 'leaf-1', got %q", esc.SourceNode)
	}
	if esc.Description != "missing error handling" {
		t.Errorf("expected description 'missing error handling', got %q", esc.Description)
	}
	if esc.SourceGapID != "gap-1" {
		t.Errorf("expected source_gap_id 'gap-1', got %q", esc.SourceGapID)
	}
	if esc.Status != EscalationOpen {
		t.Errorf("expected status 'open', got %q", esc.Status)
	}
	if !esc.Timestamp.Equal(fixed) {
		t.Errorf("expected timestamp %v, got %v", fixed, esc.Timestamp)
	}
}

func TestAddEscalation_GeneratesSequentialIDs(t *testing.T) {
	t.Parallel()
	parent := NewNodeState("orch-1", "Orchestrator", NodeOrchestrator)
	clk := clock.NewFixed(time.Date(2026, 3, 14, 12, 0, 0, 0, time.UTC))

	AddEscalation(parent, "leaf-1", "first issue", "", clk)
	AddEscalation(parent, "leaf-2", "second issue", "", clk)

	if len(parent.Audit.Escalations) != 2 {
		t.Fatalf("expected 2 escalations, got %d", len(parent.Audit.Escalations))
	}
	if parent.Audit.Escalations[0].ID != "escalation-leaf-1-1" {
		t.Errorf("expected id 'escalation-leaf-1-1', got %q", parent.Audit.Escalations[0].ID)
	}
	if parent.Audit.Escalations[1].ID != "escalation-leaf-2-2" {
		t.Errorf("expected id 'escalation-leaf-2-2', got %q", parent.Audit.Escalations[1].ID)
	}
}

// --- MoveAuditLast ---

func TestMoveAuditLast_MovesAuditToEnd(t *testing.T) {
	t.Parallel()
	ns := newLeafWithTasks(
		Task{ID: "audit", Description: "audit task", IsAudit: true, State: StatusNotStarted},
		Task{ID: "task-0001", Description: "first", State: StatusNotStarted},
		Task{ID: "task-0002", Description: "second", State: StatusNotStarted},
	)

	MoveAuditLast(ns)

	if ns.Tasks[len(ns.Tasks)-1].ID != "audit" {
		t.Errorf("expected audit task last, got %s", ns.Tasks[len(ns.Tasks)-1].ID)
	}
	if ns.Tasks[0].ID != "task-0001" {
		t.Errorf("expected task-1 first, got %s", ns.Tasks[0].ID)
	}
}

func TestMoveAuditLast_NoopWhenAlreadyLast(t *testing.T) {
	t.Parallel()
	ns := newLeafWithTasks(
		Task{ID: "task-0001", Description: "first", State: StatusNotStarted},
		Task{ID: "audit", Description: "audit task", IsAudit: true, State: StatusNotStarted},
	)

	MoveAuditLast(ns)

	if ns.Tasks[len(ns.Tasks)-1].ID != "audit" {
		t.Errorf("expected audit task last, got %s", ns.Tasks[len(ns.Tasks)-1].ID)
	}
}

func TestMoveAuditLast_NoopWhenNoAuditTask(t *testing.T) {
	t.Parallel()
	ns := newLeafWithTasks(
		Task{ID: "task-0001", Description: "first", State: StatusNotStarted},
		Task{ID: "task-0002", Description: "second", State: StatusNotStarted},
	)

	// Should not panic
	MoveAuditLast(ns)

	if ns.Tasks[0].ID != "task-0001" {
		t.Errorf("task order should be unchanged, got %s first", ns.Tasks[0].ID)
	}
}

// --- AddBreadcrumb timestamp ---

func TestAddBreadcrumb_SetsTimestamp(t *testing.T) {
	t.Parallel()
	ns := NewNodeState("leaf-1", "Test", NodeLeaf)
	fixed := time.Date(2026, 3, 14, 12, 0, 0, 0, time.UTC)
	clk := clock.NewFixed(fixed)

	AddBreadcrumb(ns, "leaf-1/task-0001", "did something", clk)

	if !ns.Audit.Breadcrumbs[0].Timestamp.Equal(fixed) {
		t.Errorf("expected %v, got %v", fixed, ns.Audit.Breadcrumbs[0].Timestamp)
	}
}

// --- TaskAdd edge cases ---

func TestTaskAdd_NoExistingTasks(t *testing.T) {
	t.Parallel()
	ns := newLeafWithTasks()

	task, err := TaskAdd(ns, "brand new")
	if err != nil {
		t.Fatal(err)
	}
	if task.ID != "task-0001" {
		t.Errorf("expected task-1, got %s", task.ID)
	}
	if task.State != StatusNotStarted {
		t.Errorf("expected not_started, got %s", task.State)
	}
	if len(ns.Tasks) != 1 {
		t.Errorf("expected 1 task, got %d", len(ns.Tasks))
	}
}

// --- IncrementFailure multiple increments ---

func TestIncrementFailure_MultipleIncrements(t *testing.T) {
	t.Parallel()
	ns := newLeafWithTasks(
		Task{ID: "task-0001", Description: "test", State: StatusInProgress, FailureCount: 5},
	)

	count, err := IncrementFailure(ns, "task-0001")
	if err != nil {
		t.Fatal(err)
	}
	if count != 6 {
		t.Errorf("expected 6, got %d", count)
	}
}
