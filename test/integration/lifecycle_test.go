//go:build integration

package integration

import (
	"strings"
	"testing"

	"github.com/dorkusprime/wolfcastle/internal/state"
)

func TestProjectLifecycle(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()

	// Initialize
	out := run(t, dir, "init")
	if !strings.Contains(out, "Initialized") {
		t.Fatalf("init output unexpected: %s", out)
	}

	// Create a leaf project
	out = run(t, dir, "project", "create", "my-feature")
	if !strings.Contains(out, "my-feature") {
		t.Fatalf("project create output unexpected: %s", out)
	}

	// Add a task
	out = run(t, dir, "task", "add", "--node", "my-feature", "implement API")
	if !strings.Contains(out, "task-0001") {
		t.Fatalf("task add output unexpected: %s", out)
	}

	// Claim the task
	run(t, dir, "task", "claim", "--node", "my-feature/task-0001")

	// Complete the task
	run(t, dir, "task", "complete", "--node", "my-feature/task-0001")

	// Verify node state via disk
	ns := loadNode(t, dir, "my-feature")
	for _, task := range ns.Tasks {
		if task.ID == "task-0001" && task.State != state.StatusComplete {
			t.Errorf("task-1 state = %s, want complete", task.State)
		}
	}

	// Verify root index reflects the project
	idx := loadRootIndex(t, dir)
	entry, ok := idx.Nodes["my-feature"]
	if !ok {
		t.Fatal("my-feature not found in root index")
	}
	if entry.Type != state.NodeLeaf {
		t.Errorf("my-feature type = %s, want leaf", entry.Type)
	}
}

func TestProjectLifecycleJSON(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()

	run(t, dir, "init")

	resp := runJSON(t, dir, "project", "create", "json-test")
	if !resp.OK {
		t.Fatalf("project create JSON not ok: %+v", resp)
	}
	if resp.Action != "project_create" {
		t.Errorf("action = %s, want project_create", resp.Action)
	}

	resp = runJSON(t, dir, "task", "add", "--node", "json-test", "do work")
	if !resp.OK {
		t.Fatalf("task add JSON not ok: %+v", resp)
	}
	if resp.Action != "task_add" {
		t.Errorf("action = %s, want task_add", resp.Action)
	}
}

func TestInboxLifecycle(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	run(t, dir, "init")

	// Add items to inbox
	run(t, dir, "inbox", "add", "refactor auth module")
	run(t, dir, "inbox", "add", "investigate flaky test")

	// List and verify
	out := run(t, dir, "inbox", "list")
	if !strings.Contains(out, "refactor auth module") {
		t.Errorf("inbox list missing first item: %s", out)
	}
	if !strings.Contains(out, "investigate flaky test") {
		t.Errorf("inbox list missing second item: %s", out)
	}

	// Clear with --all
	out = run(t, dir, "inbox", "clear", "--all")
	if !strings.Contains(out, "Cleared 2 items") {
		t.Errorf("inbox clear output unexpected: %s", out)
	}

	// Verify empty
	out = run(t, dir, "inbox", "list")
	if !strings.Contains(out, "empty") {
		t.Errorf("inbox should be empty after clear: %s", out)
	}
}

func TestAuditBreadcrumbAndEscalation(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	run(t, dir, "init")

	// Create an orchestrator with a child so we can escalate
	run(t, dir, "project", "create", "--type", "orchestrator", "parent-project")
	run(t, dir, "project", "create", "--node", "parent-project", "child-node")

	// Add a breadcrumb to the child
	run(t, dir, "audit", "breadcrumb", "--node", "parent-project/child-node", "started implementation")

	// Verify breadcrumb was recorded
	ns := loadNode(t, dir, "parent-project/child-node")
	if len(ns.Audit.Breadcrumbs) == 0 {
		t.Fatal("expected at least one breadcrumb")
	}
	if ns.Audit.Breadcrumbs[0].Text != "started implementation" {
		t.Errorf("breadcrumb text = %q, want %q", ns.Audit.Breadcrumbs[0].Text, "started implementation")
	}

	// Escalate a gap from child to parent
	run(t, dir, "audit", "escalate", "--node", "parent-project/child-node", "missing error handling spec")

	// Verify escalation on the parent
	parentNs := loadNode(t, dir, "parent-project")
	if len(parentNs.Audit.Escalations) == 0 {
		t.Fatal("expected at least one escalation on parent")
	}
	if parentNs.Audit.Escalations[0].Description != "missing error handling spec" {
		t.Errorf("escalation description = %q, want %q",
			parentNs.Audit.Escalations[0].Description, "missing error handling spec")
	}
}

func TestSpecLifecycle(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	run(t, dir, "init")

	// Create a project to link specs to
	run(t, dir, "project", "create", "spec-project")

	// Create a spec linked to the project
	out := run(t, dir, "spec", "create", "--node", "spec-project", "API Authentication Flow")
	if !strings.Contains(out, "Created spec") {
		t.Fatalf("spec create output unexpected: %s", out)
	}

	// List specs for the node
	out = run(t, dir, "spec", "list", "--node", "spec-project")
	if !strings.Contains(out, "api-authentication-flow") {
		t.Errorf("spec list output missing spec: %s", out)
	}

	// Verify node state has the spec linked
	ns := loadNode(t, dir, "spec-project")
	if len(ns.Specs) == 0 {
		t.Fatal("expected spec to be linked to node")
	}

	// Create a second spec and link it manually
	run(t, dir, "spec", "create", "Standalone Spec")
	// Link it to the project
	// We need to find the filename — it includes a timestamp, so list all specs
	out = run(t, dir, "spec", "list")
	// The standalone spec should appear in the full list
	if !strings.Contains(out, "standalone-spec") {
		t.Errorf("full spec list missing standalone spec: %s", out)
	}
}

func TestDoctorFixesMissingAuditTask(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	run(t, dir, "init")
	run(t, dir, "project", "create", "doctor-test")

	// Verify the project starts with an audit task
	ns := loadNode(t, dir, "doctor-test")
	hasAudit := false
	for _, task := range ns.Tasks {
		if task.IsAudit {
			hasAudit = true
			break
		}
	}
	if !hasAudit {
		t.Fatal("expected audit task on newly created project")
	}

	// Corrupt: remove all tasks (including audit)
	ns.Tasks = nil
	saveNode(t, dir, "doctor-test", ns)

	// Verify corruption
	ns = loadNode(t, dir, "doctor-test")
	if len(ns.Tasks) != 0 {
		t.Fatal("expected tasks to be empty after corruption")
	}

	// Run doctor --fix
	out := run(t, dir, "doctor", "--fix")
	_ = out // doctor output is informational

	// Verify audit task was restored
	ns = loadNode(t, dir, "doctor-test")
	hasAudit = false
	for _, task := range ns.Tasks {
		if task.IsAudit {
			hasAudit = true
			break
		}
	}
	if !hasAudit {
		t.Error("expected audit task to be restored by doctor --fix")
	}
}

func TestInitAlreadyInitialized(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()

	// First init
	run(t, dir, "init")

	// Second init should succeed (not error) and indicate already initialized
	out := run(t, dir, "init")
	if !strings.Contains(out, "already initialized") && !strings.Contains(out, "Already") {
		t.Errorf("second init should indicate already initialized: %s", out)
	}
}

func TestTaskClaimBeforeCreate(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	run(t, dir, "init")

	// Trying to claim a task on a non-existent node should fail
	runExpectError(t, dir, "task", "claim", "--node", "nonexistent/task-0001")
}

func TestProjectCreateDuplicate(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	run(t, dir, "init")
	run(t, dir, "project", "create", "dup-project")

	// Creating the same project again should fail
	runExpectError(t, dir, "project", "create", "dup-project")
}
