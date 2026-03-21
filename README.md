# Wolfcastle

![Wolfcastle](assets/wolfcastle-alpha.png)

**You have a codebase. _Wolfcastle will build it._**

## Why This Exists

Coding agents are good at two things: writing specs and following specs. Hand an agent a clear specification and it will produce working code. Ask it to write a spec for a well-scoped problem and it will produce a solid document. These are solved problems.

What agents cannot do is decide what to build and then build it. The gap between "here's a goal" and "here's a shipping feature" is where every autonomous coding system falls apart. Not because the models aren't capable, but because the scaffolding around them wasn't designed for sustained, unsupervised work.

Three problems kill autonomous agents in practice:

**The planning problem.** "Planning mode" tries to fix the gap between understanding a goal and executing it. But the plan and the execution happen in the same context window, so the agent drifts. It plans five things, does two, gets creative on the third, and forgets the fourth. There's no enforcement. The plan is a suggestion the agent made to itself, and it's free to ignore it. If step 3 reveals that step 4 was wrong, there's no structured way to re-plan. The agent either barrels forward or starts over.

**The context problem.** Every model invocation starts with a blank slate. The agent that just spent twenty minutes understanding your authentication flow will forget all of it when the next task begins. Decisions evaporate. Lessons from failures disappear. The agent that fixed a subtle race condition in task 1 will reintroduce it in task 3 because nothing carried the knowledge forward. This is the [Ralph loop](https://ghuntley.com/loop/): the agent running in circles, solving the same problems over and over, because it has no memory of what it already learned.

**The quality problem.** Agents produce code. Nobody checks it. Or worse, the agent checks its own work in the same context where it wrote the code, which is like asking the author to proofread their own novel. Errors of assumption survive because the assumptions were never questioned by a separate process. The code compiles. The tests pass. The architecture quietly rots.

Wolfcastle exists because these three problems have the same root cause: agents need structure that persists across invocations, enforced by something other than the agent itself.

**Planning is separated from execution.** A planning agent creates the structure: specs, projects, task trees. An execution agent follows it. Neither can overwrite the other. The daemon enforces the contract. If the executor blocks, the system decomposes rather than letting it thrash. Planning is lazy (only happens when there's no executable work) and hierarchical (each orchestrator plans its own children, keeping context small and focused).

**Context is materialized as artifacts.** [Architecture Decision Records](docs/decisions/INDEX.md) capture the "why" behind design choices, so the next agent doesn't reverse a deliberate decision. [Specifications](docs/specs/) capture the "what," giving executors a contract to follow instead of re-interpreting the goal each time. [After Action Reviews](docs/humans/audits.md) capture the "what happened," so mistakes don't repeat across tasks. These aren't optional documentation. The daemon injects them into context automatically. The audit verifies they exist and have substance.

**Quality is a separate pipeline stage.** Every piece of work gets an independent audit from a separate agent invocation with a separate context. The auditor reads [breadcrumbs](docs/humans/audits.md#breadcrumbs) (timestamped records of what each coding task produced), checks them against acceptance criteria, and files gaps. Gaps that can't be resolved locally [escalate upward](docs/humans/audits.md#gap-escalation). No code ships without a second pair of eyes, and those eyes belong to a process that didn't write the code.

The result: you point Wolfcastle at work. It handles the rest. You check in, steer priorities, make the judgment calls that the system doesn't have enough signal to make on its own. The daemon does the volume. You do the thinking.

[![CI](https://github.com/dorkusprime/wolfcastle/actions/workflows/ci.yml/badge.svg?branch=main)](https://github.com/dorkusprime/wolfcastle/actions/workflows/ci.yml)
[![CodeQL](https://github.com/dorkusprime/wolfcastle/actions/workflows/codeql.yml/badge.svg)](https://github.com/dorkusprime/wolfcastle/actions/workflows/codeql.yml)
[![codecov](https://codecov.io/gh/dorkusprime/wolfcastle/branch/main/graph/badge.svg)](https://codecov.io/gh/dorkusprime/wolfcastle)
[![Go Report Card](https://goreportcard.com/badge/github.com/dorkusprime/wolfcastle)](https://goreportcard.com/report/github.com/dorkusprime/wolfcastle)
[![Go Reference](https://pkg.go.dev/badge/github.com/dorkusprime/wolfcastle.svg)](https://pkg.go.dev/github.com/dorkusprime/wolfcastle)
[![Go version](https://img.shields.io/github/go-mod/go-version/dorkusprime/wolfcastle)](https://go.dev/)
[![License: MIT](https://img.shields.io/badge/license-MIT-blue.svg)](LICENSE)
[![Release](https://img.shields.io/github/v/release/dorkusprime/wolfcastle?include_prereleases)](https://github.com/dorkusprime/wolfcastle/releases)

Pre-release. Probably broken. Then fixed. Then broken again. On repeat until release.

## Quick Start

```bash
brew install dorkusprime/tap/wolfcastle
cd your-repo
wolfcastle init                  # creates .wolfcastle/, sets your identity
wolfcastle start                 # the daemon wakes up. work begins.
```

## How It Works

The pipeline: **inbox, triage, decomposition, execution, audit.** You feed Wolfcastle a feature request, a bug report, a refactoring goal. Wolfcastle breaks it into code changes, sends agents to write those changes, verifies the results, and commits.

### Getting work in

Two paths. Your coding agent can inject work directly: describe what you want, refine the scope, and the agent feeds it to Wolfcastle, which decomposes it into a [tree of projects and tasks](docs/humans/how-it-works.md#the-project-tree). Or drive the [CLI](docs/humans/cli.md) yourself: [`wolfcastle inbox add`](docs/humans/cli/inbox-add.md) for quick capture, [`wolfcastle project create`](docs/humans/cli/project-create.md) for structured planning, [`wolfcastle task add`](docs/humans/cli/task-add.md) for placing tasks exactly where they belong.

### The daemon

[The daemon](docs/humans/how-it-works.md#the-daemon) claims the first task, spins up your configured [coding agent](docs/humans/configuration.md#models), points it at the codebase, validates the output, [records what happened](docs/humans/audits.md#breadcrumbs), and moves to the next target. Serial execution, [depth-first](docs/humans/how-it-works.md#the-project-tree), until the tree is conquered or something gets in the way.

If a task fails, Wolfcastle tries again. If it fails ten times, Wolfcastle [decomposes it](docs/humans/failure-and-recovery.md#decomposition) into smaller, weaker problems and destroys those instead. If decomposition runs out of room, the task is [blocked](docs/humans/how-it-works.md#four-states) and Wolfcastle moves on. It does not waste time on the fallen.

Everything is deterministic except the model's output. State is [JSON on disk](docs/humans/how-it-works.md#distributed-state). The agent decides _what code to write_. [Scripts](docs/humans/cli.md#commands) enforce correctness. You can [stop the daemon](docs/humans/cli/stop.md), check on progress with [`wolfcastle status`](docs/humans/cli/status.md), rearrange things by hand, and restart. Wolfcastle [picks up exactly where it left off](docs/humans/failure-and-recovery.md#self-healing).

## The Project Tree

Code changes live in a [tree of two node types](docs/humans/how-it-works.md#the-project-tree): orchestrators (containers that represent features, modules, or milestones) and leaves (where the actual coding tasks live). Orchestrators hold other orchestrators or leaves; leaves hold an ordered list of tasks. Every leaf ends with an automatic [audit task](docs/humans/audits.md#the-audit-system) that verifies the code.

Nodes have [four states](docs/humans/how-it-works.md#four-states): `not_started`, `in_progress`, `complete`, `blocked`. State [propagates upward](docs/humans/how-it-works.md#state-propagation) deterministically. When a coding task completes, its leaf recomputes, then its parent, all the way to the root. No node sets its own state. State is a consequence of the code below it.

## Configuration

Config [merges across three tiers](docs/humans/configuration.md#three-tiers): base (Wolfcastle defaults, gitignored), custom (team-shared, committed), and local (personal, gitignored). Higher tiers override lower ones. JSON objects deep-merge; arrays replace entirely; set a field to `null` to delete it.

Manage configuration through the CLI: [`wolfcastle config show`](docs/humans/cli/config-show.md) displays the merged result, while [`config set`](docs/humans/cli/config-set.md), [`config unset`](docs/humans/cli/config-unset.md), [`config append`](docs/humans/cli/config-append.md), and [`config remove`](docs/humans/cli/config-remove.md) modify individual values in any tier.

[Agents](docs/humans/configuration.md#models) are defined as CLI commands. Anything that reads stdin and writes stdout works: Claude Code, Cursor, Copilot, GPT, Gemini, Llama, a bash script that echoes "done." Your agents, your choice. Switch providers by editing a JSON file.

## Requirements

- **Git** (branch verification, progress detection, auto-commit)
- **A coding agent** that reads stdin and writes stdout. [Claude Code](https://claude.com/claude-code) is the default. Anything that accepts a prompt and produces code works.
- **Go 1.26+** (only if building from source)
- **Local filesystem** for `.wolfcastle/`. Wolfcastle uses `flock(2)` advisory locking; network-mounted filesystems don't honor it. Keep `.wolfcastle/` on a local disk.

## Collaboration

Each engineer's project tree lives in its own [namespace](docs/humans/collaboration.md#engineer-namespacing). Everyone can see everyone else's work, but nobody writes to anyone else's state. No merge conflicts. No coordination overhead. An optional [overlap advisory](docs/humans/collaboration.md#overlap-advisory) warns you when your new project's scope collides with someone else's active work.

Wolfcastle [commits code to your current branch](docs/humans/collaboration.md#git-integration) with branch safety checks on every commit. For isolation, [`--worktree`](docs/humans/collaboration.md#worktree-isolation) runs all work in a separate git worktree. Completed projects are [auto-archived](docs/humans/collaboration.md#archive) after a configurable delay.

## CLI

[53 commands](docs/humans/cli.md#commands) across lifecycle, task management, auditing, diagnostics, and documentation. Every command accepts `--json` for structured output. Every command that operates on a node accepts `--node` with a slash-separated [tree address](docs/humans/cli.md#tree-addressing).

## Design Documents

The [Architecture Decision Records](docs/decisions/INDEX.md) document every major design choice. The [Specifications](docs/specs/) provide detailed formal specs for the state machine, configuration system, pipelines, CLI surface, and validation engine. For developer and agent guides, see [AGENTS.md](AGENTS.md) and [docs/agents/](docs/agents/).

<p align="center">
  <img src="assets/neon-wolf.png" width="150" />
</p>
