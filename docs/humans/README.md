# Wolfcastle Documentation

The field manual. Everything you need to operate the weapon.

Start with the [TUI Guide](the-tui.md) to learn the interface. Start with [How It Works](how-it-works.md) if you want the full picture.

## [TUI Guide](the-tui.md)

The primary interface. Launching, navigation, keybindings, and every screen explained. Bare `wolfcastle` opens the TUI; everything the daemon does is visible here in real time.

## [How It Works](how-it-works.md)

The core of the machine. Work enters through your coding agent, the inbox, or direct CLI commands. It lands in a project tree built from orchestrator nodes (containers) and leaf nodes (where tasks live). A daemon runs a pipeline of stages in a loop, each invoking a model with a specific role. When a model executes a task, it follows a nine-phase protocol and communicates only through deterministic script calls. State is JSON on disk, propagates upward from children to parents, and every mutation is atomic.

## [Configuration Quickstart](config-quickstart.md)

Change your model, add a stage, override a prompt, or set a task class in under a minute.

## [Configuration](configuration.md)

Config merges across three tiers: base (Wolfcastle defaults), custom (team-shared), and local (personal). Models are defined as CLI commands, so any provider that reads stdin and writes stdout works. The same three-tier system governs pipelines, prompt templates, rule fragments, audit scopes, and security boundaries.

## [Config Reference](config-reference.md)

Every configuration field: type, default, description, and example. The "I need to know what `daemon.stall_timeout_seconds` does" page.

## [Task Classes](task-classes.md)

Behavioral prompts that shape how the agent approaches work. Built-in language and framework classes, discipline classes, and how to create your own.

## [Failure and Recovery](failure-and-recovery.md)

Tasks that fail get retried. After 10 failures, Wolfcastle decomposes the task into smaller sub-tasks. Decomposition can recurse up to a configurable depth, and a hard cap prevents infinite iteration. The daemon self-heals after crashes. Blocked tasks can be unblocked three ways: a zero-cost status flip, an interactive model-assisted session, or a structured context dump for an external agent.

## [Audits and Quality](audits.md)

Every leaf gets verified by an automatic audit task that reviews breadcrumbs against defined criteria. Gaps escalate upward through the tree. A standalone `wolfcastle audit` command runs read-only codebase analysis against composable scopes, with an approval gate before findings become tasks. `wolfcastle doctor` validates and repairs the state tree itself.

## [Parallel Execution](parallel-execution.md)

Run multiple tasks concurrently with file-level scope locks. Configure the worker pool, monitor active workers and scope conflicts through the TUI dashboard or `wolfcastle status`, and diagnose contention when tasks keep yielding on the same files.

## [Collaboration](collaboration.md)

Each engineer gets their own namespace under `.wolfcastle/system/projects/`. No merge conflicts, no coordination overhead. Wolfcastle commits to your current branch with safety checks, or isolates work in a separate git worktree. Completed projects move to the archive. Everything is tracked in structured NDJSON logs.

## [CLI Reference](cli.md)

63 commands for scripting, automation, and agent integration. Every command accepts `--json` and `--node` for tree addressing. Covers the inbox, installation channels, project directory layout, and new engineer setup.
