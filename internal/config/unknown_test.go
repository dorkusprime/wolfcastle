package config

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestCheckUnknownFields_SingleTopLevelField(t *testing.T) {
	t.Parallel()
	raw := []byte(`{"modles": {}}`)
	warnings := checkUnknownFields(raw, "local/config.json")

	if len(warnings) != 1 {
		t.Fatalf("expected 1 warning, got %d: %v", len(warnings), warnings)
	}
	if !strings.Contains(warnings[0], "modles") {
		t.Errorf("expected warning to mention 'modles', got: %s", warnings[0])
	}
	if !strings.Contains(warnings[0], "local/config.json") {
		t.Errorf("expected warning to mention tier, got: %s", warnings[0])
	}
}

func TestCheckUnknownFields_NestedUnknownField(t *testing.T) {
	t.Parallel()
	raw := []byte(`{"pipeline": {"planing": {}}}`)
	warnings := checkUnknownFields(raw, "custom/config.json")

	if len(warnings) != 1 {
		t.Fatalf("expected 1 warning, got %d: %v", len(warnings), warnings)
	}
	if !strings.Contains(warnings[0], "planing") {
		t.Errorf("expected warning to mention 'planing', got: %s", warnings[0])
	}
	if !strings.Contains(warnings[0], "pipeline.planing") {
		t.Errorf("expected warning to include full path 'pipeline.planing', got: %s", warnings[0])
	}
}

func TestCheckUnknownFields_MultipleUnknownFields(t *testing.T) {
	t.Parallel()
	raw := []byte(`{"modles": {}, "pipleine": {}}`)
	warnings := checkUnknownFields(raw, "base/config.json")

	if len(warnings) != 2 {
		t.Fatalf("expected 2 warnings, got %d: %v", len(warnings), warnings)
	}
}

func TestCheckUnknownFields_NoUnknownFields(t *testing.T) {
	t.Parallel()
	raw := []byte(`{"failure": {"hard_cap": 100}}`)
	warnings := checkUnknownFields(raw, "local/config.json")

	if len(warnings) != 0 {
		t.Errorf("expected 0 warnings for clean config, got %d: %v", len(warnings), warnings)
	}
}

func TestCheckUnknownFields_EmptyObject(t *testing.T) {
	t.Parallel()
	warnings := checkUnknownFields([]byte(`{}`), "base/config.json")

	if len(warnings) != 0 {
		t.Errorf("expected 0 warnings for empty object, got %d: %v", len(warnings), warnings)
	}
}

func TestCheckUnknownFields_InvalidJSON_ReturnsNil(t *testing.T) {
	t.Parallel()
	warnings := checkUnknownFields([]byte(`{broken`), "local/config.json")

	if warnings != nil {
		t.Errorf("expected nil for invalid JSON, got: %v", warnings)
	}
}

func TestCheckUnknownFields_MapTypedFieldsAcceptArbitraryKeys(t *testing.T) {
	t.Parallel()
	// Model names are user-defined map keys; they should not trigger warnings.
	raw := []byte(`{"models": {"my-custom-model": {"command": "claude", "args": []}}}`)
	warnings := checkUnknownFields(raw, "custom/config.json")

	if len(warnings) != 0 {
		t.Errorf("expected 0 warnings for custom model name, got %d: %v", len(warnings), warnings)
	}
}

func TestCheckUnknownFields_UnknownFieldInsideModelDef(t *testing.T) {
	t.Parallel()
	// "commnd" is a typo inside a ModelDef struct; should be caught.
	raw := []byte(`{"models": {"fast": {"commnd": "claude"}}}`)
	warnings := checkUnknownFields(raw, "base/config.json")

	if len(warnings) != 1 {
		t.Fatalf("expected 1 warning for typo in ModelDef, got %d: %v", len(warnings), warnings)
	}
	if !strings.Contains(warnings[0], "commnd") {
		t.Errorf("expected warning to mention 'commnd', got: %s", warnings[0])
	}
}

func TestCheckUnknownFields_TaskClassesAcceptArbitraryKeys(t *testing.T) {
	t.Parallel()
	raw := []byte(`{"task_classes": {"my-class": {"description": "test"}}}`)
	warnings := checkUnknownFields(raw, "custom/config.json")

	if len(warnings) != 0 {
		t.Errorf("expected 0 warnings for custom task class, got %d: %v", len(warnings), warnings)
	}
}

func TestDiffKeys_Empty(t *testing.T) {
	t.Parallel()
	var warnings []string
	diffKeys(map[string]any{}, map[string]any{"a": 1}, "", "tier", &warnings)
	if len(warnings) != 0 {
		t.Errorf("expected no warnings when have is empty, got: %v", warnings)
	}
}

func TestDiffKeys_NestedPathPrefix(t *testing.T) {
	t.Parallel()
	have := map[string]any{
		"parent": map[string]any{
			"child": map[string]any{
				"unknown_leaf": true,
			},
		},
	}
	known := map[string]any{
		"parent": map[string]any{
			"child": map[string]any{},
		},
	}
	var warnings []string
	diffKeys(have, known, "", "tier", &warnings)
	if len(warnings) != 1 {
		t.Fatalf("expected 1 warning, got %d: %v", len(warnings), warnings)
	}
	if !strings.Contains(warnings[0], "parent.child.unknown_leaf") {
		t.Errorf("expected full dot path, got: %s", warnings[0])
	}
}

// Integration test: Load with unknown fields produces warnings but succeeds.
func TestLoad_UnknownField_ProducesWarning(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()

	_ = os.MkdirAll(filepath.Join(dir, "system", "local"), 0755)
	configJSON := `{"failure": {"hard_cap": 100}, "bogus_field": true}`
	if err := os.WriteFile(filepath.Join(dir, "system", "local", "config.json"), []byte(configJSON), 0644); err != nil {
		t.Fatal(err)
	}

	cfg, err := Load(dir)
	if err != nil {
		t.Fatalf("Load() should succeed with unknown fields, got error: %v", err)
	}

	if len(cfg.Warnings) != 1 {
		t.Fatalf("expected 1 warning, got %d: %v", len(cfg.Warnings), cfg.Warnings)
	}
	if !strings.Contains(cfg.Warnings[0], "bogus_field") {
		t.Errorf("expected warning about 'bogus_field', got: %s", cfg.Warnings[0])
	}
	if !strings.Contains(cfg.Warnings[0], "local/config.json") {
		t.Errorf("expected warning to mention 'local/config.json', got: %s", cfg.Warnings[0])
	}
	// The valid field should still be applied
	if cfg.Failure.HardCap != 100 {
		t.Errorf("expected hard_cap=100 despite warning, got %d", cfg.Failure.HardCap)
	}
}

func TestLoad_UnknownFieldInSpecificTier(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()

	_ = os.MkdirAll(filepath.Join(dir, "system", "custom"), 0755)
	configJSON := `{"daemn": {"poll_interval_seconds": 10}}`
	if err := os.WriteFile(filepath.Join(dir, "system", "custom", "config.json"), []byte(configJSON), 0644); err != nil {
		t.Fatal(err)
	}

	cfg, err := Load(dir)
	if err != nil {
		t.Fatalf("Load() should succeed, got: %v", err)
	}

	if len(cfg.Warnings) != 1 {
		t.Fatalf("expected 1 warning, got %d: %v", len(cfg.Warnings), cfg.Warnings)
	}
	if !strings.Contains(cfg.Warnings[0], "custom/config.json") {
		t.Errorf("expected warning to identify custom tier, got: %s", cfg.Warnings[0])
	}
}

func TestLoad_CleanConfig_NoWarnings(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()

	_ = os.MkdirAll(filepath.Join(dir, "system", "base"), 0755)
	if err := os.WriteFile(filepath.Join(dir, "system", "base", "config.json"), []byte(`{"failure": {"hard_cap": 75}}`), 0644); err != nil {
		t.Fatal(err)
	}

	cfg, err := Load(dir)
	if err != nil {
		t.Fatal(err)
	}

	if len(cfg.Warnings) != 0 {
		t.Errorf("expected no warnings for clean config, got: %v", cfg.Warnings)
	}
}

func TestLoad_MultipleUnknownFieldsAcrossTiers(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()

	_ = os.MkdirAll(filepath.Join(dir, "system", "custom"), 0755)
	_ = os.MkdirAll(filepath.Join(dir, "system", "local"), 0755)

	_ = os.WriteFile(filepath.Join(dir, "system", "custom", "config.json"),
		[]byte(`{"typo_one": true}`), 0644)
	_ = os.WriteFile(filepath.Join(dir, "system", "local", "config.json"),
		[]byte(`{"typo_two": true}`), 0644)

	cfg, err := Load(dir)
	if err != nil {
		t.Fatal(err)
	}

	if len(cfg.Warnings) != 2 {
		t.Fatalf("expected 2 warnings, got %d: %v", len(cfg.Warnings), cfg.Warnings)
	}

	// Each warning should reference its own tier
	customFound, localFound := false, false
	for _, w := range cfg.Warnings {
		if strings.Contains(w, "custom/config.json") {
			customFound = true
		}
		if strings.Contains(w, "local/config.json") {
			localFound = true
		}
	}
	if !customFound || !localFound {
		t.Errorf("expected warnings from both tiers, got: %v", cfg.Warnings)
	}
}
