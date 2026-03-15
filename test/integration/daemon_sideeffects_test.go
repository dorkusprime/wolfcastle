//go:build integration

package integration

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/dorkusprime/wolfcastle/internal/state"
)

// Test 17: Model creates a file during execution.
func TestDaemon_ModelCreatesFiles(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	run(t, dir, "init")

	scriptPath, _ := createRealisticMock(t, dir, "creates-files", MockModelConfig{
		Behaviors: []MockBehavior{
			{
				Marker:      "WOLFCASTLE_COMPLETE",
				CreateFiles: map[string]string{"hello.txt": "hello world"},
			},
		},
	})
	configureMockModels(t, dir, scriptPath)

	run(t, dir, "project", "create", "create-files-test")
	run(t, dir, "task", "add", "--node", "create-files-test", "create a file")
	setMaxIterations(t, dir, 10)
	run(t, dir, "start")

	data, err := os.ReadFile(filepath.Join(dir, "hello.txt"))
	if err != nil {
		t.Fatalf("hello.txt should exist after daemon run: %v", err)
	}
	if string(data) != "hello world" {
		t.Errorf("expected hello.txt content %q, got %q", "hello world", string(data))
	}
}

// Test 18: Model modifies an existing file.
func TestDaemon_ModelModifiesExistingFiles(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	run(t, dir, "init")

	// Create the file before the daemon runs.
	if err := os.WriteFile(filepath.Join(dir, "existing.txt"), []byte("original"), 0644); err != nil {
		t.Fatalf("writing seed file: %v", err)
	}

	scriptPath, _ := createRealisticMock(t, dir, "modifies-files", MockModelConfig{
		Behaviors: []MockBehavior{
			{
				Marker:      "WOLFCASTLE_COMPLETE",
				CreateFiles: map[string]string{"existing.txt": "modified"},
			},
		},
	})
	configureMockModels(t, dir, scriptPath)

	run(t, dir, "project", "create", "modify-files-test")
	run(t, dir, "task", "add", "--node", "modify-files-test", "modify the file")
	setMaxIterations(t, dir, 10)
	run(t, dir, "start")

	data, err := os.ReadFile(filepath.Join(dir, "existing.txt"))
	if err != nil {
		t.Fatalf("existing.txt should still exist: %v", err)
	}
	if string(data) != "modified" {
		t.Errorf("expected content %q, got %q", "modified", string(data))
	}
}

// Test 19: Model deletes an existing file.
func TestDaemon_ModelDeletesFiles(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	run(t, dir, "init")

	targetFile := filepath.Join(dir, "doomed.txt")
	if err := os.WriteFile(targetFile, []byte("going away"), 0644); err != nil {
		t.Fatalf("writing seed file: %v", err)
	}

	scriptPath, _ := createRealisticMock(t, dir, "deletes-files", MockModelConfig{
		Behaviors: []MockBehavior{
			{
				Marker:        "WOLFCASTLE_COMPLETE",
				ExtraCommands: []string{"rm -f doomed.txt"},
			},
		},
	})
	configureMockModels(t, dir, scriptPath)

	run(t, dir, "project", "create", "delete-files-test")
	run(t, dir, "task", "add", "--node", "delete-files-test", "delete the file")
	setMaxIterations(t, dir, 10)
	run(t, dir, "start")

	if _, err := os.Stat(targetFile); !os.IsNotExist(err) {
		t.Error("doomed.txt should have been deleted by the model")
	}
}

// Test 20: Model calls wolfcastle audit breadcrumb via CLI; verify breadcrumb in state.
func TestDaemon_ModelCallsBreadcrumb(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	run(t, dir, "init")

	scriptPath, _ := createRealisticMock(t, dir, "breadcrumb", MockModelConfig{
		Behaviors: []MockBehavior{
			{
				Marker:          "WOLFCASTLE_COMPLETE",
				WriteBreadcrumb: true,
				BreadcrumbText:  "implemented the auth layer",
			},
		},
	})
	configureMockModels(t, dir, scriptPath)

	run(t, dir, "project", "create", "breadcrumb-test")
	run(t, dir, "task", "add", "--node", "breadcrumb-test", "add auth")
	setMaxIterations(t, dir, 10)
	run(t, dir, "start")

	ns := loadNode(t, dir, "breadcrumb-test")
	found := false
	for _, bc := range ns.Audit.Breadcrumbs {
		if bc.Text == "implemented the auth layer" {
			found = true
			break
		}
	}
	if !found {
		t.Errorf("expected breadcrumb %q in state, got %+v", "implemented the auth layer", ns.Audit.Breadcrumbs)
	}
}

// Test 21: Model calls wolfcastle audit gap via CLI; verify gap in state.
func TestDaemon_ModelCallsGap(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	run(t, dir, "init")

	scriptPath, _ := createRealisticMock(t, dir, "gap", MockModelConfig{
		Behaviors: []MockBehavior{
			{
				Marker:   "WOLFCASTLE_COMPLETE",
				WriteGap: true,
				GapText:  "no error handling for timeout",
			},
		},
	})
	configureMockModels(t, dir, scriptPath)

	run(t, dir, "project", "create", "gap-test")
	run(t, dir, "task", "add", "--node", "gap-test", "implement endpoint")
	setMaxIterations(t, dir, 10)
	run(t, dir, "start")

	ns := loadNode(t, dir, "gap-test")
	found := false
	for _, g := range ns.Audit.Gaps {
		if g.Description == "no error handling for timeout" {
			found = true
			break
		}
	}
	if !found {
		t.Errorf("expected gap %q in state, got %+v", "no error handling for timeout", ns.Audit.Gaps)
	}
}

// Test 22: Model calls wolfcastle audit gap via CLI on a child node; verify
// escalation can be recorded on the parent orchestrator via CLI.
//
// The daemon itself does not auto-escalate gaps; escalation is a separate
// CLI operation. This test verifies the full flow: child records a gap via
// CLI, then calls `wolfcastle audit escalate` to push it to the parent.
func TestDaemon_ModelCallsEscalate(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	run(t, dir, "init")

	// Create a two-level structure: parent orchestrator with a child leaf.
	run(t, dir, "project", "create", "esc-parent")
	run(t, dir, "project", "create", "--node", "esc-parent", "esc-child")
	run(t, dir, "task", "add", "--node", "esc-parent/esc-child", "do child work")

	scriptPath, _ := createRealisticMock(t, dir, "escalate", MockModelConfig{
		Behaviors: []MockBehavior{
			{
				Marker:   "WOLFCASTLE_COMPLETE",
				WriteGap: true,
				GapText:  "missing spec for edge case",
				ExtraCommands: []string{
					"\"$BINARY_PATH\" audit escalate --node \"$NODE_ADDR\" 'missing spec for edge case' 2>/dev/null || true",
				},
			},
		},
	})
	configureMockModels(t, dir, scriptPath)
	setMaxIterations(t, dir, 10)
	run(t, dir, "start")

	// The gap should be on the child node.
	childNS := loadNode(t, dir, "esc-parent/esc-child")
	gapFound := false
	for _, g := range childNS.Audit.Gaps {
		if g.Description == "missing spec for edge case" {
			gapFound = true
			break
		}
	}
	if !gapFound {
		t.Errorf("expected gap on child node, got %+v", childNS.Audit.Gaps)
	}

	// The escalation should be on the parent node.
	parentNS := loadNode(t, dir, "esc-parent")
	escFound := false
	for _, e := range parentNS.Audit.Escalations {
		if e.Description == "missing spec for edge case" {
			escFound = true
			break
		}
	}
	if !escFound {
		t.Errorf("expected escalation on parent node, got %+v", parentNS.Audit.Escalations)
	}
}

// Test 23: Model calls `wolfcastle task complete` via CLI before emitting
// the WOLFCASTLE_COMPLETE marker.
//
// The daemon's own state save happens after the model invocation finishes.
// If the model calls `task complete` via CLI, that write may be overwritten
// by the daemon's subsequent save. The test documents this behavior: the
// task ends up complete regardless, because the daemon also processes the
// WOLFCASTLE_COMPLETE marker and transitions the task.
func TestDaemon_ModelCallsTaskComplete(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	run(t, dir, "init")

	scriptPath, _ := createRealisticMock(t, dir, "task-complete-cli", MockModelConfig{
		Behaviors: []MockBehavior{
			{
				Marker:           "WOLFCASTLE_COMPLETE",
				CallTaskComplete: true,
			},
		},
	})
	configureMockModels(t, dir, scriptPath)

	run(t, dir, "project", "create", "tc-cli-test")
	run(t, dir, "task", "add", "--node", "tc-cli-test", "complete via CLI")
	setMaxIterations(t, dir, 10)
	run(t, dir, "start")

	ns := loadNode(t, dir, "tc-cli-test")
	for _, task := range ns.Tasks {
		if task.ID == "task-0001" {
			if task.State != state.StatusComplete {
				t.Errorf("expected task-1 complete, got %s", task.State)
			}
			return
		}
	}
	t.Error("task-1 not found in node state")
}

// Test 24: Model calls `wolfcastle task block` via CLI before emitting
// the WOLFCASTLE_BLOCKED marker.
//
// Similar to test 23: the daemon's own processing of the BLOCKED marker
// will transition the task to blocked. The CLI call may be overwritten,
// but the end state should be blocked.
func TestDaemon_ModelCallsTaskBlock(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	run(t, dir, "init")

	scriptPath, _ := createRealisticMock(t, dir, "task-block-cli", MockModelConfig{
		Behaviors: []MockBehavior{
			{
				Marker:          "WOLFCASTLE_BLOCKED",
				CallTaskBlock:   true,
				TaskBlockReason: "upstream API unavailable",
			},
		},
	})
	configureMockModels(t, dir, scriptPath)

	run(t, dir, "project", "create", "tb-cli-test")
	run(t, dir, "task", "add", "--node", "tb-cli-test", "block via CLI")
	setMaxIterations(t, dir, 10)
	run(t, dir, "start")

	ns := loadNode(t, dir, "tb-cli-test")
	for _, task := range ns.Tasks {
		if task.ID == "task-0001" {
			if task.State != state.StatusBlocked {
				t.Errorf("expected task-1 blocked, got %s", task.State)
			}
			return
		}
	}
	t.Error("task-1 not found in node state")
}

// Test 25: Model creates a file but emits no terminal marker.
// The file should persist on disk, but the task remains in_progress
// because the daemon treats a missing marker as a failed invocation.
func TestDaemon_ModelCreatesFilesThenFails(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	run(t, dir, "init")

	// Use createNoMarkerStopAfterMock-style behavior: no marker, but
	// the mock creates a file. We use createRealisticMock with no marker
	// and stop after one invocation via ExtraCommands.
	stopFile := filepath.Join(dir, ".wolfcastle", "stop")
	scriptPath, _ := createRealisticMock(t, dir, "creates-then-fails", MockModelConfig{
		Behaviors: []MockBehavior{
			{
				Marker:      "", // no terminal marker
				CreateFiles: map[string]string{"orphan.txt": "left behind"},
				ExtraCommands: []string{
					"touch \"" + stopFile + "\"",
				},
			},
		},
	})
	configureMockModels(t, dir, scriptPath)

	run(t, dir, "project", "create", "fail-with-file")
	run(t, dir, "task", "add", "--node", "fail-with-file", "create then fail")
	setMaxIterations(t, dir, 10)
	run(t, dir, "start")

	// The file should exist on disk despite the failed invocation.
	data, err := os.ReadFile(filepath.Join(dir, "orphan.txt"))
	if err != nil {
		t.Fatalf("orphan.txt should persist even after failed invocation: %v", err)
	}
	if string(data) != "left behind" {
		t.Errorf("expected content %q, got %q", "left behind", string(data))
	}

	// The task should still be in_progress (no successful completion).
	ns := loadNode(t, dir, "fail-with-file")
	for _, task := range ns.Tasks {
		if task.ID == "task-0001" {
			if task.State != state.StatusInProgress {
				t.Errorf("expected task-1 in_progress after failed invocation, got %s", task.State)
			}
			return
		}
	}
	t.Error("task-1 not found in node state")
}

// Test 26: Model calls wolfcastle audit breadcrumb twice via CLI in a single
// invocation; verify both are recorded.
func TestDaemon_ModelCallsBreadcrumbMultipleTimes(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	run(t, dir, "init")

	// Use ExtraCommands to issue a second breadcrumb CLI call.
	scriptPath, _ := createRealisticMock(t, dir, "multi-breadcrumb", MockModelConfig{
		Behaviors: []MockBehavior{
			{
				Marker:          "WOLFCASTLE_COMPLETE",
				WriteBreadcrumb: true,
				BreadcrumbText:  "first breadcrumb",
				ExtraCommands: []string{
					"\"$BINARY_PATH\" audit breadcrumb --node \"$NODE_ADDR\" 'second breadcrumb' 2>/dev/null || true",
				},
			},
		},
	})
	configureMockModels(t, dir, scriptPath)

	run(t, dir, "project", "create", "multi-bc-test")
	run(t, dir, "task", "add", "--node", "multi-bc-test", "emit two breadcrumbs")
	setMaxIterations(t, dir, 10)
	run(t, dir, "start")

	ns := loadNode(t, dir, "multi-bc-test")
	if len(ns.Audit.Breadcrumbs) < 2 {
		t.Fatalf("expected at least 2 breadcrumbs, got %d: %+v", len(ns.Audit.Breadcrumbs), ns.Audit.Breadcrumbs)
	}

	texts := map[string]bool{}
	for _, bc := range ns.Audit.Breadcrumbs {
		texts[bc.Text] = true
	}
	if !texts["first breadcrumb"] {
		t.Error("missing 'first breadcrumb' in recorded breadcrumbs")
	}
	if !texts["second breadcrumb"] {
		t.Error("missing 'second breadcrumb' in recorded breadcrumbs")
	}
}

// Test 27: Model calls `wolfcastle task complete` via CLI AND emits the
// WOLFCASTLE_COMPLETE marker (double completion signal). The daemon should
// handle this without error.
func TestDaemon_ModelCallsTaskCompleteAndEmitsComplete(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	run(t, dir, "init")

	scriptPath, _ := createRealisticMock(t, dir, "double-complete", MockModelConfig{
		Behaviors: []MockBehavior{
			{
				Marker:           "WOLFCASTLE_COMPLETE",
				CallTaskComplete: true,
			},
		},
	})
	configureMockModels(t, dir, scriptPath)

	run(t, dir, "project", "create", "double-complete-test")
	run(t, dir, "task", "add", "--node", "double-complete-test", "double complete")
	setMaxIterations(t, dir, 10)
	run(t, dir, "start")

	// The daemon's in-memory state wins: it loaded the node before the
	// model ran, so the CLI's task-complete write gets overwritten by the
	// daemon's subsequent save. The daemon then processes the COMPLETE
	// marker and transitions the task itself. The key assertion is that
	// no error occurs and the task ends up complete.
	ns := loadNode(t, dir, "double-complete-test")
	for _, task := range ns.Tasks {
		if task.ID == "task-0001" {
			if task.State != state.StatusComplete {
				t.Errorf("expected task-1 complete after double signal, got %s", task.State)
			}
			return
		}
	}
	t.Error("task-1 not found in node state")
}

// Test 28: Model calls `wolfcastle task block` via CLI AND emits the
// WOLFCASTLE_BLOCKED marker (double block signal). The daemon should
// handle this without error.
func TestDaemon_ModelCallsTaskBlockAndEmitsBlocked(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	run(t, dir, "init")

	scriptPath, _ := createRealisticMock(t, dir, "double-block", MockModelConfig{
		Behaviors: []MockBehavior{
			{
				Marker:          "WOLFCASTLE_BLOCKED",
				CallTaskBlock:   true,
				TaskBlockReason: "waiting on review",
			},
		},
	})
	configureMockModels(t, dir, scriptPath)

	run(t, dir, "project", "create", "double-block-test")
	run(t, dir, "task", "add", "--node", "double-block-test", "double block")
	setMaxIterations(t, dir, 10)
	run(t, dir, "start")

	// Same as test 27: the daemon's in-memory state wins over the CLI
	// call. The BLOCKED marker processing handles the transition. The
	// key assertion is that no error occurs and the task ends up blocked.
	ns := loadNode(t, dir, "double-block-test")
	for _, task := range ns.Tasks {
		if task.ID == "task-0001" {
			if task.State != state.StatusBlocked {
				t.Errorf("expected task-1 blocked after double signal, got %s", task.State)
			}
			return
		}
	}
	t.Error("task-1 not found in node state")
}
