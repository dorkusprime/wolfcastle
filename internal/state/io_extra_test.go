package state

import (
	"os"
	"path/filepath"
	"testing"
)

func TestLoadNodeState_NormalizesNilAuditSlices(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	path := filepath.Join(dir, "state.json")

	// Write JSON with null audit slices
	json := `{
  "version": 1,
  "id": "test",
  "name": "Test",
  "type": "leaf",
  "state": "not_started",
  "audit": {
    "breadcrumbs": null,
    "gaps": null,
    "escalations": null,
    "status": ""
  }
}`
	if err := os.WriteFile(path, []byte(json), 0644); err != nil {
		t.Fatal(err)
	}

	ns, err := LoadNodeState(path)
	if err != nil {
		t.Fatal(err)
	}

	if ns.Audit.Breadcrumbs == nil {
		t.Error("Breadcrumbs should be non-nil after normalization")
	}
	if ns.Audit.Gaps == nil {
		t.Error("Gaps should be non-nil after normalization")
	}
	if ns.Audit.Escalations == nil {
		t.Error("Escalations should be non-nil after normalization")
	}
	if ns.Audit.Status != AuditPending {
		t.Errorf("expected status pending for empty status, got %s", ns.Audit.Status)
	}
}

func TestLoadNodeState_PreservesExistingAuditStatus(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	path := filepath.Join(dir, "state.json")

	json := `{
  "version": 1,
  "id": "test",
  "name": "Test",
  "type": "leaf",
  "state": "in_progress",
  "audit": {
    "breadcrumbs": [],
    "gaps": [],
    "escalations": [],
    "status": "in_progress"
  }
}`
	if err := os.WriteFile(path, []byte(json), 0644); err != nil {
		t.Fatal(err)
	}

	ns, err := LoadNodeState(path)
	if err != nil {
		t.Fatal(err)
	}

	if ns.Audit.Status != AuditInProgress {
		t.Errorf("expected status in_progress preserved, got %s", ns.Audit.Status)
	}
}

func TestSaveNodeState_CreatesDirectories(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	path := filepath.Join(dir, "sub", "deep", "state.json")

	ns := NewNodeState("n1", "Node", NodeLeaf)
	if err := SaveNodeState(path, ns); err != nil {
		t.Fatal(err)
	}

	if _, err := os.Stat(path); err != nil {
		t.Errorf("file should exist: %v", err)
	}
}
