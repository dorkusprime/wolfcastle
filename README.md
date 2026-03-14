# Wolfcastle

A model-agnostic autonomous project orchestrator. Wolfcastle breaks complex work into a persistent tree of projects, sub-projects, and tasks, then executes them through configurable multi-model pipelines.

## Status

Pre-alpha — architecture and design phase. See [Architecture Decision Records](docs/decisions/INDEX.md) and [Specifications](docs/specs/) for the full design.

## How It Works

You give Wolfcastle a goal. It decomposes that goal into a tree of projects and tasks, then works through them one at a time using AI models you configure. A long-running daemon drives the loop: pick the next task, invoke the model, validate the result, record what happened, move on. If a task fails too many times, Wolfcastle decomposes it into smaller pieces and tries again. If it gets truly stuck, it blocks and waits for you.

Everything is deterministic except the model's output. State is JSON. Mutations go through Go scripts, never the model directly. You can stop the daemon, inspect the tree, fix something by hand, and restart — it picks up where it left off.

## Core Concepts

### The Project Tree

Work is organized as a tree of two node types:

- **Orchestrator nodes** contain child nodes (other orchestrators or leaves). Their state is derived from their children — you never set it directly.
- **Leaf nodes** contain an ordered list of tasks. The last task in every leaf is always an audit task, auto-created and immovable.

The tree is traversed depth-first. Wolfcastle works on one task at a time, serially.

### Four States

Every node and task has exactly one of four states:

| State | Meaning |
|-------|---------|
| `not_started` | No work has begun. |
| `in_progress` | Work is actively happening. |
| `complete` | All work is done and verified. Terminal — never transitions out. |
| `blocked` | Cannot continue without human intervention. |

There are no other states. No "failed", "cancelled", or "paused". Work that can't proceed is blocked.

### State Propagation

State flows **upward only**. When a task completes, its leaf recomputes. When a leaf completes, its parent orchestrator recomputes, and so on up to the root. The propagation algorithm is deterministic:

- All children Not Started → parent is Not Started
- All children Complete → parent is Complete
- All non-complete children Blocked → parent is Blocked
- Anything else → parent is In Progress

### Distributed State Files

State is stored as one `state.json` per node, co-located with its project description. A root-level `state.json` serves as a centralized index for fast navigation without filesystem walks. Every state mutation writes to the affected node, its parent, and the root index in the same operation.

### Configurable Pipelines

The daemon runs a pipeline of stages each iteration. The default pipeline:

1. **expand** (cheap model) — reads the inbox, breaks items into tasks
2. **file** (mid-tier model) — organizes tasks into the right project nodes
3. **execute** (capable model) — claims a task, does the work, writes code
4. **summary** (cheap model) — writes a plain-language summary after audit completion

Each stage invokes a model as an external CLI process. The model receives an assembled prompt via stdin and communicates back through side effects: calling Wolfcastle commands, writing files, making commits. Stages don't pass output to each other — they read the current state of the world.

You can reconfigure the pipeline freely: change which models run which stages, add stages, remove stages, or use a single model for everything.

### Model Agnostic

Wolfcastle doesn't embed any model SDK. Models are defined in configuration as CLI commands with arguments:

```json
{
  "models": {
    "fast": { "command": "claude", "args": ["-p", "--model", "claude-haiku-4-5-20251001", "--output-format", "stream-json"] },
    "heavy": { "command": "claude", "args": ["-p", "--model", "claude-opus-4-6", "--output-format", "stream-json"] }
  }
}
```

Any CLI tool that accepts a prompt on stdin and produces output on stdout can be used as a model. Switch between providers by changing config — no code changes needed.

### Three-Tier Configuration

Configuration merges across three tiers:

1. **`base/`** — Wolfcastle defaults, regenerated on install/update
2. **`custom/`** — Team-shared overrides, committed to git
3. **`local/`** — Personal overrides, gitignored

JSON objects deep-merge recursively. Arrays are fully replaced (not element-merged). A field set to `null` in a higher tier deletes it. The same resolution applies to prompt templates and rule fragments.

Two config files control behavior: `config.json` (team-shared, committed) and `config.local.json` (personal, gitignored). Identity — your username and machine name, used to namespace your project directory — lives only in the local file.

### Failure Handling and Decomposition

Each task tracks a failure counter. When the counter hits a configurable threshold (default: 10), the model is prompted to decompose the task into smaller pieces. The leaf transforms into an orchestrator with new child leaves, each starting fresh. Decomposition can recurse up to a configurable depth (default: 5). A hard cap (default: 50) auto-blocks a task regardless of depth as a safety net against unbounded iteration.

### Audit Propagation

Every leaf ends with an audit task that verifies the work done by preceding tasks. As tasks complete, they write timestamped breadcrumbs describing what they did. The audit task reviews these breadcrumbs against defined criteria. If the audit finds gaps, those gaps can escalate upward to the parent orchestrator for cross-cutting verification.

### Structural Validation

A validation engine checks the consistency of the distributed state tree: root index matches disk, orchestrator states match their children, audit tasks are in the right position, no orphaned files, no invalid state values. This powers both `wolfcastle doctor` (interactive repair) and daemon startup checks (refuse to start if the tree is corrupted).

The engine classifies 17 issue types by severity. Deterministic fixes (9 types) are applied directly by Go code. Ambiguous fixes (5 types) use a configurable model with strict guardrails. One type defers to the daemon's self-healing. One requires manual intervention.

### Daemon Lifecycle

`wolfcastle start` launches a background daemon that owns the pipeline loop. The daemon:

- Manages a PID file for single-instance enforcement
- Spawns model processes in their own process group
- Intercepts SIGTERM/SIGINT and propagates to child processes
- Finishes the current stage before shutting down on stop signal
- Self-heals on startup: if a previous run crashed mid-task, it resumes that task

### Engineer Namespacing

Each engineer's project tree lives under `.wolfcastle/projects/{user}-{machine}/`. Multiple engineers work on the same repository concurrently without merge conflicts — each has their own state tree, and Wolfcastle only touches its own namespace.

## CLI Surface

Wolfcastle provides 21+ commands across these categories:

| Category | Commands |
|----------|----------|
| Lifecycle | `init`, `start`, `stop`, `status`, `follow` |
| Task | `task add`, `task claim`, `task complete`, `task block`, `task unblock` |
| Project | `project create`, `project list` |
| Audit | `audit breadcrumb`, `audit escalate`, `audit run` |
| Navigation | `navigate` |
| Diagnostics | `doctor`, `log` |
| Documentation | `spec`, `decision` |
| Archive | `archive` |
| Inbox | `inbox add`, `inbox list` |
| Integration | `install` |

All commands accept `--json` for structured output. Commands that operate on nodes accept `--node` with a slash-separated tree address (e.g., `attunement-tree/fire-impl`).

## Installation

Coming soon. Target distribution: `curl` installer, Homebrew tap, optional npm wrapper.

## Documentation

- [Architecture Decision Records](docs/decisions/INDEX.md) — 31 accepted decisions covering every major design choice
- [Specifications](docs/specs/) — 9 detailed specs covering state machine, configuration, pipelines, CLI, validation, and more
