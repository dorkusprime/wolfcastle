package task

import (
	"strings"
	"testing"

	"github.com/dorkusprime/wolfcastle/internal/state"
)

// ---------------------------------------------------------------------------
// task amend
// ---------------------------------------------------------------------------

func TestTaskAmend_Body(t *testing.T) {
	env := newTestEnv(t)
	createLeafNode(t, env, "my-project", "My Project")

	env.RootCmd.SetArgs([]string{"task", "add", "--node", "my-project", "original body"})
	if err := env.RootCmd.Execute(); err != nil {
		t.Fatalf("add: %v", err)
	}

	env.RootCmd.SetArgs([]string{"task", "amend", "--node", "my-project/task-0001", "--body", "updated body"})
	if err := env.RootCmd.Execute(); err != nil {
		t.Fatalf("amend body: %v", err)
	}

	ns := loadNodeState(t, env, "my-project")
	for _, task := range ns.Tasks {
		if task.ID == "task-0001" {
			if task.Body != "updated body" {
				t.Errorf("expected body 'updated body', got %q", task.Body)
			}
			return
		}
	}
	t.Error("task-0001 not found")
}

func TestTaskAmend_Type(t *testing.T) {
	env := newTestEnv(t)
	createLeafNode(t, env, "my-project", "My Project")

	env.RootCmd.SetArgs([]string{"task", "add", "--node", "my-project", "some task"})
	if err := env.RootCmd.Execute(); err != nil {
		t.Fatalf("add: %v", err)
	}

	env.RootCmd.SetArgs([]string{"task", "amend", "--node", "my-project/task-0001", "--type", "implementation"})
	if err := env.RootCmd.Execute(); err != nil {
		t.Fatalf("amend type: %v", err)
	}

	ns := loadNodeState(t, env, "my-project")
	for _, task := range ns.Tasks {
		if task.ID == "task-0001" {
			if task.TaskType != "implementation" {
				t.Errorf("expected type 'implementation', got %q", task.TaskType)
			}
			return
		}
	}
	t.Error("task-0001 not found")
}

func TestTaskAmend_AddSliceFields(t *testing.T) {
	env := newTestEnv(t)
	createLeafNode(t, env, "my-project", "My Project")

	env.RootCmd.SetArgs([]string{"task", "add", "--node", "my-project", "task with extras"})
	if err := env.RootCmd.Execute(); err != nil {
		t.Fatalf("add: %v", err)
	}

	env.RootCmd.SetArgs([]string{
		"task", "amend", "--node", "my-project/task-0001",
		"--add-deliverable", "docs/api.md",
		"--add-constraint", "must be idempotent",
		"--add-acceptance", "passes integration tests",
		"--add-reference", "RFC-1234",
	})
	if err := env.RootCmd.Execute(); err != nil {
		t.Fatalf("amend slice fields: %v", err)
	}

	ns := loadNodeState(t, env, "my-project")
	for _, task := range ns.Tasks {
		if task.ID != "task-0001" {
			continue
		}
		if len(task.Deliverables) != 1 || task.Deliverables[0] != "docs/api.md" {
			t.Errorf("deliverables: got %v", task.Deliverables)
		}
		if len(task.Constraints) != 1 || task.Constraints[0] != "must be idempotent" {
			t.Errorf("constraints: got %v", task.Constraints)
		}
		if len(task.AcceptanceCriteria) != 1 || task.AcceptanceCriteria[0] != "passes integration tests" {
			t.Errorf("acceptance: got %v", task.AcceptanceCriteria)
		}
		if len(task.References) != 1 || task.References[0] != "RFC-1234" {
			t.Errorf("references: got %v", task.References)
		}
		return
	}
	t.Error("task-0001 not found")
}

func TestTaskAmend_RefuseInProgress(t *testing.T) {
	env := newTestEnv(t)
	createLeafNode(t, env, "my-project", "My Project")

	env.RootCmd.SetArgs([]string{"task", "add", "--node", "my-project", "some work"})
	_ = env.RootCmd.Execute()
	env.RootCmd.SetArgs([]string{"task", "claim", "--node", "my-project/task-0001"})
	_ = env.RootCmd.Execute()

	env.RootCmd.SetArgs([]string{"task", "amend", "--node", "my-project/task-0001", "--body", "nope"})
	err := env.RootCmd.Execute()
	if err == nil {
		t.Fatal("expected error amending an in_progress task")
	}
	if !strings.Contains(err.Error(), "in_progress") {
		t.Errorf("error should mention in_progress, got: %v", err)
	}
}

func TestTaskAmend_RefuseComplete(t *testing.T) {
	env := newTestEnv(t)
	createLeafNode(t, env, "my-project", "My Project")

	env.RootCmd.SetArgs([]string{"task", "add", "--node", "my-project", "finish me"})
	_ = env.RootCmd.Execute()
	env.RootCmd.SetArgs([]string{"task", "claim", "--node", "my-project/task-0001"})
	_ = env.RootCmd.Execute()
	env.RootCmd.SetArgs([]string{"task", "complete", "--node", "my-project/task-0001"})
	_ = env.RootCmd.Execute()

	env.RootCmd.SetArgs([]string{"task", "amend", "--node", "my-project/task-0001", "--body", "too late"})
	err := env.RootCmd.Execute()
	if err == nil {
		t.Fatal("expected error amending a complete task")
	}
	if !strings.Contains(err.Error(), string(state.StatusComplete)) {
		t.Errorf("error should mention complete, got: %v", err)
	}
}

func TestTaskAmend_InvalidType(t *testing.T) {
	env := newTestEnv(t)
	createLeafNode(t, env, "my-project", "My Project")

	env.RootCmd.SetArgs([]string{"task", "add", "--node", "my-project", "typed task"})
	_ = env.RootCmd.Execute()

	env.RootCmd.SetArgs([]string{"task", "amend", "--node", "my-project/task-0001", "--type", "bogus"})
	err := env.RootCmd.Execute()
	if err == nil {
		t.Fatal("expected error for invalid task type")
	}
	if !strings.Contains(err.Error(), "invalid task type") {
		t.Errorf("error should mention 'invalid task type', got: %v", err)
	}
}

func TestTaskAmend_TaskNotFound(t *testing.T) {
	env := newTestEnv(t)
	createLeafNode(t, env, "my-project", "My Project")

	env.RootCmd.SetArgs([]string{"task", "amend", "--node", "my-project/task-9999", "--body", "ghost"})
	err := env.RootCmd.Execute()
	if err == nil {
		t.Fatal("expected error for nonexistent task")
	}
	if !strings.Contains(err.Error(), "task-9999") {
		t.Errorf("error should mention task-9999, got: %v", err)
	}
}

// ---------------------------------------------------------------------------
// appendUnique unit tests
// ---------------------------------------------------------------------------

func TestAppendUnique_Deduplicates(t *testing.T) {
	base := []string{"a", "b"}
	result := appendUnique(base, []string{"b", "c", "a", "d"})

	if len(result) != 4 {
		t.Fatalf("expected 4 items, got %d: %v", len(result), result)
	}

	expected := []string{"a", "b", "c", "d"}
	for i, want := range expected {
		if result[i] != want {
			t.Errorf("result[%d] = %q, want %q", i, result[i], want)
		}
	}
}

func TestAppendUnique_EmptyAdditions(t *testing.T) {
	base := []string{"a", "b"}
	result := appendUnique(base, nil)
	if len(result) != 2 {
		t.Errorf("expected base unchanged, got %v", result)
	}
}

func TestAppendUnique_EmptyBase(t *testing.T) {
	result := appendUnique(nil, []string{"x", "y"})
	if len(result) != 2 || result[0] != "x" || result[1] != "y" {
		t.Errorf("expected [x y], got %v", result)
	}
}
