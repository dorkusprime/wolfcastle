package inbox

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	"github.com/dorkusprime/wolfcastle/cmd/cmdutil"
	"github.com/dorkusprime/wolfcastle/internal/config"
	inboxpkg "github.com/dorkusprime/wolfcastle/internal/inbox"
	"github.com/dorkusprime/wolfcastle/internal/state"
	"github.com/dorkusprime/wolfcastle/internal/tree"
	"github.com/spf13/cobra"
)

type testEnv struct {
	WolfcastleDir string
	ProjectsDir   string
	App           *cmdutil.App
	RootCmd       *cobra.Command
}

func newTestEnv(t *testing.T) *testEnv {
	t.Helper()
	tmp := t.TempDir()
	wcDir := filepath.Join(tmp, ".wolfcastle")
	os.MkdirAll(wcDir, 0755)

	cfg := config.Defaults()
	cfg.Identity = &config.IdentityConfig{User: "test", Machine: "dev"}

	ns := "test-dev"
	projDir := filepath.Join(wcDir, "projects", ns)
	os.MkdirAll(projDir, 0755)

	idx := state.NewRootIndex()
	data, _ := json.MarshalIndent(idx, "", "  ")
	os.WriteFile(filepath.Join(projDir, "state.json"), data, 0644)

	resolver := &tree.Resolver{WolfcastleDir: wcDir, Namespace: ns}
	testApp := &cmdutil.App{
		WolfcastleDir: wcDir,
		Cfg:           cfg,
		Resolver:      resolver,
	}

	rootCmd := &cobra.Command{Use: "wolfcastle"}
	Register(testApp, rootCmd)

	return &testEnv{
		WolfcastleDir: wcDir,
		ProjectsDir:   projDir,
		App:           testApp,
		RootCmd:       rootCmd,
	}
}

func loadInbox(t *testing.T, env *testEnv) *inboxpkg.File {
	t.Helper()
	inboxPath := filepath.Join(env.ProjectsDir, "inbox.json")
	f, err := inboxpkg.Load(inboxPath)
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

func TestInboxAdd_NoResolver(t *testing.T) {
	env := newTestEnv(t)
	env.App.Resolver = nil

	env.RootCmd.SetArgs([]string{"inbox", "add", "test"})
	err := env.RootCmd.Execute()
	if err == nil {
		t.Error("expected error when resolver is nil")
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
	env.RootCmd.Execute()
	env.RootCmd.SetArgs([]string{"inbox", "add", "second idea"})
	env.RootCmd.Execute()

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
	env.RootCmd.Execute()

	// Manually set one item to "filed"
	inboxPath := filepath.Join(env.ProjectsDir, "inbox.json")
	f, _ := inboxpkg.Load(inboxPath)
	f.Items = append(f.Items, inboxpkg.Item{
		Timestamp: "2025-01-01T00:00:00Z",
		Text:      "filed idea",
		Status:    "filed",
	})
	inboxpkg.Save(inboxPath, f)

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
	env.RootCmd.Execute()
	env.RootCmd.SetArgs([]string{"inbox", "add", "idea two"})
	env.RootCmd.Execute()

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
