package orchestrator

import "testing"

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
