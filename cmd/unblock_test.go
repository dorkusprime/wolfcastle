package cmd

import (
	"strings"
	"testing"

	"github.com/dorkusprime/wolfcastle/cmd/cmdutil"
	"github.com/dorkusprime/wolfcastle/internal/clock"
	"github.com/dorkusprime/wolfcastle/internal/state"
	"github.com/dorkusprime/wolfcastle/internal/tree"
)

func TestBuildDiagnostic_BasicOutput(t *testing.T) {
	ns := state.NewNodeState("my-project", "My Project", state.NodeLeaf)
	task := &state.Task{
		ID:            "task-0001",
		Description:   "implement API",
		State:         state.StatusBlocked,
		BlockedReason: "missing spec",
		FailureCount:  3,
	}

	diag := buildDiagnostic("my-project", "task-0001", ns, task)

	if !strings.Contains(diag, "my-project") {
		t.Error("diagnostic should contain node address")
	}
	if !strings.Contains(diag, "task-0001") {
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
	state.AddBreadcrumb(ns, "task-0001", "tried approach A", clock.New())
	state.AddBreadcrumb(ns, "task-0001", "approach A failed", clock.New())

	task := &state.Task{
		ID:            "task-0001",
		Description:   "work",
		State:         state.StatusBlocked,
		BlockedReason: "stuck",
	}

	diag := buildDiagnostic("my-project", "task-0001", ns, task)
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
		ID:            "task-0001",
		Description:   "work",
		State:         state.StatusBlocked,
		BlockedReason: "stuck",
	}

	diag := buildDiagnostic("my-project", "task-0001", ns, task)
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
		ID:            "task-0001",
		State:         state.StatusBlocked,
		BlockedReason: "stuck",
	}

	diag := buildDiagnostic("my-project", "task-0001", ns, task)
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
	_, _ = state.TaskAdd(ns, "do work")
	_ = state.SaveNodeState(app.Resolver.NodeStatePath(parsed), ns)

	rootCmd.SetArgs([]string{"unblock", "--node", "my-project/task-0001"})
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
	_, _ = state.TaskAdd(ns, "do work")
	_ = state.TaskClaim(ns, "task-0001")
	_ = state.TaskBlock(ns, "task-0001", "stuck on something")
	_ = state.SaveNodeState(app.Resolver.NodeStatePath(parsed), ns)

	rootCmd.SetArgs([]string{"unblock", "--agent", "--node", "my-project/task-0001"})
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

	rootCmd.SetArgs([]string{"unblock", "--node", "my-project/task-0001"})
	err := rootCmd.Execute()
	if err == nil {
		t.Error("expected error without resolver")
	}
}

func TestLoadUnblockPreamble_NoWolfcastleDir(t *testing.T) {
	oldApp := app
	defer func() { app = oldApp }()

	app = &cmdutil.App{}
	preamble := loadUnblockPreamble()
	if preamble == "" {
		t.Error("expected non-empty fallback preamble")
	}
	if !strings.Contains(preamble, "helping a developer resolve a blocked task") {
		t.Error("expected default preamble text")
	}
}

func TestLoadUnblockPreamble_WithWolfcastleDir(t *testing.T) {
	oldApp := app
	defer func() { app = oldApp }()

	env := newTestEnv(t)
	app = env.App
	preamble := loadUnblockPreamble()
	if preamble == "" {
		t.Error("expected non-empty preamble")
	}
}

func TestBuildDiagnostic_EmptyNode(t *testing.T) {
	ns := state.NewNodeState("empty", "Empty", state.NodeLeaf)
	task := &state.Task{
		ID:    "t-1",
		State: state.StatusBlocked,
	}
	diag := buildDiagnostic("empty", "t-1", ns, task)
	if !strings.Contains(diag, "Unblock Diagnostic") {
		t.Error("diagnostic should contain header")
	}
}

func TestBuildDiagnostic_WithTaskBreadcrumbs(t *testing.T) {
	ns := state.NewNodeState("proj", "Proj", state.NodeLeaf)
	task := &state.Task{
		ID:          "t-1",
		State:       state.StatusBlocked,
		Breadcrumbs: []string{"tried X", "tried Y"},
	}
	diag := buildDiagnostic("proj", "t-1", ns, task)
	if !strings.Contains(diag, "tried X") {
		t.Error("expected task breadcrumbs in diagnostic")
	}
}
