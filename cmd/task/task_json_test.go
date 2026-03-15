package task

import (
	"testing"

	"github.com/dorkusprime/wolfcastle/internal/state"
)

func TestTaskAdd_JSONOutput(t *testing.T) {
	env := newTestEnv(t)
	env.App.JSONOutput = true
	defer func() { env.App.JSONOutput = false }()

	createLeafNode(t, env, "my-project", "My Project")

	env.RootCmd.SetArgs([]string{"task", "add", "--node", "my-project", "implement the API"})
	if err := env.RootCmd.Execute(); err != nil {
		t.Fatalf("task add (json) failed: %v", err)
	}
}

func TestTaskClaim_JSONOutput(t *testing.T) {
	env := newTestEnv(t)
	env.App.JSONOutput = true
	defer func() { env.App.JSONOutput = false }()

	createLeafNode(t, env, "my-project", "My Project")
	env.App.JSONOutput = false
	env.RootCmd.SetArgs([]string{"task", "add", "--node", "my-project", "work"})
	_ = env.RootCmd.Execute()
	env.App.JSONOutput = true

	env.RootCmd.SetArgs([]string{"task", "claim", "--node", "my-project/task-0001"})
	if err := env.RootCmd.Execute(); err != nil {
		t.Fatalf("task claim (json) failed: %v", err)
	}
}

func TestTaskComplete_JSONOutput(t *testing.T) {
	env := newTestEnv(t)
	createLeafNode(t, env, "my-project", "My Project")

	env.RootCmd.SetArgs([]string{"task", "add", "--node", "my-project", "work"})
	_ = env.RootCmd.Execute()
	env.RootCmd.SetArgs([]string{"task", "claim", "--node", "my-project/task-0001"})
	_ = env.RootCmd.Execute()

	env.App.JSONOutput = true
	defer func() { env.App.JSONOutput = false }()

	env.RootCmd.SetArgs([]string{"task", "complete", "--node", "my-project/task-0001"})
	if err := env.RootCmd.Execute(); err != nil {
		t.Fatalf("task complete (json) failed: %v", err)
	}
}

func TestTaskBlock_JSONOutput(t *testing.T) {
	env := newTestEnv(t)
	createLeafNode(t, env, "my-project", "My Project")

	env.RootCmd.SetArgs([]string{"task", "add", "--node", "my-project", "work"})
	_ = env.RootCmd.Execute()
	env.RootCmd.SetArgs([]string{"task", "claim", "--node", "my-project/task-0001"})
	_ = env.RootCmd.Execute()

	env.App.JSONOutput = true
	defer func() { env.App.JSONOutput = false }()

	env.RootCmd.SetArgs([]string{"task", "block", "--node", "my-project/task-0001", "stuck"})
	if err := env.RootCmd.Execute(); err != nil {
		t.Fatalf("task block (json) failed: %v", err)
	}
}

func TestTaskUnblock_JSONOutput(t *testing.T) {
	env := newTestEnv(t)
	createLeafNode(t, env, "my-project", "My Project")

	env.RootCmd.SetArgs([]string{"task", "add", "--node", "my-project", "work"})
	_ = env.RootCmd.Execute()
	env.RootCmd.SetArgs([]string{"task", "claim", "--node", "my-project/task-0001"})
	_ = env.RootCmd.Execute()
	env.RootCmd.SetArgs([]string{"task", "block", "--node", "my-project/task-0001", "stuck"})
	_ = env.RootCmd.Execute()

	env.App.JSONOutput = true
	defer func() { env.App.JSONOutput = false }()

	env.RootCmd.SetArgs([]string{"task", "unblock", "--node", "my-project/task-0001"})
	if err := env.RootCmd.Execute(); err != nil {
		t.Fatalf("task unblock (json) failed: %v", err)
	}
}

func TestTaskAdd_InvalidNodeAddress(t *testing.T) {
	env := newTestEnv(t)
	createLeafNode(t, env, "my-project", "My Project")

	env.RootCmd.SetArgs([]string{"task", "add", "--node", "INVALID", "description"})
	err := env.RootCmd.Execute()
	if err == nil {
		t.Error("expected error for invalid node address")
	}
}

func TestTaskAdd_NonexistentNode(t *testing.T) {
	env := newTestEnv(t)

	env.RootCmd.SetArgs([]string{"task", "add", "--node", "nonexistent", "description"})
	err := env.RootCmd.Execute()
	if err == nil {
		t.Error("expected error for nonexistent node")
	}
}

func TestTaskClaim_AlreadyClaimed(t *testing.T) {
	env := newTestEnv(t)
	createLeafNode(t, env, "my-project", "My Project")

	env.RootCmd.SetArgs([]string{"task", "add", "--node", "my-project", "work"})
	_ = env.RootCmd.Execute()
	env.RootCmd.SetArgs([]string{"task", "claim", "--node", "my-project/task-0001"})
	_ = env.RootCmd.Execute()

	// Try claiming again
	env.RootCmd.SetArgs([]string{"task", "claim", "--node", "my-project/task-0001"})
	err := env.RootCmd.Execute()
	if err == nil {
		t.Error("expected error when claiming already in-progress task")
	}
}

func TestTaskComplete_NotInProgress(t *testing.T) {
	env := newTestEnv(t)
	createLeafNode(t, env, "my-project", "My Project")

	env.RootCmd.SetArgs([]string{"task", "add", "--node", "my-project", "work"})
	_ = env.RootCmd.Execute()

	// Try completing without claiming
	env.RootCmd.SetArgs([]string{"task", "complete", "--node", "my-project/task-0001"})
	err := env.RootCmd.Execute()
	if err == nil {
		t.Error("expected error when completing not_started task")
	}
}

func TestTaskBlock_NotInProgress(t *testing.T) {
	env := newTestEnv(t)
	createLeafNode(t, env, "my-project", "My Project")

	env.RootCmd.SetArgs([]string{"task", "add", "--node", "my-project", "work"})
	_ = env.RootCmd.Execute()

	// Try blocking without claiming
	env.RootCmd.SetArgs([]string{"task", "block", "--node", "my-project/task-0001", "reason"})
	err := env.RootCmd.Execute()
	if err == nil {
		t.Error("expected error when blocking not_started task")
	}
}

func TestTaskUnblock_NotBlocked(t *testing.T) {
	env := newTestEnv(t)
	createLeafNode(t, env, "my-project", "My Project")

	env.RootCmd.SetArgs([]string{"task", "add", "--node", "my-project", "work"})
	_ = env.RootCmd.Execute()

	// Try unblocking a not_started task
	env.RootCmd.SetArgs([]string{"task", "unblock", "--node", "my-project/task-0001"})
	err := env.RootCmd.Execute()
	if err == nil {
		t.Error("expected error when unblocking non-blocked task")
	}
}

func TestTaskBlockPropagatesNodeState(t *testing.T) {
	env := newTestEnv(t)
	createLeafNode(t, env, "my-project", "My Project")

	env.RootCmd.SetArgs([]string{"task", "add", "--node", "my-project", "work"})
	_ = env.RootCmd.Execute()
	env.RootCmd.SetArgs([]string{"task", "claim", "--node", "my-project/task-0001"})
	_ = env.RootCmd.Execute()

	// Claim audit too so we can block the only non-complete task
	env.RootCmd.SetArgs([]string{"task", "claim", "--node", "my-project/audit"})
	_ = env.RootCmd.Execute()
	env.RootCmd.SetArgs([]string{"task", "complete", "--node", "my-project/audit"})
	_ = env.RootCmd.Execute()

	// Block the remaining task
	env.RootCmd.SetArgs([]string{"task", "block", "--node", "my-project/task-0001", "stuck"})
	_ = env.RootCmd.Execute()

	ns := loadNodeState(t, env, "my-project")
	if ns.State != state.StatusBlocked {
		t.Errorf("node should be blocked when all non-complete tasks are blocked, got %s", ns.State)
	}
}
