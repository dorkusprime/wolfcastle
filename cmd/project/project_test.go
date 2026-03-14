package project

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
	os.MkdirAll(wcDir, 0755)

	cfg := config.Defaults()
	cfg.Identity = &config.IdentityConfig{User: "test", Machine: "dev"}
	cfg.OverlapAdvisory.Enabled = false // disable for tests

	ns := "test-dev"
	projDir := filepath.Join(wcDir, "projects", ns)
	os.MkdirAll(projDir, 0755)

	idx := state.NewRootIndex()
	data, _ := json.MarshalIndent(idx, "", "  ")
	os.WriteFile(filepath.Join(projDir, "state.json"), data, 0644)

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

// ---------------------------------------------------------------------------
// project create
// ---------------------------------------------------------------------------

func TestProjectCreate_LeafProject(t *testing.T) {
	env := newTestEnv(t)

	env.RootCmd.SetArgs([]string{"project", "create", "--type", "leaf", "auth-system"})
	if err := env.RootCmd.Execute(); err != nil {
		t.Fatalf("project create failed: %v", err)
	}

	// Verify state.json was created
	statePath := filepath.Join(env.ProjectsDir, "auth-system", "state.json")
	ns, err := state.LoadNodeState(statePath)
	if err != nil {
		t.Fatalf("loading node state: %v", err)
	}
	if ns.Type != state.NodeLeaf {
		t.Errorf("expected leaf type, got %s", ns.Type)
	}
	if ns.Name != "auth-system" {
		t.Errorf("unexpected name: %s", ns.Name)
	}
	if ns.State != state.StatusNotStarted {
		t.Errorf("expected not_started, got %s", ns.State)
	}

	// Should have audit task
	hasAudit := false
	for _, task := range ns.Tasks {
		if task.IsAudit {
			hasAudit = true
		}
	}
	if !hasAudit {
		t.Error("leaf node should have an audit task")
	}

	// Verify root index was updated
	idx, _ := state.LoadRootIndex(filepath.Join(env.ProjectsDir, "state.json"))
	if _, ok := idx.Nodes["auth-system"]; !ok {
		t.Error("project not in root index")
	}
}

func TestProjectCreate_OrchestratorProject(t *testing.T) {
	env := newTestEnv(t)

	env.RootCmd.SetArgs([]string{"project", "create", "--type", "orchestrator", "auth-system"})
	if err := env.RootCmd.Execute(); err != nil {
		t.Fatalf("project create failed: %v", err)
	}

	statePath := filepath.Join(env.ProjectsDir, "auth-system", "state.json")
	ns, err := state.LoadNodeState(statePath)
	if err != nil {
		t.Fatalf("loading node state: %v", err)
	}
	if ns.Type != state.NodeOrchestrator {
		t.Errorf("expected orchestrator type, got %s", ns.Type)
	}
	// Orchestrators should not have tasks
	if len(ns.Tasks) > 0 {
		t.Error("orchestrator should not have tasks")
	}
}

func TestProjectCreate_ChildProject(t *testing.T) {
	env := newTestEnv(t)

	// Create parent orchestrator
	env.RootCmd.SetArgs([]string{"project", "create", "--type", "orchestrator", "auth-system"})
	env.RootCmd.Execute()

	// Create child leaf
	env.RootCmd.SetArgs([]string{"project", "create", "--type", "leaf", "--node", "auth-system", "login-flow"})
	if err := env.RootCmd.Execute(); err != nil {
		t.Fatalf("child project create failed: %v", err)
	}

	// Verify child exists
	childPath := filepath.Join(env.ProjectsDir, "auth-system", "login-flow", "state.json")
	ns, err := state.LoadNodeState(childPath)
	if err != nil {
		t.Fatalf("loading child state: %v", err)
	}
	if ns.Type != state.NodeLeaf {
		t.Errorf("expected leaf, got %s", ns.Type)
	}

	// Verify parent has child ref
	parentPath := filepath.Join(env.ProjectsDir, "auth-system", "state.json")
	parentNs, _ := state.LoadNodeState(parentPath)
	found := false
	for _, child := range parentNs.Children {
		if child.ID == "login-flow" {
			found = true
			break
		}
	}
	if !found {
		t.Error("parent should have child ref to login-flow")
	}
}

func TestProjectCreate_DuplicateName(t *testing.T) {
	env := newTestEnv(t)

	env.RootCmd.SetArgs([]string{"project", "create", "--type", "leaf", "auth-system"})
	env.RootCmd.Execute()

	env.RootCmd.SetArgs([]string{"project", "create", "--type", "leaf", "auth-system"})
	err := env.RootCmd.Execute()
	if err == nil {
		t.Error("expected error for duplicate project name")
	}
}

func TestProjectCreate_InvalidType(t *testing.T) {
	env := newTestEnv(t)
	env.RootCmd.SetArgs([]string{"project", "create", "--type", "invalid", "my-project"})
	err := env.RootCmd.Execute()
	if err == nil {
		t.Error("expected error for invalid type")
	}
}

func TestProjectCreate_NoResolver(t *testing.T) {
	env := newTestEnv(t)
	env.App.Resolver = nil
	env.RootCmd.SetArgs([]string{"project", "create", "--type", "leaf", "test"})
	err := env.RootCmd.Execute()
	if err == nil {
		t.Error("expected error when resolver is nil")
	}
}

func TestProjectCreate_ParentNotFound(t *testing.T) {
	env := newTestEnv(t)
	env.RootCmd.SetArgs([]string{"project", "create", "--type", "leaf", "--node", "nonexistent", "child"})
	err := env.RootCmd.Execute()
	if err == nil {
		t.Error("expected error when parent doesn't exist")
	}
}

func TestProjectCreate_DescriptionFile(t *testing.T) {
	env := newTestEnv(t)
	env.RootCmd.SetArgs([]string{"project", "create", "--type", "leaf", "my-project"})
	env.RootCmd.Execute()

	descPath := filepath.Join(env.ProjectsDir, "my-project", "my-project.md")
	data, err := os.ReadFile(descPath)
	if err != nil {
		t.Fatalf("reading description: %v", err)
	}
	if len(data) == 0 {
		t.Error("description file should not be empty")
	}
}

func TestProjectCreate_SetsRootMetadata(t *testing.T) {
	env := newTestEnv(t)
	env.RootCmd.SetArgs([]string{"project", "create", "--type", "leaf", "first-project"})
	env.RootCmd.Execute()

	idx, _ := state.LoadRootIndex(filepath.Join(env.ProjectsDir, "state.json"))
	if idx.RootID != "first-project" {
		t.Errorf("expected root_id 'first-project', got %q", idx.RootID)
	}
}

func TestProjectCreate_JSONOutput(t *testing.T) {
	env := newTestEnv(t)
	env.App.JSONOutput = true
	defer func() { env.App.JSONOutput = false }()

	env.RootCmd.SetArgs([]string{"project", "create", "--type", "leaf", "json-proj"})
	if err := env.RootCmd.Execute(); err != nil {
		t.Fatalf("project create JSON failed: %v", err)
	}
}

func TestProjectCreate_AutoPromoteLeafToOrchestrator(t *testing.T) {
	env := newTestEnv(t)

	// Create a leaf project (no tasks besides audit)
	env.RootCmd.SetArgs([]string{"project", "create", "--type", "leaf", "parent-leaf"})
	if err := env.RootCmd.Execute(); err != nil {
		t.Fatalf("failed to create leaf: %v", err)
	}

	// Create a child under the leaf (should auto-promote)
	env.RootCmd.SetArgs([]string{"project", "create", "--type", "leaf", "--node", "parent-leaf", "child-leaf"})
	if err := env.RootCmd.Execute(); err != nil {
		t.Fatalf("failed to create child (auto-promote): %v", err)
	}

	// Verify parent was promoted to orchestrator
	parentPath := filepath.Join(env.ProjectsDir, "parent-leaf", "state.json")
	parentNs, err := state.LoadNodeState(parentPath)
	if err != nil {
		t.Fatalf("loading parent: %v", err)
	}
	if parentNs.Type != state.NodeOrchestrator {
		t.Errorf("expected parent to be promoted to orchestrator, got %s", parentNs.Type)
	}
}

func TestProjectCreate_AutoPromoteBlockedByTasks(t *testing.T) {
	env := newTestEnv(t)

	// Create leaf project and add a non-audit task
	env.RootCmd.SetArgs([]string{"project", "create", "--type", "leaf", "tasked-leaf"})
	env.RootCmd.Execute()

	// Add a task manually
	statePath := filepath.Join(env.ProjectsDir, "tasked-leaf", "state.json")
	ns, _ := state.LoadNodeState(statePath)
	state.TaskAdd(ns, "some work")
	state.SaveNodeState(statePath, ns)

	// Trying to add child should fail because parent has tasks
	env.RootCmd.SetArgs([]string{"project", "create", "--type", "leaf", "--node", "tasked-leaf", "child"})
	err := env.RootCmd.Execute()
	if err == nil {
		t.Error("expected error when parent has non-audit tasks")
	}
}
