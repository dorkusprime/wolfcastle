package inbox

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/dorkusprime/wolfcastle/internal/state"
)

// ---------------------------------------------------------------------------
// inbox add — error paths
// ---------------------------------------------------------------------------

func TestInboxAdd_JSONOutput_Multiple(t *testing.T) {
	env := newTestEnv(t)
	env.App.JSONOutput = true
	defer func() { env.App.JSONOutput = false }()

	for _, text := range []string{"first idea", "second idea"} {
		env.RootCmd.SetArgs([]string{"inbox", "add", text})
		if err := env.RootCmd.Execute(); err != nil {
			t.Fatalf("inbox add (json) %q failed: %v", text, err)
		}
	}

	f := loadInbox(t, env)
	if len(f.Items) != 2 {
		t.Fatalf("expected 2 items, got %d", len(f.Items))
	}
}

// ---------------------------------------------------------------------------
// inbox list — error paths
// ---------------------------------------------------------------------------

func TestInboxList_NoResolver(t *testing.T) {
	env := newTestEnv(t)
	env.App.Resolver = nil

	env.RootCmd.SetArgs([]string{"inbox", "list"})
	err := env.RootCmd.Execute()
	if err == nil {
		t.Error("expected error when resolver is nil")
	}
}

// ---------------------------------------------------------------------------
// inbox clear — error paths
// ---------------------------------------------------------------------------

func TestInboxClear_NoResolver(t *testing.T) {
	env := newTestEnv(t)
	env.App.Resolver = nil

	env.RootCmd.SetArgs([]string{"inbox", "clear"})
	err := env.RootCmd.Execute()
	if err == nil {
		t.Error("expected error when resolver is nil")
	}
}

func TestInboxClear_WithExpandedItems(t *testing.T) {
	env := newTestEnv(t)

	// Add items with various statuses
	inboxPath := filepath.Join(env.ProjectsDir, "inbox.json")
	inboxData := &state.InboxFile{
		Items: []state.InboxItem{
			{Timestamp: "2025-01-01T00:00:00Z", Text: "new one", Status: "new"},
			{Timestamp: "2025-01-01T00:00:00Z", Text: "filed one", Status: "filed"},
			{Timestamp: "2025-01-01T00:00:00Z", Text: "expanded one", Status: "expanded"},
		},
	}
	_ = state.SaveInbox(inboxPath, inboxData)

	// Clear without --all should keep only "new" items
	env.RootCmd.SetArgs([]string{"inbox", "clear"})
	if err := env.RootCmd.Execute(); err != nil {
		t.Fatalf("inbox clear failed: %v", err)
	}

	f := loadInbox(t, env)
	if len(f.Items) != 1 {
		t.Fatalf("expected 1 remaining item, got %d", len(f.Items))
	}
	if f.Items[0].Text != "new one" {
		t.Errorf("wrong item kept: %s", f.Items[0].Text)
	}
}

func TestInboxClear_JSONOutput_WithoutAll(t *testing.T) {
	env := newTestEnv(t)

	// Add a filed item
	inboxPath := filepath.Join(env.ProjectsDir, "inbox.json")
	inboxData := &state.InboxFile{
		Items: []state.InboxItem{
			{Timestamp: "2025-01-01T00:00:00Z", Text: "filed", Status: "filed"},
			{Timestamp: "2025-01-01T00:00:00Z", Text: "new", Status: "new"},
		},
	}
	_ = state.SaveInbox(inboxPath, inboxData)

	env.App.JSONOutput = true
	defer func() { env.App.JSONOutput = false }()

	env.RootCmd.SetArgs([]string{"inbox", "clear"})
	if err := env.RootCmd.Execute(); err != nil {
		t.Fatalf("inbox clear (json without all) failed: %v", err)
	}
}

func TestInboxClear_AllClearsNewItems(t *testing.T) {
	env := newTestEnv(t)

	// Add items with "new" status
	env.RootCmd.SetArgs([]string{"inbox", "add", "should be cleared"})
	_ = env.RootCmd.Execute()

	env.RootCmd.SetArgs([]string{"inbox", "clear", "--all"})
	if err := env.RootCmd.Execute(); err != nil {
		t.Fatalf("inbox clear --all failed: %v", err)
	}

	f := loadInbox(t, env)
	if len(f.Items) != 0 {
		t.Errorf("expected 0 items after --all, got %d", len(f.Items))
	}
}

// ---------------------------------------------------------------------------
// inbox list — empty inbox path
// ---------------------------------------------------------------------------

// ---------------------------------------------------------------------------
// inbox add — LoadInbox error (invalid JSON in inbox file)
// ---------------------------------------------------------------------------

func TestInboxAdd_BrokenInboxFile(t *testing.T) {
	env := newTestEnv(t)

	// Write invalid JSON to inbox.json
	inboxPath := filepath.Join(env.ProjectsDir, "inbox.json")
	_ = os.WriteFile(inboxPath, []byte("not valid json"), 0644)

	env.RootCmd.SetArgs([]string{"inbox", "add", "should fail"})
	err := env.RootCmd.Execute()
	if err == nil {
		t.Error("expected error when inbox.json contains invalid JSON")
	}
}

// ---------------------------------------------------------------------------
// inbox list — LoadInbox error (invalid JSON)
// ---------------------------------------------------------------------------

func TestInboxList_BrokenInboxFile(t *testing.T) {
	env := newTestEnv(t)

	inboxPath := filepath.Join(env.ProjectsDir, "inbox.json")
	_ = os.WriteFile(inboxPath, []byte("not valid json"), 0644)

	env.RootCmd.SetArgs([]string{"inbox", "list"})
	err := env.RootCmd.Execute()
	if err == nil {
		t.Error("expected error when inbox.json contains invalid JSON")
	}
}

// ---------------------------------------------------------------------------
// inbox clear — LoadInbox error (invalid JSON)
// ---------------------------------------------------------------------------

func TestInboxClear_BrokenInboxFile(t *testing.T) {
	env := newTestEnv(t)

	inboxPath := filepath.Join(env.ProjectsDir, "inbox.json")
	_ = os.WriteFile(inboxPath, []byte("not valid json"), 0644)

	env.RootCmd.SetArgs([]string{"inbox", "clear"})
	err := env.RootCmd.Execute()
	if err == nil {
		t.Error("expected error when inbox.json contains invalid JSON")
	}
}

func TestInboxList_NonexistentPath(t *testing.T) {
	env := newTestEnv(t)

	// Remove the projects dir entirely
	_ = os.RemoveAll(env.ProjectsDir)
	_ = os.MkdirAll(env.ProjectsDir, 0755)

	// List should still work (LoadInbox returns empty on missing file)
	env.RootCmd.SetArgs([]string{"inbox", "list"})
	if err := env.RootCmd.Execute(); err != nil {
		t.Fatalf("inbox list on missing inbox failed: %v", err)
	}
}
