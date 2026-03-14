# CLI Reference

21+ commands. Every one accepts `--json` for structured output. Every one that operates on a node accepts `--node` with a slash-separated [tree address](#tree-addressing). Every one has `-h` help with dynamic content (available [scopes](audits.md#scopes), install targets, [spec](collaboration.md#specs) lists are discovered at runtime).

## Commands

| Category | Commands |
|----------|----------|
| **Lifecycle** | `init`, `start`, `stop`, `status`, `follow`, `update` |
| **Task** | `task add`, `task claim`, `task complete`, `task block`, `task unblock` |
| **Project** | `project create`, `project list` |
| **Audit** | `audit` (codebase), `audit breadcrumb`, `audit escalate` |
| **Navigation** | `navigate` |
| **Diagnostics** | `doctor`, `unblock` |
| **Documentation** | `adr create`, `spec create`, `spec link`, `spec list` |
| **Archive** | `archive add` |
| **Inbox** | `inbox add`, `inbox list` |
| **Integration** | `install` |

## Tree Addressing

Every node is addressable by its path from the root:

```
wolfcastle task add --node backend/auth/session-tokens "Implement token rotation"
wolfcastle start --node backend
wolfcastle status --node frontend/login-flow
```

Scripts validate that the target node exists and is the correct type. You cannot add a task to an [orchestrator](how-it-works.md#the-project-tree). You cannot create a child under a leaf. The tree has rules.

## The Inbox

For ideas that arrive while work is underway:

```
wolfcastle inbox add "Support OAuth2 PKCE flow"
wolfcastle inbox list
```

Items land in the inbox. The [expand pipeline stage](how-it-works.md#the-pipeline) picks them up, decomposes them into tasks, and the file stage organizes them into the [tree](how-it-works.md#the-project-tree).

## Installation

Three distribution channels:

- **`curl` installer**: Zero dependencies. Download and run.
- **Homebrew tap**: `brew install wolfcastle`
- **npm wrapper**: Optional, for teams already in that ecosystem.
- **Self-update**: `wolfcastle update` refreshes the binary and regenerates `base/`.

### Claude Code Integration

```
wolfcastle install skill
```

Installs the Wolfcastle skill for Claude Code. Uses symlinks where supported (auto-updates with `wolfcastle update`) and falls back to file copy on platforms that don't.

## Project Layout

`wolfcastle init` creates the `.wolfcastle/` directory. [Configuration](configuration.md) merges across the `base/`, `custom/`, and `local/` tiers:

```
.wolfcastle/
  .gitignore
  config.json              <- team-shared config (committed)
  config.local.json        <- personal config, identity (gitignored)
  base/                    <- Wolfcastle defaults, prompts, scripts (gitignored)
  custom/                  <- team overrides and additions (committed)
  local/                   <- personal overrides (gitignored)
  projects/                <- live work trees, per engineer (committed)
    wild-macbook/
    dave-workstation/
  archive/                 <- completed work summaries (committed)
  docs/                    <- ADRs and specs (committed)
    decisions/
    specs/
  logs/                    <- NDJSON iteration logs (gitignored)
  worktrees/               <- git worktrees when using --worktree (gitignored)
```

**Committed**: `config.json`, `custom/`, `projects/`, `archive/`, `docs/`

**Gitignored**: `base/`, `local/`, `config.local.json`, `logs/`, `worktrees/`

### New Engineer Setup

1. Clone the repo. You get `config.json`, `custom/`, `archive/`, and `docs/` immediately.
2. Install Wolfcastle. `brew install wolfcastle` or the curl installer.
3. `wolfcastle init`. Creates `config.local.json` with your [identity](configuration.md#identity). Generates `base/`.
4. `wolfcastle start`. [The daemon](how-it-works.md#the-daemon) wakes up. Your [namespace](collaboration.md#engineer-namespacing) is created. Work begins.
