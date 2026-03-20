package inbox

import (
	"path/filepath"
	"testing"

	"github.com/dorkusprime/wolfcastle/cmd/cmdutil"
	"github.com/dorkusprime/wolfcastle/internal/clock"
	"github.com/dorkusprime/wolfcastle/internal/state"
	"github.com/dorkusprime/wolfcastle/internal/testutil"
	"github.com/spf13/cobra"
)

type testEnv struct {
	WolfcastleDir string
	ProjectsDir   string
	App           *cmdutil.App
	RootCmd       *cobra.Command
	env           *testutil.Environment
}

func newTestEnv(t *testing.T) *testEnv {
	t.Helper()

	env := testutil.NewEnvironment(t)
	af := env.ToAppFields()

	testApp := &cmdutil.App{
		Config:   af.Config,
		Identity: af.Identity,
		State:    af.State,
		Prompts:  af.Prompts,
		Classes:  af.Classes,
		Daemon:   af.Daemon,
		Git:      af.Git,
		Clock:    clock.New(),
	}

	rootCmd := &cobra.Command{Use: "wolfcastle"}
	rootCmd.AddGroup(
		&cobra.Group{ID: "lifecycle", Title: "Lifecycle:"},
		&cobra.Group{ID: "work", Title: "Work Management:"},
		&cobra.Group{ID: "audit", Title: "Auditing:"},
		&cobra.Group{ID: "docs", Title: "Documentation:"},
		&cobra.Group{ID: "diagnostics", Title: "Diagnostics:"},
		&cobra.Group{ID: "integration", Title: "Integration:"},
	)
	Register(testApp, rootCmd)

	return &testEnv{
		WolfcastleDir: env.Root,
		ProjectsDir:   env.ProjectsDir(),
		App:           testApp,
		RootCmd:       rootCmd,
		env:           env,
	}
}

func loadInbox(t *testing.T, env *testEnv) *state.InboxFile {
	t.Helper()
	inboxPath := filepath.Join(env.ProjectsDir, "inbox.json")
	f, err := state.LoadInbox(inboxPath)
	if err != nil {
		t.Fatalf("loading inbox: %v", err)
	}
	return f
}

// ---------------------------------------------------------------------------
// inbox add
// ---------------------------------------------------------------------------

func TestInboxAdd_Success(t *testing.T) {
	env := newTestEnv(t)

	env.RootCmd.SetArgs([]string{"inbox", "add", "refactor auth middleware"})
	if err := env.RootCmd.Execute(); err != nil {
		t.Fatalf("inbox add failed: %v", err)
	}

	f := loadInbox(t, env)
	if len(f.Items) != 1 {
		t.Fatalf("expected 1 item, got %d", len(f.Items))
	}
	if f.Items[0].Text != "refactor auth middleware" {
		t.Errorf("unexpected text: %s", f.Items[0].Text)
	}
	if f.Items[0].Status != "new" {
		t.Errorf("expected status 'new', got %s", f.Items[0].Status)
	}
	if f.Items[0].Timestamp == "" {
		t.Error("timestamp should be set")
	}
}

func TestInboxAdd_Multiple(t *testing.T) {
	env := newTestEnv(t)

	for _, text := range []string{"idea one", "idea two", "idea three"} {
		env.RootCmd.SetArgs([]string{"inbox", "add", text})
		if err := env.RootCmd.Execute(); err != nil {
			t.Fatalf("inbox add %q failed: %v", text, err)
		}
	}

	f := loadInbox(t, env)
	if len(f.Items) != 3 {
		t.Fatalf("expected 3 items, got %d", len(f.Items))
	}
}

func TestInboxAdd_EmptyText(t *testing.T) {
	env := newTestEnv(t)
	env.RootCmd.SetArgs([]string{"inbox", "add", "   "})
	err := env.RootCmd.Execute()
	if err == nil {
		t.Error("expected error for empty text")
	}
}

func TestInboxAdd_NoIdentity(t *testing.T) {
	env := newTestEnv(t)
	env.App.Identity = nil

	env.RootCmd.SetArgs([]string{"inbox", "add", "test"})
	err := env.RootCmd.Execute()
	if err == nil {
		t.Error("expected error when identity is nil")
	}
}

// ---------------------------------------------------------------------------
// inbox list
// ---------------------------------------------------------------------------

func TestInboxList_Empty(t *testing.T) {
	env := newTestEnv(t)

	env.RootCmd.SetArgs([]string{"inbox", "list"})
	if err := env.RootCmd.Execute(); err != nil {
		t.Fatalf("inbox list failed: %v", err)
	}
}

func TestInboxList_WithItems(t *testing.T) {
	env := newTestEnv(t)

	env.RootCmd.SetArgs([]string{"inbox", "add", "first idea"})
	_ = env.RootCmd.Execute()
	env.RootCmd.SetArgs([]string{"inbox", "add", "second idea"})
	_ = env.RootCmd.Execute()

	env.RootCmd.SetArgs([]string{"inbox", "list"})
	if err := env.RootCmd.Execute(); err != nil {
		t.Fatalf("inbox list failed: %v", err)
	}
}

// ---------------------------------------------------------------------------
// inbox clear
// ---------------------------------------------------------------------------

func TestInboxClear_ClearsFiledOnly(t *testing.T) {
	env := newTestEnv(t)

	// Add items
	env.RootCmd.SetArgs([]string{"inbox", "add", "new idea"})
	_ = env.RootCmd.Execute()

	// Manually set one item to "filed"
	inboxPath := filepath.Join(env.ProjectsDir, "inbox.json")
	f, _ := state.LoadInbox(inboxPath)
	f.Items = append(f.Items, state.InboxItem{
		Timestamp: "2025-01-01T00:00:00Z",
		Text:      "filed idea",
		Status:    "filed",
	})
	_ = state.SaveInbox(inboxPath, f)

	env.RootCmd.SetArgs([]string{"inbox", "clear"})
	if err := env.RootCmd.Execute(); err != nil {
		t.Fatalf("inbox clear failed: %v", err)
	}

	f = loadInbox(t, env)
	// Only "new" items should remain
	if len(f.Items) != 1 {
		t.Fatalf("expected 1 remaining item, got %d", len(f.Items))
	}
	if f.Items[0].Status != "new" {
		t.Errorf("remaining item should be 'new', got %s", f.Items[0].Status)
	}
}

func TestInboxClear_All(t *testing.T) {
	env := newTestEnv(t)

	env.RootCmd.SetArgs([]string{"inbox", "add", "idea one"})
	_ = env.RootCmd.Execute()
	env.RootCmd.SetArgs([]string{"inbox", "add", "idea two"})
	_ = env.RootCmd.Execute()

	env.RootCmd.SetArgs([]string{"inbox", "clear", "--all"})
	if err := env.RootCmd.Execute(); err != nil {
		t.Fatalf("inbox clear --all failed: %v", err)
	}

	f := loadInbox(t, env)
	if len(f.Items) != 0 {
		t.Errorf("expected empty inbox after --all, got %d items", len(f.Items))
	}
}

func TestInboxClear_EmptyInbox(t *testing.T) {
	env := newTestEnv(t)
	env.RootCmd.SetArgs([]string{"inbox", "clear"})
	if err := env.RootCmd.Execute(); err != nil {
		t.Fatalf("clear on empty inbox should succeed: %v", err)
	}
}
