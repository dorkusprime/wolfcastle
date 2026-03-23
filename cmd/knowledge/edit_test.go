package knowledge

import (
	"os"
	"testing"

	"github.com/dorkusprime/wolfcastle/internal/knowledge"
)

func TestKnowledgeEdit_NoIdentity(t *testing.T) {
	env := newTestEnv(t)
	env.App.Identity = nil

	env.RootCmd.SetArgs([]string{"knowledge", "edit"})
	err := env.RootCmd.Execute()
	if err == nil {
		t.Error("expected error when identity is nil")
	}
}

func TestKnowledgeEdit_JSON(t *testing.T) {
	env := newTestEnv(t)
	env.App.JSON = true

	env.RootCmd.SetArgs([]string{"knowledge", "edit"})
	if err := env.RootCmd.Execute(); err != nil {
		t.Fatalf("knowledge edit --json failed: %v", err)
	}
}

func TestKnowledgeEdit_CreatesFile(t *testing.T) {
	env := newTestEnv(t)

	// Use EDITOR=true so the "editor" exits immediately.
	t.Setenv("EDITOR", "true")

	path := knowledge.FilePath(env.WolfcastleDir, env.App.Identity.Namespace)

	// File should not exist yet.
	if _, err := os.Stat(path); err == nil {
		t.Fatal("knowledge file should not exist before edit")
	}

	env.RootCmd.SetArgs([]string{"knowledge", "edit"})
	if err := env.RootCmd.Execute(); err != nil {
		t.Fatalf("knowledge edit failed: %v", err)
	}

	// File should now exist with template content.
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("reading created file: %v", err)
	}
	if string(data) != "# Codebase Knowledge\n\n" {
		t.Errorf("unexpected template content: %q", string(data))
	}
}

func TestKnowledgeEdit_ExistingFile(t *testing.T) {
	env := newTestEnv(t)
	t.Setenv("EDITOR", "true")

	ns := env.App.Identity.Namespace
	if err := knowledge.Append(env.WolfcastleDir, ns, "existing entry"); err != nil {
		t.Fatal(err)
	}

	env.RootCmd.SetArgs([]string{"knowledge", "edit"})
	if err := env.RootCmd.Execute(); err != nil {
		t.Fatalf("knowledge edit failed: %v", err)
	}

	// Content should be preserved (editor=true doesn't modify).
	content, err := knowledge.Read(env.WolfcastleDir, ns)
	if err != nil {
		t.Fatal(err)
	}
	if content == "" {
		t.Error("expected existing content to be preserved")
	}
}

func TestKnowledgeEdit_EditorFailure(t *testing.T) {
	env := newTestEnv(t)
	t.Setenv("EDITOR", "false")

	env.RootCmd.SetArgs([]string{"knowledge", "edit"})
	err := env.RootCmd.Execute()
	if err == nil {
		t.Error("expected error when editor exits non-zero")
	}
}

func TestKnowledgeEdit_DefaultEditor(t *testing.T) {
	env := newTestEnv(t)
	// Unset EDITOR so it falls back to vi; use JSON mode to avoid launching it.
	t.Setenv("EDITOR", "")
	env.App.JSON = true

	env.RootCmd.SetArgs([]string{"knowledge", "edit"})
	if err := env.RootCmd.Execute(); err != nil {
		t.Fatalf("knowledge edit --json (default editor) failed: %v", err)
	}
}

func TestEnsureKnowledgeFile_BadPath(t *testing.T) {
	// Point at a path where the parent is a file, not a directory.
	err := ensureKnowledgeFile("/dev/null/impossible/knowledge.md")
	if err == nil {
		t.Error("expected error for impossible path")
	}
}
