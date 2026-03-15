//go:build integration

package integration

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestConfig_ThreeTierMerge(t *testing.T) {
	dir := t.TempDir()
	run(t, dir, "init")

	// Merge a custom poll interval and log_level override into the existing
	// config.local.json, preserving the identity that init created.
	mergeLocalConfig(t, dir, map[string]any{
		"daemon": map[string]any{
			"poll_interval_seconds": 42,
			"log_level":             "debug",
		},
	})

	// Run status --json to exercise config loading (the three-tier merge
	// resolves defaults <- config.json <- config.local.json)
	resp := runJSON(t, dir, "status")
	if !resp.OK {
		t.Fatalf("status command failed: %+v", resp)
	}
}

func TestConfig_NullDeletion(t *testing.T) {
	dir := t.TempDir()
	run(t, dir, "init")

	wcDir := filepath.Join(dir, ".wolfcastle")

	// Set identity to null in config.local.json. This null-deletes
	// the identity key from the merged config, causing the resolver
	// to fail because it requires identity to determine the namespace.
	localConfig := map[string]any{
		"identity": nil,
	}
	writeJSON(t, filepath.Join(wcDir, "config.local.json"), localConfig)

	// Status (which requires identity) should fail after null deletion
	out := runExpectError(t, dir, "status")
	if !strings.Contains(out, "identity") {
		t.Errorf("expected identity-related error after null deletion, got: %s", out)
	}
}

func TestConfig_PromptOverride(t *testing.T) {
	dir := t.TempDir()
	run(t, dir, "init")

	// Create a custom prompt override
	customDir := filepath.Join(dir, ".wolfcastle", "custom", "prompts")
	if err := os.MkdirAll(customDir, 0755); err != nil {
		t.Fatalf("creating custom prompts dir: %v", err)
	}
	customPrompt := "# Custom Execute\nThis is a custom execute prompt.\n"
	if err := os.WriteFile(filepath.Join(customDir, "execute.md"), []byte(customPrompt), 0644); err != nil {
		t.Fatalf("writing custom prompt: %v", err)
	}

	// Verify the custom prompt file exists (the daemon would use it during
	// prompt assembly, but we can verify it's in the right location)
	overridePath := filepath.Join(customDir, "execute.md")
	if _, err := os.Stat(overridePath); os.IsNotExist(err) {
		t.Error("custom prompt override file not created")
	}

	// Run a command that loads config to verify nothing breaks
	resp := runJSON(t, dir, "status")
	if !resp.OK {
		t.Fatalf("status failed with custom prompt: %+v", resp)
	}
}

func TestConfig_ModelDefinition(t *testing.T) {
	dir := t.TempDir()
	run(t, dir, "init")

	// Create a mock model script
	scriptPath := createMockModel(t, dir, "custom-model", "complete")

	// Configure it as the model for all tiers
	configureMockModels(t, dir, scriptPath)

	// Create a project and task so the daemon has work
	run(t, dir, "project", "create", "model-test")
	run(t, dir, "task", "add", "--node", "model-test", "test custom model")

	setMaxIterations(t, dir, 5)

	// Run the daemon, which exercises the custom model definition
	out := run(t, dir, "start")

	// The daemon should have used our mock model and completed the task
	if strings.Contains(out, "model") && strings.Contains(out, "not found") {
		t.Errorf("daemon failed to find custom model: %s", out)
	}

	ns := loadNode(t, dir, "model-test")
	for _, task := range ns.Tasks {
		if task.ID == "task-1" && task.State != "complete" {
			t.Errorf("expected task complete with custom model, got %s", task.State)
		}
	}
}

// writeJSON marshals v and writes it to path.
func writeJSON(t *testing.T, path string, v any) {
	t.Helper()
	data, err := json.MarshalIndent(v, "", "  ")
	if err != nil {
		t.Fatalf("marshaling JSON for %s: %v", path, err)
	}
	if err := os.WriteFile(path, data, 0644); err != nil {
		t.Fatalf("writing %s: %v", path, err)
	}
}
