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
	_ = os.MkdirAll(filepath.Join(dir, "system", "base"), 0755)
	if err := os.WriteFile(filepath.Join(dir, "system", "base", "config.json"), []byte("{}"), 0644); err != nil {
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

	_ = os.MkdirAll(filepath.Join(dir, "system", "custom"), 0755)
	configJSON := `{"failure": {"hard_cap": 100}}`
	if err := os.WriteFile(filepath.Join(dir, "system", "custom", "config.json"), []byte(configJSON), 0644); err != nil {
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

	_ = os.MkdirAll(filepath.Join(dir, "system", "custom"), 0755)
	_ = os.MkdirAll(filepath.Join(dir, "system", "local"), 0755)
	configJSON := `{"failure": {"hard_cap": 100}}`
	localJSON := `{"failure": {"hard_cap": 200}}`
	if err := os.WriteFile(filepath.Join(dir, "system", "custom", "config.json"), []byte(configJSON), 0644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "system", "local", "config.json"), []byte(localJSON), 0644); err != nil {
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

func TestLoad_ThreeTierMerge(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()

	_ = os.MkdirAll(filepath.Join(dir, "system", "base"), 0755)
	_ = os.MkdirAll(filepath.Join(dir, "system", "custom"), 0755)
	_ = os.MkdirAll(filepath.Join(dir, "system", "local"), 0755)

	// base sets hard_cap to 50
	_ = os.WriteFile(filepath.Join(dir, "system", "base", "config.json"), []byte(`{"failure": {"hard_cap": 50}}`), 0644)
	// custom overrides to 100
	_ = os.WriteFile(filepath.Join(dir, "system", "custom", "config.json"), []byte(`{"failure": {"hard_cap": 100, "decomposition_threshold": 5}}`), 0644)
	// local overrides hard_cap to 200, but decomposition_threshold stays from custom
	_ = os.WriteFile(filepath.Join(dir, "system", "local", "config.json"), []byte(`{"failure": {"hard_cap": 200}}`), 0644)

	cfg, err := Load(dir)
	if err != nil {
		t.Fatal(err)
	}

	if cfg.Failure.HardCap != 200 {
		t.Errorf("expected local to win: hard_cap=200, got %d", cfg.Failure.HardCap)
	}
	if cfg.Failure.DecompositionThreshold != 5 {
		t.Errorf("expected custom to win for decomposition_threshold=5, got %d", cfg.Failure.DecompositionThreshold)
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
	intake := cfg.Pipeline.Stages["intake"]
	intake.Model = "nonexistent"
	cfg.Pipeline.Stages["intake"] = intake

	err := Validate(cfg)
	if err == nil {
		t.Error("expected error for missing model reference")
	}
}

func TestValidate_CatchesStageOrderDuplicates(t *testing.T) {
	t.Parallel()
	cfg := Defaults()
	cfg.Identity = &IdentityConfig{User: "u", Machine: "m"}
	cfg.Pipeline.StageOrder = []string{"intake", "execute", "intake"}

	err := Validate(cfg)
	if err == nil {
		t.Error("expected error for duplicate entry in stage_order")
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

func TestValidate_SkipsOverlapAdvisoryModelValidation(t *testing.T) {
	t.Parallel()
	cfg := Defaults()
	cfg.Identity = &IdentityConfig{User: "u", Machine: "m"}
	cfg.OverlapAdvisory.Enabled = true
	cfg.OverlapAdvisory.Model = "nonexistent"

	err := Validate(cfg)
	if err != nil {
		t.Errorf("overlap_advisory model should not be validated (ADR-041), got: %v", err)
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

func TestValidateStructure_CatchesInvalidFailureValues(t *testing.T) {
	t.Parallel()
	cfg := Defaults()
	cfg.Failure.DecompositionThreshold = 0

	err := ValidateStructure(cfg)
	if err == nil {
		t.Error("expected error for zero decomposition threshold (minimum is 1)")
	}
}

func TestValidateStructure_CatchesEmptyModelCommand(t *testing.T) {
	t.Parallel()
	cfg := Defaults()
	cfg.Models["broken"] = ModelDef{Command: ""}
	cfg.Pipeline.Stages = map[string]PipelineStage{"test": {Model: "broken", PromptFile: "test.md"}}
	cfg.Pipeline.StageOrder = []string{"test"}

	err := ValidateStructure(cfg)
	if err == nil {
		t.Error("expected error for empty model command")
	}
}

func TestValidateStructure_CatchesEmptyStagePromptFile(t *testing.T) {
	t.Parallel()
	cfg := Defaults()
	intake := cfg.Pipeline.Stages["intake"]
	intake.PromptFile = ""
	cfg.Pipeline.Stages["intake"] = intake

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

func TestValidateStructure_CatchesNegativeOverlapThreshold(t *testing.T) {
	t.Parallel()
	cfg := Defaults()
	cfg.OverlapAdvisory.Threshold = -0.5

	err := ValidateStructure(cfg)
	if err == nil {
		t.Error("expected error for negative overlap threshold")
	}
	if !strings.Contains(err.Error(), "overlap_advisory.threshold") {
		t.Errorf("expected mention of overlap_advisory.threshold in error, got: %v", err)
	}
}

func TestValidateStructure_CatchesOverlapThresholdAboveOne(t *testing.T) {
	t.Parallel()
	cfg := Defaults()
	cfg.OverlapAdvisory.Threshold = 1.5

	err := ValidateStructure(cfg)
	if err == nil {
		t.Error("expected error for overlap threshold > 1")
	}
	if !strings.Contains(err.Error(), "overlap_advisory.threshold") {
		t.Errorf("expected mention of overlap_advisory.threshold in error, got: %v", err)
	}
}

func TestValidateStructure_AcceptsOverlapThresholdAtBoundaries(t *testing.T) {
	t.Parallel()

	// Threshold exactly 0 should be valid
	cfg := Defaults()
	cfg.OverlapAdvisory.Threshold = 0
	if err := ValidateStructure(cfg); err != nil {
		t.Errorf("threshold=0 should be valid, got: %v", err)
	}

	// Threshold exactly 1 should be valid
	cfg2 := Defaults()
	cfg2.OverlapAdvisory.Threshold = 1
	if err := ValidateStructure(cfg2); err != nil {
		t.Errorf("threshold=1 should be valid, got: %v", err)
	}
}

func TestPipelineStage_IsEnabled_DefaultTrue(t *testing.T) {
	t.Parallel()
	s := PipelineStage{Model: "fast", PromptFile: "test.md"}
	if !s.IsEnabled() {
		t.Error("expected IsEnabled() to return true by default")
	}
}

func TestPipelineStage_IsEnabled_ExplicitTrue(t *testing.T) {
	t.Parallel()
	enabled := true
	s := PipelineStage{Model: "fast", PromptFile: "test.md", Enabled: &enabled}
	if !s.IsEnabled() {
		t.Error("expected IsEnabled() to return true when explicitly set true")
	}
}

func TestPipelineStage_IsEnabled_ExplicitFalse(t *testing.T) {
	t.Parallel()
	enabled := false
	s := PipelineStage{Model: "fast", PromptFile: "test.md", Enabled: &enabled}
	if s.IsEnabled() {
		t.Error("expected IsEnabled() to return false when explicitly set false")
	}
}

func TestPipelineStage_ShouldSkipPromptAssembly_DefaultFalse(t *testing.T) {
	t.Parallel()
	s := PipelineStage{Model: "fast", PromptFile: "test.md"}
	if s.ShouldSkipPromptAssembly() {
		t.Error("expected ShouldSkipPromptAssembly() to return false by default")
	}
}

func TestPipelineStage_ShouldSkipPromptAssembly_ExplicitTrue(t *testing.T) {
	t.Parallel()
	skip := true
	s := PipelineStage{Model: "fast", PromptFile: "test.md", SkipPromptAssembly: &skip}
	if !s.ShouldSkipPromptAssembly() {
		t.Error("expected ShouldSkipPromptAssembly() to return true when explicitly set true")
	}
}

func TestPipelineStage_ShouldSkipPromptAssembly_ExplicitFalse(t *testing.T) {
	t.Parallel()
	skip := false
	s := PipelineStage{Model: "fast", PromptFile: "test.md", SkipPromptAssembly: &skip}
	if s.ShouldSkipPromptAssembly() {
		t.Error("expected ShouldSkipPromptAssembly() to return false when explicitly set false")
	}
}

func TestLoad_LocalOnly_NoBaseOrCustom(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()

	_ = os.MkdirAll(filepath.Join(dir, "system", "local"), 0755)
	localJSON := `{"failure": {"hard_cap": 999}}`
	if err := os.WriteFile(filepath.Join(dir, "system", "local", "config.json"), []byte(localJSON), 0644); err != nil {
		t.Fatal(err)
	}

	cfg, err := Load(dir)
	if err != nil {
		t.Fatal(err)
	}

	if cfg.Failure.HardCap != 999 {
		t.Errorf("expected hard_cap=999 from local override, got %d", cfg.Failure.HardCap)
	}
	// Other defaults should be preserved
	if cfg.Failure.DecompositionThreshold != 10 {
		t.Errorf("expected default decomposition_threshold=10, got %d", cfg.Failure.DecompositionThreshold)
	}
}

func TestLoad_InvalidJSON_ReturnsError(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()

	_ = os.MkdirAll(filepath.Join(dir, "system", "base"), 0755)
	if err := os.WriteFile(filepath.Join(dir, "system", "base", "config.json"), []byte("{invalid json}"), 0644); err != nil {
		t.Fatal(err)
	}

	_, err := Load(dir)
	if err == nil {
		t.Error("expected error for invalid JSON in base/config.json")
	}
}

func TestLoad_InvalidLocalJSON_ReturnsError(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()

	_ = os.MkdirAll(filepath.Join(dir, "system", "local"), 0755)
	if err := os.WriteFile(filepath.Join(dir, "system", "local", "config.json"), []byte("{not valid}"), 0644); err != nil {
		t.Fatal(err)
	}

	_, err := Load(dir)
	if err == nil {
		t.Error("expected error for invalid JSON in local/config.json")
	}
}

func TestLoad_ValidationFailure_ReturnsError(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()

	// Set pipeline stages to empty map, which should fail structural validation
	_ = os.MkdirAll(filepath.Join(dir, "system", "custom"), 0755)
	configJSON := `{"pipeline": {"stages": {}, "stage_order": []}}`
	if err := os.WriteFile(filepath.Join(dir, "system", "custom", "config.json"), []byte(configJSON), 0644); err != nil {
		t.Fatal(err)
	}

	_, err := Load(dir)
	if err == nil {
		t.Error("expected error for invalid config structure")
	}
	if !strings.Contains(err.Error(), "config validation failed") {
		t.Errorf("expected validation error message, got: %v", err)
	}
}

func TestValidateStructure_CatchesZeroMaxAgeDays(t *testing.T) {
	t.Parallel()
	cfg := Defaults()
	cfg.Logs.MaxAgeDays = 0

	err := ValidateStructure(cfg)
	if err == nil {
		t.Error("expected error for zero max_age_days")
	}
	if !strings.Contains(err.Error(), "logs.max_age_days") {
		t.Errorf("expected mention of logs.max_age_days, got: %v", err)
	}
}

func TestValidateStructure_CatchesStageOrderReferencingUnknownStage(t *testing.T) {
	t.Parallel()
	cfg := Defaults()
	cfg.Pipeline.StageOrder = []string{"intake", "execute", "ghost"}

	err := ValidateStructure(cfg)
	if err == nil {
		t.Error("expected error for stage_order referencing unknown stage")
	}
	if !strings.Contains(err.Error(), "unknown stage") {
		t.Errorf("expected 'unknown stage' in error, got: %v", err)
	}
}

func TestValidateStructure_CatchesStageMissingFromStageOrder(t *testing.T) {
	t.Parallel()
	cfg := Defaults()
	cfg.Pipeline.StageOrder = []string{"intake"} // missing "execute"

	err := ValidateStructure(cfg)
	if err == nil {
		t.Error("expected error for stage missing from stage_order")
	}
	if !strings.Contains(err.Error(), "missing from stage_order") {
		t.Errorf("expected 'missing from stage_order' in error, got: %v", err)
	}
}

func TestValidateStructure_CatchesNegativeMaxDecompositionDepth(t *testing.T) {
	t.Parallel()
	cfg := Defaults()
	cfg.Failure.MaxDecompositionDepth = -1

	err := ValidateStructure(cfg)
	if err == nil {
		t.Error("expected error for negative max_decomposition_depth")
	}
}

func TestValidateStructure_CatchesInvalidHardCap(t *testing.T) {
	t.Parallel()
	cfg := Defaults()
	cfg.Failure.HardCap = 0

	err := ValidateStructure(cfg)
	if err == nil {
		t.Error("expected error for zero hard_cap (minimum is 1)")
	}
}

func TestValidateStructure_CatchesNegativeBlockedPollInterval(t *testing.T) {
	t.Parallel()
	cfg := Defaults()
	cfg.Daemon.BlockedPollIntervalSeconds = 0

	err := ValidateStructure(cfg)
	if err == nil {
		t.Error("expected error for zero blocked_poll_interval_seconds")
	}
}

func TestValidateStructure_CatchesInvocationTimeoutBelowMinimum(t *testing.T) {
	t.Parallel()
	cfg := Defaults()
	cfg.Daemon.InvocationTimeoutSeconds = 30

	err := ValidateStructure(cfg)
	if err == nil {
		t.Error("expected error for invocation_timeout_seconds below 60")
	}
}

func TestValidateStructure_CatchesNegativeMaxTurns(t *testing.T) {
	t.Parallel()
	cfg := Defaults()
	cfg.Daemon.MaxTurnsPerInvocation = 0

	err := ValidateStructure(cfg)
	if err == nil {
		t.Error("expected error for zero max_turns_per_invocation")
	}
}

func TestValidateStructure_CatchesNegativeMaxRestarts(t *testing.T) {
	t.Parallel()
	cfg := Defaults()
	cfg.Daemon.MaxRestarts = -1

	err := ValidateStructure(cfg)
	if err == nil {
		t.Error("expected error for negative max_restarts")
	}
}

func TestValidateStructure_CatchesNegativeRestartDelay(t *testing.T) {
	t.Parallel()
	cfg := Defaults()
	cfg.Daemon.RestartDelaySeconds = -1

	err := ValidateStructure(cfg)
	if err == nil {
		t.Error("expected error for negative restart_delay_seconds")
	}
}

func TestValidateStructure_PassesOnDefaults(t *testing.T) {
	t.Parallel()
	cfg := Defaults()

	if err := ValidateStructure(cfg); err != nil {
		t.Errorf("expected no error on valid defaults, got: %v", err)
	}
}

func TestValidateStructure_CatchesUnknownStageModel(t *testing.T) {
	t.Parallel()
	cfg := Defaults()
	intake := cfg.Pipeline.Stages["intake"]
	intake.Model = "nonexistent"
	cfg.Pipeline.Stages["intake"] = intake

	err := ValidateStructure(cfg)
	if err == nil {
		t.Error("expected error for unknown model reference in stage")
	}
	if !strings.Contains(err.Error(), "unknown model") {
		t.Errorf("expected 'unknown model' in error, got: %v", err)
	}
}
