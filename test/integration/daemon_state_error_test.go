//go:build integration

package integration

import (
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"testing"

	"github.com/dorkusprime/wolfcastle/internal/state"
)

// ---------------------------------------------------------------------------
// Multi-iteration tests (37-43)
// ---------------------------------------------------------------------------

// Test 37: Clean YIELD then COMPLETE across two iterations.
func TestDaemon_CleanYieldComplete_TwoIterations(t *testing.T) {
	dir := t.TempDir()
	run(t, dir, "init")

	scriptPath, counterFile := createCounterMock(t, dir, 1) // 1 yield, then complete
	configureMockModels(t, dir, scriptPath)

	run(t, dir, "project", "create", "two-iter")
	run(t, dir, "task", "add", "--node", "two-iter", "two iteration task")

	setMaxIterations(t, dir, 10)
	run(t, dir, "start")

	data, err := os.ReadFile(counterFile)
	if err != nil {
		t.Fatalf("reading counter: %v", err)
	}
	count, _ := strconv.Atoi(strings.TrimSpace(string(data)))
	if count != 2 {
		t.Errorf("expected 2 invocations (1 yield + 1 complete), got %d", count)
	}

	ns := loadNode(t, dir, "two-iter")
	for _, task := range ns.Tasks {
		if task.ID == "task-1" {
			if task.State != state.StatusComplete {
				t.Errorf("expected complete, got %s", task.State)
			}
			return
		}
	}
	t.Error("task-1 not found")
}

// Test 38: YIELD five times, then COMPLETE on the sixth invocation.
func TestDaemon_YieldFiveThenComplete(t *testing.T) {
	dir := t.TempDir()
	run(t, dir, "init")

	scriptPath, counterFile := createCounterMock(t, dir, 5)
	configureMockModels(t, dir, scriptPath)

	run(t, dir, "project", "create", "five-yield")
	run(t, dir, "task", "add", "--node", "five-yield", "five yields then done")

	setMaxIterations(t, dir, 20)
	run(t, dir, "start")

	data, err := os.ReadFile(counterFile)
	if err != nil {
		t.Fatalf("reading counter: %v", err)
	}
	count, _ := strconv.Atoi(strings.TrimSpace(string(data)))
	if count != 6 {
		t.Errorf("expected 6 invocations (5 yields + 1 complete), got %d", count)
	}

	ns := loadNode(t, dir, "five-yield")
	for _, task := range ns.Tasks {
		if task.ID == "task-1" && task.State != state.StatusComplete {
			t.Errorf("expected complete, got %s", task.State)
		}
	}
}

// Test 39: Each iteration creates a different file, all present after completion.
func TestDaemon_YieldWithDifferentFilesEachIteration(t *testing.T) {
	dir := t.TempDir()
	run(t, dir, "init")

	scriptPath, _ := createRealisticMock(t, dir, "diff-files", MockModelConfig{
		Behaviors: []MockBehavior{
			{
				Marker:      "WOLFCASTLE_YIELD",
				CreateFiles: map[string]string{"iter1.txt": "first"},
			},
			{
				Marker:      "WOLFCASTLE_YIELD",
				CreateFiles: map[string]string{"iter2.txt": "second"},
			},
			{
				Marker:      "WOLFCASTLE_COMPLETE",
				CreateFiles: map[string]string{"iter3.txt": "third"},
			},
		},
	})
	configureMockModels(t, dir, scriptPath)

	run(t, dir, "project", "create", "diff-files-test")
	run(t, dir, "task", "add", "--node", "diff-files-test", "create different files")

	setMaxIterations(t, dir, 10)
	run(t, dir, "start")

	for _, name := range []string{"iter1.txt", "iter2.txt", "iter3.txt"} {
		path := filepath.Join(dir, name)
		if _, err := os.Stat(path); os.IsNotExist(err) {
			t.Errorf("expected %s to exist after daemon run", name)
		}
	}
}

// Test 40: Breadcrumb text differs across iterations; both are recorded.
func TestDaemon_BreadcrumbDiffersAcrossIterations(t *testing.T) {
	dir := t.TempDir()
	run(t, dir, "init")

	scriptPath, _ := createRealisticMock(t, dir, "bc-diff", MockModelConfig{
		Behaviors: []MockBehavior{
			{
				Marker:          "WOLFCASTLE_YIELD",
				WriteBreadcrumb: true,
				BreadcrumbText:  "breadcrumb alpha",
			},
			{
				Marker:          "WOLFCASTLE_COMPLETE",
				WriteBreadcrumb: true,
				BreadcrumbText:  "breadcrumb beta",
			},
		},
	})
	configureMockModels(t, dir, scriptPath)

	run(t, dir, "project", "create", "bc-diff-test")
	run(t, dir, "task", "add", "--node", "bc-diff-test", "breadcrumb test")

	setMaxIterations(t, dir, 10)
	run(t, dir, "start")

	ns := loadNode(t, dir, "bc-diff-test")
	texts := make(map[string]bool)
	for _, bc := range ns.Audit.Breadcrumbs {
		texts[bc.Text] = true
	}
	if !texts["breadcrumb alpha"] {
		t.Error("missing breadcrumb alpha")
	}
	if !texts["breadcrumb beta"] {
		t.Error("missing breadcrumb beta")
	}
}

// Test 41: First invocation fails (no marker), second completes. Failure count is recorded.
func TestDaemon_FailureThenComplete(t *testing.T) {
	dir := t.TempDir()
	run(t, dir, "init")

	scriptPath, _ := createRealisticMock(t, dir, "fail-then-ok", MockModelConfig{
		Behaviors: []MockBehavior{
			{Marker: ""}, // no marker => failure
			{Marker: "WOLFCASTLE_COMPLETE"},
		},
	})
	configureMockModels(t, dir, scriptPath)

	setFailureAndIterationConfig(t, dir, 10, 0, 50, 20)

	run(t, dir, "project", "create", "fail-ok-test")
	run(t, dir, "task", "add", "--node", "fail-ok-test", "fail then succeed")

	run(t, dir, "start")

	ns := loadNode(t, dir, "fail-ok-test")
	for _, task := range ns.Tasks {
		if task.ID == "task-1" {
			if task.State != state.StatusComplete {
				t.Errorf("expected complete, got %s", task.State)
			}
			if task.FailureCount < 1 {
				t.Errorf("expected failure count >= 1, got %d", task.FailureCount)
			}
			return
		}
	}
	t.Error("task-1 not found")
}

// Test 42: Alternating failures and yields, eventually completing.
func TestDaemon_AlternatingFailuresAndYields(t *testing.T) {
	dir := t.TempDir()
	run(t, dir, "init")

	scriptPath, _ := createRealisticMock(t, dir, "alt-fail-yield", MockModelConfig{
		Behaviors: []MockBehavior{
			{Marker: ""},                    // fail
			{Marker: "WOLFCASTLE_YIELD"},    // yield
			{Marker: ""},                    // fail
			{Marker: "WOLFCASTLE_COMPLETE"}, // done
		},
	})
	configureMockModels(t, dir, scriptPath)

	setFailureAndIterationConfig(t, dir, 10, 0, 50, 20)

	run(t, dir, "project", "create", "alt-test")
	run(t, dir, "task", "add", "--node", "alt-test", "alternating task")

	run(t, dir, "start")

	ns := loadNode(t, dir, "alt-test")
	for _, task := range ns.Tasks {
		if task.ID == "task-1" {
			if task.State != state.StatusComplete {
				t.Errorf("expected complete, got %s", task.State)
			}
			if task.FailureCount < 2 {
				t.Errorf("expected failure count >= 2, got %d", task.FailureCount)
			}
			return
		}
	}
	t.Error("task-1 not found")
}

// Test 43: When failure count exceeds decomposition threshold, the task is flagged.
// Full decomposition (creating child nodes via CLI) would require the mock to
// call wolfcastle project create, which is tested in isolation. Here we verify
// the needs_decomposition flag is set by the daemon.
func TestDaemon_DecompositionCreatesChildren(t *testing.T) {
	dir := t.TempDir()
	run(t, dir, "init")

	// 4 invocations with no marker, then stop
	scriptPath := createNoMarkerStopAfterMock(t, dir, 4)
	configureMockModels(t, dir, scriptPath)

	// Decomposition threshold = 2, max depth = 3, hard cap = 50
	setFailureAndIterationConfig(t, dir, 2, 3, 50, 20)

	run(t, dir, "project", "create", "decomp-test")
	run(t, dir, "task", "add", "--node", "decomp-test", "decomposable task")

	run(t, dir, "start")

	ns := loadNode(t, dir, "decomp-test")
	for _, task := range ns.Tasks {
		if task.ID == "task-1" {
			if !task.NeedsDecomposition {
				t.Error("expected needs_decomposition to be true after exceeding decomposition threshold")
			}
			if task.FailureCount < 2 {
				t.Errorf("expected failure count >= 2, got %d", task.FailureCount)
			}
			return
		}
	}
	t.Error("task-1 not found")
}

// ---------------------------------------------------------------------------
// State tests (44-49)
// ---------------------------------------------------------------------------

// Test 44: A freshly created task starts in not_started state.
func TestDaemon_TaskStartsNotStarted(t *testing.T) {
	dir := t.TempDir()
	run(t, dir, "init")

	run(t, dir, "project", "create", "fresh-state")
	run(t, dir, "task", "add", "--node", "fresh-state", "fresh task")

	ns := loadNode(t, dir, "fresh-state")
	for _, task := range ns.Tasks {
		if task.ID == "task-1" {
			if task.State != state.StatusNotStarted {
				t.Errorf("expected not_started, got %s", task.State)
			}
			return
		}
	}
	t.Error("task-1 not found")
}

// Test 45: A task left in_progress (crash) is picked up by the daemon on restart.
func TestDaemon_TaskStartsInProgress_CrashRecovery(t *testing.T) {
	dir := t.TempDir()
	run(t, dir, "init")

	scriptPath := createMockModel(t, dir, "complete", "complete")
	configureMockModels(t, dir, scriptPath)

	run(t, dir, "project", "create", "crash-recover")
	run(t, dir, "task", "add", "--node", "crash-recover", "interrupted task")

	// Simulate crash: set task to in_progress manually
	ns := loadNode(t, dir, "crash-recover")
	for i := range ns.Tasks {
		if ns.Tasks[i].ID == "task-1" {
			ns.Tasks[i].State = state.StatusInProgress
		}
	}
	saveNode(t, dir, "crash-recover", ns)

	setMaxIterations(t, dir, 10)
	run(t, dir, "start")

	ns = loadNode(t, dir, "crash-recover")
	for _, task := range ns.Tasks {
		if task.ID == "task-1" {
			if task.State != state.StatusComplete {
				t.Errorf("expected complete after crash recovery, got %s", task.State)
			}
			return
		}
	}
	t.Error("task-1 not found")
}

// Test 46: A blocked task must be unblocked before the daemon will work on it.
func TestDaemon_TaskStartsBlocked_ThenUnblocked(t *testing.T) {
	dir := t.TempDir()
	run(t, dir, "init")

	scriptPath := createMockModel(t, dir, "complete", "complete")
	configureMockModels(t, dir, scriptPath)

	run(t, dir, "project", "create", "block-test")
	run(t, dir, "task", "add", "--node", "block-test", "blocked task")

	// Set task to blocked (requires going through in_progress first)
	ns := loadNode(t, dir, "block-test")
	for i := range ns.Tasks {
		if ns.Tasks[i].ID == "task-1" {
			ns.Tasks[i].State = state.StatusBlocked
			ns.Tasks[i].BlockedReason = "external dependency"
		}
	}
	ns.State = state.StatusBlocked
	saveNode(t, dir, "block-test", ns)

	// Unblock before starting daemon (Tier 1: simple flip)
	run(t, dir, "task", "unblock", "--node", "block-test/task-1")

	setMaxIterations(t, dir, 10)
	run(t, dir, "start")

	ns = loadNode(t, dir, "block-test")
	for _, task := range ns.Tasks {
		if task.ID == "task-1" {
			if task.State != state.StatusComplete {
				t.Errorf("expected complete after unblock, got %s", task.State)
			}
			return
		}
	}
	t.Error("task-1 not found")
}

// Test 47: Multiple tasks on a single leaf are processed sequentially.
func TestDaemon_MultipleTasksInOneLeaf(t *testing.T) {
	dir := t.TempDir()
	run(t, dir, "init")

	// Use a counter mock that yields enough times so the daemon processes
	// all tasks. With 3 tasks, we need at least 3 complete iterations.
	// Using yieldCount=0 means every call emits COMPLETE. The stop file
	// is only created by the last behavior, so we use createRealisticMock
	// with enough COMPLETE behaviors.
	scriptPath, _ := createRealisticMock(t, dir, "multi-task-mock", MockModelConfig{
		Behaviors: []MockBehavior{
			{Marker: "WOLFCASTLE_COMPLETE"},
			{Marker: "WOLFCASTLE_COMPLETE"},
			{Marker: "WOLFCASTLE_COMPLETE"},
			// Extra behavior in case an audit task is auto-created
			{Marker: "WOLFCASTLE_COMPLETE"},
		},
	})
	configureMockModels(t, dir, scriptPath)

	run(t, dir, "project", "create", "multi-task")
	run(t, dir, "task", "add", "--node", "multi-task", "first task")
	run(t, dir, "task", "add", "--node", "multi-task", "second task")
	run(t, dir, "task", "add", "--node", "multi-task", "third task")

	setMaxIterations(t, dir, 20)
	run(t, dir, "start")

	ns := loadNode(t, dir, "multi-task")
	completedCount := 0
	for _, task := range ns.Tasks {
		if !task.IsAudit && task.State == state.StatusComplete {
			completedCount++
		}
	}
	if completedCount < 3 {
		t.Errorf("expected at least 3 non-audit tasks complete, got %d", completedCount)
	}
}

// Test 48: Multiple leaf nodes under a parent orchestrator are all completed.
func TestDaemon_MultipleLeavesUnderOrchestrator(t *testing.T) {
	dir := t.TempDir()
	run(t, dir, "init")

	// Need multiple COMPLETE behaviors because each invocation checks for
	// a stop file. The last behavior (catch-all) always creates the stop
	// file, so we provide enough behaviors to cover both leaves plus any
	// audit tasks that may be auto-created.
	scriptPath, _ := createRealisticMock(t, dir, "multi-leaf-mock", MockModelConfig{
		Behaviors: []MockBehavior{
			{Marker: "WOLFCASTLE_COMPLETE"},
			{Marker: "WOLFCASTLE_COMPLETE"},
			// Extra for audit tasks
			{Marker: "WOLFCASTLE_COMPLETE"},
			{Marker: "WOLFCASTLE_COMPLETE"},
		},
	})
	configureMockModels(t, dir, scriptPath)

	run(t, dir, "project", "create", "parent-orch", "--type", "orchestrator")
	run(t, dir, "project", "create", "leaf-a", "--node", "parent-orch")
	run(t, dir, "project", "create", "leaf-b", "--node", "parent-orch")
	run(t, dir, "task", "add", "--node", "parent-orch/leaf-a", "task in leaf A")
	run(t, dir, "task", "add", "--node", "parent-orch/leaf-b", "task in leaf B")

	setMaxIterations(t, dir, 20)
	run(t, dir, "start")

	for _, leaf := range []string{"parent-orch/leaf-a", "parent-orch/leaf-b"} {
		ns := loadNode(t, dir, leaf)
		for _, task := range ns.Tasks {
			if task.ID == "task-1" && task.State != state.StatusComplete {
				t.Errorf("%s/task-1: expected complete, got %s", leaf, task.State)
			}
		}
	}
}

// Test 49: An audit task on a leaf executes after all regular tasks complete.
// Note: Audit tasks on orchestrators are not reachable through DFS navigation
// after child propagation marks the orchestrator complete. This test verifies
// that leaf-level audit tasks (is_audit=true) do run correctly after the leaf's
// regular tasks finish.
func TestDaemon_AuditTaskExecutesAfterAllComplete(t *testing.T) {
	dir := t.TempDir()
	run(t, dir, "init")

	// Three COMPLETE behaviors: task-1, task-2 (audit), plus catch-all
	scriptPath, _ := createRealisticMock(t, dir, "audit-leaf-mock", MockModelConfig{
		Behaviors: []MockBehavior{
			{Marker: "WOLFCASTLE_COMPLETE"},
			{Marker: "WOLFCASTLE_COMPLETE"},
			{Marker: "WOLFCASTLE_COMPLETE"},
		},
	})
	configureMockModels(t, dir, scriptPath)

	run(t, dir, "project", "create", "audit-leaf")
	run(t, dir, "task", "add", "--node", "audit-leaf", "regular work")

	// Manually add an audit task to the leaf
	ns := loadNode(t, dir, "audit-leaf")
	ns.Tasks = append(ns.Tasks, state.Task{
		ID:          "audit-1",
		Description: "audit the leaf",
		State:       state.StatusNotStarted,
		IsAudit:     true,
	})
	saveNode(t, dir, "audit-leaf", ns)

	setMaxIterations(t, dir, 20)
	run(t, dir, "start")

	ns = loadNode(t, dir, "audit-leaf")
	for _, task := range ns.Tasks {
		if task.ID == "audit-1" {
			if task.State != state.StatusComplete {
				t.Errorf("audit task expected complete, got %s", task.State)
			}
			return
		}
	}
	t.Error("audit-1 not found on leaf")
}

// ---------------------------------------------------------------------------
// Error tests (50-55)
// ---------------------------------------------------------------------------

// Test 50: Model command not found causes a daemon error, not a panic.
func TestDaemon_ModelCommandNotFound(t *testing.T) {
	dir := t.TempDir()
	run(t, dir, "init")

	// Point models at a nonexistent binary
	configureWithArgs(t, dir, "/nonexistent/model-binary-xyz", nil)

	run(t, dir, "project", "create", "cmd-not-found")
	run(t, dir, "task", "add", "--node", "cmd-not-found", "doomed task")

	setMaxIterations(t, dir, 2)

	// The daemon should not panic. It may exit with an error or report
	// an iteration error. Either way, the process should terminate.
	out := run(t, dir, "start")

	// The task should not be complete
	ns := loadNode(t, dir, "cmd-not-found")
	for _, task := range ns.Tasks {
		if task.ID == "task-1" && task.State == state.StatusComplete {
			t.Error("task should not be complete when model command is not found")
		}
	}
	_ = out
}

// Test 51: Model exits with non-zero exit code. Daemon treats it as a failure
// (no terminal marker), increments failure count, and continues.
func TestDaemon_ModelExitsNonZero(t *testing.T) {
	dir := t.TempDir()
	run(t, dir, "init")

	// Create a script that exits 1 on first call, then completes
	scriptPath, _ := createRealisticMock(t, dir, "exit-nonzero", MockModelConfig{
		Behaviors: []MockBehavior{
			{
				Marker:        "",
				ExtraCommands: []string{"exit 1"},
			},
			{
				Marker: "WOLFCASTLE_COMPLETE",
			},
		},
	})
	configureMockModels(t, dir, scriptPath)

	setFailureAndIterationConfig(t, dir, 10, 0, 50, 20)

	run(t, dir, "project", "create", "nonzero-exit")
	run(t, dir, "task", "add", "--node", "nonzero-exit", "might fail task")

	run(t, dir, "start")

	ns := loadNode(t, dir, "nonzero-exit")
	for _, task := range ns.Tasks {
		if task.ID == "task-1" {
			if task.State != state.StatusComplete {
				t.Errorf("expected complete after recovery from non-zero exit, got %s", task.State)
			}
			return
		}
	}
	t.Error("task-1 not found")
}

// Test 52: Model produces empty stdout. Treated as no-marker failure.
func TestDaemon_ModelProducesEmptyStdout(t *testing.T) {
	dir := t.TempDir()
	run(t, dir, "init")

	// Create a mock that produces truly empty output on first call
	scriptsDir := filepath.Join(dir, ".wolfcastle", "mock-scripts")
	if err := os.MkdirAll(scriptsDir, 0755); err != nil {
		t.Fatal(err)
	}
	stopFile := filepath.Join(dir, ".wolfcastle", "stop")
	counterFile := filepath.Join(scriptsDir, "empty-counter.txt")
	if err := os.WriteFile(counterFile, []byte("0"), 0644); err != nil {
		t.Fatal(err)
	}

	emptyScript := filepath.Join(scriptsDir, "empty-stdout.sh")
	body := fmt.Sprintf(`#!/bin/sh
cat > /dev/null
COUNTER_FILE="%s"
COUNT=$(cat "$COUNTER_FILE" 2>/dev/null || printf '0')
COUNT=$((COUNT + 1))
printf '%%s' "$COUNT" > "$COUNTER_FILE"
if [ "$COUNT" -le 1 ]; then
  # Produce absolutely nothing on stdout
  true
else
  printf '{"type":"result","text":"WOLFCASTLE_COMPLETE"}\n'
  touch "%s"
fi
`, counterFile, stopFile)
	if err := os.WriteFile(emptyScript, []byte(body), 0755); err != nil {
		t.Fatal(err)
	}
	configureMockModels(t, dir, emptyScript)

	setFailureAndIterationConfig(t, dir, 10, 0, 50, 20)

	run(t, dir, "project", "create", "empty-stdout-test")
	run(t, dir, "task", "add", "--node", "empty-stdout-test", "empty output task")

	run(t, dir, "start")

	ns := loadNode(t, dir, "empty-stdout-test")
	for _, task := range ns.Tasks {
		if task.ID == "task-1" {
			if task.FailureCount < 1 {
				t.Errorf("expected failure count >= 1 for empty stdout, got %d", task.FailureCount)
			}
			if task.State != state.StatusComplete {
				t.Errorf("expected complete after recovery, got %s", task.State)
			}
			return
		}
	}
	t.Error("task-1 not found")
}

// Test 53: Model produces ~1MB of output. The daemon handles it without crashing.
func TestDaemon_ModelProducesHugeStdout(t *testing.T) {
	dir := t.TempDir()
	run(t, dir, "init")

	scriptsDir := filepath.Join(dir, ".wolfcastle", "mock-scripts")
	if err := os.MkdirAll(scriptsDir, 0755); err != nil {
		t.Fatal(err)
	}
	stopFile := filepath.Join(dir, ".wolfcastle", "stop")

	// Script generates ~1MB of output, then the terminal marker
	hugeScript := filepath.Join(scriptsDir, "huge-stdout.sh")
	body := fmt.Sprintf(`#!/bin/sh
cat > /dev/null
# Generate ~1MB of output (1024 lines of ~1024 chars each)
i=0
while [ "$i" -lt 1024 ]; do
  printf '{"type":"assistant","text":"%s"}\n' "$(printf '%%0*d' 1000 0)"
  i=$((i + 1))
done
printf '{"type":"result","text":"WOLFCASTLE_COMPLETE"}\n'
touch "%s"
`, "%s", stopFile)
	// Fix the double-format: the inner %s is literal in the printf
	body = fmt.Sprintf(`#!/bin/sh
cat > /dev/null
i=0
while [ "$i" -lt 1024 ]; do
  printf '{"type":"assistant","text":"'
  printf '%%0*d' 1000 0
  printf '"}\n'
  i=$((i + 1))
done
printf '{"type":"result","text":"WOLFCASTLE_COMPLETE"}\n'
touch "%s"
`, stopFile)
	if err := os.WriteFile(hugeScript, []byte(body), 0755); err != nil {
		t.Fatal(err)
	}
	configureMockModels(t, dir, hugeScript)

	run(t, dir, "project", "create", "huge-stdout-test")
	run(t, dir, "task", "add", "--node", "huge-stdout-test", "huge output task")

	setMaxIterations(t, dir, 10)
	run(t, dir, "start")

	ns := loadNode(t, dir, "huge-stdout-test")
	for _, task := range ns.Tasks {
		if task.ID == "task-1" {
			if task.State != state.StatusComplete {
				t.Errorf("expected complete despite huge output, got %s", task.State)
			}
			return
		}
	}
	t.Error("task-1 not found")
}

// Test 54: Model times out. With invocation_timeout_seconds=60 and a mock that
// replaces itself with a long sleep (exec sleep), the daemon should cancel the
// invocation and treat it as failure. The first invocation is killed by the
// timeout; the second completes normally.
// This test takes ~62s due to the minimum allowed timeout being 60s.
func TestDaemon_ModelTimesOut(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping timeout test in short mode (takes ~60s)")
	}

	dir := t.TempDir()
	run(t, dir, "init")

	scriptsDir := filepath.Join(dir, ".wolfcastle", "mock-scripts")
	if err := os.MkdirAll(scriptsDir, 0755); err != nil {
		t.Fatal(err)
	}
	stopFile := filepath.Join(dir, ".wolfcastle", "stop")
	counterFile := filepath.Join(scriptsDir, "timeout-counter.txt")
	if err := os.WriteFile(counterFile, []byte("0"), 0644); err != nil {
		t.Fatal(err)
	}

	// First call: consume stdin, bump counter, then exec sleep (replaces
	// the shell process entirely, so SIGKILL from context reaches it).
	// Second call: complete immediately.
	timeoutScript := filepath.Join(scriptsDir, "timeout-mock.sh")
	body := fmt.Sprintf(`#!/bin/sh
cat > /dev/null
COUNTER_FILE="%s"
COUNT=$(cat "$COUNTER_FILE" 2>/dev/null || printf '0')
COUNT=$((COUNT + 1))
printf '%%s' "$COUNT" > "$COUNTER_FILE"
if [ "$COUNT" -le 1 ]; then
  exec sleep 300
else
  printf '{"type":"result","text":"WOLFCASTLE_COMPLETE"}\n'
  touch "%s"
fi
`, counterFile, stopFile)
	if err := os.WriteFile(timeoutScript, []byte(body), 0755); err != nil {
		t.Fatal(err)
	}
	configureMockModels(t, dir, timeoutScript)

	// Set invocation timeout to 60s (minimum allowed), high failure caps
	mergeLocalConfig(t, dir, map[string]any{
		"daemon": map[string]any{
			"max_iterations":             20,
			"invocation_timeout_seconds": 60,
		},
		"failure": map[string]any{
			"decomposition_threshold": 10,
			"max_decomposition_depth": 0,
			"hard_cap":                50,
		},
	})

	run(t, dir, "project", "create", "timeout-test")
	run(t, dir, "task", "add", "--node", "timeout-test", "slow task")

	run(t, dir, "start")

	ns := loadNode(t, dir, "timeout-test")
	for _, task := range ns.Tasks {
		if task.ID == "task-1" {
			if task.State != state.StatusComplete {
				t.Errorf("expected complete after timeout recovery, got %s", task.State)
			}
			return
		}
	}
	t.Error("task-1 not found")
}

// Test 55: Model writes to stderr. The daemon should not crash and the task
// should still complete based on stdout markers.
func TestDaemon_ModelWritesToStderr(t *testing.T) {
	dir := t.TempDir()
	run(t, dir, "init")

	scriptsDir := filepath.Join(dir, ".wolfcastle", "mock-scripts")
	if err := os.MkdirAll(scriptsDir, 0755); err != nil {
		t.Fatal(err)
	}
	stopFile := filepath.Join(dir, ".wolfcastle", "stop")

	stderrScript := filepath.Join(scriptsDir, "stderr-mock.sh")
	body := fmt.Sprintf(`#!/bin/sh
cat > /dev/null
echo "WARNING: something alarming happened" >&2
echo "ERROR: but we soldier on" >&2
printf '{"type":"assistant","text":"Working despite warnings."}\n'
printf '{"type":"result","text":"WOLFCASTLE_COMPLETE"}\n'
touch "%s"
`, stopFile)
	if err := os.WriteFile(stderrScript, []byte(body), 0755); err != nil {
		t.Fatal(err)
	}
	configureMockModels(t, dir, stderrScript)

	run(t, dir, "project", "create", "stderr-test")
	run(t, dir, "task", "add", "--node", "stderr-test", "stderr producing task")

	setMaxIterations(t, dir, 10)
	run(t, dir, "start")

	ns := loadNode(t, dir, "stderr-test")
	for _, task := range ns.Tasks {
		if task.ID == "task-1" {
			if task.State != state.StatusComplete {
				t.Errorf("expected complete despite stderr output, got %s", task.State)
			}
			return
		}
	}
	t.Error("task-1 not found")
}
