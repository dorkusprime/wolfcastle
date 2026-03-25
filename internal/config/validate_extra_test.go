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

func TestValidateStructure_CatchesZeroMaxWorkers(t *testing.T) {
	t.Parallel()
	cfg := Defaults()
	cfg.Daemon.Parallel.MaxWorkers = 0

	err := ValidateStructure(cfg)
	if err == nil {
		t.Error("expected error for zero max_workers")
	}
	if !strings.Contains(err.Error(), "daemon.parallel.max_workers must be >= 1") {
		t.Errorf("expected max_workers message, got: %v", err)
	}
}

func TestValidateStructure_CatchesNegativeMaxWorkers(t *testing.T) {
	t.Parallel()
	cfg := Defaults()
	cfg.Daemon.Parallel.MaxWorkers = -3

	err := ValidateStructure(cfg)
	if err == nil {
		t.Error("expected error for negative max_workers")
	}
}

func TestValidateStructure_AcceptsValidMaxWorkers(t *testing.T) {
	t.Parallel()
	cfg := Defaults()
	cfg.Daemon.Parallel.MaxWorkers = 5
	if err := ValidateStructure(cfg); err != nil {
		t.Errorf("max_workers=5 should be valid, got: %v", err)
	}
}

func TestValidateStructure_GitConfigFieldCombinations(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name            string
		autoCommit      bool
		commitOnSuccess bool
		commitOnFailure bool
		commitState     bool
	}{
		{name: "all enabled", autoCommit: true, commitOnSuccess: true, commitOnFailure: true, commitState: true},
		{name: "auto_commit false ignores sub-fields", autoCommit: false, commitOnSuccess: true, commitOnFailure: true, commitState: true},
		{name: "auto_commit false all sub-fields false", autoCommit: false, commitOnSuccess: false, commitOnFailure: false, commitState: false},
		{name: "auto_commit true success only", autoCommit: true, commitOnSuccess: true, commitOnFailure: false, commitState: false},
		{name: "auto_commit true failure only", autoCommit: true, commitOnSuccess: false, commitOnFailure: true, commitState: false},
		{name: "auto_commit true state only", autoCommit: true, commitOnSuccess: false, commitOnFailure: false, commitState: true},
		{name: "auto_commit true all sub-fields false", autoCommit: true, commitOnSuccess: false, commitOnFailure: false, commitState: false},
		{name: "all false", autoCommit: false, commitOnSuccess: false, commitOnFailure: false, commitState: false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			cfg := Defaults()
			cfg.Git.AutoCommit = tt.autoCommit
			cfg.Git.CommitOnSuccess = tt.commitOnSuccess
			cfg.Git.CommitOnFailure = tt.commitOnFailure
			cfg.Git.CommitState = tt.commitState

			if err := ValidateStructure(cfg); err != nil {
				t.Errorf("expected valid config, got: %v", err)
			}
		})
	}
}

func TestValidateStructure_GitCommitFormatUnchangedByNewFields(t *testing.T) {
	t.Parallel()
	cfg := Defaults()
	cfg.Git.AutoCommit = true
	cfg.Git.CommitOnSuccess = true
	cfg.Git.CommitOnFailure = true
	cfg.Git.CommitState = true
	cfg.Git.CommitMessageFormat = ""

	err := ValidateStructure(cfg)
	if err == nil {
		t.Error("expected error for empty commit message format even with new fields set")
	}
	if !strings.Contains(err.Error(), "commit_message_format") {
		t.Errorf("expected mention of commit_message_format, got: %v", err)
	}
}
