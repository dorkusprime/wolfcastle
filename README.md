# Wolfcastle

![Wolfcastle](assets/wolfcastle-alpha.png)

**You have a codebase. _Wolfcastle will build it._**

You give Wolfcastle a goal. It breaks that goal into a tree of targets and sends AI coding agents to destroy them, depth-first, until the tree is conquered. While you do whatever it is you do.

## Why This Exists

Coding agents are remarkably good at writing specs and following specs. Where they struggle is the middle: deciding what to build, building it, and maintaining quality across dozens of tasks without human intervention. Not because the models aren't capable, but because the scaffolding around them wasn't designed for sustained, unsupervised work.

### The Planning Problem

Some coding agents offer a "planning mode" that asks the agent to think before it acts. But the agent is both the planner and the executor. Keep context and the two blur together: the agent plans five things, does two, gets creative on the third, and forgets the fourth. Clear context and the executor can reinterpret or ignore the plan because nothing enforces it. Either way, the plan is a suggestion the agent made to itself.

Wolfcastle separates planning from execution. A planning agent creates the structure: specs, projects, task trees. An execution agent follows it. Neither can overwrite the other. The daemon enforces the contract. If the executor blocks, the system [decomposes](docs/humans/failure-and-recovery.md#decomposition) rather than letting it thrash.

### The Context Problem

Context degrades. Windows fill up, conversations get compacted, and earlier decisions contradict later ones. When context contains contradictions, the model has to resolve those conflicts on every single turn, burning capacity on reconciliation instead of work. The agent that spent twenty minutes understanding your authentication flow loses the nuance by the time the next task begins. Decisions evaporate. Lessons from failures disappear. This is the [Ralph loop](https://ghuntley.com/loop/): the agent running in circles, solving the same problems over and over, because it has no memory of what it already learned.

Wolfcastle materializes context as artifacts. [Architecture Decision Records](docs/decisions/INDEX.md) capture the "why" behind design choices, so the next agent doesn't reverse a deliberate decision. [Specifications](docs/specs/) capture the "what," giving executors a contract instead of re-interpreting the goal each time. [After Action Reviews](docs/humans/audits.md) capture the "what happened," so mistakes don't repeat. The daemon injects these into context automatically.

### The Quality Problem

Agents produce code. Nobody checks it. Or worse, the agent checks its own work in the same context where it wrote the code. Errors of assumption survive because the assumptions were never questioned by a separate process.

Wolfcastle [audits](docs/humans/audits.md#the-audit-system) every piece of work from a separate invocation with fresh context. Audits are hierarchical: each [leaf](docs/humans/how-it-works.md#the-project-tree) gets its own audit scoped to what that leaf built, and each orchestrator gets an audit scoped to how its children integrate. The auditor reads [breadcrumbs](docs/humans/audits.md#breadcrumbs), checks them against acceptance criteria, and files gaps. Gaps that can't be resolved locally [escalate upward](docs/humans/audits.md#gap-escalation) to the parent, and escalation can propagate to the root. No code ships without a second pair of eyes, and those eyes belong to a process that didn't write the code.

### The Human Dependency Problem

Most agent workflows require a human at every turn. Approve this plan. Review this code. Decide what to do next. The agent is capable, but the process assumes someone is always watching. Scale is limited by the human's availability, not the agent's throughput.

Wolfcastle inverts the relationship. The daemon runs the full lifecycle: intake, planning, execution, audit, remediation, commit. The human's role is to steer, not operate. Check [`wolfcastle status`](docs/humans/cli/status.md), adjust priorities, [unblock](docs/humans/failure-and-recovery.md#the-unblock-workflow) the rare task that genuinely needs judgment. The system does the volume. You do the thinking.

## Status

[![CI](https://github.com/dorkusprime/wolfcastle/actions/workflows/ci.yml/badge.svg?branch=main)](https://github.com/dorkusprime/wolfcastle/actions/workflows/ci.yml)
[![CodeQL](https://github.com/dorkusprime/wolfcastle/actions/workflows/codeql.yml/badge.svg)](https://github.com/dorkusprime/wolfcastle/actions/workflows/codeql.yml)
[![codecov](https://codecov.io/gh/dorkusprime/wolfcastle/branch/main/graph/badge.svg)](https://codecov.io/gh/dorkusprime/wolfcastle)
[![Go Report Card](https://goreportcard.com/badge/github.com/dorkusprime/wolfcastle)](https://goreportcard.com/report/github.com/dorkusprime/wolfcastle)
[![Go Reference](https://pkg.go.dev/badge/github.com/dorkusprime/wolfcastle.svg)](https://pkg.go.dev/github.com/dorkusprime/wolfcastle)
[![Go version](https://img.shields.io/github/go-mod/go-version/dorkusprime/wolfcastle)](https://go.dev/)
[![License: MIT](https://img.shields.io/badge/license-MIT-blue.svg)](LICENSE)
[![Release](https://img.shields.io/github/v/release/dorkusprime/wolfcastle?include_prereleases)](https://github.com/dorkusprime/wolfcastle/releases)

Pre-release. Probably broken. Then fixed. Then broken again. On repeat until release. Install if you're feeling brave:

```bash
brew install dorkusprime/tap/wolfcastle
cd your-repo
wolfcastle init                                            # scaffold .wolfcastle/
wolfcastle inbox add "Build a website for my donut stand"  # queue up work
wolfcastle start                                           # the daemon wakes up

# in another terminal:
wolfcastle status -w                                       # watch it work
```

`init` creates the `.wolfcastle/` directory with config, prompts, and state scaffolding. `inbox add` queues a feature request for the daemon to decompose into projects and tasks. `start` launches the daemon, which runs the full pipeline until there's nothing left to do. `status` shows you what's happening.

Feed it work through the [CLI](docs/humans/cli.md) or let your coding agent inject work directly. The [daemon](docs/humans/how-it-works.md#the-daemon) takes it from there: triage, planning, execution, audit, commit. Serial, depth-first, until the [tree](docs/humans/how-it-works.md#the-project-tree) is conquered or something gets in the way.

## How It Works

Wolfcastle organizes code changes into a [tree](docs/humans/how-it-works.md#the-project-tree) of orchestrators (features, modules, milestones) and leaves (where the coding happens). Every node ends with an automatic audit and, if necessary, remediation. Nodes have [four states](docs/humans/how-it-works.md#four-states): `not_started`, `in_progress`, `complete`, `blocked`. State [propagates upward](docs/humans/how-it-works.md#state-propagation) deterministically. No node decides its own fate.

The daemon runs a [pipeline of stages](docs/humans/how-it-works.md#the-pipeline). Intake triages inbox items into projects. Execution claims tasks and points agents at code. Audits verify the results. Each stage is a separate model invocation with a specific role. Stages are [configured as a dictionary](docs/humans/configuration.md#pipelines), overridable per tier.

The tree grows at runtime. Tasks that fail [decompose](docs/humans/failure-and-recovery.md#decomposition) into smaller problems. Agents can trigger decomposition mid-task when they recognize the work is too broad. Orchestrators spawn new children when planning reveals additional work. A [hard cap](docs/humans/failure-and-recovery.md#task-failure-escalation) stops any task from fighting forever. If the daemon crashes, it [recovers on restart](docs/humans/failure-and-recovery.md#self-healing). Completed trees are [auto-archived](docs/humans/collaboration.md#archive).

Everything is deterministic except the model's output. State, specs, ADRs, AARs, breadcrumbs, and audit findings all live as [files on disk](docs/humans/how-it-works.md#distributed-state). Nothing important stays in memory. Every decision, every lesson, every result persists across invocations, restarts, and crashes.

## Configuration

Config [merges across three tiers](docs/humans/configuration.md#three-tiers): base (defaults, gitignored), custom (team-shared, committed), local (personal, gitignored). Manage it through [`wolfcastle config`](docs/humans/cli/config-show.md) commands or edit the JSON directly.

[Agents](docs/humans/configuration.md#models) are defined as CLI commands. Anything that reads stdin and writes stdout works: Claude Code, Cursor, Copilot, GPT, Gemini, Llama, a bash script that echoes "done." Your agents, your choice.

## Collaboration

Each engineer's work lives in its own [namespace](docs/humans/collaboration.md#engineer-namespacing). Everyone can see everyone else's state, but nobody writes to anyone else's. No merge conflicts. Wolfcastle [commits to your current branch](docs/humans/collaboration.md#git-integration) with safety checks, or to an [isolated worktree](docs/humans/collaboration.md#worktree-isolation) if you prefer.

## Requirements

- **Git** (branch verification, progress detection, auto-commit)
- **A coding agent** that reads stdin and writes stdout
- **Go 1.26+** (only if building from source)
- **Local filesystem** for `.wolfcastle/` ([why](SECURITY.md))

## Token Usage

Wolfcastle turns tokens into code. It uses a lot of them. Every planning pass, every execution, every audit, every remediation is a separate model invocation. The scaffolding makes each invocation more efficient (persistent context means less re-discovery, CLI scripts handle state instead of the model reasoning through it, deterministic operations like propagation and validation never touch the model at all), but the volume is real. If you're using a metered API, expect meaningful spend. An unlimited plan (like Anthropic's Max plan for Claude Code) is the practical choice for sustained use.

## More

- [53 CLI commands](docs/humans/cli.md)
- [Architecture Decision Records](docs/decisions/INDEX.md) (79 and counting)
- [Specifications](docs/specs/)
- [Developer guides](docs/agents/)
- [AGENTS.md](AGENTS.md)

<p align="center">
  <img src="assets/neon-wolf.png" width="150" />
</p>
