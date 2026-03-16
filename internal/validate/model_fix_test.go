package validate

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/dorkusprime/wolfcastle/internal/config"
	"github.com/dorkusprime/wolfcastle/internal/invoke"
)

func TestBuildDoctorPrompt_FallbackWhenNoDir(t *testing.T) {
	t.Parallel()
	issue := Issue{
		Node:        "test/node",
		Category:    CatInvalidStateValue,
		FixType:     FixModelAssisted,
		Description: "Invalid state value: \"garbage\"",
	}

	prompt := buildDoctorPrompt("", issue)
	if prompt == "" {
		t.Error("expected non-empty prompt")
	}
	if !strings.Contains(prompt, "test/node") {
		t.Error("prompt should contain node address")
	}
	if !strings.Contains(prompt, CatInvalidStateValue) {
		t.Error("prompt should contain category")
	}
	if !strings.Contains(prompt, "not_started|in_progress|complete|blocked") {
		t.Error("prompt should list valid states")
	}
}

func TestBuildDoctorPrompt_FallbackWhenDirMissingTemplate(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	issue := Issue{
		Node:        "test/node",
		Category:    CatMultipleInProgress,
		FixType:     FixModelAssisted,
		Description: "Multiple tasks in progress",
	}

	prompt := buildDoctorPrompt(dir, issue)
	if prompt == "" {
		t.Error("expected non-empty fallback prompt")
	}
	if !strings.Contains(prompt, "Multiple tasks in progress") {
		t.Error("prompt should contain issue description")
	}
}

func TestBuildDoctorPrompt_WithTemplate(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()

	promptDir := filepath.Join(dir, "system", "base", "prompts")
	_ = os.MkdirAll(promptDir, 0755)
	_ = os.WriteFile(filepath.Join(promptDir, "doctor.md"),
		[]byte(`Fix node {{.Node}}: {{.Category}} ({{.FixType}}) - {{.Description}}`), 0644)

	issue := Issue{
		Node:        "my/node",
		Category:    CatInvalidStateValue,
		FixType:     FixModelAssisted,
		Description: "bad state",
	}

	prompt := buildDoctorPrompt(dir, issue)
	if !strings.Contains(prompt, "my/node") {
		t.Error("expected node in rendered prompt")
	}
	if !strings.Contains(prompt, CatInvalidStateValue) {
		t.Error("expected category in rendered prompt")
	}
}

func TestDoctorPromptContext_Fields(t *testing.T) {
	t.Parallel()
	ctx := DoctorPromptContext{
		Node:        "test/node",
		Category:    CatMultipleInProgress,
		FixType:     FixModelAssisted,
		Description: "some description",
	}
	if ctx.Node != "test/node" {
		t.Errorf("unexpected Node: %q", ctx.Node)
	}
	if ctx.Category != CatMultipleInProgress {
		t.Errorf("unexpected Category: %q", ctx.Category)
	}
}

func TestTryModelAssistedFix_RequiresNodeAddress(t *testing.T) {
	t.Parallel()
	issue := Issue{Node: "", Category: CatMultipleInProgress}
	model := config.ModelDef{Command: "echo", Args: []string{"test"}}
	ok, err := TryModelAssistedFix(context.Background(), invoke.NewProcessInvoker(), model, issue, t.TempDir())
	if ok {
		t.Error("expected ok=false for empty node address")
	}
	if err == nil {
		t.Error("expected error for empty node address")
	}
	if !strings.Contains(err.Error(), "node address") {
		t.Errorf("expected 'node address' in error, got: %v", err)
	}
}
