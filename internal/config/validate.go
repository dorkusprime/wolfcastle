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

	// Pipeline must have at least one stage
	if len(cfg.Pipeline.Stages) == 0 {
		errs = append(errs, "pipeline has no stages — at least one stage is required")
	}

	// All stage model references must exist
	for _, stage := range cfg.Pipeline.Stages {
		if _, ok := cfg.Models[stage.Model]; !ok {
			errs = append(errs, fmt.Sprintf("pipeline stage %q references unknown model %q", stage.Name, stage.Model))
		}
	}

	// Stage names must be unique
	names := make(map[string]bool)
	for _, stage := range cfg.Pipeline.Stages {
		if stage.Name == "" {
			errs = append(errs, "pipeline stage has empty name")
		} else if names[stage.Name] {
			errs = append(errs, fmt.Sprintf("duplicate pipeline stage name %q", stage.Name))
		}
		names[stage.Name] = true
	}

	// Stage prompt files must be non-empty
	for _, stage := range cfg.Pipeline.Stages {
		if stage.PromptFile == "" {
			errs = append(errs, fmt.Sprintf("pipeline stage %q has empty prompt_file", stage.Name))
		}
	}

	// Failure thresholds
	if cfg.Failure.DecompositionThreshold < 0 {
		errs = append(errs, fmt.Sprintf("failure.decomposition_threshold (%d) must be >= 0", cfg.Failure.DecompositionThreshold))
	}
	if cfg.Failure.MaxDecompositionDepth < 0 {
		errs = append(errs, fmt.Sprintf("failure.max_decomposition_depth (%d) must be >= 0", cfg.Failure.MaxDecompositionDepth))
	}
	if cfg.Failure.HardCap < 0 {
		errs = append(errs, fmt.Sprintf("failure.hard_cap (%d) must be >= 0", cfg.Failure.HardCap))
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
	if cfg.Daemon.InvocationTimeoutSeconds <= 0 {
		errs = append(errs, fmt.Sprintf("daemon.invocation_timeout_seconds (%d) must be > 0", cfg.Daemon.InvocationTimeoutSeconds))
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

	// Log retention
	if cfg.Logs.MaxFiles <= 0 {
		errs = append(errs, fmt.Sprintf("logs.max_files (%d) must be > 0", cfg.Logs.MaxFiles))
	}
	if cfg.Logs.MaxAgeDays <= 0 {
		errs = append(errs, fmt.Sprintf("logs.max_age_days (%d) must be > 0", cfg.Logs.MaxAgeDays))
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

	// Check summary model reference (only when enabled)
	if cfg.Summary.Enabled {
		if _, ok := cfg.Models[cfg.Summary.Model]; !ok {
			errs = append(errs, fmt.Sprintf("summary references unknown model %q", cfg.Summary.Model))
		}
	}

	// Check doctor model reference
	if _, ok := cfg.Models[cfg.Doctor.Model]; !ok {
		errs = append(errs, fmt.Sprintf("doctor references unknown model %q", cfg.Doctor.Model))
	}

	// Check unblock model reference
	if _, ok := cfg.Models[cfg.Unblock.Model]; !ok {
		errs = append(errs, fmt.Sprintf("unblock references unknown model %q", cfg.Unblock.Model))
	}

	// Check overlap advisory model reference (only when enabled)
	if cfg.OverlapAdvisory.Enabled {
		if _, ok := cfg.Models[cfg.OverlapAdvisory.Model]; !ok {
			errs = append(errs, fmt.Sprintf("overlap_advisory references unknown model %q", cfg.OverlapAdvisory.Model))
		}
	}

	// Check audit model reference
	if _, ok := cfg.Models[cfg.Audit.Model]; !ok {
		errs = append(errs, fmt.Sprintf("audit references unknown model %q", cfg.Audit.Model))
	}

	// Check identity presence
	if cfg.Identity == nil {
		errs = append(errs, "identity not configured — run wolfcastle init first")
	}

	if len(errs) > 0 {
		return fmt.Errorf("config validation failed:\n  - %s", strings.Join(errs, "\n  - "))
	}
	return nil
}
