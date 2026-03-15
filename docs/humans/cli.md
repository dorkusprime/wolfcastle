# CLI Reference

Every command accepts `--json` for structured output. Every command that operates on a node accepts `--node` with a slash-separated [tree address](#tree-addressing). Every command has `-h` help with dynamic content (available [scopes](audits.md#scopes), install targets, [spec](collaboration.md#specs) lists are discovered at runtime).

## Commands

| Category          | Commands                                                                                                                                                                                                                                                                                |
| ----------------- | --------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------- |
| **Lifecycle**     | [`init`](cli/init.md), [`start`](cli/start.md), [`stop`](cli/stop.md), [`status`](cli/status.md), [`follow`](cli/follow.md), [`update`](cli/update.md), [`version`](cli/version.md)                                                                                                     |
| **Task**          | [`task add`](cli/task-add.md), [`task claim`](cli/task-claim.md), [`task complete`](cli/task-complete.md), [`task block`](cli/task-block.md), [`task unblock`](cli/task-unblock.md)                                                                                                     |
| **Project**       | [`project create`](cli/project-create.md)                                                                                                                                                                                                                                               |
| **Audit**         | [`audit run`](cli/audit-run.md), [`audit list`](cli/audit-list.md), [`audit show`](cli/audit-show.md), [`audit scope`](cli/audit-scope.md), [`audit breadcrumb`](cli/audit-breadcrumb.md), [`audit gap`](cli/audit-gap.md), [`audit fix-gap`](cli/audit-fix-gap.md), [`audit escalate`](cli/audit-escalate.md), [`audit resolve`](cli/audit-resolve.md), [`audit pending`](cli/audit-pending.md), [`audit approve`](cli/audit-approve.md), [`audit reject`](cli/audit-reject.md), [`audit history`](cli/audit-history.md) |
| **Navigation**    | [`navigate`](cli/navigate.md)                                                                                                                                                                                                                                                           |
| **Diagnostics**   | [`doctor`](cli/doctor.md), [`unblock`](cli/unblock.md)                                                                                                                                                                                                                                  |
| **Documentation** | [`adr create`](cli/adr-create.md), [`spec create`](cli/spec-create.md), [`spec link`](cli/spec-link.md), [`spec list`](cli/spec-list.md)                                                                                                                                                |
| **Archive**       | [`archive add`](cli/archive-add.md)                                                                                                                                                                                                                                                     |
| **Inbox**         | [`inbox add`](cli/inbox-add.md), [`inbox list`](cli/inbox-list.md), [`inbox clear`](cli/inbox-clear.md)                                                                                                                                                                                 |
| **Integration**   | [`install skill`](cli/install.md)                                                                                                                                                                                                                                                       |

Each command has its own page with flags, exit codes, consequences, and cross-references. See [cli/](cli/) for the full set.

## Tree Addressing

Every node is addressable by its path from the root:

```
wolfcastle task add --node backend/auth/session-tokens "Implement token rotation"
wolfcastle start --node backend
wolfcastle status --node frontend/login-flow
```

Scripts validate that the target node exists and is the correct type. You cannot add a task to an [orchestrator](how-it-works.md#the-project-tree). You cannot create a child under a leaf. The tree has rules.

## The Inbox

The inbox is the fastest way to get an idea into Wolfcastle. [`wolfcastle inbox add`](cli/inbox-add.md) drops a text item into a queue. The daemon's [expand stage](how-it-works.md#the-pipeline) picks it up, uses a model to decompose it into tasks, and the file stage organizes those tasks into the tree. You can also [`inbox list`](cli/inbox-list.md) to see what's pending and [`inbox clear`](cli/inbox-clear.md) to wipe the queue.

## Installation

Three distribution channels:

- **`curl` installer**: Zero dependencies. Download and run.
- **Homebrew tap**: `brew install wolfcastle`
- **npm wrapper**: Optional, for teams already in that ecosystem.
- **Self-update**: [`wolfcastle update`](cli/update.md) refreshes the binary and regenerates `base/`.

### Claude Code Integration

```
wolfcastle install skill
```

Installs the Wolfcastle skill for Claude Code. Uses symlinks where supported (auto-updates with [`wolfcastle update`](cli/update.md)) and falls back to file copy on platforms that don't. See [`install skill`](cli/install.md) for details.

## Project Layout

[`wolfcastle init`](cli/init.md) creates the `.wolfcastle/` directory. [Configuration](configuration.md) merges across the `base/`, `custom/`, and `local/` tiers:

```
.wolfcastle/
  .gitignore
  base/                    <- Wolfcastle defaults, prompts, scripts (gitignored)
    config.json            <- compiled defaults (gitignored)
  custom/                  <- team overrides and additions (committed)
    config.json            <- team-shared config (committed)
  local/                   <- personal overrides (gitignored)
    config.json            <- personal config, identity (gitignored)
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

**Committed**: `custom/`, `projects/`, `archive/`, `docs/`

**Gitignored**: `base/`, `local/`, `logs/`, `worktrees/`

### New Engineer Setup

1. Clone the repo. You get `custom/`, `archive/`, and `docs/` immediately.
2. Install Wolfcastle. `brew install wolfcastle` or the curl installer.
3. `git config core.hooksPath .githooks` to activate the pre-commit hook (fmt, vet, build, lint).
4. [`wolfcastle init`](cli/init.md). Creates `local/config.json` with your [identity](configuration.md#identity). Generates `base/`.
5. [`wolfcastle start`](cli/start.md). [The daemon](how-it-works.md#the-daemon) wakes up. Your [namespace](collaboration.md#engineer-namespacing) is created. Work begins.
