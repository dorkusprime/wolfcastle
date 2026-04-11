package tree

import (
	"strings"
	"testing"

	"charm.land/lipgloss/v2"

	"github.com/dorkusprime/wolfcastle/internal/state"
)

// ---------------------------------------------------------------------------
// statusGlyphOnBg: all status branches
// ---------------------------------------------------------------------------

func TestStatusGlyphOnBg_Complete(t *testing.T) {
	t.Parallel()
	bg := lipgloss.Color("236")
	got := statusGlyphOnBg(state.StatusComplete, bg)
	if !strings.Contains(got, "●") {
		t.Errorf("expected ● for complete, got %q", got)
	}
}

func TestStatusGlyphOnBg_InProgress(t *testing.T) {
	t.Parallel()
	bg := lipgloss.Color("236")
	got := statusGlyphOnBg(state.StatusInProgress, bg)
	if !strings.Contains(got, "◐") {
		t.Errorf("expected ◐ for in_progress, got %q", got)
	}
}

func TestStatusGlyphOnBg_Blocked(t *testing.T) {
	t.Parallel()
	bg := lipgloss.Color("236")
	got := statusGlyphOnBg(state.StatusBlocked, bg)
	if !strings.Contains(got, "☢") {
		t.Errorf("expected ☢ for blocked, got %q", got)
	}
}

func TestStatusGlyphOnBg_Default(t *testing.T) {
	t.Parallel()
	bg := lipgloss.Color("236")
	got := statusGlyphOnBg(state.StatusNotStarted, bg)
	if !strings.Contains(got, "◯") {
		t.Errorf("expected ◯ for not_started, got %q", got)
	}
}

// ---------------------------------------------------------------------------
// taskStatusGlyphOnBg: non-InProgress branches
// ---------------------------------------------------------------------------

func TestTaskStatusGlyphOnBg_Complete(t *testing.T) {
	t.Parallel()
	bg := lipgloss.Color("236")
	got := taskStatusGlyphOnBg(state.StatusComplete, bg)
	if !strings.Contains(got, "●") {
		t.Errorf("expected ● for complete task, got %q", got)
	}
}

func TestTaskStatusGlyphOnBg_Blocked(t *testing.T) {
	t.Parallel()
	bg := lipgloss.Color("236")
	got := taskStatusGlyphOnBg(state.StatusBlocked, bg)
	if !strings.Contains(got, "☢") {
		t.Errorf("expected ☢ for blocked task, got %q", got)
	}
}

func TestTaskStatusGlyphOnBg_NotStarted(t *testing.T) {
	t.Parallel()
	bg := lipgloss.Color("236")
	got := taskStatusGlyphOnBg(state.StatusNotStarted, bg)
	if !strings.Contains(got, "◯") {
		t.Errorf("expected ◯ for not_started task, got %q", got)
	}
}

// ---------------------------------------------------------------------------
// renderTaskRow: ancestor hit path
// ---------------------------------------------------------------------------

func TestRenderTaskRow_AncestorHit(t *testing.T) {
	t.Parallel()
	row := TreeRow{
		Addr:   "node/t-001",
		Name:   "A Task",
		Depth:  1,
		Status: state.StatusComplete,
		IsTask: true,
	}
	normal := RenderRow(row, 60, false, false)
	ancestor := RenderRow(row, 60, false, false, false, true)
	if normal == ancestor {
		t.Error("ancestor hit task row should differ from normal task row")
	}
}

func TestRenderNodeRow_AncestorHit(t *testing.T) {
	t.Parallel()
	row := TreeRow{
		Addr:       "a",
		Name:       "Node",
		Depth:      0,
		NodeType:   state.NodeOrchestrator,
		Status:     state.StatusInProgress,
		Expandable: true,
	}
	normal := RenderRow(row, 60, false, false)
	ancestor := RenderRow(row, 60, false, false, false, true)
	if normal == ancestor {
		t.Error("ancestor hit node row should differ from normal node row")
	}
}

// ---------------------------------------------------------------------------
// renderTaskRow: selected path coverage
// ---------------------------------------------------------------------------

func TestRenderTaskRow_Selected_Complete(t *testing.T) {
	t.Parallel()
	row := TreeRow{
		Addr:   "node/t-002",
		Name:   "Complete task",
		Depth:  1,
		Status: state.StatusComplete,
		IsTask: true,
	}
	got := RenderRow(row, 60, true, false)
	if !strings.Contains(got, "t-002") {
		t.Error("selected task row should contain task ID")
	}
}

func TestRenderTaskRow_Selected_Blocked(t *testing.T) {
	t.Parallel()
	row := TreeRow{
		Addr:   "node/t-003",
		Name:   "Blocked task",
		Depth:  1,
		Status: state.StatusBlocked,
		IsTask: true,
	}
	got := RenderRow(row, 60, true, false)
	if !strings.Contains(got, "t-003") {
		t.Error("selected blocked task row should contain task ID")
	}
}

// ---------------------------------------------------------------------------
// taskHint rendering in tree rows
// ---------------------------------------------------------------------------

func TestRenderNodeRow_WithTaskHint(t *testing.T) {
	t.Parallel()
	row := TreeRow{
		Addr:       "alpha",
		Name:       "Alpha Node",
		Depth:      0,
		NodeType:   state.NodeLeaf,
		Status:     state.StatusInProgress,
		Expandable: true,
		TaskHint:   "2/5 tasks",
	}
	got := RenderRow(row, 80, false, false)
	if !strings.Contains(got, "2/5 tasks") {
		t.Errorf("expected task hint in output, got %q", got)
	}
}

// ---------------------------------------------------------------------------
// taskHint: all branches
// ---------------------------------------------------------------------------

func TestTaskHint_NoTasks(t *testing.T) {
	t.Parallel()
	ns := &state.NodeState{}
	got := taskHint(ns)
	if got != "" {
		t.Errorf("expected empty string for no tasks, got %q", got)
	}
}

func TestTaskHint_TasksNoFailures(t *testing.T) {
	t.Parallel()
	ns := &state.NodeState{
		Tasks: []state.Task{
			{ID: "t1"}, {ID: "t2"}, {ID: "t3"},
		},
	}
	got := taskHint(ns)
	if got != "(3 tasks)" {
		t.Errorf("expected '(3 tasks)', got %q", got)
	}
}

func TestTaskHint_TasksWithFailures(t *testing.T) {
	t.Parallel()
	ns := &state.NodeState{
		Tasks: []state.Task{
			{ID: "t1", FailureCount: 2},
			{ID: "t2", FailureCount: 0},
			{ID: "t3", FailureCount: 1},
		},
	}
	got := taskHint(ns)
	if got != "(3 tasks, 3 failures)" {
		t.Errorf("expected '(3 tasks, 3 failures)', got %q", got)
	}
}
