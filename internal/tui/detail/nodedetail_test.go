package detail

import (
	"strings"
	"testing"
	"time"

	"github.com/dorkusprime/wolfcastle/internal/state"
)

func TestNewNodeDetailModel_Defaults(t *testing.T) {
	t.Parallel()
	m := NewNodeDetailModel()
	if m.addr != "" {
		t.Errorf("addr should be empty, got %q", m.addr)
	}
	if m.node != nil {
		t.Error("node should be nil")
	}
	if m.readErr {
		t.Error("readErr should be false")
	}
	if m.isTarget {
		t.Error("isTarget should be false")
	}
}

func makeOrchestratorNode() *state.NodeState {
	return &state.NodeState{
		Version:            1,
		ID:                 "root",
		Name:               "My Project",
		Type:               state.NodeOrchestrator,
		State:              state.StatusInProgress,
		DecompositionDepth: 2,
		Children: []state.ChildRef{
			{ID: "child-a", Address: "root/child-a", State: state.StatusComplete},
			{ID: "child-b", Address: "root/child-b", State: state.StatusInProgress},
		},
	}
}

func makeLeafNode() *state.NodeState {
	return &state.NodeState{
		Version:            1,
		ID:                 "leaf-1",
		Name:               "Auth Module",
		Type:               state.NodeLeaf,
		State:              state.StatusNotStarted,
		DecompositionDepth: 0,
		Tasks: []state.Task{
			{ID: "task-0001", Title: "Write handler", State: state.StatusComplete},
			{ID: "task-0002", Description: "Add tests", State: state.StatusNotStarted},
		},
	}
}

func TestLoad_OrchestratorNode(t *testing.T) {
	t.Parallel()
	m := NewNodeDetailModel()
	m.SetSize(80, 40)
	node := makeOrchestratorNode()
	m.Load("root", node, nil, false)

	view := m.View()
	if !strings.Contains(view, "My Project") {
		t.Errorf("view should contain node name, got %q", view)
	}
	if !strings.Contains(view, "orchestrator") {
		t.Errorf("view should contain type, got %q", view)
	}
	if !strings.Contains(view, "Children") {
		t.Errorf("orchestrator view should contain Children section, got %q", view)
	}
	if !strings.Contains(view, "child-a") {
		t.Errorf("view should list children, got %q", view)
	}
	if !strings.Contains(view, "child-b") {
		t.Errorf("view should list children, got %q", view)
	}
}

func TestLoad_LeafNode(t *testing.T) {
	t.Parallel()
	m := NewNodeDetailModel()
	m.SetSize(80, 40)
	node := makeLeafNode()
	m.Load("root/leaf-1", node, nil, false)

	view := m.View()
	if !strings.Contains(view, "Auth Module") {
		t.Errorf("view should contain node name, got %q", view)
	}
	if !strings.Contains(view, "Tasks") {
		t.Errorf("leaf view should contain Tasks section, got %q", view)
	}
	if !strings.Contains(view, "task-0001") {
		t.Errorf("view should list tasks, got %q", view)
	}
	if !strings.Contains(view, "Write handler") {
		t.Errorf("view should show task title, got %q", view)
	}
	// task-0002 has no title, should fall back to description
	if !strings.Contains(view, "Add tests") {
		t.Errorf("view should fall back to description when title empty, got %q", view)
	}
}

func TestLoad_WithAuditData(t *testing.T) {
	t.Parallel()
	m := NewNodeDetailModel()
	m.SetSize(80, 40)

	started := time.Now().Add(-5 * time.Minute)
	completed := time.Now().Add(-1 * time.Minute)
	node := &state.NodeState{
		Version: 1,
		ID:      "n1",
		Name:    "Audited Node",
		Type:    state.NodeLeaf,
		State:   state.StatusComplete,
		Audit: state.AuditState{
			Status:        state.AuditPassed,
			StartedAt:     &started,
			CompletedAt:   &completed,
			ResultSummary: "All checks passed",
			Gaps: []state.Gap{
				{ID: "g1", Status: state.GapOpen},
				{ID: "g2", Status: state.GapFixed},
				{ID: "g3", Status: state.GapFixed},
			},
			Escalations: []state.Escalation{
				{ID: "e1", Status: state.EscalationOpen},
				{ID: "e2", Status: state.EscalationResolved},
			},
		},
	}
	m.Load("root/n1", node, nil, false)

	view := m.View()
	if !strings.Contains(view, "Audit") {
		t.Errorf("view should contain Audit section, got %q", view)
	}
	if !strings.Contains(view, "1 open") {
		t.Errorf("view should show open gaps count, got %q", view)
	}
	if !strings.Contains(view, "2 fixed") {
		t.Errorf("view should show fixed gaps count, got %q", view)
	}
	if !strings.Contains(view, "1 open") {
		t.Errorf("view should show open escalations, got %q", view)
	}
	if !strings.Contains(view, "All checks passed") {
		t.Errorf("view should show result summary, got %q", view)
	}
}

func TestLoad_WithScope(t *testing.T) {
	t.Parallel()
	m := NewNodeDetailModel()
	m.SetSize(80, 40)

	node := &state.NodeState{
		Version: 1,
		ID:      "n1",
		Name:    "Scoped Node",
		Type:    state.NodeOrchestrator,
		State:   state.StatusInProgress,
		Scope:   "Build the entire authentication system including JWT and session management",
	}
	m.Load("root/n1", node, nil, false)

	view := m.View()
	if !strings.Contains(view, "Scope") {
		t.Errorf("view should contain Scope section, got %q", view)
	}
	if !strings.Contains(view, "authentication") {
		t.Errorf("view should contain scope text, got %q", view)
	}
}

func TestLoad_WithCriteria(t *testing.T) {
	t.Parallel()
	m := NewNodeDetailModel()
	m.SetSize(80, 40)

	node := &state.NodeState{
		Version:         1,
		ID:              "n1",
		Name:            "Criteria Node",
		Type:            state.NodeOrchestrator,
		State:           state.StatusInProgress,
		SuccessCriteria: []string{"All tests pass", "Coverage > 90%"},
	}
	m.Load("root/n1", node, nil, false)

	view := m.View()
	if !strings.Contains(view, "Success Criteria") {
		t.Errorf("view should contain Success Criteria section, got %q", view)
	}
	if !strings.Contains(view, "All tests pass") {
		t.Errorf("view should show criteria items, got %q", view)
	}
	if !strings.Contains(view, "Coverage > 90%") {
		t.Errorf("view should show all criteria items, got %q", view)
	}
}

func TestLoad_WithSpecs(t *testing.T) {
	t.Parallel()
	m := NewNodeDetailModel()
	m.SetSize(80, 40)

	node := &state.NodeState{
		Version: 1,
		ID:      "n1",
		Name:    "Spec Node",
		Type:    state.NodeLeaf,
		State:   state.StatusNotStarted,
		Specs:   []string{"docs/spec-auth.md", "docs/spec-api.md"},
	}
	m.Load("root/n1", node, nil, false)

	view := m.View()
	if !strings.Contains(view, "Specs") {
		t.Errorf("view should contain Specs section, got %q", view)
	}
	if !strings.Contains(view, "docs/spec-auth.md") {
		t.Errorf("view should list specs, got %q", view)
	}
}

func TestLoad_IsTarget(t *testing.T) {
	t.Parallel()
	m := NewNodeDetailModel()
	m.SetSize(80, 40)

	node := &state.NodeState{
		Version: 1,
		ID:      "n1",
		Name:    "Target Node",
		Type:    state.NodeLeaf,
		State:   state.StatusInProgress,
	}
	m.Load("root/n1", node, nil, true)

	view := m.View()
	if !strings.Contains(view, "▶") {
		t.Errorf("target node should show ▶ prefix, got %q", view)
	}
}

func TestLoad_NotTarget(t *testing.T) {
	t.Parallel()
	m := NewNodeDetailModel()
	m.SetSize(80, 40)

	node := &state.NodeState{
		Version: 1,
		ID:      "n1",
		Name:    "Normal Node",
		Type:    state.NodeLeaf,
		State:   state.StatusInProgress,
	}
	m.Load("root/n1", node, nil, false)

	view := m.View()
	if strings.Contains(view, "▶") {
		t.Errorf("non-target node should not show ▶ prefix, got %q", view)
	}
}

func TestSetReadError(t *testing.T) {
	t.Parallel()
	m := NewNodeDetailModel()
	m.SetSize(80, 40)
	m.SetReadError("root/broken")

	view := m.View()
	if !strings.Contains(view, "Intelligence unavailable") {
		t.Errorf("error view should contain unavailable message, got %q", view)
	}
	if !strings.Contains(view, "root/broken") {
		t.Errorf("error view should contain address, got %q", view)
	}
	if m.addr != "root/broken" {
		t.Errorf("addr should be set to 'root/broken', got %q", m.addr)
	}
}

func TestAddr(t *testing.T) {
	t.Parallel()
	m := NewNodeDetailModel()
	m.SetSize(80, 40)
	node := makeOrchestratorNode()
	m.Load("root/proj", node, nil, false)

	if m.Addr() != "root/proj" {
		t.Errorf("Addr() should return 'root/proj', got %q", m.Addr())
	}
}

func TestSetSize_Propagates(t *testing.T) {
	t.Parallel()
	m := NewNodeDetailModel()
	m.SetSize(120, 50)

	if m.width != 120 {
		t.Errorf("expected width 120, got %d", m.width)
	}
	if m.height != 50 {
		t.Errorf("expected height 50, got %d", m.height)
	}
}

func TestRelativeTime_JustNow(t *testing.T) {
	t.Parallel()
	result := relativeTime(time.Now())
	if result != "just now" {
		t.Errorf("expected 'just now', got %q", result)
	}
}

func TestRelativeTime_Seconds(t *testing.T) {
	t.Parallel()
	result := relativeTime(time.Now().Add(-30 * time.Second))
	if !strings.Contains(result, "s ago") {
		t.Errorf("expected seconds ago format, got %q", result)
	}
}

func TestRelativeTime_Minutes(t *testing.T) {
	t.Parallel()
	result := relativeTime(time.Now().Add(-5 * time.Minute))
	if result != "5m ago" {
		t.Errorf("expected '5m ago', got %q", result)
	}
}

func TestRelativeTime_OneMinute(t *testing.T) {
	t.Parallel()
	result := relativeTime(time.Now().Add(-1*time.Minute - 10*time.Second))
	if result != "1m ago" {
		t.Errorf("expected '1m ago', got %q", result)
	}
}

func TestRelativeTime_Hours(t *testing.T) {
	t.Parallel()
	result := relativeTime(time.Now().Add(-3 * time.Hour))
	if !strings.Contains(result, "3h ago") {
		t.Errorf("expected '3h ago (HH:MM:SS)' format, got %q", result)
	}
	if !strings.Contains(result, "(") {
		t.Errorf("hours format should contain exact time in parens, got %q", result)
	}
}

func TestRelativeTime_OneHour(t *testing.T) {
	t.Parallel()
	result := relativeTime(time.Now().Add(-1*time.Hour - 10*time.Minute))
	if !strings.Contains(result, "1h ago") {
		t.Errorf("expected '1h ago' format, got %q", result)
	}
}

func TestRelativeTime_Days(t *testing.T) {
	t.Parallel()
	result := relativeTime(time.Now().Add(-48 * time.Hour))
	if !strings.Contains(result, "2d ago") {
		t.Errorf("expected '2d ago' format, got %q", result)
	}
}

func TestRelativeTime_OneDay(t *testing.T) {
	t.Parallel()
	result := relativeTime(time.Now().Add(-25 * time.Hour))
	if !strings.Contains(result, "1d ago") {
		t.Errorf("expected '1d ago' format, got %q", result)
	}
}

func TestRelativeTime_Future(t *testing.T) {
	t.Parallel()
	// Future times get clamped to 0 duration
	result := relativeTime(time.Now().Add(1 * time.Hour))
	if result != "just now" {
		t.Errorf("future time should show 'just now', got %q", result)
	}
}

func TestEmptySections_Omitted(t *testing.T) {
	t.Parallel()
	m := NewNodeDetailModel()
	m.SetSize(80, 40)

	// A minimal node with no scope, criteria, children, tasks, audit, specs
	node := &state.NodeState{
		Version: 1,
		ID:      "n1",
		Name:    "Minimal",
		Type:    state.NodeLeaf,
		State:   state.StatusNotStarted,
	}
	m.Load("root/n1", node, nil, false)

	view := m.View()
	if strings.Contains(view, "Scope") {
		t.Errorf("empty scope should be omitted, got %q", view)
	}
	if strings.Contains(view, "Success Criteria") {
		t.Errorf("empty criteria should be omitted, got %q", view)
	}
	if strings.Contains(view, "Children") {
		t.Errorf("leaf should not show Children, got %q", view)
	}
	if strings.Contains(view, "Tasks") {
		t.Errorf("empty tasks should be omitted, got %q", view)
	}
	if strings.Contains(view, "Specs") {
		t.Errorf("empty specs should be omitted, got %q", view)
	}
}

func TestAudit_InProgress_ShowsInProgress(t *testing.T) {
	t.Parallel()
	m := NewNodeDetailModel()
	m.SetSize(80, 40)

	started := time.Now().Add(-10 * time.Minute)
	node := &state.NodeState{
		Version: 1,
		ID:      "n1",
		Name:    "Auditing",
		Type:    state.NodeLeaf,
		State:   state.StatusInProgress,
		Audit: state.AuditState{
			Status:    state.AuditInProgress,
			StartedAt: &started,
		},
	}
	m.Load("root/n1", node, nil, false)

	view := m.View()
	if !strings.Contains(view, "in progress") {
		t.Errorf("in-progress audit should show completion status, got %q", view)
	}
}

func TestAudit_NoSummary_ShowsNoneYet(t *testing.T) {
	t.Parallel()
	m := NewNodeDetailModel()
	m.SetSize(80, 40)

	node := &state.NodeState{
		Version: 1,
		ID:      "n1",
		Name:    "Pending",
		Type:    state.NodeLeaf,
		State:   state.StatusNotStarted,
		Audit: state.AuditState{
			Status: state.AuditPending,
		},
	}
	m.Load("root/n1", node, nil, false)

	view := m.View()
	if !strings.Contains(view, "none yet") {
		t.Errorf("empty summary should show 'none yet', got %q", view)
	}
}

func TestUpdate_PassesToViewport(t *testing.T) {
	t.Parallel()
	m := NewNodeDetailModel()
	m.SetSize(80, 40)
	node := makeOrchestratorNode()
	m.Load("root", node, nil, false)

	// Passing a key press should not panic
	m, _ = m.Update(keyPress('j'))
}

func TestNodeDetail_Update_NonKeyMsg_Ignored(t *testing.T) {
	t.Parallel()
	m := NewNodeDetailModel()
	type customMsg struct{}
	_, _ = m.Update(customMsg{})
}

func TestCountGaps(t *testing.T) {
	t.Parallel()
	gaps := []state.Gap{
		{Status: state.GapOpen},
		{Status: state.GapOpen},
		{Status: state.GapFixed},
	}
	open, fixed := countGaps(gaps)
	if open != 2 {
		t.Errorf("expected 2 open, got %d", open)
	}
	if fixed != 1 {
		t.Errorf("expected 1 fixed, got %d", fixed)
	}
}

func TestCountGaps_Empty(t *testing.T) {
	t.Parallel()
	open, fixed := countGaps(nil)
	if open != 0 || fixed != 0 {
		t.Errorf("expected 0/0 for nil gaps, got %d/%d", open, fixed)
	}
}

func TestCountOpenEscalations(t *testing.T) {
	t.Parallel()
	escs := []state.Escalation{
		{Status: state.EscalationOpen},
		{Status: state.EscalationResolved},
		{Status: state.EscalationOpen},
	}
	if got := countOpenEscalations(escs); got != 2 {
		t.Errorf("expected 2 open escalations, got %d", got)
	}
}

func TestCountOpenEscalations_Empty(t *testing.T) {
	t.Parallel()
	if got := countOpenEscalations(nil); got != 0 {
		t.Errorf("expected 0 for nil escalations, got %d", got)
	}
}

func TestWrapIndent(t *testing.T) {
	t.Parallel()
	result := wrapIndent("hello world this is a test", 20, "  ")
	if !strings.Contains(result, "  ") {
		t.Errorf("wrapped text should contain indent, got %q", result)
	}
	lines := strings.Split(result, "\n")
	if len(lines) < 2 {
		t.Errorf("text should wrap to multiple lines at width 20, got %d lines", len(lines))
	}
}

func TestWrapIndent_Empty(t *testing.T) {
	t.Parallel()
	result := wrapIndent("", 80, "  ")
	if result != "" {
		t.Errorf("empty text should produce empty result, got %q", result)
	}
}

func TestWrapIndent_NarrowWidth(t *testing.T) {
	t.Parallel()
	// Width so small that usable < 20 triggers the minimum
	result := wrapIndent("hello world", 5, "  ")
	if result == "" {
		t.Error("narrow width should still produce output")
	}
}

func TestRebuildContent_NilNode(t *testing.T) {
	t.Parallel()
	m := NewNodeDetailModel()
	m.SetSize(80, 40)
	// rebuildContent with nil node should not panic
	m.rebuildContent()
}

func TestRebuildContent_ReadError(t *testing.T) {
	t.Parallel()
	m := NewNodeDetailModel()
	m.SetSize(80, 40)
	m.readErr = true
	// rebuildContent with readErr should bail early without panic
	m.rebuildContent()
}

func TestSetSize_SmallWidth(t *testing.T) {
	t.Parallel()
	m := NewNodeDetailModel()
	node := makeOrchestratorNode()
	node.Scope = "Some scope text that needs wrapping"
	m.SetSize(10, 40)
	m.Load("root", node, nil, false)
	// The wrapWidth < 20 guard should use 80 as default.
	// At small viewport widths the rendered output is truncated by the
	// viewport itself, but the content should still be generated without panic.
	view := m.View()
	if view == "" {
		t.Error("small width should still produce output")
	}
	if !strings.Contains(view, "Scope") {
		t.Errorf("small width should still render Scope heading, got %q", view)
	}
}
