# Wolfcastle

![Wolfcastle](assets/wolfcastle-alpha.png)

**You have goals. _Wolfcastle will destroy them._**

Wolfcastle breaks complex work into pieces and sends AI models to eliminate every one of them, as deep as the work demands. If your models can write code, Wolfcastle will put them to work. While you do whatever it is you do.

## Status
[![CI](https://github.com/dorkusprime/wolfcastle/actions/workflows/ci.yml/badge.svg?branch=main)](https://github.com/dorkusprime/wolfcastle/actions/workflows/ci.yml)
[![codecov](https://codecov.io/gh/dorkusprime/wolfcastle/branch/main/graph/badge.svg)](https://codecov.io/gh/dorkusprime/wolfcastle)
[![Go Report Card](https://goreportcard.com/badge/github.com/dorkusprime/wolfcastle)](https://goreportcard.com/report/github.com/dorkusprime/wolfcastle)
[![Go Reference](https://pkg.go.dev/badge/github.com/dorkusprime/wolfcastle.svg)](https://pkg.go.dev/github.com/dorkusprime/wolfcastle)
[![Go version](https://img.shields.io/github/go-mod/go-version/dorkusprime/wolfcastle)](https://go.dev/)
[![License: MIT](https://img.shields.io/badge/license-MIT-blue.svg)](LICENSE)
[![Status](https://img.shields.io/badge/status-pre--release-blue)](https://github.com/dorkusprime/wolfcastle)

Pre-release. The architecture is locked, the implementation is complete, and the test suite is comprehensive. Heading toward a first tagged release.

See the [Architecture Decision Records](docs/decisions/INDEX.md) and [Specifications](docs/specs/) for the full design.

## How It Works

If [Ralph](https://ghuntley.com/loop/) is the dorky kid running in circles on the playground, Wolfcastle is a grown up version of that kid but with biceps, a heroic stride, and a relentless drive to knock down every task list in sight.

Two ways to [get work in](docs/humans/how-it-works.md#getting-work-in):

### Let your agent handle it
You're in your coding agent, and you describe what you want. "Add OAuth2 PKCE support to the auth service." You go back and forth, refining the scope, adding constraints, specifying how you want things structured. When you're satisfied, the agent injects the work into Wolfcastle, which decomposes it into a [tree of projects and tasks](docs/humans/how-it-works.md#the-project-tree).

### Do it yourself
You can also drive Wolfcastle directly through the [CLI](docs/humans/cli.md): [`wolfcastle inbox add`](docs/humans/cli/inbox-add.md) for quick capture, [`wolfcastle project create`](docs/humans/cli/project-create.md) for structured planning, [`wolfcastle task add`](docs/humans/cli/task-add.md) for placing tasks exactly where they belong.

### Then the daemon takes over
[The daemon](docs/humans/how-it-works.md#the-daemon) takes over: it picks the first target, invokes your configured [model](docs/humans/configuration.md#models), validates the result, [records what happened](docs/humans/audits.md#breadcrumbs), and moves to the next. Serial execution, [depth-first](docs/humans/how-it-works.md#the-project-tree), until the tree is conquered or something gets in the way.

If a task fails, Wolfcastle tries again. If it fails ten times, Wolfcastle [decomposes it](docs/humans/failure-and-recovery.md#decomposition) into smaller, weaker problems and destroys those instead. If decomposition runs out of room, the task is [blocked](docs/humans/how-it-works.md#four-states) and Wolfcastle moves on. It does not waste time on the fallen.

Everything is deterministic except the model's output. State is [JSON on disk](docs/humans/how-it-works.md#distributed-state). The model decides _what_ to do. [Scripts](docs/humans/cli.md#commands) do it _correctly_. You can [stop the daemon](docs/humans/cli/stop.md), check on progress with [`wolfcastle status`](docs/humans/cli/status.md), rearrange things by hand, and restart. Wolfcastle [picks up exactly where it left off](docs/humans/failure-and-recovery.md#self-healing).

## Quick Start

```bash
brew install wolfcastle          # or: curl -sSL https://wolfcastle.dev/install | sh
cd your-repo
wolfcastle init                  # creates .wolfcastle/, sets your identity
wolfcastle start                 # the daemon wakes up. work begins.
```

## The Project Tree

Work lives in a [tree of two node types](docs/humans/how-it-works.md#the-project-tree): orchestrators (containers) and leaves (where tasks live). Orchestrators hold other orchestrators or leaves; leaves hold an ordered list of tasks. Every leaf ends with an automatic [audit task](docs/humans/audits.md#the-audit-system) that verifies the work. The tree goes as deep as the work demands.

Nodes have [four states](docs/humans/how-it-works.md#four-states): `not_started`, `in_progress`, `complete`, `blocked`. State [propagates upward](docs/humans/how-it-works.md#state-propagation) deterministically. When a task completes, its leaf recomputes, then its parent, all the way to the root. No node sets its own state. State is a consequence of the work below it, stored as [distributed JSON on disk](docs/humans/how-it-works.md#distributed-state).

## The Daemon

`wolfcastle start` launches a [daemon](docs/humans/how-it-works.md#the-daemon) that runs a [pipeline of stages](docs/humans/how-it-works.md#the-pipeline) in a loop. The default pipeline has four stages: expand (break inbox items into tasks), file (organize tasks into the tree), execute (claim a task and do the work), and summary (write a plain-language record after audit). Each stage invokes a [model](docs/humans/configuration.md#models) with a specific role. Stages are decoupled; they read state from disk and act on it independently.

When the execute stage claims a task, the model follows a [seven-phase protocol](docs/humans/how-it-works.md#seven-phase-execution): claim, study, implement, validate, record, commit, yield. The model communicates only through deterministic script calls that enforce invariants. It cannot corrupt the tree.

## Configuration

Config [merges across three tiers](docs/humans/configuration.md#three-tiers): base (Wolfcastle defaults, gitignored), custom (team-shared, committed), and local (personal, gitignored). Higher tiers override lower ones. JSON objects deep-merge; arrays replace entirely; set a field to `null` to delete it.

[Models](docs/humans/configuration.md#models) are defined as CLI commands. Anything that reads stdin and writes stdout works: Claude, GPT, Gemini, Llama, a bash script that echoes "done." Switch providers by editing a JSON file. The same three-tier system governs prompt templates, [rule fragments](docs/humans/configuration.md#rule-fragments), and [audit scopes](docs/humans/audits.md#scopes). [Pipelines](docs/humans/configuration.md#pipelines) and [security boundaries](docs/humans/configuration.md#security) are configured the same way.

## Failure and Recovery

A task that fails gets retried. After 10 failures, Wolfcastle [decomposes it](docs/humans/failure-and-recovery.md#decomposition) into smaller sub-tasks and attacks those instead. Decomposition can recurse (with a configurable depth limit), and a [hard cap](docs/humans/failure-and-recovery.md#task-failure-escalation) of 50 failures stops any task from fighting forever. All thresholds are configurable.

If the daemon crashes mid-task, it [recovers on restart](docs/humans/failure-and-recovery.md#self-healing) by finding the in-progress task and letting the model decide whether to resume, roll back, or block. Blocked tasks can be [unblocked three ways](docs/humans/failure-and-recovery.md#the-unblock-workflow): a zero-cost status flip, an interactive model-assisted session, or a structured context dump for an external agent.

## Audits and Quality

Every leaf gets verified by an [audit task](docs/humans/audits.md#the-audit-system) that reviews [breadcrumbs](docs/humans/audits.md#breadcrumbs) (timestamped records of what each task did) against the leaf's criteria. Gaps that can't be resolved locally [escalate to the parent](docs/humans/audits.md#gap-escalation), and escalation can propagate to the root.

Separately, `wolfcastle audit run` is a read-only codebase analysis tool. It runs your code against [composable scopes](docs/humans/audits.md#scopes) (DRY, modularity, decomposition, etc.) and produces a Markdown report. Findings only become tasks if you [approve them](docs/humans/audits.md#the-approval-gate). [`wolfcastle doctor`](docs/humans/audits.md#wolfcastle-doctor) validates the [state tree](docs/humans/audits.md#structural-validation) itself, fixing most issue types with deterministic Go code and routing the remaining ambiguous cases to a model for reasoning.

## Collaboration

Each engineer's project tree lives in its own [namespace](docs/humans/collaboration.md#engineer-namespacing) under `.wolfcastle/projects/` (e.g., `wild-macbook/`, `dave-workstation/`). Everyone can see everyone else's work, but nobody writes to anyone else's state. No merge conflicts. No coordination overhead. An optional [overlap advisory](docs/humans/collaboration.md#overlap-advisory) warns you when your new project's scope collides with someone else's active work.

Wolfcastle [commits to your current branch](docs/humans/collaboration.md#git-integration) by default with branch safety checks on every commit. If someone switches branches underneath it, the daemon blocks immediately. For isolation, [`--worktree`](docs/humans/collaboration.md#worktree-isolation) runs all work in a separate git worktree that gets cleaned up on completion. Completed projects are moved to the [archive](docs/humans/collaboration.md#archive), and everything is tracked in [structured logs](docs/humans/collaboration.md#logging).

## CLI

[32 commands](docs/humans/cli.md#commands) across lifecycle, task management, auditing, diagnostics, and documentation. Every command accepts `--json` for structured output. Every command that operates on a node accepts `--node` with a slash-separated [tree address](docs/humans/cli.md#tree-addressing). The [inbox](docs/humans/cli.md#the-inbox) (`wolfcastle inbox add`) captures ideas mid-flight for the daemon to decompose and file. See the full [project layout](docs/humans/cli.md#project-layout) and [installation options](docs/humans/cli.md#installation) in the CLI reference.

## Design Documents

The [Architecture Decision Records](docs/decisions/INDEX.md) document every major design choice. The [Specifications](docs/specs/) provide detailed formal specs for the state machine, configuration system, pipelines, CLI surface, and validation engine. For developer and agent guides covering code standards, architecture, and internals, see [AGENTS.md](AGENTS.md) and [docs/agents/](docs/agents/).
