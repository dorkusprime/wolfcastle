package knowledge

import (
	"os"
	"strings"
	"testing"

	"github.com/dorkusprime/wolfcastle/cmd/cmdutil"
	"github.com/dorkusprime/wolfcastle/internal/clock"
	"github.com/dorkusprime/wolfcastle/internal/knowledge"
	"github.com/dorkusprime/wolfcastle/internal/testutil"
	"github.com/spf13/cobra"
)

type testEnv struct {
	WolfcastleDir string
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
		App:           testApp,
		RootCmd:       rootCmd,
		env:           env,
	}
}

func TestKnowledgeAdd_Success(t *testing.T) {
	env := newTestEnv(t)

	env.RootCmd.SetArgs([]string{"knowledge", "add", "integration tests require docker compose up"})
	if err := env.RootCmd.Execute(); err != nil {
		t.Fatalf("knowledge add failed: %v", err)
	}

	content, err := knowledge.Read(env.WolfcastleDir, env.App.Identity.Namespace)
	if err != nil {
		t.Fatalf("reading knowledge file: %v", err)
	}
	if !strings.Contains(content, "integration tests require docker compose up") {
		t.Errorf("expected entry in knowledge file, got: %s", content)
	}
}

func TestKnowledgeAdd_AddsBulletPrefix(t *testing.T) {
	env := newTestEnv(t)

	env.RootCmd.SetArgs([]string{"knowledge", "add", "no bullet here"})
	if err := env.RootCmd.Execute(); err != nil {
		t.Fatalf("knowledge add failed: %v", err)
	}

	content, err := knowledge.Read(env.WolfcastleDir, env.App.Identity.Namespace)
	if err != nil {
		t.Fatalf("reading knowledge file: %v", err)
	}
	if !strings.HasPrefix(content, "- ") {
		t.Errorf("expected bullet prefix, got: %q", content)
	}
}

func TestKnowledgeAdd_PreservesExistingBullet(t *testing.T) {
	env := newTestEnv(t)

	env.RootCmd.SetArgs([]string{"knowledge", "add", "--", "- already has bullet"})
	if err := env.RootCmd.Execute(); err != nil {
		t.Fatalf("knowledge add failed: %v", err)
	}

	content, err := knowledge.Read(env.WolfcastleDir, env.App.Identity.Namespace)
	if err != nil {
		t.Fatalf("reading knowledge file: %v", err)
	}
	if strings.Contains(content, "- - ") {
		t.Errorf("double bullet prefix detected: %q", content)
	}
}

func TestKnowledgeAdd_MultipleEntries(t *testing.T) {
	env := newTestEnv(t)

	entries := []string{"first fact", "second fact", "third fact"}
	for _, entry := range entries {
		env.RootCmd.SetArgs([]string{"knowledge", "add", entry})
		if err := env.RootCmd.Execute(); err != nil {
			t.Fatalf("knowledge add %q failed: %v", entry, err)
		}
	}

	content, err := knowledge.Read(env.WolfcastleDir, env.App.Identity.Namespace)
	if err != nil {
		t.Fatalf("reading knowledge file: %v", err)
	}
	for _, entry := range entries {
		if !strings.Contains(content, entry) {
			t.Errorf("expected %q in knowledge file", entry)
		}
	}
}

func TestKnowledgeAdd_EmptyEntry(t *testing.T) {
	env := newTestEnv(t)
	env.RootCmd.SetArgs([]string{"knowledge", "add", "   "})
	err := env.RootCmd.Execute()
	if err == nil {
		t.Error("expected error for empty entry")
	}
}

func TestKnowledgeAdd_NoArgs(t *testing.T) {
	env := newTestEnv(t)
	env.RootCmd.SetArgs([]string{"knowledge", "add"})
	err := env.RootCmd.Execute()
	if err == nil {
		t.Error("expected error for missing argument")
	}
}

func TestKnowledgeAdd_NoIdentity(t *testing.T) {
	env := newTestEnv(t)
	env.App.Identity = nil

	env.RootCmd.SetArgs([]string{"knowledge", "add", "test"})
	err := env.RootCmd.Execute()
	if err == nil {
		t.Error("expected error when identity is nil")
	}
}

func TestKnowledgeAdd_ExceedsBudget(t *testing.T) {
	env := newTestEnv(t)

	// Write a large existing file to fill the budget.
	ns := env.App.Identity.Namespace
	p := knowledge.FilePath(env.WolfcastleDir, ns)
	if err := os.MkdirAll(p[:len(p)-len(ns+".md")], 0o755); err != nil {
		t.Fatal(err)
	}
	// Default budget is 2000 tokens. Fill with enough words to exceed it.
	bigContent := strings.Repeat("word ", 2000)
	if err := os.WriteFile(p, []byte(bigContent), 0o644); err != nil {
		t.Fatal(err)
	}

	env.RootCmd.SetArgs([]string{"knowledge", "add", "one more entry"})
	err := env.RootCmd.Execute()
	if err == nil {
		t.Error("expected error when budget is exceeded")
	}
	if err != nil && !strings.Contains(err.Error(), "exceeds budget") {
		t.Errorf("expected budget error, got: %v", err)
	}
}

func TestKnowledgeAdd_JSON(t *testing.T) {
	env := newTestEnv(t)
	env.App.JSON = true

	env.RootCmd.SetArgs([]string{"knowledge", "add", "json test entry"})
	if err := env.RootCmd.Execute(); err != nil {
		t.Fatalf("knowledge add --json failed: %v", err)
	}

	content, err := knowledge.Read(env.WolfcastleDir, env.App.Identity.Namespace)
	if err != nil {
		t.Fatalf("reading knowledge file: %v", err)
	}
	if !strings.Contains(content, "json test entry") {
		t.Errorf("expected entry in knowledge file after JSON mode add")
	}
}
