package state

import (
	"testing"
	"time"

	"github.com/dorkusprime/wolfcastle/internal/clock"
)

func newLeafWithTasks(tasks ...Task) *NodeState {
	ns := NewNodeState("leaf-1", "Test Leaf", NodeLeaf)
	ns.Tasks = tasks
	return ns
}

func TestTaskAdd_InsertsBeforeAudit(t *testing.T) {
	t.Parallel()
	ns := newLeafWithTasks(
		Task{ID: "task-0001", Description: "first", State: StatusNotStarted},
		Task{ID: "audit", Description: "audit task", State: StatusNotStarted, IsAudit: true},
	)

	task, err := TaskAdd(ns, "new task")
	if err != nil {
		t.Fatal(err)
	}
	if task.ID != "task-0002" {
		t.Errorf("expected task-2, got %s", task.ID)
	}
	// audit should still be last
	if ns.Tasks[len(ns.Tasks)-1].ID != "audit" {
		t.Error("audit task should remain last")
	}
	if ns.Tasks[1].ID != "task-0002" {
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
	if t1.ID != "task-0001" {
		t.Errorf("expected task-1, got %s", t1.ID)
	}

	t2, err := TaskAdd(ns, "second")
	if err != nil {
		t.Fatal(err)
	}
	if t2.ID != "task-0002" {
		t.Errorf("expected task-2, got %s", t2.ID)
	}

	t3, err := TaskAdd(ns, "third")
	if err != nil {
		t.Fatal(err)
	}
	if t3.ID != "task-0003" {
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
		Task{ID: "task-0001", Description: "test", State: StatusNotStarted},
	)

	if err := TaskClaim(ns, "task-0001"); err != nil {
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
		Task{ID: "task-0001", Description: "test", State: StatusInProgress},
	)

	err := TaskClaim(ns, "task-0001")
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
		Task{ID: "task-0001", Description: "test", State: StatusInProgress},
	)
	ns.State = StatusInProgress

	if err := TaskComplete(ns, "task-0001"); err != nil {
		t.Fatal(err)
	}
	if ns.Tasks[0].State != StatusComplete {
		t.Errorf("expected complete, got %s", ns.Tasks[0].State)
	}
}

func TestTaskComplete_MarksNodeCompleteWhenAllTasksDone(t *testing.T) {
	t.Parallel()
	ns := newLeafWithTasks(
		Task{ID: "task-0001", Description: "first", State: StatusComplete},
		Task{ID: "task-0002", Description: "second", State: StatusInProgress},
	)
	ns.State = StatusInProgress

	if err := TaskComplete(ns, "task-0002"); err != nil {
		t.Fatal(err)
	}
	if ns.State != StatusComplete {
		t.Errorf("node should be complete, got %s", ns.State)
	}
}

func TestTaskComplete_DoesNotMarkNodeCompleteWithRemainingTasks(t *testing.T) {
	t.Parallel()
	ns := newLeafWithTasks(
		Task{ID: "task-0001", Description: "first", State: StatusInProgress},
		Task{ID: "task-0002", Description: "second", State: StatusNotStarted},
	)
	ns.State = StatusInProgress

	if err := TaskComplete(ns, "task-0001"); err != nil {
		t.Fatal(err)
	}
	if ns.State == StatusComplete {
		t.Error("node should not be complete with remaining tasks")
	}
}

func TestTaskComplete_BlocksNodeWhenRemainingTasksBlocked(t *testing.T) {
	t.Parallel()
	ns := newLeafWithTasks(
		Task{ID: "task-0001", Description: "work", State: StatusInProgress},
		Task{ID: "task-0002", Description: "more work", State: StatusComplete},
		Task{ID: "audit", Description: "audit", State: StatusBlocked, IsAudit: true, BlockedReason: "open gaps"},
	)
	ns.State = StatusInProgress

	if err := TaskComplete(ns, "task-0001"); err != nil {
		t.Fatal(err)
	}
	if ns.State != StatusBlocked {
		t.Errorf("node should be blocked when all non-complete tasks are blocked, got %s", ns.State)
	}
}

func TestTaskComplete_StaysInProgressWithNotStartedTasks(t *testing.T) {
	t.Parallel()
	ns := newLeafWithTasks(
		Task{ID: "task-0001", Description: "work", State: StatusInProgress},
		Task{ID: "task-0002", Description: "next", State: StatusNotStarted},
		Task{ID: "audit", Description: "audit", State: StatusBlocked, IsAudit: true},
	)
	ns.State = StatusInProgress

	if err := TaskComplete(ns, "task-0001"); err != nil {
		t.Fatal(err)
	}
	if ns.State != StatusInProgress {
		t.Errorf("node should stay in_progress with not_started tasks remaining, got %s", ns.State)
	}
}

func TestTaskBlock_TransitionsInProgressToBlocked(t *testing.T) {
	t.Parallel()
	ns := newLeafWithTasks(
		Task{ID: "task-0001", Description: "test", State: StatusInProgress},
	)
	ns.State = StatusInProgress

	if err := TaskBlock(ns, "task-0001", "stuck"); err != nil {
		t.Fatal(err)
	}
	if ns.Tasks[0].State != StatusBlocked {
		t.Errorf("expected blocked, got %s", ns.Tasks[0].State)
	}
	if ns.Tasks[0].BlockedReason != "stuck" {
		t.Errorf("expected reason 'stuck', got %q", ns.Tasks[0].BlockedReason)
	}
}

func TestTaskBlock_PreBlocksNotStartedTask(t *testing.T) {
	t.Parallel()
	ns := newLeafWithTasks(
		Task{ID: "task-0001", Description: "test", State: StatusNotStarted},
	)

	err := TaskBlock(ns, "task-0001", "framework does not exist")
	if err != nil {
		t.Fatalf("pre-blocking a not_started task should succeed: %v", err)
	}
	if ns.Tasks[0].State != StatusBlocked {
		t.Errorf("expected blocked, got %s", ns.Tasks[0].State)
	}
	if ns.Tasks[0].BlockedReason != "framework does not exist" {
		t.Errorf("expected reason preserved, got %q", ns.Tasks[0].BlockedReason)
	}
}

func TestTaskBlock_FailsOnComplete(t *testing.T) {
	t.Parallel()
	ns := newLeafWithTasks(
		Task{ID: "task-0001", Description: "test", State: StatusComplete},
	)

	err := TaskBlock(ns, "task-0001", "reason")
	if err == nil {
		t.Error("expected error when blocking complete task")
	}
}

func TestTaskBlock_BlocksNodeWhenAllNonCompleteBlocked(t *testing.T) {
	t.Parallel()
	ns := newLeafWithTasks(
		Task{ID: "task-0001", Description: "first", State: StatusComplete},
		Task{ID: "task-0002", Description: "second", State: StatusInProgress},
	)
	ns.State = StatusInProgress

	if err := TaskBlock(ns, "task-0002", "stuck"); err != nil {
		t.Fatal(err)
	}
	if ns.State != StatusBlocked {
		t.Errorf("node should be blocked when all non-complete tasks are blocked, got %s", ns.State)
	}
}

func TestTaskUnblock_TransitionsBlockedToNotStarted(t *testing.T) {
	t.Parallel()
	ns := newLeafWithTasks(
		Task{ID: "task-0001", Description: "test", State: StatusBlocked, BlockedReason: "stuck", FailureCount: 3},
	)
	ns.State = StatusBlocked

	if err := TaskUnblock(ns, "task-0001"); err != nil {
		t.Fatal(err)
	}
	if ns.Tasks[0].State != StatusNotStarted {
		t.Errorf("expected not_started, got %s", ns.Tasks[0].State)
	}
	if ns.Tasks[0].BlockedReason != "" {
		t.Errorf("block reason should be cleared, got %q", ns.Tasks[0].BlockedReason)
	}
}

func TestTaskUnblock_ResetsFailureCounter(t *testing.T) {
	t.Parallel()
	ns := newLeafWithTasks(
		Task{ID: "task-0001", Description: "test", State: StatusBlocked, FailureCount: 5},
	)
	ns.State = StatusBlocked

	if err := TaskUnblock(ns, "task-0001"); err != nil {
		t.Fatal(err)
	}
	if ns.Tasks[0].FailureCount != 0 {
		t.Errorf("failure count should be reset, got %d", ns.Tasks[0].FailureCount)
	}
}

func TestTaskUnblock_SetsNodeInProgress(t *testing.T) {
	t.Parallel()
	ns := newLeafWithTasks(
		Task{ID: "task-0001", Description: "test", State: StatusBlocked},
	)
	ns.State = StatusBlocked

	if err := TaskUnblock(ns, "task-0001"); err != nil {
		t.Fatal(err)
	}
	if ns.State != StatusInProgress {
		t.Errorf("node should be in_progress after unblock, got %s", ns.State)
	}
}

func TestIncrementFailure_IncrementsAndReturnsCount(t *testing.T) {
	t.Parallel()
	ns := newLeafWithTasks(
		Task{ID: "task-0001", Description: "test", State: StatusInProgress, FailureCount: 0},
	)

	count, err := IncrementFailure(ns, "task-0001")
	if err != nil {
		t.Fatal(err)
	}
	if count != 1 {
		t.Errorf("expected 1, got %d", count)
	}

	count, err = IncrementFailure(ns, "task-0001")
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
	clk := clock.NewFixed(time.Date(2026, 3, 14, 12, 0, 0, 0, time.UTC))

	AddBreadcrumb(ns, "leaf-1/task-0001", "did something", clk)
	AddBreadcrumb(ns, "leaf-1/task-0002", "did more", clk)

	if len(ns.Audit.Breadcrumbs) != 2 {
		t.Fatalf("expected 2 breadcrumbs, got %d", len(ns.Audit.Breadcrumbs))
	}
	if ns.Audit.Breadcrumbs[0].Task != "leaf-1/task-0001" {
		t.Errorf("expected task addr leaf-1/task-1, got %s", ns.Audit.Breadcrumbs[0].Task)
	}
	if ns.Audit.Breadcrumbs[1].Text != "did more" {
		t.Errorf("expected text 'did more', got %s", ns.Audit.Breadcrumbs[1].Text)
	}
	if !ns.Audit.Breadcrumbs[0].Timestamp.Equal(clk.T) {
		t.Errorf("expected timestamp %v, got %v", clk.T, ns.Audit.Breadcrumbs[0].Timestamp)
	}
}
