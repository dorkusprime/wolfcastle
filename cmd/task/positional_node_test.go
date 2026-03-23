package task

import (
	"strings"
	"testing"

	"github.com/dorkusprime/wolfcastle/internal/state"
)

// ---------------------------------------------------------------------------
// Positional task address: claim
// ---------------------------------------------------------------------------

func TestTaskClaim_Positional(t *testing.T) {
	env := newTestEnv(t)
	env.createLeafNode(t, "my-project", "My Project")

	env.RootCmd.SetArgs([]string{"task", "add", "--node", "my-project", "do something"})
	_ = env.RootCmd.Execute()

	env.RootCmd.SetArgs([]string{"task", "claim", "my-project/task-0001"})
	if err := env.RootCmd.Execute(); err != nil {
		t.Fatalf("positional claim failed: %v", err)
	}

	ns := env.loadNodeState(t, "my-project")
	if ns.Tasks[0].State != state.StatusInProgress {
		t.Errorf("expected in_progress, got %s", ns.Tasks[0].State)
	}
}

func TestTaskClaim_BothPositionalAndFlag(t *testing.T) {
	env := newTestEnv(t)

	env.RootCmd.SetArgs([]string{"task", "claim", "my-project/task-0001", "--node", "my-project/task-0001"})
	err := env.RootCmd.Execute()
	if err == nil {
		t.Fatal("expected error when both positional and --node are provided")
	}
	if !strings.Contains(err.Error(), "not both") {
		t.Errorf("expected 'not both' in error, got: %v", err)
	}
}

func TestTaskClaim_NoArgs(t *testing.T) {
	env := newTestEnv(t)

	env.RootCmd.SetArgs([]string{"task", "claim"})
	err := env.RootCmd.Execute()
	if err == nil {
		t.Fatal("expected error when no task address provided")
	}
	if !strings.Contains(err.Error(), "task address required") {
		t.Errorf("expected 'task address required' in error, got: %v", err)
	}
}

// ---------------------------------------------------------------------------
// Positional task address: complete
// ---------------------------------------------------------------------------

func TestTaskComplete_Positional(t *testing.T) {
	env := newTestEnv(t)
	env.createLeafNode(t, "my-project", "My Project")

	env.RootCmd.SetArgs([]string{"task", "add", "--node", "my-project", "do work"})
	_ = env.RootCmd.Execute()
	env.RootCmd.SetArgs([]string{"task", "claim", "my-project/task-0001"})
	_ = env.RootCmd.Execute()

	env.RootCmd.SetArgs([]string{"task", "complete", "my-project/task-0001"})
	if err := env.RootCmd.Execute(); err != nil {
		t.Fatalf("positional complete failed: %v", err)
	}

	ns := env.loadNodeState(t, "my-project")
	if ns.Tasks[0].State != state.StatusComplete {
		t.Errorf("expected complete, got %s", ns.Tasks[0].State)
	}
}

func TestTaskComplete_BothPositionalAndFlag(t *testing.T) {
	env := newTestEnv(t)

	env.RootCmd.SetArgs([]string{"task", "complete", "my-project/task-0001", "--node", "my-project/task-0001"})
	err := env.RootCmd.Execute()
	if err == nil {
		t.Fatal("expected error when both positional and --node are provided")
	}
	if !strings.Contains(err.Error(), "not both") {
		t.Errorf("expected 'not both' in error, got: %v", err)
	}
}

// ---------------------------------------------------------------------------
// Positional task address: block
// ---------------------------------------------------------------------------

func TestTaskBlock_Positional(t *testing.T) {
	env := newTestEnv(t)
	env.createLeafNode(t, "my-project", "My Project")

	env.RootCmd.SetArgs([]string{"task", "add", "--node", "my-project", "work item"})
	_ = env.RootCmd.Execute()
	env.RootCmd.SetArgs([]string{"task", "claim", "my-project/task-0001"})
	_ = env.RootCmd.Execute()

	env.RootCmd.SetArgs([]string{"task", "block", "my-project/task-0001", "waiting on API"})
	if err := env.RootCmd.Execute(); err != nil {
		t.Fatalf("positional block failed: %v", err)
	}

	ns := env.loadNodeState(t, "my-project")
	if ns.Tasks[0].State != state.StatusBlocked {
		t.Errorf("expected blocked, got %s", ns.Tasks[0].State)
	}
	if ns.Tasks[0].BlockedReason != "waiting on API" {
		t.Errorf("unexpected block reason: %s", ns.Tasks[0].BlockedReason)
	}
}

func TestTaskBlock_BothPositionalAndFlag(t *testing.T) {
	env := newTestEnv(t)

	env.RootCmd.SetArgs([]string{"task", "block", "my-project/task-0001", "--node", "my-project/task-0001", "reason"})
	err := env.RootCmd.Execute()
	if err == nil {
		t.Fatal("expected error when both positional and --node are provided")
	}
	if !strings.Contains(err.Error(), "not both") {
		t.Errorf("expected 'not both' in error, got: %v", err)
	}
}

func TestTaskBlock_NoArgs(t *testing.T) {
	env := newTestEnv(t)

	env.RootCmd.SetArgs([]string{"task", "block"})
	err := env.RootCmd.Execute()
	if err == nil {
		t.Fatal("expected error when no args provided")
	}
}

func TestTaskBlock_SingleArgNoFlag(t *testing.T) {
	env := newTestEnv(t)

	// Single positional arg without --node is ambiguous
	env.RootCmd.SetArgs([]string{"task", "block", "my-project/task-0001"})
	err := env.RootCmd.Execute()
	if err == nil {
		t.Fatal("expected error for single arg without --node")
	}
	if !strings.Contains(err.Error(), "two arguments required") {
		t.Errorf("expected 'two arguments required' in error, got: %v", err)
	}
}

// ---------------------------------------------------------------------------
// Positional task address: unblock
// ---------------------------------------------------------------------------

func TestTaskUnblock_Positional(t *testing.T) {
	env := newTestEnv(t)
	env.createLeafNode(t, "my-project", "My Project")

	env.RootCmd.SetArgs([]string{"task", "add", "--node", "my-project", "work"})
	_ = env.RootCmd.Execute()
	env.RootCmd.SetArgs([]string{"task", "claim", "my-project/task-0001"})
	_ = env.RootCmd.Execute()
	env.RootCmd.SetArgs([]string{"task", "block", "--node", "my-project/task-0001", "stuck"})
	_ = env.RootCmd.Execute()

	env.RootCmd.SetArgs([]string{"task", "unblock", "my-project/task-0001"})
	if err := env.RootCmd.Execute(); err != nil {
		t.Fatalf("positional unblock failed: %v", err)
	}

	ns := env.loadNodeState(t, "my-project")
	if ns.Tasks[0].State != state.StatusNotStarted {
		t.Errorf("expected not_started after unblock, got %s", ns.Tasks[0].State)
	}
}

func TestTaskUnblock_BothPositionalAndFlag(t *testing.T) {
	env := newTestEnv(t)

	env.RootCmd.SetArgs([]string{"task", "unblock", "my-project/task-0001", "--node", "my-project/task-0001"})
	err := env.RootCmd.Execute()
	if err == nil {
		t.Fatal("expected error when both positional and --node are provided")
	}
	if !strings.Contains(err.Error(), "not both") {
		t.Errorf("expected 'not both' in error, got: %v", err)
	}
}

// ---------------------------------------------------------------------------
// Positional task address: amend
// ---------------------------------------------------------------------------

func TestTaskAmend_Positional(t *testing.T) {
	env := newTestEnv(t)
	env.createLeafNode(t, "my-project", "My Project")

	env.RootCmd.SetArgs([]string{"task", "add", "--node", "my-project", "original"})
	_ = env.RootCmd.Execute()

	env.RootCmd.SetArgs([]string{"task", "amend", "my-project/task-0001", "--body", "updated body"})
	if err := env.RootCmd.Execute(); err != nil {
		t.Fatalf("positional amend failed: %v", err)
	}

	ns := env.loadNodeState(t, "my-project")
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

func TestTaskAmend_BothPositionalAndFlag(t *testing.T) {
	env := newTestEnv(t)

	env.RootCmd.SetArgs([]string{"task", "amend", "my-project/task-0001", "--node", "my-project/task-0001", "--body", "x"})
	err := env.RootCmd.Execute()
	if err == nil {
		t.Fatal("expected error when both positional and --node are provided")
	}
	if !strings.Contains(err.Error(), "not both") {
		t.Errorf("expected 'not both' in error, got: %v", err)
	}
}
