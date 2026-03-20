# Wolfcastle

![Wolfcastle](assets/wolfcastle-alpha.png)

**You have a codebase. _Wolfcastle will build it._**

You give Wolfcastle a goal. It breaks that goal into a tree of targets and sends AI coding agents to destroy them, depth-first, until the tree is conquered. While you do whatever it is you do.

## Status

[![CI](https://github.com/dorkusprime/wolfcastle/actions/workflows/ci.yml/badge.svg?branch=main)](https://github.com/dorkusprime/wolfcastle/actions/workflows/ci.yml)
[![CodeQL](https://github.com/dorkusprime/wolfcastle/actions/workflows/codeql.yml/badge.svg)](https://github.com/dorkusprime/wolfcastle/actions/workflows/codeql.yml)
[![codecov](https://codecov.io/gh/dorkusprime/wolfcastle/branch/main/graph/badge.svg)](https://codecov.io/gh/dorkusprime/wolfcastle)
[![Go Report Card](https://goreportcard.com/badge/github.com/dorkusprime/wolfcastle)](https://goreportcard.com/report/github.com/dorkusprime/wolfcastle)
[![Go Reference](https://pkg.go.dev/badge/github.com/dorkusprime/wolfcastle.svg)](https://pkg.go.dev/github.com/dorkusprime/wolfcastle)
[![Go version](https://img.shields.io/github/go-mod/go-version/dorkusprime/wolfcastle)](https://go.dev/)
[![License: MIT](https://img.shields.io/badge/license-MIT-blue.svg)](LICENSE)
[![Release](https://img.shields.io/github/v/release/dorkusprime/wolfcastle?include_prereleases)](https://github.com/dorkusprime/wolfcastle/releases)

Pre-release. Probably broken. Then fixed. Then broken again. On repeat until release.

Install via Homebrew if you're feeling brave: `brew install dorkusprime/tap/wolfcastle`

See the [Architecture Decision Records](docs/decisions/INDEX.md) and [Specifications](docs/specs/) for the full design.

## How It Works

If [Ralph](https://ghuntley.com/loop/) is the dorky kid running in circles on the playground, Wolfcastle is a grown up version of that kid but with biceps, a heroic stride, and a relentless drive to ship code.

The pipeline: **inbox, triage, decomposition, execution, audit.** You feed Wolfcastle a feature request, a bug report, a refactoring goal. Wolfcastle breaks it into code changes, sends agents to write those changes, verifies the results, and commits. Two ways to [get work in](docs/humans/how-it-works.md#getting-work-in):

### Let your agent handle it

You're in your coding agent, and you describe what you want built. "Add OAuth2 PKCE support to the auth service." You go back and forth, refining the scope, adding constraints, specifying how you want the code structured. When you're satisfied, the agent injects the work into Wolfcastle, which decomposes it into a [tree of projects and tasks](docs/humans/how-it-works.md#the-project-tree), each one a concrete coding operation.

### Do it yourself

You can also drive Wolfcastle directly through the [CLI](docs/humans/cli.md): [`wolfcastle inbox add`](docs/humans/cli/inbox-add.md) for quick capture, [`wolfcastle project create`](docs/humans/cli/project-create.md) for structured planning, [`wolfcastle task add`](docs/humans/cli/task-add.md) for placing tasks exactly where they belong in the build.

### Then the daemon takes over

[The daemon](docs/humans/how-it-works.md#the-daemon) takes over: it claims the first task, spins up your configured [coding agent](docs/humans/configuration.md#models), points it at the codebase, validates the output, [records what happened](docs/humans/audits.md#breadcrumbs), and moves to the next target. Serial execution, [depth-first](docs/humans/how-it-works.md#the-project-tree), until the tree is conquered or something gets in the way.

If a task fails, Wolfcastle tries again. If it fails ten times, Wolfcastle [decomposes it](docs/humans/failure-and-recovery.md#decomposition) into smaller, weaker problems and destroys those instead. If decomposition runs out of room, the task is [blocked](docs/humans/how-it-works.md#four-states) and Wolfcastle moves on. It does not waste time on the fallen.

Everything is deterministic except the model's output. State is [JSON on disk](docs/humans/how-it-works.md#distributed-state). The agent decides _what code to write_. [Scripts](docs/humans/cli.md#commands) enforce correctness. You can [stop the daemon](docs/humans/cli/stop.md), check on progress with [`wolfcastle status`](docs/humans/cli/status.md), rearrange things by hand, and restart. Wolfcastle [picks up exactly where it left off](docs/humans/failure-and-recovery.md#self-healing).

## Requirements

- **Git** (branch verification, progress detection, auto-commit)
- **A coding agent** that reads stdin and writes stdout. [Claude Code](https://claude.com/claude-code) is the default. Anything that accepts a prompt and produces code works: Cursor, Copilot, GPT, Gemini, Llama, a bash script that echoes "done."
- **Go 1.26+** (only if building from source)

## Quick Start

```bash
brew install dorkusprime/tap/wolfcastle
cd your-repo
wolfcastle init                  # creates .wolfcastle/, sets your identity
wolfcastle start                 # the daemon wakes up. work begins.
```

## The Project Tree

Code changes live in a [tree of two node types](docs/humans/how-it-works.md#the-project-tree): orchestrators (containers that represent features, modules, or milestones) and leaves (where the actual coding tasks live). Orchestrators hold other orchestrators or leaves; leaves hold an ordered list of tasks. Every leaf ends with an automatic [audit task](docs/humans/audits.md#the-audit-system) that verifies the code. The tree goes as deep as the feature demands.

Nodes have [four states](docs/humans/how-it-works.md#four-states): `not_started`, `in_progress`, `complete`, `blocked`. State [propagates upward](docs/humans/how-it-works.md#state-propagation) deterministically. When a coding task completes, its leaf recomputes, then its parent, all the way to the root. No node sets its own state. State is a consequence of the code below it, stored as [distributed JSON on disk](docs/humans/how-it-works.md#distributed-state).

## The Daemon

`wolfcastle start` launches a [daemon](docs/humans/how-it-works.md#the-daemon) that runs a [pipeline of stages](docs/humans/how-it-works.md#the-pipeline). The default pipeline has two stages: intake (triage inbox items into projects and coding tasks) and execute (claim a task, point an agent at the code, ship the result). Intake runs in a parallel goroutine, processing new inbox items independently of task execution. Summaries are generated inline during execution via the `WOLFCASTLE_SUMMARY:` marker (ADR-036), not as a separate stage. Each stage invokes a [coding agent](docs/humans/configuration.md#models) with a specific role. Stages are decoupled; they read state from disk and act on it independently.

When the execute stage claims a task, the agent follows a [ten-phase protocol](docs/humans/how-it-works.md#execution-protocol): claim, study, implement, validate, record, document, commit, signal, pre-block, follow-up. The agent reads and writes your codebase, but communicates with Wolfcastle only through deterministic script calls that enforce invariants. It cannot corrupt the tree.

## Configuration

Config [merges across three tiers](docs/humans/configuration.md#three-tiers): base (Wolfcastle defaults, gitignored), custom (team-shared, committed), and local (personal, gitignored). Higher tiers override lower ones. JSON objects deep-merge; arrays replace entirely; set a field to `null` to delete it.

[Agents](docs/humans/configuration.md#models) are defined as CLI commands. Anything that reads stdin and writes stdout works: Claude Code, Cursor, Copilot, GPT, Gemini, Llama, a bash script that echoes "done." Your agents, your choice. Switch providers by editing a JSON file. The same three-tier system governs prompt templates, [rule fragments](docs/humans/configuration.md#rule-fragments), and [audit scopes](docs/humans/audits.md#scopes). [Pipelines](docs/humans/configuration.md#pipelines) and [security boundaries](docs/humans/configuration.md#security) are configured the same way.

## Failure and Recovery

A coding task that fails gets retried. After 10 failures, Wolfcastle [decomposes it](docs/humans/failure-and-recovery.md#decomposition) into smaller sub-tasks and attacks those instead. If the agent can't write a working migration in one shot, Wolfcastle breaks it into schema changes, data backfill, and validation, then destroys each piece individually. Decomposition can recurse (with a configurable depth limit), and a [hard cap](docs/humans/failure-and-recovery.md#task-failure-escalation) of 50 failures stops any task from fighting forever. All thresholds are configurable.

If the daemon crashes mid-implementation, it [recovers on restart](docs/humans/failure-and-recovery.md#self-healing) by finding the in-progress task and letting the agent decide whether to resume, roll back, or block. Blocked tasks can be [unblocked three ways](docs/humans/failure-and-recovery.md#the-unblock-workflow): a zero-cost status flip, an interactive agent-assisted session, or a structured context dump for an external agent.

## Audits and Quality

Agents write code. Wolfcastle verifies it. Every leaf gets an [audit task](docs/humans/audits.md#the-audit-system) that reviews [breadcrumbs](docs/humans/audits.md#breadcrumbs) (timestamped records of what each coding task produced) against the leaf's acceptance criteria. Gaps that can't be resolved locally [escalate to the parent](docs/humans/audits.md#gap-escalation), and escalation can propagate to the root.

Separately, `wolfcastle audit run` is a read-only codebase analysis tool. It runs your code against [composable scopes](docs/humans/audits.md#scopes) (DRY, modularity, decomposition, etc.) and produces a Markdown report. Findings only become coding tasks if you [approve them](docs/humans/audits.md#the-approval-gate). [`wolfcastle doctor`](docs/humans/audits.md#wolfcastle-doctor) validates the [state tree](docs/humans/audits.md#structural-validation) itself, fixing most issue types with deterministic Go code and routing the remaining ambiguous cases to an agent for reasoning.

## Collaboration

Each engineer's project tree lives in its own [namespace](docs/humans/collaboration.md#engineer-namespacing) under `.wolfcastle/system/projects/` (e.g., `wild-macbook/`, `dave-workstation/`). Everyone can see everyone else's work, but nobody writes to anyone else's state. No merge conflicts. No coordination overhead. An optional [overlap advisory](docs/humans/collaboration.md#overlap-advisory) warns you when your new project's scope collides with someone else's active work.

Wolfcastle [commits code to your current branch](docs/humans/collaboration.md#git-integration) by default with branch safety checks on every commit. If someone switches branches underneath it, the daemon blocks immediately. For isolation, [`--worktree`](docs/humans/collaboration.md#worktree-isolation) runs all work in a separate git worktree that gets cleaned up on completion. Completed projects are moved to the [archive](docs/humans/collaboration.md#archive), and everything is tracked in [structured logs](docs/humans/collaboration.md#logging).

## CLI

[43 commands](docs/humans/cli.md#commands) across lifecycle, task management, auditing, diagnostics, and documentation. Every command accepts `--json` for structured output. Every command that operates on a node accepts `--node` with a slash-separated [tree address](docs/humans/cli.md#tree-addressing). The [inbox](docs/humans/cli.md#the-inbox) (`wolfcastle inbox add`) captures feature requests and bug reports mid-flight for the daemon to decompose into coding tasks. See the full [project layout](docs/humans/cli.md#project-layout) and [installation options](docs/humans/cli.md#installation) in the CLI reference.

## Design Documents

The [Architecture Decision Records](docs/decisions/INDEX.md) document every major design choice. The [Specifications](docs/specs/) provide detailed formal specs for the state machine, configuration system, pipelines, CLI surface, and validation engine. For developer and agent guides covering code standards, architecture, and internals, see [AGENTS.md](AGENTS.md) and [docs/agents/](docs/agents/).

<p align="center">
  <img src="assets/neon-wolf.png" width="150" />
</p>
