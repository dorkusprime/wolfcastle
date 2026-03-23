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

Pre-release. Probably broken. Then fixed. Then broken again. On repeat until release. Install if you're feeling brave:

```bash
brew install dorkusprime/tap/wolfcastle
cd your-repo

wolfcastle init                                            # scaffold .wolfcastle/
wolfcastle inbox add "Build a website for my donut stand"  # queue up work
wolfcastle start                                           # the daemon wakes up

# in another terminal, same directory::
wolfcastle status -w                                       # watch it work
```

`init` creates the `.wolfcastle/` directory with config, prompts, and state scaffolding. `inbox add` queues a feature request for the daemon to decompose into projects and tasks. `start` launches the daemon, which runs the full pipeline until there's nothing left to do. `status` shows you what's happening (and `-w` keeps on showing it to you). Everything is [configurable](#configuration).

### Daemon Mode

You can also run Wolfcastle in the background and just let it go. Give it work as you find work for it to do. No LLM calls are made if there's no work to be done:

```bash
brew install dorkusprime/tap/wolfcastle
cd your-repo

wolfcastle init                                            # scaffold .wolfcastle/
wolfcastle start -d                                        # the daemon wakes up

# Whenever you get to it
wolfcastle inbox add "Build a website for my donut stand"  # queue up work

# in another terminal, same directory:
wolfcastle status -w                                       # watch it work
```

Feed it work through the [CLI](docs/humans/cli.md) or chat with your coding agent to hammer out the details and have it inject the work directly with a markdown file path. A detailed spec or PRD gets the most predictable results, but Wolfcastle takes vague directions too without getting lost in the wrong state like a second lieutenant. "Make the API faster" becomes specs, task trees, and acceptance criteria before any code gets written. The [daemon](docs/humans/how-it-works.md#the-daemon) takes it from there: triage, planning, execution, audit, commit. Serial, depth-first, until the [tree](docs/humans/how-it-works.md#the-project-tree) is conquered or something insurmountable gets in the way.

## Why This Exists

Coding agents are remarkably good at _writing_ specs and _following_ specs. Where they fall apart is the middle: deciding what to build, building it, and maintaining quality across dozens of tasks without human intervention. The models and their coding harnesses are capable. What's missing is the scaffolding around them: the structure that lets them work for hours without a human in the loop. Four problems kill autonomous agents in practice.

### Planning

Coding agents work best when they know exactly what to do. The problem is getting from a goal to a plan. Some agents offer a "planning mode" to bridge this gap, but the agent is still both planner and executor. _Keep_ context and the two blur together: the agent plans five things, does two, gets creative on the third, and forgets the fourth. _Clear_ context and the executor can reinterpret the plan because nothing enforces it. Either way, the plan is a suggestion the agent made to itself.

Wolfcastle separates planning from execution. A planning agent creates the structure: [specs](docs/specs/), projects, task trees. An execution agent follows it. Neither can overwrite the other. The [daemon](docs/humans/how-it-works.md#the-daemon) enforces the contract. If the executor blocks, the system [decomposes](docs/humans/failure-and-recovery.md#decomposition) the problem into smaller, weaker targets and destroys those instead.

### Context

Context degrades over time: windows fill up, conversations get compacted, and earlier decisions contradict later ones. When context contradicts itself, the model reconciles those conflicts on every turn, burning capacity on bookkeeping instead of work. Decisions evaporate, lessons from failures disappear, and the agent runs in circles solving the same problems over and over.

Wolfcastle gives each task a fresh invocation with clean context and persists knowledge as artifacts on disk. [Architecture Decision Records](docs/decisions/INDEX.md) (ADRs) capture the "why" behind design choices so the next agent doesn't reverse a deliberate decision. [Specifications](docs/specs/) give executors a contract instead of re-interpreting the goal each time. [After Action Reviews](docs/humans/audits.md) (AARs) capture lessons learned so mistakes die the first time. [Codebase knowledge files](docs/humans/how-it-works.md#codebase-knowledge-files) accumulate informal observations (build quirks, hidden dependencies, undocumented conventions) that grow across tasks, giving each new agent the institutional memory that none of the formal artifacts capture. The daemon injects all of these into context automatically: each invocation starts clean but informed.

### Quality

Agents produce code, and nobody checks it. Or worse, the agent checks its own work in the same context where it wrote the code: the author proofreading their own novel. Errors of assumption survive because the assumptions were never questioned by a separate process. The code compiles, the tests pass because they were written to pass, and the architecture quietly rots.

Wolfcastle [audits](docs/humans/audits.md#the-audit-system) every piece of work from a separate invocation with fresh context. Audits are hierarchical: each [leaf](docs/humans/how-it-works.md#the-project-tree) gets its own audit scoped to what it built, each [orchestrator](docs/humans/how-it-works.md#the-project-tree) gets an audit scoped to how its children integrate. The auditor reads [breadcrumbs](docs/humans/audits.md#breadcrumbs) (timestamped records of what each task produced), checks them against acceptance criteria, and files gaps. Gaps that can't be resolved locally [escalate upward](docs/humans/audits.md#gap-escalation). No code ships without a second pair of eyes, and those eyes belong to a process that didn't write the code.

### Human Dependency

Most agent workflows assume someone is always watching: approving plans, reviewing code, deciding what to do next. The agent is capable, but the process treats it like an intern. Scale hits the human's availability long before it hits the agent's throughput.

Wolfcastle runs the full lifecycle without waiting for a human: [intake](docs/humans/how-it-works.md#getting-work-in), [planning](docs/humans/how-it-works.md#the-pipeline), [execution](docs/humans/how-it-works.md#execution-protocol), [audit](docs/humans/audits.md#the-audit-system), [remediation](docs/humans/audits.md#gap-escalation), commit. Your role is to check [`wolfcastle status`](docs/humans/cli/status.md), adjust priorities, and [unblock](docs/humans/failure-and-recovery.md#the-unblock-workflow) the rare task that genuinely needs judgment. The system does the volume. You do the thinking.

## Standing on the Shoulders of . . . Ralph Wiggum

The [Ralph loop](https://ghuntley.com/ralph/) changed how people think about coding agents. Put an agent in a `while` loop, feed it a spec, and let it run: each iteration does one thing with fresh context, and backpressure from tests, compilation, and linting keeps the output honest. Simple, effective, and [widely adopted](https://ghuntley.com/loop/), though [not without rough edges](https://news.ycombinator.com/item?id=46750937): [context rot](https://www.alibabacloud.com/blog/from-react-to-ralph-loop-a-continuous-iteration-paradigm-for-ai-agents_602799) degrades output as windows fill, [placeholder implementations](https://www.aihero.dev/tips-for-ai-coding-with-ralph-wiggum) slip through when the model chases compilation over correctness, and [architectural drift](http://www.zerosync.co/blog/ralph-loop-technical-deep-dive) accumulates across iterations without independent review.

Ralph works because it respects a fundamental constraint: context degrades. The specs get re-read every pass, but they're read _clean_, with no competing noise from failed attempts or abandoned approaches. The signal-to-noise ratio is what matters.

Wolfcastle inherits these ideas: fresh context per task, backpressure validation, one thing at a time. It also pushes further on a principle Ralph hints at: don't make the model do work that doesn't require a model. Finding the next task, grooming a backlog, propagating state, validating structure: these are deterministic operations. Asking a non-deterministic agent to perform them reliably is a recipe for a soup sandwich. Wolfcastle handles all of it with [CLI scripts](docs/humans/cli.md) and daemon logic. The agent reads code and writes code. Everything else is handled by Go binaries that never hallucinate, never drift, and never burn tokens on bookkeeping.

The difference is what sits around the core loop: planning agents that decide what to work on next, structured artifacts that carry knowledge forward without static files that grow until they hit the context ceiling, hierarchical audits that catch what compile-test-lint cannot, and a daemon that runs the loop without a human at the keyboard.

Ralph is the insight. Wolfcastle is the infrastructure.

## How It Works

Wolfcastle organizes code changes into a [project tree](docs/humans/how-it-works.md#the-project-tree). Orchestrators represent features, modules, or milestones. Leaves are where the actual coding tasks live. Every node ends with an automatic [audit](docs/humans/audits.md#the-audit-system) and, if gaps are found, [remediation](docs/humans/audits.md#gap-escalation). Nodes have [four states](docs/humans/how-it-works.md#four-states): `not_started`, `in_progress`, `complete`, `blocked`. State [propagates upward](docs/humans/how-it-works.md#state-propagation) deterministically: when a task completes, its leaf recomputes, then its parent, all the way to the root. No node decides its own fate. Insubordination is not a valid state.

The daemon hunts down the next task and eliminates it, running a [pipeline of stages](docs/humans/how-it-works.md#the-pipeline): intake triages inbox items into projects, planning agents decompose projects into specs and task trees, execution claims tasks and points agents at code, audits verify the results from a separate invocation with fresh context. Each stage is a separate model call with a specific role. Stages are [configured as a dictionary](docs/humans/configuration.md#pipeline-stages), and each one can be overridden individually across [config tiers](docs/humans/configuration.md#three-tier-directory-structure) (base, custom, local) without replacing the entire pipeline.

The tree grows at runtime. Tasks that fail [decompose](docs/humans/failure-and-recovery.md#decomposition) into smaller, weaker problems. Agents can trigger decomposition mid-task when they recognize the work is too broad. Orchestrators spawn new children when planning reveals additional work. A [hard cap](docs/humans/failure-and-recovery.md#task-failure-escalation) stops any task from fighting forever. If the machine goes down mid-task, the daemon [recovers on restart](docs/humans/failure-and-recovery.md#self-healing) and resumes from the interrupted task. Completed trees are [auto-archived](docs/humans/collaboration.md#archive). It does not waste time on the fallen.

Everything is deterministic except the model's output. State, specs, ADRs, AARs, breadcrumbs, and audit findings all live as [files on disk](docs/humans/how-it-works.md#distributed-state), tracked in version control. Nothing important stays in memory. Every decision, every lesson, every result persists across invocations, restarts, and crashes. The model handles what models are good at: reading code, writing code, making judgment calls. [CLI scripts](docs/humans/cli.md) handle what's deterministic: state transitions, propagation, validation, navigation. Neither does the other's job.

## Configuration

Three tiers: Base, custom, local. [Higher tiers override lower ones](docs/humans/configuration.md#three-tier-directory-structure). New to configuration? Start with the [quickstart](docs/humans/config-quickstart.md). Base ships with the release and gets regenerated by `wolfcastle init`. Custom is committed to the repo and shared with your team. Local is gitignored, yours alone, for personal overrides. JSON objects deep-merge; arrays replace entirely; set a field to `null` to eliminate it. Configuration is not a democracy. Manage it through [`wolfcastle config`](docs/humans/cli/config-show.md) commands or edit the JSON directly.

[Agents](docs/humans/configuration.md#model-configuration) are defined as CLI commands. Anything that reads stdin and writes stdout works: Claude Code, Cursor, Copilot, GPT, Gemini, Llama, a bash script wrapping `curl` Your agents, your choice. Switch providers by editing a JSON file.

## Collaboration

Each engineer's work lives in its own [namespace](docs/humans/collaboration.md#engineer-namespacing). Everyone can see everyone else's state, but nobody writes to anyone else's. No merge conflicts. No coordination overhead. The [daemon commits deterministically](docs/humans/collaboration.md#daemon-side-commits) after each task, including partial work on failure, so nothing is lost. Agents never touch git. Run in an [isolated worktree](docs/humans/collaboration.md#worktree-isolation) if you prefer.

## Token Usage

Wolfcastle turns tokens into code. It uses a lot of them. Every planning pass, every execution, every audit, every remediation is a separate model invocation. The scaffolding makes each invocation more efficient: persistent artifacts mean less re-discovery, CLI scripts handle state instead of the model reasoning through it, and deterministic operations like propagation and validation never touch the model at all. But the volume is real. If you're using a metered API, expect meaningful spend. An unlimited plan (like Anthropic's Max plan for Claude Code) is the practical choice for sustained use.

## Requirements

- **Git** (branch verification, progress detection, auto-commit)
- **A coding agent** that reads stdin and writes stdout
- **Local filesystem** for `.wolfcastle/` ([why](SECURITY.md#filesystem-requirements))

## More

- [59 CLI commands](docs/humans/cli.md)
- [Architecture Decision Records](docs/decisions/INDEX.md) (94 and counting)
- [Specifications](docs/specs/)
- [Developer guides](docs/agents/)
- [AGENTS.md](AGENTS.md)

<p align="center">
  <img src="assets/neon-wolf.png" width="150" />
</p>
