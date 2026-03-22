# wolfcastle init

Scaffolds the `.wolfcastle/` directory and sets your identity. This is the first command you run in any repo. Nothing else works until this one has.

## What It Does

Creates the full `.wolfcastle/` directory structure: `base/`, `custom/`, `local/`, `projects/`, `archive/`, `docs/`, `logs/`. Writes the `.gitignore` that separates committed files from local-only files. Generates `base/config.json` with compiled defaults for [models](../configuration.md#model-configuration), [pipelines](../configuration.md#pipeline-stages), and [failure thresholds](../failure-and-recovery.md#task-failure-escalation). Creates an empty `custom/config.json` for team overrides.

Auto-detects your identity from `whoami` and `hostname` (stripping `.local` suffix), writes it to `local/config.json`, and creates your [namespace](../collaboration.md#engineer-namespacing) directory at `projects/{user}-{machine}/` with a root `state.json` index.

Generates `base/` contents from the installed binary: prompts, rules, and the script reference that gets injected into model context.

## Flags

| Flag | Description |
|------|-------------|
| `--force` | Re-scaffolds `base/` and refreshes identity without overwriting `custom/config.json`, `local/`, or state files. Migrates old-style root `config.json` and `config.local.json` automatically. Safe to run repeatedly. |
| `--json` | Output as structured JSON. |

## Exit Codes

| Code | Meaning |
|------|---------|
| 0 | Success. |
| 1 | Directory not writable, malformed config, or other setup failure. |

## Consequences

- Creates the `.wolfcastle/` directory tree and all scaffolding files.
- Writes `local/config.json` with your detected identity (gitignored).
- Creates your engineer namespace under `projects/`.
- With `--force`: regenerates `base/` and updates identity fields only. Migrates old-style root `config.json` to `custom/config.json` and `config.local.json` to `local/config.json`. Does not touch `custom/`, `local/`, or existing state.

## See Also

- [Configuration](../configuration.md) for the three-tier config system this sets up.
- [Project Layout](../cli.md#project-layout) for the full directory structure.
- [`wolfcastle update`](update.md) to regenerate `base/` after a binary update.
