package config

import (
	"strings"
	"testing"
)

func TestValidateStructure_CatchesInvalidLogLevel(t *testing.T) {
	t.Parallel()
	cfg := Defaults()
	cfg.Daemon.LogLevel = "verbose"

	err := ValidateStructure(cfg)
	if err == nil {
		t.Error("expected error for invalid log level")
	}
	if !strings.Contains(err.Error(), "log_level") {
		t.Errorf("expected mention of log_level, got: %v", err)
	}
}

func TestValidateStructure_AcceptsAllValidLogLevels(t *testing.T) {
	t.Parallel()
	for _, level := range []string{"debug", "info", "warn", "error"} {
		cfg := Defaults()
		cfg.Daemon.LogLevel = level
		if err := ValidateStructure(cfg); err != nil {
			t.Errorf("log level %q should be valid, got: %v", level, err)
		}
	}
}

func TestValidateStructure_AcceptsEmptyLogLevel(t *testing.T) {
	t.Parallel()
	cfg := Defaults()
	cfg.Daemon.LogLevel = ""
	if err := ValidateStructure(cfg); err != nil {
		t.Errorf("empty log level should be valid, got: %v", err)
	}
}

func TestValidateStructure_CatchesNegativeRetryInitialDelay(t *testing.T) {
	t.Parallel()
	cfg := Defaults()
	cfg.Retries.InitialDelaySeconds = 0

	err := ValidateStructure(cfg)
	if err == nil {
		t.Error("expected error for zero initial delay")
	}
}

func TestValidateStructure_CatchesNegativeRetryMaxDelay(t *testing.T) {
	t.Parallel()
	cfg := Defaults()
	cfg.Retries.MaxDelaySeconds = 0

	err := ValidateStructure(cfg)
	if err == nil {
		t.Error("expected error for zero max delay")
	}
}

func TestValidateStructure_CatchesTooLowMaxRetries(t *testing.T) {
	t.Parallel()
	cfg := Defaults()
	cfg.Retries.MaxRetries = -2

	err := ValidateStructure(cfg)
	if err == nil {
		t.Error("expected error for max_retries < -1")
	}
}

func TestValidateStructure_AcceptsUnlimitedRetries(t *testing.T) {
	t.Parallel()
	cfg := Defaults()
	cfg.Retries.MaxRetries = -1
	if err := ValidateStructure(cfg); err != nil {
		t.Errorf("max_retries=-1 (unlimited) should be valid, got: %v", err)
	}
}

func TestValidateStructure_CatchesEmptyGitCommitFormat(t *testing.T) {
	t.Parallel()
	cfg := Defaults()
	cfg.Git.CommitMessageFormat = ""

	err := ValidateStructure(cfg)
	if err == nil {
		t.Error("expected error for empty commit message format")
	}
}

func TestValidateStructure_CatchesValidationCommandErrors(t *testing.T) {
	t.Parallel()
	cfg := Defaults()
	cfg.Validation.Commands = []ValidationCommand{
		{Name: "", Run: "echo test", TimeoutSeconds: 10},
	}

	err := ValidateStructure(cfg)
	if err == nil {
		t.Error("expected error for empty validation command name")
	}
}

func TestValidateStructure_CatchesDuplicateValidationCommandNames(t *testing.T) {
	t.Parallel()
	cfg := Defaults()
	cfg.Validation.Commands = []ValidationCommand{
		{Name: "lint", Run: "echo lint", TimeoutSeconds: 10},
		{Name: "lint", Run: "echo lint2", TimeoutSeconds: 10},
	}

	err := ValidateStructure(cfg)
	if err == nil {
		t.Error("expected error for duplicate validation command names")
	}
}

func TestValidateStructure_CatchesEmptyValidationCommandRun(t *testing.T) {
	t.Parallel()
	cfg := Defaults()
	cfg.Validation.Commands = []ValidationCommand{
		{Name: "lint", Run: "", TimeoutSeconds: 10},
	}

	err := ValidateStructure(cfg)
	if err == nil {
		t.Error("expected error for empty validation command run")
	}
}

func TestValidateStructure_CatchesZeroValidationTimeout(t *testing.T) {
	t.Parallel()
	cfg := Defaults()
	cfg.Validation.Commands = []ValidationCommand{
		{Name: "lint", Run: "echo test", TimeoutSeconds: 0},
	}

	err := ValidateStructure(cfg)
	if err == nil {
		t.Error("expected error for zero validation command timeout")
	}
}

func TestValidate_CatchesMissingDoctorPromptFile(t *testing.T) {
	t.Parallel()
	cfg := Defaults()
	cfg.Identity = &IdentityConfig{User: "u", Machine: "m"}
	cfg.Doctor.PromptFile = ""

	err := Validate(cfg)
	if err == nil {
		t.Error("expected error for empty doctor prompt file")
	}
}

func TestValidate_CatchesMissingUnblockPromptFile(t *testing.T) {
	t.Parallel()
	cfg := Defaults()
	cfg.Identity = &IdentityConfig{User: "u", Machine: "m"}
	cfg.Unblock.PromptFile = ""

	err := Validate(cfg)
	if err == nil {
		t.Error("expected error for empty unblock prompt file")
	}
}

func TestValidate_CatchesMissingSummaryPromptFile(t *testing.T) {
	t.Parallel()
	cfg := Defaults()
	cfg.Identity = &IdentityConfig{User: "u", Machine: "m"}
	cfg.Summary.Enabled = true
	cfg.Summary.PromptFile = ""

	err := Validate(cfg)
	if err == nil {
		t.Error("expected error for empty summary prompt file when enabled")
	}
}

func TestValidateStructure_CatchesZeroStallTimeout(t *testing.T) {
	t.Parallel()
	cfg := Defaults()
	cfg.Daemon.StallTimeoutSeconds = 0

	err := ValidateStructure(cfg)
	if err == nil {
		t.Error("expected error for zero stall timeout")
	}
	if !strings.Contains(err.Error(), "stall_timeout_seconds") {
		t.Errorf("expected mention of stall_timeout_seconds, got: %v", err)
	}
}

func TestValidateStructure_CatchesNegativeStallTimeout(t *testing.T) {
	t.Parallel()
	cfg := Defaults()
	cfg.Daemon.StallTimeoutSeconds = -5

	err := ValidateStructure(cfg)
	if err == nil {
		t.Error("expected error for negative stall timeout")
	}
}

func TestValidateStructure_AcceptsPositiveStallTimeout(t *testing.T) {
	t.Parallel()
	cfg := Defaults()
	cfg.Daemon.StallTimeoutSeconds = 60
	if err := ValidateStructure(cfg); err != nil {
		t.Errorf("positive stall timeout should be valid, got: %v", err)
	}
}

func TestValidate_CatchesMissingAuditPromptFile(t *testing.T) {
	t.Parallel()
	cfg := Defaults()
	cfg.Identity = &IdentityConfig{User: "u", Machine: "m"}
	cfg.Audit.PromptFile = ""

	err := Validate(cfg)
	if err == nil {
		t.Error("expected error for empty audit prompt file")
	}
}
