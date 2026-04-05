package audit

import (
	"os"
	"path/filepath"
	"runtime"
	"testing"

	"github.com/dorkusprime/wolfcastle/internal/state"
)

// ── approve: filesystem error paths via chmod ──────────────────────

func TestApprove_MkdirAllError_ReadOnly(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("permission test not reliable on Windows")
	}

	env := newTestEnv(t)

	batch := &state.Batch{
		ID:     "audit-perm",
		Status: state.BatchPending,
		Scopes: []string{"test"},
		Findings: []state.Finding{
			{ID: "f-1", Title: "Valid Finding", Status: state.FindingPending},
		},
	}
	_ = state.SaveBatch(filepath.Join(env.WolfcastleDir, "audit-state.json"), batch)

	// Lock the projects dir so MkdirAll for the new node fails.
	_ = os.Chmod(env.ProjectsDir, 0555)
	t.Cleanup(func() { _ = os.Chmod(env.ProjectsDir, 0755) })

	env.RootCmd.SetArgs([]string{"audit", "approve", "f-1"})
	err := env.RootCmd.Execute()
	// The approve loop catches the mkdir error via PrintHuman and skips,
	// then reports "no pending findings" because nothing was approved.
	if err == nil {
		t.Error("expected error (no findings could be approved)")
	}
}

func TestApprove_SaveNodeStateError_ReadOnly(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("permission test not reliable on Windows")
	}

	env := newTestEnv(t)

	batch := &state.Batch{
		ID:     "audit-save",
		Status: state.BatchPending,
		Scopes: []string{"test"},
		Findings: []state.Finding{
			{ID: "f-1", Title: "Good Finding", Status: state.FindingPending},
		},
	}
	_ = state.SaveBatch(filepath.Join(env.WolfcastleDir, "audit-state.json"), batch)

	// Pre-create the node directory, then lock it so SaveNodeState fails.
	nodeDir := filepath.Join(env.ProjectsDir, "good-finding")
	_ = os.MkdirAll(nodeDir, 0755)
	_ = os.Chmod(nodeDir, 0555)
	t.Cleanup(func() { _ = os.Chmod(nodeDir, 0755) })

	env.RootCmd.SetArgs([]string{"audit", "approve", "f-1"})
	err := env.RootCmd.Execute()
	if err == nil {
		t.Error("expected error when SaveNodeState fails")
	}
}

func TestApprove_WriteFileError_ReadOnly(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("permission test not reliable on Windows")
	}

	env := newTestEnv(t)

	batch := &state.Batch{
		ID:     "audit-wf",
		Status: state.BatchPending,
		Scopes: []string{"test"},
		Findings: []state.Finding{
			{ID: "f-1", Title: "Write Test", Description: "desc", Status: state.FindingPending},
		},
	}
	_ = state.SaveBatch(filepath.Join(env.WolfcastleDir, "audit-state.json"), batch)

	// Lock the projects dir so description markdown write fails.
	// The approve code continues despite WriteFile failure (it's a warning),
	// so we also need SaveBatch/SaveRootIndex to work. Lock only the
	// place where the .md file is written: the projects dir root.
	_ = os.Chmod(env.ProjectsDir, 0555)
	t.Cleanup(func() { _ = os.Chmod(env.ProjectsDir, 0755) })

	env.RootCmd.SetArgs([]string{"audit", "approve", "f-1"})
	// MkdirAll for node dir also fails. Finding is skipped.
	err := env.RootCmd.Execute()
	if err == nil {
		t.Error("expected error since no finding was successfully approved")
	}
}

func TestApprove_SaveBatchError_ReadOnly(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("permission test not reliable on Windows")
	}

	env := newTestEnv(t)

	batch := &state.Batch{
		ID:     "audit-sb",
		Status: state.BatchPending,
		Scopes: []string{"test"},
		Findings: []state.Finding{
			// Use a title that maps to an existing node so it skips creation.
			{ID: "f-1", Title: "Existing Node", Status: state.FindingPending},
		},
	}
	batchPath := filepath.Join(env.WolfcastleDir, "audit-state.json")
	_ = state.SaveBatch(batchPath, batch)

	// Create a node so the "project already exists" path is used
	// (approve without creating).
	idx, _ := state.LoadRootIndex(filepath.Join(env.ProjectsDir, "state.json"))
	idx.Nodes["existing-node"] = state.IndexEntry{
		Name: "Existing Node", Type: state.NodeLeaf,
		State: state.StatusNotStarted, Address: "existing-node",
	}
	_ = state.SaveRootIndex(filepath.Join(env.ProjectsDir, "state.json"), idx)

	// Lock the wolfcastle dir so SaveBatch (atomicWriteJSON) fails.
	_ = os.Chmod(env.WolfcastleDir, 0555)
	t.Cleanup(func() { _ = os.Chmod(env.WolfcastleDir, 0755) })

	env.RootCmd.SetArgs([]string{"audit", "approve", "f-1"})
	err := env.RootCmd.Execute()
	if err == nil {
		t.Error("expected error when SaveBatch fails")
	}
}

func TestApprove_SaveRootIndexError_ReadOnly(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("permission test not reliable on Windows")
	}

	env := newTestEnv(t)

	batch := &state.Batch{
		ID:     "audit-sri",
		Status: state.BatchPending,
		Scopes: []string{"test"},
		Findings: []state.Finding{
			{ID: "f-1", Title: "Index Test", Status: state.FindingPending},
			{ID: "f-2", Title: "Padding", Status: state.FindingRejected},
		},
	}
	batchPath := filepath.Join(env.WolfcastleDir, "audit-state.json")
	_ = state.SaveBatch(batchPath, batch)

	// Pre-create the node so approve goes through the "already exists" path.
	idx, _ := state.LoadRootIndex(filepath.Join(env.ProjectsDir, "state.json"))
	idx.Nodes["index-test"] = state.IndexEntry{
		Name: "Index Test", Type: state.NodeLeaf,
		State: state.StatusNotStarted, Address: "index-test",
	}
	_ = state.SaveRootIndex(filepath.Join(env.ProjectsDir, "state.json"), idx)

	env.RootCmd.SetArgs([]string{"audit", "approve", "f-1"})
	// Execute once to approve (SaveBatch writes to wolfcastle dir).
	// To make SaveBatch succeed but SaveRootIndex fail, we lock the
	// projects dir after the batch save.
	// Since we can't inject a hook, we lock projects dir so that
	// SaveRootIndex (inside projects/) fails. But SaveBatch is in wolfcastle/.
	_ = os.Chmod(env.ProjectsDir, 0555)
	t.Cleanup(func() { _ = os.Chmod(env.ProjectsDir, 0755) })

	err := env.RootCmd.Execute()
	if err == nil {
		t.Error("expected error when SaveRootIndex fails")
	}
}
