# Configuration

## Three Tiers

Configuration merges across three tiers. Each tier overrides the one below it.

| Tier | Location | Ownership | Git Status |
|------|----------|-----------|------------|
| **base/** | `.wolfcastle/base/` | Wolfcastle | Gitignored. Regenerated on init/update. |
| **custom/** | `.wolfcastle/custom/` | Team | Committed. Shared across all engineers. |
| **local/** | `.wolfcastle/local/` | You | Gitignored. Personal overrides. |

**JSON objects** deep-merge recursively. Override a single nested field without rewriting the whole object. **Arrays** replace entirely. Set a field to **`null`** in a higher tier to delete it from the resolved config.

The same three-tier resolution applies to prompt templates and [rule fragments](#rule-fragments). Same-named files in higher tiers completely replace lower tier versions. New files are added.

Two config files control runtime behavior:

- **`config.json`**: Team-shared, committed. Models, [pipelines](#pipelines), thresholds, validation commands.
- **`config.local.json`**: Personal, gitignored. [Identity](#identity), model overrides, local preferences.

## Models

Wolfcastle does not embed any model SDK. It does not import any provider library. It does not care who made your model or where it runs.

Models are defined as CLI commands:

```json
{
  "models": {
    "fast": {
      "command": "claude",
      "args": ["-p", "--model", "claude-haiku-4-5-20251001", "--output-format", "stream-json"]
    },
    "mid": {
      "command": "claude",
      "args": ["-p", "--model", "claude-sonnet-4-6", "--output-format", "stream-json"]
    },
    "heavy": {
      "command": "claude",
      "args": ["-p", "--model", "claude-opus-4-6", "--output-format", "stream-json"]
    }
  }
}
```

Any CLI tool that accepts a prompt on stdin and produces output on stdout is a valid model. Switch providers by changing config. No code changes. No recompilation.

Authentication is your problem. Use environment variables, CLI login commands, or whatever your provider demands. Wolfcastle calls the command. The command figures out the rest.

## Pipelines

The [daemon](how-it-works.md#the-daemon) runs a pipeline of stages. Each stage names a model tier and a prompt file:

```json
{
  "pipeline": {
    "stages": [
      { "name": "expand",  "model": "fast", "prompt_file": "expand.md" },
      { "name": "file",    "model": "mid",  "prompt_file": "file.md" },
      { "name": "execute", "model": "heavy", "prompt_file": "execute.md" },
      { "name": "summary", "model": "fast", "prompt_file": "summary.md", "enabled": true }
    ]
  }
}
```

Add stages. Remove stages. Reorder stages. Run a single-stage pipeline with one model that does everything. Stages can be individually enabled or disabled.

## Identity

Your identity lives in `config.local.json`, auto-populated on `wolfcastle init`:

```json
{
  "identity": {
    "user": "wild",
    "machine": "macbook"
  }
}
```

This determines your project [namespace](collaboration.md#engineer-namespacing). Your work lives under `.wolfcastle/projects/wild-macbook/`. Nobody else writes there. You write nowhere else.

## Rule Fragments

Prompts and rules are assembled from composable fragments with sensible defaults. Wolfcastle ships base fragments covering git conventions, commit format, ADR usage, and more. Teams add custom fragments in `custom/`. Engineers add personal fragments in `local/`. (See the [project layout](cli.md#project-layout) for directory structure.)

Fragments merge in order defined by config. An empty array means auto-discovery in alphabetical order. An explicit array means you control the sequence.

## Security

Wolfcastle does not sandbox anything. Security is configured at the model level through CLI flags in the `models` dictionary:

```json
{
  "models": {
    "heavy": {
      "command": "claude",
      "args": ["--dangerously-skip-permissions", "-p", "--model", "claude-opus-4-6"]
    }
  }
}
```

The executing model's capabilities are determined entirely by the flags you gave it. Teams enforce permissions through config review of `config.json`. Individual engineers loosen permissions in gitignored `config.local.json` at their own risk.
