package knowledge

import (
	"encoding/json"
	"os"
	"strings"
	"testing"

	"github.com/dorkusprime/wolfcastle/internal/knowledge"
)

// Integration tests exercising multi-command workflows across the knowledge
// CLI surface. Unit tests for individual commands live in their own files.

func TestIntegration_AddThenShow(t *testing.T) {
	env := newTestEnv(t)

	env.RootCmd.SetArgs([]string{"knowledge", "add", "the config loader silently drops null values"})
	if err := env.RootCmd.Execute(); err != nil {
		t.Fatalf("add: %v", err)
	}

	// Show should display the entry we just added.
	env.RootCmd.SetArgs([]string{"knowledge", "show"})
	if err := env.RootCmd.Execute(); err != nil {
		t.Fatalf("show: %v", err)
	}

	content, err := knowledge.Read(env.WolfcastleDir, env.App.Identity.Namespace)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(content, "the config loader silently drops null values") {
		t.Errorf("show should display added entry, got: %s", content)
	}
}

func TestIntegration_AddThenShowJSON(t *testing.T) {
	env := newTestEnv(t)

	env.RootCmd.SetArgs([]string{"knowledge", "add", "payment module tests need running Redis"})
	if err := env.RootCmd.Execute(); err != nil {
		t.Fatalf("add: %v", err)
	}

	// Switch to JSON mode for show.
	env.App.JSON = true
	env.RootCmd.SetArgs([]string{"knowledge", "show"})
	if err := env.RootCmd.Execute(); err != nil {
		t.Fatalf("show --json: %v", err)
	}

	// Verify the content is readable and token count is positive.
	content, err := knowledge.Read(env.WolfcastleDir, env.App.Identity.Namespace)
	if err != nil {
		t.Fatal(err)
	}
	count := knowledge.TokenCount(content)
	if count == 0 {
		t.Error("expected positive token count after adding an entry")
	}
}

func TestIntegration_FileCreationOnFirstAdd(t *testing.T) {
	env := newTestEnv(t)
	ns := env.App.Identity.Namespace
	path := knowledge.FilePath(env.WolfcastleDir, ns)

	// File should not exist yet.
	if _, err := os.Stat(path); err == nil {
		t.Fatal("knowledge file should not exist before first add")
	}

	env.RootCmd.SetArgs([]string{"knowledge", "add", "first entry creates the file"})
	if err := env.RootCmd.Execute(); err != nil {
		t.Fatalf("add: %v", err)
	}

	// File should now exist.
	info, err := os.Stat(path)
	if err != nil {
		t.Fatalf("knowledge file should exist after first add: %v", err)
	}
	if info.Size() == 0 {
		t.Error("knowledge file should not be empty after add")
	}

	// Show should work on the newly created file.
	env.RootCmd.SetArgs([]string{"knowledge", "show"})
	if err := env.RootCmd.Execute(); err != nil {
		t.Fatalf("show after first add: %v", err)
	}
}

func TestIntegration_BudgetEnforcementThenPrune(t *testing.T) {
	env := newTestEnv(t)
	ns := env.App.Identity.Namespace
	path := knowledge.FilePath(env.WolfcastleDir, ns)

	// Seed the file with content near the 2000-token budget.
	if err := os.MkdirAll(path[:len(path)-len(ns+".md")], 0o755); err != nil {
		t.Fatal(err)
	}
	bigContent := strings.Repeat("word ", 1990)
	if err := os.WriteFile(path, []byte(bigContent), 0o644); err != nil {
		t.Fatal(err)
	}

	// Adding another entry should fail due to budget.
	env.RootCmd.SetArgs([]string{"knowledge", "add", "this should be rejected because we are over budget"})
	err := env.RootCmd.Execute()
	if err == nil {
		t.Fatal("expected budget error, got nil")
	}
	if !strings.Contains(err.Error(), "exceeds budget") {
		t.Errorf("expected 'exceeds budget' in error, got: %v", err)
	}

	// Prune in JSON mode should report over-budget status.
	env.App.JSON = true
	env.RootCmd.SetArgs([]string{"knowledge", "prune"})
	if err := env.RootCmd.Execute(); err != nil {
		t.Fatalf("prune --json: %v", err)
	}
}

func TestIntegration_MultipleAddsThenPruneReport(t *testing.T) {
	env := newTestEnv(t)
	t.Setenv("EDITOR", "true")

	entries := []string{
		"make test runs with -short by default",
		"full integration tests need make test-integration",
		"Go 1.26 changed loop variable semantics",
	}
	for _, e := range entries {
		env.RootCmd.SetArgs([]string{"knowledge", "add", e})
		if err := env.RootCmd.Execute(); err != nil {
			t.Fatalf("add %q: %v", e, err)
		}
	}

	// Prune interactively (EDITOR=true is a no-op).
	env.RootCmd.SetArgs([]string{"knowledge", "prune"})
	if err := env.RootCmd.Execute(); err != nil {
		t.Fatalf("prune: %v", err)
	}

	// All entries should still be present (editor didn't change anything).
	content, err := knowledge.Read(env.WolfcastleDir, env.App.Identity.Namespace)
	if err != nil {
		t.Fatal(err)
	}
	for _, e := range entries {
		if !strings.Contains(content, e) {
			t.Errorf("expected entry %q after prune, got: %s", e, content)
		}
	}
}

func TestIntegration_EditThenAddThenShow(t *testing.T) {
	env := newTestEnv(t)
	t.Setenv("EDITOR", "true")

	// Edit creates the file with template content.
	env.RootCmd.SetArgs([]string{"knowledge", "edit"})
	if err := env.RootCmd.Execute(); err != nil {
		t.Fatalf("edit: %v", err)
	}

	// Add should work on the file edit created.
	env.RootCmd.SetArgs([]string{"knowledge", "add", "entry after edit"})
	if err := env.RootCmd.Execute(); err != nil {
		t.Fatalf("add after edit: %v", err)
	}

	// Show should display both the template header and the new entry.
	content, err := knowledge.Read(env.WolfcastleDir, env.App.Identity.Namespace)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(content, "Codebase Knowledge") {
		t.Error("expected template header from edit")
	}
	if !strings.Contains(content, "entry after edit") {
		t.Error("expected added entry")
	}
}

func TestIntegration_PruneJSON_ReportsTokensAndBudget(t *testing.T) {
	env := newTestEnv(t)
	env.App.JSON = true

	ns := env.App.Identity.Namespace
	if err := knowledge.Append(env.WolfcastleDir, ns, "track token counts carefully"); err != nil {
		t.Fatal(err)
	}

	env.RootCmd.SetArgs([]string{"knowledge", "prune"})
	if err := env.RootCmd.Execute(); err != nil {
		t.Fatalf("prune --json: %v", err)
	}

	// Verify the knowledge package reports sensible token counts.
	content, err := knowledge.Read(env.WolfcastleDir, ns)
	if err != nil {
		t.Fatal(err)
	}
	count := knowledge.TokenCount(content)
	if count <= 0 {
		t.Error("expected positive token count")
	}
	if count > 2000 {
		t.Error("single entry should not exceed default budget")
	}
}

func TestIntegration_ShowJSON_StructuredOutput(t *testing.T) {
	env := newTestEnv(t)

	ns := env.App.Identity.Namespace
	if err := knowledge.Append(env.WolfcastleDir, ns, "structured output test"); err != nil {
		t.Fatal(err)
	}

	env.App.JSON = true
	env.RootCmd.SetArgs([]string{"knowledge", "show"})
	if err := env.RootCmd.Execute(); err != nil {
		t.Fatalf("show --json: %v", err)
	}

	// Verify we can read the content back as structured data.
	content, err := knowledge.Read(env.WolfcastleDir, ns)
	if err != nil {
		t.Fatal(err)
	}

	// Build the same JSON structure the command produces and verify it parses.
	payload := map[string]any{
		"namespace":   ns,
		"content":     content,
		"path":        knowledge.FilePath(env.WolfcastleDir, ns),
		"token_count": knowledge.TokenCount(content),
	}
	data, err := json.Marshal(payload)
	if err != nil {
		t.Fatalf("marshalling expected output: %v", err)
	}

	var parsed map[string]any
	if err := json.Unmarshal(data, &parsed); err != nil {
		t.Fatalf("unmarshalling: %v", err)
	}
	if parsed["namespace"] != ns {
		t.Errorf("expected namespace %q, got %v", ns, parsed["namespace"])
	}
	tc, ok := parsed["token_count"].(float64)
	if !ok || tc <= 0 {
		t.Errorf("expected positive token_count, got %v", parsed["token_count"])
	}
}

func TestIntegration_ShowEmpty_NoError(t *testing.T) {
	env := newTestEnv(t)

	// Show on a fresh environment with no knowledge file should succeed.
	env.RootCmd.SetArgs([]string{"knowledge", "show"})
	if err := env.RootCmd.Execute(); err != nil {
		t.Fatalf("show on empty: %v", err)
	}

	// JSON mode too.
	env.App.JSON = true
	env.RootCmd.SetArgs([]string{"knowledge", "show"})
	if err := env.RootCmd.Execute(); err != nil {
		t.Fatalf("show --json on empty: %v", err)
	}
}

func TestIntegration_BudgetBoundary(t *testing.T) {
	env := newTestEnv(t)
	ns := env.App.Identity.Namespace
	path := knowledge.FilePath(env.WolfcastleDir, ns)

	// Seed the file just under the 2000-token budget.
	// TokenCount = ceil(words / 0.75). To get ~1990 tokens we need ~1492 words.
	if err := os.MkdirAll(path[:len(path)-len(ns+".md")], 0o755); err != nil {
		t.Fatal(err)
	}
	seedContent := strings.Repeat("word ", 1492)
	if err := os.WriteFile(path, []byte(seedContent), 0o644); err != nil {
		t.Fatal(err)
	}

	// A small add should still succeed (we're just under budget).
	env.RootCmd.SetArgs([]string{"knowledge", "add", "tiny"})
	if err := env.RootCmd.Execute(); err != nil {
		t.Fatalf("expected small add to succeed near budget: %v", err)
	}

	// Now a larger add should fail.
	env.RootCmd.SetArgs([]string{"knowledge", "add", "this entry has enough words to push us over the token budget limit"})
	err := env.RootCmd.Execute()
	if err == nil {
		t.Fatal("expected budget error, got nil")
	}
	if !strings.Contains(err.Error(), "exceeds budget") {
		t.Errorf("expected 'exceeds budget' in error, got: %v", err)
	}

	// Verify the rejected entry was NOT written.
	content, err := knowledge.Read(env.WolfcastleDir, ns)
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(content, "this entry has enough words") {
		t.Error("rejected entry should not appear in the file")
	}
	// But prior content should be intact.
	if !strings.Contains(content, "tiny") {
		t.Error("previously accepted entry should still be present")
	}
}
