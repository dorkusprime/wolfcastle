package daemon

import (
	"path/filepath"
	"strings"
	"testing"

	"github.com/dorkusprime/wolfcastle/internal/state"
)

// ═══════════════════════════════════════════════════════════════════════════
// checkSpecReviewNeeded
// ═══════════════════════════════════════════════════════════════════════════

func TestCheckSpecReviewNeeded_CreatesReviewForSpecTask(t *testing.T) {
	t.Parallel()
	d := testDaemon(t)

	setupLeafNode(t, d, "my-node", []state.Task{
		{ID: "task-0001", Description: "Write auth spec", State: state.StatusComplete, TaskType: "spec"},
	})

	created := d.checkSpecReviewNeeded("my-node", "task-0001")
	if !created {
		t.Fatal("expected review task to be created")
	}

	ns, err := d.Store.ReadNode("my-node")
	if err != nil {
		t.Fatal(err)
	}

	// Find the review task.
	var review *state.Task
	for i := range ns.Tasks {
		if ns.Tasks[i].ID == "task-0001-review" {
			review = &ns.Tasks[i]
			break
		}
	}
	if review == nil {
		t.Fatal("review task not found in node state")
	}
	if review.TaskType != specReviewTaskType {
		t.Errorf("expected task type %q, got %q", specReviewTaskType, review.TaskType)
	}
	if review.State != state.StatusNotStarted {
		t.Errorf("expected state not_started, got %s", review.State)
	}
	if !strings.Contains(review.Description, "Write auth spec") {
		t.Errorf("review description should reference original spec, got %q", review.Description)
	}
	if !strings.Contains(review.Body, "task-0001") {
		t.Errorf("review body should reference the spec task ID, got %q", review.Body)
	}
}

func TestCheckSpecReviewNeeded_SkipsNonSpecTask(t *testing.T) {
	t.Parallel()
	d := testDaemon(t)

	setupLeafNode(t, d, "my-node", []state.Task{
		{ID: "task-0001", Description: "Implement auth", State: state.StatusComplete, TaskType: "implementation"},
	})

	created := d.checkSpecReviewNeeded("my-node", "task-0001")
	if created {
		t.Error("should not create review for non-spec task")
	}
}

func TestCheckSpecReviewNeeded_SkipsEmptyTaskType(t *testing.T) {
	t.Parallel()
	d := testDaemon(t)

	setupLeafNode(t, d, "my-node", []state.Task{
		{ID: "task-0001", Description: "Do something", State: state.StatusComplete},
	})

	created := d.checkSpecReviewNeeded("my-node", "task-0001")
	if created {
		t.Error("should not create review for task without type")
	}
}

func TestCheckSpecReviewNeeded_IdempotentOnSecondCall(t *testing.T) {
	t.Parallel()
	d := testDaemon(t)

	setupLeafNode(t, d, "my-node", []state.Task{
		{ID: "task-0001", Description: "Write spec", State: state.StatusComplete, TaskType: "spec"},
	})

	// First call creates the review.
	d.checkSpecReviewNeeded("my-node", "task-0001")

	// Second call should be a no-op.
	created := d.checkSpecReviewNeeded("my-node", "task-0001")
	if created {
		t.Error("should not create duplicate review")
	}

	// Verify only one review task exists.
	ns, err := d.Store.ReadNode("my-node")
	if err != nil {
		t.Fatal(err)
	}
	count := 0
	for _, task := range ns.Tasks {
		if task.ID == "task-0001-review" {
			count++
		}
	}
	if count != 1 {
		t.Errorf("expected exactly 1 review task, found %d", count)
	}
}

func TestCheckSpecReviewNeeded_IncludesNodeSpecs(t *testing.T) {
	t.Parallel()
	d := testDaemon(t)

	projDir := d.Store.Dir()
	ns := state.NewNodeState("my-node", "My Node", state.NodeLeaf)
	ns.Specs = []string{"auth-flow-spec.md"}
	ns.Tasks = []state.Task{
		{ID: "task-0001", Description: "Auth spec", State: state.StatusComplete, TaskType: "spec",
			References: []string{"docs/api.md"}},
	}
	writeJSON(t, filepath.Join(projDir, "my-node", "state.json"), ns)

	idx := state.NewRootIndex()
	idx.Root = []string{"my-node"}
	idx.Nodes["my-node"] = state.IndexEntry{
		Name: "My Node", Type: state.NodeLeaf, State: state.StatusInProgress, Address: "my-node",
	}
	writeJSON(t, filepath.Join(projDir, "state.json"), idx)

	d.checkSpecReviewNeeded("my-node", "task-0001")

	updated, err := d.Store.ReadNode("my-node")
	if err != nil {
		t.Fatal(err)
	}
	for _, task := range updated.Tasks {
		if task.ID == "task-0001-review" {
			// Should have both the task reference and the node spec.
			hasAPI := false
			hasAuthFlow := false
			for _, r := range task.References {
				if r == "docs/api.md" {
					hasAPI = true
				}
				if r == "auth-flow-spec.md" {
					hasAuthFlow = true
				}
			}
			if !hasAPI {
				t.Error("review task should include task-level references")
			}
			if !hasAuthFlow {
				t.Error("review task should include node-level specs")
			}
			return
		}
	}
	t.Fatal("review task not found")
}

func TestCheckSpecReviewNeeded_MissingNode(t *testing.T) {
	t.Parallel()
	d := testDaemon(t)

	created := d.checkSpecReviewNeeded("nonexistent", "task-0001")
	if created {
		t.Error("should return false for nonexistent node")
	}
}

func TestCheckSpecReviewNeeded_MissingTask(t *testing.T) {
	t.Parallel()
	d := testDaemon(t)

	setupLeafNode(t, d, "my-node", []state.Task{
		{ID: "task-0001", Description: "Something", State: state.StatusComplete, TaskType: "spec"},
	})

	created := d.checkSpecReviewNeeded("my-node", "task-9999")
	if created {
		t.Error("should return false for nonexistent task")
	}
}

func TestCheckSpecReviewNeeded_InsertsBeforeAudit(t *testing.T) {
	t.Parallel()
	d := testDaemon(t)

	setupLeafNode(t, d, "my-node", []state.Task{
		{ID: "task-0001", Description: "Write spec", State: state.StatusComplete, TaskType: "spec"},
		{ID: "audit", Description: "Audit", State: state.StatusNotStarted, IsAudit: true},
	})

	d.checkSpecReviewNeeded("my-node", "task-0001")

	ns, err := d.Store.ReadNode("my-node")
	if err != nil {
		t.Fatal(err)
	}

	// The audit task should still be last.
	last := ns.Tasks[len(ns.Tasks)-1]
	if !last.IsAudit {
		t.Errorf("audit task should remain last, but got %s", last.ID)
	}

	// Review should be second-to-last (before audit).
	if len(ns.Tasks) < 3 {
		t.Fatalf("expected at least 3 tasks, got %d", len(ns.Tasks))
	}
	secondToLast := ns.Tasks[len(ns.Tasks)-2]
	if secondToLast.ID != "task-0001-review" {
		t.Errorf("review should be before audit, got %s", secondToLast.ID)
	}
}

// ═══════════════════════════════════════════════════════════════════════════
// handleSpecReviewBlocked
// ═══════════════════════════════════════════════════════════════════════════

func TestHandleSpecReviewBlocked_FeedsBackToSpecTask(t *testing.T) {
	t.Parallel()
	d := testDaemon(t)

	setupLeafNode(t, d, "my-node", []state.Task{
		{ID: "task-0001", Description: "Write spec", State: state.StatusComplete, TaskType: "spec"},
		{ID: "task-0001-review", Description: "Review spec", State: state.StatusBlocked,
			TaskType: specReviewTaskType, BlockedReason: "Missing error handling in Section 3"},
	})

	handled := d.handleSpecReviewBlocked("my-node", "task-0001-review")
	if !handled {
		t.Fatal("expected review block to be handled")
	}

	ns, err := d.Store.ReadNode("my-node")
	if err != nil {
		t.Fatal(err)
	}

	// The original spec task should be reset to not_started.
	for _, task := range ns.Tasks {
		if task.ID == "task-0001" {
			if task.State != state.StatusNotStarted {
				t.Errorf("spec task should be reset to not_started, got %s", task.State)
			}
			if !strings.Contains(task.Body, "Missing error handling in Section 3") {
				t.Error("spec task body should contain review feedback")
			}
			if !strings.Contains(task.Body, "Review Feedback") {
				t.Error("spec task body should have review feedback header")
			}
			return
		}
	}
	t.Fatal("spec task not found after feedback delivery")
}

func TestHandleSpecReviewBlocked_SkipsNonReviewTask(t *testing.T) {
	t.Parallel()
	d := testDaemon(t)

	setupLeafNode(t, d, "my-node", []state.Task{
		{ID: "task-0001", Description: "Some task", State: state.StatusBlocked,
			TaskType: "implementation", BlockedReason: "missing dep"},
	})

	handled := d.handleSpecReviewBlocked("my-node", "task-0001")
	if handled {
		t.Error("should not handle non-review task")
	}
}

func TestHandleSpecReviewBlocked_SkipsTaskWithoutReviewSuffix(t *testing.T) {
	t.Parallel()
	d := testDaemon(t)

	setupLeafNode(t, d, "my-node", []state.Task{
		{ID: "task-0001", Description: "Some task", State: state.StatusBlocked,
			TaskType: specReviewTaskType, BlockedReason: "issues found"},
	})

	handled := d.handleSpecReviewBlocked("my-node", "task-0001")
	if handled {
		t.Error("should not handle task without -review suffix")
	}
}

func TestHandleSpecReviewBlocked_ResetsFailureCount(t *testing.T) {
	t.Parallel()
	d := testDaemon(t)

	setupLeafNode(t, d, "my-node", []state.Task{
		{ID: "task-0001", Description: "Write spec", State: state.StatusComplete, TaskType: "spec",
			FailureCount: 2, LastFailureType: "no_progress"},
		{ID: "task-0001-review", Description: "Review spec", State: state.StatusBlocked,
			TaskType: specReviewTaskType, BlockedReason: "Contradictions found"},
	})

	d.handleSpecReviewBlocked("my-node", "task-0001-review")

	ns, err := d.Store.ReadNode("my-node")
	if err != nil {
		t.Fatal(err)
	}

	for _, task := range ns.Tasks {
		if task.ID == "task-0001" {
			if task.FailureCount != 0 {
				t.Errorf("failure count should be reset to 0, got %d", task.FailureCount)
			}
			if task.LastFailureType != "" {
				t.Errorf("last failure type should be cleared, got %q", task.LastFailureType)
			}
			return
		}
	}
	t.Fatal("spec task not found")
}

func TestHandleSpecReviewBlocked_MissingNode(t *testing.T) {
	t.Parallel()
	d := testDaemon(t)

	handled := d.handleSpecReviewBlocked("nonexistent", "task-0001-review")
	if handled {
		t.Error("should return false for nonexistent node")
	}
}

func TestHandleSpecReviewBlocked_EmptyBlockedReason(t *testing.T) {
	t.Parallel()
	d := testDaemon(t)

	setupLeafNode(t, d, "my-node", []state.Task{
		{ID: "task-0001", Description: "Write spec", State: state.StatusComplete, TaskType: "spec"},
		{ID: "task-0001-review", Description: "Review spec", State: state.StatusBlocked,
			TaskType: specReviewTaskType, BlockedReason: ""},
	})

	handled := d.handleSpecReviewBlocked("my-node", "task-0001-review")
	if !handled {
		t.Fatal("expected handling even with empty blocked reason")
	}

	ns, err := d.Store.ReadNode("my-node")
	if err != nil {
		t.Fatal(err)
	}

	for _, task := range ns.Tasks {
		if task.ID == "task-0001" {
			if !strings.Contains(task.Body, "see review task body") {
				t.Error("should have fallback feedback text when blocked reason is empty")
			}
			return
		}
	}
	t.Fatal("spec task not found")
}
