package config

import (
	"os"
	"path/filepath"
	"strings"
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

func TestValidate_CatchesEmptyPipelineStages(t *testing.T) {
	t.Parallel()
	cfg := Defaults()
	cfg.Identity = &IdentityConfig{User: "u", Machine: "m"}
	cfg.Pipeline.Stages = nil

	err := Validate(cfg)
	if err == nil {
		t.Error("expected error for empty pipeline stages")
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

func TestValidate_CatchesMissingSummaryModel(t *testing.T) {
	t.Parallel()
	cfg := Defaults()
	cfg.Identity = &IdentityConfig{User: "u", Machine: "m"}
	cfg.Summary.Enabled = true
	cfg.Summary.Model = "nonexistent"

	err := Validate(cfg)
	if err == nil {
		t.Error("expected error for missing summary model reference")
	}
}

func TestValidate_CatchesMissingDoctorModel(t *testing.T) {
	t.Parallel()
	cfg := Defaults()
	cfg.Identity = &IdentityConfig{User: "u", Machine: "m"}
	cfg.Doctor.Model = "nonexistent"

	err := Validate(cfg)
	if err == nil {
		t.Error("expected error for missing doctor model reference")
	}
}

func TestValidate_CatchesMissingUnblockModel(t *testing.T) {
	t.Parallel()
	cfg := Defaults()
	cfg.Identity = &IdentityConfig{User: "u", Machine: "m"}
	cfg.Unblock.Model = "nonexistent"

	err := Validate(cfg)
	if err == nil {
		t.Error("expected error for missing unblock model reference")
	}
}

func TestValidate_CatchesMissingOverlapAdvisoryModel(t *testing.T) {
	t.Parallel()
	cfg := Defaults()
	cfg.Identity = &IdentityConfig{User: "u", Machine: "m"}
	cfg.OverlapAdvisory.Enabled = true
	cfg.OverlapAdvisory.Model = "nonexistent"

	err := Validate(cfg)
	if err == nil {
		t.Error("expected error for missing overlap_advisory model reference")
	}
}

func TestValidate_CatchesMissingAuditModel(t *testing.T) {
	t.Parallel()
	cfg := Defaults()
	cfg.Identity = &IdentityConfig{User: "u", Machine: "m"}
	cfg.Audit.Model = "nonexistent"

	err := Validate(cfg)
	if err == nil {
		t.Error("expected error for missing audit model reference")
	}
}

func TestValidate_CatchesMissingIdentity(t *testing.T) {
	t.Parallel()
	cfg := Defaults()
	cfg.Identity = nil

	err := Validate(cfg)
	if err == nil {
		t.Error("expected error for missing identity")
	}
}

func TestValidate_SkipsDisabledSummaryModelCheck(t *testing.T) {
	t.Parallel()
	cfg := Defaults()
	cfg.Identity = &IdentityConfig{User: "u", Machine: "m"}
	cfg.Summary.Enabled = false
	cfg.Summary.Model = "nonexistent"

	err := Validate(cfg)
	if err != nil {
		t.Errorf("expected no error when summary is disabled, got: %v", err)
	}
}

func TestValidate_SkipsDisabledOverlapAdvisoryModelCheck(t *testing.T) {
	t.Parallel()
	cfg := Defaults()
	cfg.Identity = &IdentityConfig{User: "u", Machine: "m"}
	cfg.OverlapAdvisory.Enabled = false
	cfg.OverlapAdvisory.Model = "nonexistent"

	err := Validate(cfg)
	if err != nil {
		t.Errorf("expected no error when overlap_advisory is disabled, got: %v", err)
	}
}

func TestValidateStructure_CatchesNegativeDaemonTimings(t *testing.T) {
	t.Parallel()
	cfg := Defaults()
	cfg.Daemon.PollIntervalSeconds = 0

	err := ValidateStructure(cfg)
	if err == nil {
		t.Error("expected error for zero poll interval")
	}
}

func TestValidateStructure_CatchesNegativeFailureValues(t *testing.T) {
	t.Parallel()
	cfg := Defaults()
	cfg.Failure.DecompositionThreshold = -1

	err := ValidateStructure(cfg)
	if err == nil {
		t.Error("expected error for negative decomposition threshold")
	}
}

func TestValidateStructure_CatchesEmptyModelCommand(t *testing.T) {
	t.Parallel()
	cfg := Defaults()
	cfg.Models["broken"] = ModelDef{Command: ""}
	cfg.Pipeline.Stages = []PipelineStage{{Name: "test", Model: "broken", PromptFile: "test.md"}}

	err := ValidateStructure(cfg)
	if err == nil {
		t.Error("expected error for empty model command")
	}
}

func TestValidateStructure_CatchesEmptyStagePromptFile(t *testing.T) {
	t.Parallel()
	cfg := Defaults()
	cfg.Pipeline.Stages[0].PromptFile = ""

	err := ValidateStructure(cfg)
	if err == nil {
		t.Error("expected error for empty stage prompt file")
	}
}

func TestValidateStructure_ReportsMultipleErrors(t *testing.T) {
	t.Parallel()
	cfg := Defaults()
	cfg.Pipeline.Stages = nil
	cfg.Daemon.PollIntervalSeconds = 0
	cfg.Failure.HardCap = -1

	err := ValidateStructure(cfg)
	if err == nil {
		t.Fatal("expected error")
	}
	// Should contain multiple error lines
	errStr := err.Error()
	if count := len(strings.Split(errStr, "\n")); count < 3 {
		t.Errorf("expected multiple error lines, got %d: %s", count, errStr)
	}
}

func TestValidateStructure_CatchesNegativeLogRetention(t *testing.T) {
	t.Parallel()
	cfg := Defaults()
	cfg.Logs.MaxFiles = 0

	err := ValidateStructure(cfg)
	if err == nil {
		t.Error("expected error for zero max_files")
	}
}
