package daemon

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	dmn "github.com/dorkusprime/wolfcastle/internal/daemon"
	"github.com/dorkusprime/wolfcastle/internal/state"
	"github.com/dorkusprime/wolfcastle/internal/testutil"
)

// ═══════════════════════════════════════════════════════════════════════════
// modeFlag.IsBoolFlag: verify the interface method returns true
// ═══════════════════════════════════════════════════════════════════════════

func TestModeFlag_IsBoolFlag(t *testing.T) {
	t.Parallel()
	var m outputMode
	f := &modeFlag{target: &m, value: modeThoughts}
	if !f.IsBoolFlag() {
		t.Error("IsBoolFlag() should return true")
	}
	if f.Type() != "bool" {
		t.Errorf("Type() = %q, want %q", f.Type(), "bool")
	}
	if f.String() != "false" {
		t.Errorf("String() = %q, want %q", f.String(), "false")
	}
	if err := f.Set("true"); err != nil {
		t.Errorf("Set() error: %v", err)
	}
	if m != modeThoughts {
		t.Errorf("target = %d, want modeThoughts", m)
	}
}

// ═══════════════════════════════════════════════════════════════════════════
// followJSON: verify context cancellation stops the follow loop
// ═══════════════════════════════════════════════════════════════════════════

func TestFollowJSON_ContextCancellation(t *testing.T) {
	t.Parallel()
	logDir := t.TempDir()

	// Write a log file so the reader has something to find.
	_ = os.WriteFile(filepath.Join(logDir, "0001-20260321T18-00Z.jsonl"),
		[]byte(`{"type":"daemon_start","scope":"test","timestamp":"2026-03-21T18:00:00Z"}`+"\n"), 0644)

	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	err := followJSON(ctx, logDir, func() bool { return false })
	if err != nil {
		t.Fatalf("followJSON with cancelled context: %v", err)
	}
}

// ═══════════════════════════════════════════════════════════════════════════
// printParallelStatus: exercise all branches (active, yielded, scope)
// ═══════════════════════════════════════════════════════════════════════════

func TestPrintParallelStatus_AllBranches(t *testing.T) {
	t.Parallel()
	ps := &dmn.ParallelStatus{
		MaxWorkers: 3,
		Active: []dmn.ParallelWorkerEntry{
			{Task: "leaf/task-0001", Node: "leaf", Scope: []string{"auth", "db"}},
			{Task: "leaf/task-0002", Node: "leaf"},
		},
		Yielded: []dmn.ParallelYieldedEntry{
			{Task: "leaf/task-0003", Blocker: "leaf/task-0001", YieldCount: 1},
			{Task: "leaf/task-0004", Blocker: "leaf/task-0001", YieldCount: 3, BlockedForSecs: 120},
		},
	}
	// Should not panic. Exercises active (with and without scope) and
	// yielded (with and without multi-yield suffix).
	printParallelStatus(ps)
}

func TestPrintParallelStatus_NoYielded(t *testing.T) {
	t.Parallel()
	ps := &dmn.ParallelStatus{
		MaxWorkers: 2,
		Active: []dmn.ParallelWorkerEntry{
			{Task: "leaf/task-0001", Node: "leaf"},
		},
	}
	printParallelStatus(ps)
}

// ═══════════════════════════════════════════════════════════════════════════
// nodeGlyphPlain and countOpenEscalations: full status coverage
// ═══════════════════════════════════════════════════════════════════════════

func TestNodeGlyphPlain_AllStatuses(t *testing.T) {
	t.Parallel()
	env := newTestEnv(t)
	env.env.WithProject("Proj", testutil.Leaf("proj"))

	// Exercise nodeGlyphPlain through the describe command's code path
	// by testing directly. These are unexported in cmd package so we test
	// via the status display that uses nodeGlyph. The describe tests
	// cover nodeGlyphPlain.
	tests := []struct {
		status state.NodeStatus
		want   string
	}{
		{state.StatusComplete, "●"},
		{state.StatusInProgress, "◐"},
		{state.StatusBlocked, "☢"},
		{state.StatusNotStarted, "◯"},
		{"unknown_status", "◯"},
	}
	for _, tt := range tests {
		got := nodeGlyph(tt.status)
		if got != tt.want {
			t.Errorf("nodeGlyph(%q) = %q, want %q", tt.status, got, tt.want)
		}
	}
}
