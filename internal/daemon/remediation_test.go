package daemon

import (
	"context"
	"path/filepath"
	"testing"

	"github.com/dorkusprime/wolfcastle/internal/config"
	"github.com/dorkusprime/wolfcastle/internal/state"
)

// ═══════════════════════════════════════════════════════════════════════════
// SKIP bypasses progress check
// ═══════════════════════════════════════════════════════════════════════════

func TestRunIteration_SkipBypassesProgressCheck(t *testing.T) {
	t.Parallel()
	d := testDaemon(t)

	// Git repo with no file changes: normally COMPLETE would be rejected,
	// but SKIP should bypass the progress check entirely.
	initTestGitRepo(t, d.RepoDir)

	d.Config.Models["skip-echo"] = config.ModelDef{
		Command: "echo",
		Args:    []string{"WOLFCASTLE_SKIP already done"},
	}
	d.Config.Pipeline.Stages = []config.PipelineStage{
		{Name: "execute", Model: "skip-echo", PromptFile: "execute.md"},
	}
	d.Config.Retries.MaxRetries = 0
	_ = d.Logger.StartIteration()
	defer d.Logger.Close()

	setupLeafNode(t, d, "skip-node", []state.Task{
		{ID: "task-0001", Description: "already resolved elsewhere", State: state.StatusInProgress},
	})
	writePromptFile(t, d.WolfcastleDir, "execute.md")

	idx, _ := d.Store.ReadIndex()
	nav := &state.NavigationResult{NodeAddress: "skip-node", TaskID: "task-0001", Found: true}
	err := d.runIteration(context.Background(), nav, idx)
	if err != nil {
		t.Fatalf("runIteration error: %v", err)
	}

	ns, _ := d.Store.ReadNode("skip-node")
	for _, task := range ns.Tasks {
		if task.ID == "task-0001" {
			if task.State != state.StatusComplete {
				t.Errorf("expected complete after SKIP, got %s", task.State)
			}
			if task.FailureCount != 0 {
				t.Errorf("expected 0 failures after SKIP, got %d", task.FailureCount)
			}
			return
		}
	}
	t.Error("task-0001 not found")
}

// ═══════════════════════════════════════════════════════════════════════════
// Audit tasks skip the git progress check
// ═══════════════════════════════════════════════════════════════════════════

func TestRunIteration_AuditSkipsProgressCheck(t *testing.T) {
	t.Parallel()
	d := testDaemon(t)

	// Git repo with no code changes: audit tasks should still complete,
	// since their work lives in .wolfcastle/system/ state mutations.
	initTestGitRepo(t, d.RepoDir)

	d.Config.Models["echo"] = config.ModelDef{
		Command: "echo",
		Args:    []string{"WOLFCASTLE_COMPLETE"},
	}
	d.Config.Pipeline.Stages = []config.PipelineStage{
		{Name: "execute", Model: "echo", PromptFile: "execute.md"},
	}
	d.Config.Retries.MaxRetries = 0
	_ = d.Logger.StartIteration()
	defer d.Logger.Close()

	setupLeafNode(t, d, "audit-node", []state.Task{
		{ID: "audit-0001", Description: "audit the node", State: state.StatusInProgress, IsAudit: true},
	})
	writePromptFile(t, d.WolfcastleDir, "execute.md")

	idx, _ := d.Store.ReadIndex()
	nav := &state.NavigationResult{NodeAddress: "audit-node", TaskID: "audit-0001", Found: true}
	err := d.runIteration(context.Background(), nav, idx)
	if err != nil {
		t.Fatalf("runIteration error: %v", err)
	}

	ns, _ := d.Store.ReadNode("audit-node")
	for _, task := range ns.Tasks {
		if task.ID == "audit-0001" {
			if task.State != state.StatusComplete {
				t.Errorf("audit task should complete despite no git changes, got %s", task.State)
			}
			return
		}
	}
	t.Error("audit-0001 not found")
}

// ═══════════════════════════════════════════════════════════════════════════
// Missing deliverables warn but don't block completion
// ═══════════════════════════════════════════════════════════════════════════

func TestRunIteration_MissingDeliverables_WarnsButCompletes(t *testing.T) {
	t.Parallel()
	d := testDaemon(t)

	// No git repo: progress check assumes true when git is unavailable,
	// so we isolate the deliverables behavior.
	d.Config.Models["echo"] = config.ModelDef{
		Command: "echo",
		Args:    []string{"WOLFCASTLE_COMPLETE"},
	}
	d.Config.Pipeline.Stages = []config.PipelineStage{
		{Name: "execute", Model: "echo", PromptFile: "execute.md"},
	}
	d.Config.Retries.MaxRetries = 0
	_ = d.Logger.StartIteration()
	defer d.Logger.Close()

	projDir := d.Store.Dir()
	ns := state.NewNodeState("deliv-node", "deliv-node", state.NodeLeaf)
	ns.Tasks = []state.Task{
		{
			ID:           "task-0001",
			Description:  "write nonexistent.go",
			State:        state.StatusNotStarted,
			Deliverables: []string{"nonexistent.go"},
		},
	}
	idx := state.NewRootIndex()
	idx.Root = []string{"deliv-node"}
	idx.Nodes["deliv-node"] = state.IndexEntry{
		Name: "deliv-node", Type: state.NodeLeaf, State: state.StatusNotStarted, Address: "deliv-node",
	}
	writeJSON(t, filepath.Join(d.Store.Dir(), "state.json"), idx)
	writeJSON(t, filepath.Join(projDir, "deliv-node", "state.json"), ns)
	writePromptFile(t, d.WolfcastleDir, "execute.md")

	idx2, _ := d.Store.ReadIndex()
	nav := &state.NavigationResult{NodeAddress: "deliv-node", TaskID: "task-0001", Found: true}
	err := d.runIteration(context.Background(), nav, idx2)
	if err != nil {
		t.Fatalf("runIteration error: %v", err)
	}

	reloaded, _ := d.Store.ReadNode("deliv-node")
	for _, task := range reloaded.Tasks {
		if task.ID == "task-0001" {
			if task.State != state.StatusComplete {
				t.Errorf("task should complete despite missing deliverables, got %s", task.State)
			}
			if task.FailureCount != 0 {
				t.Errorf("expected 0 failures, got %d", task.FailureCount)
			}
			return
		}
	}
	t.Error("task-0001 not found")
}

// ═══════════════════════════════════════════════════════════════════════════
// findNewTasks — detects new non-audit tasks between snapshots
// ═══════════════════════════════════════════════════════════════════════════

func TestFindNewTasks(t *testing.T) {
	t.Parallel()

	before := &state.NodeState{
		Tasks: []state.Task{
			{ID: "task-0001", Description: "original"},
		},
	}
	after := &state.NodeState{
		Tasks: []state.Task{
			{ID: "task-0001", Description: "original"},
			{ID: "task-0002", Description: "new subtask"},
			{ID: "task-0003", Description: "another subtask"},
			{ID: "audit-0001", Description: "audit", IsAudit: true},
		},
	}

	got := findNewTasks(before, after)

	if len(got) != 2 {
		t.Fatalf("expected 2 new tasks, got %d: %v", len(got), got)
	}

	// Verify the expected IDs appear (order preserved from after.Tasks)
	expected := map[string]bool{"task-0002": true, "task-0003": true}
	for _, id := range got {
		if !expected[id] {
			t.Errorf("unexpected task ID in result: %s", id)
		}
	}

	// Audit task must be excluded
	for _, id := range got {
		if id == "audit-0001" {
			t.Error("audit tasks should be excluded from findNewTasks")
		}
	}
}

func TestFindNewTasks_NoNewTasks(t *testing.T) {
	t.Parallel()

	ns := &state.NodeState{
		Tasks: []state.Task{
			{ID: "task-0001", Description: "same"},
		},
	}

	got := findNewTasks(ns, ns)
	if len(got) != 0 {
		t.Errorf("expected 0 new tasks, got %d: %v", len(got), got)
	}
}

// ═══════════════════════════════════════════════════════════════════════════
// autoCompleteDecomposedParents — blocked parent auto-completes
// ═══════════════════════════════════════════════════════════════════════════

func TestAutoCompleteDecomposedParents(t *testing.T) {
	t.Parallel()
	d := testDaemon(t)
	_ = d.Logger.StartIteration()
	defer d.Logger.Close()

	setupLeafNode(t, d, "decomp-node", []state.Task{
		{
			ID:            "task-0001",
			Description:   "parent task",
			State:         state.StatusBlocked,
			BlockedReason: "decomposed into subtasks: task-0002, task-0003",
		},
		{
			ID:          "task-0002",
			Description: "subtask A",
			State:       state.StatusComplete,
		},
		{
			ID:          "task-0003",
			Description: "subtask B",
			State:       state.StatusComplete,
		},
	})

	d.autoCompleteDecomposedParents("decomp-node")

	ns, _ := d.Store.ReadNode("decomp-node")
	for _, task := range ns.Tasks {
		if task.ID == "task-0001" {
			if task.State != state.StatusComplete {
				t.Errorf("parent should be auto-completed when all subtasks are complete, got %s", task.State)
			}
			return
		}
	}
	t.Error("task-0001 not found")
}

func TestAutoCompleteDecomposedParents_IncompleteSubtask(t *testing.T) {
	t.Parallel()
	d := testDaemon(t)
	_ = d.Logger.StartIteration()
	defer d.Logger.Close()

	setupLeafNode(t, d, "partial-node", []state.Task{
		{
			ID:            "task-0001",
			Description:   "parent task",
			State:         state.StatusBlocked,
			BlockedReason: "decomposed into subtasks: task-0002, task-0003",
		},
		{
			ID:          "task-0002",
			Description: "subtask A",
			State:       state.StatusComplete,
		},
		{
			ID:          "task-0003",
			Description: "subtask B",
			State:       state.StatusInProgress,
		},
	})

	d.autoCompleteDecomposedParents("partial-node")

	ns, _ := d.Store.ReadNode("partial-node")
	for _, task := range ns.Tasks {
		if task.ID == "task-0001" {
			if task.State != state.StatusBlocked {
				t.Errorf("parent should remain blocked when subtasks are incomplete, got %s", task.State)
			}
			return
		}
	}
	t.Error("task-0001 not found")
}
