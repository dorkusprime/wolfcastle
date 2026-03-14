package audit

import (
	"os"
	"path/filepath"
	"runtime"
	"testing"

	"github.com/dorkusprime/wolfcastle/internal/state"
)

// ── audit breadcrumb — SaveNodeState error ──────────────────────────

func TestBreadcrumb_SaveNodeStateError_ReadOnly(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("permission test not reliable on Windows")
	}

	env := newTestEnv(t)
	createLeafNode(t, env, "bc-proj", "BC Project")

	nodeDir := filepath.Join(env.ProjectsDir, "bc-proj")
	_ = os.Chmod(nodeDir, 0555)
	t.Cleanup(func() { _ = os.Chmod(nodeDir, 0755) })

	env.RootCmd.SetArgs([]string{"audit", "breadcrumb", "--node", "bc-proj", "some note"})
	err := env.RootCmd.Execute()
	if err == nil {
		t.Error("expected error when SaveNodeState fails for breadcrumb")
	}
}

// ── audit escalate — SaveNodeState error ────────────────────────────

func TestEscalate_SaveNodeStateError_ReadOnly(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("permission test not reliable on Windows")
	}

	env := newTestEnv(t)
	createOrchestratorWithChild(t, env, "esc-parent", "esc-parent/esc-child")

	parentDir := filepath.Join(env.ProjectsDir, "esc-parent")
	_ = os.Chmod(parentDir, 0555)
	t.Cleanup(func() { _ = os.Chmod(parentDir, 0755) })

	env.RootCmd.SetArgs([]string{"audit", "escalate", "--node", "esc-parent/esc-child", "problem"})
	err := env.RootCmd.Execute()
	if err == nil {
		t.Error("expected error when SaveNodeState fails for escalate")
	}
}

// ── audit fix-gap — SaveNodeState error ─────────────────────────────

func TestFixGap_SaveNodeStateError_ReadOnly(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("permission test not reliable on Windows")
	}

	env := newTestEnv(t)
	createLeafNode(t, env, "fg-proj", "FG Project")

	// Add a gap first.
	env.RootCmd.SetArgs([]string{"audit", "gap", "--node", "fg-proj", "a gap"})
	_ = env.RootCmd.Execute()

	ns := loadNodeState(t, env, "fg-proj")
	gapID := ns.Audit.Gaps[0].ID

	// Lock so SaveNodeState fails.
	nodeDir := filepath.Join(env.ProjectsDir, "fg-proj")
	_ = os.Chmod(nodeDir, 0555)
	t.Cleanup(func() { _ = os.Chmod(nodeDir, 0755) })

	env.RootCmd.SetArgs([]string{"audit", "fix-gap", "--node", "fg-proj", gapID})
	err := env.RootCmd.Execute()
	if err == nil {
		t.Error("expected error when SaveNodeState fails for fix-gap")
	}
}

// ── audit gap — SaveNodeState error ─────────────────────────────────

func TestGap_SaveNodeStateError_ReadOnly(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("permission test not reliable on Windows")
	}

	env := newTestEnv(t)
	createLeafNode(t, env, "gap-proj", "Gap Project")

	nodeDir := filepath.Join(env.ProjectsDir, "gap-proj")
	_ = os.Chmod(nodeDir, 0555)
	t.Cleanup(func() { _ = os.Chmod(nodeDir, 0755) })

	env.RootCmd.SetArgs([]string{"audit", "gap", "--node", "gap-proj", "some gap"})
	err := env.RootCmd.Execute()
	if err == nil {
		t.Error("expected error when SaveNodeState fails for gap")
	}
}

// ── audit resolve — SaveNodeState error ─────────────────────────────

func TestResolve_SaveNodeStateError_ReadOnly(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("permission test not reliable on Windows")
	}

	env := newTestEnv(t)
	createOrchestratorWithChild(t, env, "res-parent", "res-parent/res-child")

	// Add escalation.
	env.RootCmd.SetArgs([]string{"audit", "escalate", "--node", "res-parent/res-child", "issue"})
	_ = env.RootCmd.Execute()

	parentNs := loadNodeState(t, env, "res-parent")
	escID := parentNs.Audit.Escalations[0].ID

	parentDir := filepath.Join(env.ProjectsDir, "res-parent")
	_ = os.Chmod(parentDir, 0555)
	t.Cleanup(func() { _ = os.Chmod(parentDir, 0755) })

	env.RootCmd.SetArgs([]string{"audit", "resolve", "--node", "res-parent", escID})
	err := env.RootCmd.Execute()
	if err == nil {
		t.Error("expected error when SaveNodeState fails for resolve")
	}
}

// ── audit reject — SaveBatch error ──────────────────────────────────

func TestReject_SaveBatchError_ReadOnly(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("permission test not reliable on Windows")
	}

	env := newTestEnv(t)

	batch := &state.Batch{
		ID:     "audit-rej",
		Status: state.BatchPending,
		Scopes: []string{"test"},
		Findings: []state.Finding{
			{ID: "f-1", Title: "Reject Me", Status: state.FindingPending},
			{ID: "f-2", Title: "Also Pending", Status: state.FindingPending},
		},
	}
	_ = state.SaveBatch(filepath.Join(env.WolfcastleDir, "audit-state.json"), batch)

	// Lock the wolfcastle dir so SaveBatch (atomicWriteJSON) fails.
	_ = os.Chmod(env.WolfcastleDir, 0555)
	t.Cleanup(func() { _ = os.Chmod(env.WolfcastleDir, 0755) })

	env.RootCmd.SetArgs([]string{"audit", "reject", "f-1"})
	err := env.RootCmd.Execute()
	if err == nil {
		t.Error("expected error when SaveBatch fails for reject")
	}
}
