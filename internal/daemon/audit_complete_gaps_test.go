package daemon

import (
	"context"
	"path/filepath"
	"testing"
	"time"

	"github.com/dorkusprime/wolfcastle/internal/config"
	"github.com/dorkusprime/wolfcastle/internal/state"
)

// setupLeafNodeWithGaps creates a leaf node with the given tasks and audit gaps.
func setupLeafNodeWithGaps(t *testing.T, d *Daemon, nodeAddr string, tasks []state.Task, gaps []state.Gap) {
	t.Helper()
	projDir := d.Store.Dir()
	idx := state.NewRootIndex()
	idx.Root = []string{nodeAddr}
	idx.Nodes[nodeAddr] = state.IndexEntry{
		Name:    nodeAddr,
		Type:    state.NodeLeaf,
		State:   state.StatusNotStarted,
		Address: nodeAddr,
	}
	writeJSON(t, filepath.Join(d.Store.Dir(), "state.json"), idx)

	ns := state.NewNodeState(nodeAddr, nodeAddr, state.NodeLeaf)
	ns.Tasks = tasks
	ns.Audit.Gaps = gaps
	statePath := filepath.Join(projDir, nodeAddr, "state.json")
	writeJSON(t, statePath, ns)
}

// ═══════════════════════════════════════════════════════════════════════════
// Completing an audit task with open gaps creates remediation subtasks
// ═══════════════════════════════════════════════════════════════════════════

func TestRunIteration_AuditCompleteWithOpenGaps_CreatesRemediation(t *testing.T) {
	t.Parallel()
	d := testDaemon(t)

	initTestGitRepo(t, d.RepoDir)

	d.Config.Models["echo"] = config.ModelDef{
		Command: "echo",
		Args:    []string{"WOLFCASTLE_COMPLETE"},
	}
	d.Config.Pipeline.Stages = map[string]config.PipelineStage{
		"execute": {Model: "echo", PromptFile: "stages/execute.md"},
	}
	d.Config.Pipeline.StageOrder = []string{"execute"}
	d.Config.Retries.MaxRetries = 0
	_ = d.Logger.StartIteration()
	defer d.Logger.Close()

	gaps := []state.Gap{
		{ID: "gap-1", Description: "missing error handling", Status: state.GapOpen, Timestamp: time.Now()},
		{ID: "gap-2", Description: "no test coverage", Status: state.GapOpen, Timestamp: time.Now()},
	}
	setupLeafNodeWithGaps(t, d, "audit-gaps-node", []state.Task{
		{ID: "audit-0001", Description: "audit the node", State: state.StatusInProgress, IsAudit: true},
	}, gaps)
	writePromptFile(t, d.WolfcastleDir, "stages/execute.md")

	idx, _ := d.Store.ReadIndex()
	nav := &state.NavigationResult{NodeAddress: "audit-gaps-node", TaskID: "audit-0001", Found: true}
	err := d.runIteration(context.Background(), nav, idx)
	if err != nil {
		t.Fatalf("runIteration error: %v", err)
	}

	ns, _ := d.Store.ReadNode("audit-gaps-node")

	// The audit task should NOT be complete.
	for _, task := range ns.Tasks {
		if task.ID == "audit-0001" {
			if task.State == state.StatusComplete {
				t.Fatal("audit task should not complete when open gaps exist")
			}
			break
		}
	}

	// Remediation subtasks should have been created (one per open gap).
	subtaskCount := 0
	for _, task := range ns.Tasks {
		if task.ID != "audit-0001" {
			subtaskCount++
		}
	}
	if subtaskCount != 2 {
		t.Errorf("expected 2 remediation subtasks, got %d", subtaskCount)
	}
}

// ═══════════════════════════════════════════════════════════════════════════
// Completing an audit task with no open gaps succeeds normally
// ═══════════════════════════════════════════════════════════════════════════

func TestRunIteration_AuditCompleteNoOpenGaps_Succeeds(t *testing.T) {
	t.Parallel()
	d := testDaemon(t)

	initTestGitRepo(t, d.RepoDir)

	d.Config.Models["echo"] = config.ModelDef{
		Command: "echo",
		Args:    []string{"WOLFCASTLE_COMPLETE"},
	}
	d.Config.Pipeline.Stages = map[string]config.PipelineStage{
		"execute": {Model: "echo", PromptFile: "stages/execute.md"},
	}
	d.Config.Pipeline.StageOrder = []string{"execute"}
	d.Config.Retries.MaxRetries = 0
	_ = d.Logger.StartIteration()
	defer d.Logger.Close()

	// All gaps are fixed, so completion should proceed.
	gaps := []state.Gap{
		{ID: "gap-1", Description: "was missing error handling", Status: state.GapFixed, Timestamp: time.Now()},
	}
	setupLeafNodeWithGaps(t, d, "audit-clean-node", []state.Task{
		{ID: "audit-0001", Description: "audit the node", State: state.StatusInProgress, IsAudit: true},
	}, gaps)
	writePromptFile(t, d.WolfcastleDir, "stages/execute.md")

	idx, _ := d.Store.ReadIndex()
	nav := &state.NavigationResult{NodeAddress: "audit-clean-node", TaskID: "audit-0001", Found: true}
	err := d.runIteration(context.Background(), nav, idx)
	if err != nil {
		t.Fatalf("runIteration error: %v", err)
	}

	ns, _ := d.Store.ReadNode("audit-clean-node")
	for _, task := range ns.Tasks {
		if task.ID == "audit-0001" {
			if task.State != state.StatusComplete {
				t.Errorf("audit task with no open gaps should complete, got %s", task.State)
			}
			return
		}
	}
	t.Error("audit-0001 not found")
}

// ═══════════════════════════════════════════════════════════════════════════
// Completing a non-audit task with open gaps still succeeds
// ═══════════════════════════════════════════════════════════════════════════

func TestRunIteration_NonAuditCompleteWithOpenGaps_Succeeds(t *testing.T) {
	t.Parallel()
	d := testDaemon(t)

	// No git repo: checkGitProgress assumes true when git is unavailable,
	// isolating the open-gaps behavior from the progress gate.
	d.Config.Models["echo"] = config.ModelDef{
		Command: "echo",
		Args:    []string{"WOLFCASTLE_COMPLETE"},
	}
	d.Config.Pipeline.Stages = map[string]config.PipelineStage{
		"execute": {Model: "echo", PromptFile: "stages/execute.md"},
	}
	d.Config.Pipeline.StageOrder = []string{"execute"}
	d.Config.Retries.MaxRetries = 0
	_ = d.Logger.StartIteration()
	defer d.Logger.Close()

	// Open gaps exist on the node, but the completing task is NOT an audit.
	gaps := []state.Gap{
		{ID: "gap-1", Description: "something open", Status: state.GapOpen, Timestamp: time.Now()},
	}
	setupLeafNodeWithGaps(t, d, "noaudit-gaps-node", []state.Task{
		{ID: "task-0001", Description: "implement feature", State: state.StatusInProgress},
	}, gaps)
	writePromptFile(t, d.WolfcastleDir, "stages/execute.md")

	idx, _ := d.Store.ReadIndex()
	nav := &state.NavigationResult{NodeAddress: "noaudit-gaps-node", TaskID: "task-0001", Found: true}
	err := d.runIteration(context.Background(), nav, idx)
	if err != nil {
		t.Fatalf("runIteration error: %v", err)
	}

	ns, _ := d.Store.ReadNode("noaudit-gaps-node")
	for _, task := range ns.Tasks {
		if task.ID == "task-0001" {
			if task.State != state.StatusComplete {
				t.Errorf("non-audit task should complete regardless of open gaps, got %s", task.State)
			}
			return
		}
	}
	t.Error("task-0001 not found")
}
