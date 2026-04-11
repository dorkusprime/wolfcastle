//go:build acceptance

// Package acceptance contains end-to-end tests that exercise the TUI through
// a real tea.Program running headless via teatest. These tests verify that
// keystrokes produce visible effects, screens render expected content, and
// navigation flows work from a user's perspective.
package acceptance

import (
	"bytes"
	"os"
	"path/filepath"
	"testing"
	"time"

	teatest "github.com/charmbracelet/x/exp/teatest/v2"

	"github.com/dorkusprime/wolfcastle/internal/state"
	"github.com/dorkusprime/wolfcastle/internal/testutil"
	"github.com/dorkusprime/wolfcastle/internal/tui/app"
)

const (
	termWidth  = 120
	termHeight = 60
	waitTime   = 3 * time.Second
)

// newWelcomeTUI creates a TUIModel that will enter welcome state (no .wolfcastle/).
func newWelcomeTUI(t *testing.T) *teatest.TestModel {
	t.Helper()
	dir := t.TempDir()
	m := app.NewTUIModel(nil, nil, dir, "test")
	return teatest.NewTestModel(t, m, teatest.WithInitialTermSize(termWidth, termHeight))
}

// newColdTUI creates a TUIModel with a scaffolded project but no daemon.
func newColdTUI(t *testing.T) *teatest.TestModel {
	t.Helper()
	env := testutil.NewEnvironment(t)
	repoDir := filepath.Dir(env.Root) // parent of .wolfcastle/
	m := app.NewTUIModel(env.State, env.Daemon, repoDir, "test")
	return teatest.NewTestModel(t, m, teatest.WithInitialTermSize(termWidth, termHeight))
}

// newColdTUIWithTree creates a TUIModel with a populated tree.
func newColdTUIWithTree(t *testing.T) *teatest.TestModel {
	t.Helper()
	env := testutil.NewEnvironment(t)
	populateTree(t, env)
	repoDir := filepath.Dir(env.Root)
	m := app.NewTUIModel(env.State, env.Daemon, repoDir, "test")
	return teatest.NewTestModel(t, m, teatest.WithInitialTermSize(termWidth, termHeight))
}

// populateTree writes a small tree into the environment: one orchestrator
// with two leaves, one complete and one not started.
func populateTree(t *testing.T, env *testutil.Environment) {
	t.Helper()

	err := env.State.MutateIndex(func(idx *state.RootIndex) error {
		idx.Root = []string{"demo-project"}
		idx.Nodes["demo-project"] = state.IndexEntry{
			Name:     "demo-project",
			Type:     state.NodeOrchestrator,
			State:    state.StatusInProgress,
			Address:  "demo-project",
			Children: []string{"demo-project/backend", "demo-project/frontend"},
		}
		idx.Nodes["demo-project/backend"] = state.IndexEntry{
			Name:    "backend",
			Type:    state.NodeLeaf,
			State:   state.StatusComplete,
			Address: "demo-project/backend",
			Parent:  "demo-project",
		}
		idx.Nodes["demo-project/frontend"] = state.IndexEntry{
			Name:    "frontend",
			Type:    state.NodeLeaf,
			State:   state.StatusNotStarted,
			Address: "demo-project/frontend",
			Parent:  "demo-project",
		}
		return nil
	})
	if err != nil {
		t.Fatalf("populate index: %v", err)
	}

	nodes := map[string]*state.NodeState{
		"demo-project": {
			Name:  "demo-project",
			Type:  state.NodeOrchestrator,
			State: state.StatusInProgress,
		},
		"demo-project/backend": {
			Name:  "backend",
			Type:  state.NodeLeaf,
			State: state.StatusComplete,
			Tasks: []state.Task{
				{ID: "task-1", Title: "Build the API", State: state.StatusComplete},
				{ID: "audit", Title: "Audit", State: state.StatusComplete, IsAudit: true},
			},
		},
		"demo-project/frontend": {
			Name:  "frontend",
			Type:  state.NodeLeaf,
			State: state.StatusNotStarted,
			Tasks: []state.Task{
				{ID: "task-1", Title: "Build the UI", State: state.StatusNotStarted},
				{ID: "audit", Title: "Audit", State: state.StatusNotStarted, IsAudit: true},
			},
		},
	}

	for addr, ns := range nodes {
		p, pathErr := env.State.NodePath(addr)
		if pathErr != nil {
			t.Fatalf("node path %s: %v", addr, pathErr)
		}
		if mkErr := os.MkdirAll(filepath.Dir(p), 0o755); mkErr != nil {
			t.Fatalf("mkdir %s: %v", addr, mkErr)
		}
		if saveErr := state.SaveNodeState(p, ns); saveErr != nil {
			t.Fatalf("save node %s: %v", addr, saveErr)
		}
	}
}

// newColdTUIAllComplete creates a TUIModel where every node is complete.
func newColdTUIAllComplete(t *testing.T) *teatest.TestModel {
	t.Helper()
	env := testutil.NewEnvironment(t)
	populateAllComplete(t, env)
	repoDir := filepath.Dir(env.Root)
	m := app.NewTUIModel(env.State, env.Daemon, repoDir, "test")
	return teatest.NewTestModel(t, m, teatest.WithInitialTermSize(termWidth, termHeight))
}

// newColdTUIAllBlocked creates a TUIModel where every node is blocked.
func newColdTUIAllBlocked(t *testing.T) *teatest.TestModel {
	t.Helper()
	env := testutil.NewEnvironment(t)
	populateAllBlocked(t, env)
	repoDir := filepath.Dir(env.Root)
	m := app.NewTUIModel(env.State, env.Daemon, repoDir, "test")
	return teatest.NewTestModel(t, m, teatest.WithInitialTermSize(termWidth, termHeight))
}

// newColdTUIWithMixedStates creates a TUIModel with complete, in_progress,
// not_started, and blocked nodes.
func newColdTUIWithMixedStates(t *testing.T) *teatest.TestModel {
	t.Helper()
	env := testutil.NewEnvironment(t)
	populateMixedTree(t, env)
	repoDir := filepath.Dir(env.Root)
	m := app.NewTUIModel(env.State, env.Daemon, repoDir, "test")
	return teatest.NewTestModel(t, m, teatest.WithInitialTermSize(termWidth, termHeight))
}

// populateAllComplete writes a tree where every node and task is complete.
func populateAllComplete(t *testing.T, env *testutil.Environment) {
	t.Helper()

	err := env.State.MutateIndex(func(idx *state.RootIndex) error {
		idx.Root = []string{"done-project"}
		idx.Nodes["done-project"] = state.IndexEntry{
			Name:     "done-project",
			Type:     state.NodeOrchestrator,
			State:    state.StatusComplete,
			Address:  "done-project",
			Children: []string{"done-project/alpha", "done-project/bravo"},
		}
		idx.Nodes["done-project/alpha"] = state.IndexEntry{
			Name:    "alpha",
			Type:    state.NodeLeaf,
			State:   state.StatusComplete,
			Address: "done-project/alpha",
			Parent:  "done-project",
		}
		idx.Nodes["done-project/bravo"] = state.IndexEntry{
			Name:    "bravo",
			Type:    state.NodeLeaf,
			State:   state.StatusComplete,
			Address: "done-project/bravo",
			Parent:  "done-project",
		}
		return nil
	})
	if err != nil {
		t.Fatalf("populate all-complete index: %v", err)
	}

	nodes := map[string]*state.NodeState{
		"done-project": {
			Name:  "done-project",
			Type:  state.NodeOrchestrator,
			State: state.StatusComplete,
		},
		"done-project/alpha": {
			Name:  "alpha",
			Type:  state.NodeLeaf,
			State: state.StatusComplete,
			Tasks: []state.Task{
				{ID: "task-1", Title: "Task Alpha", State: state.StatusComplete},
			},
		},
		"done-project/bravo": {
			Name:  "bravo",
			Type:  state.NodeLeaf,
			State: state.StatusComplete,
			Tasks: []state.Task{
				{ID: "task-1", Title: "Task Bravo", State: state.StatusComplete},
			},
		},
	}

	saveNodes(t, env, nodes)
}

// populateAllBlocked writes a tree where every node is blocked.
func populateAllBlocked(t *testing.T, env *testutil.Environment) {
	t.Helper()

	err := env.State.MutateIndex(func(idx *state.RootIndex) error {
		idx.Root = []string{"stuck-project"}
		idx.Nodes["stuck-project"] = state.IndexEntry{
			Name:     "stuck-project",
			Type:     state.NodeOrchestrator,
			State:    state.StatusBlocked,
			Address:  "stuck-project",
			Children: []string{"stuck-project/alpha"},
		}
		idx.Nodes["stuck-project/alpha"] = state.IndexEntry{
			Name:    "alpha",
			Type:    state.NodeLeaf,
			State:   state.StatusBlocked,
			Address: "stuck-project/alpha",
			Parent:  "stuck-project",
		}
		return nil
	})
	if err != nil {
		t.Fatalf("populate all-blocked index: %v", err)
	}

	nodes := map[string]*state.NodeState{
		"stuck-project": {
			Name:  "stuck-project",
			Type:  state.NodeOrchestrator,
			State: state.StatusBlocked,
		},
		"stuck-project/alpha": {
			Name:  "alpha",
			Type:  state.NodeLeaf,
			State: state.StatusBlocked,
			Tasks: []state.Task{
				{ID: "task-1", Title: "Blocked task", State: state.StatusBlocked},
			},
		},
	}

	saveNodes(t, env, nodes)
}

// populateMixedTree writes a tree with one node in each status.
func populateMixedTree(t *testing.T, env *testutil.Environment) {
	t.Helper()

	err := env.State.MutateIndex(func(idx *state.RootIndex) error {
		idx.Root = []string{"mixed-project"}
		idx.Nodes["mixed-project"] = state.IndexEntry{
			Name:    "mixed-project",
			Type:    state.NodeOrchestrator,
			State:   state.StatusInProgress,
			Address: "mixed-project",
			Children: []string{
				"mixed-project/done",
				"mixed-project/wip",
				"mixed-project/todo",
				"mixed-project/stuck",
			},
		}
		idx.Nodes["mixed-project/done"] = state.IndexEntry{
			Name: "done", Type: state.NodeLeaf, State: state.StatusComplete,
			Address: "mixed-project/done", Parent: "mixed-project",
		}
		idx.Nodes["mixed-project/wip"] = state.IndexEntry{
			Name: "wip", Type: state.NodeLeaf, State: state.StatusInProgress,
			Address: "mixed-project/wip", Parent: "mixed-project",
		}
		idx.Nodes["mixed-project/todo"] = state.IndexEntry{
			Name: "todo", Type: state.NodeLeaf, State: state.StatusNotStarted,
			Address: "mixed-project/todo", Parent: "mixed-project",
		}
		idx.Nodes["mixed-project/stuck"] = state.IndexEntry{
			Name: "stuck", Type: state.NodeLeaf, State: state.StatusBlocked,
			Address: "mixed-project/stuck", Parent: "mixed-project",
		}
		return nil
	})
	if err != nil {
		t.Fatalf("populate mixed index: %v", err)
	}

	nodes := map[string]*state.NodeState{
		"mixed-project": {
			Name:  "mixed-project",
			Type:  state.NodeOrchestrator,
			State: state.StatusInProgress,
		},
		"mixed-project/done": {
			Name: "done", Type: state.NodeLeaf, State: state.StatusComplete,
			Tasks: []state.Task{{ID: "task-1", Title: "Finished", State: state.StatusComplete}},
		},
		"mixed-project/wip": {
			Name: "wip", Type: state.NodeLeaf, State: state.StatusInProgress,
			Tasks: []state.Task{{ID: "task-1", Title: "Working", State: state.StatusInProgress}},
		},
		"mixed-project/todo": {
			Name: "todo", Type: state.NodeLeaf, State: state.StatusNotStarted,
			Tasks: []state.Task{{ID: "task-1", Title: "Pending", State: state.StatusNotStarted}},
		},
		"mixed-project/stuck": {
			Name: "stuck", Type: state.NodeLeaf, State: state.StatusBlocked,
			Tasks: []state.Task{{ID: "task-1", Title: "Stuck", State: state.StatusBlocked}},
		},
	}

	saveNodes(t, env, nodes)
}

// saveNodes persists a map of node states to the environment's store.
func saveNodes(t *testing.T, env *testutil.Environment, nodes map[string]*state.NodeState) {
	t.Helper()
	for addr, ns := range nodes {
		p, pathErr := env.State.NodePath(addr)
		if pathErr != nil {
			t.Fatalf("node path %s: %v", addr, pathErr)
		}
		if mkErr := os.MkdirAll(filepath.Dir(p), 0o755); mkErr != nil {
			t.Fatalf("mkdir %s: %v", addr, mkErr)
		}
		if saveErr := state.SaveNodeState(p, ns); saveErr != nil {
			t.Fatalf("save node %s: %v", addr, saveErr)
		}
	}
}

// contains returns a WaitFor condition that checks for a substring.
func contains(s string) func([]byte) bool {
	return func(b []byte) bool {
		return bytes.Contains(b, []byte(s))
	}
}

// containsAll returns true if every provided substring is present in b.
func containsAll(b []byte, subs ...string) bool {
	for _, s := range subs {
		if !bytes.Contains(b, []byte(s)) {
			return false
		}
	}
	return true
}

// quit sends q and waits for the program to finish.
func quit(t *testing.T, tm *teatest.TestModel) {
	t.Helper()
	tm.Type("q")
	tm.WaitFinished(t, teatest.WithFinalTimeout(waitTime))
}
