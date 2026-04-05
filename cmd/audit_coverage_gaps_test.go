package cmd

import (
	"testing"

	"github.com/dorkusprime/wolfcastle/internal/state"
)

// ═══════════════════════════════════════════════════════════════════════════
// nodeGlyphPlain: all status branches
// ═══════════════════════════════════════════════════════════════════════════

func TestNodeGlyphPlain_AllStatuses(t *testing.T) {
	t.Parallel()
	tests := []struct {
		status state.NodeStatus
		want   string
	}{
		{state.StatusComplete, "●"},
		{state.StatusInProgress, "◐"},
		{state.StatusBlocked, "☢"},
		{state.StatusNotStarted, "◯"},
		{"something_unexpected", "◯"},
	}
	for _, tt := range tests {
		t.Run(string(tt.status), func(t *testing.T) {
			got := nodeGlyphPlain(tt.status)
			if got != tt.want {
				t.Errorf("nodeGlyphPlain(%q) = %q, want %q", tt.status, got, tt.want)
			}
		})
	}
}

// ═══════════════════════════════════════════════════════════════════════════
// countOpenEscalations: open, closed, mixed, empty
// ═══════════════════════════════════════════════════════════════════════════

func TestCountOpenEscalations(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name string
		escs []state.Escalation
		want int
	}{
		{"nil", nil, 0},
		{"empty", []state.Escalation{}, 0},
		{"all open", []state.Escalation{
			{ID: "e1", Status: state.EscalationOpen},
			{ID: "e2", Status: state.EscalationOpen},
		}, 2},
		{"all resolved", []state.Escalation{
			{ID: "e1", Status: state.EscalationResolved},
		}, 0},
		{"mixed", []state.Escalation{
			{ID: "e1", Status: state.EscalationOpen},
			{ID: "e2", Status: state.EscalationResolved},
			{ID: "e3", Status: state.EscalationOpen},
		}, 2},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := countOpenEscalations(tt.escs)
			if got != tt.want {
				t.Errorf("countOpenEscalations() = %d, want %d", got, tt.want)
			}
		})
	}
}

// ═══════════════════════════════════════════════════════════════════════════
// countGaps: open, fixed, mixed, empty
// ═══════════════════════════════════════════════════════════════════════════

func TestCountGaps_AllCombinations(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name     string
		gaps     []state.Gap
		wantOpen int
		wantFix  int
	}{
		{"nil", nil, 0, 0},
		{"empty", []state.Gap{}, 0, 0},
		{"all open", []state.Gap{
			{ID: "g1", Status: state.GapOpen},
			{ID: "g2", Status: state.GapOpen},
		}, 2, 0},
		{"all fixed", []state.Gap{
			{ID: "g1", Status: state.GapFixed},
		}, 0, 1},
		{"mixed", []state.Gap{
			{ID: "g1", Status: state.GapOpen},
			{ID: "g2", Status: state.GapFixed},
			{ID: "g3", Status: state.GapOpen},
		}, 2, 1},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			open, fixed := countGaps(tt.gaps)
			if open != tt.wantOpen || fixed != tt.wantFix {
				t.Errorf("countGaps() = (%d, %d), want (%d, %d)", open, fixed, tt.wantOpen, tt.wantFix)
			}
		})
	}
}
