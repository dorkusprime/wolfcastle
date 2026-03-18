package audit

import (
	"path/filepath"
	"testing"
	"time"

	"github.com/dorkusprime/wolfcastle/internal/state"
)

// ---------------------------------------------------------------------------
// audit history - with entries
// ---------------------------------------------------------------------------

func TestHistory_WithEntries(t *testing.T) {
	env := newTestEnv(t)

	now := time.Now()
	history := &state.History{
		Entries: []state.HistoryEntry{
			{
				BatchID:     "batch-1",
				CompletedAt: now.Add(-24 * time.Hour),
				Scopes:      []string{"security"},
				Decisions: []state.Decision{
					{FindingID: "f-1", Title: "Auth Bypass", Action: string(state.FindingApproved), CreatedNode: "auth-bypass"},
					{FindingID: "f-2", Title: "Minor Issue", Action: string(state.FindingRejected)},
				},
			},
			{
				BatchID:     "batch-2",
				CompletedAt: now,
				Scopes:      []string{"performance"},
				Decisions: []state.Decision{
					{FindingID: "f-3", Title: "Slow Query", Action: string(state.FindingApproved), CreatedNode: "slow-query"},
				},
			},
		},
	}
	historyPath := filepath.Join(env.WolfcastleDir, "audit-review-history.json")
	_ = state.SaveHistory(historyPath, history)

	env.RootCmd.SetArgs([]string{"audit", "history"})
	if err := env.RootCmd.Execute(); err != nil {
		t.Fatalf("history with entries failed: %v", err)
	}
}

func TestHistory_WithEntries_JSON(t *testing.T) {
	env := newTestEnv(t)
	env.App.JSONOutput = true
	defer func() { env.App.JSONOutput = false }()

	now := time.Now()
	history := &state.History{
		Entries: []state.HistoryEntry{
			{
				BatchID:     "batch-1",
				CompletedAt: now,
				Scopes:      []string{"security"},
				Decisions: []state.Decision{
					{FindingID: "f-1", Title: "Issue", Action: string(state.FindingApproved)},
				},
			},
		},
	}
	_ = state.SaveHistory(filepath.Join(env.WolfcastleDir, "audit-review-history.json"), history)

	env.RootCmd.SetArgs([]string{"audit", "history"})
	if err := env.RootCmd.Execute(); err != nil {
		t.Fatalf("history (json) with entries failed: %v", err)
	}
}

// ---------------------------------------------------------------------------
// audit run - edge cases (without model invocation)
// ---------------------------------------------------------------------------

func TestRunCmd_NoIdentity(t *testing.T) {
	env := newTestEnv(t)
	env.App.Identity = nil

	env.RootCmd.SetArgs([]string{"audit", "run"})
	err := env.RootCmd.Execute()
	if err == nil {
		t.Error("expected error when identity is nil")
	}
}

func TestRunCmd_ListFlag(t *testing.T) {
	env := newTestEnv(t)

	env.RootCmd.SetArgs([]string{"audit", "run", "--list"})
	if err := env.RootCmd.Execute(); err != nil {
		t.Fatalf("audit run --list failed: %v", err)
	}
}

func TestRunCmd_ListFlagJSON(t *testing.T) {
	env := newTestEnv(t)
	env.App.JSONOutput = true
	defer func() { env.App.JSONOutput = false }()

	env.RootCmd.SetArgs([]string{"audit", "run", "--list"})
	if err := env.RootCmd.Execute(); err != nil {
		t.Fatalf("audit run --list (json) failed: %v", err)
	}
}

func TestRunCmd_PendingBatchExists(t *testing.T) {
	env := newTestEnv(t)

	batch := &state.Batch{
		ID:     "existing",
		Status: state.BatchPending,
		Findings: []state.Finding{
			{ID: "f-1", Title: "Test", Status: state.FindingPending},
		},
	}
	_ = state.SaveBatch(filepath.Join(env.WolfcastleDir, "audit-state.json"), batch)

	env.RootCmd.SetArgs([]string{"audit", "run"})
	err := env.RootCmd.Execute()
	if err == nil {
		t.Error("expected error when pending batch exists")
	}
}

func TestRunCmd_UnknownScope(t *testing.T) {
	env := newTestEnv(t)

	env.RootCmd.SetArgs([]string{"audit", "run", "--scope", "nonexistent"})
	err := env.RootCmd.Execute()
	if err == nil {
		t.Error("expected error for unknown scope")
	}
}

func TestRunCmd_NoScopes(t *testing.T) {
	env := newTestEnv(t)

	// No scope files exist, no --list flag, no --scope flag
	// This should fail because no scopes found
	env.RootCmd.SetArgs([]string{"audit", "run"})
	err := env.RootCmd.Execute()
	if err == nil {
		t.Error("expected error when no scopes found")
	}
}
