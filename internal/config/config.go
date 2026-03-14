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
				Args:    []string{"-p", "--model", "claude-haiku-4-5-20251001", "--output-format", "stream-json", "--dangerously-skip-permissions"},
			},
			"mid": {
				Command: "claude",
				Args:    []string{"-p", "--model", "claude-sonnet-4-6", "--output-format", "stream-json", "--dangerously-skip-permissions"},
			},
			"heavy": {
				Command: "claude",
				Args:    []string{"-p", "--model", "claude-opus-4-6", "--output-format", "stream-json", "--verbose", "--dangerously-skip-permissions"},
			},
		},
		Pipeline: PipelineConfig{
			Stages: []PipelineStage{
				{Name: "expand", Model: "fast", PromptFile: "expand.md"},
				{Name: "file", Model: "mid", PromptFile: "file.md"},
				{Name: "execute", Model: "heavy", PromptFile: "execute.md"},
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
			BlockedPollIntervalSeconds: 60,
			MaxIterations:              -1,
			MaxTurnsPerInvocation:      200,
			InvocationTimeoutSeconds:   3600,
			MaxRestarts:                3,
			RestartDelaySeconds:        2,
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

// Load reads and merges configuration from the .wolfcastle directory.
// Resolution order: hardcoded defaults <- config.json <- config.local.json
func Load(wolfcastleDir string) (*Config, error) {
	// Start with defaults as raw map
	defaultsRaw, err := structToMap(Defaults())
	if err != nil {
		return nil, err
	}

	// Load config.json
	configPath := filepath.Join(wolfcastleDir, "config.json")
	configRaw, err := loadJSONFile(configPath)
	if err != nil && !os.IsNotExist(err) {
		return nil, err
	}
	if configRaw != nil {
		defaultsRaw = DeepMerge(defaultsRaw, configRaw)
	}

	// Load config.local.json
	localPath := filepath.Join(wolfcastleDir, "config.local.json")
	localRaw, err := loadJSONFile(localPath)
	if err != nil && !os.IsNotExist(err) {
		return nil, err
	}
	if localRaw != nil {
		defaultsRaw = DeepMerge(defaultsRaw, localRaw)
	}

	// Marshal back to Config struct
	merged, err := json.Marshal(defaultsRaw)
	if err != nil {
		return nil, err
	}
	var cfg Config
	if err := json.Unmarshal(merged, &cfg); err != nil {
		return nil, err
	}

	// Validate structural integrity (skip identity — handled by resolver)
	if err := ValidateStructure(&cfg); err != nil {
		return nil, fmt.Errorf("config validation failed: %w", err)
	}

	return &cfg, nil
}

func loadJSONFile(path string) (map[string]any, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	var m map[string]any
	if err := json.Unmarshal(data, &m); err != nil {
		return nil, err
	}
	return m, nil
}

func structToMap(v any) (map[string]any, error) {
	data, err := json.Marshal(v)
	if err != nil {
		return nil, err
	}
	var m map[string]any
	if err := json.Unmarshal(data, &m); err != nil {
		return nil, err
	}
	return m, nil
}
