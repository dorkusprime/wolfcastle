package state

import (
	"os"
	"path/filepath"
	"testing"
	"time"
)

// ---------------------------------------------------------------------------
// LoadBatch
// ---------------------------------------------------------------------------

func TestLoadBatch_NonExistentFile(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	path := filepath.Join(dir, "no-such-file.json")

	b, err := LoadBatch(path)
	if err != nil {
		t.Fatalf("expected nil error, got %v", err)
	}
	if b != nil {
		t.Fatalf("expected nil batch, got %+v", b)
	}
}

func TestLoadBatch_ValidFile(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	path := filepath.Join(dir, "batch.json")

	content := `{
  "id": "batch-1",
  "timestamp": "2025-06-01T12:00:00Z",
  "scopes": ["scope-a"],
  "status": "pending",
  "findings": [
    {
      "id": "f1",
      "title": "Finding One",
      "status": "pending"
    }
  ]
}`
	if err := os.WriteFile(path, []byte(content), 0644); err != nil {
		t.Fatal(err)
	}

	b, err := LoadBatch(path)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if b.ID != "batch-1" {
		t.Errorf("expected id 'batch-1', got %q", b.ID)
	}
	if b.Status != BatchPending {
		t.Errorf("expected status 'pending', got %q", b.Status)
	}
	if len(b.Scopes) != 1 || b.Scopes[0] != "scope-a" {
		t.Errorf("expected scopes [scope-a], got %v", b.Scopes)
	}
	if len(b.Findings) != 1 {
		t.Fatalf("expected 1 finding, got %d", len(b.Findings))
	}
	if b.Findings[0].Title != "Finding One" {
		t.Errorf("expected finding title 'Finding One', got %q", b.Findings[0].Title)
	}
}

func TestLoadBatch_MalformedJSON(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	path := filepath.Join(dir, "bad.json")

	if err := os.WriteFile(path, []byte("{not json!"), 0644); err != nil {
		t.Fatal(err)
	}

	_, err := LoadBatch(path)
	if err == nil {
		t.Error("expected error for malformed JSON")
	}
}

// ---------------------------------------------------------------------------
// SaveBatch
// ---------------------------------------------------------------------------

func TestSaveBatch_WritesValidJSON(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	path := filepath.Join(dir, "batch.json")

	ts := time.Date(2025, 6, 1, 12, 0, 0, 0, time.UTC)
	b := &Batch{
		ID:        "batch-2",
		Timestamp: ts,
		Scopes:    []string{"s1", "s2"},
		Status:    BatchPending,
		Findings: []Finding{
			{ID: "f1", Title: "Alpha", Status: FindingPending},
		},
	}

	if err := SaveBatch(path, b); err != nil {
		t.Fatal(err)
	}

	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if len(data) == 0 {
		t.Fatal("file should not be empty")
	}
	// File should end with a trailing newline.
	if data[len(data)-1] != '\n' {
		t.Error("expected trailing newline")
	}
}

func TestSaveBatch_Roundtrip(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	path := filepath.Join(dir, "batch.json")

	ts := time.Date(2025, 7, 4, 9, 30, 0, 0, time.UTC)
	decidedAt := time.Date(2025, 7, 4, 10, 0, 0, 0, time.UTC)
	original := &Batch{
		ID:        "batch-rt",
		Timestamp: ts,
		Scopes:    []string{"scope-x"},
		Status:    BatchCompleted,
		Findings: []Finding{
			{ID: "f1", Title: "First", Description: "desc-1", Status: FindingApproved, DecidedAt: &decidedAt, CreatedNode: "node-1"},
			{ID: "f2", Title: "Second", Status: FindingRejected},
		},
		RawOutput: "raw output here",
	}

	if err := SaveBatch(path, original); err != nil {
		t.Fatal(err)
	}

	loaded, err := LoadBatch(path)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if loaded.ID != original.ID {
		t.Errorf("expected id %q, got %q", original.ID, loaded.ID)
	}
	if loaded.Status != original.Status {
		t.Errorf("expected status %q, got %q", original.Status, loaded.Status)
	}
	if loaded.RawOutput != original.RawOutput {
		t.Errorf("expected raw_output %q, got %q", original.RawOutput, loaded.RawOutput)
	}
	if len(loaded.Findings) != 2 {
		t.Fatalf("expected 2 findings, got %d", len(loaded.Findings))
	}
	if loaded.Findings[0].Description != "desc-1" {
		t.Errorf("expected description 'desc-1', got %q", loaded.Findings[0].Description)
	}
	if loaded.Findings[0].DecidedAt == nil || !loaded.Findings[0].DecidedAt.Equal(decidedAt) {
		t.Errorf("expected decided_at %v, got %v", decidedAt, loaded.Findings[0].DecidedAt)
	}
	if loaded.Findings[0].CreatedNode != "node-1" {
		t.Errorf("expected created_node 'node-1', got %q", loaded.Findings[0].CreatedNode)
	}
}

// ---------------------------------------------------------------------------
// RemoveBatch
// ---------------------------------------------------------------------------

func TestRemoveBatch_RemovesFile(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	path := filepath.Join(dir, "batch.json")

	if err := os.WriteFile(path, []byte(`{}`), 0644); err != nil {
		t.Fatal(err)
	}

	if err := RemoveBatch(path); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if _, err := os.Stat(path); !os.IsNotExist(err) {
		t.Error("file should have been removed")
	}
}

func TestRemoveBatch_NoErrorIfMissing(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	path := filepath.Join(dir, "nonexistent.json")

	if err := RemoveBatch(path); err != nil {
		t.Fatalf("expected no error for missing file, got %v", err)
	}
}

// ---------------------------------------------------------------------------
// LoadHistory
// ---------------------------------------------------------------------------

func TestLoadHistory_NonExistentFile(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	path := filepath.Join(dir, "no-history.json")

	h, err := LoadHistory(path)
	if err != nil {
		t.Fatalf("expected nil error, got %v", err)
	}
	if h == nil {
		t.Fatal("expected non-nil History")
	}
	if len(h.Entries) != 0 {
		t.Errorf("expected empty entries, got %d", len(h.Entries))
	}
}

func TestLoadHistory_ValidFile(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	path := filepath.Join(dir, "history.json")

	content := `{
  "entries": [
    {
      "batch_id": "b1",
      "completed_at": "2025-05-01T08:00:00Z",
      "scopes": ["s1"],
      "decisions": [
        {"finding_id": "f1", "title": "T1", "action": "approved", "timestamp": "2025-05-01T08:01:00Z"}
      ]
    }
  ]
}`
	if err := os.WriteFile(path, []byte(content), 0644); err != nil {
		t.Fatal(err)
	}

	h, err := LoadHistory(path)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(h.Entries) != 1 {
		t.Fatalf("expected 1 entry, got %d", len(h.Entries))
	}
	if h.Entries[0].BatchID != "b1" {
		t.Errorf("expected batch_id 'b1', got %q", h.Entries[0].BatchID)
	}
	if len(h.Entries[0].Decisions) != 1 {
		t.Fatalf("expected 1 decision, got %d", len(h.Entries[0].Decisions))
	}
	if h.Entries[0].Decisions[0].Action != "approved" {
		t.Errorf("expected action 'approved', got %q", h.Entries[0].Decisions[0].Action)
	}
}

func TestLoadHistory_MalformedJSON(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	path := filepath.Join(dir, "bad.json")

	if err := os.WriteFile(path, []byte("<<<not json>>>"), 0644); err != nil {
		t.Fatal(err)
	}

	_, err := LoadHistory(path)
	if err == nil {
		t.Error("expected error for malformed JSON")
	}
}

// ---------------------------------------------------------------------------
// SaveHistory
// ---------------------------------------------------------------------------

func TestSaveHistory_WritesValidJSON(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	path := filepath.Join(dir, "history.json")

	h := &History{
		Entries: []HistoryEntry{
			{
				BatchID:     "b1",
				CompletedAt: time.Date(2025, 5, 1, 8, 0, 0, 0, time.UTC),
				Scopes:      []string{"s1"},
				Decisions:   []Decision{{FindingID: "f1", Title: "T1", Action: "approved", Timestamp: time.Date(2025, 5, 1, 8, 1, 0, 0, time.UTC)}},
			},
		},
	}

	if err := SaveHistory(path, h); err != nil {
		t.Fatal(err)
	}

	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if len(data) == 0 {
		t.Fatal("file should not be empty")
	}
	if data[len(data)-1] != '\n' {
		t.Error("expected trailing newline")
	}
}

func TestSaveHistory_Roundtrip(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	path := filepath.Join(dir, "history.json")

	original := &History{
		Entries: []HistoryEntry{
			{
				BatchID:     "b-rt",
				CompletedAt: time.Date(2025, 8, 15, 14, 0, 0, 0, time.UTC),
				Scopes:      []string{"x", "y"},
				Decisions: []Decision{
					{FindingID: "f1", Title: "Alpha", Action: "approved", Timestamp: time.Date(2025, 8, 15, 14, 5, 0, 0, time.UTC), CreatedNode: "n1"},
					{FindingID: "f2", Title: "Beta", Action: "rejected", Timestamp: time.Date(2025, 8, 15, 14, 6, 0, 0, time.UTC)},
				},
			},
		},
	}

	if err := SaveHistory(path, original); err != nil {
		t.Fatal(err)
	}

	loaded, err := LoadHistory(path)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(loaded.Entries) != 1 {
		t.Fatalf("expected 1 entry, got %d", len(loaded.Entries))
	}
	e := loaded.Entries[0]
	if e.BatchID != "b-rt" {
		t.Errorf("expected batch_id 'b-rt', got %q", e.BatchID)
	}
	if len(e.Decisions) != 2 {
		t.Fatalf("expected 2 decisions, got %d", len(e.Decisions))
	}
	if e.Decisions[0].CreatedNode != "n1" {
		t.Errorf("expected created_node 'n1', got %q", e.Decisions[0].CreatedNode)
	}
}

// ---------------------------------------------------------------------------
// EnforceRetention
// ---------------------------------------------------------------------------

func TestEnforceRetention_MaxEntriesZero_NoOp(t *testing.T) {
	t.Parallel()
	h := &History{
		Entries: []HistoryEntry{
			{BatchID: "a", CompletedAt: time.Now()},
			{BatchID: "b", CompletedAt: time.Now()},
		},
	}
	EnforceRetention(h, 0, 0)
	if len(h.Entries) != 2 {
		t.Errorf("expected 2 entries unchanged, got %d", len(h.Entries))
	}
}

func TestEnforceRetention_MaxEntries_Trims(t *testing.T) {
	t.Parallel()
	now := time.Now().UTC()
	h := &History{
		Entries: []HistoryEntry{
			{BatchID: "oldest", CompletedAt: now.Add(-3 * time.Hour)},
			{BatchID: "middle", CompletedAt: now.Add(-2 * time.Hour)},
			{BatchID: "newest", CompletedAt: now.Add(-1 * time.Hour)},
		},
	}

	EnforceRetention(h, 2, 0)

	if len(h.Entries) != 2 {
		t.Fatalf("expected 2 entries, got %d", len(h.Entries))
	}
	// Should keep the two most recent.
	ids := map[string]bool{}
	for _, e := range h.Entries {
		ids[e.BatchID] = true
	}
	if ids["oldest"] {
		t.Error("oldest entry should have been trimmed")
	}
	if !ids["newest"] || !ids["middle"] {
		t.Error("newest and middle entries should be kept")
	}
}

func TestEnforceRetention_MaxAgeDays_RemovesOld(t *testing.T) {
	t.Parallel()
	now := time.Now().UTC()
	h := &History{
		Entries: []HistoryEntry{
			{BatchID: "ancient", CompletedAt: now.AddDate(0, 0, -60)},
			{BatchID: "recent", CompletedAt: now.Add(-1 * time.Hour)},
		},
	}

	EnforceRetention(h, 0, 30)

	if len(h.Entries) != 1 {
		t.Fatalf("expected 1 entry, got %d", len(h.Entries))
	}
	if h.Entries[0].BatchID != "recent" {
		t.Errorf("expected 'recent' to survive, got %q", h.Entries[0].BatchID)
	}
}

func TestEnforceRetention_BothConstraints(t *testing.T) {
	t.Parallel()
	now := time.Now().UTC()
	h := &History{
		Entries: []HistoryEntry{
			{BatchID: "old", CompletedAt: now.AddDate(0, 0, -100)},
			{BatchID: "a", CompletedAt: now.Add(-3 * time.Hour)},
			{BatchID: "b", CompletedAt: now.Add(-2 * time.Hour)},
			{BatchID: "c", CompletedAt: now.Add(-1 * time.Hour)},
		},
	}

	// Age removes "old"; maxEntries=2 then trims to 2 most recent of the remaining 3.
	EnforceRetention(h, 2, 30)

	if len(h.Entries) != 2 {
		t.Fatalf("expected 2 entries, got %d", len(h.Entries))
	}
	ids := map[string]bool{}
	for _, e := range h.Entries {
		ids[e.BatchID] = true
	}
	if ids["old"] || ids["a"] {
		t.Error("old and 'a' should have been removed")
	}
}

// ---------------------------------------------------------------------------
// Error-wrapping branches (non-NotExist read errors, write errors)
// ---------------------------------------------------------------------------

func TestLoadBatch_ReadError(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	// Passing a directory path triggers a non-NotExist read error.
	_, err := LoadBatch(dir)
	if err == nil {
		t.Error("expected error when reading a directory")
	}
}

func TestLoadHistory_ReadError(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	_, err := LoadHistory(dir)
	if err == nil {
		t.Error("expected error when reading a directory")
	}
}

func TestSaveBatch_CreatesDirectories(t *testing.T) {
	t.Parallel()
	path := filepath.Join(t.TempDir(), "nested", "deep", "batch.json")
	b := &Batch{ID: "x"}
	if err := SaveBatch(path, b); err != nil {
		t.Fatalf("expected SaveBatch to create directories, got error: %v", err)
	}
	if _, err := os.Stat(path); err != nil {
		t.Errorf("file should exist: %v", err)
	}
}

func TestSaveHistory_CreatesDirectories(t *testing.T) {
	t.Parallel()
	path := filepath.Join(t.TempDir(), "nested", "deep", "history.json")
	h := &History{}
	if err := SaveHistory(path, h); err != nil {
		t.Fatalf("expected SaveHistory to create directories, got error: %v", err)
	}
	if _, err := os.Stat(path); err != nil {
		t.Errorf("file should exist: %v", err)
	}
}

func TestRemoveBatch_NonNotExistError(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	// Create a non-empty directory. Os.Remove on it returns a non-NotExist error.
	nested := filepath.Join(dir, "subdir")
	if err := os.Mkdir(nested, 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(nested, "file.txt"), []byte("x"), 0644); err != nil {
		t.Fatal(err)
	}
	err := RemoveBatch(nested)
	if err == nil {
		t.Error("expected error when removing a non-empty directory")
	}
}

func TestEnforceRetention_EmptyHistory(t *testing.T) {
	t.Parallel()
	h := &History{}

	EnforceRetention(h, 5, 30)

	if len(h.Entries) != 0 {
		t.Errorf("expected empty entries, got %d", len(h.Entries))
	}
}
