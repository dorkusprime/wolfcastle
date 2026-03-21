package project

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	"github.com/dorkusprime/wolfcastle/internal/config"
	"github.com/dorkusprime/wolfcastle/internal/tierfs"
)

func newMigrationService(t *testing.T) (*MigrationService, string) {
	t.Helper()
	root := filepath.Join(t.TempDir(), ".wolfcastle")
	if err := os.MkdirAll(root, 0755); err != nil {
		t.Fatal(err)
	}
	tiers := tierfs.New(filepath.Join(root, "system"))
	cfg := config.NewConfigRepositoryWithTiers(tiers, root)
	return &MigrationService{config: cfg, root: root}, root
}

// --- MigrateDirectoryLayout ---

func TestMigrateDirectoryLayout_MovesFlatDirectories(t *testing.T) {
	t.Parallel()
	svc, root := newMigrationService(t)

	// Create old flat layout at root level.
	for _, d := range []string{"base", "custom", "local", "projects", "logs"} {
		if err := os.MkdirAll(filepath.Join(root, d), 0755); err != nil {
			t.Fatal(err)
		}
		// Drop a marker file so we can verify the move.
		if err := os.WriteFile(filepath.Join(root, d, "marker.txt"), []byte(d), 0644); err != nil {
			t.Fatal(err)
		}
	}

	if err := svc.MigrateDirectoryLayout(); err != nil {
		t.Fatal(err)
	}

	// Each directory should now live under system/.
	for _, d := range []string{"base", "custom", "local", "projects", "logs"} {
		marker := filepath.Join(root, "system", d, "marker.txt")
		data, err := os.ReadFile(marker)
		if err != nil {
			t.Errorf("expected system/%s/marker.txt to exist: %v", d, err)
			continue
		}
		if string(data) != d {
			t.Errorf("system/%s/marker.txt: got %q, want %q", d, data, d)
		}

		// Original should be gone.
		if _, err := os.Stat(filepath.Join(root, d)); !os.IsNotExist(err) {
			t.Errorf("old directory %s should have been moved", d)
		}
	}
}

func TestMigrateDirectoryLayout_MovesFiles(t *testing.T) {
	t.Parallel()
	svc, root := newMigrationService(t)

	// Need base/ to trigger migration (otherwise treated as fresh install).
	if err := os.MkdirAll(filepath.Join(root, "base"), 0755); err != nil {
		t.Fatal(err)
	}
	for _, f := range []string{"wolfcastle.pid", "stop", "daemon.log", "daemon.meta.json"} {
		if err := os.WriteFile(filepath.Join(root, f), []byte(f), 0644); err != nil {
			t.Fatal(err)
		}
	}

	if err := svc.MigrateDirectoryLayout(); err != nil {
		t.Fatal(err)
	}

	for _, f := range []string{"wolfcastle.pid", "stop", "daemon.log", "daemon.meta.json"} {
		data, err := os.ReadFile(filepath.Join(root, "system", f))
		if err != nil {
			t.Errorf("expected system/%s to exist: %v", f, err)
			continue
		}
		if string(data) != f {
			t.Errorf("system/%s: got %q, want %q", f, data, f)
		}
	}
}

func TestMigrateDirectoryLayout_IdempotentWhenSystemExists(t *testing.T) {
	t.Parallel()
	svc, root := newMigrationService(t)

	// Create system/ so migration sees it and returns early.
	if err := os.MkdirAll(filepath.Join(root, "system"), 0755); err != nil {
		t.Fatal(err)
	}

	// Also create a flat directory that should NOT be moved.
	if err := os.MkdirAll(filepath.Join(root, "base"), 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, "base", "sentinel.txt"), []byte("stay"), 0644); err != nil {
		t.Fatal(err)
	}

	if err := svc.MigrateDirectoryLayout(); err != nil {
		t.Fatal(err)
	}

	// base/ at root should still be there; migration was a no-op.
	if _, err := os.Stat(filepath.Join(root, "base", "sentinel.txt")); err != nil {
		t.Error("base/sentinel.txt should still exist when system/ already present")
	}
}

func TestMigrateDirectoryLayout_FreshInstallation(t *testing.T) {
	t.Parallel()
	svc, root := newMigrationService(t)

	// No old layout, no system/ directory. Migration should just create system/.
	if err := svc.MigrateDirectoryLayout(); err != nil {
		t.Fatal(err)
	}

	info, err := os.Stat(filepath.Join(root, "system"))
	if err != nil {
		t.Fatal("system/ should be created on fresh install:", err)
	}
	if !info.IsDir() {
		t.Error("system/ should be a directory")
	}
}

// --- MigrateOldConfig ---

func TestMigrateOldConfig_MovesRootConfigToCustomTier(t *testing.T) {
	t.Parallel()
	svc, root := newMigrationService(t)

	// Ensure system/custom exists.
	if err := os.MkdirAll(filepath.Join(root, "system", "custom"), 0755); err != nil {
		t.Fatal(err)
	}

	oldCfg := `{"failure": {"hard_cap": 42}}`
	if err := os.WriteFile(filepath.Join(root, "config.json"), []byte(oldCfg), 0644); err != nil {
		t.Fatal(err)
	}

	if err := svc.MigrateOldConfig(); err != nil {
		t.Fatal(err)
	}

	// custom/config.json should contain the old content.
	data, err := os.ReadFile(filepath.Join(root, "system", "custom", "config.json"))
	if err != nil {
		t.Fatal("custom/config.json should exist:", err)
	}
	if string(data) != oldCfg {
		t.Errorf("custom/config.json: got %s, want %s", data, oldCfg)
	}

	// Old file should be removed.
	if _, err := os.Stat(filepath.Join(root, "config.json")); !os.IsNotExist(err) {
		t.Error("root config.json should be removed after migration")
	}
}

func TestMigrateOldConfig_SkipsMoveWhenCustomConfigExists(t *testing.T) {
	t.Parallel()
	svc, root := newMigrationService(t)

	customDir := filepath.Join(root, "system", "custom")
	if err := os.MkdirAll(customDir, 0755); err != nil {
		t.Fatal(err)
	}

	// Pre-existing custom config.
	existing := `{"existing": true}`
	if err := os.WriteFile(filepath.Join(customDir, "config.json"), []byte(existing), 0644); err != nil {
		t.Fatal(err)
	}

	// Old root config that should NOT overwrite the existing custom one.
	if err := os.WriteFile(filepath.Join(root, "config.json"), []byte(`{"old": true}`), 0644); err != nil {
		t.Fatal(err)
	}

	if err := svc.MigrateOldConfig(); err != nil {
		t.Fatal(err)
	}

	// Custom config should be unchanged.
	data, err := os.ReadFile(filepath.Join(customDir, "config.json"))
	if err != nil {
		t.Fatal(err)
	}
	if string(data) != existing {
		t.Error("custom/config.json should not be overwritten when it already exists")
	}

	// Old file still gets removed.
	if _, err := os.Stat(filepath.Join(root, "config.json")); !os.IsNotExist(err) {
		t.Error("root config.json should still be removed")
	}
}

func TestMigrateOldConfig_MergesLocalConfig(t *testing.T) {
	t.Parallel()
	svc, root := newMigrationService(t)

	localDir := filepath.Join(root, "system", "local")
	if err := os.MkdirAll(localDir, 0755); err != nil {
		t.Fatal(err)
	}

	// Existing local config with some keys.
	existingLocal := map[string]any{"keep_me": "yes"}
	writeTestJSON(t, filepath.Join(localDir, "config.json"), existingLocal)

	// Old config.local.json with keys to merge in.
	oldLocal := map[string]any{
		"identity": map[string]any{"user": "alice", "machine": "box"},
		"extra":    "value",
	}
	writeTestJSON(t, filepath.Join(root, "config.local.json"), oldLocal)

	if err := svc.MigrateOldConfig(); err != nil {
		t.Fatal(err)
	}

	data, err := os.ReadFile(filepath.Join(localDir, "config.json"))
	if err != nil {
		t.Fatal(err)
	}
	var result map[string]any
	if err := json.Unmarshal(data, &result); err != nil {
		t.Fatal(err)
	}

	// Both old and existing keys should be present.
	if result["keep_me"] != "yes" {
		t.Error("existing local keys should be preserved")
	}
	if result["extra"] != "value" {
		t.Error("merged keys from config.local.json should be present")
	}
	identity, _ := result["identity"].(map[string]any)
	if identity["user"] != "alice" {
		t.Error("identity from config.local.json should be merged")
	}

	// Old file should be removed.
	if _, err := os.Stat(filepath.Join(root, "config.local.json")); !os.IsNotExist(err) {
		t.Error("config.local.json should be removed after migration")
	}
}

func TestMigrateOldConfig_HandlesMissingSources(t *testing.T) {
	t.Parallel()
	svc, _ := newMigrationService(t)

	// No config.json or config.local.json exist. Should succeed silently.
	if err := svc.MigrateOldConfig(); err != nil {
		t.Fatal("MigrateOldConfig should handle missing source files gracefully:", err)
	}
}

// --- MigratePromptLayout ---

func TestMigratePromptLayout_MovesFlatPrompts(t *testing.T) {
	t.Parallel()
	svc, root := newMigrationService(t)

	// Set up a base tier with flat prompt files.
	basePrompts := filepath.Join(root, "system", "base", "prompts")
	if err := os.MkdirAll(basePrompts, 0755); err != nil {
		t.Fatal(err)
	}
	for _, f := range []string{"execute.md", "intake.md", "plan-initial.md", "audit.md", "summary.md"} {
		if err := os.WriteFile(filepath.Join(basePrompts, f), []byte("content:"+f), 0644); err != nil {
			t.Fatal(err)
		}
	}

	if err := svc.MigratePromptLayout(); err != nil {
		t.Fatal(err)
	}

	// Stage files should be under stages/.
	for _, f := range []string{"execute.md", "intake.md", "plan-initial.md"} {
		data, err := os.ReadFile(filepath.Join(basePrompts, "stages", f))
		if err != nil {
			t.Errorf("expected stages/%s to exist: %v", f, err)
			continue
		}
		if string(data) != "content:"+f {
			t.Errorf("stages/%s content mismatch", f)
		}
		// Original should be gone.
		if _, err := os.Stat(filepath.Join(basePrompts, f)); !os.IsNotExist(err) {
			t.Errorf("%s should have moved out of flat prompts/", f)
		}
	}

	// Audit file should be under audits/.
	data, err := os.ReadFile(filepath.Join(basePrompts, "audits", "audit.md"))
	if err != nil {
		t.Fatal("expected audits/audit.md:", err)
	}
	if string(data) != "content:audit.md" {
		t.Error("audits/audit.md content mismatch")
	}

	// summary.md stays in the root prompts/ (not a stage or audit prompt).
	if _, err := os.Stat(filepath.Join(basePrompts, "summary.md")); err != nil {
		t.Error("summary.md should remain in flat prompts/:", err)
	}
}

func TestMigratePromptLayout_IdempotentWhenStagesExist(t *testing.T) {
	t.Parallel()
	svc, root := newMigrationService(t)

	basePrompts := filepath.Join(root, "system", "base", "prompts")
	stagesDir := filepath.Join(basePrompts, "stages")
	if err := os.MkdirAll(stagesDir, 0755); err != nil {
		t.Fatal(err)
	}
	// Write a file directly into stages/ (already migrated).
	if err := os.WriteFile(filepath.Join(stagesDir, "execute.md"), []byte("already here"), 0644); err != nil {
		t.Fatal(err)
	}
	// Also write a flat file that should NOT be touched.
	if err := os.WriteFile(filepath.Join(basePrompts, "intake.md"), []byte("flat"), 0644); err != nil {
		t.Fatal(err)
	}

	if err := svc.MigratePromptLayout(); err != nil {
		t.Fatal(err)
	}

	// stages/execute.md should remain untouched.
	data, err := os.ReadFile(filepath.Join(stagesDir, "execute.md"))
	if err != nil {
		t.Fatal(err)
	}
	if string(data) != "already here" {
		t.Error("stages/execute.md should not be overwritten")
	}

	// Flat intake.md should remain (tier skipped since stages/ exists).
	if _, err := os.Stat(filepath.Join(basePrompts, "intake.md")); err != nil {
		t.Error("intake.md should remain when migration is skipped")
	}
}

func TestMigratePromptLayout_HandlesCustomAndLocalTiers(t *testing.T) {
	t.Parallel()
	svc, root := newMigrationService(t)

	// Set up custom tier with flat execute.md override.
	customPrompts := filepath.Join(root, "system", "custom", "prompts")
	if err := os.MkdirAll(customPrompts, 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(customPrompts, "execute.md"), []byte("custom"), 0644); err != nil {
		t.Fatal(err)
	}

	if err := svc.MigratePromptLayout(); err != nil {
		t.Fatal(err)
	}

	// Custom execute.md should move to stages/.
	data, err := os.ReadFile(filepath.Join(customPrompts, "stages", "execute.md"))
	if err != nil {
		t.Fatal("expected custom/prompts/stages/execute.md:", err)
	}
	if string(data) != "custom" {
		t.Error("custom/prompts/stages/execute.md content mismatch")
	}
}

func TestMigratePromptLayout_HandlesMissingPromptDir(t *testing.T) {
	t.Parallel()
	svc, _ := newMigrationService(t)

	// No prompts/ directory at all. Should succeed silently.
	if err := svc.MigratePromptLayout(); err != nil {
		t.Fatal("MigratePromptLayout should handle missing prompts/ gracefully:", err)
	}
}

// --- MigrateStagesFormat ---

// writeTierConfig writes a JSON config to the given tier's config.json,
// creating intermediate directories as needed.
func writeTierConfig(t *testing.T, root, tier string, cfg map[string]any) {
	t.Helper()
	dir := filepath.Join(root, "system", tier)
	if err := os.MkdirAll(dir, 0755); err != nil {
		t.Fatal(err)
	}
	writeTestJSON(t, filepath.Join(dir, "config.json"), cfg)
}

// readTierConfig reads and unmarshals a tier's config.json.
func readTierConfig(t *testing.T, root, tier string) map[string]any {
	t.Helper()
	data, err := os.ReadFile(filepath.Join(root, "system", tier, "config.json"))
	if err != nil {
		t.Fatal(err)
	}
	var result map[string]any
	if err := json.Unmarshal(data, &result); err != nil {
		t.Fatal(err)
	}
	return result
}

func TestMigrateStagesFormat_ConvertsArrayToMap(t *testing.T) {
	t.Parallel()
	svc, root := newMigrationService(t)

	writeTierConfig(t, root, "custom", map[string]any{
		"pipeline": map[string]any{
			"stages": []any{
				map[string]any{"name": "intake", "prompt": "intake.md", "model": "fast"},
				map[string]any{"name": "execute", "prompt": "execute.md"},
				map[string]any{"name": "review", "prompt": "review.md", "retries": float64(3)},
			},
		},
	})

	if err := svc.MigrateStagesFormat(); err != nil {
		t.Fatal(err)
	}

	cfg := readTierConfig(t, root, "custom")
	pipeline := cfg["pipeline"].(map[string]any)
	stages := pipeline["stages"].(map[string]any)

	// Verify each stage was converted, name field stripped.
	intake := stages["intake"].(map[string]any)
	if intake["prompt"] != "intake.md" || intake["model"] != "fast" {
		t.Errorf("intake stage: got %v", intake)
	}
	if _, hasName := intake["name"]; hasName {
		t.Error("name field should be stripped from stage object")
	}

	execute := stages["execute"].(map[string]any)
	if execute["prompt"] != "execute.md" {
		t.Errorf("execute stage: got %v", execute)
	}

	review := stages["review"].(map[string]any)
	if review["retries"] != float64(3) {
		t.Errorf("review stage: got %v", review)
	}

	// Verify stage_order preserves array ordering.
	orderRaw := pipeline["stage_order"].([]any)
	order := make([]string, len(orderRaw))
	for i, v := range orderRaw {
		order[i] = v.(string)
	}
	expected := []string{"intake", "execute", "review"}
	if len(order) != len(expected) {
		t.Fatalf("stage_order length: got %d, want %d", len(order), len(expected))
	}
	for i := range expected {
		if order[i] != expected[i] {
			t.Errorf("stage_order[%d]: got %q, want %q", i, order[i], expected[i])
		}
	}
}

func TestMigrateStagesFormat_AlreadyMigratedIsNoop(t *testing.T) {
	t.Parallel()
	svc, root := newMigrationService(t)

	original := map[string]any{
		"pipeline": map[string]any{
			"stages": map[string]any{
				"intake":  map[string]any{"prompt": "intake.md"},
				"execute": map[string]any{"prompt": "execute.md"},
			},
			"stage_order": []any{"intake", "execute"},
		},
	}
	writeTierConfig(t, root, "base", original)

	// Read file content before migration.
	before, err := os.ReadFile(filepath.Join(root, "system", "base", "config.json"))
	if err != nil {
		t.Fatal(err)
	}

	if err := svc.MigrateStagesFormat(); err != nil {
		t.Fatal(err)
	}

	after, err := os.ReadFile(filepath.Join(root, "system", "base", "config.json"))
	if err != nil {
		t.Fatal(err)
	}

	if string(before) != string(after) {
		t.Error("already-migrated config should not be rewritten")
	}
}

func TestMigrateStagesFormat_EmptyArray(t *testing.T) {
	t.Parallel()
	svc, root := newMigrationService(t)

	writeTierConfig(t, root, "local", map[string]any{
		"pipeline": map[string]any{
			"stages": []any{},
		},
	})

	if err := svc.MigrateStagesFormat(); err != nil {
		t.Fatal(err)
	}

	cfg := readTierConfig(t, root, "local")
	pipeline := cfg["pipeline"].(map[string]any)
	stages := pipeline["stages"].(map[string]any)

	if len(stages) != 0 {
		t.Errorf("empty array should produce empty map, got %v", stages)
	}

	// stage_order should not be set for empty arrays.
	if _, hasOrder := pipeline["stage_order"]; hasOrder {
		t.Error("empty stages should not produce stage_order")
	}
}

func TestMigrateStagesFormat_SingleStage(t *testing.T) {
	t.Parallel()
	svc, root := newMigrationService(t)

	writeTierConfig(t, root, "base", map[string]any{
		"pipeline": map[string]any{
			"stages": []any{
				map[string]any{"name": "only", "prompt": "only.md"},
			},
		},
	})

	if err := svc.MigrateStagesFormat(); err != nil {
		t.Fatal(err)
	}

	cfg := readTierConfig(t, root, "base")
	pipeline := cfg["pipeline"].(map[string]any)
	stages := pipeline["stages"].(map[string]any)

	if len(stages) != 1 {
		t.Fatalf("expected 1 stage, got %d", len(stages))
	}
	only := stages["only"].(map[string]any)
	if only["prompt"] != "only.md" {
		t.Errorf("single stage: got %v", only)
	}

	orderRaw := pipeline["stage_order"].([]any)
	if len(orderRaw) != 1 || orderRaw[0] != "only" {
		t.Errorf("stage_order: got %v, want [only]", orderRaw)
	}
}

func TestMigrateStagesFormat_MissingNameSkipped(t *testing.T) {
	t.Parallel()
	svc, root := newMigrationService(t)

	writeTierConfig(t, root, "custom", map[string]any{
		"pipeline": map[string]any{
			"stages": []any{
				map[string]any{"name": "good", "prompt": "good.md"},
				map[string]any{"prompt": "no-name.md"},           // missing name
				map[string]any{"name": "", "prompt": "empty.md"}, // empty name
				map[string]any{"name": float64(42)},              // non-string name
				"not-a-map",                                       // not a map at all
				map[string]any{"name": "also-good", "prompt": "also.md"},
			},
		},
	})

	if err := svc.MigrateStagesFormat(); err != nil {
		t.Fatal(err)
	}

	cfg := readTierConfig(t, root, "custom")
	pipeline := cfg["pipeline"].(map[string]any)
	stages := pipeline["stages"].(map[string]any)

	if len(stages) != 2 {
		t.Fatalf("expected 2 valid stages, got %d: %v", len(stages), stages)
	}
	if _, ok := stages["good"]; !ok {
		t.Error("stage 'good' should be present")
	}
	if _, ok := stages["also-good"]; !ok {
		t.Error("stage 'also-good' should be present")
	}

	orderRaw := pipeline["stage_order"].([]any)
	if len(orderRaw) != 2 {
		t.Fatalf("stage_order should have 2 entries, got %v", orderRaw)
	}
}

func TestMigrateStagesFormat_DuplicateNamesLastWins(t *testing.T) {
	t.Parallel()
	svc, root := newMigrationService(t)

	writeTierConfig(t, root, "base", map[string]any{
		"pipeline": map[string]any{
			"stages": []any{
				map[string]any{"name": "dup", "prompt": "first.md"},
				map[string]any{"name": "unique", "prompt": "unique.md"},
				map[string]any{"name": "dup", "prompt": "second.md"},
			},
		},
	})

	if err := svc.MigrateStagesFormat(); err != nil {
		t.Fatal(err)
	}

	cfg := readTierConfig(t, root, "base")
	pipeline := cfg["pipeline"].(map[string]any)
	stages := pipeline["stages"].(map[string]any)

	dup := stages["dup"].(map[string]any)
	if dup["prompt"] != "second.md" {
		t.Errorf("duplicate name should keep last value: got prompt=%v, want second.md", dup["prompt"])
	}

	// stage_order should not have duplicates.
	orderRaw := pipeline["stage_order"].([]any)
	dupCount := 0
	for _, v := range orderRaw {
		if v == "dup" {
			dupCount++
		}
	}
	if dupCount != 1 {
		t.Errorf("stage_order should contain 'dup' exactly once, got %d times in %v", dupCount, orderRaw)
	}
}

func TestMigrateStagesFormat_TierFileDoesNotExist(t *testing.T) {
	t.Parallel()
	svc, _ := newMigrationService(t)

	// No tier config files exist at all. Should succeed silently.
	if err := svc.MigrateStagesFormat(); err != nil {
		t.Fatal("missing tier files should not cause an error:", err)
	}
}

func TestMigrateStagesFormat_PipelineKeyAbsent(t *testing.T) {
	t.Parallel()
	svc, root := newMigrationService(t)

	// Config exists but has no pipeline key.
	writeTierConfig(t, root, "base", map[string]any{
		"identity": map[string]any{"user": "test"},
	})

	before, err := os.ReadFile(filepath.Join(root, "system", "base", "config.json"))
	if err != nil {
		t.Fatal(err)
	}

	if err := svc.MigrateStagesFormat(); err != nil {
		t.Fatal(err)
	}

	after, err := os.ReadFile(filepath.Join(root, "system", "base", "config.json"))
	if err != nil {
		t.Fatal(err)
	}

	if string(before) != string(after) {
		t.Error("config without pipeline key should not be modified")
	}
}

func TestMigrateStagesFormat_PreservesExistingStageOrder(t *testing.T) {
	t.Parallel()
	svc, root := newMigrationService(t)

	writeTierConfig(t, root, "custom", map[string]any{
		"pipeline": map[string]any{
			"stages": []any{
				map[string]any{"name": "a", "prompt": "a.md"},
				map[string]any{"name": "b", "prompt": "b.md"},
			},
			"stage_order": []any{"b", "a"},
		},
	})

	if err := svc.MigrateStagesFormat(); err != nil {
		t.Fatal(err)
	}

	cfg := readTierConfig(t, root, "custom")
	pipeline := cfg["pipeline"].(map[string]any)

	// Existing stage_order should be preserved, not overwritten.
	orderRaw := pipeline["stage_order"].([]any)
	if len(orderRaw) != 2 || orderRaw[0] != "b" || orderRaw[1] != "a" {
		t.Errorf("existing stage_order should be preserved: got %v, want [b a]", orderRaw)
	}
}

func TestMigrateStagesFormat_InvalidJSON(t *testing.T) {
	t.Parallel()
	svc, root := newMigrationService(t)

	// Write invalid JSON to a tier config.
	dir := filepath.Join(root, "system", "local")
	if err := os.MkdirAll(dir, 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "config.json"), []byte("{not json"), 0644); err != nil {
		t.Fatal(err)
	}

	// Should skip gracefully, not error.
	if err := svc.MigrateStagesFormat(); err != nil {
		t.Fatal("invalid JSON should be skipped, not cause error:", err)
	}
}

func TestMigrateStagesFormat_PipelineNotAMap(t *testing.T) {
	t.Parallel()
	svc, root := newMigrationService(t)

	// pipeline is a string instead of a map.
	writeTierConfig(t, root, "base", map[string]any{
		"pipeline": "not-a-map",
	})

	if err := svc.MigrateStagesFormat(); err != nil {
		t.Fatal("non-map pipeline should be skipped:", err)
	}
}

func TestMigrateStagesFormat_StagesAbsentFromPipeline(t *testing.T) {
	t.Parallel()
	svc, root := newMigrationService(t)

	// Pipeline exists but has no stages key.
	writeTierConfig(t, root, "base", map[string]any{
		"pipeline": map[string]any{
			"timeout": float64(30),
		},
	})

	if err := svc.MigrateStagesFormat(); err != nil {
		t.Fatal("missing stages key should be skipped:", err)
	}
}

func TestMigrateStagesFormat_StagesNotArrayOrMap(t *testing.T) {
	t.Parallel()
	svc, root := newMigrationService(t)

	// stages is a string, neither array nor map.
	writeTierConfig(t, root, "base", map[string]any{
		"pipeline": map[string]any{
			"stages": "unexpected-type",
		},
	})

	if err := svc.MigrateStagesFormat(); err != nil {
		t.Fatal("non-array/map stages should be skipped:", err)
	}
}

func TestMigrateStagesFormat_ReadError(t *testing.T) {
	t.Parallel()
	svc, root := newMigrationService(t)

	dir := filepath.Join(root, "system", "base")
	if err := os.MkdirAll(dir, 0755); err != nil {
		t.Fatal(err)
	}
	// Create config.json as a directory to trigger a read error.
	if err := os.MkdirAll(filepath.Join(dir, "config.json"), 0755); err != nil {
		t.Fatal(err)
	}

	err := svc.MigrateStagesFormat()
	if err == nil {
		t.Fatal("expected error when config.json is unreadable")
	}
}

func TestMigrateStagesFormat_MultipleTiers(t *testing.T) {
	t.Parallel()
	svc, root := newMigrationService(t)

	// Array-format stages in both base and custom tiers.
	for _, tier := range []string{"base", "custom"} {
		writeTierConfig(t, root, tier, map[string]any{
			"pipeline": map[string]any{
				"stages": []any{
					map[string]any{"name": tier + "-stage", "prompt": tier + ".md"},
				},
			},
		})
	}

	if err := svc.MigrateStagesFormat(); err != nil {
		t.Fatal(err)
	}

	for _, tier := range []string{"base", "custom"} {
		cfg := readTierConfig(t, root, tier)
		pipeline := cfg["pipeline"].(map[string]any)
		stages := pipeline["stages"].(map[string]any)
		stageName := tier + "-stage"
		if _, ok := stages[stageName]; !ok {
			t.Errorf("%s tier: expected stage %q in migrated output", tier, stageName)
		}
	}
}

func writeTestJSON(t *testing.T, path string, v any) {
	t.Helper()
	data, err := json.MarshalIndent(v, "", "  ")
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, data, 0644); err != nil {
		t.Fatal(err)
	}
}
