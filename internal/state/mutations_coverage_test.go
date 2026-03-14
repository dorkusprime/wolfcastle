package state

import (
	"testing"
	"time"

	"github.com/dorkusprime/wolfcastle/internal/clock"
)

// ═══════════════════════════════════════════════════════════════════════════
// AddEscalation — clock parameter and nested address coverage
// ═══════════════════════════════════════════════════════════════════════════

func TestAddEscalation_WithNestedAddress(t *testing.T) {
	t.Parallel()
	parent := NewNodeState("orch-1", "Orchestrator", NodeOrchestrator)
	fixed := time.Date(2026, 6, 15, 10, 30, 0, 0, time.UTC)
	clk := clock.NewFixed(fixed)

	AddEscalation(parent, "parent/child/grandchild", "deep issue", "gap-deep", clk)

	if len(parent.Audit.Escalations) != 1 {
		t.Fatalf("expected 1 escalation, got %d", len(parent.Audit.Escalations))
	}
	esc := parent.Audit.Escalations[0]
	// The child slug should be extracted from the last segment
	if esc.ID != "escalation-grandchild-1" {
		t.Errorf("expected id 'escalation-grandchild-1', got %q", esc.ID)
	}
	if !esc.Timestamp.Equal(fixed) {
		t.Errorf("expected timestamp %v, got %v", fixed, esc.Timestamp)
	}
	if esc.SourceNode != "parent/child/grandchild" {
		t.Errorf("expected source_node 'parent/child/grandchild', got %q", esc.SourceNode)
	}
	if esc.SourceGapID != "gap-deep" {
		t.Errorf("expected source_gap_id 'gap-deep', got %q", esc.SourceGapID)
	}
}

func TestAddEscalation_WithMockClock(t *testing.T) {
	t.Parallel()
	parent := NewNodeState("orch-2", "Orchestrator", NodeOrchestrator)
	start := time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)
	clk := clock.NewMock(start)

	AddEscalation(parent, "child-a", "first", "gap-1", clk)
	clk.Advance(5 * time.Minute)
	AddEscalation(parent, "child-b", "second", "gap-2", clk)

	if len(parent.Audit.Escalations) != 2 {
		t.Fatalf("expected 2 escalations, got %d", len(parent.Audit.Escalations))
	}

	first := parent.Audit.Escalations[0]
	second := parent.Audit.Escalations[1]

	if !first.Timestamp.Equal(start) {
		t.Errorf("first escalation timestamp wrong: got %v", first.Timestamp)
	}
	expected := start.Add(5 * time.Minute)
	if !second.Timestamp.Equal(expected) {
		t.Errorf("second escalation timestamp wrong: expected %v, got %v", expected, second.Timestamp)
	}
}

func TestAddEscalation_EmptySourceGapID(t *testing.T) {
	t.Parallel()
	parent := NewNodeState("orch-3", "Orchestrator", NodeOrchestrator)
	clk := clock.NewFixed(time.Date(2026, 3, 14, 12, 0, 0, 0, time.UTC))

	AddEscalation(parent, "leaf-x", "no gap reference", "", clk)

	esc := parent.Audit.Escalations[0]
	if esc.SourceGapID != "" {
		t.Errorf("expected empty source_gap_id, got %q", esc.SourceGapID)
	}
}

func TestAddEscalation_TopLevelAddress(t *testing.T) {
	t.Parallel()
	parent := NewNodeState("orch-4", "Orchestrator", NodeOrchestrator)
	clk := clock.NewFixed(time.Date(2026, 3, 14, 12, 0, 0, 0, time.UTC))

	// Single-segment address (no slash) means childSlug == sourceNode
	AddEscalation(parent, "simple-leaf", "issue", "gap-1", clk)

	esc := parent.Audit.Escalations[0]
	if esc.ID != "escalation-simple-leaf-1" {
		t.Errorf("expected id 'escalation-simple-leaf-1', got %q", esc.ID)
	}
}
