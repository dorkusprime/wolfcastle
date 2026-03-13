package config

import "fmt"

// Validate checks the resolved config for consistency.
func Validate(cfg *Config) error {
	// Check model references in pipeline stages
	for _, stage := range cfg.Pipeline.Stages {
		if _, ok := cfg.Models[stage.Model]; !ok {
			return fmt.Errorf("pipeline stage %q references unknown model %q", stage.Name, stage.Model)
		}
	}

	// Check stage name uniqueness
	names := make(map[string]bool)
	for _, stage := range cfg.Pipeline.Stages {
		if names[stage.Name] {
			return fmt.Errorf("duplicate pipeline stage name %q", stage.Name)
		}
		names[stage.Name] = true
	}

	// Check summary model reference
	if cfg.Summary.Enabled {
		if _, ok := cfg.Models[cfg.Summary.Model]; !ok {
			return fmt.Errorf("summary references unknown model %q", cfg.Summary.Model)
		}
	}

	// Check doctor model reference
	if _, ok := cfg.Models[cfg.Doctor.Model]; !ok {
		return fmt.Errorf("doctor references unknown model %q", cfg.Doctor.Model)
	}

	// Check unblock model reference
	if _, ok := cfg.Models[cfg.Unblock.Model]; !ok {
		return fmt.Errorf("unblock references unknown model %q", cfg.Unblock.Model)
	}

	// Check overlap advisory model reference
	if cfg.OverlapAdvisory.Enabled {
		if _, ok := cfg.Models[cfg.OverlapAdvisory.Model]; !ok {
			return fmt.Errorf("overlap_advisory references unknown model %q", cfg.OverlapAdvisory.Model)
		}
	}

	// Check audit model reference
	if _, ok := cfg.Models[cfg.Audit.Model]; !ok {
		return fmt.Errorf("audit references unknown model %q", cfg.Audit.Model)
	}

	// Check failure thresholds
	if cfg.Failure.HardCap < cfg.Failure.DecompositionThreshold {
		return fmt.Errorf("failure.hard_cap (%d) must be >= failure.decomposition_threshold (%d)",
			cfg.Failure.HardCap, cfg.Failure.DecompositionThreshold)
	}

	// Check identity presence
	if cfg.Identity == nil {
		return fmt.Errorf("identity not configured — run wolfcastle init first")
	}

	return nil
}
