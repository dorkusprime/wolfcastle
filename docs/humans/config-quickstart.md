# Configuration Quickstart

Four common configuration tasks, each under a minute.

## Change Your Model

The `heavy` model handles execution by default (Claude Opus 4.6). To swap it, drop this into `.wolfcastle/system/custom/config.json`:

```json
{
  "models": {
    "heavy": {
      "command": "claude",
      "args": [
        "-p",
        "--model", "claude-sonnet-4-6",
        "--output-format", "stream-json",
        "--verbose",
        "--dangerously-skip-permissions"
      ]
    }
  }
}
```

Or use the CLI, which writes to `system/local/` by default:

```bash
wolfcastle config set models.heavy.args '["-p","--model","claude-sonnet-4-6","--output-format","stream-json","--verbose","--dangerously-skip-permissions"]'
```

Use `--tier custom` to write to the team-shared tier instead of local.

## Add a Custom Stage

Stages live in `pipeline.stages` as named entries, and `pipeline.stage_order` controls which ones run. To add a `spec-review` stage that runs between intake and execute, put this in `.wolfcastle/system/custom/config.json`:

```json
{
  "pipeline": {
    "stages": {
      "spec-review": {
        "model": "mid",
        "prompt_file": "stages/spec-review.md",
        "allowed_commands": ["spec list", "status"]
      }
    },
    "stage_order": ["intake", "spec-review", "execute"]
  }
}
```

Then drop your stage prompt at `.wolfcastle/system/custom/prompts/stages/spec-review.md`. The stage won't run without both the config entry and the prompt file.

## Override a Prompt

Wolfcastle resolves prompts through three tiers: `base/prompts/` (defaults), `custom/prompts/` (team-shared), `local/prompts/` (personal). A file in a higher tier replaces the same-named file in a lower tier entirely.

To override the execute stage prompt for your whole team, place your version at:

```
.wolfcastle/system/custom/prompts/stages/execute.md
```

For a personal override only you see, use `local/` instead:

```
.wolfcastle/system/local/prompts/stages/execute.md
```

No config changes needed. Wolfcastle picks up the override on the next iteration.

## Set a Task Class

Classes are behavioral prompts that shape how the agent approaches a task. Assign one at creation time:

```bash
wolfcastle task add "Implement auth middleware" --node my-project/auth --class coding/typescript
```

This selects the `coding/typescript` prompt, which guides the agent toward TypeScript idioms and conventions. For framework-specific behavior, use a nested class like `coding/typescript/react`, which falls back to `coding/typescript` if no React-specific prompt exists.

See [Task Classes](task-classes.md) for the full list of built-in classes and how to create your own.

---

For how configuration works under the hood, see the [Configuration Guide](configuration.md). For every available field, see the [Config Reference](config-reference.md).
