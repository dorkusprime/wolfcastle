package config

import (
	"os"
	"path/filepath"
	"testing"
)

func TestDefaults_ReturnsValidConfig(t *testing.T) {
	t.Parallel()
	cfg := Defaults()

	if len(cfg.Models) == 0 {
		t.Error("models should not be empty")
	}
	if _, ok := cfg.Models["fast"]; !ok {
		t.Error("missing 'fast' model")
	}
	if _, ok := cfg.Models["mid"]; !ok {
		t.Error("missing 'mid' model")
	}
	if _, ok := cfg.Models["heavy"]; !ok {
		t.Error("missing 'heavy' model")
	}
	if len(cfg.Pipeline.Stages) == 0 {
		t.Error("pipeline stages should not be empty")
	}
	if cfg.Logs.MaxFiles == 0 {
		t.Error("logs.max_files should be populated")
	}
	if cfg.Failure.DecompositionThreshold == 0 {
		t.Error("failure.decomposition_threshold should be populated")
	}
	if cfg.Failure.HardCap == 0 {
		t.Error("failure.hard_cap should be populated")
	}
	if cfg.Daemon.PollIntervalSeconds == 0 {
		t.Error("daemon.poll_interval_seconds should be populated")
	}
	if cfg.Summary.Model == "" {
		t.Error("summary.model should be populated")
	}
	if cfg.Doctor.Model == "" {
		t.Error("doctor.model should be populated")
	}
	if cfg.Unblock.Model == "" {
		t.Error("unblock.model should be populated")
	}
	if cfg.Audit.Model == "" {
		t.Error("audit.model should be populated")
	}
}

func TestLoad_EmptyConfigJSON_ReturnsDefaults(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "config.json"), []byte("{}"), 0644); err != nil {
		t.Fatal(err)
	}

	cfg, err := Load(dir)
	if err != nil {
		t.Fatal(err)
	}

	defaults := Defaults()
	if len(cfg.Models) != len(defaults.Models) {
		t.Errorf("expected %d models, got %d", len(defaults.Models), len(cfg.Models))
	}
	if cfg.Failure.HardCap != defaults.Failure.HardCap {
		t.Errorf("expected hard_cap=%d, got %d", defaults.Failure.HardCap, cfg.Failure.HardCap)
	}
}

func TestLoad_WithOverrides_MergesCorrectly(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()

	configJSON := `{"failure": {"hard_cap": 100}}`
	if err := os.WriteFile(filepath.Join(dir, "config.json"), []byte(configJSON), 0644); err != nil {
		t.Fatal(err)
	}

	cfg, err := Load(dir)
	if err != nil {
		t.Fatal(err)
	}

	if cfg.Failure.HardCap != 100 {
		t.Errorf("expected hard_cap=100, got %d", cfg.Failure.HardCap)
	}
	// Other fields should still be defaults
	if cfg.Failure.DecompositionThreshold != 10 {
		t.Errorf("decomposition_threshold should remain 10, got %d", cfg.Failure.DecompositionThreshold)
	}
}

func TestLoad_LocalOverridesConfig(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()

	configJSON := `{"failure": {"hard_cap": 100}}`
	localJSON := `{"failure": {"hard_cap": 200}}`
	if err := os.WriteFile(filepath.Join(dir, "config.json"), []byte(configJSON), 0644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "config.local.json"), []byte(localJSON), 0644); err != nil {
		t.Fatal(err)
	}

	cfg, err := Load(dir)
	if err != nil {
		t.Fatal(err)
	}

	if cfg.Failure.HardCap != 200 {
		t.Errorf("local should override: expected hard_cap=200, got %d", cfg.Failure.HardCap)
	}
}

func TestLoad_NoConfigFiles_ReturnsDefaults(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()

	cfg, err := Load(dir)
	if err != nil {
		t.Fatal(err)
	}

	defaults := Defaults()
	if cfg.Failure.HardCap != defaults.Failure.HardCap {
		t.Errorf("expected default hard_cap=%d, got %d", defaults.Failure.HardCap, cfg.Failure.HardCap)
	}
}

func TestValidate_CatchesMissingModelReferences(t *testing.T) {
	t.Parallel()
	cfg := Defaults()
	cfg.Identity = &IdentityConfig{User: "u", Machine: "m"}
	cfg.Pipeline.Stages[0].Model = "nonexistent"

	err := Validate(cfg)
	if err == nil {
		t.Error("expected error for missing model reference")
	}
}

func TestValidate_CatchesDuplicateStageNames(t *testing.T) {
	t.Parallel()
	cfg := Defaults()
	cfg.Identity = &IdentityConfig{User: "u", Machine: "m"}
	cfg.Pipeline.Stages = append(cfg.Pipeline.Stages, PipelineStage{
		Name: "expand", Model: "fast", PromptFile: "dup.md",
	})

	err := Validate(cfg)
	if err == nil {
		t.Error("expected error for duplicate stage names")
	}
}

func TestValidate_CatchesHardCapBelowDecompositionThreshold(t *testing.T) {
	t.Parallel()
	cfg := Defaults()
	cfg.Identity = &IdentityConfig{User: "u", Machine: "m"}
	cfg.Failure.HardCap = 5
	cfg.Failure.DecompositionThreshold = 10

	err := Validate(cfg)
	if err == nil {
		t.Error("expected error for hard_cap < decomposition_threshold")
	}
}

func TestValidate_PassesOnDefaults(t *testing.T) {
	t.Parallel()
	cfg := Defaults()
	cfg.Identity = &IdentityConfig{User: "u", Machine: "m"}

	if err := Validate(cfg); err != nil {
		t.Errorf("expected no error on valid config, got: %v", err)
	}
}
