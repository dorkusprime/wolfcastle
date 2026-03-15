package pipeline

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/dorkusprime/wolfcastle/internal/state"
)

// ═══════════════════════════════════════════════════════════════════════════
// BuildIterationContextFull — the nodeDir-aware variant
// ═══════════════════════════════════════════════════════════════════════════

func TestBuildIterationContextFull_WithTaskMD(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	nodeDir := filepath.Join(dir, "node")
	_ = os.MkdirAll(nodeDir, 0755)

	// Write a task .md file
	_ = os.WriteFile(filepath.Join(nodeDir, "task-0001.md"), []byte("# Task 1\nDo something."), 0644)

	ns := state.NewNodeState("my-node", "My Node", state.NodeLeaf)
	ns.Tasks = []state.Task{
		{ID: "task-0001", Description: "do work", State: state.StatusNotStarted},
	}

	result := BuildIterationContextFull("", nodeDir, "my-node", ns, "task-0001")
	if !strings.Contains(result, "**Node:** my-node") {
		t.Error("expected node address in output")
	}
	if !strings.Contains(result, "Do something.") {
		t.Error("expected task .md content in output")
	}
}

func TestBuildIterationContextFull_NoTaskMD(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()

	ns := state.NewNodeState("my-node", "My Node", state.NodeLeaf)
	ns.Tasks = []state.Task{
		{ID: "task-0001", Description: "do work", State: state.StatusNotStarted},
	}

	result := BuildIterationContextFull("", dir, "my-node", ns, "task-0001")
	if !strings.Contains(result, "**Task:** my-node/task-0001") {
		t.Error("expected task reference in output")
	}
}

func TestBuildIterationContextFull_WithDeliverables(t *testing.T) {
	t.Parallel()
	ns := state.NewNodeState("my-node", "My Node", state.NodeLeaf)
	ns.Tasks = []state.Task{
		{
			ID:           "task-0001",
			Description:  "generate report",
			State:        state.StatusNotStarted,
			Deliverables: []string{"report.txt", "summary.md"},
		},
	}

	result := BuildIterationContextFull("", "", "my-node", ns, "task-0001")
	if !strings.Contains(result, "report.txt") {
		t.Error("expected deliverables in output")
	}
}
