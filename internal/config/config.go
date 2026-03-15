// Package config handles loading, merging, and validating the Wolfcastle
// configuration. Configuration is resolved by deep-merging hardcoded
// defaults with the three-tier config files: base/config.json,
// custom/config.json, and local/config.json (ADR-018, ADR-053, ADR-063).
package config

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
)

// Defaults returns the hardcoded default configuration.
func Defaults() *Config {
	return &Config{
		Models: map[string]ModelDef{
			"fast": {
				Command: "claude",
				Args:    []string{"-p", "--model", "claude-haiku-4-5-20251001", "--output-format", "stream-json", "--verbose", "--dangerously-skip-permissions"},
			},
			"mid": {
				Command: "claude",
				Args:    []string{"-p", "--model", "claude-sonnet-4-6", "--output-format", "stream-json", "--verbose", "--dangerously-skip-permissions"},
			},
			"heavy": {
				Command: "claude",
				Args:    []string{"-p", "--model", "claude-opus-4-6", "--output-format", "stream-json", "--verbose", "--dangerously-skip-permissions"},
			},
		},
		Pipeline: PipelineConfig{
			Stages: []PipelineStage{
				{
					Name:            "intake",
					Model:           "mid",
					PromptFile:      "intake.md",
					AllowedCommands: []string{"project create", "task add", "status"},
				},
				{
					Name:            "execute",
					Model:           "heavy",
					PromptFile:      "execute.md",
					AllowedCommands: []string{"project create", "task add", "task block", "audit breadcrumb", "audit escalate", "audit gap", "audit fix-gap", "audit scope", "audit summary", "audit resolve-escalation", "status", "spec list"},
				},
			},
		},
		Logs: LogsConfig{
			MaxFiles:   100,
			MaxAgeDays: 30,
			Compress:   true,
		},
		Retries: RetriesConfig{
			InitialDelaySeconds: 30,
			MaxDelaySeconds:     600,
			MaxRetries:          -1,
		},
		Failure: FailureConfig{
			DecompositionThreshold: 10,
			MaxDecompositionDepth:  5,
			HardCap:                50,
		},
		Summary: SummaryConfig{
			Enabled:    true,
			Model:      "fast",
			PromptFile: "summary.md",
		},
		Docs: DocsConfig{
			Directory: "docs",
		},
		Validation: ValidationConfig{
			Commands: []ValidationCommand{},
		},
		Prompts: PromptsConfig{
			Fragments:        []string{},
			ExcludeFragments: []string{},
		},
		Daemon: DaemonConfig{
			PollIntervalSeconds:        5,
			BlockedPollIntervalSeconds: 5,
			InboxPollIntervalSeconds:   5,
			MaxIterations:              -1,
			MaxTurnsPerInvocation:      200,
			InvocationTimeoutSeconds:   3600,
			MaxRestarts:                3,
			RestartDelaySeconds:        2,
			LogLevel:                   "info",
		},
		Git: GitConfig{
			AutoCommit:          true,
			CommitMessageFormat: "wolfcastle: {action} [{node}]",
			VerifyBranch:        true,
		},
		Doctor: DoctorConfig{
			Model:      "mid",
			PromptFile: "doctor.md",
		},
		OverlapAdvisory: OverlapConfig{
			Enabled:   true,
			Model:     "fast",
			Threshold: 0.3,
		},
		Unblock: UnblockConfig{
			Model:      "heavy",
			PromptFile: "unblock.md",
		},
		Audit: AuditCommandConfig{
			Model:      "heavy",
			PromptFile: "audit.md",
		},
	}
}

// configTiers lists the three-tier config file paths relative to the
// wolfcastle directory, in resolution order from lowest to highest priority.
var configTiers = []string{
	"base/config.json",
	"custom/config.json",
	"local/config.json",
}

// Load reads and merges configuration from the .wolfcastle directory.
// Resolution order: hardcoded defaults <- base/config.json <- custom/config.json <- local/config.json
func Load(wolfcastleDir string) (*Config, error) {
	// Start with defaults as raw map
	result, err := structToMap(Defaults())
	if err != nil {
		return nil, fmt.Errorf("marshaling defaults: %w", err)
	}

	// Overlay each tier in order
	for _, tier := range configTiers {
		path := filepath.Join(wolfcastleDir, tier)
		raw, err := loadJSONFile(path)
		if err != nil && !os.IsNotExist(err) {
			return nil, fmt.Errorf("reading %s: %w", tier, err)
		}
		if raw != nil {
			result = DeepMerge(result, raw)
		}
	}

	// Marshal back to Config struct
	merged, err := json.Marshal(result)
	if err != nil {
		return nil, fmt.Errorf("marshaling merged config: %w", err)
	}
	var cfg Config
	if err := json.Unmarshal(merged, &cfg); err != nil {
		return nil, fmt.Errorf("unmarshaling merged config: %w", err)
	}

	// Validate structural integrity (skip identity — handled by resolver)
	if err := ValidateStructure(&cfg); err != nil {
		return nil, fmt.Errorf("config validation failed: %w", err)
	}

	return &cfg, nil
}

// loadJSONFile reads a JSON file and returns its contents as a map.
// Returns (nil, os.ErrNotExist) if the file does not exist.
func loadJSONFile(path string) (map[string]any, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	var m map[string]any
	if err := json.Unmarshal(data, &m); err != nil {
		return nil, fmt.Errorf("parsing %s: %w", filepath.Base(path), err)
	}
	return m, nil
}

// structToMap converts a struct to a map[string]any via JSON round-trip.
func structToMap(v any) (map[string]any, error) {
	data, err := json.Marshal(v)
	if err != nil {
		return nil, fmt.Errorf("marshaling struct: %w", err)
	}
	var m map[string]any
	if err := json.Unmarshal(data, &m); err != nil {
		return nil, fmt.Errorf("unmarshaling to map: %w", err)
	}
	return m, nil
}
