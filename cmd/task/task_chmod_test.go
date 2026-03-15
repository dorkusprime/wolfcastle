package task

import (
	"os"
	"path/filepath"
	"runtime"
	"testing"
)

// ── task commands — SaveNodeState error via read-only state dir ──────

func TestTaskAdd_SaveNodeStateError_ReadOnly(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("permission test not reliable on Windows")
	}

	env := newTestEnv(t)
	createLeafNode(t, env, "locked-proj", "Locked Project")

	// Lock the node directory so SaveNodeState fails.
	nodeDir := filepath.Join(env.ProjectsDir, "locked-proj")
	_ = os.Chmod(nodeDir, 0555)
	t.Cleanup(func() { _ = os.Chmod(nodeDir, 0755) })

	env.RootCmd.SetArgs([]string{"task", "add", "--node", "locked-proj", "new task"})
	err := env.RootCmd.Execute()
	if err == nil {
		t.Error("expected error when SaveNodeState fails for task add")
	}
}

func TestTaskClaim_SaveNodeStateError_ReadOnly(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("permission test not reliable on Windows")
	}

	env := newTestEnv(t)
	createLeafNode(t, env, "claim-proj", "Claim Project")

	// Add a task first (with writable dir).
	env.RootCmd.SetArgs([]string{"task", "add", "--node", "claim-proj", "claimable task"})
	if err := env.RootCmd.Execute(); err != nil {
		t.Fatal(err)
	}

	// Lock the node directory so SaveNodeState fails during claim.
	nodeDir := filepath.Join(env.ProjectsDir, "claim-proj")
	_ = os.Chmod(nodeDir, 0555)
	t.Cleanup(func() { _ = os.Chmod(nodeDir, 0755) })

	env.RootCmd.SetArgs([]string{"task", "claim", "--node", "claim-proj/task-0001"})
	err := env.RootCmd.Execute()
	if err == nil {
		t.Error("expected error when SaveNodeState fails for task claim")
	}
}

func TestTaskComplete_SaveNodeStateError_ReadOnly(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("permission test not reliable on Windows")
	}

	env := newTestEnv(t)
	createLeafNode(t, env, "complete-proj", "Complete Project")

	// Add and claim a task.
	env.RootCmd.SetArgs([]string{"task", "add", "--node", "complete-proj", "completable"})
	_ = env.RootCmd.Execute()
	env.RootCmd.SetArgs([]string{"task", "claim", "--node", "complete-proj/task-0001"})
	_ = env.RootCmd.Execute()

	// Lock the node directory so SaveNodeState fails during complete.
	nodeDir := filepath.Join(env.ProjectsDir, "complete-proj")
	_ = os.Chmod(nodeDir, 0555)
	t.Cleanup(func() { _ = os.Chmod(nodeDir, 0755) })

	env.RootCmd.SetArgs([]string{"task", "complete", "--node", "complete-proj/task-0001"})
	err := env.RootCmd.Execute()
	if err == nil {
		t.Error("expected error when SaveNodeState fails for task complete")
	}
}

func TestTaskBlock_SaveNodeStateError_ReadOnly(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("permission test not reliable on Windows")
	}

	env := newTestEnv(t)
	createLeafNode(t, env, "block-proj", "Block Project")

	// Add and claim a task.
	env.RootCmd.SetArgs([]string{"task", "add", "--node", "block-proj", "blockable"})
	_ = env.RootCmd.Execute()
	env.RootCmd.SetArgs([]string{"task", "claim", "--node", "block-proj/task-0001"})
	_ = env.RootCmd.Execute()

	// Lock the node directory.
	nodeDir := filepath.Join(env.ProjectsDir, "block-proj")
	_ = os.Chmod(nodeDir, 0555)
	t.Cleanup(func() { _ = os.Chmod(nodeDir, 0755) })

	env.RootCmd.SetArgs([]string{"task", "block", "--node", "block-proj/task-0001", "stuck"})
	err := env.RootCmd.Execute()
	if err == nil {
		t.Error("expected error when SaveNodeState fails for task block")
	}
}

func TestTaskUnblock_SaveNodeStateError_ReadOnly(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("permission test not reliable on Windows")
	}

	env := newTestEnv(t)
	createLeafNode(t, env, "unblock-proj", "Unblock Project")

	// Add, claim, then block a task.
	env.RootCmd.SetArgs([]string{"task", "add", "--node", "unblock-proj", "blockable"})
	_ = env.RootCmd.Execute()
	env.RootCmd.SetArgs([]string{"task", "claim", "--node", "unblock-proj/task-0001"})
	_ = env.RootCmd.Execute()
	env.RootCmd.SetArgs([]string{"task", "block", "--node", "unblock-proj/task-0001", "blocked reason"})
	_ = env.RootCmd.Execute()

	// Lock the node directory.
	nodeDir := filepath.Join(env.ProjectsDir, "unblock-proj")
	_ = os.Chmod(nodeDir, 0555)
	t.Cleanup(func() { _ = os.Chmod(nodeDir, 0755) })

	env.RootCmd.SetArgs([]string{"task", "unblock", "--node", "unblock-proj/task-0001"})
	err := env.RootCmd.Execute()
	if err == nil {
		t.Error("expected error when SaveNodeState fails for task unblock")
	}
}
