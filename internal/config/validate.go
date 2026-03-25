package config

import (
	"fmt"
	"strings"
)

// ValidateStructure checks config consistency without requiring identity.
// Called automatically during Load(). For full validation including identity,
// use Validate().
func ValidateStructure(cfg *Config) error {
	var errs []string

	// Version must not exceed what this binary understands
	if cfg.Version > CurrentVersion {
		errs = append(errs, fmt.Sprintf("config version %d is newer than this binary supports (%d); upgrade wolfcastle", cfg.Version, CurrentVersion))
	}

	// Pipeline must have at least one stage
	if len(cfg.Pipeline.Stages) == 0 {
		errs = append(errs, "pipeline has no stages: at least one stage is required")
	}

	// Stage order integrity: no duplicates, every entry must exist in Stages
	orderSeen := make(map[string]bool, len(cfg.Pipeline.StageOrder))
	for _, name := range cfg.Pipeline.StageOrder {
		if orderSeen[name] {
			errs = append(errs, fmt.Sprintf("duplicate entry %q in stage_order", name))
		}
		orderSeen[name] = true
		if _, ok := cfg.Pipeline.Stages[name]; !ok {
			errs = append(errs, fmt.Sprintf("stage_order references unknown stage %q", name))
		}
	}
	// Every key in Stages must appear in StageOrder
	for name := range cfg.Pipeline.Stages {
		if !orderSeen[name] {
			errs = append(errs, fmt.Sprintf("stage %q exists in stages but is missing from stage_order", name))
		}
	}

	// All stage model references must exist
	for name, stage := range cfg.Pipeline.Stages {
		if _, ok := cfg.Models[stage.Model]; !ok {
			errs = append(errs, fmt.Sprintf("pipeline stage %q references unknown model %q", name, stage.Model))
		}
	}

	// Stage prompt files must be non-empty
	for name, stage := range cfg.Pipeline.Stages {
		if stage.PromptFile == "" {
			errs = append(errs, fmt.Sprintf("pipeline stage %q has empty prompt_file", name))
		}
	}

	// Failure thresholds (spec: decomposition_threshold min 1, hard_cap min 1)
	if cfg.Failure.DecompositionThreshold < 1 {
		errs = append(errs, fmt.Sprintf("failure.decomposition_threshold (%d) must be >= 1", cfg.Failure.DecompositionThreshold))
	}
	if cfg.Failure.MaxDecompositionDepth < 0 {
		errs = append(errs, fmt.Sprintf("failure.max_decomposition_depth (%d) must be >= 0", cfg.Failure.MaxDecompositionDepth))
	}
	if cfg.Failure.HardCap < 1 {
		errs = append(errs, fmt.Sprintf("failure.hard_cap (%d) must be >= 1", cfg.Failure.HardCap))
	}
	if cfg.Failure.HardCap > 0 && cfg.Failure.HardCap < cfg.Failure.DecompositionThreshold {
		errs = append(errs, fmt.Sprintf("failure.hard_cap (%d) must be >= failure.decomposition_threshold (%d)",
			cfg.Failure.HardCap, cfg.Failure.DecompositionThreshold))
	}

	// Daemon timing values must be positive
	if cfg.Daemon.PollIntervalSeconds <= 0 {
		errs = append(errs, fmt.Sprintf("daemon.poll_interval_seconds (%d) must be > 0", cfg.Daemon.PollIntervalSeconds))
	}
	if cfg.Daemon.BlockedPollIntervalSeconds <= 0 {
		errs = append(errs, fmt.Sprintf("daemon.blocked_poll_interval_seconds (%d) must be > 0", cfg.Daemon.BlockedPollIntervalSeconds))
	}
	if cfg.Daemon.InvocationTimeoutSeconds < 60 {
		errs = append(errs, fmt.Sprintf("daemon.invocation_timeout_seconds (%d) must be >= 60", cfg.Daemon.InvocationTimeoutSeconds))
	}
	if cfg.Daemon.StallTimeoutSeconds <= 0 {
		errs = append(errs, fmt.Sprintf("daemon.stall_timeout_seconds (%d) must be > 0", cfg.Daemon.StallTimeoutSeconds))
	}
	if cfg.Daemon.MaxTurnsPerInvocation <= 0 {
		errs = append(errs, fmt.Sprintf("daemon.max_turns_per_invocation (%d) must be > 0", cfg.Daemon.MaxTurnsPerInvocation))
	}
	if cfg.Daemon.MaxRestarts < 0 {
		errs = append(errs, fmt.Sprintf("daemon.max_restarts (%d) must be >= 0", cfg.Daemon.MaxRestarts))
	}
	if cfg.Daemon.RestartDelaySeconds < 0 {
		errs = append(errs, fmt.Sprintf("daemon.restart_delay_seconds (%d) must be >= 0", cfg.Daemon.RestartDelaySeconds))
	}
	validLogLevels := map[string]bool{"debug": true, "info": true, "warn": true, "error": true}
	if cfg.Daemon.LogLevel != "" && !validLogLevels[cfg.Daemon.LogLevel] {
		errs = append(errs, fmt.Sprintf("daemon.log_level %q must be one of: debug, info, warn, error", cfg.Daemon.LogLevel))
	}
	if cfg.Daemon.Parallel.MaxWorkers < 1 {
		errs = append(errs, "daemon.parallel.max_workers must be >= 1")
	}

	// Log retention
	if cfg.Logs.MaxFiles <= 0 {
		errs = append(errs, fmt.Sprintf("logs.max_files (%d) must be > 0", cfg.Logs.MaxFiles))
	}
	if cfg.Logs.MaxAgeDays <= 0 {
		errs = append(errs, fmt.Sprintf("logs.max_age_days (%d) must be > 0", cfg.Logs.MaxAgeDays))
	}

	// Retries constraints
	if cfg.Retries.InitialDelaySeconds < 1 {
		errs = append(errs, fmt.Sprintf("retries.initial_delay_seconds (%d) must be >= 1", cfg.Retries.InitialDelaySeconds))
	}
	if cfg.Retries.MaxDelaySeconds < 1 {
		errs = append(errs, fmt.Sprintf("retries.max_delay_seconds (%d) must be >= 1", cfg.Retries.MaxDelaySeconds))
	}
	if cfg.Retries.MaxRetries < -1 {
		errs = append(errs, fmt.Sprintf("retries.max_retries (%d) must be >= -1", cfg.Retries.MaxRetries))
	}

	// Validation command names and timeouts
	cmdNames := make(map[string]bool)
	for i, cmd := range cfg.Validation.Commands {
		if cmd.Name == "" {
			errs = append(errs, fmt.Sprintf("validation.commands[%d] has empty name", i))
		} else if cmdNames[cmd.Name] {
			errs = append(errs, fmt.Sprintf("validation.commands[%d] has duplicate name %q", i, cmd.Name))
		}
		cmdNames[cmd.Name] = true
		if cmd.Run == "" {
			errs = append(errs, fmt.Sprintf("validation.commands[%d].run must not be empty", i))
		}
		if cmd.TimeoutSeconds < 1 {
			errs = append(errs, fmt.Sprintf("validation.commands[%d].timeout_seconds (%d) must be >= 1", i, cmd.TimeoutSeconds))
		}
	}

	// Git commit message format must contain the {action} placeholder
	if cfg.Git.CommitMessageFormat == "" {
		errs = append(errs, "git.commit_message_format must not be empty")
	}

	// Overlap advisory threshold must be in [0, 1]
	if cfg.OverlapAdvisory.Threshold < 0 || cfg.OverlapAdvisory.Threshold > 1 {
		errs = append(errs, fmt.Sprintf("overlap_advisory.threshold (%.2f) must be between 0 and 1", cfg.OverlapAdvisory.Threshold))
	}

	// Knowledge token budget
	if cfg.Knowledge.MaxTokens < 1 {
		errs = append(errs, fmt.Sprintf("knowledge.max_tokens (%d) must be >= 1", cfg.Knowledge.MaxTokens))
	}

	// Model definitions must have a command
	for name, model := range cfg.Models {
		if model.Command == "" {
			errs = append(errs, fmt.Sprintf("model %q has empty command", name))
		}
	}

	if len(errs) > 0 {
		return fmt.Errorf("config validation failed:\n  - %s", strings.Join(errs, "\n  - "))
	}
	return nil
}

// Validate checks the resolved config for consistency including identity.
func Validate(cfg *Config) error {
	// Run structural checks first
	if err := ValidateStructure(cfg); err != nil {
		return err
	}

	var errs []string

	// Check summary model and prompt_file (only when enabled)
	if cfg.Summary.Enabled {
		if _, ok := cfg.Models[cfg.Summary.Model]; !ok {
			errs = append(errs, fmt.Sprintf("summary references unknown model %q", cfg.Summary.Model))
		}
		if cfg.Summary.PromptFile == "" {
			errs = append(errs, "summary.prompt_file must not be empty")
		}
	}

	// Check doctor model and prompt_file
	if _, ok := cfg.Models[cfg.Doctor.Model]; !ok {
		errs = append(errs, fmt.Sprintf("doctor references unknown model %q", cfg.Doctor.Model))
	}
	if cfg.Doctor.PromptFile == "" {
		errs = append(errs, "doctor.prompt_file must not be empty")
	}

	// Check unblock model and prompt_file
	if _, ok := cfg.Models[cfg.Unblock.Model]; !ok {
		errs = append(errs, fmt.Sprintf("unblock references unknown model %q", cfg.Unblock.Model))
	}
	if cfg.Unblock.PromptFile == "" {
		errs = append(errs, "unblock.prompt_file must not be empty")
	}

	// Overlap advisory model is optional — algorithmic detection (ADR-041)
	// does not require a model. The model field is retained for potential
	// future hybrid use but is not validated.

	// Check audit model and prompt_file
	if _, ok := cfg.Models[cfg.Audit.Model]; !ok {
		errs = append(errs, fmt.Sprintf("audit references unknown model %q", cfg.Audit.Model))
	}
	if cfg.Audit.PromptFile == "" {
		errs = append(errs, "audit.prompt_file must not be empty")
	}

	// Check identity presence
	if cfg.Identity == nil {
		errs = append(errs, "identity not configured. Run 'wolfcastle init' first")
	}

	if len(errs) > 0 {
		return fmt.Errorf("config validation failed:\n  - %s", strings.Join(errs, "\n  - "))
	}
	return nil
}
