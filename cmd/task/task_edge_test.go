package task

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/dorkusprime/wolfcastle/internal/config"
	"github.com/dorkusprime/wolfcastle/internal/state"
	"github.com/dorkusprime/wolfcastle/internal/tree"
)

// ---------------------------------------------------------------------------
// task block — error paths
// ---------------------------------------------------------------------------

func TestTaskBlock_NoResolver(t *testing.T) {
	env := newTestEnv(t)
	env.App.Resolver = nil

	env.RootCmd.SetArgs([]string{"task", "block", "--node", "my-project/task-0001", "stuck"})
	err := env.RootCmd.Execute()
	if err == nil {
		t.Error("expected error when resolver is nil")
	}
}

func TestTaskBlock_InvalidAddress(t *testing.T) {
	env := newTestEnv(t)

	env.RootCmd.SetArgs([]string{"task", "block", "--node", "single", "stuck"})
	err := env.RootCmd.Execute()
	if err == nil {
		t.Error("expected error for non-task address")
	}
}

func TestTaskBlock_NonexistentNode(t *testing.T) {
	env := newTestEnv(t)

	env.RootCmd.SetArgs([]string{"task", "block", "--node", "nonexistent/task-0001", "stuck"})
	err := env.RootCmd.Execute()
	if err == nil {
		t.Error("expected error for nonexistent node")
	}
}

// ---------------------------------------------------------------------------
// task claim — error paths
// ---------------------------------------------------------------------------

func TestTaskClaim_NoResolver(t *testing.T) {
	env := newTestEnv(t)
	env.App.Resolver = nil

	env.RootCmd.SetArgs([]string{"task", "claim", "--node", "my-project/task-0001"})
	err := env.RootCmd.Execute()
	if err == nil {
		t.Error("expected error when resolver is nil")
	}
}

func TestTaskClaim_NonexistentNode(t *testing.T) {
	env := newTestEnv(t)

	env.RootCmd.SetArgs([]string{"task", "claim", "--node", "nonexistent/task-0001"})
	err := env.RootCmd.Execute()
	if err == nil {
		t.Error("expected error for nonexistent node")
	}
}

// ---------------------------------------------------------------------------
// task complete — error paths
// ---------------------------------------------------------------------------

func TestTaskComplete_NoResolver(t *testing.T) {
	env := newTestEnv(t)
	env.App.Resolver = nil

	env.RootCmd.SetArgs([]string{"task", "complete", "--node", "my-project/task-0001"})
	err := env.RootCmd.Execute()
	if err == nil {
		t.Error("expected error when resolver is nil")
	}
}

// ---------------------------------------------------------------------------
// task unblock — error paths
// ---------------------------------------------------------------------------

func TestTaskUnblock_NoResolver(t *testing.T) {
	env := newTestEnv(t)
	env.App.Resolver = nil

	env.RootCmd.SetArgs([]string{"task", "unblock", "--node", "my-project/task-0001"})
	err := env.RootCmd.Execute()
	if err == nil {
		t.Error("expected error when resolver is nil")
	}
}

func TestTaskUnblock_InvalidAddress(t *testing.T) {
	env := newTestEnv(t)

	env.RootCmd.SetArgs([]string{"task", "unblock", "--node", "single"})
	err := env.RootCmd.Execute()
	if err == nil {
		t.Error("expected error for non-task address")
	}
}

func TestTaskUnblock_NonexistentNode(t *testing.T) {
	env := newTestEnv(t)

	env.RootCmd.SetArgs([]string{"task", "unblock", "--node", "nonexistent/task-0001"})
	err := env.RootCmd.Execute()
	if err == nil {
		t.Error("expected error for nonexistent node")
	}
}

// ---------------------------------------------------------------------------
// task block — ParseAddress error after SplitTaskAddress succeeds
// ---------------------------------------------------------------------------

func TestTaskBlock_InvalidNodeAfterSplit(t *testing.T) {
	env := newTestEnv(t)
	// "INVALID" fails ValidateSlug (uppercase), but "INVALID/task-0001" passes SplitTaskAddress
	env.RootCmd.SetArgs([]string{"task", "block", "--node", "INVALID/task-0001", "stuck"})
	err := env.RootCmd.Execute()
	if err == nil {
		t.Error("expected error for invalid node address after split")
	}
}

// ---------------------------------------------------------------------------
// task claim — ParseAddress error after SplitTaskAddress succeeds
// ---------------------------------------------------------------------------

func TestTaskClaim_InvalidNodeAfterSplit(t *testing.T) {
	env := newTestEnv(t)
	env.RootCmd.SetArgs([]string{"task", "claim", "--node", "INVALID/task-0001"})
	err := env.RootCmd.Execute()
	if err == nil {
		t.Error("expected error for invalid node address after split")
	}
}

// ---------------------------------------------------------------------------
// task complete — ParseAddress error after SplitTaskAddress succeeds
// ---------------------------------------------------------------------------

func TestTaskComplete_InvalidNodeAfterSplit(t *testing.T) {
	env := newTestEnv(t)
	env.RootCmd.SetArgs([]string{"task", "complete", "--node", "INVALID/task-0001"})
	err := env.RootCmd.Execute()
	if err == nil {
		t.Error("expected error for invalid node address after split")
	}
}

// ---------------------------------------------------------------------------
// task unblock — ParseAddress error after SplitTaskAddress succeeds
// ---------------------------------------------------------------------------

func TestTaskUnblock_InvalidNodeAfterSplit(t *testing.T) {
	env := newTestEnv(t)
	env.RootCmd.SetArgs([]string{"task", "unblock", "--node", "INVALID/task-0001"})
	err := env.RootCmd.Execute()
	if err == nil {
		t.Error("expected error for invalid node address after split")
	}
}

// ---------------------------------------------------------------------------
// task complete — validation edge cases
// ---------------------------------------------------------------------------

func TestTaskComplete_ValidationDefaultTimeout(t *testing.T) {
	env := newTestEnv(t)
	createLeafNode(t, env, "my-project", "My Project")

	// Validation with TimeoutSeconds == 0 (should use default 30s)
	env.App.Cfg.Validation.Commands = []config.ValidationCommand{
		{Name: "default timeout", Run: "true"},
	}

	env.RootCmd.SetArgs([]string{"task", "add", "--node", "my-project", "work"})
	_ = env.RootCmd.Execute()
	env.RootCmd.SetArgs([]string{"task", "claim", "--node", "my-project/task-0001"})
	_ = env.RootCmd.Execute()

	env.RootCmd.SetArgs([]string{"task", "complete", "--node", "my-project/task-0001"})
	if err := env.RootCmd.Execute(); err != nil {
		t.Fatalf("complete with default timeout validation failed: %v", err)
	}
}

func TestTaskComplete_NilConfig(t *testing.T) {
	env := newTestEnv(t)
	createLeafNode(t, env, "my-project", "My Project")

	// Set Cfg to nil to test the nil-config branch
	env.App.Cfg = nil

	env.RootCmd.SetArgs([]string{"task", "add", "--node", "my-project", "work"})
	_ = env.RootCmd.Execute()

	// Restore config for claim
	env.App.Cfg = config.Defaults()
	env.App.Cfg.Identity = &config.IdentityConfig{User: "test", Machine: "dev"}

	env.RootCmd.SetArgs([]string{"task", "claim", "--node", "my-project/task-0001"})
	_ = env.RootCmd.Execute()

	// Set Cfg to nil again for complete
	env.App.Cfg = nil

	env.RootCmd.SetArgs([]string{"task", "complete", "--node", "my-project/task-0001"})
	if err := env.RootCmd.Execute(); err != nil {
		t.Fatalf("complete with nil config failed: %v", err)
	}
}

// ---------------------------------------------------------------------------
// task complete — JSON output when node becomes complete
// ---------------------------------------------------------------------------

func TestTaskComplete_JSONOutput_NodeComplete(t *testing.T) {
	env := newTestEnv(t)
	createLeafNode(t, env, "my-project", "My Project")

	// Add, claim, complete a task
	env.RootCmd.SetArgs([]string{"task", "add", "--node", "my-project", "work"})
	_ = env.RootCmd.Execute()
	env.RootCmd.SetArgs([]string{"task", "claim", "--node", "my-project/task-0001"})
	_ = env.RootCmd.Execute()
	env.RootCmd.SetArgs([]string{"task", "complete", "--node", "my-project/task-0001"})
	_ = env.RootCmd.Execute()

	// Complete the audit task to make node complete
	env.RootCmd.SetArgs([]string{"task", "claim", "--node", "my-project/audit"})
	_ = env.RootCmd.Execute()

	env.App.JSONOutput = true
	defer func() { env.App.JSONOutput = false }()

	env.RootCmd.SetArgs([]string{"task", "complete", "--node", "my-project/audit"})
	if err := env.RootCmd.Execute(); err != nil {
		t.Fatalf("complete audit (json) failed: %v", err)
	}
}

// ---------------------------------------------------------------------------
// task complete — human output when node becomes complete
// ---------------------------------------------------------------------------

// ---------------------------------------------------------------------------
// task claim — PropagateState error (broken root index)
// ---------------------------------------------------------------------------

func TestTaskClaim_PropagateError(t *testing.T) {
	env := newTestEnv(t)
	createLeafNode(t, env, "my-project", "My Project")

	env.RootCmd.SetArgs([]string{"task", "add", "--node", "my-project", "work"})
	_ = env.RootCmd.Execute()

	// Break the root index to cause PropagateState to fail
	rootStatePath := filepath.Join(env.ProjectsDir, "state.json")
	_ = os.WriteFile(rootStatePath, []byte("invalid json"), 0644)

	env.RootCmd.SetArgs([]string{"task", "claim", "--node", "my-project/task-0001"})
	err := env.RootCmd.Execute()
	if err == nil {
		t.Error("expected error when PropagateState fails")
	}
}

// ---------------------------------------------------------------------------
// task block — PropagateState error (broken root index)
// ---------------------------------------------------------------------------

func TestTaskBlock_PropagateError(t *testing.T) {
	env := newTestEnv(t)
	createLeafNode(t, env, "my-project", "My Project")

	env.RootCmd.SetArgs([]string{"task", "add", "--node", "my-project", "work"})
	_ = env.RootCmd.Execute()
	env.RootCmd.SetArgs([]string{"task", "claim", "--node", "my-project/task-0001"})
	_ = env.RootCmd.Execute()

	// Break the root index
	rootStatePath := filepath.Join(env.ProjectsDir, "state.json")
	_ = os.WriteFile(rootStatePath, []byte("invalid json"), 0644)

	env.RootCmd.SetArgs([]string{"task", "block", "--node", "my-project/task-0001", "stuck"})
	err := env.RootCmd.Execute()
	if err == nil {
		t.Error("expected error when PropagateState fails")
	}
}

// ---------------------------------------------------------------------------
// task complete — PropagateState error (broken root index)
// ---------------------------------------------------------------------------

func TestTaskComplete_PropagateError(t *testing.T) {
	env := newTestEnv(t)
	createLeafNode(t, env, "my-project", "My Project")

	env.RootCmd.SetArgs([]string{"task", "add", "--node", "my-project", "work"})
	_ = env.RootCmd.Execute()
	env.RootCmd.SetArgs([]string{"task", "claim", "--node", "my-project/task-0001"})
	_ = env.RootCmd.Execute()

	// Break the root index
	rootStatePath := filepath.Join(env.ProjectsDir, "state.json")
	_ = os.WriteFile(rootStatePath, []byte("invalid json"), 0644)

	env.RootCmd.SetArgs([]string{"task", "complete", "--node", "my-project/task-0001"})
	err := env.RootCmd.Execute()
	if err == nil {
		t.Error("expected error when PropagateState fails")
	}
}

// ---------------------------------------------------------------------------
// task unblock — PropagateState error (broken root index)
// ---------------------------------------------------------------------------

func TestTaskUnblock_PropagateError(t *testing.T) {
	env := newTestEnv(t)
	createLeafNode(t, env, "my-project", "My Project")

	env.RootCmd.SetArgs([]string{"task", "add", "--node", "my-project", "work"})
	_ = env.RootCmd.Execute()
	env.RootCmd.SetArgs([]string{"task", "claim", "--node", "my-project/task-0001"})
	_ = env.RootCmd.Execute()
	env.RootCmd.SetArgs([]string{"task", "block", "--node", "my-project/task-0001", "stuck"})
	_ = env.RootCmd.Execute()

	// Break the root index
	rootStatePath := filepath.Join(env.ProjectsDir, "state.json")
	_ = os.WriteFile(rootStatePath, []byte("invalid json"), 0644)

	env.RootCmd.SetArgs([]string{"task", "unblock", "--node", "my-project/task-0001"})
	err := env.RootCmd.Execute()
	if err == nil {
		t.Error("expected error when PropagateState fails")
	}
}

// ---------------------------------------------------------------------------
// task add — TaskAdd error (non-leaf node)
// ---------------------------------------------------------------------------

func TestTaskAdd_NonLeafNode(t *testing.T) {
	env := newTestEnv(t)

	// Create an orchestrator node
	parsed, _ := tree.ParseAddress("orch-node")
	nodeDir := filepath.Join(env.ProjectsDir, filepath.Join(parsed.Parts...))
	_ = os.MkdirAll(nodeDir, 0755)

	ns := state.NewNodeState(parsed.Leaf(), "Orchestrator", state.NodeOrchestrator)
	saveJSON(t, filepath.Join(nodeDir, "state.json"), ns)

	idx, _ := state.LoadRootIndex(filepath.Join(env.ProjectsDir, "state.json"))
	idx.Nodes["orch-node"] = state.IndexEntry{
		Name:     "Orchestrator",
		Type:     state.NodeOrchestrator,
		State:    state.StatusNotStarted,
		Address:  "orch-node",
		Children: []string{},
	}
	_ = state.SaveRootIndex(filepath.Join(env.ProjectsDir, "state.json"), idx)

	env.RootCmd.SetArgs([]string{"task", "add", "--node", "orch-node", "should fail"})
	err := env.RootCmd.Execute()
	if err == nil {
		t.Error("expected error adding task to orchestrator node")
	}
}

func TestTaskComplete_NodeBecomeComplete(t *testing.T) {
	env := newTestEnv(t)
	createLeafNode(t, env, "my-project", "My Project")

	// Add, claim, complete a task
	env.RootCmd.SetArgs([]string{"task", "add", "--node", "my-project", "work"})
	_ = env.RootCmd.Execute()
	env.RootCmd.SetArgs([]string{"task", "claim", "--node", "my-project/task-0001"})
	_ = env.RootCmd.Execute()
	env.RootCmd.SetArgs([]string{"task", "complete", "--node", "my-project/task-0001"})
	_ = env.RootCmd.Execute()

	// Complete audit task — node should become complete
	env.RootCmd.SetArgs([]string{"task", "claim", "--node", "my-project/audit"})
	_ = env.RootCmd.Execute()

	env.RootCmd.SetArgs([]string{"task", "complete", "--node", "my-project/audit"})
	if err := env.RootCmd.Execute(); err != nil {
		t.Fatalf("complete audit failed: %v", err)
	}
}
