package task

import (
	"testing"
)

// ---------------------------------------------------------------------------
// All five commands: empty --node "" guards
// These exercise the belt-and-suspenders guard behind MarkFlagRequired.
// ---------------------------------------------------------------------------

func TestTaskAdd_EmptyNodeGuard(t *testing.T) {
	env := newTestEnv(t)
	env.createLeafNode(t, "my-project", "My Project")

	env.RootCmd.SetArgs([]string{"task", "add", "--node", "", "description"})
	err := env.RootCmd.Execute()
	if err == nil {
		t.Error("expected error when --node is empty for task add")
	}
}

func TestTaskClaim_EmptyNodeGuard(t *testing.T) {
	env := newTestEnv(t)

	env.RootCmd.SetArgs([]string{"task", "claim", "--node", ""})
	err := env.RootCmd.Execute()
	if err == nil {
		t.Error("expected error when --node is empty for task claim")
	}
}

func TestTaskComplete_EmptyNodeGuard(t *testing.T) {
	env := newTestEnv(t)

	env.RootCmd.SetArgs([]string{"task", "complete", "--node", ""})
	err := env.RootCmd.Execute()
	if err == nil {
		t.Error("expected error when --node is empty for task complete")
	}
}

func TestTaskBlock_EmptyNodeGuard(t *testing.T) {
	env := newTestEnv(t)

	env.RootCmd.SetArgs([]string{"task", "block", "--node", "", "reason"})
	err := env.RootCmd.Execute()
	if err == nil {
		t.Error("expected error when --node is empty for task block")
	}
}

func TestTaskUnblock_EmptyNodeGuard(t *testing.T) {
	env := newTestEnv(t)

	env.RootCmd.SetArgs([]string{"task", "unblock", "--node", ""})
	err := env.RootCmd.Execute()
	if err == nil {
		t.Error("expected error when --node is empty for task unblock")
	}
}
