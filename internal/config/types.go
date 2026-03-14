package config

// Config is the resolved Wolfcastle configuration after merging
// config.json + config.local.json + hardcoded defaults.
type Config struct {
	Models          map[string]ModelDef `json:"models"`
	Pipeline        PipelineConfig      `json:"pipeline"`
	Logs            LogsConfig          `json:"logs"`
	Retries         RetriesConfig       `json:"retries"`
	Failure         FailureConfig       `json:"failure"`
	Identity        *IdentityConfig     `json:"identity,omitempty"`
	Summary         SummaryConfig       `json:"summary"`
	Docs            DocsConfig          `json:"docs"`
	Validation      ValidationConfig    `json:"validation"`
	Prompts         PromptsConfig       `json:"prompts"`
	Daemon          DaemonConfig        `json:"daemon"`
	Git             GitConfig           `json:"git"`
	Doctor          DoctorConfig        `json:"doctor"`
	OverlapAdvisory OverlapConfig       `json:"overlap_advisory"`
	Unblock         UnblockConfig       `json:"unblock"`
	Audit           AuditCommandConfig  `json:"audit"`
}

// ModelDef defines a CLI model invocation.
type ModelDef struct {
	Command string   `json:"command"`
	Args    []string `json:"args"`
}

// PipelineConfig defines the stage pipeline.
type PipelineConfig struct {
	Stages []PipelineStage `json:"stages"`
}

// PipelineStage defines a single pipeline stage.
type PipelineStage struct {
	Name               string `json:"name"`
	Model              string `json:"model"`
	PromptFile         string `json:"prompt_file"`
	Enabled            *bool  `json:"enabled,omitempty"`
	SkipPromptAssembly *bool  `json:"skip_prompt_assembly,omitempty"`
}

// IsEnabled returns whether the stage is enabled (default true).
func (s PipelineStage) IsEnabled() bool {
	if s.Enabled == nil {
		return true
	}
	return *s.Enabled
}

// ShouldSkipPromptAssembly returns whether to skip prompt assembly (default false).
func (s PipelineStage) ShouldSkipPromptAssembly() bool {
	if s.SkipPromptAssembly == nil {
		return false
	}
	return *s.SkipPromptAssembly
}

type LogsConfig struct {
	MaxFiles   int  `json:"max_files"`
	MaxAgeDays int  `json:"max_age_days"`
	Compress   bool `json:"compress"`
}

type RetriesConfig struct {
	InitialDelaySeconds int `json:"initial_delay_seconds"`
	MaxDelaySeconds     int `json:"max_delay_seconds"`
	MaxRetries          int `json:"max_retries"`
}

type FailureConfig struct {
	DecompositionThreshold int `json:"decomposition_threshold"`
	MaxDecompositionDepth  int `json:"max_decomposition_depth"`
	HardCap                int `json:"hard_cap"`
}

type IdentityConfig struct {
	User    string `json:"user"`
	Machine string `json:"machine"`
}

type SummaryConfig struct {
	Enabled    bool   `json:"enabled"`
	Model      string `json:"model"`
	PromptFile string `json:"prompt_file"`
}

type DocsConfig struct {
	Directory string `json:"directory"`
}

type ValidationConfig struct {
	Commands []ValidationCommand `json:"commands"`
}

type ValidationCommand struct {
	Name           string `json:"name"`
	Run            string `json:"run"`
	TimeoutSeconds int    `json:"timeout_seconds"`
}

type PromptsConfig struct {
	Fragments        []string `json:"fragments"`
	ExcludeFragments []string `json:"exclude_fragments"`
}

type DaemonConfig struct {
	PollIntervalSeconds        int    `json:"poll_interval_seconds"`
	BlockedPollIntervalSeconds int    `json:"blocked_poll_interval_seconds"`
	MaxIterations              int    `json:"max_iterations"`
	MaxTurnsPerInvocation      int    `json:"max_turns_per_invocation"`
	InvocationTimeoutSeconds   int    `json:"invocation_timeout_seconds"`
	MaxRestarts                int    `json:"max_restarts"`
	RestartDelaySeconds        int    `json:"restart_delay_seconds"`
	LogLevel                   string `json:"log_level"`
}

type GitConfig struct {
	AutoCommit          bool   `json:"auto_commit"`
	CommitMessageFormat string `json:"commit_message_format"`
	VerifyBranch        bool   `json:"verify_branch"`
}

type DoctorConfig struct {
	Model      string `json:"model"`
	PromptFile string `json:"prompt_file"`
}

type OverlapConfig struct {
	Enabled   bool    `json:"enabled"`
	Model     string  `json:"model"`
	Threshold float64 `json:"threshold"`
}

type UnblockConfig struct {
	Model      string `json:"model"`
	PromptFile string `json:"prompt_file"`
}

type AuditCommandConfig struct {
	Model      string `json:"model"`
	PromptFile string `json:"prompt_file"`
}
