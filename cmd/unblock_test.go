package cmd

import (
	"strings"
	"testing"

	"github.com/dorkusprime/wolfcastle/internal/state"
	"github.com/dorkusprime/wolfcastle/internal/tree"
)

func TestBuildDiagnostic_BasicOutput(t *testing.T) {
	ns := state.NewNodeState("my-project", "My Project", state.NodeLeaf)
	task := &state.Task{
		ID:            "task-1",
		Description:   "implement API",
		State:         state.StatusBlocked,
		BlockedReason: "missing spec",
		FailureCount:  3,
	}

	diag := buildDiagnostic("my-project", "task-1", ns, task)

	if !strings.Contains(diag, "my-project") {
		t.Error("diagnostic should contain node address")
	}
	if !strings.Contains(diag, "task-1") {
		t.Error("diagnostic should contain task ID")
	}
	if !strings.Contains(diag, "missing spec") {
		t.Error("diagnostic should contain block reason")
	}
	if !strings.Contains(diag, "3") {
		t.Error("diagnostic should contain failure count")
	}
}

func TestBuildDiagnostic_WithBreadcrumbs(t *testing.T) {
	ns := state.NewNodeState("my-project", "My Project", state.NodeLeaf)
	state.AddBreadcrumb(ns, "task-1", "tried approach A")
	state.AddBreadcrumb(ns, "task-1", "approach A failed")

	task := &state.Task{
		ID:            "task-1",
		Description:   "work",
		State:         state.StatusBlocked,
		BlockedReason: "stuck",
	}

	diag := buildDiagnostic("my-project", "task-1", ns, task)
	if !strings.Contains(diag, "tried approach A") {
		t.Error("diagnostic should contain breadcrumbs")
	}
}

func TestBuildDiagnostic_WithScope(t *testing.T) {
	ns := state.NewNodeState("my-project", "My Project", state.NodeLeaf)
	ns.Audit.Scope = &state.AuditScope{
		Description: "verify auth module",
		Files:       []string{"auth.go", "login.go"},
		Systems:     []string{"auth"},
	}

	task := &state.Task{
		ID:            "task-1",
		Description:   "work",
		State:         state.StatusBlocked,
		BlockedReason: "stuck",
	}

	diag := buildDiagnostic("my-project", "task-1", ns, task)
	if !strings.Contains(diag, "verify auth module") {
		t.Error("diagnostic should contain scope description")
	}
	if !strings.Contains(diag, "auth.go") {
		t.Error("diagnostic should contain scope files")
	}
}

func TestBuildDiagnostic_WithSpecs(t *testing.T) {
	ns := state.NewNodeState("my-project", "My Project", state.NodeLeaf)
	ns.Specs = []string{"spec-1.md", "spec-2.md"}

	task := &state.Task{
		ID:            "task-1",
		State:         state.StatusBlocked,
		BlockedReason: "stuck",
	}

	diag := buildDiagnostic("my-project", "task-1", ns, task)
	if !strings.Contains(diag, "spec-1.md") {
		t.Error("diagnostic should contain linked specs")
	}
}

func TestUnblockCmd_TaskNotBlocked(t *testing.T) {
	oldApp := app
	defer func() { app = oldApp }()

	env := newTestEnv(t)
	app = env.App
	env.createLeafNode(t, "my-project", "My Project")

	// Add a task (not_started, not blocked)
	parsed, _ := tree.ParseAddress("my-project")
	ns := env.loadNodeState(t, "my-project")
	state.TaskAdd(ns, "do work")
	state.SaveNodeState(app.Resolver.NodeStatePath(parsed), ns)

	rootCmd.SetArgs([]string{"unblock", "--node", "my-project/task-1"})
	err := rootCmd.Execute()
	if err == nil {
		t.Error("expected error when task is not blocked")
	}
}

func TestUnblockCmd_TaskNotFound(t *testing.T) {
	oldApp := app
	defer func() { app = oldApp }()

	env := newTestEnv(t)
	app = env.App
	env.createLeafNode(t, "my-project", "My Project")

	rootCmd.SetArgs([]string{"unblock", "--node", "my-project/task-99"})
	err := rootCmd.Execute()
	if err == nil {
		t.Error("expected error when task not found")
	}
}

func TestUnblockCmd_AgentMode(t *testing.T) {
	oldApp := app
	defer func() { app = oldApp }()

	env := newTestEnv(t)
	app = env.App
	env.createLeafNode(t, "my-project", "My Project")

	// Create a blocked task
	parsed, _ := tree.ParseAddress("my-project")
	ns := env.loadNodeState(t, "my-project")
	state.TaskAdd(ns, "do work")
	state.TaskClaim(ns, "task-1")
	state.TaskBlock(ns, "task-1", "stuck on something")
	state.SaveNodeState(app.Resolver.NodeStatePath(parsed), ns)

	rootCmd.SetArgs([]string{"unblock", "--agent", "--node", "my-project/task-1"})
	if err := rootCmd.Execute(); err != nil {
		t.Fatalf("unblock --agent failed: %v", err)
	}
}

func TestUnblockCmd_NoResolver(t *testing.T) {
	oldApp := app
	defer func() { app = oldApp }()

	env := newTestEnv(t)
	app = env.App
	app.Resolver = nil

	rootCmd.SetArgs([]string{"unblock", "--node", "my-project/task-1"})
	err := rootCmd.Execute()
	if err == nil {
		t.Error("expected error without resolver")
	}
}
