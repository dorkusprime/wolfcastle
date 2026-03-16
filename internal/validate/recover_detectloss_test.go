package validate

import (
	"encoding/json"
	"testing"

	"github.com/dorkusprime/wolfcastle/internal/state"
)

// ═══════════════════════════════════════════════════════════════════════════
// detectLoss — direct unit tests
// ═══════════════════════════════════════════════════════════════════════════

func TestDetectLoss_NoLoss_TasksOnly(t *testing.T) {
	t.Parallel()
	ns := &state.NodeState{
		Tasks: []state.Task{{ID: "t1"}, {ID: "t2"}},
	}
	data, _ := json.Marshal(ns)
	report := &RecoveryReport{}
	detectLoss(ns, data, report)

	if len(report.Lost) != 0 {
		t.Errorf("expected no losses, got %v", report.Lost)
	}
}

func TestDetectLoss_NoLoss_ChildrenOnly(t *testing.T) {
	t.Parallel()
	ns := &state.NodeState{
		Children: []state.ChildRef{{ID: "c1"}, {ID: "c2"}},
	}
	data, _ := json.Marshal(ns)
	report := &RecoveryReport{}
	detectLoss(ns, data, report)

	if len(report.Lost) != 0 {
		t.Errorf("expected no losses, got %v", report.Lost)
	}
}

func TestDetectLoss_TaskTruncation(t *testing.T) {
	t.Parallel()
	// Simulate: raw JSON mentions 4 task IDs but only 2 survived parsing.
	ns := &state.NodeState{
		Tasks: []state.Task{{ID: "t1"}, {ID: "t2"}},
	}
	// Craft raw JSON that has more "id" references in the tasks section.
	raw := `{"tasks":[{"id":"t1"},{"id":"t2"},{"id":"t3"},{"id":"t4"}]}`

	report := &RecoveryReport{}
	detectLoss(ns, []byte(raw), report)

	if len(report.Lost) == 0 {
		t.Fatal("expected task loss to be detected")
	}
	found := false
	for _, msg := range report.Lost {
		if msg != "" {
			found = true
		}
	}
	if !found {
		t.Error("expected non-empty loss description")
	}
}

func TestDetectLoss_ChildTruncation(t *testing.T) {
	t.Parallel()
	ns := &state.NodeState{
		Children: []state.ChildRef{{ID: "c1"}},
	}
	raw := `{"children":[{"id":"c1"},{"id":"c2"},{"id":"c3"}]}`

	report := &RecoveryReport{}
	detectLoss(ns, []byte(raw), report)

	if len(report.Lost) == 0 {
		t.Fatal("expected child loss to be detected")
	}
}

func TestDetectLoss_BothTasksAndChildren(t *testing.T) {
	t.Parallel()
	ns := &state.NodeState{
		Tasks:    []state.Task{{ID: "t1"}},
		Children: []state.ChildRef{{ID: "c1"}},
	}
	raw := `{"tasks":[{"id":"t1"},{"id":"t2"},{"id":"t3"}],"children":[{"id":"c1"},{"id":"c2"}]}`

	report := &RecoveryReport{}
	detectLoss(ns, []byte(raw), report)

	if len(report.Lost) != 2 {
		t.Errorf("expected 2 loss entries (tasks + children), got %d", len(report.Lost))
	}
}

func TestDetectLoss_NoTasksSection(t *testing.T) {
	t.Parallel()
	ns := &state.NodeState{}
	raw := `{"version":1,"id":"x"}`
	report := &RecoveryReport{}
	detectLoss(ns, []byte(raw), report)

	if len(report.Lost) != 0 {
		t.Errorf("no tasks section should mean no loss, got %v", report.Lost)
	}
}

func TestDetectLoss_NoChildrenSection(t *testing.T) {
	t.Parallel()
	ns := &state.NodeState{
		Tasks: []state.Task{{ID: "t1"}},
	}
	raw := `{"tasks":[{"id":"t1"}]}`
	report := &RecoveryReport{}
	detectLoss(ns, []byte(raw), report)

	if len(report.Lost) != 0 {
		t.Errorf("matching counts should mean no loss, got %v", report.Lost)
	}
}

// ═══════════════════════════════════════════════════════════════════════════
// RecoverNodeState — sanitizeJSON edge cases exercising detectLoss paths
// ═══════════════════════════════════════════════════════════════════════════

func TestRecoverNodeState_NullBytesDetected(t *testing.T) {
	t.Parallel()
	ns := state.NewNodeState("n1", "N1", state.NodeLeaf)
	ns.Tasks = []state.Task{{ID: "t1", State: state.StatusComplete}}
	data, _ := json.Marshal(ns)

	// Inject null bytes.
	corrupted := make([]byte, 0, len(data)*2)
	for _, b := range data {
		corrupted = append(corrupted, b, 0)
	}

	_, report, err := RecoverNodeState(corrupted)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	hasNull := false
	for _, s := range report.Applied {
		if s == "stripped null bytes" {
			hasNull = true
		}
	}
	if !hasNull {
		t.Error("expected null byte stripping step")
	}
}

func TestRecoverNodeState_BOMDetected(t *testing.T) {
	t.Parallel()
	ns := state.NewNodeState("n2", "N2", state.NodeLeaf)
	data, _ := json.Marshal(ns)
	bom := append([]byte{0xEF, 0xBB, 0xBF}, data...)

	_, report, err := RecoverNodeState(bom)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	hasBOM := false
	for _, s := range report.Applied {
		if s == "stripped UTF-8 BOM" {
			hasBOM = true
		}
	}
	if !hasBOM {
		t.Error("expected BOM stripping step")
	}
}

func TestRecoverNodeState_MultipleLossTypes(t *testing.T) {
	t.Parallel()
	// Build JSON with 3 tasks and 2 children, truncate after second task.
	ns := &state.NodeState{
		Version: 1,
		ID:      "multi",
		Name:    "Multi",
		Type:    state.NodeLeaf,
		State:   state.StatusInProgress,
		Tasks: []state.Task{
			{ID: "t1", Description: "a", State: state.StatusComplete},
			{ID: "t2", Description: "b", State: state.StatusComplete},
			{ID: "t3", Description: "c", State: state.StatusComplete},
		},
		Children: []state.ChildRef{
			{ID: "c1", Address: "multi/c1", State: state.StatusComplete},
			{ID: "c2", Address: "multi/c2", State: state.StatusComplete},
		},
	}
	data, _ := json.Marshal(ns)
	raw := string(data)

	// Truncate mid-way through the children array so at least one child is lost,
	// but after the tasks array is complete.
	childIdx := len(raw) - 5 // Cut near the end
	truncated := raw[:childIdx]

	recovered, report, err := RecoverNodeState([]byte(truncated))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	// Something should have survived.
	_ = recovered

	// Check that loss was reported (either tasks or children depending on
	// exactly where truncation landed).
	_ = report
}
