//go:build integration

package integration

import (
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"testing"

	"github.com/dorkusprime/wolfcastle/internal/state"
)

func TestDaemon_SimpleComplete(t *testing.T) {
	dir := t.TempDir()
	run(t, dir, "init")

	scriptPath := createMockModel(t, dir, "complete", "complete")
	configureMockModels(t, dir, scriptPath)

	run(t, dir, "project", "create", "test-project")
	run(t, dir, "task", "add", "--node", "test-project", "do something")

	setMaxIterations(t, dir, 10)
	run(t, dir, "start")

	ns := loadNode(t, dir, "test-project")
	found := false
	for _, task := range ns.Tasks {
		if task.ID == "task-1" {
			found = true
			if task.State != state.StatusComplete {
				t.Errorf("expected task-1 state complete, got %s", task.State)
			}
		}
	}
	if !found {
		t.Error("task-1 not found in node state")
	}
}

func TestDaemon_YieldThenComplete(t *testing.T) {
	dir := t.TempDir()
	run(t, dir, "init")

	scriptPath, counterFile := createCounterMock(t, dir, 1)
	configureMockModels(t, dir, scriptPath)

	run(t, dir, "project", "create", "yield-test")
	run(t, dir, "task", "add", "--node", "yield-test", "yielding task")

	setMaxIterations(t, dir, 10)
	run(t, dir, "start")

	// Read counter to verify 2 invocations (1 yield + 1 complete)
	data, err := os.ReadFile(counterFile)
	if err != nil {
		t.Fatalf("reading counter file: %v", err)
	}
	count, err := strconv.Atoi(strings.TrimSpace(string(data)))
	if err != nil {
		t.Fatalf("parsing counter: %v", err)
	}
	if count != 2 {
		t.Errorf("expected 2 invocations, got %d", count)
	}

	ns := loadNode(t, dir, "yield-test")
	for _, task := range ns.Tasks {
		if task.ID == "task-1" && task.State != state.StatusComplete {
			t.Errorf("expected task-1 complete, got %s", task.State)
		}
	}
}

func TestDaemon_Blocked(t *testing.T) {
	dir := t.TempDir()
	run(t, dir, "init")

	scriptPath := createMockModel(t, dir, "blocked", "blocked")
	configureMockModels(t, dir, scriptPath)

	run(t, dir, "project", "create", "blocked-test")
	run(t, dir, "task", "add", "--node", "blocked-test", "will block")

	setMaxIterations(t, dir, 10)
	run(t, dir, "start")

	ns := loadNode(t, dir, "blocked-test")
	for _, task := range ns.Tasks {
		if task.ID == "task-1" {
			if task.State != state.StatusBlocked {
				t.Errorf("expected task-1 blocked, got %s", task.State)
			}
			return
		}
	}
	t.Error("task-1 not found")
}

func TestDaemon_FailureEscalation(t *testing.T) {
	dir := t.TempDir()
	run(t, dir, "init")

	// Use a no-marker mock that creates the stop file after 5 invocations
	scriptPath := createNoMarkerStopAfterMock(t, dir, 5)
	configureMockModels(t, dir, scriptPath)

	// Set a low decomposition threshold (3) so it triggers before stop.
	// hard_cap=50 so the task stays in_progress long enough.
	setFailureAndIterationConfig(t, dir, 3, 5, 50, 20)

	run(t, dir, "project", "create", "fail-test")
	run(t, dir, "task", "add", "--node", "fail-test", "doomed task")

	run(t, dir, "start")

	ns := loadNode(t, dir, "fail-test")
	for _, task := range ns.Tasks {
		if task.ID == "task-1" {
			if task.FailureCount < 3 {
				t.Errorf("expected failure count >= 3, got %d", task.FailureCount)
			}
			if !task.NeedsDecomposition {
				t.Error("expected needs_decomposition to be true after hitting threshold")
			}
			return
		}
	}
	t.Error("task-1 not found")
}

func TestDaemon_HardCapAutoBlock(t *testing.T) {
	dir := t.TempDir()
	run(t, dir, "init")

	// Use a no-marker mock that creates the stop file after 4 invocations.
	// With hard_cap=2: task-1 blocks after 2 failures, audit task blocks
	// after 2 more, totaling 4 invocations.
	scriptPath := createNoMarkerStopAfterMock(t, dir, 4)
	configureMockModels(t, dir, scriptPath)

	// decomposition_threshold must be <= hard_cap per validation rules.
	setFailureAndIterationConfig(t, dir, 2, 0, 2, 20)

	run(t, dir, "project", "create", "hardcap-test")
	run(t, dir, "task", "add", "--node", "hardcap-test", "will hit hard cap")

	run(t, dir, "start")

	ns := loadNode(t, dir, "hardcap-test")
	for _, task := range ns.Tasks {
		if task.ID == "task-1" {
			if task.State != state.StatusBlocked {
				t.Errorf("expected auto-blocked, got %s", task.State)
			}
			if task.FailureCount < 2 {
				t.Errorf("expected failure count >= 2, got %d", task.FailureCount)
			}
			return
		}
	}
	t.Error("task-1 not found")
}

func TestDaemon_FileCreation(t *testing.T) {
	dir := t.TempDir()
	run(t, dir, "init")

	scriptPath := createMockModel(t, dir, "create-file", "create-file")
	configureMockModels(t, dir, scriptPath)

	run(t, dir, "project", "create", "filecreate-test")
	run(t, dir, "task", "add", "--node", "filecreate-test", "create a file")

	setMaxIterations(t, dir, 10)
	run(t, dir, "start")

	createdFile := filepath.Join(dir, "mock-created-file.txt")
	if _, err := os.Stat(createdFile); os.IsNotExist(err) {
		t.Error("expected mock-created-file.txt to exist after daemon run")
	}
}

func TestDaemon_EmptyTree(t *testing.T) {
	dir := t.TempDir()
	run(t, dir, "init")

	scriptPath := createMockModel(t, dir, "complete", "complete")
	configureMockModels(t, dir, scriptPath)

	// Place a stop file so the daemon exits immediately after finding no work.
	// Without this, the daemon would poll indefinitely since max_iterations
	// only counts work iterations.
	stopFile := filepath.Join(dir, ".wolfcastle", "stop")
	if err := os.WriteFile(stopFile, []byte(""), 0644); err != nil {
		t.Fatalf("creating stop file: %v", err)
	}

	out := run(t, dir, "start")

	// The daemon should not crash
	if !strings.Contains(out, "stop file") && !strings.Contains(out, "No work") && !strings.Contains(out, "self-healing") {
		t.Errorf("expected daemon to handle empty tree gracefully, got: %s", out)
	}
}

func TestDaemon_SelfHealing(t *testing.T) {
	dir := t.TempDir()
	run(t, dir, "init")

	scriptPath := createMockModel(t, dir, "complete", "complete")
	configureMockModels(t, dir, scriptPath)

	run(t, dir, "project", "create", "heal-test")
	run(t, dir, "task", "add", "--node", "heal-test", "interrupted task")

	// Manually set task to in_progress (simulating a crash during execution)
	ns := loadNode(t, dir, "heal-test")
	for i := range ns.Tasks {
		if ns.Tasks[i].ID == "task-1" {
			ns.Tasks[i].State = state.StatusInProgress
		}
	}
	saveNode(t, dir, "heal-test", ns)

	setMaxIterations(t, dir, 10)
	run(t, dir, "start")

	// Task should be completed after self-healing picks it up
	ns = loadNode(t, dir, "heal-test")
	for _, task := range ns.Tasks {
		if task.ID == "task-1" {
			if task.State != state.StatusComplete {
				t.Errorf("expected task-1 complete after self-healing, got %s", task.State)
			}
			return
		}
	}
	t.Error("task-1 not found")
}

func TestDaemon_MaxIterations(t *testing.T) {
	dir := t.TempDir()
	run(t, dir, "init")

	// Use a yield mock so the daemon keeps re-entering the loop.
	// Each yield is a work iteration, counting toward max_iterations.
	scriptPath := createMockModel(t, dir, "yield", "yield")
	configureMockModels(t, dir, scriptPath)

	run(t, dir, "project", "create", "maxiter-test")
	run(t, dir, "task", "add", "--node", "maxiter-test", "infinite yielder")

	setMaxIterations(t, dir, 3)
	out := run(t, dir, "start")

	if !strings.Contains(out, "iteration cap") {
		t.Errorf("expected daemon to stop at iteration cap, got: %s", out)
	}
}

// setFailureAndIterationConfig merges both failure thresholds and daemon
// max_iterations into config.local.json, preserving identity and other fields.
func setFailureAndIterationConfig(t *testing.T, dir string, decompositionThreshold, maxDepth, hardCap, maxIterations int) {
	t.Helper()
	mergeLocalConfig(t, dir, map[string]any{
		"daemon": map[string]any{
			"max_iterations": maxIterations,
		},
		"failure": map[string]any{
			"decomposition_threshold": decompositionThreshold,
			"max_decomposition_depth": maxDepth,
			"hard_cap":                hardCap,
		},
	})
}
