package orchestrator

import (
	"testing"

	"github.com/dorkusprime/wolfcastle/cmd/cmdutil"
	"github.com/dorkusprime/wolfcastle/internal/clock"
	"github.com/dorkusprime/wolfcastle/internal/state"
	"github.com/dorkusprime/wolfcastle/internal/testutil"
	"github.com/spf13/cobra"
)

type testEnv struct {
	WolfcastleDir string
	ProjectsDir   string
	App           *cmdutil.App
	RootCmd       *cobra.Command
	env           *testutil.Environment
}

func newTestEnv(t *testing.T) *testEnv {
	t.Helper()

	env := testutil.NewEnvironment(t)
	af := env.ToAppFields()

	testApp := &cmdutil.App{
		Config:   af.Config,
		Identity: af.Identity,
		State:    af.State,
		Prompts:  af.Prompts,
		Classes:  af.Classes,
		Daemon:   af.Daemon,
		Git:      af.Git,
		Clock:    clock.New(),
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
		WolfcastleDir: env.Root,
		ProjectsDir:   env.ProjectsDir(),
		App:           testApp,
		RootCmd:       rootCmd,
		env:           env,
	}
}

func (e *testEnv) createOrchestratorNode(t *testing.T, addr, name string) {
	t.Helper()
	e.env.WithProject(name, testutil.Orchestrator(addr))
}

func (e *testEnv) loadNodeState(t *testing.T, addr string) *state.NodeState {
	t.Helper()
	ns, err := e.env.State.ReadNode(addr)
	if err != nil {
		t.Fatalf("loading node state for %s: %v", addr, err)
	}
	return ns
}

// ---------------------------------------------------------------------------
// criteria add
// ---------------------------------------------------------------------------

func TestCriteria_AddSuccess(t *testing.T) {
	env := newTestEnv(t)
	env.createOrchestratorNode(t, "my-project", "My Project")

	env.RootCmd.SetArgs([]string{"orchestrator", "criteria", "--node", "my-project", "all tests pass"})
	if err := env.RootCmd.Execute(); err != nil {
		t.Fatalf("criteria add failed: %v", err)
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
// criteria add duplicate (idempotent)
// ---------------------------------------------------------------------------

func TestCriteria_AddDuplicate(t *testing.T) {
	env := newTestEnv(t)
	env.createOrchestratorNode(t, "my-project", "My Project")

	for i := 0; i < 3; i++ {
		env.RootCmd.SetArgs([]string{"orchestrator", "criteria", "--node", "my-project", "all tests pass"})
		if err := env.RootCmd.Execute(); err != nil {
			t.Fatalf("criteria add %d failed: %v", i, err)
		}
	}

	ns := env.loadNodeState(t, "my-project")
	if len(ns.SuccessCriteria) != 1 {
		t.Errorf("expected 1 criterion after duplicates, got %d", len(ns.SuccessCriteria))
	}
}

// ---------------------------------------------------------------------------
// criteria --list
// ---------------------------------------------------------------------------

func TestCriteria_List(t *testing.T) {
	env := newTestEnv(t)
	env.createOrchestratorNode(t, "my-project", "My Project")

	// Add two criteria first
	env.RootCmd.SetArgs([]string{"orchestrator", "criteria", "--node", "my-project", "all tests pass"})
	_ = env.RootCmd.Execute()
	env.RootCmd.SetArgs([]string{"orchestrator", "criteria", "--node", "my-project", "docs updated"})
	_ = env.RootCmd.Execute()

	// List them (should not error)
	env.RootCmd.SetArgs([]string{"orchestrator", "criteria", "--node", "my-project", "--list"})
	if err := env.RootCmd.Execute(); err != nil {
		t.Fatalf("criteria list failed: %v", err)
	}

	// Verify the underlying state still has both
	ns := env.loadNodeState(t, "my-project")
	if len(ns.SuccessCriteria) != 2 {
		t.Errorf("expected 2 criteria, got %d", len(ns.SuccessCriteria))
	}
}

func TestCriteria_ListEmpty(t *testing.T) {
	env := newTestEnv(t)
	env.createOrchestratorNode(t, "my-project", "My Project")

	env.RootCmd.SetArgs([]string{"orchestrator", "criteria", "--node", "my-project", "--list"})
	if err := env.RootCmd.Execute(); err != nil {
		t.Fatalf("criteria list on empty node failed: %v", err)
	}
}

// ---------------------------------------------------------------------------
// missing --node flag
// ---------------------------------------------------------------------------

func TestCriteria_MissingNode(t *testing.T) {
	env := newTestEnv(t)

	env.RootCmd.SetArgs([]string{"orchestrator", "criteria", "some criterion"})
	err := env.RootCmd.Execute()
	if err == nil {
		t.Error("expected error when --node not provided")
	}
}

// ---------------------------------------------------------------------------
// empty criterion text
// ---------------------------------------------------------------------------

func TestCriteria_EmptyText(t *testing.T) {
	env := newTestEnv(t)
	env.createOrchestratorNode(t, "my-project", "My Project")

	env.RootCmd.SetArgs([]string{"orchestrator", "criteria", "--node", "my-project", "   "})
	err := env.RootCmd.Execute()
	if err == nil {
		t.Error("expected error for whitespace-only criterion")
	}
}

func TestCriteria_NoCriterionArg(t *testing.T) {
	env := newTestEnv(t)
	env.createOrchestratorNode(t, "my-project", "My Project")

	env.RootCmd.SetArgs([]string{"orchestrator", "criteria", "--node", "my-project"})
	err := env.RootCmd.Execute()
	if err == nil {
		t.Error("expected error when no criterion argument provided")
	}
}

// ---------------------------------------------------------------------------
// node not found
// ---------------------------------------------------------------------------

func TestCriteria_NodeNotFound(t *testing.T) {
	env := newTestEnv(t)

	env.RootCmd.SetArgs([]string{"orchestrator", "criteria", "--node", "nonexistent", "some criterion"})
	err := env.RootCmd.Execute()
	if err == nil {
		t.Error("expected error for nonexistent node")
	}
}

func TestCriteria_ListOnMissingNodeReturnsEmpty(t *testing.T) {
	env := newTestEnv(t)

	// ReadNode returns a default empty state for nonexistent nodes,
	// so --list on a missing address succeeds with zero criteria.
	env.RootCmd.SetArgs([]string{"orchestrator", "criteria", "--node", "nonexistent", "--list"})
	if err := env.RootCmd.Execute(); err != nil {
		t.Fatalf("expected no error for list on missing node, got: %v", err)
	}
}

// ---------------------------------------------------------------------------
// no resolver
// ---------------------------------------------------------------------------

func TestCriteria_NoIdentity(t *testing.T) {
	env := newTestEnv(t)
	env.App.Identity = nil

	env.RootCmd.SetArgs([]string{"orchestrator", "criteria", "--node", "my-project", "test"})
	err := env.RootCmd.Execute()
	if err == nil {
		t.Error("expected error when identity is nil")
	}
}
