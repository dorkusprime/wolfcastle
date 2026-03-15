package state

import (
	"os"
	"path/filepath"
	"testing"
)

// ---------------------------------------------------------------------------
// LoadInbox
// ---------------------------------------------------------------------------

func TestLoadInbox_NonExistentFile(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	path := filepath.Join(dir, "no-such-inbox.json")

	f, err := LoadInbox(path)
	if err != nil {
		t.Fatalf("expected nil error, got %v", err)
	}
	if f == nil {
		t.Fatal("expected non-nil InboxFile")
	}
	if len(f.Items) != 0 {
		t.Errorf("expected empty items, got %d", len(f.Items))
	}
}

func TestLoadInbox_ValidFile(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	path := filepath.Join(dir, "inbox.json")

	content := `{
  "items": [
    {"timestamp": "2025-06-01T10:00:00Z", "text": "first item", "status": "new"},
    {"timestamp": "2025-06-02T10:00:00Z", "text": "second item", "status": "filed"}
  ]
}`
	if err := os.WriteFile(path, []byte(content), 0644); err != nil {
		t.Fatal(err)
	}

	f, err := LoadInbox(path)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(f.Items) != 2 {
		t.Fatalf("expected 2 items, got %d", len(f.Items))
	}
	if f.Items[0].Text != "first item" {
		t.Errorf("expected text 'first item', got %q", f.Items[0].Text)
	}
	if f.Items[0].Status != "new" {
		t.Errorf("expected status 'new', got %q", f.Items[0].Status)
	}
	if f.Items[1].Status != "filed" {
		t.Errorf("expected status 'filed', got %q", f.Items[1].Status)
	}
}

func TestLoadInbox_MalformedJSON(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	path := filepath.Join(dir, "bad.json")

	if err := os.WriteFile(path, []byte("{{broken}"), 0644); err != nil {
		t.Fatal(err)
	}

	_, err := LoadInbox(path)
	if err == nil {
		t.Error("expected error for malformed JSON")
	}
}

// ---------------------------------------------------------------------------
// SaveInbox
// ---------------------------------------------------------------------------

func TestLoadInbox_ReadError(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	// Reading a directory triggers a non-NotExist read error.
	_, err := LoadInbox(dir)
	if err == nil {
		t.Error("expected error when reading a directory")
	}
}

func TestSaveInbox_CreatesDirectories(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	path := filepath.Join(dir, "nested", "deep", "inbox.json")

	f := &InboxFile{Items: []InboxItem{{Text: "x"}}}
	if err := SaveInbox(path, f); err != nil {
		t.Fatalf("expected SaveInbox to create directories, got error: %v", err)
	}

	if _, err := os.Stat(path); err != nil {
		t.Errorf("file should exist: %v", err)
	}
}

func TestSaveInbox_WritesValidJSONWithTrailingNewline(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	path := filepath.Join(dir, "inbox.json")

	f := &InboxFile{
		Items: []InboxItem{
			{Timestamp: "2025-06-01T10:00:00Z", Text: "hello", Status: "new"},
		},
	}

	if err := SaveInbox(path, f); err != nil {
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

func TestSaveInbox_Roundtrip(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	path := filepath.Join(dir, "inbox.json")

	original := &InboxFile{
		Items: []InboxItem{
			{Timestamp: "2025-06-01T10:00:00Z", Text: "item one", Status: "new"},
			{Timestamp: "2025-06-02T11:00:00Z", Text: "item two", Status: "filed"},
		},
	}

	if err := SaveInbox(path, original); err != nil {
		t.Fatal(err)
	}

	loaded, err := LoadInbox(path)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(loaded.Items) != 2 {
		t.Fatalf("expected 2 items, got %d", len(loaded.Items))
	}
	if loaded.Items[0].Text != "item one" {
		t.Errorf("expected text 'item one', got %q", loaded.Items[0].Text)
	}
	if loaded.Items[0].Timestamp != "2025-06-01T10:00:00Z" {
		t.Errorf("expected timestamp '2025-06-01T10:00:00Z', got %q", loaded.Items[0].Timestamp)
	}
	if loaded.Items[1].Status != "filed" {
		t.Errorf("expected status 'filed', got %q", loaded.Items[1].Status)
	}
}
