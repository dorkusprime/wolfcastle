package state

import (
	"testing"
)

func newLeafWithTasks(tasks ...Task) *NodeState {
	ns := NewNodeState("leaf-1", "Test Leaf", NodeLeaf)
	ns.Tasks = tasks
	return ns
}

func TestTaskAdd_InsertsBeforeAudit(t *testing.T) {
	t.Parallel()
	ns := newLeafWithTasks(
		Task{ID: "task-1", Description: "first", State: StatusNotStarted},
		Task{ID: "audit", Description: "audit task", State: StatusNotStarted},
	)

	task, err := TaskAdd(ns, "new task")
	if err != nil {
		t.Fatal(err)
	}
	if task.ID != "task-2" {
		t.Errorf("expected task-2, got %s", task.ID)
	}
	// audit should still be last
	if ns.Tasks[len(ns.Tasks)-1].ID != "audit" {
		t.Error("audit task should remain last")
	}
	if ns.Tasks[1].ID != "task-2" {
		t.Errorf("new task should be before audit, got %s at index 1", ns.Tasks[1].ID)
	}
}

func TestTaskAdd_GeneratesSequentialIDs(t *testing.T) {
	t.Parallel()
	ns := newLeafWithTasks()

	t1, err := TaskAdd(ns, "first")
	if err != nil {
		t.Fatal(err)
	}
	if t1.ID != "task-1" {
		t.Errorf("expected task-1, got %s", t1.ID)
	}

	t2, err := TaskAdd(ns, "second")
	if err != nil {
		t.Fatal(err)
	}
	if t2.ID != "task-2" {
		t.Errorf("expected task-2, got %s", t2.ID)
	}

	t3, err := TaskAdd(ns, "third")
	if err != nil {
		t.Fatal(err)
	}
	if t3.ID != "task-3" {
		t.Errorf("expected task-3, got %s", t3.ID)
	}
}

func TestTaskAdd_FailsOnOrchestratorNodes(t *testing.T) {
	t.Parallel()
	ns := NewNodeState("orch-1", "Orchestrator", NodeOrchestrator)

	_, err := TaskAdd(ns, "should fail")
	if err == nil {
		t.Error("expected error when adding task to orchestrator")
	}
}

func TestTaskClaim_TransitionsNotStartedToInProgress(t *testing.T) {
	t.Parallel()
	ns := newLeafWithTasks(
		Task{ID: "task-1", Description: "test", State: StatusNotStarted},
	)

	if err := TaskClaim(ns, "task-1"); err != nil {
		t.Fatal(err)
	}
	if ns.Tasks[0].State != StatusInProgress {
		t.Errorf("expected in_progress, got %s", ns.Tasks[0].State)
	}
	if ns.State != StatusInProgress {
		t.Errorf("node should be in_progress, got %s", ns.State)
	}
}

func TestTaskClaim_FailsOnWrongState(t *testing.T) {
	t.Parallel()
	ns := newLeafWithTasks(
		Task{ID: "task-1", Description: "test", State: StatusInProgress},
	)

	err := TaskClaim(ns, "task-1")
	if err == nil {
		t.Error("expected error when claiming in_progress task")
	}
}

func TestTaskClaim_FailsOnMissingTask(t *testing.T) {
	t.Parallel()
	ns := newLeafWithTasks()

	err := TaskClaim(ns, "task-99")
	if err == nil {
		t.Error("expected error for missing task")
	}
}

func TestTaskComplete_TransitionsInProgressToComplete(t *testing.T) {
	t.Parallel()
	ns := newLeafWithTasks(
		Task{ID: "task-1", Description: "test", State: StatusInProgress},
	)
	ns.State = StatusInProgress

	if err := TaskComplete(ns, "task-1"); err != nil {
		t.Fatal(err)
	}
	if ns.Tasks[0].State != StatusComplete {
		t.Errorf("expected complete, got %s", ns.Tasks[0].State)
	}
}

func TestTaskComplete_MarksNodeCompleteWhenAllTasksDone(t *testing.T) {
	t.Parallel()
	ns := newLeafWithTasks(
		Task{ID: "task-1", Description: "first", State: StatusComplete},
		Task{ID: "task-2", Description: "second", State: StatusInProgress},
	)
	ns.State = StatusInProgress

	if err := TaskComplete(ns, "task-2"); err != nil {
		t.Fatal(err)
	}
	if ns.State != StatusComplete {
		t.Errorf("node should be complete, got %s", ns.State)
	}
}

func TestTaskComplete_DoesNotMarkNodeCompleteWithRemainingTasks(t *testing.T) {
	t.Parallel()
	ns := newLeafWithTasks(
		Task{ID: "task-1", Description: "first", State: StatusInProgress},
		Task{ID: "task-2", Description: "second", State: StatusNotStarted},
	)
	ns.State = StatusInProgress

	if err := TaskComplete(ns, "task-1"); err != nil {
		t.Fatal(err)
	}
	if ns.State == StatusComplete {
		t.Error("node should not be complete with remaining tasks")
	}
}

func TestTaskBlock_TransitionsInProgressToBlocked(t *testing.T) {
	t.Parallel()
	ns := newLeafWithTasks(
		Task{ID: "task-1", Description: "test", State: StatusInProgress},
	)
	ns.State = StatusInProgress

	if err := TaskBlock(ns, "task-1", "stuck"); err != nil {
		t.Fatal(err)
	}
	if ns.Tasks[0].State != StatusBlocked {
		t.Errorf("expected blocked, got %s", ns.Tasks[0].State)
	}
	if ns.Tasks[0].BlockReason != "stuck" {
		t.Errorf("expected reason 'stuck', got %q", ns.Tasks[0].BlockReason)
	}
}

func TestTaskBlock_FailsOnNotStarted(t *testing.T) {
	t.Parallel()
	ns := newLeafWithTasks(
		Task{ID: "task-1", Description: "test", State: StatusNotStarted},
	)

	err := TaskBlock(ns, "task-1", "reason")
	if err == nil {
		t.Error("expected error when blocking not_started task")
	}
}

func TestTaskBlock_BlocksNodeWhenAllNonCompleteBlocked(t *testing.T) {
	t.Parallel()
	ns := newLeafWithTasks(
		Task{ID: "task-1", Description: "first", State: StatusComplete},
		Task{ID: "task-2", Description: "second", State: StatusInProgress},
	)
	ns.State = StatusInProgress

	if err := TaskBlock(ns, "task-2", "stuck"); err != nil {
		t.Fatal(err)
	}
	if ns.State != StatusBlocked {
		t.Errorf("node should be blocked when all non-complete tasks are blocked, got %s", ns.State)
	}
}

func TestTaskUnblock_TransitionsBlockedToNotStarted(t *testing.T) {
	t.Parallel()
	ns := newLeafWithTasks(
		Task{ID: "task-1", Description: "test", State: StatusBlocked, BlockReason: "stuck", FailureCount: 3},
	)
	ns.State = StatusBlocked

	if err := TaskUnblock(ns, "task-1"); err != nil {
		t.Fatal(err)
	}
	if ns.Tasks[0].State != StatusNotStarted {
		t.Errorf("expected not_started, got %s", ns.Tasks[0].State)
	}
	if ns.Tasks[0].BlockReason != "" {
		t.Errorf("block reason should be cleared, got %q", ns.Tasks[0].BlockReason)
	}
}

func TestTaskUnblock_ResetsFailureCounter(t *testing.T) {
	t.Parallel()
	ns := newLeafWithTasks(
		Task{ID: "task-1", Description: "test", State: StatusBlocked, FailureCount: 5},
	)
	ns.State = StatusBlocked

	if err := TaskUnblock(ns, "task-1"); err != nil {
		t.Fatal(err)
	}
	if ns.Tasks[0].FailureCount != 0 {
		t.Errorf("failure count should be reset, got %d", ns.Tasks[0].FailureCount)
	}
}

func TestTaskUnblock_SetsNodeInProgress(t *testing.T) {
	t.Parallel()
	ns := newLeafWithTasks(
		Task{ID: "task-1", Description: "test", State: StatusBlocked},
	)
	ns.State = StatusBlocked

	if err := TaskUnblock(ns, "task-1"); err != nil {
		t.Fatal(err)
	}
	if ns.State != StatusInProgress {
		t.Errorf("node should be in_progress after unblock, got %s", ns.State)
	}
}

func TestIncrementFailure_IncrementsAndReturnsCount(t *testing.T) {
	t.Parallel()
	ns := newLeafWithTasks(
		Task{ID: "task-1", Description: "test", State: StatusInProgress, FailureCount: 0},
	)

	count, err := IncrementFailure(ns, "task-1")
	if err != nil {
		t.Fatal(err)
	}
	if count != 1 {
		t.Errorf("expected 1, got %d", count)
	}

	count, err = IncrementFailure(ns, "task-1")
	if err != nil {
		t.Fatal(err)
	}
	if count != 2 {
		t.Errorf("expected 2, got %d", count)
	}
}

func TestIncrementFailure_FailsOnMissingTask(t *testing.T) {
	t.Parallel()
	ns := newLeafWithTasks()

	_, err := IncrementFailure(ns, "task-99")
	if err == nil {
		t.Error("expected error for missing task")
	}
}

func TestAddBreadcrumb_AppendsToAuditTrail(t *testing.T) {
	t.Parallel()
	ns := NewNodeState("leaf-1", "Test", NodeLeaf)

	AddBreadcrumb(ns, "leaf-1/task-1", "did something")
	AddBreadcrumb(ns, "leaf-1/task-2", "did more")

	if len(ns.Audit.Breadcrumbs) != 2 {
		t.Fatalf("expected 2 breadcrumbs, got %d", len(ns.Audit.Breadcrumbs))
	}
	if ns.Audit.Breadcrumbs[0].Task != "leaf-1/task-1" {
		t.Errorf("expected task addr leaf-1/task-1, got %s", ns.Audit.Breadcrumbs[0].Task)
	}
	if ns.Audit.Breadcrumbs[1].Text != "did more" {
		t.Errorf("expected text 'did more', got %s", ns.Audit.Breadcrumbs[1].Text)
	}
}
