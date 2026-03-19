package project

import (
	"os"
	"path/filepath"
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
		Config:        af.Config,
		Identity:      af.Identity,
		State:         af.State,
		Prompts:       af.Prompts,
		Classes:       af.Classes,
		Daemon:        af.Daemon,
		Git:           af.Git,
		Clock:         clock.New(),
		WolfcastleDir: af.WolfcastleDir,
		Cfg:           af.Cfg,
		Store:         af.State,
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
	// Orchestrators should have exactly one task: the audit
	if len(ns.Tasks) != 1 {
		t.Errorf("orchestrator should have 1 audit task, got %d", len(ns.Tasks))
	} else if !ns.Tasks[0].IsAudit {
		t.Error("orchestrator's task should be an audit")
	}
}

func TestProjectCreate_ChildProject(t *testing.T) {
	env := newTestEnv(t)

	// Create parent orchestrator
	env.RootCmd.SetArgs([]string{"project", "create", "--type", "orchestrator", "auth-system"})
	_ = env.RootCmd.Execute()

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
	_ = env.RootCmd.Execute()

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

func TestProjectCreate_NoIdentity(t *testing.T) {
	env := newTestEnv(t)
	env.App.Identity = nil
	env.RootCmd.SetArgs([]string{"project", "create", "--type", "leaf", "test"})
	err := env.RootCmd.Execute()
	if err == nil {
		t.Error("expected error when identity is nil")
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
	_ = env.RootCmd.Execute()

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
	_ = env.RootCmd.Execute()

	idx, _ := state.LoadRootIndex(filepath.Join(env.ProjectsDir, "state.json"))
	if idx.RootID != "first-project" {
		t.Errorf("expected root_id 'first-project', got %q", idx.RootID)
	}
}

func TestProjectCreate_JSONOutput(t *testing.T) {
	env := newTestEnv(t)
	env.App.JSON = true
	defer func() { env.App.JSON = false }()

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
	_ = env.RootCmd.Execute()

	// Add a task manually
	statePath := filepath.Join(env.ProjectsDir, "tasked-leaf", "state.json")
	ns, _ := state.LoadNodeState(statePath)
	_, _ = state.TaskAdd(ns, "some work")
	_ = state.SaveNodeState(statePath, ns)

	// Trying to add child should fail because parent has tasks
	env.RootCmd.SetArgs([]string{"project", "create", "--type", "leaf", "--node", "tasked-leaf", "child"})
	err := env.RootCmd.Execute()
	if err == nil {
		t.Error("expected error when parent has non-audit tasks")
	}
}
