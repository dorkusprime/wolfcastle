//go:build integration

package integration

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/dorkusprime/wolfcastle/internal/output"
	"github.com/dorkusprime/wolfcastle/internal/state"
)

func TestCLI_InitIdempotent(t *testing.T) {
	dir := t.TempDir()
	run(t, dir, "init")

	// Second init without --force should succeed and report already initialized
	out := run(t, dir, "init")
	if !strings.Contains(out, "already initialized") && !strings.Contains(out, "Already") {
		t.Errorf("expected idempotent init message, got: %s", out)
	}

	// Init with --force should also succeed
	out = run(t, dir, "init", "--force")
	if !strings.Contains(out, "Reinitialized") && !strings.Contains(out, "reinitialized") {
		t.Errorf("expected reinit message with --force, got: %s", out)
	}
}

func TestCLI_ProjectHierarchy(t *testing.T) {
	dir := t.TempDir()
	run(t, dir, "init")

	// Create an orchestrator parent
	run(t, dir, "project", "create", "--type", "orchestrator", "parent")

	// Create two leaf children
	run(t, dir, "project", "create", "--node", "parent", "child-a")
	run(t, dir, "project", "create", "--node", "parent", "child-b")

	// Verify root index structure
	idx := loadRootIndex(t, dir)
	parentEntry, ok := idx.Nodes["parent"]
	if !ok {
		t.Fatal("parent not found in root index")
	}
	if parentEntry.Type != state.NodeOrchestrator {
		t.Errorf("parent type = %s, want orchestrator", parentEntry.Type)
	}

	childA, ok := idx.Nodes["parent/child-a"]
	if !ok {
		t.Fatal("parent/child-a not found in root index")
	}
	if childA.Type != state.NodeLeaf {
		t.Errorf("child-a type = %s, want leaf", childA.Type)
	}
	if childA.Parent != "parent" {
		t.Errorf("child-a parent = %q, want %q", childA.Parent, "parent")
	}

	childB, ok := idx.Nodes["parent/child-b"]
	if !ok {
		t.Fatal("parent/child-b not found in root index")
	}
	if childB.Parent != "parent" {
		t.Errorf("child-b parent = %q, want %q", childB.Parent, "parent")
	}
}

func TestCLI_TaskLifecycle(t *testing.T) {
	dir := t.TempDir()
	run(t, dir, "init")
	run(t, dir, "project", "create", "lifecycle-proj")

	// Add
	out := run(t, dir, "task", "add", "--node", "lifecycle-proj", "implement feature")
	if !strings.Contains(out, "task-1") {
		t.Fatalf("task add output unexpected: %s", out)
	}

	ns := loadNode(t, dir, "lifecycle-proj")
	var task *state.Task
	for i := range ns.Tasks {
		if ns.Tasks[i].ID == "task-1" {
			task = &ns.Tasks[i]
			break
		}
	}
	if task == nil {
		t.Fatal("task-1 not found after add")
	}
	if task.State != state.StatusNotStarted {
		t.Errorf("after add: state = %s, want not_started", task.State)
	}

	// Claim
	run(t, dir, "task", "claim", "--node", "lifecycle-proj/task-1")
	ns = loadNode(t, dir, "lifecycle-proj")
	for _, tsk := range ns.Tasks {
		if tsk.ID == "task-1" {
			if tsk.State != state.StatusInProgress {
				t.Errorf("after claim: state = %s, want in_progress", tsk.State)
			}
		}
	}

	// Complete
	run(t, dir, "task", "complete", "--node", "lifecycle-proj/task-1")
	ns = loadNode(t, dir, "lifecycle-proj")
	for _, tsk := range ns.Tasks {
		if tsk.ID == "task-1" {
			if tsk.State != state.StatusComplete {
				t.Errorf("after complete: state = %s, want complete", tsk.State)
			}
		}
	}
}

func TestCLI_TaskBlockUnblock(t *testing.T) {
	dir := t.TempDir()
	run(t, dir, "init")
	run(t, dir, "project", "create", "block-proj")
	run(t, dir, "task", "add", "--node", "block-proj", "blockable task")
	run(t, dir, "task", "claim", "--node", "block-proj/task-1")

	// Block
	run(t, dir, "task", "block", "--node", "block-proj/task-1", "waiting on API")

	ns := loadNode(t, dir, "block-proj")
	for _, task := range ns.Tasks {
		if task.ID == "task-1" {
			if task.State != state.StatusBlocked {
				t.Errorf("after block: state = %s, want blocked", task.State)
			}
			if task.BlockedReason != "waiting on API" {
				t.Errorf("block reason = %q, want %q", task.BlockedReason, "waiting on API")
			}
		}
	}

	// Unblock
	run(t, dir, "task", "unblock", "--node", "block-proj/task-1")

	ns = loadNode(t, dir, "block-proj")
	for _, task := range ns.Tasks {
		if task.ID == "task-1" {
			if task.State != state.StatusNotStarted {
				t.Errorf("after unblock: state = %s, want not_started", task.State)
			}
			if task.BlockedReason != "" {
				t.Errorf("block reason should be cleared, got %q", task.BlockedReason)
			}
		}
	}
}

func TestCLI_InboxLifecycle(t *testing.T) {
	dir := t.TempDir()
	run(t, dir, "init")

	// Add items
	run(t, dir, "inbox", "add", "refactor the auth module")
	run(t, dir, "inbox", "add", "investigate flaky CI")

	// List
	out := run(t, dir, "inbox", "list")
	if !strings.Contains(out, "refactor the auth module") {
		t.Errorf("inbox list missing first item: %s", out)
	}
	if !strings.Contains(out, "investigate flaky CI") {
		t.Errorf("inbox list missing second item: %s", out)
	}

	// Clear all
	out = run(t, dir, "inbox", "clear", "--all")
	if !strings.Contains(out, "Cleared") {
		t.Errorf("inbox clear output unexpected: %s", out)
	}

	// Verify empty
	out = run(t, dir, "inbox", "list")
	if !strings.Contains(out, "empty") && !strings.Contains(out, "No") {
		t.Errorf("inbox should be empty after clear: %s", out)
	}
}

func TestCLI_SpecManagement(t *testing.T) {
	dir := t.TempDir()
	run(t, dir, "init")
	run(t, dir, "project", "create", "spec-proj")

	// Create a spec linked to the project
	out := run(t, dir, "spec", "create", "--node", "spec-proj", "API Auth Flow")
	if !strings.Contains(out, "Created spec") {
		t.Fatalf("spec create output unexpected: %s", out)
	}

	// List specs for the node
	out = run(t, dir, "spec", "list", "--node", "spec-proj")
	if !strings.Contains(out, "api-auth-flow") {
		t.Errorf("spec list missing expected spec: %s", out)
	}

	// Verify node state has the spec linked
	ns := loadNode(t, dir, "spec-proj")
	if len(ns.Specs) == 0 {
		t.Error("expected at least one spec linked to node")
	}
}

func TestCLI_ADRCreation(t *testing.T) {
	dir := t.TempDir()
	run(t, dir, "init")

	out := run(t, dir, "adr", "create", "Use JWT for Authentication")
	if !strings.Contains(out, "Created ADR") {
		t.Fatalf("adr create output unexpected: %s", out)
	}

	// Verify the file exists in the docs/decisions directory
	docsDir := filepath.Join(dir, ".wolfcastle", "docs", "decisions")
	entries, err := os.ReadDir(docsDir)
	if err != nil {
		t.Fatalf("reading decisions dir: %v", err)
	}

	found := false
	for _, e := range entries {
		if strings.Contains(e.Name(), "use-jwt-for-authentication") {
			found = true
			// Read the file and verify format
			data, err := os.ReadFile(filepath.Join(docsDir, e.Name()))
			if err != nil {
				t.Fatalf("reading ADR file: %v", err)
			}
			content := string(data)
			if !strings.Contains(content, "# Use JWT for Authentication") {
				t.Error("ADR missing title heading")
			}
			if !strings.Contains(content, "## Status") {
				t.Error("ADR missing Status section")
			}
			if !strings.Contains(content, "## Context") {
				t.Error("ADR missing Context section")
			}
		}
	}
	if !found {
		t.Error("ADR file not found in docs/decisions/")
	}
}

func TestCLI_DoctorFindsIssues(t *testing.T) {
	dir := t.TempDir()
	run(t, dir, "init")
	run(t, dir, "project", "create", "doctor-test")

	// Corrupt: remove all tasks (including audit task)
	ns := loadNode(t, dir, "doctor-test")
	ns.Tasks = nil
	saveNode(t, dir, "doctor-test", ns)

	// Run doctor without --fix, verify it reports issues
	out := run(t, dir, "doctor")
	if !strings.Contains(out, "issue") && !strings.Contains(out, "ERROR") && !strings.Contains(out, "WARN") {
		t.Errorf("expected doctor to find issues, got: %s", out)
	}
}

func TestCLI_DoctorFixes(t *testing.T) {
	dir := t.TempDir()
	run(t, dir, "init")
	run(t, dir, "project", "create", "doctor-fix-test")

	// Corrupt: remove audit task
	ns := loadNode(t, dir, "doctor-fix-test")
	ns.Tasks = nil
	saveNode(t, dir, "doctor-fix-test", ns)

	// Run doctor --fix
	run(t, dir, "doctor", "--fix")

	// Verify audit task was restored
	ns = loadNode(t, dir, "doctor-fix-test")
	hasAudit := false
	for _, task := range ns.Tasks {
		if task.IsAudit {
			hasAudit = true
		}
	}
	if !hasAudit {
		t.Error("expected audit task to be restored by doctor --fix")
	}
}

func TestCLI_NavigateFindsWork(t *testing.T) {
	dir := t.TempDir()
	run(t, dir, "init")
	run(t, dir, "project", "create", "nav-proj")
	run(t, dir, "task", "add", "--node", "nav-proj", "find me")

	out := run(t, dir, "navigate")
	if !strings.Contains(out, "nav-proj") {
		t.Errorf("navigate did not find the project: %s", out)
	}
	if !strings.Contains(out, "task-1") {
		t.Errorf("navigate did not find task-1: %s", out)
	}
}

func TestCLI_NavigateDepthFirst(t *testing.T) {
	dir := t.TempDir()
	run(t, dir, "init")

	// Create a two-level tree: orchestrator -> two leaves with tasks
	run(t, dir, "project", "create", "--type", "orchestrator", "root-orch")
	run(t, dir, "project", "create", "--node", "root-orch", "first-child")
	run(t, dir, "project", "create", "--node", "root-orch", "second-child")

	run(t, dir, "task", "add", "--node", "root-orch/first-child", "first task")
	run(t, dir, "task", "add", "--node", "root-orch/second-child", "second task")

	// Navigate should find the first child's task (DFS order)
	out := run(t, dir, "navigate")
	if !strings.Contains(out, "first-child") {
		t.Errorf("expected DFS to find first-child first, got: %s", out)
	}
}

func TestCLI_StatusOutput(t *testing.T) {
	dir := t.TempDir()
	run(t, dir, "init")

	// Create nodes in different states
	run(t, dir, "project", "create", "status-complete")
	run(t, dir, "project", "create", "status-blocked")
	run(t, dir, "project", "create", "status-pending")

	// Complete one project's task
	run(t, dir, "task", "add", "--node", "status-complete", "finish me")
	run(t, dir, "task", "claim", "--node", "status-complete/task-1")
	run(t, dir, "task", "complete", "--node", "status-complete/task-1")

	// Block another
	run(t, dir, "task", "add", "--node", "status-blocked", "block me")
	run(t, dir, "task", "claim", "--node", "status-blocked/task-1")
	run(t, dir, "task", "block", "--node", "status-blocked/task-1", "stuck")

	out := run(t, dir, "status")
	if !strings.Contains(out, "Total:") || !strings.Contains(out, "Blocked:") {
		t.Errorf("status output missing expected fields: %s", out)
	}
}

func TestCLI_JSONOutputConsistency(t *testing.T) {
	dir := t.TempDir()
	run(t, dir, "init")

	// Test multiple commands with --json and verify envelope structure
	commands := []struct {
		args           []string
		expectedAction string
	}{
		{[]string{"project", "create", "json-proj"}, "project_create"},
		{[]string{"task", "add", "--node", "json-proj", "json task"}, "task_add"},
		{[]string{"navigate"}, "navigate"},
		{[]string{"status"}, "status"},
	}

	for _, tc := range commands {
		resp := runJSON(t, dir, tc.args...)
		if !resp.OK {
			t.Errorf("command %v returned not-ok: %+v", tc.args, resp)
		}
		if resp.Action != tc.expectedAction {
			t.Errorf("command %v: action = %q, want %q", tc.args, resp.Action, tc.expectedAction)
		}
	}
}

func TestCLI_ArchiveAdd(t *testing.T) {
	dir := t.TempDir()
	run(t, dir, "init")
	run(t, dir, "project", "create", "archive-proj")
	run(t, dir, "task", "add", "--node", "archive-proj", "complete me")
	run(t, dir, "task", "claim", "--node", "archive-proj/task-1")
	run(t, dir, "task", "complete", "--node", "archive-proj/task-1")

	// Complete the audit task too so the node is fully complete
	ns := loadNode(t, dir, "archive-proj")
	for i, task := range ns.Tasks {
		if task.IsAudit && task.State != state.StatusComplete {
			ns.Tasks[i].State = state.StatusComplete
		}
	}
	// Mark the node itself as complete
	ns.State = state.StatusComplete
	saveNode(t, dir, "archive-proj", ns)

	out := run(t, dir, "archive", "add", "--node", "archive-proj")
	if !strings.Contains(out, "Archived") {
		t.Errorf("archive add output unexpected: %s", out)
	}

	// Verify archive file exists
	archiveDir := filepath.Join(dir, ".wolfcastle", "archive")
	entries, err := os.ReadDir(archiveDir)
	if err != nil {
		t.Fatalf("reading archive dir: %v", err)
	}
	if len(entries) == 0 {
		t.Error("expected at least one archive file")
	}
}

func TestCLI_OverlapAdvisory(t *testing.T) {
	dir := t.TempDir()
	run(t, dir, "init")

	// Create two projects with similar names; overlap is advisory-only
	// so this just verifies no crash. The overlap check requires a model,
	// so we just ensure the projects are created without error.
	run(t, dir, "project", "create", "auth-login")
	run(t, dir, "project", "create", "auth-signup")

	// Verify both exist in the root index
	idx := loadRootIndex(t, dir)
	if _, ok := idx.Nodes["auth-login"]; !ok {
		t.Error("auth-login not in root index")
	}
	if _, ok := idx.Nodes["auth-signup"]; !ok {
		t.Error("auth-signup not in root index")
	}
}

// Verify that the JSON envelope from runJSON has the standard structure.
func TestCLI_JSONEnvelopeStructure(t *testing.T) {
	dir := t.TempDir()
	run(t, dir, "init")

	out := run(t, dir, "--json", "project", "create", "envelope-test")

	var raw map[string]any
	if err := json.Unmarshal([]byte(out), &raw); err != nil {
		t.Fatalf("JSON output is not valid JSON: %v\nraw: %s", err, out)
	}

	// Every envelope must have "ok" and "action" fields
	if _, ok := raw["ok"]; !ok {
		t.Error("JSON envelope missing 'ok' field")
	}
	if _, ok := raw["action"]; !ok {
		t.Error("JSON envelope missing 'action' field")
	}
}

// Suppress unused import warnings by referencing output.Response
var _ output.Response
