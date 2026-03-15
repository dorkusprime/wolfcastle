package task

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	"github.com/dorkusprime/wolfcastle/cmd/cmdutil"
	"github.com/dorkusprime/wolfcastle/internal/config"
	"github.com/dorkusprime/wolfcastle/internal/state"
	"github.com/dorkusprime/wolfcastle/internal/tree"
	"github.com/spf13/cobra"
)

type testEnv struct {
	WolfcastleDir string
	ProjectsDir   string
	App           *cmdutil.App
	RootCmd       *cobra.Command
}

func newTestEnv(t *testing.T) *testEnv {
	t.Helper()
	tmp := t.TempDir()
	wcDir := filepath.Join(tmp, ".wolfcastle")
	_ = os.MkdirAll(wcDir, 0755)

	cfg := config.Defaults()
	cfg.Identity = &config.IdentityConfig{User: "test", Machine: "dev"}

	ns := "test-dev"
	projDir := filepath.Join(wcDir, "projects", ns)
	_ = os.MkdirAll(projDir, 0755)

	idx := state.NewRootIndex()
	saveJSON(t, filepath.Join(projDir, "state.json"), idx)

	resolver := &tree.Resolver{WolfcastleDir: wcDir, Namespace: ns}
	testApp := &cmdutil.App{
		WolfcastleDir: wcDir,
		Cfg:           cfg,
		Resolver:      resolver,
	}

	rootCmd := &cobra.Command{Use: "wolfcastle"}
	rootCmd.AddGroup(
		&cobra.Group{ID: "lifecycle", Title: "Lifecycle:"},
		&cobra.Group{ID: "work", Title: "Work Management:"},
		&cobra.Group{ID: "audit", Title: "Auditing:"},
		&cobra.Group{ID: "docs", Title: "Documentation:"},
		&cobra.Group{ID: "diagnostics", Title: "Diagnostics:"},
		&cobra.Group{ID: "integration", Title: "Integration:"},
	)
	Register(testApp, rootCmd)

	return &testEnv{
		WolfcastleDir: wcDir,
		ProjectsDir:   projDir,
		App:           testApp,
		RootCmd:       rootCmd,
	}
}

func saveJSON(t *testing.T, path string, v any) {
	t.Helper()
	data, err := json.MarshalIndent(v, "", "  ")
	if err != nil {
		t.Fatal(err)
	}
	_ = os.MkdirAll(filepath.Dir(path), 0755)
	if err := os.WriteFile(path, data, 0644); err != nil {
		t.Fatal(err)
	}
}

func createLeafNode(t *testing.T, env *testEnv, addr, name string) {
	t.Helper()
	parsed, _ := tree.ParseAddress(addr)
	nodeDir := filepath.Join(env.ProjectsDir, filepath.Join(parsed.Parts...))
	_ = os.MkdirAll(nodeDir, 0755)

	ns := state.NewNodeState(parsed.Leaf(), name, state.NodeLeaf)
	ns.Tasks = []state.Task{
		{ID: "audit", Description: "Audit", State: state.StatusNotStarted, IsAudit: true},
	}
	saveJSON(t, filepath.Join(nodeDir, "state.json"), ns)

	idx, _ := state.LoadRootIndex(filepath.Join(env.ProjectsDir, "state.json"))
	idx.Nodes[addr] = state.IndexEntry{
		Name:     name,
		Type:     state.NodeLeaf,
		State:    state.StatusNotStarted,
		Address:  addr,
		Children: []string{},
	}
	_ = state.SaveRootIndex(filepath.Join(env.ProjectsDir, "state.json"), idx)
}

func loadNodeState(t *testing.T, env *testEnv, addr string) *state.NodeState {
	t.Helper()
	parsed, _ := tree.ParseAddress(addr)
	statePath := filepath.Join(env.ProjectsDir, filepath.Join(parsed.Parts...), "state.json")
	ns, err := state.LoadNodeState(statePath)
	if err != nil {
		t.Fatalf("loading node state for %s: %v", addr, err)
	}
	return ns
}

// ---------------------------------------------------------------------------
// task add
// ---------------------------------------------------------------------------

func TestTaskAdd_Success(t *testing.T) {
	env := newTestEnv(t)
	createLeafNode(t, env, "my-project", "My Project")

	env.RootCmd.SetArgs([]string{"task", "add", "--node", "my-project", "implement the API"})
	if err := env.RootCmd.Execute(); err != nil {
		t.Fatalf("task add failed: %v", err)
	}

	ns := loadNodeState(t, env, "my-project")
	if len(ns.Tasks) != 2 {
		t.Fatalf("expected 2 tasks (1 added + audit), got %d", len(ns.Tasks))
	}
	if ns.Tasks[0].ID != "task-0001" {
		t.Errorf("expected task-0001, got %s", ns.Tasks[0].ID)
	}
	if ns.Tasks[0].Description != "implement the API" {
		t.Errorf("unexpected description: %s", ns.Tasks[0].Description)
	}
	if ns.Tasks[0].State != state.StatusNotStarted {
		t.Errorf("expected not_started, got %s", ns.Tasks[0].State)
	}
	// Audit task should remain last
	if !ns.Tasks[len(ns.Tasks)-1].IsAudit {
		t.Error("audit task should be last")
	}
}

func TestTaskAdd_MultipleAdds(t *testing.T) {
	env := newTestEnv(t)
	createLeafNode(t, env, "my-project", "My Project")

	for i := 0; i < 3; i++ {
		env.RootCmd.SetArgs([]string{"task", "add", "--node", "my-project", "task desc"})
		if err := env.RootCmd.Execute(); err != nil {
			t.Fatalf("task add %d failed: %v", i, err)
		}
	}

	ns := loadNodeState(t, env, "my-project")
	// 3 added + 1 audit
	if len(ns.Tasks) != 4 {
		t.Fatalf("expected 4 tasks, got %d", len(ns.Tasks))
	}
	if ns.Tasks[0].ID != "task-0001" || ns.Tasks[1].ID != "task-0002" || ns.Tasks[2].ID != "task-0003" {
		t.Errorf("unexpected task IDs: %s, %s, %s", ns.Tasks[0].ID, ns.Tasks[1].ID, ns.Tasks[2].ID)
	}
}

func TestTaskAdd_MissingNode(t *testing.T) {
	env := newTestEnv(t)
	env.RootCmd.SetArgs([]string{"task", "add", "description"})
	err := env.RootCmd.Execute()
	if err == nil {
		t.Error("expected error when --node not provided")
	}
}

func TestTaskAdd_EmptyDescription(t *testing.T) {
	env := newTestEnv(t)
	createLeafNode(t, env, "my-project", "My Project")

	env.RootCmd.SetArgs([]string{"task", "add", "--node", "my-project", "   "})
	err := env.RootCmd.Execute()
	if err == nil {
		t.Error("expected error for empty description")
	}
}

func TestTaskAdd_NoResolver(t *testing.T) {
	env := newTestEnv(t)
	env.App.Resolver = nil

	env.RootCmd.SetArgs([]string{"task", "add", "--node", "my-project", "test"})
	err := env.RootCmd.Execute()
	if err == nil {
		t.Error("expected error when resolver is nil")
	}
}

// ---------------------------------------------------------------------------
// task claim
// ---------------------------------------------------------------------------

func TestTaskClaim_Success(t *testing.T) {
	env := newTestEnv(t)
	createLeafNode(t, env, "my-project", "My Project")

	// Add a task first
	env.RootCmd.SetArgs([]string{"task", "add", "--node", "my-project", "do something"})
	_ = env.RootCmd.Execute()

	// Claim it
	env.RootCmd.SetArgs([]string{"task", "claim", "--node", "my-project/task-0001"})
	if err := env.RootCmd.Execute(); err != nil {
		t.Fatalf("task claim failed: %v", err)
	}

	ns := loadNodeState(t, env, "my-project")
	if ns.Tasks[0].State != state.StatusInProgress {
		t.Errorf("expected in_progress, got %s", ns.Tasks[0].State)
	}
	if ns.State != state.StatusInProgress {
		t.Errorf("node state should be in_progress, got %s", ns.State)
	}
}

func TestTaskClaim_InvalidAddress(t *testing.T) {
	env := newTestEnv(t)
	env.RootCmd.SetArgs([]string{"task", "claim", "--node", "single"})
	err := env.RootCmd.Execute()
	if err == nil {
		t.Error("expected error for non-task address")
	}
}

// ---------------------------------------------------------------------------
// task complete
// ---------------------------------------------------------------------------

func TestTaskComplete_Success(t *testing.T) {
	env := newTestEnv(t)
	createLeafNode(t, env, "my-project", "My Project")

	env.RootCmd.SetArgs([]string{"task", "add", "--node", "my-project", "do work"})
	_ = env.RootCmd.Execute()

	env.RootCmd.SetArgs([]string{"task", "claim", "--node", "my-project/task-0001"})
	_ = env.RootCmd.Execute()

	env.RootCmd.SetArgs([]string{"task", "complete", "--node", "my-project/task-0001"})
	if err := env.RootCmd.Execute(); err != nil {
		t.Fatalf("task complete failed: %v", err)
	}

	ns := loadNodeState(t, env, "my-project")
	if ns.Tasks[0].State != state.StatusComplete {
		t.Errorf("expected complete, got %s", ns.Tasks[0].State)
	}
}

// ---------------------------------------------------------------------------
// task block
// ---------------------------------------------------------------------------

func TestTaskBlock_Success(t *testing.T) {
	env := newTestEnv(t)
	createLeafNode(t, env, "my-project", "My Project")

	env.RootCmd.SetArgs([]string{"task", "add", "--node", "my-project", "work item"})
	_ = env.RootCmd.Execute()

	env.RootCmd.SetArgs([]string{"task", "claim", "--node", "my-project/task-0001"})
	_ = env.RootCmd.Execute()

	env.RootCmd.SetArgs([]string{"task", "block", "--node", "my-project/task-0001", "waiting on API"})
	if err := env.RootCmd.Execute(); err != nil {
		t.Fatalf("task block failed: %v", err)
	}

	ns := loadNodeState(t, env, "my-project")
	if ns.Tasks[0].State != state.StatusBlocked {
		t.Errorf("expected blocked, got %s", ns.Tasks[0].State)
	}
	if ns.Tasks[0].BlockedReason != "waiting on API" {
		t.Errorf("unexpected block reason: %s", ns.Tasks[0].BlockedReason)
	}
}

func TestTaskBlock_EmptyReason(t *testing.T) {
	env := newTestEnv(t)
	createLeafNode(t, env, "my-project", "My Project")

	env.RootCmd.SetArgs([]string{"task", "add", "--node", "my-project", "work"})
	_ = env.RootCmd.Execute()
	env.RootCmd.SetArgs([]string{"task", "claim", "--node", "my-project/task-0001"})
	_ = env.RootCmd.Execute()

	env.RootCmd.SetArgs([]string{"task", "block", "--node", "my-project/task-0001", "   "})
	err := env.RootCmd.Execute()
	if err == nil {
		t.Error("expected error for empty block reason")
	}
}

// ---------------------------------------------------------------------------
// task unblock
// ---------------------------------------------------------------------------

func TestTaskUnblock_Success(t *testing.T) {
	env := newTestEnv(t)
	createLeafNode(t, env, "my-project", "My Project")

	env.RootCmd.SetArgs([]string{"task", "add", "--node", "my-project", "work"})
	_ = env.RootCmd.Execute()
	env.RootCmd.SetArgs([]string{"task", "claim", "--node", "my-project/task-0001"})
	_ = env.RootCmd.Execute()
	env.RootCmd.SetArgs([]string{"task", "block", "--node", "my-project/task-0001", "stuck"})
	_ = env.RootCmd.Execute()

	env.RootCmd.SetArgs([]string{"task", "unblock", "--node", "my-project/task-0001"})
	if err := env.RootCmd.Execute(); err != nil {
		t.Fatalf("task unblock failed: %v", err)
	}

	ns := loadNodeState(t, env, "my-project")
	if ns.Tasks[0].State != state.StatusNotStarted {
		t.Errorf("expected not_started after unblock, got %s", ns.Tasks[0].State)
	}
	if ns.Tasks[0].BlockedReason != "" {
		t.Error("block reason should be cleared after unblock")
	}
}

func TestTaskUnblock_MissingNode(t *testing.T) {
	env := newTestEnv(t)
	env.RootCmd.SetArgs([]string{"task", "unblock"})
	err := env.RootCmd.Execute()
	if err == nil {
		t.Error("expected error when --node not provided")
	}
}

// ---------------------------------------------------------------------------
// task complete with validation
// ---------------------------------------------------------------------------

func TestTaskComplete_WithValidation(t *testing.T) {
	env := newTestEnv(t)
	createLeafNode(t, env, "my-project", "My Project")

	// Add a validation command that succeeds
	env.App.Cfg.Validation.Commands = []config.ValidationCommand{
		{Name: "true check", Run: "true", TimeoutSeconds: 5},
	}

	env.RootCmd.SetArgs([]string{"task", "add", "--node", "my-project", "validated task"})
	_ = env.RootCmd.Execute()
	env.RootCmd.SetArgs([]string{"task", "claim", "--node", "my-project/task-0001"})
	_ = env.RootCmd.Execute()

	env.RootCmd.SetArgs([]string{"task", "complete", "--node", "my-project/task-0001"})
	if err := env.RootCmd.Execute(); err != nil {
		t.Fatalf("task complete with validation failed: %v", err)
	}
}

func TestTaskComplete_ValidationFails(t *testing.T) {
	env := newTestEnv(t)
	createLeafNode(t, env, "my-project", "My Project")

	// Add a validation command that fails
	env.App.Cfg.Validation.Commands = []config.ValidationCommand{
		{Name: "fail check", Run: "false", TimeoutSeconds: 5},
	}

	env.RootCmd.SetArgs([]string{"task", "add", "--node", "my-project", "validated task"})
	_ = env.RootCmd.Execute()
	env.RootCmd.SetArgs([]string{"task", "claim", "--node", "my-project/task-0001"})
	_ = env.RootCmd.Execute()

	env.RootCmd.SetArgs([]string{"task", "complete", "--node", "my-project/task-0001"})
	err := env.RootCmd.Execute()
	if err == nil {
		t.Error("expected error when validation fails")
	}
}

func TestTaskComplete_InvalidAddress(t *testing.T) {
	env := newTestEnv(t)
	env.RootCmd.SetArgs([]string{"task", "complete", "--node", "single-part"})
	err := env.RootCmd.Execute()
	if err == nil {
		t.Error("expected error for non-task address")
	}
}

func TestTaskComplete_NonexistentNode(t *testing.T) {
	env := newTestEnv(t)
	env.RootCmd.SetArgs([]string{"task", "complete", "--node", "nonexistent/task-0001"})
	err := env.RootCmd.Execute()
	if err == nil {
		t.Error("expected error for nonexistent node")
	}
}

// ---------------------------------------------------------------------------
// Full lifecycle: add -> claim -> complete
// ---------------------------------------------------------------------------

func TestTaskLifecycle_FullFlow(t *testing.T) {
	env := newTestEnv(t)
	createLeafNode(t, env, "my-project", "My Project")

	// Add
	env.RootCmd.SetArgs([]string{"task", "add", "--node", "my-project", "implement feature"})
	if err := env.RootCmd.Execute(); err != nil {
		t.Fatalf("add: %v", err)
	}

	// Claim
	env.RootCmd.SetArgs([]string{"task", "claim", "--node", "my-project/task-0001"})
	if err := env.RootCmd.Execute(); err != nil {
		t.Fatalf("claim: %v", err)
	}

	// Complete
	env.RootCmd.SetArgs([]string{"task", "complete", "--node", "my-project/task-0001"})
	if err := env.RootCmd.Execute(); err != nil {
		t.Fatalf("complete: %v", err)
	}

	// Now complete the audit task
	env.RootCmd.SetArgs([]string{"task", "claim", "--node", "my-project/audit"})
	if err := env.RootCmd.Execute(); err != nil {
		t.Fatalf("claim audit: %v", err)
	}

	env.RootCmd.SetArgs([]string{"task", "complete", "--node", "my-project/audit"})
	if err := env.RootCmd.Execute(); err != nil {
		t.Fatalf("complete audit: %v", err)
	}

	ns := loadNodeState(t, env, "my-project")
	if ns.State != state.StatusComplete {
		t.Errorf("node should be complete when all tasks done, got %s", ns.State)
	}
}
