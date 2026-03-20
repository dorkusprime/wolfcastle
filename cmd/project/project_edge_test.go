package project

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/dorkusprime/wolfcastle/internal/config"
	"github.com/dorkusprime/wolfcastle/internal/state"
)

// ---------------------------------------------------------------------------
// project create — overlap advisory enabled
// ---------------------------------------------------------------------------

func TestProjectCreate_OverlapEnabled(t *testing.T) {
	env := newTestEnv(t)
	cfg := config.Defaults()
	cfg.OverlapAdvisory.Enabled = true
	cfg.OverlapAdvisory.Threshold = 0.1
	_ = os.MkdirAll(filepath.Join(env.WolfcastleDir, "system", "base"), 0755)
	_ = env.App.Config.WriteBase(cfg)

	// Create another engineer's namespace with similar project
	otherDir := filepath.Join(env.WolfcastleDir, "system", "projects", "alice-dev")
	_ = os.MkdirAll(otherDir, 0755)
	_ = os.WriteFile(filepath.Join(otherDir, "auth-system.md"),
		[]byte("authentication system endpoint login"), 0644)

	env.RootCmd.SetArgs([]string{"project", "create", "--type", "leaf", "auth-system"})
	if err := env.RootCmd.Execute(); err != nil {
		t.Fatalf("project create with overlap failed: %v", err)
	}
}

// ---------------------------------------------------------------------------
// project create — invalid slug
// ---------------------------------------------------------------------------

func TestProjectCreate_InvalidSlug(t *testing.T) {
	env := newTestEnv(t)

	// Names that produce invalid slugs (e.g., all special chars)
	env.RootCmd.SetArgs([]string{"project", "create", "--type", "leaf", "---"})
	err := env.RootCmd.Execute()
	if err == nil {
		t.Error("expected error for invalid slug from name")
	}
}

// ---------------------------------------------------------------------------
// project create — auto promote leaf with audit-only tasks
// ---------------------------------------------------------------------------

func TestProjectCreate_AutoPromoteLeafWithAuditOnly(t *testing.T) {
	env := newTestEnv(t)

	// Create a leaf that only has audit task
	env.RootCmd.SetArgs([]string{"project", "create", "--type", "leaf", "parent-node"})
	if err := env.RootCmd.Execute(); err != nil {
		t.Fatalf("create parent leaf: %v", err)
	}

	// Verify it's a leaf
	parentPath := filepath.Join(env.ProjectsDir, "parent-node", "state.json")
	parentNs, _ := state.LoadNodeState(parentPath)
	if parentNs.Type != state.NodeLeaf {
		t.Fatalf("expected leaf, got %s", parentNs.Type)
	}

	// Create orchestrator child under it (auto-promote)
	env.RootCmd.SetArgs([]string{"project", "create", "--type", "orchestrator", "--node", "parent-node", "child-orch"})
	if err := env.RootCmd.Execute(); err != nil {
		t.Fatalf("create orchestrator child: %v", err)
	}

	parentNs, _ = state.LoadNodeState(parentPath)
	if parentNs.Type != state.NodeOrchestrator {
		t.Errorf("expected parent promoted to orchestrator, got %s", parentNs.Type)
	}
	if len(parentNs.Tasks) != 0 {
		t.Error("promoted parent should have no tasks")
	}
}

// ---------------------------------------------------------------------------
// project create — child under orchestrator (no promotion needed)
// ---------------------------------------------------------------------------

func TestProjectCreate_ChildUnderOrchestrator(t *testing.T) {
	env := newTestEnv(t)

	env.RootCmd.SetArgs([]string{"project", "create", "--type", "orchestrator", "orch-parent"})
	_ = env.RootCmd.Execute()

	env.RootCmd.SetArgs([]string{"project", "create", "--type", "leaf", "--node", "orch-parent", "child-one"})
	if err := env.RootCmd.Execute(); err != nil {
		t.Fatalf("create child under orchestrator: %v", err)
	}

	env.RootCmd.SetArgs([]string{"project", "create", "--type", "leaf", "--node", "orch-parent", "child-two"})
	if err := env.RootCmd.Execute(); err != nil {
		t.Fatalf("create second child: %v", err)
	}

	// Verify parent has both children
	parentPath := filepath.Join(env.ProjectsDir, "orch-parent", "state.json")
	parentNs, _ := state.LoadNodeState(parentPath)
	if len(parentNs.Children) != 2 {
		t.Errorf("expected 2 children, got %d", len(parentNs.Children))
	}
}

// ---------------------------------------------------------------------------
// project create — duplicate child name
// ---------------------------------------------------------------------------

func TestProjectCreate_DuplicateChildName(t *testing.T) {
	env := newTestEnv(t)

	env.RootCmd.SetArgs([]string{"project", "create", "--type", "orchestrator", "parent"})
	_ = env.RootCmd.Execute()

	env.RootCmd.SetArgs([]string{"project", "create", "--type", "leaf", "--node", "parent", "child"})
	_ = env.RootCmd.Execute()

	env.RootCmd.SetArgs([]string{"project", "create", "--type", "leaf", "--node", "parent", "child"})
	err := env.RootCmd.Execute()
	if err == nil {
		t.Error("expected error for duplicate child name")
	}
}

// ---------------------------------------------------------------------------
// project create — root metadata not set on second project
// ---------------------------------------------------------------------------

func TestProjectCreate_SecondRootProjectNoMetadataOverwrite(t *testing.T) {
	env := newTestEnv(t)

	env.RootCmd.SetArgs([]string{"project", "create", "--type", "leaf", "first"})
	_ = env.RootCmd.Execute()

	env.RootCmd.SetArgs([]string{"project", "create", "--type", "leaf", "second"})
	_ = env.RootCmd.Execute()

	idx, _ := state.LoadRootIndex(filepath.Join(env.ProjectsDir, "state.json"))
	if idx.RootID != "first" {
		t.Errorf("root_id should still be 'first', got %q", idx.RootID)
	}
}

// ---------------------------------------------------------------------------
// project create — child with JSON output
// ---------------------------------------------------------------------------

// ---------------------------------------------------------------------------
// project create — with all overlapping namespaces
// ---------------------------------------------------------------------------

func TestProjectCreate_OverlapWithMultipleEngineers(t *testing.T) {
	env := newTestEnv(t)
	cfg := config.Defaults()
	cfg.OverlapAdvisory.Enabled = true
	cfg.OverlapAdvisory.Threshold = 0.1
	_ = os.MkdirAll(filepath.Join(env.WolfcastleDir, "system", "base"), 0755)
	_ = env.App.Config.WriteBase(cfg)

	// Create two other engineers with similar projects
	for _, engineer := range []string{"alice-dev", "bob-dev"} {
		dir := filepath.Join(env.WolfcastleDir, "system", "projects", engineer)
		_ = os.MkdirAll(dir, 0755)
		_ = os.WriteFile(filepath.Join(dir, "auth-system.md"),
			[]byte("authentication authorization system login"), 0644)
	}

	env.RootCmd.SetArgs([]string{"project", "create", "--type", "leaf", "auth-system"})
	if err := env.RootCmd.Execute(); err != nil {
		t.Fatalf("create with overlap failed: %v", err)
	}
}

// ---------------------------------------------------------------------------
// project create — loading root index error (broken state.json)
// ---------------------------------------------------------------------------

func TestProjectCreate_BrokenRootIndex(t *testing.T) {
	env := newTestEnv(t)

	// Break the root index
	_ = os.WriteFile(filepath.Join(env.ProjectsDir, "state.json"), []byte("invalid json"), 0644)

	env.RootCmd.SetArgs([]string{"project", "create", "--type", "leaf", "my-project"})
	err := env.RootCmd.Execute()
	if err == nil {
		t.Error("expected error when root index is broken")
	}
}

// ---------------------------------------------------------------------------
// project create — nested child (3 levels deep)
// ---------------------------------------------------------------------------

func TestProjectCreate_NestedChild(t *testing.T) {
	env := newTestEnv(t)

	// Create orchestrator
	env.RootCmd.SetArgs([]string{"project", "create", "--type", "orchestrator", "top"})
	_ = env.RootCmd.Execute()

	// Create orchestrator child
	env.RootCmd.SetArgs([]string{"project", "create", "--type", "orchestrator", "--node", "top", "mid"})
	_ = env.RootCmd.Execute()

	// Create leaf grandchild
	env.RootCmd.SetArgs([]string{"project", "create", "--type", "leaf", "--node", "top/mid", "bottom"})
	if err := env.RootCmd.Execute(); err != nil {
		t.Fatalf("nested child create failed: %v", err)
	}

	// Verify the chain exists
	idx, _ := state.LoadRootIndex(filepath.Join(env.ProjectsDir, "state.json"))
	if _, ok := idx.Nodes["top/mid/bottom"]; !ok {
		t.Error("nested child should be in root index")
	}
}

func TestProjectCreate_ChildJSONOutput(t *testing.T) {
	env := newTestEnv(t)

	env.RootCmd.SetArgs([]string{"project", "create", "--type", "orchestrator", "parent-json"})
	_ = env.RootCmd.Execute()

	env.App.JSON = true
	defer func() { env.App.JSON = false }()

	env.RootCmd.SetArgs([]string{"project", "create", "--type", "leaf", "--node", "parent-json", "child-json"})
	if err := env.RootCmd.Execute(); err != nil {
		t.Fatalf("child create JSON failed: %v", err)
	}
}
