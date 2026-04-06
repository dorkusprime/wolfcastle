package project

import (
	"testing"

	"github.com/dorkusprime/wolfcastle/internal/state"
)

func TestCreateProject_RootLevelLeaf(t *testing.T) {
	t.Parallel()
	idx := state.NewRootIndex()

	ns, addr, err := CreateProject(idx, "", "my-leaf", "My Leaf", state.NodeLeaf)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if addr != "my-leaf" {
		t.Errorf("expected addr 'my-leaf', got %q", addr)
	}
	if ns.ID != "my-leaf" {
		t.Errorf("expected ID 'my-leaf', got %q", ns.ID)
	}
	if ns.Name != "My Leaf" {
		t.Errorf("expected Name 'My Leaf', got %q", ns.Name)
	}
	if ns.Type != state.NodeLeaf {
		t.Errorf("expected type leaf, got %q", ns.Type)
	}
	if ns.State != state.StatusNotStarted {
		t.Errorf("expected state not_started, got %q", ns.State)
	}

	// Verify root index was updated
	entry, ok := idx.Nodes["my-leaf"]
	if !ok {
		t.Fatal("expected node to be in root index")
	}
	if entry.Name != "My Leaf" {
		t.Errorf("expected entry name 'My Leaf', got %q", entry.Name)
	}
	if entry.Parent != "" {
		t.Errorf("expected empty parent, got %q", entry.Parent)
	}

	// Root-level nodes appear in idx.Root
	if len(idx.Root) != 1 || idx.Root[0] != "my-leaf" {
		t.Errorf("expected Root=['my-leaf'], got %v", idx.Root)
	}
}

func TestCreateProject_RootLevelOrchestrator(t *testing.T) {
	t.Parallel()
	idx := state.NewRootIndex()

	ns, addr, err := CreateProject(idx, "", "my-orch", "My Orchestrator", state.NodeOrchestrator)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if addr != "my-orch" {
		t.Errorf("expected addr 'my-orch', got %q", addr)
	}
	if ns.Type != state.NodeOrchestrator {
		t.Errorf("expected type orchestrator, got %q", ns.Type)
	}

	// Orchestrators should get an audit task (verifies aggregate quality)
	auditFound := false
	for _, task := range ns.Tasks {
		if task.IsAudit {
			auditFound = true
		}
	}
	if !auditFound {
		t.Error("orchestrator should have an audit task")
	}
}

func TestCreateProject_LeafGetsAuditTask(t *testing.T) {
	t.Parallel()
	idx := state.NewRootIndex()

	ns, _, err := CreateProject(idx, "", "audited", "Audited Leaf", state.NodeLeaf)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(ns.Tasks) != 1 {
		t.Fatalf("expected 1 task, got %d", len(ns.Tasks))
	}
	audit := ns.Tasks[0]
	if audit.ID != "audit" {
		t.Errorf("expected task ID 'audit', got %q", audit.ID)
	}
	if !audit.IsAudit {
		t.Error("expected IsAudit=true")
	}
	if audit.State != state.StatusNotStarted {
		t.Errorf("expected audit state not_started, got %q", audit.State)
	}
}

func TestCreateProject_ChildUnderParent(t *testing.T) {
	t.Parallel()
	idx := state.NewRootIndex()

	// Create parent first
	_, _, err := CreateProject(idx, "", "parent", "Parent", state.NodeOrchestrator)
	if err != nil {
		t.Fatalf("unexpected error creating parent: %v", err)
	}

	// Create child under parent
	ns, addr, err := CreateProject(idx, "parent", "child", "Child", state.NodeLeaf)
	if err != nil {
		t.Fatalf("unexpected error creating child: %v", err)
	}
	if addr != "parent/child" {
		t.Errorf("expected addr 'parent/child', got %q", addr)
	}
	if ns.ID != "child" {
		t.Errorf("expected ID 'child', got %q", ns.ID)
	}

	// Verify parent-child relationship in index
	childEntry := idx.Nodes["parent/child"]
	if childEntry.Parent != "parent" {
		t.Errorf("expected parent 'parent', got %q", childEntry.Parent)
	}

	parentEntry := idx.Nodes["parent"]
	if len(parentEntry.Children) != 1 || parentEntry.Children[0] != "parent/child" {
		t.Errorf("expected parent children ['parent/child'], got %v", parentEntry.Children)
	}
}

func TestCreateProject_DuplicateDetection(t *testing.T) {
	t.Parallel()
	idx := state.NewRootIndex()

	_, _, err := CreateProject(idx, "", "dupe", "Dupe", state.NodeLeaf)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	_, _, err = CreateProject(idx, "", "dupe", "Dupe Again", state.NodeLeaf)
	if err == nil {
		t.Error("expected error for duplicate node")
	}
}

func TestCreateProject_MultipleRootNodes(t *testing.T) {
	t.Parallel()
	idx := state.NewRootIndex()

	_, _, err := CreateProject(idx, "", "first", "First", state.NodeLeaf)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	_, _, err = CreateProject(idx, "", "second", "Second", state.NodeLeaf)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if len(idx.Root) != 2 {
		t.Errorf("expected 2 root nodes, got %d", len(idx.Root))
	}
	if len(idx.Nodes) != 2 {
		t.Errorf("expected 2 nodes in index, got %d", len(idx.Nodes))
	}
}

func TestCreateProject_ChildrenListInitializedEmpty(t *testing.T) {
	t.Parallel()
	idx := state.NewRootIndex()

	_, _, err := CreateProject(idx, "", "node", "Node", state.NodeLeaf)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	entry := idx.Nodes["node"]
	if entry.Children == nil {
		t.Error("expected Children to be non-nil empty slice")
	}
	if len(entry.Children) != 0 {
		t.Errorf("expected 0 children, got %d", len(entry.Children))
	}
}

func TestCreateProject_DeeplyNestedChild(t *testing.T) {
	t.Parallel()
	idx := state.NewRootIndex()

	// Create a chain: grandparent -> parent -> child
	_, _, err := CreateProject(idx, "", "gp", "Grandparent", state.NodeOrchestrator)
	if err != nil {
		t.Fatal(err)
	}
	_, _, err = CreateProject(idx, "gp", "par", "Parent", state.NodeOrchestrator)
	if err != nil {
		t.Fatal(err)
	}
	ns, addr, err := CreateProject(idx, "gp/par", "child", "Child", state.NodeLeaf)
	if err != nil {
		t.Fatal(err)
	}

	if addr != "gp/par/child" {
		t.Errorf("expected addr 'gp/par/child', got %q", addr)
	}
	if ns.Type != state.NodeLeaf {
		t.Errorf("expected type leaf, got %q", ns.Type)
	}

	// Verify the parent got the child link
	parEntry := idx.Nodes["gp/par"]
	if len(parEntry.Children) != 1 || parEntry.Children[0] != "gp/par/child" {
		t.Errorf("expected parent children ['gp/par/child'], got %v", parEntry.Children)
	}
}
