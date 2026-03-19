package orchestrator

import (
	"strings"
	"testing"
)

// ---------------------------------------------------------------------------
// JSON output: criteria add
// ---------------------------------------------------------------------------

func TestCriteria_AddJSON(t *testing.T) {
	env := newTestEnv(t)
	env.createOrchestratorNode(t, "my-project", "My Project")
	env.App.JSONOutput = true

	env.RootCmd.SetArgs([]string{"orchestrator", "criteria", "--node", "my-project", "all tests pass"})
	if err := env.RootCmd.Execute(); err != nil {
		t.Fatalf("criteria add (JSON) failed: %v", err)
	}

	ns := env.loadNodeState(t, "my-project")
	if len(ns.SuccessCriteria) != 1 {
		t.Fatalf("expected 1 criterion, got %d", len(ns.SuccessCriteria))
	}
	if ns.SuccessCriteria[0] != "all tests pass" {
		t.Errorf("expected 'all tests pass', got %q", ns.SuccessCriteria[0])
	}
}

// ---------------------------------------------------------------------------
// JSON output: criteria list with entries
// ---------------------------------------------------------------------------

func TestCriteria_ListJSON(t *testing.T) {
	env := newTestEnv(t)
	env.createOrchestratorNode(t, "my-project", "My Project")

	// Add criteria in human mode first.
	env.RootCmd.SetArgs([]string{"orchestrator", "criteria", "--node", "my-project", "all tests pass"})
	_ = env.RootCmd.Execute()
	env.RootCmd.SetArgs([]string{"orchestrator", "criteria", "--node", "my-project", "docs updated"})
	_ = env.RootCmd.Execute()

	// Switch to JSON and list.
	env.App.JSONOutput = true
	env.RootCmd.SetArgs([]string{"orchestrator", "criteria", "--node", "my-project", "--list"})
	if err := env.RootCmd.Execute(); err != nil {
		t.Fatalf("criteria list (JSON) failed: %v", err)
	}

	ns := env.loadNodeState(t, "my-project")
	if len(ns.SuccessCriteria) != 2 {
		t.Errorf("expected 2 criteria, got %d", len(ns.SuccessCriteria))
	}
}

// ---------------------------------------------------------------------------
// JSON output: criteria list when empty
// ---------------------------------------------------------------------------

func TestCriteria_ListEmptyJSON(t *testing.T) {
	env := newTestEnv(t)
	env.createOrchestratorNode(t, "my-project", "My Project")
	env.App.JSONOutput = true

	env.RootCmd.SetArgs([]string{"orchestrator", "criteria", "--node", "my-project", "--list"})
	if err := env.RootCmd.Execute(); err != nil {
		t.Fatalf("criteria list empty (JSON) failed: %v", err)
	}
}

// ---------------------------------------------------------------------------
// Internal guard: nodeAddr == "" (bypasses cobra's MarkFlagRequired)
// ---------------------------------------------------------------------------

func TestCriteria_EmptyNodeAddr(t *testing.T) {
	env := newTestEnv(t)

	// Build the criteria subcommand directly so we can call RunE without
	// cobra's required-flag validation intercepting the empty value.
	criteriaCmd := newCriteriaCmd(env.App)
	criteriaCmd.SetArgs([]string{"some criterion"})

	err := criteriaCmd.RunE(criteriaCmd, []string{"some criterion"})
	if err == nil {
		t.Fatal("expected error for empty --node value")
	}
	if !strings.Contains(err.Error(), "--node is required") {
		t.Errorf("unexpected error message: %v", err)
	}
}

// ---------------------------------------------------------------------------
// List mode with invalid address triggers ReadNode error
// ---------------------------------------------------------------------------

func TestCriteria_ListReadNodeError(t *testing.T) {
	env := newTestEnv(t)

	// An address containing ".." is rejected by the state store's path
	// validation, producing a ReadNode error inside the --list branch.
	env.RootCmd.SetArgs([]string{"orchestrator", "criteria", "--node", "bad/../addr", "--list"})
	err := env.RootCmd.Execute()
	if err == nil {
		t.Fatal("expected error when ReadNode fails on invalid address in list mode")
	}
}
