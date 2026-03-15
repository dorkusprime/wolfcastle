package validate

import (
	"encoding/json"
	"strings"
	"testing"

	"github.com/dorkusprime/wolfcastle/internal/state"
)

// ── RecoverNodeState ─────────────────────────────────────────────────────

func TestRecoverNodeState_ValidJSON(t *testing.T) {
	ns := state.NewNodeState("node-1", "Test Node", state.NodeLeaf)
	ns.Tasks = []state.Task{
		{ID: "t1", Description: "do stuff", State: state.StatusComplete},
		{ID: "t2", Description: "do more", State: state.StatusInProgress},
	}
	data, _ := json.Marshal(ns)

	recovered, report, err := RecoverNodeState(data)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if recovered.ID != "node-1" {
		t.Errorf("expected ID node-1, got %s", recovered.ID)
	}
	if len(recovered.Tasks) != 2 {
		t.Errorf("expected 2 tasks, got %d", len(recovered.Tasks))
	}
	if len(report.Lost) != 0 {
		t.Errorf("expected no losses, got %v", report.Lost)
	}
}

func TestRecoverNodeState_EmptyFile(t *testing.T) {
	recovered, report, err := RecoverNodeState([]byte{})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if recovered.Version != 1 {
		t.Errorf("expected version 1, got %d", recovered.Version)
	}
	if recovered.State != state.StatusNotStarted {
		t.Errorf("expected not_started, got %s", recovered.State)
	}
	if len(report.Applied) == 0 {
		t.Error("expected at least one applied step for empty file")
	}
}

func TestRecoverNodeState_WhitespaceOnly(t *testing.T) {
	recovered, _, err := RecoverNodeState([]byte("   \n\t  "))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if recovered.Version != 1 {
		t.Errorf("expected version 1 for whitespace-only file, got %d", recovered.Version)
	}
}

func TestRecoverNodeState_NullBytes(t *testing.T) {
	ns := state.NewNodeState("node-2", "Null Test", state.NodeLeaf)
	data, _ := json.Marshal(ns)
	// Sprinkle null bytes into the JSON.
	corrupted := make([]byte, 0, len(data)*2)
	for _, b := range data {
		corrupted = append(corrupted, b, 0)
	}

	recovered, report, err := RecoverNodeState(corrupted)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if recovered.ID != "node-2" {
		t.Errorf("expected ID node-2, got %s", recovered.ID)
	}
	found := false
	for _, step := range report.Applied {
		if strings.Contains(step, "null") {
			found = true
		}
	}
	if !found {
		t.Error("expected null byte stripping to be reported")
	}
}

func TestRecoverNodeState_BOM(t *testing.T) {
	ns := state.NewNodeState("node-bom", "BOM Test", state.NodeLeaf)
	data, _ := json.Marshal(ns)
	bom := append([]byte{0xEF, 0xBB, 0xBF}, data...)

	recovered, report, err := RecoverNodeState(bom)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if recovered.ID != "node-bom" {
		t.Errorf("expected ID node-bom, got %s", recovered.ID)
	}
	found := false
	for _, step := range report.Applied {
		if strings.Contains(step, "BOM") {
			found = true
		}
	}
	if !found {
		t.Error("expected BOM stripping to be reported")
	}
}

func TestRecoverNodeState_TrailingGarbage(t *testing.T) {
	ns := state.NewNodeState("node-trail", "Trailing", state.NodeLeaf)
	ns.Tasks = []state.Task{
		{ID: "t1", Description: "task one", State: state.StatusComplete},
	}
	data, _ := json.Marshal(ns)
	garbage := append(data, []byte(`\x00\x00garbage{{{not json`)...)

	recovered, report, err := RecoverNodeState(garbage)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if recovered.ID != "node-trail" {
		t.Errorf("expected ID node-trail, got %s", recovered.ID)
	}
	if len(recovered.Tasks) != 1 {
		t.Errorf("expected 1 task, got %d", len(recovered.Tasks))
	}
	hasStrip := false
	for _, step := range report.Applied {
		if strings.Contains(step, "trailing") {
			hasStrip = true
		}
	}
	if !hasStrip {
		t.Error("expected trailing garbage stripping to be reported")
	}
}

func TestRecoverNodeState_TruncatedSimple(t *testing.T) {
	// Truncate a valid JSON object mid-field.
	full := `{"version":1,"id":"node-trunc","name":"Truncated","type":"leaf","state":"not_started","tasks":[{"id":"task-`
	recovered, report, err := RecoverNodeState([]byte(full))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if recovered.ID != "node-trunc" {
		t.Errorf("expected ID node-trunc, got %s", recovered.ID)
	}
	if recovered.Name != "Truncated" {
		t.Errorf("expected Name Truncated, got %s", recovered.Name)
	}
	hasClose := false
	for _, step := range report.Applied {
		if strings.Contains(step, "truncated") || strings.Contains(step, "closed") {
			hasClose = true
		}
	}
	if !hasClose {
		t.Error("expected truncation repair to be reported")
	}
}

func TestRecoverNodeState_TruncatedAfterComma(t *testing.T) {
	// Truncation right after a comma, mid-array.
	full := `{"version":1,"id":"node-tc","name":"TC","type":"leaf","state":"in_progress","tasks":[{"id":"t1","description":"ok","state":"complete","failure_count":0,"is_audit":false},`
	recovered, _, err := RecoverNodeState([]byte(full))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if recovered.ID != "node-tc" {
		t.Errorf("expected ID node-tc, got %s", recovered.ID)
	}
	// The first task should survive.
	if len(recovered.Tasks) < 1 {
		t.Errorf("expected at least 1 task to survive, got %d", len(recovered.Tasks))
	}
}

func TestRecoverNodeState_CompletelyUnrecoverable(t *testing.T) {
	_, _, err := RecoverNodeState([]byte("this is not json at all"))
	if err == nil {
		t.Error("expected error for completely unrecoverable data")
	}
}

func TestRecoverNodeState_LossDetection(t *testing.T) {
	// Build valid JSON for a node with 5 tasks, then serialize it and
	// truncate after the second task's closing brace plus comma, leaving
	// the raw bytes with 5 "id" fields in the tasks region but only 2
	// recoverable task objects.
	ns := &state.NodeState{
		Version: 1,
		ID:      "node-loss",
		Name:    "Loss",
		Type:    state.NodeLeaf,
		State:   state.StatusInProgress,
		Tasks: []state.Task{
			{ID: "t1", Description: "first", State: state.StatusComplete},
			{ID: "t2", Description: "second", State: state.StatusComplete},
			{ID: "t3", Description: "third", State: state.StatusComplete},
			{ID: "t4", Description: "fourth", State: state.StatusComplete},
			{ID: "t5", Description: "fifth", State: state.StatusComplete},
		},
	}
	data, _ := json.Marshal(ns)
	raw := string(data)

	// Find the start of t3's object and truncate mid-key.
	t3Idx := strings.Index(raw, `"id":"t3"`)
	if t3Idx < 0 {
		t.Fatal("could not find t3 in serialized JSON")
	}
	truncated := raw[:t3Idx+4] // keeps `"id":` and cuts off

	recovered, report, err := RecoverNodeState([]byte(truncated))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(recovered.Tasks) >= 5 {
		t.Errorf("expected fewer than 5 tasks, got %d", len(recovered.Tasks))
	}
	if len(report.Lost) == 0 {
		t.Error("expected loss report when tasks are missing")
	}
}

// ── RecoverRootIndex ─────────────────────────────────────────────────────

func TestRecoverRootIndex_ValidJSON(t *testing.T) {
	idx := state.NewRootIndex()
	idx.Nodes["a"] = state.IndexEntry{Name: "Alpha", Type: state.NodeLeaf, State: state.StatusNotStarted, Address: "a"}
	data, _ := json.Marshal(idx)

	recovered, _, err := RecoverRootIndex(data)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(recovered.Nodes) != 1 {
		t.Errorf("expected 1 node, got %d", len(recovered.Nodes))
	}
}

func TestRecoverRootIndex_EmptyFile(t *testing.T) {
	recovered, report, err := RecoverRootIndex([]byte{})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if recovered.Version != 1 {
		t.Errorf("expected version 1, got %d", recovered.Version)
	}
	if recovered.Nodes == nil {
		t.Error("expected non-nil Nodes map")
	}
	if len(report.Applied) == 0 {
		t.Error("expected at least one applied step")
	}
}

func TestRecoverRootIndex_TrailingGarbage(t *testing.T) {
	idx := state.NewRootIndex()
	idx.Nodes["b"] = state.IndexEntry{Name: "Beta", Type: state.NodeLeaf, State: state.StatusComplete, Address: "b"}
	data, _ := json.Marshal(idx)
	garbage := append(data, []byte(`}}}}extra`)...)

	recovered, _, err := RecoverRootIndex(garbage)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if _, ok := recovered.Nodes["b"]; !ok {
		t.Error("expected node 'b' to survive")
	}
}

func TestRecoverRootIndex_BOMAndNullBytes(t *testing.T) {
	idx := state.NewRootIndex()
	data, _ := json.Marshal(idx)
	// BOM + null bytes.
	corrupted := append([]byte{0xEF, 0xBB, 0xBF}, data...)
	corrupted = append(corrupted[:5], append([]byte{0, 0}, corrupted[5:]...)...)

	recovered, report, err := RecoverRootIndex(corrupted)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if recovered.Nodes == nil {
		t.Error("expected non-nil Nodes")
	}
	bomFound := false
	nullFound := false
	for _, step := range report.Applied {
		if strings.Contains(step, "BOM") {
			bomFound = true
		}
		if strings.Contains(step, "null") {
			nullFound = true
		}
	}
	if !bomFound {
		t.Error("expected BOM step")
	}
	if !nullFound {
		t.Error("expected null byte step")
	}
}

func TestRecoverRootIndex_Truncated(t *testing.T) {
	full := `{"version":1,"nodes":{"a":{"name":"Alpha","type":"leaf","state":"not_started","address":"a","decomposition_depth":0`
	recovered, _, err := RecoverRootIndex([]byte(full))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if _, ok := recovered.Nodes["a"]; !ok {
		t.Error("expected node 'a' to be recovered from truncated index")
	}
}

// ── sanitizeJSON unit tests ──────────────────────────────────────────────

func TestSanitizeJSON_NoChanges(t *testing.T) {
	input := []byte(`{"hello":"world"}`)
	out, steps := sanitizeJSON(input)
	if string(out) != string(input) {
		t.Errorf("expected no change, got %q", out)
	}
	if len(steps) != 0 {
		t.Errorf("expected no steps, got %v", steps)
	}
}

func TestSanitizeJSON_AllTransforms(t *testing.T) {
	// BOM + null bytes + whitespace.
	input := append([]byte{0xEF, 0xBB, 0xBF}, []byte("  {\"a\": \x00 1}  ")...)
	out, steps := sanitizeJSON(input)
	if strings.Contains(string(out), "\x00") {
		t.Error("null bytes should be stripped")
	}
	if len(steps) < 2 {
		t.Errorf("expected at least 2 steps (BOM + null), got %d: %v", len(steps), steps)
	}
}
