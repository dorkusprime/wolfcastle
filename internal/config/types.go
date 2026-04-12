package config

// Config is the resolved Wolfcastle configuration after merging
// hardcoded defaults + base/config.json + custom/config.json + local/config.json.
type Config struct {
	Version         int                 `json:"version"`
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
	Archive         ArchiveConfig       `json:"archive"`
	Knowledge       KnowledgeConfig     `json:"knowledge"`
	TaskClasses     map[string]ClassDef `json:"task_classes,omitempty"`

	// Warnings collects non-fatal diagnostic messages from config loading,
	// such as unknown fields in tier files. Excluded from JSON serialization.
	Warnings []string `json:"-"`
}

// ModelDef defines a CLI model invocation.
type ModelDef struct {
	Command string   `json:"command"`
	Args    []string `json:"args"`
}

// PipelineConfig defines the stage pipeline.
type PipelineConfig struct {
	Stages     map[string]PipelineStage `json:"stages"`
	StageOrder []string                 `json:"stage_order,omitempty"`
	Planning   PlanningConfig           `json:"planning"`
}

// PlanningConfig controls orchestrator planning passes.
type PlanningConfig struct {
	Enabled         bool   `json:"enabled"`
	Model           string `json:"model,omitempty"`
	MaxChildren     int    `json:"max_children,omitempty"`
	MaxTasksPerLeaf int    `json:"max_tasks_per_leaf,omitempty"`
	MaxReplans      int    `json:"max_replans,omitempty"`
	MaxReviewPasses int    `json:"max_review_passes,omitempty"`
}

// PipelineStage defines a single pipeline stage.
type PipelineStage struct {
	Model              string   `json:"model"`
	PromptFile         string   `json:"prompt_file"`
	Enabled            *bool    `json:"enabled,omitempty"`
	SkipPromptAssembly *bool    `json:"skip_prompt_assembly,omitempty"`
	AllowedCommands    []string `json:"allowed_commands,omitempty"`
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

// LogsConfig controls NDJSON log retention (ADR-012).
type LogsConfig struct {
	MaxFiles   int  `json:"max_files"`
	MaxAgeDays int  `json:"max_age_days"`
	Compress   bool `json:"compress"`
}

// RetriesConfig controls retry behavior for failed model invocations.
type RetriesConfig struct {
	InitialDelaySeconds int `json:"initial_delay_seconds"`
	MaxDelaySeconds     int `json:"max_delay_seconds"`
	MaxRetries          int `json:"max_retries"`
}

// FailureConfig controls decomposition thresholds and hard failure caps.
type FailureConfig struct {
	DecompositionThreshold int `json:"decomposition_threshold"`
	MaxDecompositionDepth  int `json:"max_decomposition_depth"`
	HardCap                int `json:"hard_cap"`
}

// IdentityConfig identifies the engineer and machine running Wolfcastle.
type IdentityConfig struct {
	User    string `json:"user"`
	Machine string `json:"machine"`
}

// SummaryConfig controls the optional post-completion summary stage (ADR-016).
type SummaryConfig struct {
	Enabled    bool   `json:"enabled"`
	Model      string `json:"model"`
	PromptFile string `json:"prompt_file"`
}

// DocsConfig controls the documentation output directory.
type DocsConfig struct {
	Directory string `json:"directory"`
}

// ValidationConfig defines user-specified validation commands run after task completion.
type ValidationConfig struct {
	Commands []ValidationCommand `json:"commands"`
}

// ValidationCommand is a single shell command used for post-task validation.
type ValidationCommand struct {
	Name           string `json:"name"`
	Run            string `json:"run"`
	TimeoutSeconds int    `json:"timeout_seconds"`
}

// PromptsConfig controls prompt fragment inclusion and exclusion.
type PromptsConfig struct {
	Fragments        []string `json:"fragments"`
	ExcludeFragments []string `json:"exclude_fragments"`
}

// ParallelConfig controls parallel sibling node execution.
type ParallelConfig struct {
	Enabled    bool `json:"enabled"`
	MaxWorkers int  `json:"max_workers"`
}

// DaemonConfig controls the daemon's polling, timeout, and restart behavior.
type DaemonConfig struct {
	PollIntervalSeconds        int            `json:"poll_interval_seconds"`
	BlockedPollIntervalSeconds int            `json:"blocked_poll_interval_seconds"`
	InboxPollIntervalSeconds   int            `json:"inbox_poll_interval_seconds"`
	MaxIterations              int            `json:"max_iterations"`
	MaxTurnsPerInvocation      int            `json:"max_turns_per_invocation"`
	InvocationTimeoutSeconds   int            `json:"invocation_timeout_seconds"`
	StallTimeoutSeconds        int            `json:"stall_timeout_seconds"`
	MaxRestarts                int            `json:"max_restarts"`
	RestartDelaySeconds        int            `json:"restart_delay_seconds"`
	LogLevel                   string         `json:"log_level"`
	Parallel                   ParallelConfig `json:"parallel"`
}

// GitConfig controls automatic commit behavior and branch verification.
type GitConfig struct {
	AutoCommit            bool   `json:"auto_commit"`
	CommitOnSuccess       bool   `json:"commit_on_success"`
	CommitOnFailure       bool   `json:"commit_on_failure"`
	CommitState           bool   `json:"commit_state"`
	CommitPrefix          string `json:"commit_prefix"`
	CommitMessageFormat   string `json:"commit_message_format"`
	VerifyBranch          bool   `json:"verify_branch"`
	SkipHooksOnAutoCommit bool   `json:"skip_hooks_on_auto_commit"`
}

// DoctorConfig configures the structural validation and repair command (ADR-025).
type DoctorConfig struct {
	Model      string `json:"model"`
	PromptFile string `json:"prompt_file"`
}

// OverlapConfig configures the overlap advisory system (ADR-027, ADR-041).
type OverlapConfig struct {
	Enabled   bool    `json:"enabled"`
	Model     string  `json:"model"`
	Threshold float64 `json:"threshold"`
}

// UnblockConfig configures the unblock workflow (ADR-028).
type UnblockConfig struct {
	Model      string `json:"model"`
	PromptFile string `json:"prompt_file"`
}

// AuditCommandConfig configures the codebase audit command (ADR-029).
type AuditCommandConfig struct {
	Model        string `json:"model"`
	PromptFile   string `json:"prompt_file"`
	RequireTests string `json:"require_tests,omitempty"` // "block" (default), "warn", "skip"
}

// ArchiveConfig controls automatic archival of completed project trees.
type ArchiveConfig struct {
	AutoArchiveEnabled    bool `json:"auto_archive_enabled"`
	AutoArchiveDelayHours int  `json:"auto_archive_delay_hours"`
	PollIntervalSeconds   int  `json:"archive_poll_interval_seconds"`
}

// KnowledgeConfig controls codebase knowledge file settings.
type KnowledgeConfig struct {
	MaxTokens int `json:"max_tokens"`
}

// ClassDef defines a single task class entry in the config. Classes are
// behavioral modifiers: a description shown to the intake model for
// classification, and an optional model override for execution.
type ClassDef struct {
	Description string `json:"description"`
	Model       string `json:"model,omitempty"`
}
