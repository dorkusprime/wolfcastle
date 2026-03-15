# Wolfcastle Configuration Schema

## Overview

Wolfcastle uses three JSON configuration files following the three-tier pattern (ADR-063):

| File | Purpose | Git status | Written by |
|------|---------|------------|------------|
| `base/config.json` | Compiled defaults | Gitignored | `wolfcastle init` / `wolfcastle update` |
| `custom/config.json` | Team-shared overrides | Committed | User / team |
| `local/config.json` | Personal overrides and identity | Gitignored | `wolfcastle init` + user |

Wolfcastle regenerates `base/config.json` on every init or update. Wolfcastle reads but never writes to `custom/config.json`. Wolfcastle writes `local/config.json` only during `wolfcastle init` (to populate identity); the user may edit it freely afterward.

### Merge Semantics

Per ADR-018 and ADR-063, the three files are merged via **recursive deep merge** in order: `base/config.json` <- `custom/config.json` <- `local/config.json`. Keys in higher tiers override the same keys in lower tiers at the deepest level. Unspecified keys inherit from the tier below.

Example: if `base/config.json` defines models `fast` and `heavy`, and `local/config.json` redefines only `heavy`, the resolved config contains both models, with `fast` from base and `heavy` from local.

Arrays are **not** deep-merged. An array in a higher tier replaces the entire array from the tier below. This applies to `pipeline.stages`, model `args`, `prompts.fragments`, and any other array-valued field.

---

## Field Eligibility

| Field | `base/config.json` | `custom/config.json` | `local/config.json` | Notes |
|-------|:---:|:---:|:---:|-------|
| `models` | Yes | Yes | Yes | Local overrides swap model tiers for personal use |
| `pipeline` | Yes | Yes | Yes | Local can redefine stages (e.g. skip summary) |
| `logs` | Yes | Yes | Yes | |
| `retries` | Yes | Yes | Yes | |
| `failure` | Yes | Yes | Yes | |
| `identity` | No | No | **Yes** | Auto-populated by `wolfcastle init`. Never shared. |
| `summary` | Yes | Yes | Yes | |
| `docs` | Yes | Yes | Yes | |
| `validation` | Yes | Yes | Yes | |
| `prompts` | Yes | Yes | Yes | |
| `daemon` | Yes | Yes | Yes | |
| `git` | Yes | Yes | Yes | |
| `doctor` | Yes | Yes | Yes | |
| `unblock` | Yes | Yes | Yes | |
| `overlap_advisory` | Yes | Yes | Yes | Enabled by default in team config; can be disabled in local config |
| `audit` | Yes | Yes | Yes | Codebase audit command configuration (ADR-029) |

`identity` is the only field restricted to `local/config.json`. All other fields may appear in any tier.

---

## Full JSON Schema

```json
{
  "$schema": "https://json-schema.org/draft/2020-12/schema",
  "$id": "https://wolfcastle.dev/schemas/config.json",
  "title": "Wolfcastle Configuration",
  "description": "Schema for base/config.json, custom/config.json, and local/config.json",
  "type": "object",
  "additionalProperties": false,
  "properties": {

    "models": {
      "type": "object",
      "description": "Named model definitions. Keys are reference names used by pipeline stages. (ADR-013)",
      "additionalProperties": {
        "type": "object",
        "required": ["command", "args"],
        "additionalProperties": false,
        "properties": {
          "command": {
            "type": "string",
            "description": "CLI executable to invoke (e.g. \"claude\", \"openai\", \"ollama\")."
          },
          "args": {
            "type": "array",
            "items": { "type": "string" },
            "description": "Arguments passed to the command. Includes model name, output format, permission flags, and any other CLI-specific settings. Security posture is controlled here (ADR-022)."
          }
        }
      },
      "default": {
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
    },

    "pipeline": {
      "type": "object",
      "description": "Pipeline stage configuration. Stages execute in order each daemon iteration. (ADR-006, ADR-013)",
      "additionalProperties": false,
      "properties": {
        "stages": {
          "type": "array",
          "description": "Ordered list of pipeline stages. Each stage runs one model invocation.",
          "items": {
            "type": "object",
            "required": ["name", "model", "prompt_file"],
            "additionalProperties": false,
            "properties": {
              "name": {
                "type": "string",
                "description": "Human-readable stage identifier. Must be unique within the pipeline."
              },
              "model": {
                "type": "string",
                "description": "Key referencing an entry in the top-level `models` dictionary."
              },
              "prompt_file": {
                "type": "string",
                "description": "Filename of the prompt template, resolved through the three-tier merge (base/ -> custom/ -> local/) per ADR-009 and ADR-018."
              },
              "enabled": {
                "type": "boolean",
                "default": true,
                "description": "When false, the stage is skipped entirely during pipeline execution. Allows opt-out without removing the stage from config."
              },
              "skip_prompt_assembly": {
                "type": "boolean",
                "default": false,
                "description": "When true, the stage receives only its own prompt_file content as the prompt, without the full system prompt assembly (no rule fragments, no script reference). Useful for lightweight stages like summary."
              }
            }
          },
          "default": [
            { "name": "expand", "model": "fast", "prompt_file": "expand.md" },
            { "name": "file", "model": "mid", "prompt_file": "file.md" },
            { "name": "execute", "model": "heavy", "prompt_file": "execute.md" }
          ]
        }
      }
    },

    "logs": {
      "type": "object",
      "description": "NDJSON log retention settings. Logs are per-iteration files in .wolfcastle/logs/. (ADR-012)",
      "additionalProperties": false,
      "properties": {
        "max_files": {
          "type": "integer",
          "minimum": 1,
          "default": 100,
          "description": "Maximum number of log files to retain. Oldest files are deleted first."
        },
        "max_age_days": {
          "type": "integer",
          "minimum": 1,
          "default": 30,
          "description": "Maximum age in days. Log files older than this are deleted regardless of count."
        },
        "compress": {
          "type": "boolean",
          "default": true,
          "description": "Whether to gzip-compress log files that are no longer the active iteration."
        }
      },
      "default": {
        "max_files": 100,
        "max_age_days": 30,
        "compress": true
      }
    },

    "retries": {
      "type": "object",
      "description": "Model invocation retry settings for transient failures (API errors, crashes, empty output). Uses exponential backoff. (ADR-019)",
      "additionalProperties": false,
      "properties": {
        "initial_delay_seconds": {
          "type": "integer",
          "minimum": 1,
          "default": 30,
          "description": "Delay in seconds before the first retry."
        },
        "max_delay_seconds": {
          "type": "integer",
          "minimum": 1,
          "default": 600,
          "description": "Maximum delay between retries (backoff cap)."
        },
        "max_retries": {
          "type": "integer",
          "minimum": -1,
          "default": -1,
          "description": "Maximum number of retries. -1 means unlimited (wait patiently for API recovery)."
        }
      },
      "default": {
        "initial_delay_seconds": 30,
        "max_delay_seconds": 600,
        "max_retries": -1
      }
    },

    "failure": {
      "type": "object",
      "description": "Task-level failure escalation thresholds. Controls when the model is prompted to decompose and when tasks are force-blocked. (ADR-019)",
      "additionalProperties": false,
      "properties": {
        "decomposition_threshold": {
          "type": "integer",
          "minimum": 1,
          "default": 10,
          "description": "Number of consecutive task failures before the model is prompted to decompose the task into smaller pieces."
        },
        "max_decomposition_depth": {
          "type": "integer",
          "minimum": 0,
          "default": 5,
          "description": "Maximum decomposition nesting depth. At max depth, hitting the threshold results in auto-block instead of decomposition."
        },
        "hard_cap": {
          "type": "integer",
          "minimum": 1,
          "default": 50,
          "description": "Absolute maximum failures per task regardless of depth. Safety net against unbounded iteration."
        }
      },
      "default": {
        "decomposition_threshold": 10,
        "max_decomposition_depth": 5,
        "hard_cap": 50
      }
    },

    "identity": {
      "type": "object",
      "description": "Engineer identity. Auto-populated by `wolfcastle init` from whoami and hostname. Used to namespace the project directory (e.g. projects/wild-macbook/). local/config.json ONLY. (ADR-009)",
      "additionalProperties": false,
      "required": ["user", "machine"],
      "properties": {
        "user": {
          "type": "string",
          "description": "Engineer username. Defaults to output of `whoami` at init time."
        },
        "machine": {
          "type": "string",
          "description": "Machine identifier. Defaults to output of `hostname` (short form) at init time."
        }
      }
    },

    "summary": {
      "type": "object",
      "description": "Controls the archive summary model stage that runs after audit completion. (ADR-016)",
      "additionalProperties": false,
      "properties": {
        "enabled": {
          "type": "boolean",
          "default": true,
          "description": "Whether to run a model stage to generate a plain-language summary of completed nodes before archiving. Set to false to skip the summary and save tokens."
        },
        "model": {
          "type": "string",
          "default": "fast",
          "description": "Key referencing an entry in the `models` dictionary. Summary is typically a cheap model since it only writes prose."
        },
        "prompt_file": {
          "type": "string",
          "default": "summary.md",
          "description": "Prompt template for the summary stage, resolved through three-tier merge."
        }
      },
      "default": {
        "enabled": true,
        "model": "fast",
        "prompt_file": "summary.md"
      }
    },

    "docs": {
      "type": "object",
      "description": "Documentation management settings. (ADR-010)",
      "additionalProperties": false,
      "properties": {
        "directory": {
          "type": "string",
          "default": "docs",
          "description": "Path to the documentation directory, relative to .wolfcastle/. Override to point at an existing project docs directory (e.g. \"../docs\" to use the repo root's docs/ folder)."
        }
      },
      "default": {
        "directory": "docs"
      }
    },

    "validation": {
      "type": "object",
      "description": "Commands that Wolfcastle runs to validate task completion. These are executed after the model reports a task as complete, before marking it done in state.",
      "additionalProperties": false,
      "properties": {
        "commands": {
          "type": "array",
          "description": "Ordered list of shell commands to run for validation. Each command must exit 0 to pass. If any command fails, the task is not marked complete and the model is informed of the failure.",
          "items": {
            "type": "object",
            "required": ["name", "run"],
            "additionalProperties": false,
            "properties": {
              "name": {
                "type": "string",
                "description": "Human-readable label for this validation step (shown in logs and model output)."
              },
              "run": {
                "type": "string",
                "description": "Shell command to execute. Runs in the repository root."
              },
              "timeout_seconds": {
                "type": "integer",
                "minimum": 1,
                "default": 300,
                "description": "Maximum time in seconds before the command is killed."
              }
            }
          },
          "default": []
        }
      },
      "default": {
        "commands": []
      }
    },

    "prompts": {
      "type": "object",
      "description": "Prompt assembly configuration. Controls how rule fragments are ordered and merged into the system prompt. (ADR-005, ADR-017)",
      "additionalProperties": false,
      "properties": {
        "fragments": {
          "type": "array",
          "description": "Ordered list of rule fragment filenames to include in prompt assembly. Fragments are resolved through the three-tier merge (base/ -> custom/ -> local/). Order determines injection order in the system prompt. If omitted, all fragments found across all tiers are included in alphabetical order.",
          "items": { "type": "string" },
          "default": []
        },
        "exclude_fragments": {
          "type": "array",
          "description": "Fragment filenames to exclude from prompt assembly. Useful when the default set includes a fragment you do not want without having to enumerate all others in `fragments`.",
          "items": { "type": "string" },
          "default": []
        }
      },
      "default": {
        "fragments": [],
        "exclude_fragments": []
      }
    },

    "daemon": {
      "type": "object",
      "description": "Daemon loop behavior settings. (ADR-020)",
      "additionalProperties": false,
      "properties": {
        "poll_interval_seconds": {
          "type": "integer",
          "minimum": 1,
          "default": 5,
          "description": "Seconds to wait between iterations when there is work available."
        },
        "blocked_poll_interval_seconds": {
          "type": "integer",
          "minimum": 1,
          "default": 60,
          "description": "Seconds to wait between checks when all tasks are blocked or there is no work. Longer interval to avoid busy-waiting."
        },
        "max_iterations": {
          "type": "integer",
          "minimum": -1,
          "default": -1,
          "description": "Maximum number of daemon iterations before auto-stop. -1 means unlimited (run until stopped or all work is done)."
        },
        "max_turns_per_invocation": {
          "type": "integer",
          "minimum": 1,
          "default": 200,
          "description": "Maximum number of conversational turns per model invocation within a single iteration. Prevents runaway model sessions."
        },
        "invocation_timeout_seconds": {
          "type": "integer",
          "minimum": 60,
          "default": 3600,
          "description": "Maximum wall-clock time in seconds for a single model invocation. If exceeded, the child process is killed and the invocation is treated as an invocation failure (retried with backoff per the retries config). Default is 3600 (1 hour)."
        },
        "max_restarts": {
          "type": "integer",
          "minimum": 0,
          "default": 3,
          "description": "Maximum number of automatic restarts the supervisor will attempt when the daemon crashes. Set to 0 to disable automatic restarts."
        },
        "restart_delay_seconds": {
          "type": "integer",
          "minimum": 0,
          "default": 2,
          "description": "Delay in seconds between automatic restart attempts."
        }
      },
      "default": {
        "poll_interval_seconds": 5,
        "blocked_poll_interval_seconds": 60,
        "max_iterations": -1,
        "max_turns_per_invocation": 200,
        "invocation_timeout_seconds": 3600,
        "max_restarts": 3,
        "restart_delay_seconds": 2
      }
    },

    "doctor": {
      "type": "object",
      "description": "Configuration for `wolfcastle doctor`, the structural validation and repair command. The model is only invoked for ambiguous fixes; deterministic fixes are handled directly by Go code. (ADR-025)",
      "additionalProperties": false,
      "properties": {
        "model": {
          "type": "string",
          "default": "mid",
          "description": "Key referencing an entry in the `models` dictionary. Used when the doctor encounters ambiguous issues that require model reasoning to resolve."
        },
        "prompt_file": {
          "type": "string",
          "default": "doctor.md",
          "description": "Prompt template for the doctor's model-assisted fixes, resolved through three-tier merge."
        }
      },
      "default": {
        "model": "mid",
        "prompt_file": "doctor.md"
      }
    },

    "overlap_advisory": {
      "type": "object",
      "description": "Cross-engineer overlap detection at project creation time. Uses bigram Jaccard similarity to compare a new project's scope against other engineers' active projects and prints an advisory. Algorithmic — no model invocation required. Read-only and informational only. (ADR-027, ADR-041)",
      "additionalProperties": false,
      "properties": {
        "enabled": {
          "type": "boolean",
          "default": true,
          "description": "Whether to run the overlap check when creating a new project. Enabled by default in team config; can be disabled in local/config.json for solo engineers."
        },
        "model": {
          "type": "string",
          "default": "fast",
          "description": "Retained for potential future hybrid detection. Not currently used or validated (ADR-041)."
        },
        "threshold": {
          "type": "number",
          "default": 0.3,
          "description": "Jaccard similarity threshold (0–1) above which projects are flagged as overlapping. Lower values are more sensitive. (ADR-041)"
        }
      },
      "default": {
        "enabled": true,
        "model": "fast",
        "threshold": 0.3
      }
    },

    "unblock": {
      "type": "object",
      "description": "Configuration for `wolfcastle unblock`, the interactive model-assisted unblocking command. The model is invoked only in interactive mode (not agent mode). (ADR-028)",
      "additionalProperties": false,
      "properties": {
        "model": {
          "type": "string",
          "default": "heavy",
          "description": "Key referencing an entry in the `models` dictionary. Unblock uses a capable model since it handles genuinely hard problems that the autonomous loop could not resolve."
        },
        "prompt_file": {
          "type": "string",
          "default": "unblock.md",
          "description": "Prompt template for the unblock session, resolved through three-tier merge."
        }
      },
      "default": {
        "model": "heavy",
        "prompt_file": "unblock.md"
      }
    },

    "audit": {
      "type": "object",
      "description": "Configuration for `wolfcastle audit`, the codebase audit command with discoverable scopes. (ADR-029)",
      "additionalProperties": false,
      "properties": {
        "model": {
          "type": "string",
          "default": "heavy",
          "description": "Key referencing an entry in the `models` dictionary. Codebase audits use a capable model for thorough analysis."
        },
        "prompt_file": {
          "type": "string",
          "default": "audit.md",
          "description": "Prompt template for the audit command, resolved through three-tier merge."
        }
      },
      "default": {
        "model": "heavy",
        "prompt_file": "audit.md"
      }
    },

    "git": {
      "type": "object",
      "description": "Git behavior configuration. (ADR-015)",
      "additionalProperties": false,
      "properties": {
        "auto_commit": {
          "type": "boolean",
          "default": true,
          "description": "Whether the daemon automatically commits state and code changes after each task completion."
        },
        "commit_message_format": {
          "type": "string",
          "default": "wolfcastle: {action} [{node}]",
          "description": "Template for commit messages. Placeholders: {action} (e.g. \"complete task\", \"add breadcrumb\"), {node} (tree address of the affected node), {user} (identity.user), {machine} (identity.machine)."
        },
        "verify_branch": {
          "type": "boolean",
          "default": true,
          "description": "Whether to verify the current branch matches the startup branch before each commit. If false, Wolfcastle will not check for branch switches. (ADR-015)"
        }
      },
      "default": {
        "auto_commit": true,
        "commit_message_format": "wolfcastle: {action} [{node}]",
        "verify_branch": true
      }
    }
  }
}
```

---

## Defaults Summary

All fields are optional. Omitted fields use the defaults specified above. A completely empty `config.json` (`{}`) is valid and produces a fully functional configuration using all defaults.

| Key | Default |
|-----|---------|
| `models.fast` | `claude -p --model claude-haiku-4-5-20251001 --output-format stream-json --dangerously-skip-permissions` |
| `models.mid` | `claude -p --model claude-sonnet-4-6 --output-format stream-json --dangerously-skip-permissions` |
| `models.heavy` | `claude -p --model claude-opus-4-6 --output-format stream-json --verbose --dangerously-skip-permissions` |
| `pipeline.stages` | expand (fast) -> file (mid) -> execute (heavy) |
| `logs.max_files` | `100` |
| `logs.max_age_days` | `30` |
| `logs.compress` | `true` |
| `retries.initial_delay_seconds` | `30` |
| `retries.max_delay_seconds` | `600` |
| `retries.max_retries` | `-1` (unlimited) |
| `failure.decomposition_threshold` | `10` |
| `failure.max_decomposition_depth` | `5` |
| `failure.hard_cap` | `50` |
| `summary.enabled` | `true` |
| `summary.model` | `"fast"` |
| `summary.prompt_file` | `"summary.md"` |
| `docs.directory` | `"docs"` |
| `validation.commands` | `[]` (none) |
| `prompts.fragments` | `[]` (auto-discover, alphabetical) |
| `prompts.exclude_fragments` | `[]` |
| `daemon.poll_interval_seconds` | `5` |
| `daemon.blocked_poll_interval_seconds` | `60` |
| `daemon.max_iterations` | `-1` (unlimited) |
| `daemon.max_turns_per_invocation` | `200` |
| `daemon.invocation_timeout_seconds` | `3600` (1 hour) |
| `daemon.max_restarts` | `3` |
| `daemon.restart_delay_seconds` | `2` |
| `git.auto_commit` | `true` |
| `git.commit_message_format` | `"wolfcastle: {action} [{node}]"` |
| `git.verify_branch` | `true` |
| `doctor.model` | `"mid"` |
| `doctor.prompt_file` | `"doctor.md"` |
| `unblock.model` | `"heavy"` |
| `unblock.prompt_file` | `"unblock.md"` |
| `overlap_advisory.enabled` | `true` |
| `overlap_advisory.model` | `"fast"` |
| `overlap_advisory.threshold` | `0.3` |
| `audit.model` | `"heavy"` |
| `audit.prompt_file` | `"audit.md"` |

---

## Example: `custom/config.json`

This is the team-shared configuration committed to git. Shows all fields with their default values for reference. In practice, teams only need to include fields where they diverge from defaults (since `base/config.json` already contains compiled defaults).

```json
{
  "models": {
    "fast": {
      "command": "claude",
      "args": [
        "-p",
        "--model", "claude-haiku-4-5-20251001",
        "--output-format", "stream-json",
        "--dangerously-skip-permissions"
      ]
    },
    "mid": {
      "command": "claude",
      "args": [
        "-p",
        "--model", "claude-sonnet-4-6",
        "--output-format", "stream-json",
        "--dangerously-skip-permissions"
      ]
    },
    "heavy": {
      "command": "claude",
      "args": [
        "-p",
        "--model", "claude-opus-4-6",
        "--output-format", "stream-json",
        "--verbose",
        "--dangerously-skip-permissions"
      ]
    }
  },

  "pipeline": {
    "stages": [
      { "name": "expand", "model": "fast", "prompt_file": "expand.md" },
      { "name": "file", "model": "mid", "prompt_file": "file.md" },
      { "name": "execute", "model": "heavy", "prompt_file": "execute.md" }
    ]
  },

  "logs": {
    "max_files": 100,
    "max_age_days": 30,
    "compress": true
  },

  "retries": {
    "initial_delay_seconds": 30,
    "max_delay_seconds": 600,
    "max_retries": -1
  },

  "failure": {
    "decomposition_threshold": 10,
    "max_decomposition_depth": 5,
    "hard_cap": 50
  },

  "summary": {
    "enabled": true,
    "model": "fast",
    "prompt_file": "summary.md"
  },

  "docs": {
    "directory": "docs"
  },

  "validation": {
    "commands": [
      {
        "name": "tests",
        "run": "go test ./...",
        "timeout_seconds": 300
      },
      {
        "name": "lint",
        "run": "golangci-lint run",
        "timeout_seconds": 120
      }
    ]
  },

  "prompts": {
    "fragments": [],
    "exclude_fragments": []
  },

  "daemon": {
    "poll_interval_seconds": 5,
    "blocked_poll_interval_seconds": 60,
    "max_iterations": -1,
    "max_turns_per_invocation": 200,
    "invocation_timeout_seconds": 3600
  },

  "doctor": {
    "model": "mid",
    "prompt_file": "doctor.md"
  },

  "unblock": {
    "model": "heavy",
    "prompt_file": "unblock.md"
  },

  "overlap_advisory": {
    "enabled": true,
    "model": "fast",
    "threshold": 0.3
  },

  "audit": {
    "model": "heavy",
    "prompt_file": "audit.md"
  },

  "git": {
    "auto_commit": true,
    "commit_message_format": "wolfcastle: {action} [{node}]",
    "verify_branch": true
  }
}
```

---

## Example: `local/config.json`

This is a personal overrides file, gitignored. Shows identity (always present after init) and a model override swapping the heavy tier to a cheaper model for local development.

```json
{
  "identity": {
    "user": "wild",
    "machine": "macbook"
  },

  "models": {
    "heavy": {
      "command": "claude",
      "args": [
        "-p",
        "--model", "claude-sonnet-4-6",
        "--output-format", "stream-json",
        "--dangerously-skip-permissions"
      ]
    }
  },

  "daemon": {
    "max_iterations": 10,
    "max_turns_per_invocation": 50
  }
}
```

After deep merge, the resolved configuration has:
- `identity.user` = `"wild"`, `identity.machine` = `"macbook"` (from local)
- `models.fast` = Claude Haiku (from base/config.json, unchanged)
- `models.mid` = Claude Sonnet (from base/config.json, unchanged)
- `models.heavy` = Claude Sonnet (from local/config.json, overridden)
- `daemon.poll_interval_seconds` = `5` (from base/config.json default)
- `daemon.max_iterations` = `10` (from local/config.json, overridden)
- `daemon.max_turns_per_invocation` = `50` (from local/config.json, overridden)
- All other fields = defaults from base/config.json

---

## Merge Semantics Reference

Per ADR-018, config merging follows these rules:

### Objects: Recursive Deep Merge

For every key at every nesting level, tiers are merged in order (base, then custom, then local):
1. If the key exists only in a lower tier, use that value.
2. If the key exists only in a higher tier, use that value.
3. If the key exists in multiple tiers and all values are objects, recurse (deep merge the child objects).
4. If the key exists in multiple tiers and any value is not an object, the highest tier wins (full replacement).

### Arrays: Full Replacement

Arrays are never element-merged. If a higher tier provides an array value for any key, it completely replaces the array from lower tiers. This means:
- Overriding `pipeline.stages` replaces the entire stage list.
- Overriding a model's `args` replaces the entire args array.
- Overriding `prompts.fragments` replaces the entire fragment list.

### Null Deletion

A field set to `null` in a higher tier removes that key from the resolved config. This allows local or custom config to explicitly remove a setting from a lower tier. For example, setting `"validation": null` in `local/config.json` disables all validation commands locally.

### Resolution Order

```
defaults (hardcoded in Go) <- base/config.json <- custom/config.json <- local/config.json
```

The Go binary carries hardcoded defaults for every field. `base/config.json` contains compiled defaults (identical to the hardcoded ones, provided for visibility). `custom/config.json` overrides those for the team. `local/config.json` overrides the result for the individual engineer. An empty `custom/config.json` (`{}`) and absent `local/config.json` produce a fully functional configuration.

---

## Validation Rules

The following validation is performed when config is loaded:

1. **Model references**: Every `model` value in `pipeline.stages`, `summary.model`, `doctor.model`, `unblock.model`, and `audit.model` must reference a key that exists in the resolved `models` dictionary (after merge). Note: `overlap_advisory.model` is not validated — overlap detection is algorithmic per ADR-041.
2. **Stage name uniqueness**: No two entries in `pipeline.stages` may share the same `name`.
3. **No identity in custom/config.json**: If `identity` appears in `custom/config.json`, emit a warning. Identity is personal and should only be in `local/config.json`.
4. **Type checking**: All fields must match their declared types. Unknown keys at the top level are rejected (`additionalProperties: false`).
5. **Constraint checking**: Numeric fields respect their `minimum` constraints. `hard_cap` must be >= `decomposition_threshold`.
6. **Prompt file existence**: `prompt_file` values in `pipeline.stages`, `summary.prompt_file`, `doctor.prompt_file`, `unblock.prompt_file`, and `audit.prompt_file` should resolve to an existing file in at least one tier (base/, custom/, or local/). Missing prompt files produce a startup error.

---

## ADR Traceability

| Config section | Source ADR(s) |
|----------------|---------------|
| `models` | ADR-004, ADR-013, ADR-022 |
| `pipeline` | ADR-006, ADR-013 |
| `logs` | ADR-012 |
| `retries` | ADR-019 |
| `failure` | ADR-019 |
| `identity` | ADR-009 |
| `summary` | ADR-016 |
| `docs` | ADR-010 |
| `validation` | ADR-002, ADR-007 |
| `prompts` | ADR-005, ADR-017 |
| `daemon` | ADR-020 |
| `git` | ADR-015 |
| `doctor` | ADR-025 |
| `unblock` | ADR-028 |
| `overlap_advisory` | ADR-027, ADR-041 |
| `audit` | ADR-029 |
| Merge semantics | ADR-018 |
| Three-tier layering | ADR-009 |
