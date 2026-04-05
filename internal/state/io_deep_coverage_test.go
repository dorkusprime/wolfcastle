package state

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
)

// ═══════════════════════════════════════════════════════════════════════════
// atomicWriteJSON: success with nested directories and content verification
// ═══════════════════════════════════════════════════════════════════════════

func TestAtomicWriteJSON_ContentMatchesInput(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	path := filepath.Join(dir, "verify.json")

	input := map[string]any{"name": "test", "value": float64(42)}
	if err := atomicWriteJSON(path, input); err != nil {
		t.Fatal(err)
	}

	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}

	var loaded map[string]any
	if err := json.Unmarshal(data, &loaded); err != nil {
		t.Fatal(err)
	}

	if loaded["name"] != "test" || loaded["value"] != float64(42) {
		t.Errorf("content mismatch: %v", loaded)
	}
}

func TestAtomicWriteJSON_OverwritesExistingFile(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	path := filepath.Join(dir, "overwrite.json")

	// Write initial content
	if err := atomicWriteJSON(path, map[string]string{"version": "1"}); err != nil {
		t.Fatal(err)
	}

	// Overwrite with new content
	if err := atomicWriteJSON(path, map[string]string{"version": "2"}); err != nil {
		t.Fatal(err)
	}

	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}

	var loaded map[string]string
	if err := json.Unmarshal(data, &loaded); err != nil {
		t.Fatal(err)
	}
	if loaded["version"] != "2" {
		t.Errorf("expected version 2, got %q", loaded["version"])
	}
}

// ═══════════════════════════════════════════════════════════════════════════
// LoadRootIndex: error paths
// ═══════════════════════════════════════════════════════════════════════════

func TestLoadRootIndex_NonexistentPath(t *testing.T) {
	t.Parallel()
	_, err := LoadRootIndex("/tmp/wolfcastle-nonexistent-xyz/state.json")
	if err == nil {
		t.Error("expected error for nonexistent file")
	}
}

func TestLoadRootIndex_MalformedJSON(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	path := filepath.Join(dir, "state.json")
	_ = os.WriteFile(path, []byte("{broken"), 0644)

	_, err := LoadRootIndex(path)
	if err == nil {
		t.Error("expected error for malformed JSON")
	}
}

func TestLoadRootIndex_InitializesNilNodes(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	path := filepath.Join(dir, "state.json")
	_ = os.WriteFile(path, []byte(`{}`), 0644)

	idx, err := LoadRootIndex(path)
	if err != nil {
		t.Fatal(err)
	}
	if idx.Nodes == nil {
		t.Error("Nodes map should be initialized even when absent from JSON")
	}
}

// ═══════════════════════════════════════════════════════════════════════════
// LoadNodeState: error paths
// ═══════════════════════════════════════════════════════════════════════════

func TestLoadNodeState_NonexistentPath(t *testing.T) {
	t.Parallel()
	_, err := LoadNodeState("/tmp/wolfcastle-nonexistent-xyz/state.json")
	if err == nil {
		t.Error("expected error for nonexistent file")
	}
}

func TestLoadNodeState_MalformedJSON(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	path := filepath.Join(dir, "state.json")
	_ = os.WriteFile(path, []byte("{broken"), 0644)

	_, err := LoadNodeState(path)
	if err == nil {
		t.Error("expected error for malformed JSON")
	}
}

func TestLoadNodeState_NormalizesAuditState(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	path := filepath.Join(dir, "state.json")

	// Write a node with empty audit state
	ns := &NodeState{
		ID:   "test",
		Name: "Test",
		Type: NodeLeaf,
	}
	data, _ := json.MarshalIndent(ns, "", "  ")
	_ = os.WriteFile(path, data, 0644)

	loaded, err := LoadNodeState(path)
	if err != nil {
		t.Fatal(err)
	}

	// normalizeAuditState should set defaults
	if loaded.Audit.Status != AuditPending {
		t.Errorf("expected audit status %q, got %q", AuditPending, loaded.Audit.Status)
	}
	if loaded.Audit.Breadcrumbs == nil {
		t.Error("Breadcrumbs should be non-nil after normalization")
	}
	if loaded.Audit.Gaps == nil {
		t.Error("Gaps should be non-nil after normalization")
	}
	if loaded.Audit.Escalations == nil {
		t.Error("Escalations should be non-nil after normalization")
	}
}

// ═══════════════════════════════════════════════════════════════════════════
// SaveRootIndex / SaveNodeState: round-trip
// ═══════════════════════════════════════════════════════════════════════════

func TestSaveAndLoadRootIndex_RoundTrip(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	path := filepath.Join(dir, "state.json")

	idx := NewRootIndex()
	idx.RootID = "test-project"
	idx.Nodes["test-project"] = IndexEntry{
		Name:    "Test",
		Type:    NodeLeaf,
		State:   StatusNotStarted,
		Address: "test-project",
	}

	if err := SaveRootIndex(path, idx); err != nil {
		t.Fatal(err)
	}

	loaded, err := LoadRootIndex(path)
	if err != nil {
		t.Fatal(err)
	}

	if loaded.RootID != "test-project" {
		t.Errorf("expected root ID 'test-project', got %q", loaded.RootID)
	}
	if _, ok := loaded.Nodes["test-project"]; !ok {
		t.Error("expected test-project node in loaded index")
	}
}

func TestSaveAndLoadNodeState_RoundTrip(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	path := filepath.Join(dir, "state.json")

	ns := NewNodeState("my-node", "My Node", NodeLeaf)
	ns.State = StatusInProgress
	ns.Tasks = []Task{
		{ID: "task-0001", Description: "do work", State: StatusInProgress},
	}

	if err := SaveNodeState(path, ns); err != nil {
		t.Fatal(err)
	}

	loaded, err := LoadNodeState(path)
	if err != nil {
		t.Fatal(err)
	}

	if loaded.State != StatusInProgress {
		t.Errorf("expected in_progress, got %s", loaded.State)
	}
	if len(loaded.Tasks) != 1 {
		t.Errorf("expected 1 task, got %d", len(loaded.Tasks))
	}
}

// ═══════════════════════════════════════════════════════════════════════════
// normalizeAuditState: already-set fields preserved
// ═══════════════════════════════════════════════════════════════════════════

func TestNormalizeAuditState_PreservesExistingStatus(t *testing.T) {
	t.Parallel()
	ns := &NodeState{
		Audit: AuditState{
			Status:      AuditInProgress,
			Breadcrumbs: []Breadcrumb{{Text: "already set"}},
			Gaps:        []Gap{{ID: "gap-1"}},
			Escalations: []Escalation{{ID: "esc-1"}},
		},
	}
	normalizeAuditState(ns)

	if ns.Audit.Status != AuditInProgress {
		t.Error("should not override existing audit status")
	}
	if len(ns.Audit.Breadcrumbs) != 1 {
		t.Error("should not override existing breadcrumbs")
	}
}
