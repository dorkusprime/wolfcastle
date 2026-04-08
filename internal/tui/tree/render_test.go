package tree

import (
	"strings"
	"testing"

	"charm.land/lipgloss/v2"

	"github.com/dorkusprime/wolfcastle/internal/state"
)

func TestRenderRow_OrchestratorCollapsed(t *testing.T) {
	row := TreeRow{
		Addr:       "parent",
		Name:       "My Orchestrator",
		Depth:      0,
		NodeType:   state.NodeOrchestrator,
		Status:     state.StatusInProgress,
		Expandable: true,
		IsExpanded: false,
	}

	out := RenderRow(row, 60, false, false)
	if !strings.Contains(out, "▸") {
		t.Error("collapsed orchestrator should show ▸")
	}
	if strings.Contains(out, "▾") {
		t.Error("collapsed orchestrator should not show ▾")
	}
}

func TestRenderRow_OrchestratorExpanded(t *testing.T) {
	row := TreeRow{
		Addr:       "parent",
		Name:       "My Orchestrator",
		Depth:      0,
		NodeType:   state.NodeOrchestrator,
		Status:     state.StatusInProgress,
		Expandable: true,
		IsExpanded: true,
	}

	out := RenderRow(row, 60, false, false)
	if !strings.Contains(out, "▾") {
		t.Error("expanded orchestrator should show ▾")
	}
}

func TestRenderRow_Leaf_NoExpandMarker(t *testing.T) {
	row := TreeRow{
		Addr:       "leaf1",
		Name:       "Simple Leaf",
		Depth:      1,
		NodeType:   state.NodeLeaf,
		Status:     state.StatusComplete,
		Expandable: false,
		IsExpanded: false,
	}

	out := RenderRow(row, 60, false, false)
	if strings.Contains(out, "▸") || strings.Contains(out, "▾") {
		t.Error("non-expandable leaf should not show expand markers")
	}
}

func TestRenderRow_Task(t *testing.T) {
	row := TreeRow{
		Addr:   "leaf1/t-001",
		Name:   "Implement feature",
		Depth:  2,
		Status: state.StatusInProgress,
		IsTask: true,
	}

	out := RenderRow(row, 80, false, false)
	if !strings.Contains(out, "t-001") {
		t.Error("task row should show task ID")
	}
	if !strings.Contains(out, "Implement feature") {
		t.Error("task row should show task title")
	}
}

func TestRenderRow_Selected(t *testing.T) {
	normal := RenderRow(TreeRow{
		Addr: "a", Name: "Node", NodeType: state.NodeLeaf, Expandable: true,
	}, 40, false, false)

	selected := RenderRow(TreeRow{
		Addr: "a", Name: "Node", NodeType: state.NodeLeaf, Expandable: true,
	}, 40, true, false)

	// Selected and normal should differ (styling applied).
	if normal == selected {
		t.Error("selected row should differ from normal row")
	}
}

func TestRenderRow_CurrentTarget(t *testing.T) {
	row := TreeRow{
		Addr:       "target-node",
		Name:       "Target",
		Depth:      0,
		NodeType:   state.NodeLeaf,
		Status:     state.StatusInProgress,
		Expandable: true,
	}

	out := RenderRow(row, 60, false, true)
	if !strings.Contains(out, "▶") {
		t.Error("current target row should contain ▶ prefix")
	}
}

func TestRenderRow_NotCurrentTarget(t *testing.T) {
	row := TreeRow{
		Addr:       "other-node",
		Name:       "Other",
		Depth:      0,
		NodeType:   state.NodeLeaf,
		Status:     state.StatusComplete,
		Expandable: true,
	}

	out := RenderRow(row, 60, false, false)
	if strings.Contains(out, "▶") {
		t.Error("non-target row should not contain ▶")
	}
}

func TestStatusGlyph_Complete(t *testing.T) {
	g := statusGlyph(state.StatusComplete)
	if !strings.Contains(g, "●") {
		t.Errorf("complete glyph should contain ●, got %q", g)
	}
}

func TestStatusGlyph_InProgress(t *testing.T) {
	g := statusGlyph(state.StatusInProgress)
	if !strings.Contains(g, "◐") {
		t.Errorf("in_progress glyph should contain ◐, got %q", g)
	}
}

func TestStatusGlyph_Blocked(t *testing.T) {
	g := statusGlyph(state.StatusBlocked)
	if !strings.Contains(g, "☢") {
		t.Errorf("blocked glyph should contain ☢, got %q", g)
	}
}

func TestStatusGlyph_NotStarted(t *testing.T) {
	g := statusGlyph(state.StatusNotStarted)
	if !strings.Contains(g, "◯") {
		t.Errorf("not_started glyph should contain ◯, got %q", g)
	}
}

func TestStatusGlyph_Default(t *testing.T) {
	// Unknown status should fall through to default (◯).
	g := statusGlyph("something_weird")
	if !strings.Contains(g, "◯") {
		t.Errorf("unknown status glyph should default to ◯, got %q", g)
	}
}

func TestTruncate(t *testing.T) {
	tests := []struct {
		name   string
		input  string
		maxLen int
		want   string
	}{
		{"short enough", "hello", 10, "hello"},
		{"exact fit", "hello", 5, "hello"},
		{"needs truncation", "hello world", 6, "hello…"},
		{"maxLen 1", "hello", 1, "…"},
		{"maxLen 0", "hello", 0, ""},
		{"negative maxLen", "hello", -1, ""},
		{"unicode", "日本語テスト", 4, "日本語…"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := truncate(tt.input, tt.maxLen)
			if got != tt.want {
				t.Errorf("truncate(%q, %d) = %q, want %q", tt.input, tt.maxLen, got, tt.want)
			}
		})
	}
}

func TestView_Empty(t *testing.T) {
	m := NewTreeModel()
	v := m.View()
	if !strings.Contains(v, "no nodes") {
		t.Errorf("empty tree view should contain 'no nodes', got %q", v)
	}
}

func TestView_RendersVisibleRows(t *testing.T) {
	m := NewTreeModel()
	m.SetIndex(simpleIndex())
	m.SetSize(60, 2) // only 2 rows visible

	v := m.View()
	lines := strings.Split(v, "\n")
	if len(lines) != 2 {
		t.Errorf("with height=2, expected 2 lines, got %d", len(lines))
	}
}

func TestView_ScrollTopRespected(t *testing.T) {
	m := NewTreeModel()
	m.SetIndex(simpleIndex())
	m.SetSize(60, 2)
	m.scrollTop = 1

	v := m.View()
	lines := strings.Split(v, "\n")
	if len(lines) != 2 {
		t.Errorf("expected 2 lines from scrollTop=1, got %d", len(lines))
	}
	// First visible line should be Beta (index 1), not Alpha (index 0).
	if !strings.Contains(lines[0], "Beta") {
		t.Errorf("first visible line should be Beta, got %q", lines[0])
	}
}

func TestView_ScrollTopBeyondList(t *testing.T) {
	m := NewTreeModel()
	m.SetIndex(simpleIndex())
	m.SetSize(60, 10)
	m.scrollTop = 100

	v := m.View()
	if v != "" {
		t.Errorf("scrollTop beyond list should produce empty string, got %q", v)
	}
}

func TestView_NegativeScrollTop(t *testing.T) {
	m := NewTreeModel()
	m.SetIndex(simpleIndex())
	m.SetSize(60, 10)
	m.scrollTop = -5

	// Should not panic.
	v := m.View()
	if !strings.Contains(v, "Alpha") {
		t.Error("negative scrollTop should still render from beginning")
	}
}

func TestRenderRow_LongNameTruncation(t *testing.T) {
	longName := strings.Repeat("A", 200)
	row := TreeRow{
		Addr:       "long",
		Name:       longName,
		Depth:      0,
		NodeType:   state.NodeLeaf,
		Status:     state.StatusNotStarted,
		Expandable: true,
	}

	out := RenderRow(row, 40, false, false)
	if strings.Contains(out, longName) {
		t.Error("long name should be truncated when width is narrow")
	}
	if !strings.Contains(out, "…") {
		t.Error("truncated name should contain ellipsis")
	}
}

func TestRenderRow_VeryNarrowWidth(t *testing.T) {
	row := TreeRow{
		Addr:       "narrow",
		Name:       "Some Node",
		Depth:      0,
		NodeType:   state.NodeLeaf,
		Status:     state.StatusComplete,
		Expandable: true,
	}

	// Should not panic even at very small widths.
	out := RenderRow(row, 5, false, false)
	if out == "" {
		t.Error("render at narrow width should still produce output")
	}
}

func TestView_CurrentTargetHighlighted(t *testing.T) {
	m := NewTreeModel()
	m.SetIndex(simpleIndex())
	m.SetSize(60, 10)
	m.SetCurrentTarget("beta")

	v := m.View()
	if !strings.Contains(v, "▶") {
		t.Error("view should contain target marker ▶ for the current target")
	}
}

func TestRenderTaskRow_IDExtraction(t *testing.T) {
	// The task ID is extracted from the last segment of the address.
	row := TreeRow{
		Addr:   "deep/nested/node/task-42",
		Name:   "A Task",
		Depth:  3,
		Status: state.StatusComplete,
		IsTask: true,
	}

	out := RenderRow(row, 80, false, false)
	if !strings.Contains(out, "task-42") {
		t.Error("task row should extract and display the task ID from address")
	}
}

func TestRenderRow_SearchHit(t *testing.T) {
	row := TreeRow{
		Addr:       "a",
		Name:       "SearchMe",
		Depth:      0,
		NodeType:   state.NodeLeaf,
		Status:     state.StatusNotStarted,
		Expandable: true,
	}

	normal := RenderRow(row, 60, false, false)
	hit := RenderRow(row, 60, false, false, true)

	if normal == hit {
		t.Error("search hit row should differ from normal row")
	}
}

func TestRenderRow_SearchHit_SelectedTakesPrecedence(t *testing.T) {
	row := TreeRow{
		Addr:       "a",
		Name:       "SearchMe",
		Depth:      0,
		NodeType:   state.NodeLeaf,
		Status:     state.StatusNotStarted,
		Expandable: true,
	}

	selected := RenderRow(row, 60, true, false)
	selectedAndHit := RenderRow(row, 60, true, false, true)

	// When selected, the search hit styling should not apply (selected takes priority).
	if selected != selectedAndHit {
		t.Error("selected styling should take precedence over search hit")
	}
}

func TestView_SearchMatchHighlighted(t *testing.T) {
	m := NewTreeModel()
	m.SetIndex(simpleIndex())
	m.SetSize(60, 10)
	m.SetSearchMatches(map[int]bool{1: true})

	v := m.View()
	// The view should contain Beta (the highlighted row).
	if !strings.Contains(v, "Beta") {
		t.Error("view should still contain the search-matched row text")
	}
}

func TestRenderRow_TaskSearchHit(t *testing.T) {
	row := TreeRow{
		Addr:   "node/t-001",
		Name:   "A task",
		Depth:  1,
		Status: state.StatusInProgress,
		IsTask: true,
	}

	normal := RenderRow(row, 60, false, false)
	hit := RenderRow(row, 60, false, false, true)

	if normal == hit {
		t.Error("search hit task row should differ from normal task row")
	}
}

func TestRenderRow_DepthIndentation(t *testing.T) {
	shallow := RenderRow(TreeRow{
		Addr: "a", Name: "Shallow", Depth: 0, NodeType: state.NodeLeaf, Expandable: true,
	}, 80, false, false)

	deep := RenderRow(TreeRow{
		Addr: "b", Name: "Deep", Depth: 3, NodeType: state.NodeLeaf, Expandable: true,
	}, 80, false, false)

	// The deeper row should contain the indentation spaces ("  " repeated per depth).
	// Depth 3 means 6 spaces of indent inside the rendered line.
	if !strings.Contains(deep, "      ") {
		t.Error("depth=3 row should contain 6-space indentation")
	}

	// Shallow (depth 0) should NOT have 6 spaces of indentation before the marker.
	// We check that the two rows differ, confirming depth affects layout.
	if shallow == deep {
		t.Error("different depths should produce different output")
	}
}

// ---------------------------------------------------------------------------
// Task glyph aligns with the wolfcastle status screen
// ---------------------------------------------------------------------------
//
// Task rows render their state with a different glyph from node
// rows so the in_progress task shows the same → arrow that the
// wolfcastle status CLI uses. The change is scoped to task rows
// only — node rows continue to use ◐ for in_progress because the
// status propagation up to leaves and orchestrators reads better
// with the half-circle glyph.

func TestTaskStatusGlyph_InProgressShowsArrow(t *testing.T) {
	got := taskStatusGlyph(state.StatusInProgress)
	if !strings.Contains(got, "→") {
		t.Errorf("taskStatusGlyph(StatusInProgress) = %q, want a string containing →", got)
	}
}

func TestTaskStatusGlyph_OtherStatesPassThrough(t *testing.T) {
	cases := map[state.NodeStatus]string{
		state.StatusComplete:   "●",
		state.StatusBlocked:    "☢",
		state.StatusNotStarted: "◯",
	}
	for st, want := range cases {
		got := taskStatusGlyph(st)
		if !strings.Contains(got, want) {
			t.Errorf("taskStatusGlyph(%v) = %q, want a string containing %q", st, got, want)
		}
	}
}

// TestStatusGlyph_NodeRowsUnchangedForInProgress is the regression
// test for the deliberate scope-narrowing decision: the cosmetic
// change to → applies only to task rows, not to node rows. If
// someone later edits statusGlyph itself instead of taskStatusGlyph,
// this test catches it.
func TestStatusGlyph_NodeRowsUnchangedForInProgress(t *testing.T) {
	got := statusGlyph(state.StatusInProgress)
	if !strings.Contains(got, "◐") {
		t.Errorf("statusGlyph(StatusInProgress) = %q, want a string containing ◐ (node rows must keep the half-circle glyph)", got)
	}
	if strings.Contains(got, "→") {
		t.Errorf("statusGlyph(StatusInProgress) = %q, must not contain → (that glyph is reserved for task rows)", got)
	}
}

func TestTaskStatusGlyphOnBg_InProgressShowsArrow(t *testing.T) {
	bg := lipgloss.Color("236")
	got := taskStatusGlyphOnBg(state.StatusInProgress, bg)
	if !strings.Contains(got, "→") {
		t.Errorf("taskStatusGlyphOnBg(StatusInProgress, bg) = %q, want a string containing →", got)
	}
}

// TestRenderTaskRow_InProgressUsesArrow exercises the actual task
// row rendering path (not just the glyph helper) to confirm the
// new glyph reaches the rendered output.
func TestRenderTaskRow_InProgressUsesArrow(t *testing.T) {
	row := TreeRow{
		Addr:   "alpha/task-0001",
		Name:   "deploy frobnicator",
		Depth:  1,
		Status: state.StatusInProgress,
		IsTask: true,
	}
	rendered := RenderRow(row, 80, false, false)
	if !strings.Contains(rendered, "→") {
		t.Errorf("rendered task row should contain →, got %q", rendered)
	}
	if strings.Contains(rendered, "◐") {
		t.Errorf("rendered task row must not contain the node-style ◐ glyph, got %q", rendered)
	}
}
