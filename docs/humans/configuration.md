# Configuration Guide

Wolfcastle's configuration lives in a layered system of JSON files, prompt templates, and rule fragments. Everything resolves through three tiers, from broadest defaults to personal overrides, and the merge rules are simple enough to hold in your head. This page explains the machinery. For a quick first customization, see the [How It Works](how-it-works.md) page. For every field and its default, see the [Config Reference](config-reference.md).

---

## Three-Tier Directory Structure

All configuration lives under `.wolfcastle/system/`, split into three directories:

```
.wolfcastle/system/
├── base/                  ← Wolfcastle defaults (gitignored, regenerated)
│   ├── config.json
│   ├── prompts/
│   │   ├── stages/
│   │   │   ├── intake.md
│   │   │   └── execute.md
│   │   ├── classes/
│   │   │   ├── universal.md
│   │   │   └── coding/
│   │   │       ├── go.md
│   │   │       ├── python.md
│   │   │       └── ...
│   │   └── summary.md
│   ├── rules/
│   │   ├── adr-policy.md
│   │   └── git-conventions.md
│   └── audits/
│       └── audit.md
│
├── custom/                ← Team overrides (committed, shared)
│   ├── config.json
│   ├── prompts/           ← (optional, same structure as base)
│   └── rules/             ← (optional, same structure as base)
│
└── local/                 ← Personal overrides (gitignored)
    ├── config.json
    ├── prompts/           ← (optional, same structure as base)
    └── rules/             ← (optional, same structure as base)
```

Each tier has the same internal layout. The resolution order, from lowest to highest priority:

| Tier | Location | Ownership | Git Status |
|------|----------|-----------|------------|
| **base/** | `.wolfcastle/system/base/` | Wolfcastle | Gitignored. Regenerated on `wolfcastle init` and `wolfcastle update`. |
| **custom/** | `.wolfcastle/system/custom/` | Team | Committed. Shared across all engineers via version control. |
| **local/** | `.wolfcastle/system/local/` | You | Gitignored. Personal overrides that never leave your machine. |

**base/** is the foundation. Wolfcastle regenerates it on every `init` or `update`, so hand-edits here will be overwritten. Think of it as the read-only default layer.

**custom/** is where your team puts shared configuration: model definitions, pipeline tweaks, validation commands, custom class prompts. Because it's committed, everyone on the team gets the same settings. Code review applies here just as it does to application code.

**local/** is yours alone. Identity (username and machine) lives here automatically after `init`. Personal model overrides, experimental prompt tweaks, looser permissions for your own workflow, anything you wouldn't commit but need for your environment.

### Prompt and Rule Resolution

Prompts and rule fragments follow the same three-tier resolution as `config.json`. When Wolfcastle needs a file like `prompts/stages/execute.md`, it checks `local/` first, then `custom/`, then `base/`. The first match wins, and the match replaces the lower-tier version entirely (no partial merging of prompt content). New files added in higher tiers are simply included alongside the base set.

This means overriding a base prompt is as simple as placing a file with the same relative path in `custom/` or `local/`. You get the base version's behavior by default and can replace it cleanly when you need something different.

---

## Merge Semantics

Configuration merges happen across the three tiers, with higher tiers overriding lower ones. The merge follows three rules, and they apply recursively through the JSON structure.

### Rule 1: Objects Deep-Merge

JSON objects merge recursively. You can override a single nested field without rewriting the entire parent object.

**base/config.json:**
```json
{
  "daemon": {
    "poll_interval_seconds": 5,
    "stall_timeout_seconds": 120,
    "max_restarts": 3
  }
}
```

**custom/config.json:**
```json
{
  "daemon": {
    "stall_timeout_seconds": 300
  }
}
```

**Resolved result:**
```json
{
  "daemon": {
    "poll_interval_seconds": 5,
    "stall_timeout_seconds": 300,
    "max_restarts": 3
  }
}
```

Only `stall_timeout_seconds` changed. The other fields carry through from base untouched.

### Rule 2: Arrays Replace Entirely

Arrays are not merged element-by-element. A higher tier's array replaces the lower tier's array wholesale. This keeps ordering predictable.

**base/config.json:**
```json
{
  "pipeline": {
    "stage_order": ["intake", "execute"]
  }
}
```

**custom/config.json:**
```json
{
  "pipeline": {
    "stage_order": ["lint-check", "intake", "execute"]
  }
}
```

**Resolved result:**
```json
{
  "pipeline": {
    "stage_order": ["lint-check", "intake", "execute"]
  }
}
```

The custom array replaces the base array entirely. If you want to prepend a stage, you must include the full list.

### Rule 3: Null Deletes

Setting a field to `null` in a higher tier removes it from the resolved config.

**base/config.json:**
```json
{
  "models": {
    "fast": { "command": "claude", "args": ["..."] },
    "mid":  { "command": "claude", "args": ["..."] },
    "heavy": { "command": "claude", "args": ["..."] }
  }
}
```

**custom/config.json:**
```json
{
  "models": {
    "fast": null
  }
}
```

**Resolved result:**
```json
{
  "models": {
    "mid":  { "command": "claude", "args": ["..."] },
    "heavy": { "command": "claude", "args": ["..."] }
  }
}
```

The `fast` model is gone. Any stage or command referencing it will fail at runtime, so use this deliberately.

---

## Common Customizations

These recipes cover the most frequent configuration changes. Each shows the relevant JSON and, where available, the CLI shortcut. For the full field reference, see the [Config Reference](config-reference.md).

### Model Configuration

Wolfcastle does not embed any model SDK or provider library. Models are defined as CLI commands: an executable and its arguments. The assembled prompt is piped to stdin, and the model's output comes back on stdout. Any tool that follows that contract is a valid model.

**Defining a new model:**

```json
{
  "models": {
    "local-llama": {
      "command": "ollama",
      "args": ["run", "llama3", "--format", "json"]
    }
  }
}
```

Place this in `custom/config.json` to share it with the team, or `local/config.json` for personal use. The deep-merge rule means your new model is added alongside the existing defaults without overwriting them.

**Switching a stage's model:**

```json
{
  "pipeline": {
    "stages": {
      "execute": {
        "model": "local-llama"
      }
    }
  }
}
```

Only the `model` field on the `execute` stage changes; its prompt file, allowed commands, and enabled state carry through from base.

**CLI alternative:**

```
wolfcastle config set models.local-llama.command ollama
wolfcastle config set models.local-llama.args '["run", "llama3", "--format", "json"]'
wolfcastle config set pipeline.stages.execute.model local-llama
```

Authentication is your problem. Use environment variables, CLI login commands, or whatever your provider requires. Wolfcastle calls the command; the command handles the rest.

See [Config Reference: models](config-reference.md#models) for the full field breakdown.

### Pipeline Stages

The [daemon](how-it-works.md#the-daemon) runs a pipeline of stages for each iteration. Stages are defined as a dictionary keyed by name, and execution order is controlled by the `stage_order` array.

**Adding a custom stage:**

```json
{
  "pipeline": {
    "stages": {
      "spec-review": {
        "model": "mid",
        "prompt_file": "spec-review.md",
        "allowed_commands": ["status", "spec list"]
      }
    },
    "stage_order": ["intake", "spec-review", "execute"]
  }
}
```

The dict format lets you override a single stage's fields from a higher tier without rewriting the whole stages map. The `stage_order` array does require the full list (arrays replace, remember).

**Disabling a stage:**

```json
{
  "pipeline": {
    "stages": {
      "intake": {
        "enabled": false
      }
    }
  }
}
```

The stage remains in config but won't run. This is cleaner than removing it, because re-enabling is a one-field change.

See [Config Reference: pipeline](config-reference.md#pipeline) for all stage fields, including `skip_prompt_assembly` and `allowed_commands`.

### Class Overrides

Task classes are behavioral prompts that shape how the execution agent approaches work. The class system is covered in full on the [Task Classes](task-classes.md) page; this section covers the configuration mechanics.

**Adding a custom class:**

```json
{
  "task_classes": {
    "coding/go-internal": {
      "description": "Go code following internal team conventions, custom linter rules"
    }
  }
}
```

Then place your prompt at `system/custom/prompts/classes/coding/go-internal.md`. The prompt should describe your coding conventions, nothing about Wolfcastle internals. Verify with `wolfcastle config show --section task_classes`.

**Overriding a built-in class prompt:**

Place a file with the same relative path in a higher tier:

```
.wolfcastle/system/custom/prompts/classes/coding/go.md
```

Your file replaces the base version completely. Write only your team's Go conventions; the universal prompt and system mechanics are injected separately.

See [Task Classes: Creating a Custom Class](task-classes.md#creating-a-custom-class) for the full walkthrough.

### Prompt Overrides

Any base prompt can be overridden by placing a file with the same relative path in `custom/` or `local/`. The override replaces the base version entirely.

```
# Override the execute stage prompt for your team
.wolfcastle/system/custom/prompts/stages/execute.md

# Override the universal class prompt personally
.wolfcastle/system/local/prompts/classes/universal.md
```

This works for stage prompts, class prompts, the summary prompt, the doctor prompt, audit prompts, and any other file under `prompts/`.

### Rule Fragments

Rule fragments are composable text files assembled into the prompt context. Wolfcastle ships base fragments covering git conventions, ADR usage, and commit formatting. Teams add custom fragments in `custom/rules/`, and engineers add personal fragments in `local/rules/`.

Fragment ordering depends on configuration:

- **Empty array** (the default): auto-discovery in alphabetical order across all tiers.
- **Explicit array**: only listed fragments are included, in the order you specify.

```json
{
  "prompts": {
    "fragments": ["rules/git-conventions.md", "rules/adr-policy.md", "rules/team-style.md"]
  }
}
```

To exclude specific fragments without specifying the full list:

```json
{
  "prompts": {
    "exclude_fragments": ["rules/verbose-logging.md"]
  }
}
```

See [Config Reference: prompts](config-reference.md#prompts) for field details.

### Codebase Knowledge

Codebase knowledge files accumulate what agents learn about the codebase across tasks. The main configuration control is the token budget: how large the knowledge file can grow before requiring pruning.

```json
{
  "knowledge": {
    "max_tokens": 2000
  }
}
```

The budget matters because the entire knowledge file is injected into every task's context. A larger budget gives agents more accumulated wisdom to work with but consumes more of the model's context window. The default of 2000 tokens balances usefulness against context cost.

When an entry would push the file over budget, `wolfcastle knowledge add` rejects it and asks for pruning. The daemon can create a maintenance task to handle this automatically, reviewing the file, removing stale entries, and consolidating related ones. See [Config Reference: knowledge](config-reference.md#knowledge) for the field details.

### Audit Scopes

Audit behavior is configured through the `audit` section. The main controls are the model used for audit analysis and the prompt file.

```json
{
  "audit": {
    "model": "heavy",
    "prompt_file": "audits/audit.md"
  }
}
```

Override the prompt in `custom/` to change how audits evaluate code for your team's standards.

See [Config Reference: audit](config-reference.md#audit) for all audit fields.

---

## Inspecting Config

The `wolfcastle config show` command displays the fully resolved configuration, with all three tiers merged. This is the single source of truth for what Wolfcastle will actually use.

```
wolfcastle config show
```

**Filter to a section** (pass the section name as a positional argument):

```
wolfcastle config show pipeline
wolfcastle config show models
wolfcastle config show daemon
```

**View a single tier's raw content** (before merge):

```
wolfcastle config show --tier base
wolfcastle config show --tier custom
wolfcastle config show --tier local
```

**Suppress defaults** to see only what you've explicitly set:

```
wolfcastle config show --raw
```

A practical workflow: dump the current config as a starting point, then edit what you need.

```
wolfcastle config show --section pipeline > /tmp/pipeline.json
# Edit /tmp/pipeline.json
# Copy the relevant parts into system/custom/config.json
```

Or dump a single tier to see exactly what overrides exist:

```
wolfcastle config show --tier custom
```

---

## CLI Config Commands

The CLI provides write commands as an alternative to hand-editing JSON files. All write commands default to the `local` tier; pass `--tier custom` for team-shared overrides.

### config set

Set a scalar or object at a dot-delimited JSON path.

```
wolfcastle config set daemon.stall_timeout_seconds 300
wolfcastle config set pipeline.stages.execute.model local-llama
wolfcastle config set --tier custom daemon.stall_timeout_seconds 300
```

### config unset

Remove a key from the specified tier.

```
wolfcastle config unset models.fast
wolfcastle config unset --tier custom models.fast
```

### config append

Append a value to an array field.

```
wolfcastle config append pipeline.stage_order spec-review
wolfcastle config append prompts.exclude_fragments rules/verbose-logging.md
```

### config remove

Remove a value from an array field.

```
wolfcastle config remove pipeline.stage_order intake
wolfcastle config remove prompts.exclude_fragments rules/verbose-logging.md
```

Paths are dot-delimited and correspond to the JSON structure: `pipeline.stages.execute.model`, `failure.hard_cap`, `logs.max_files`. The three-tier merge still applies after every mutation: a `config set` in local overrides both base and custom, and a `config set --tier custom` writes to the team-shared layer.

For the full command syntax, see the CLI reference pages for [config show](cli/config-show.md), [config set](cli/config-set.md), [config unset](cli/config-unset.md), [config append](cli/config-append.md), and [config remove](cli/config-remove.md).

---

## Security

Wolfcastle does not sandbox model invocations. Security is configured at the model level through the CLI flags you pass in the `models` dictionary.

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

The executing model's capabilities are determined entirely by the flags you give it. If you pass `--dangerously-skip-permissions`, the model can do whatever it wants. If you don't, the model's own permission system applies.

**Team enforcement:** put your model definitions in `custom/config.json`. Because it's committed, changes go through code review. The team agrees on what permissions models get, and that agreement lives in version control alongside the code.

**Personal loosening:** if you need broader permissions for your own workflow, override in `local/config.json`. It's gitignored, so your looser settings never reach the team's config. The risk is yours.

This separation, committed team policy in `custom/` and personal exceptions in gitignored `local/`, is the core security model. There's no runtime permission layer beyond what the underlying model tool provides.
