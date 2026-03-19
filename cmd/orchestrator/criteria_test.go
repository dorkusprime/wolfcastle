package orchestrator

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
	projDir := filepath.Join(wcDir, "system", "projects", ns)
	_ = os.MkdirAll(projDir, 0755)

	idx := state.NewRootIndex()
	saveJSON(t, filepath.Join(projDir, "state.json"), idx)

	resolver := &tree.Resolver{WolfcastleDir: wcDir, Namespace: ns}
	store := state.NewStateStore(resolver.ProjectsDir(), state.DefaultLockTimeout)
	testApp := &cmdutil.App{
		Identity:      &config.Identity{User: "test", Machine: "dev", Namespace: ns},
		State:         store,
		WolfcastleDir: wcDir,
		Cfg:           cfg,
		Resolver:      resolver,
		Store:         store,
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

func createOrchestratorNode(t *testing.T, env *testEnv, addr, name string) {
	t.Helper()
	parsed, _ := tree.ParseAddress(addr)
	nodeDir := filepath.Join(env.ProjectsDir, filepath.Join(parsed.Parts...))
	_ = os.MkdirAll(nodeDir, 0755)

	ns := state.NewNodeState(parsed.Leaf(), name, state.NodeOrchestrator)
	saveJSON(t, filepath.Join(nodeDir, "state.json"), ns)

	idx, _ := state.LoadRootIndex(filepath.Join(env.ProjectsDir, "state.json"))
	idx.Nodes[addr] = state.IndexEntry{
		Name:     name,
		Type:     state.NodeOrchestrator,
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
// criteria add
// ---------------------------------------------------------------------------

func TestCriteria_AddSuccess(t *testing.T) {
	env := newTestEnv(t)
	createOrchestratorNode(t, env, "my-project", "My Project")

	env.RootCmd.SetArgs([]string{"orchestrator", "criteria", "--node", "my-project", "all tests pass"})
	if err := env.RootCmd.Execute(); err != nil {
		t.Fatalf("criteria add failed: %v", err)
	}

	ns := loadNodeState(t, env, "my-project")
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
	createOrchestratorNode(t, env, "my-project", "My Project")

	for i := 0; i < 3; i++ {
		env.RootCmd.SetArgs([]string{"orchestrator", "criteria", "--node", "my-project", "all tests pass"})
		if err := env.RootCmd.Execute(); err != nil {
			t.Fatalf("criteria add %d failed: %v", i, err)
		}
	}

	ns := loadNodeState(t, env, "my-project")
	if len(ns.SuccessCriteria) != 1 {
		t.Errorf("expected 1 criterion after duplicates, got %d", len(ns.SuccessCriteria))
	}
}

// ---------------------------------------------------------------------------
// criteria --list
// ---------------------------------------------------------------------------

func TestCriteria_List(t *testing.T) {
	env := newTestEnv(t)
	createOrchestratorNode(t, env, "my-project", "My Project")

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
	ns := loadNodeState(t, env, "my-project")
	if len(ns.SuccessCriteria) != 2 {
		t.Errorf("expected 2 criteria, got %d", len(ns.SuccessCriteria))
	}
}

func TestCriteria_ListEmpty(t *testing.T) {
	env := newTestEnv(t)
	createOrchestratorNode(t, env, "my-project", "My Project")

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
	createOrchestratorNode(t, env, "my-project", "My Project")

	env.RootCmd.SetArgs([]string{"orchestrator", "criteria", "--node", "my-project", "   "})
	err := env.RootCmd.Execute()
	if err == nil {
		t.Error("expected error for whitespace-only criterion")
	}
}

func TestCriteria_NoCriterionArg(t *testing.T) {
	env := newTestEnv(t)
	createOrchestratorNode(t, env, "my-project", "My Project")

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
