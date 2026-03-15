# ADR-053: Centralized Configuration Defaults

## Status
Accepted

## Date
2026-03-14

## Context
Config defaults are scattered across multiple locations: `config.Load()` initializes some fields, `config.Validate()` applies some defaults during validation, embedded template files contain others, and the daemon fills in remaining gaps at runtime. This makes it difficult to answer "what is the default value of X?" without tracing through multiple code paths. Scattered defaults also invite inconsistency — the same field may receive different default values depending on which code path initializes it first.

Consolidating all defaults into a single function provides a clear, auditable source of truth and simplifies the config loading pipeline.

## Decision

Create a `DefaultConfig()` function in `internal/config/config.go` that returns a fully-populated `Config` struct with all operational defaults. This becomes the single source of truth for default values.

### Structure

```go
func DefaultConfig() *Config {
    return &Config{
        Models: map[string]ModelDef{
            "fast":  {Command: "claude", Args: [...]},
            "mid":   {Command: "claude", Args: [...]},
            "heavy": {Command: "claude", Args: [...]},
        },
        Pipeline: PipelineConfig{
            Stages: []PipelineStage{
                {Name: "expand", Model: "fast", PromptFile: "expand.md"},
                {Name: "file", Model: "mid", PromptFile: "file.md"},
                {Name: "execute", Model: "heavy", PromptFile: "execute.md"},
            },
        },
        Daemon: DaemonConfig{
            PollIntervalSeconds:        10,
            BlockedPollIntervalSeconds: 60,
            InvocationTimeoutSeconds:   600,
            MaxIterations:              0,
            MaxRestarts:                3,
            RestartDelaySeconds:        30,
            LogLevel:                   "info",
            LockTimeoutSeconds:         5,
        },
        Failure: FailureConfig{
            DecompositionThreshold: 10,
            MaxDecompositionDepth:  5,
            HardCap:               50,
        },
        Retries: RetryConfig{
            InitialDelaySeconds: 30,
            MaxDelaySeconds:     600,
            MaxRetries:          -1,
        },
        Logs: LogConfig{
            MaxFiles:   100,
            MaxAgeDays: 30,
        },
        // ... all other sections with complete defaults
    }
}
```

### Merge Pipeline

```
DefaultConfig() → overlay base/config.json → overlay custom/config.json → overlay local/config.json → Validate()
```

The `Load()` function changes from "read JSON, fill in missing fields" to "start from defaults, overlay user JSON, validate." Fields not present in user config retain their defaults. Fields explicitly set to `null` in user config delete the default (per ADR-018 null-deletion semantics).

### DefaultLocalConfig()

A separate `DefaultLocalConfig()` function handles identity defaults (reading hostname and username from the environment), keeping environment-dependent values isolated from the pure-data defaults in `DefaultConfig()`.

## Consequences
- "What is the default for X?" is answered by reading one function
- No more scattered default initialization across Load/Validate/daemon
- User config files only need to contain overrides — empty `config.json` produces a fully valid config
- The embedded template for `config.json` written by `wolfcastle init` can be a minimal override file rather than a full config dump
- Test code can call `DefaultConfig()` to get a valid config without building one field by field
