# Wolfcastle

![Wolfcastle](assets/wolfcastle-alpha.png)

You have a goal. Wolfcastle will destroy it.

Wolfcastle is a model-agnostic autonomous project orchestrator. It takes complex work, breaks it into a tree of projects and tasks that goes as deep as it needs to, then sends AI models to hunt every task down and eliminate it. One by one. Relentlessly. While you do whatever it is you do.

## Status

Pre-alpha. Architecture and design phase. The blueprints for the weapon are complete. Construction has not begun.

See the [Architecture Decision Records](docs/decisions/INDEX.md) and [Specifications](docs/specs/) for the full design.

## How It Works

You give Wolfcastle a goal. It decomposes that goal into a tree of projects and tasks. A daemon takes over: it picks the next target, invokes a model, validates the result, records what happened, and moves to the next target. No breaks. No hesitation. Serial execution, depth-first, until the tree is conquered or something gets in the way.

If a task fails, Wolfcastle tries again. If it fails ten times, Wolfcastle decomposes it into smaller, weaker problems and destroys those instead. If decomposition runs out of room, the task is blocked and Wolfcastle moves on. It does not waste time on the fallen.

Everything is deterministic except the model's output. State is JSON on disk. The model decides *what* to do. Go scripts do it *correctly*. You can stop the daemon, inspect the tree, rearrange things by hand, and restart. Wolfcastle picks up exactly where it left off. It does not forget.

## Quick Start

```bash
brew install wolfcastle          # or: curl -sSL https://wolfcastle.dev/install | sh
cd your-repo
wolfcastle init                  # creates .wolfcastle/, sets your identity
wolfcastle start                 # the daemon wakes up. work begins.
```

## The Project Tree

Work lives in a tree of two node types: orchestrators (containers) and leaves (where tasks live). Orchestrators hold other orchestrators or leaves; leaves hold an ordered list of tasks. Every leaf ends with an automatic audit task that verifies the work. The tree goes as deep as the work demands.

Nodes have four states: `not_started`, `in_progress`, `complete`, `blocked`. State propagates upward deterministically. When a task completes, its leaf recomputes, then its parent, all the way to the root. No node sets its own state. State is a consequence of the work below it.

[Full details: tree structure, state propagation rules, distributed state layout.](docs/humans/how-it-works.md)

## The Daemon

`wolfcastle start` launches a daemon that runs a pipeline of stages in a loop. The default pipeline has four stages: expand (break inbox items into tasks), file (organize tasks into the tree), execute (claim a task and do the work), and summary (write a plain-language record after audit). Each stage invokes a model with a specific role. Stages are decoupled; they read state from disk and act on it independently.

When the execute stage claims a task, the model follows a seven-phase protocol: claim, study, implement, validate, record, commit, yield. The model communicates only through deterministic script calls that enforce invariants. It cannot corrupt the tree.

[Full details: pipeline stages, seven-phase execution, process management.](docs/humans/how-it-works.md#the-daemon)

## Configuration

Config merges across three tiers: base (Wolfcastle defaults, gitignored), custom (team-shared, committed), and local (personal, gitignored). Higher tiers override lower ones. JSON objects deep-merge; arrays replace entirely; set a field to `null` to delete it.

Models are defined as CLI commands. Anything that reads stdin and writes stdout works: Claude, GPT, Gemini, Llama, a bash script that echoes "done." Switch providers by editing a JSON file. The same three-tier system governs prompt templates, rule fragments, and audit scopes.

[Full details: config merging, model definitions, pipelines, identity, rule fragments, security model.](docs/humans/configuration.md)

## Failure and Recovery

A task that fails gets retried. After 10 failures, Wolfcastle decomposes it into smaller sub-tasks and attacks those instead. Decomposition can recurse (with a configurable depth limit), and a hard cap of 50 failures stops any task from fighting forever. All thresholds are configurable.

If the daemon crashes mid-task, it recovers on restart by finding the in-progress task and letting the model decide whether to resume, roll back, or block. Blocked tasks can be unblocked three ways: a zero-cost status flip, an interactive model-assisted session, or a structured context dump for an external agent.

[Full details: failure escalation, decomposition mechanics, API backoff, self-healing, unblock workflow.](docs/humans/failure-and-recovery.md)

## Audits and Quality

Every leaf gets verified by an audit task that reviews breadcrumbs (timestamped records of what each task did) against the leaf's criteria. Gaps that can't be resolved locally escalate to the parent, and escalation can propagate to the root.

Separately, `wolfcastle audit` is a read-only codebase analysis tool. It runs your code against composable scopes (DRY, modularity, decomposition, etc.) and produces a Markdown report. Findings only become tasks if you approve them. `wolfcastle doctor` validates the state tree itself and can fix 9 of 17 issue types deterministically without a model.

[Full details: breadcrumbs, gap escalation, codebase audit scopes, approval gate, doctor/validation.](docs/humans/audits.md)

## Collaboration

Each engineer's project tree lives in its own namespace under `.wolfcastle/projects/` (e.g., `wild-macbook/`, `dave-workstation/`). Everyone can see everyone else's work, but nobody writes to anyone else's state. No merge conflicts. No coordination overhead. An optional overlap advisory warns you when your new project's scope collides with someone else's active work.

Wolfcastle commits to your current branch by default with branch safety checks on every commit. If someone switches branches underneath it, the daemon blocks immediately. For isolation, `--worktree` runs all work in a separate git worktree that gets cleaned up on completion.

[Full details: namespacing, overlap advisory, git integration, worktrees, specs, logging, archive.](docs/humans/collaboration.md)

## CLI

21+ commands across lifecycle, task management, auditing, diagnostics, and documentation. Every command accepts `--json` for structured output. Every command that operates on a node accepts `--node` with a slash-separated tree address. The inbox (`wolfcastle inbox add`) captures ideas mid-flight for the daemon to decompose and file.

[Full details: command reference, tree addressing, inbox, installation, project layout.](docs/humans/cli.md)

## Design Documents

The [Architecture Decision Records](docs/decisions/INDEX.md) document every major design choice. The [Specifications](docs/specs/) provide detailed formal specs for the state machine, configuration system, pipelines, CLI surface, and validation engine.
