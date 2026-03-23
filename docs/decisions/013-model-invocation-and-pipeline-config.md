# ADR-013: Model Invocation via CLI Shell-Out with Pipeline Configuration

## Status
Accepted

## Date
2026-03-12

## Context
Wolfcastle is model-agnostic (ADR-004) and needs to invoke different models at different pipeline stages. Ralph hardcoded three Claude models and shelled out to the `claude` CLI. We need a flexible approach that avoids building API clients into Wolfcastle while allowing teams to configure their model tiers and pipeline stages.

## Decision

### Model Invocation
Wolfcastle invokes models by shelling out to a CLI command. It does not make direct API calls. This keeps Wolfcastle provider-agnostic: any CLI that accepts a prompt and produces output works.

### Model Definitions
Models are defined in a `models` dictionary in `config.json`. Each model has a key (used as a reference), a command, and args (including permission flags, output format, and any other CLI-specific settings):

```json
{
  "models": {
    "fast": {
      "command": "claude",
      "args": ["-p", "--model", "claude-haiku-4-5-20251001", "--output-format", "stream-json", "--dangerously-skip-permissions"]
    },
    "mid": {
      "command": "claude",
      "args": ["-p", "--model", "claude-sonnet-4-6", "--output-format", "stream-json", "--dangerously-skip-permissions"]
    },
    "heavy": {
      "command": "claude",
      "args": ["-p", "--model", "claude-opus-4-6", "--output-format", "stream-json", "--verbose", "--dangerously-skip-permissions"]
    }
  }
}
```

### Pipeline Stages
Pipeline stages reference models by key and specify a prompt file:

```json
{
  "pipeline": {
    "stages": [
      { "name": "expand", "model": "fast", "prompt_file": "expand.md" },
      { "name": "file", "model": "mid", "prompt_file": "file.md" },
      { "name": "execute", "model": "heavy", "prompt_file": "execute.md" }
    ]
  }
}
```

### Override via Local Config
Individual engineers can override model definitions in `local/config.json`: e.g. swapping "heavy" to a cheaper model during development. The three-tier merge (ADR-009, ADR-063) handles resolution: `base/config.json` → `custom/config.json` → `local/config.json`.

## Consequences
- Wolfcastle has zero provider-specific code: no API clients, no auth handling
- Adding a new provider means pointing `command` at a different CLI
- Permission flags (e.g. `--dangerously-skip-permissions`) are explicitly configured per model, visible in config
- Pipeline stages are decoupled from model details: renaming or swapping a model tier doesn't require updating every stage
- Engineers can experiment with different models locally without affecting the team
