package knowledge

import (
	"strings"
	"testing"

	"github.com/dorkusprime/wolfcastle/internal/knowledge"
)

func TestKnowledgeShow_Empty(t *testing.T) {
	env := newTestEnv(t)

	env.RootCmd.SetArgs([]string{"knowledge", "show"})
	if err := env.RootCmd.Execute(); err != nil {
		t.Fatalf("knowledge show failed: %v", err)
	}
}

func TestKnowledgeShow_WithContent(t *testing.T) {
	env := newTestEnv(t)

	// Add an entry first.
	ns := env.App.Identity.Namespace
	if err := knowledge.Append(env.WolfcastleDir, ns, "test entry one"); err != nil {
		t.Fatal(err)
	}

	env.RootCmd.SetArgs([]string{"knowledge", "show"})
	if err := env.RootCmd.Execute(); err != nil {
		t.Fatalf("knowledge show failed: %v", err)
	}
}

func TestKnowledgeShow_NoIdentity(t *testing.T) {
	env := newTestEnv(t)
	env.App.Identity = nil

	env.RootCmd.SetArgs([]string{"knowledge", "show"})
	err := env.RootCmd.Execute()
	if err == nil {
		t.Error("expected error when identity is nil")
	}
}

func TestKnowledgeShow_JSON(t *testing.T) {
	env := newTestEnv(t)
	env.App.JSON = true

	ns := env.App.Identity.Namespace
	if err := knowledge.Append(env.WolfcastleDir, ns, "json show entry"); err != nil {
		t.Fatal(err)
	}

	env.RootCmd.SetArgs([]string{"knowledge", "show"})
	if err := env.RootCmd.Execute(); err != nil {
		t.Fatalf("knowledge show --json failed: %v", err)
	}
}

func TestKnowledgeShow_JSONEmpty(t *testing.T) {
	env := newTestEnv(t)
	env.App.JSON = true

	env.RootCmd.SetArgs([]string{"knowledge", "show"})
	if err := env.RootCmd.Execute(); err != nil {
		t.Fatalf("knowledge show --json (empty) failed: %v", err)
	}
}

func TestKnowledgeShow_TokenCount(t *testing.T) {
	env := newTestEnv(t)

	ns := env.App.Identity.Namespace
	entries := []string{"first fact", "second fact", "third fact"}
	for _, e := range entries {
		if err := knowledge.Append(env.WolfcastleDir, ns, e); err != nil {
			t.Fatal(err)
		}
	}

	content, err := knowledge.Read(env.WolfcastleDir, ns)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(content, "first fact") {
		t.Error("expected content to contain entries")
	}
	count := knowledge.TokenCount(content)
	if count == 0 {
		t.Error("expected non-zero token count for non-empty content")
	}
}
