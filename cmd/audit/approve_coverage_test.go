package audit

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/dorkusprime/wolfcastle/internal/state"
)

// ═══════════════════════════════════════════════════════════════════════════
// approve: additional approval paths
// ═══════════════════════════════════════════════════════════════════════════

func TestApprove_WithDescriptionField(t *testing.T) {
	env := newTestEnv(t)

	batch := &state.Batch{
		ID:     "audit-desc",
		Status: state.BatchPending,
		Scopes: []string{"quality"},
		Findings: []state.Finding{
			{
				ID:          "finding-desc",
				Title:       "Documentation Gap",
				Description: "Several API endpoints lack documentation for error responses",
				Status:      state.FindingPending,
			},
		},
	}
	batchPath := filepath.Join(env.WolfcastleDir, "audit-state.json")
	_ = state.SaveBatch(batchPath, batch)

	env.RootCmd.SetArgs([]string{"audit", "approve", "finding-desc"})
	if err := env.RootCmd.Execute(); err != nil {
		t.Fatalf("approve with description failed: %v", err)
	}

	// Batch should be archived since single finding
	if _, err := os.Stat(batchPath); !os.IsNotExist(err) {
		t.Error("batch should be removed after all findings decided")
	}
}

func TestApprove_MultipleFindings_ApproveAll(t *testing.T) {
	env := newTestEnv(t)

	batch := &state.Batch{
		ID:     "audit-multi",
		Status: state.BatchPending,
		Scopes: []string{"test"},
		Findings: []state.Finding{
			{ID: "f-1", Title: "First Good Finding", Status: state.FindingPending},
			{ID: "f-2", Title: "Second Good Finding", Status: state.FindingPending},
			{ID: "f-3", Title: "Third Good Finding", Status: state.FindingPending},
		},
	}
	_ = state.SaveBatch(filepath.Join(env.WolfcastleDir, "audit-state.json"), batch)

	env.RootCmd.SetArgs([]string{"audit", "approve", "--all"})
	err := env.RootCmd.Execute()
	if err != nil {
		t.Fatalf("approve --all with multiple findings failed: %v", err)
	}
}

func TestApprove_MixedPendingAndDecided(t *testing.T) {
	env := newTestEnv(t)

	batch := &state.Batch{
		ID:     "audit-mixed",
		Status: state.BatchPending,
		Scopes: []string{"security"},
		Findings: []state.Finding{
			{ID: "f-1", Title: "Already Rejected", Status: state.FindingRejected},
			{ID: "f-2", Title: "Pending Approval", Status: state.FindingPending},
		},
	}
	batchPath := filepath.Join(env.WolfcastleDir, "audit-state.json")
	_ = state.SaveBatch(batchPath, batch)

	env.RootCmd.SetArgs([]string{"audit", "approve", "f-2"})
	if err := env.RootCmd.Execute(); err != nil {
		t.Fatalf("approve mixed batch failed: %v", err)
	}

	// All findings decided now, batch should be archived
	if _, err := os.Stat(batchPath); !os.IsNotExist(err) {
		t.Error("batch file should be removed after all findings decided")
	}
}

func TestApprove_AllJSON(t *testing.T) {
	env := newTestEnv(t)
	env.App.JSON = true
	defer func() { env.App.JSON = false }()

	batch := &state.Batch{
		ID:     "audit-all-json",
		Status: state.BatchPending,
		Scopes: []string{"test"},
		Findings: []state.Finding{
			{ID: "f-1", Title: "First Finding", Status: state.FindingPending},
			{ID: "f-2", Title: "Second Finding", Status: state.FindingPending},
		},
	}
	_ = state.SaveBatch(filepath.Join(env.WolfcastleDir, "audit-state.json"), batch)

	env.RootCmd.SetArgs([]string{"audit", "approve", "--all"})
	if err := env.RootCmd.Execute(); err != nil {
		t.Fatalf("approve --all --json failed: %v", err)
	}
}
