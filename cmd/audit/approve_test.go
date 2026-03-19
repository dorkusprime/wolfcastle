package audit

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/dorkusprime/wolfcastle/internal/state"
)

// ---------------------------------------------------------------------------
// audit approve
// ---------------------------------------------------------------------------

func TestApprove_SingleFinding(t *testing.T) {
	env := newTestEnv(t)

	batch := &state.Batch{
		ID:     "audit-test",
		Status: state.BatchPending,
		Scopes: []string{"security"},
		Findings: []state.Finding{
			{ID: "finding-1", Title: "Missing Rate Limiting", Status: state.FindingPending, Description: "No rate limiting on API"},
		},
	}
	batchPath := filepath.Join(env.WolfcastleDir, "audit-state.json")
	_ = state.SaveBatch(batchPath, batch)

	env.RootCmd.SetArgs([]string{"audit", "approve", "finding-1"})
	if err := env.RootCmd.Execute(); err != nil {
		t.Fatalf("approve failed: %v", err)
	}

	// Batch should be archived since all findings decided
	_, err := os.Stat(batchPath)
	if !os.IsNotExist(err) {
		t.Error("batch file should be removed after all findings decided")
	}

	// History should contain the decision
	historyPath := filepath.Join(env.WolfcastleDir, "audit-review-history.json")
	history, err := state.LoadHistory(historyPath)
	if err != nil {
		t.Fatalf("loading history: %v", err)
	}
	if len(history.Entries) != 1 {
		t.Fatalf("expected 1 history entry, got %d", len(history.Entries))
	}
}

func TestApprove_All(t *testing.T) {
	env := newTestEnv(t)

	batch := &state.Batch{
		ID:     "audit-test",
		Status: state.BatchPending,
		Scopes: []string{"security"},
		Findings: []state.Finding{
			{ID: "finding-1", Title: "Rate Limiting Issue", Status: state.FindingPending},
			{ID: "finding-2", Title: "Auth Bypass Risk", Status: state.FindingPending},
		},
	}
	batchPath := filepath.Join(env.WolfcastleDir, "audit-state.json")
	_ = state.SaveBatch(batchPath, batch)

	env.RootCmd.SetArgs([]string{"audit", "approve", "--all"})
	if err := env.RootCmd.Execute(); err != nil {
		t.Fatalf("approve --all failed: %v", err)
	}
}

func TestApprove_NoBatch(t *testing.T) {
	env := newTestEnv(t)

	env.RootCmd.SetArgs([]string{"audit", "approve", "finding-1"})
	err := env.RootCmd.Execute()
	if err == nil {
		t.Error("expected error when no batch exists")
	}
}

func TestApprove_NoArgs(t *testing.T) {
	env := newTestEnv(t)

	batch := &state.Batch{
		ID:     "audit-test",
		Status: state.BatchPending,
		Findings: []state.Finding{
			{ID: "f-1", Title: "Test", Status: state.FindingPending},
		},
	}
	_ = state.SaveBatch(filepath.Join(env.WolfcastleDir, "audit-state.json"), batch)

	env.RootCmd.SetArgs([]string{"audit", "approve"})
	err := env.RootCmd.Execute()
	if err == nil {
		t.Error("expected error with no args and no --all")
	}
}

func TestApprove_FindingNotFound(t *testing.T) {
	env := newTestEnv(t)

	batch := &state.Batch{
		ID:     "audit-test",
		Status: state.BatchPending,
		Scopes: []string{"test"},
		Findings: []state.Finding{
			{ID: "f-1", Title: "Test", Status: state.FindingPending},
		},
	}
	_ = state.SaveBatch(filepath.Join(env.WolfcastleDir, "audit-state.json"), batch)

	env.RootCmd.SetArgs([]string{"audit", "approve", "nonexistent"})
	err := env.RootCmd.Execute()
	if err == nil {
		t.Error("expected error for nonexistent finding ID")
	}
}

func TestApprove_AlreadyDecided(t *testing.T) {
	env := newTestEnv(t)

	batch := &state.Batch{
		ID:     "audit-test",
		Status: state.BatchPending,
		Scopes: []string{"test"},
		Findings: []state.Finding{
			{ID: "f-1", Title: "Test", Status: state.FindingRejected},
		},
	}
	_ = state.SaveBatch(filepath.Join(env.WolfcastleDir, "audit-state.json"), batch)

	env.RootCmd.SetArgs([]string{"audit", "approve", "f-1"})
	err := env.RootCmd.Execute()
	if err == nil {
		t.Error("expected error when finding already decided")
	}
}

func TestApprove_NoPendingForAll(t *testing.T) {
	env := newTestEnv(t)

	batch := &state.Batch{
		ID:     "audit-test",
		Status: state.BatchPending,
		Scopes: []string{"test"},
		Findings: []state.Finding{
			{ID: "f-1", Title: "Test", Status: state.FindingRejected},
		},
	}
	_ = state.SaveBatch(filepath.Join(env.WolfcastleDir, "audit-state.json"), batch)

	env.RootCmd.SetArgs([]string{"audit", "approve", "--all"})
	err := env.RootCmd.Execute()
	if err == nil {
		t.Error("expected error when no pending findings for --all")
	}
}

func TestApprove_JSONOutput(t *testing.T) {
	env := newTestEnv(t)
	env.App.JSONOutput = true
	defer func() { env.App.JSONOutput = false }()

	batch := &state.Batch{
		ID:     "test",
		Status: state.BatchPending,
		Scopes: []string{"test"},
		Findings: []state.Finding{
			{ID: "f-1", Title: "Finding One", Status: state.FindingPending},
			{ID: "f-2", Title: "Finding Two", Status: state.FindingPending},
		},
	}
	_ = state.SaveBatch(filepath.Join(env.WolfcastleDir, "audit-state.json"), batch)

	env.RootCmd.SetArgs([]string{"audit", "approve", "f-1"})
	if err := env.RootCmd.Execute(); err != nil {
		t.Fatalf("approve (json) failed: %v", err)
	}
}

func TestApprove_NoIdentity(t *testing.T) {
	env := newTestEnv(t)
	env.App.Identity = nil

	env.RootCmd.SetArgs([]string{"audit", "approve", "f-1"})
	err := env.RootCmd.Execute()
	if err == nil {
		t.Error("expected error when identity is nil")
	}
}

func TestApprove_ExistingProject(t *testing.T) {
	env := newTestEnv(t)

	// Create a node that matches the slug of the finding title
	env.createLeafNode(t, "rate-limiting-issue", "Rate Limiting Issue")

	batch := &state.Batch{
		ID:     "test",
		Status: state.BatchPending,
		Scopes: []string{"test"},
		Findings: []state.Finding{
			{ID: "f-1", Title: "Rate Limiting Issue", Status: state.FindingPending},
		},
	}
	_ = state.SaveBatch(filepath.Join(env.WolfcastleDir, "audit-state.json"), batch)

	env.RootCmd.SetArgs([]string{"audit", "approve", "f-1"})
	if err := env.RootCmd.Execute(); err != nil {
		t.Fatalf("approve existing project failed: %v", err)
	}
}

// ---------------------------------------------------------------------------
// finalizeBatchIfComplete
// ---------------------------------------------------------------------------

func TestFinalizeBatchIfComplete_StillPending(t *testing.T) {
	env := newTestEnv(t)

	batch := &state.Batch{
		ID:     "test",
		Status: state.BatchPending,
		Scopes: []string{"test"},
		Findings: []state.Finding{
			{ID: "f-1", Title: "Done", Status: state.FindingApproved},
			{ID: "f-2", Title: "Pending", Status: state.FindingPending},
		},
	}
	batchPath := filepath.Join(env.WolfcastleDir, "audit-state.json")
	_ = state.SaveBatch(batchPath, batch)

	err := finalizeBatchIfComplete(env.App, batch, batchPath)
	if err != nil {
		t.Fatalf("finalizeBatchIfComplete failed: %v", err)
	}
	// File should still exist since there's a pending finding
	if _, err := os.Stat(batchPath); os.IsNotExist(err) {
		t.Error("batch file should still exist when findings are pending")
	}
}

func TestFinalizeBatchIfComplete_AllDecided(t *testing.T) {
	env := newTestEnv(t)

	batch := &state.Batch{
		ID:     "test",
		Status: state.BatchPending,
		Scopes: []string{"test"},
		Findings: []state.Finding{
			{ID: "f-1", Title: "Approved", Status: state.FindingApproved},
			{ID: "f-2", Title: "Rejected", Status: state.FindingRejected},
		},
	}
	batchPath := filepath.Join(env.WolfcastleDir, "audit-state.json")
	_ = state.SaveBatch(batchPath, batch)

	err := finalizeBatchIfComplete(env.App, batch, batchPath)
	if err != nil {
		t.Fatalf("finalizeBatchIfComplete failed: %v", err)
	}
	// Batch should be removed
	if _, err := os.Stat(batchPath); !os.IsNotExist(err) {
		t.Error("batch file should be removed when all findings decided")
	}
}
