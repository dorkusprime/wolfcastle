package state

import (
	"testing"
)

// --- TaskClaim additional coverage ---

func TestTaskClaim_ErrorsForCompleteTask(t *testing.T) {
	t.Parallel()
	ns := newLeafWithTasks(
		Task{ID: "task-1", Description: "done", State: StatusComplete},
	)
	err := TaskClaim(ns, "task-1")
	if err == nil {
		t.Error("expected error when claiming a complete task")
	}
}

func TestTaskClaim_ErrorsForBlockedTask(t *testing.T) {
	t.Parallel()
	ns := newLeafWithTasks(
		Task{ID: "task-1", Description: "stuck", State: StatusBlocked},
	)
	err := TaskClaim(ns, "task-1")
	if err == nil {
		t.Error("expected error when claiming a blocked task")
	}
}

func TestTaskClaim_SyncsAuditLifecycle(t *testing.T) {
	t.Parallel()
	ns := newLeafWithTasks(
		Task{ID: "task-1", Description: "test", State: StatusNotStarted},
	)
	if err := TaskClaim(ns, "task-1"); err != nil {
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
		Task{ID: "task-1", Description: "first", State: StatusInProgress},
		Task{ID: "task-2", Description: "second", State: StatusNotStarted},
	)
	ns.State = StatusInProgress

	if err := TaskBlock(ns, "task-1", "stuck"); err != nil {
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
		Task{ID: "task-1", Description: "test", State: StatusNotStarted},
	)
	err := TaskComplete(ns, "task-1")
	if err == nil {
		t.Error("expected error when completing not_started task")
	}
}

func TestTaskComplete_FailsOnBlockedTask(t *testing.T) {
	t.Parallel()
	ns := newLeafWithTasks(
		Task{ID: "task-1", Description: "test", State: StatusBlocked},
	)
	err := TaskComplete(ns, "task-1")
	if err == nil {
		t.Error("expected error when completing blocked task")
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
		Task{ID: "task-1", Description: "test", State: StatusInProgress},
	)
	ns.State = StatusInProgress
	if err := TaskComplete(ns, "task-1"); err != nil {
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
		Task{ID: "task-1", Description: "test", State: StatusNotStarted},
	)
	err := TaskUnblock(ns, "task-1")
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
		Task{ID: "task-1", Description: "test", State: StatusBlocked, FailureCount: 2},
	)
	ns.State = StatusBlocked
	if err := TaskUnblock(ns, "task-1"); err != nil {
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
		Task{ID: "task-1", Description: "test", State: StatusInProgress},
	)
	SetNeedsDecomposition(ns, "task-1", true)
	if !ns.Tasks[0].NeedsDecomposition {
		t.Error("expected NeedsDecomposition to be true")
	}
}

func TestSetNeedsDecomposition_ClearsFlag(t *testing.T) {
	t.Parallel()
	ns := newLeafWithTasks(
		Task{ID: "task-1", Description: "test", State: StatusInProgress, NeedsDecomposition: true},
	)
	SetNeedsDecomposition(ns, "task-1", false)
	if ns.Tasks[0].NeedsDecomposition {
		t.Error("expected NeedsDecomposition to be false")
	}
}

func TestSetNeedsDecomposition_NoopForUnknownTask(t *testing.T) {
	t.Parallel()
	ns := newLeafWithTasks(
		Task{ID: "task-1", Description: "test", State: StatusInProgress},
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

	AddEscalation(parent, "leaf-1", "missing error handling", "gap-1")

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
	if esc.Status != "open" {
		t.Errorf("expected status 'open', got %q", esc.Status)
	}
	if esc.Timestamp.IsZero() {
		t.Error("expected timestamp to be set")
	}
}

func TestAddEscalation_GeneratesSequentialIDs(t *testing.T) {
	t.Parallel()
	parent := NewNodeState("orch-1", "Orchestrator", NodeOrchestrator)

	AddEscalation(parent, "leaf-1", "first issue", "")
	AddEscalation(parent, "leaf-2", "second issue", "")

	if len(parent.Audit.Escalations) != 2 {
		t.Fatalf("expected 2 escalations, got %d", len(parent.Audit.Escalations))
	}
	if parent.Audit.Escalations[0].ID != "escalation-orch-1-1" {
		t.Errorf("expected id 'escalation-orch-1-1', got %q", parent.Audit.Escalations[0].ID)
	}
	if parent.Audit.Escalations[1].ID != "escalation-orch-1-2" {
		t.Errorf("expected id 'escalation-orch-1-2', got %q", parent.Audit.Escalations[1].ID)
	}
}

// --- MoveAuditLast ---

func TestMoveAuditLast_MovesAuditToEnd(t *testing.T) {
	t.Parallel()
	ns := newLeafWithTasks(
		Task{ID: "audit", Description: "audit task", IsAudit: true, State: StatusNotStarted},
		Task{ID: "task-1", Description: "first", State: StatusNotStarted},
		Task{ID: "task-2", Description: "second", State: StatusNotStarted},
	)

	MoveAuditLast(ns)

	if ns.Tasks[len(ns.Tasks)-1].ID != "audit" {
		t.Errorf("expected audit task last, got %s", ns.Tasks[len(ns.Tasks)-1].ID)
	}
	if ns.Tasks[0].ID != "task-1" {
		t.Errorf("expected task-1 first, got %s", ns.Tasks[0].ID)
	}
}

func TestMoveAuditLast_NoopWhenAlreadyLast(t *testing.T) {
	t.Parallel()
	ns := newLeafWithTasks(
		Task{ID: "task-1", Description: "first", State: StatusNotStarted},
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
		Task{ID: "task-1", Description: "first", State: StatusNotStarted},
		Task{ID: "task-2", Description: "second", State: StatusNotStarted},
	)

	// Should not panic
	MoveAuditLast(ns)

	if ns.Tasks[0].ID != "task-1" {
		t.Errorf("task order should be unchanged, got %s first", ns.Tasks[0].ID)
	}
}

// --- AddBreadcrumb timestamp ---

func TestAddBreadcrumb_SetsTimestamp(t *testing.T) {
	t.Parallel()
	ns := NewNodeState("leaf-1", "Test", NodeLeaf)

	AddBreadcrumb(ns, "leaf-1/task-1", "did something")

	if ns.Audit.Breadcrumbs[0].Timestamp.IsZero() {
		t.Error("expected non-zero timestamp")
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
	if task.ID != "task-1" {
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
		Task{ID: "task-1", Description: "test", State: StatusInProgress, FailureCount: 5},
	)

	count, err := IncrementFailure(ns, "task-1")
	if err != nil {
		t.Fatal(err)
	}
	if count != 6 {
		t.Errorf("expected 6, got %d", count)
	}
}
