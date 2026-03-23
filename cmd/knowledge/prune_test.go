package knowledge

import (
	"os"
	"strings"
	"testing"

	"github.com/dorkusprime/wolfcastle/internal/knowledge"
)

func TestKnowledgePrune_NoIdentity(t *testing.T) {
	env := newTestEnv(t)
	env.App.Identity = nil

	env.RootCmd.SetArgs([]string{"knowledge", "prune"})
	err := env.RootCmd.Execute()
	if err == nil {
		t.Error("expected error when identity is nil")
	}
}

func TestKnowledgePrune_JSON_Empty(t *testing.T) {
	env := newTestEnv(t)
	env.App.JSON = true

	env.RootCmd.SetArgs([]string{"knowledge", "prune"})
	if err := env.RootCmd.Execute(); err != nil {
		t.Fatalf("knowledge prune --json failed: %v", err)
	}
}

func TestKnowledgePrune_JSON_WithContent(t *testing.T) {
	env := newTestEnv(t)
	env.App.JSON = true

	ns := env.App.Identity.Namespace
	if err := knowledge.Append(env.WolfcastleDir, ns, "prune test entry"); err != nil {
		t.Fatal(err)
	}

	env.RootCmd.SetArgs([]string{"knowledge", "prune"})
	if err := env.RootCmd.Execute(); err != nil {
		t.Fatalf("knowledge prune --json failed: %v", err)
	}
}

func TestKnowledgePrune_Interactive(t *testing.T) {
	env := newTestEnv(t)
	t.Setenv("EDITOR", "true")

	ns := env.App.Identity.Namespace
	if err := knowledge.Append(env.WolfcastleDir, ns, "entry to maybe prune"); err != nil {
		t.Fatal(err)
	}

	env.RootCmd.SetArgs([]string{"knowledge", "prune"})
	if err := env.RootCmd.Execute(); err != nil {
		t.Fatalf("knowledge prune failed: %v", err)
	}
}

func TestKnowledgePrune_CreatesFile(t *testing.T) {
	env := newTestEnv(t)
	t.Setenv("EDITOR", "true")

	path := knowledge.FilePath(env.WolfcastleDir, env.App.Identity.Namespace)
	if _, err := os.Stat(path); err == nil {
		t.Fatal("knowledge file should not exist before prune")
	}

	env.RootCmd.SetArgs([]string{"knowledge", "prune"})
	if err := env.RootCmd.Execute(); err != nil {
		t.Fatalf("knowledge prune failed: %v", err)
	}

	if _, err := os.Stat(path); err != nil {
		t.Fatal("knowledge file should exist after prune")
	}
}

func TestKnowledgePrune_EditorFailure(t *testing.T) {
	env := newTestEnv(t)
	t.Setenv("EDITOR", "false")

	env.RootCmd.SetArgs([]string{"knowledge", "prune"})
	err := env.RootCmd.Execute()
	if err == nil {
		t.Error("expected error when editor exits non-zero")
	}
}

func TestKnowledgePrune_OverBudget(t *testing.T) {
	env := newTestEnv(t)
	t.Setenv("EDITOR", "true")

	ns := env.App.Identity.Namespace
	path := knowledge.FilePath(env.WolfcastleDir, ns)
	if err := os.MkdirAll(path[:len(path)-len(ns+".md")], 0o755); err != nil {
		t.Fatal(err)
	}
	// Fill with enough content to exceed the 2000 token budget.
	bigContent := strings.Repeat("word ", 2000)
	if err := os.WriteFile(path, []byte(bigContent), 0o644); err != nil {
		t.Fatal(err)
	}

	// Prune runs the editor (true = no-op), then reports over-budget.
	env.RootCmd.SetArgs([]string{"knowledge", "prune"})
	if err := env.RootCmd.Execute(); err != nil {
		t.Fatalf("knowledge prune failed: %v", err)
	}
}

func TestKnowledgePrune_JSON_OverBudget(t *testing.T) {
	env := newTestEnv(t)
	env.App.JSON = true

	ns := env.App.Identity.Namespace
	path := knowledge.FilePath(env.WolfcastleDir, ns)
	if err := os.MkdirAll(path[:len(path)-len(ns+".md")], 0o755); err != nil {
		t.Fatal(err)
	}
	bigContent := strings.Repeat("word ", 2000)
	if err := os.WriteFile(path, []byte(bigContent), 0o644); err != nil {
		t.Fatal(err)
	}

	env.RootCmd.SetArgs([]string{"knowledge", "prune"})
	if err := env.RootCmd.Execute(); err != nil {
		t.Fatalf("knowledge prune --json failed: %v", err)
	}
}
