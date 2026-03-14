# Wolfcastle

You have a goal. Wolfcastle will destroy it.

Wolfcastle is a model-agnostic autonomous project orchestrator. It takes complex work, breaks it into a tree of projects and tasks that goes as deep as it needs to, then sends AI models to hunt every task down and eliminate it. One by one. Relentlessly. While you do whatever it is you do.

## Status

Pre-alpha. Architecture and design phase. The blueprints for the weapon are complete. Construction has not begun.

See the [31 Architecture Decision Records](docs/decisions/INDEX.md) and [9 Specifications](docs/specs/) for the full design.

## How It Works

You give Wolfcastle a goal. It decomposes that goal into a tree of projects and tasks. A daemon takes over: it picks the next target, invokes a model, validates the result, records what happened, and moves to the next target. No breaks. No hesitation. Serial execution, depth-first, until the tree is conquered or something gets in the way.

If a task fails, Wolfcastle tries again. If it fails ten times, Wolfcastle decomposes it into smaller, weaker problems and destroys those instead. If decomposition itself runs out of room, the task is blocked and Wolfcastle moves on. It does not waste time on the fallen.

Everything is deterministic except the model's output. State is JSON. All mutations go through Go scripts. The model decides *what* to do. The scripts do it *correctly*. You can stop the daemon, inspect the tree, rearrange things by hand, and restart. Wolfcastle picks up exactly where it left off. It does not forget.

## The Project Tree

Work is organized as a tree. The tree has two node types and no depth limit.

**Orchestrator nodes** contain child nodes — other orchestrators or leaves. Their state is computed from their children. You do not set it. You do not touch it. The children report upward and the orchestrator obeys the math.

**Leaf nodes** contain an ordered list of tasks. The last task in every leaf is an audit task — auto-created, immovable, non-negotiable. Every piece of work gets verified. No exceptions.

Orchestrators can contain orchestrators. Those can contain more orchestrators. The tree goes as deep as the work demands. A single goal becomes projects, sub-projects, sub-sub-projects, and tasks at every level. Wolfcastle does not care how deep it goes. It will find the bottom.

```
goal/
  backend/
    auth/
      session-tokens/        ← leaf: tasks live here
      oauth-provider/        ← leaf
    database/
      migrations/            ← leaf
      connection-pool/       ← leaf
  frontend/
    login-flow/
      form-validation/       ← leaf
      error-states/          ← leaf
```

Traversal is depth-first. Wolfcastle walks the tree top-to-bottom, left-to-right, and works on one task at a time. One target. One model. No mercy.

## Four States

Every node and task has exactly one of four states.

| State | What It Means |
|-------|---------------|
| `not_started` | Waiting. Its time will come. |
| `in_progress` | Under attack. |
| `complete` | Destroyed. Terminal. Never comes back. |
| `blocked` | Cannot proceed. Waiting for a human to do their part. |

There is no `failed`. There is no `cancelled`. There is no `paused`. Work that cannot continue is blocked. Work that is done is complete. Everything else is in progress or waiting.

### State Propagation

State flows upward. Only upward. When a task completes, its leaf recomputes. When a leaf completes, its parent orchestrator recomputes. This continues to the root. The algorithm is deterministic:

- All children not started → parent is not started
- All children complete → parent is complete
- All non-complete children blocked → parent is blocked
- Anything else → parent is in progress

No node sets its own state. State is a consequence of the work below it. Insubordination is not a valid state.

## Distributed State

State is stored as one `state.json` per node, co-located with its project description and task documents. A root-level `state.json` serves as a centralized index for fast navigation without walking the filesystem.

```
.wolfcastle/projects/wild-macbook/
  state.json                        ← root index
  backend/
    state.json                      ← orchestrator state
    backend.md                      ← project description
    auth/
      state.json                    ← orchestrator state
      auth.md
      session-tokens/
        state.json                  ← leaf state (tasks, audit, failures)
        session-tokens.md           ← project description
        task-3.md                   ← task working document (optional)
```

Every state mutation writes to the affected node, its parent chain, and the root index in the same operation. Task descriptions live in the leaf's `state.json`. Rich working documents — findings, context, research — go in optional Markdown files next to the state. Only the active task's working document gets injected into the model's context. Wolfcastle respects the context window. It is surgical, not reckless.

## The Daemon

`wolfcastle start` launches the daemon. It owns the pipeline loop and does not share.

```
wolfcastle start                          # foreground — watch it work
wolfcastle start -d                       # background — it works while you don't
wolfcastle start --node backend/auth      # scoped — only this subtree
wolfcastle start --worktree feature/auth  # isolated — separate git worktree
```

The daemon runs one iteration at a time. Each iteration walks the configured pipeline stages, invokes models, and advances the tree. Between iterations, it checks for stop signals. On SIGTERM or SIGINT, it finishes the current stage, cleans up child processes, and shuts down. It does not leave a mess.

### Self-Healing

If the daemon crashes mid-task — power failure, OOM, act of god — it recovers on restart. It finds the task left `in_progress`, hands the situation to the model, and lets it decide: resume, roll back, or block. Stale PID files are detected and ignored. The daemon does not panic. It has seen worse.

### Process Management

- Child model processes spawn in their own process group
- SIGTERM/SIGINT propagate to all children before shutdown
- PID file written in background mode only (`.wolfcastle/wolfcastle.pid`)
- Single-instance enforcement — one daemon per namespace
- `wolfcastle stop` for graceful shutdown; `wolfcastle stop --force` for immediate termination

## Configurable Pipelines

The daemon runs a pipeline of stages. Each stage invokes a model with a specific role. The default pipeline:

| Stage | Model Tier | Mission |
|-------|-----------|---------|
| **expand** | cheap | Reads the inbox. Breaks new items into tasks. |
| **file** | mid | Organizes tasks into the correct project nodes. |
| **execute** | capable | Claims a task. Does the work. Writes code. Makes commits. |
| **summary** | cheap | Writes a plain-language summary after audit completion. |

Each stage is an independent operation. Stages do not pass output to each other. They read the current state of the world and act on it. The expand stage creates tasks. The execute stage finds them. No coupling. No handoffs. Just state on disk and models that know how to read it.

### Pipeline Configuration

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

Add stages. Remove stages. Reorder stages. Run a single-stage pipeline with one model that does everything. Wolfcastle does not judge your architecture. It executes it.

Stages can be individually enabled or disabled. The summary stage is opt-out for cost-sensitive operations. Each stage can skip prompt assembly if it brings its own context.

### The Seven-Phase Execution Model

When the execute stage claims a task, the model follows a seven-phase protocol:

1. **Claim** — Lock the task. It belongs to this model now.
2. **Study** — Read the project description, specs, breadcrumbs, and any linked context.
3. **Implement** — Do the work. Write code. Make changes.
4. **Validate** — Run the configured validation commands (tests, lints, builds).
5. **Record** — Write breadcrumbs describing what was done and why.
6. **Commit** — Commit the changes with a structured message.
7. **Yield** — Release the task. Report completion or failure.

The model communicates with Wolfcastle through script calls. It runs `wolfcastle task claim`, `wolfcastle audit breadcrumb`, `wolfcastle task complete`. Every side effect goes through a deterministic command that validates inputs and enforces invariants. The model cannot corrupt the tree. It can only ask the scripts to make valid changes.

## Model Agnostic

Wolfcastle does not embed any model SDK. It does not import any provider library. It does not care who made your model or where it runs.

Models are defined in configuration as CLI commands:

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

Any CLI tool that accepts a prompt on stdin and produces output on stdout is a valid model. Claude, GPT, Gemini, Llama, a bash script that echoes "done" — Wolfcastle invokes it the same way. Switch providers by changing config. No code changes. No recompilation. No vendor lock-in.

Authentication is your problem. Wolfcastle does not store API keys, manage tokens, or negotiate with identity providers. Use environment variables, CLI login commands, or whatever your provider demands. Wolfcastle calls the command. The command figures out the rest.

## Three-Tier Configuration

Configuration merges across three tiers. Each tier overrides the one below it.

| Tier | Location | Ownership | Git Status |
|------|----------|-----------|------------|
| **base/** | `.wolfcastle/base/` | Wolfcastle | Gitignored. Regenerated on init/update. |
| **custom/** | `.wolfcastle/custom/` | Team | Committed. Shared across all engineers. |
| **local/** | `.wolfcastle/local/` | You | Gitignored. Personal overrides. |

**JSON objects** deep-merge recursively. Override a single nested field without rewriting the whole object. **Arrays** replace entirely — no element merging, no ambiguity. Set a field to **`null`** in a higher tier to delete it from the resolved config.

The same three-tier resolution applies to prompt templates and rule fragments. Same-named files in higher tiers completely replace lower tier versions. New files are added. The merge is predictable. Configuration is not a democracy.

Two config files control runtime behavior:

- **`config.json`** — Team-shared, committed. Models, pipelines, thresholds, validation commands.
- **`config.local.json`** — Personal, gitignored. Identity, model overrides, local preferences.

### Identity

Your identity lives in `config.local.json`, auto-populated on `wolfcastle init`:

```json
{
  "identity": {
    "user": "wild",
    "machine": "macbook"
  }
}
```

This determines your project namespace. Your work lives under `.wolfcastle/projects/wild-macbook/`. Nobody else writes there. You write nowhere else.

## Failure Handling and Decomposition

Tasks fail. Wolfcastle does not take it personally. It takes it systematically.

Each task tracks a failure counter. The escalation path:

| Failures | Depth OK | Action |
|----------|----------|--------|
| < 10 | — | Keep trying. The enemy weakens with each attempt. |
| = 10 | Yes | Decompose. The task becomes an orchestrator with smaller child tasks. Each child starts fresh. |
| = 10 | No (depth limit reached) | Auto-block. Decomposition cannot recurse forever. |
| = 50 | — | Hard block. Safety net. No matter the depth, no matter the context, the task is done fighting. |

All thresholds are configurable:

```json
{
  "failure": {
    "decomposition_threshold": 10,
    "max_decomposition_depth": 5,
    "hard_cap": 50
  }
}
```

### Decomposition in Detail

When a task hits the decomposition threshold, the model is prompted to break it into smaller problems. The leaf node transforms into an orchestrator node with new child leaves. Each child inherits `decomposition_depth + 1`. Each child's failure counter starts at zero. Fresh targets. Clean slate.

Decomposition can recurse. A decomposed task's children can themselves decompose. The tree grows deeper as problems get harder. The `max_decomposition_depth` setting prevents infinite recursion. The `hard_cap` prevents infinite iteration. Between them, Wolfcastle always stops eventually.

### API Failure Handling

Model API failures (timeouts, rate limits, server errors) get exponential backoff:

```json
{
  "api_retry": {
    "initial_delay_seconds": 30,
    "max_delay_seconds": 600,
    "max_retries": -1
  }
}
```

`max_retries: -1` means unlimited. Wolfcastle will wait as long as it takes. The server will come back. They always come back.

## The Audit System

Every leaf ends with an audit task. It is created automatically. It cannot be moved. It cannot be deleted. It runs last, after every other task in the leaf has completed. Its job: verify that the work actually happened and actually works.

### Breadcrumbs

As tasks execute, they write timestamped breadcrumbs via `wolfcastle audit breadcrumb`. Each breadcrumb describes what was done, why, and what changed. These are not terse commit messages. They are rich, explanatory records — the raw material for verification.

### Audit Execution

The audit task reviews all breadcrumbs against the leaf's defined criteria. It checks:

- Did the implementation match the requirements?
- Are there gaps between what was planned and what was done?
- Do the validation results confirm the work?

### Gap Escalation

If the audit finds gaps it cannot resolve locally, it escalates them upward to the parent orchestrator via `wolfcastle audit escalate`. The parent's audit scope now includes cross-cutting verification of those gaps. Escalation can propagate all the way to the root if necessary.

### Audit Status

Audit tracking has its own lifecycle, separate from task states:

| Status | Meaning |
|--------|---------|
| `pending` | Audit has not started. |
| `in_progress` | Audit is running. |
| `passed` | All criteria met. No gaps. |
| `failed` | Gaps found. Escalation may follow. |

Gaps are tracked individually with deterministic IDs, open/fixed status, and full traceability.

## Engineer Namespacing

Multiple engineers work on the same repo simultaneously. No merge conflicts. No coordination overhead. Each engineer's project tree lives in its own namespace:

```
.wolfcastle/projects/
  wild-macbook/          ← your tree
  dave-workstation/      ← Dave's tree
  sarah-laptop/          ← Sarah's tree
```

Each engineer reads and writes only their own namespace. Wolfcastle enforces this. Everyone can see everyone else's work — the `projects/` directory is committed — but nobody steps on anyone else's state.

`wolfcastle status` shows your tree. `wolfcastle status --all` aggregates across all engineers at runtime. No shared index file. No merge conflicts. No drama.

### Overlap Advisory

When you create a new project, Wolfcastle optionally scans other engineers' active projects and alerts you if scope overlaps. Read-only. Informational. No blocking, no state changes. You can ignore it entirely. But you were warned.

```json
{
  "overlap_advisory": {
    "enabled": true,
    "model": "fast"
  }
}
```

## Git Integration

### Default Behavior

Wolfcastle commits to your current branch. No branch creation. No branch management. No magic. At the start of each iteration and before every commit, Wolfcastle verifies the current branch matches the branch recorded at startup. If someone switched branches underneath it, the daemon blocks immediately. It does not commit to the wrong branch. Ever.

### Worktree Isolation

For those who want separation:

```
wolfcastle start --worktree feature/auth
```

Wolfcastle creates a git worktree in `.wolfcastle/worktrees/`, checks out the specified branch (or creates it from HEAD), and runs all work inside the worktree. Your working directory is never touched. Review the work when you're ready. Merge it when you're satisfied. The worktree is cleaned up on stop or completion.

### Combined Scoping

Node scoping and worktree isolation compose:

```
wolfcastle start --worktree feature/auth --node backend/auth
```

Isolated branch. Focused subtree. Maximum precision.

## Structural Validation

The validation engine checks the entire distributed state tree for consistency. It classifies 17 distinct issue types by severity:

- **9 deterministic fixes** — Missing audit task, stale index entry, orphaned files. Go code fixes these directly. No model needed.
- **5 ambiguous fixes** — Conflicting state, unclear intent. A configurable model reasons about the fix with strict guardrails.
- **1 daemon self-healing** — Crash recovery. Handled on next startup.
- **1 manual** — Requires human judgment. You are the model now.
- **1 cross-engineer** — Overlap or conflict across namespaces.

### wolfcastle doctor

Interactive validation and repair:

```
wolfcastle doctor
```

Scans the tree. Reports findings with locations and severity. You choose: fix all, fix selected, or abort. Deterministic fixes are applied by Go code. Ambiguous fixes are reasoned by a model you configure:

```json
{
  "doctor": {
    "model": "mid",
    "prompt_file": "doctor.md"
  }
}
```

The validation engine also runs a subset of checks on daemon startup. If the tree is corrupted, the daemon refuses to start. It will not build on a broken foundation.

## The Unblock Workflow

Tasks block. It happens. Wolfcastle provides three escalating tiers to deal with it.

### Tier 1: Status Flip

```
wolfcastle task unblock --node backend/auth/session-tokens
```

Zero cost. You already fixed the problem externally. This resets the failure counter and sets the task back to `not_started`. No model involved. Instant.

### Tier 2: Interactive Model-Assisted

```
wolfcastle unblock --node backend/auth/session-tokens
```

Multi-turn conversation with a model, pre-loaded with everything: block reason, failure history, breadcrumbs, audit context, previous attempts. You and the model work through the fix together. This is explicitly not autonomous — the human drives. When you're done, run Tier 1 to flip the status.

### Tier 3: Agent Context Dump

```
wolfcastle unblock --agent --node backend/auth/session-tokens
```

Rich structured diagnostic output for consumption by an external agent. Full block diagnostic, breadcrumbs, audit state, file paths, suggested approaches, and instructions. Feed it to whatever agent you're running. Wolfcastle provides the intelligence. The agent provides the muscle.

All tiers reset the task to `not_started`, not `in_progress`. Fresh evaluation. No blind resumption.

## Codebase Audit

A standalone command for auditing your codebase against composable, discoverable scopes:

```
wolfcastle audit                              # all scopes
wolfcastle audit --scope dry,modularity       # specific scopes
wolfcastle audit --list                       # show available scopes
```

The audit is strictly read-only. The model reads your code, analyzes it against the requested scopes, and produces a Markdown report. It does not modify files, create branches, write code, or touch your codebase in any way. The only output is the report. Observation without interference.

### Scopes

Scopes are enum-like IDs backed by prompt fragments. Base scopes ship with Wolfcastle (`dry`, `modularity`, `decomposition`, `comments`, etc.). Add custom scopes in `custom/audits/` or personal scopes in `local/audits/`. All three tiers are discovered at runtime.

### The Approval Gate

Audit findings do not become tasks automatically. The model generates prioritized findings in its Markdown report. You review them. Approve all, review individually, or reject all. Approved findings become projects and tasks in your tree. Rejected findings disappear. Nothing changes until you say so. Wolfcastle does not create work without permission. It has manners. Aggressive manners, but manners.

## In-Flight Specs

Living specifications that travel with the work:

```
wolfcastle spec create --node backend/auth "Authentication Protocol"
wolfcastle spec link --node backend/auth/oauth oauth-spec.md
wolfcastle spec list --node backend/auth
```

Specs live in the committed `docs/specs/` directory with ISO 8601 timestamp filenames. Each node's `state.json` references the specs relevant to it. Only referenced specs are injected into the model's context for that node. The model is told other specs exist and can pull them on demand.

Multiple nodes can reference the same spec. A cross-cutting specification links to every node that needs it. Context stays minimal by default. Models reach for more when they need it.

## Composable Rule Fragments

Prompts and rules are assembled from composable fragments with sensible defaults. Wolfcastle ships base fragments covering git conventions, commit format, ADR usage, and more. Teams add custom fragments in `custom/`. Engineers add personal fragments in `local/`.

Fragments merge in order defined by config. An empty array means auto-discovery in alphabetical order. An explicit array means you control the sequence. Override one concern without rewriting everything.

## Script Reference via Prompt Injection

The model needs to know what commands are available. Wolfcastle generates a complete script reference from Go source code and injects it into the system prompt. No separate documentation to maintain. No drift between what the docs say and what the code does. The reference regenerates on `wolfcastle init` and `wolfcastle update`. It lives in gitignored `base/` where it belongs.

## Logging

Logs are NDJSON — one self-contained JSON record per line. Each daemon iteration produces its own log file:

```
.wolfcastle/logs/0001-20260312T18-45Z.jsonl
.wolfcastle/logs/0002-20260312T18-47Z.jsonl
```

Iteration prefix for ordering. Timestamp for context. `wolfcastle follow` finds the latest file and tails it, watching for new files as iterations advance.

Retention is configurable:

```json
{
  "logs": {
    "max_files": 100,
    "max_age_days": 30,
    "compress": true
  }
}
```

Query with `jq` for quick filters. Point DuckDB at the directory for SQL over your entire log history. Wolfcastle writes the data. You choose the weapon.

## Archive

When a project completes, it graduates to the archive. Each archive entry is a self-contained Markdown file with a timestamp filename:

```
.wolfcastle/archive/2026-03-12T18-45Z-auth-implementation-complete.md
```

Contents, generated deterministically from state:

- **Summary** — Model-written plain-language summary of what was accomplished and why it matters
- **Breadcrumbs** — Chronological record of task-level work
- **Audit results** — Scopes verified, gaps found and fixed, escalations
- **Metadata** — Node path, completion timestamp, engineer identity, branch

The summary is written by a configurable model after audit completion. It is opt-out for cost-sensitive operations. The rest is deterministic. No extra model call at archive time.

Archive filenames are unique by construction — timestamp plus slug. Two engineers completing different work produce different files. Append-only. Merge-conflict-proof. Searchable, readable history that survives every rebase.

## The Inbox

For ideas that arrive while work is underway:

```
wolfcastle inbox add "Support OAuth2 PKCE flow"
wolfcastle inbox list
```

Items land in the inbox. The expand pipeline stage picks them up, decomposes them into tasks, and the file stage organizes them into the tree. You throw things at Wolfcastle. It catches them, breaks them down, and files them where they belong.

## CLI Surface

21+ commands. Every one accepts `--json` for structured output. Every one that operates on a node accepts `--node` with a slash-separated tree address. Every one has `-h` help with dynamic content — available scopes, install targets, and spec lists are discovered at runtime.

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

### Tree Addressing

Every node is addressable by its path from the root:

```
wolfcastle task add --node backend/auth/session-tokens "Implement token rotation"
wolfcastle start --node backend
wolfcastle status --node frontend/login-flow
```

Scripts validate that the target node exists and is the correct type. You cannot add a task to an orchestrator. You cannot create a child under a leaf. The tree has rules. Wolfcastle enforces them.

## Security Model

Wolfcastle does not sandbox anything. It does not filter commands, restrict file access, or enforce permission policies. Security is configured at the model level through CLI flags in the `models` dictionary:

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

The executing model's capabilities are determined entirely by the flags you gave it. Teams enforce permissions through config review of `config.json`. Individual engineers loosen permissions in gitignored `config.local.json` at their own risk. Wolfcastle makes the security posture explicit and auditable. What you configure is what you get.

## Project Layout

`wolfcastle init` creates the `.wolfcastle/` directory:

```
.wolfcastle/
  .gitignore
  config.json              ← team-shared config (committed)
  config.local.json        ← personal config, identity (gitignored)
  base/                    ← Wolfcastle defaults, prompts, scripts (gitignored)
  custom/                  ← team overrides and additions (committed)
  local/                   ← personal overrides (gitignored)
  projects/                ← live work trees, per engineer (committed)
    wild-macbook/
    dave-workstation/
  archive/                 ← completed work summaries (committed)
  docs/                    ← ADRs and specs (committed)
    decisions/
    specs/
  logs/                    ← NDJSON iteration logs (gitignored)
  worktrees/               ← git worktrees when using --worktree (gitignored)
```

**Committed**: `config.json`, `custom/`, `projects/`, `archive/`, `docs/`
**Gitignored**: `base/`, `local/`, `config.local.json`, `logs/`, `worktrees/`

### New Engineer Setup

1. Clone the repo. You get `config.json`, `custom/`, `archive/`, and `docs/` immediately.
2. Install Wolfcastle. `brew install wolfcastle` or the curl installer.
3. `wolfcastle init`. Creates `config.local.json` with your identity. Generates `base/`.
4. `wolfcastle start`. The daemon wakes up. Your namespace is created. Work begins.

## Installation

Coming soon. Three distribution channels:

- **`curl` installer** — Zero dependencies. Download and run.
- **Homebrew tap** — `brew install wolfcastle` for the civilized.
- **npm wrapper** — Optional, for teams already in that ecosystem.
- **Self-update** — `wolfcastle update` refreshes the binary and regenerates `base/`.

### Claude Code Integration

```
wolfcastle install skill
```

Installs the Wolfcastle skill for Claude Code. Uses symlinks where supported (auto-updates with `wolfcastle update`) and falls back to file copy on platforms that don't. Enables native Wolfcastle command access from within Claude Code sessions.

## Documentation

- [Architecture Decision Records](docs/decisions/INDEX.md) — 31 accepted decisions covering every major design choice
- [Specifications](docs/specs/) — 9 detailed specs covering state machine, configuration, pipelines, CLI, validation, and more
- [Voice Guide](prompts/VOICE.md) — How Wolfcastle talks
