# Wolfcastle Documentation

The field manual. Everything you need to operate the weapon.

Start with [How It Works](how-it-works.md) if you want the full picture. Start with the [CLI Reference](cli.md) if you want to hit things.

## [How It Works](how-it-works.md)

The core of the machine. Work enters through your coding agent, the inbox, or direct CLI commands. It lands in a project tree built from orchestrator nodes (containers) and leaf nodes (where tasks live). A daemon runs a pipeline of stages in a loop, each invoking a model with a specific role. When a model executes a task, it follows a seven-phase protocol and communicates only through deterministic script calls. State is JSON on disk, propagates upward from children to parents, and every mutation is atomic.

## [Configuration](configuration.md)

Config merges across three tiers: base (Wolfcastle defaults), custom (team-shared), and local (personal). Models are defined as CLI commands, so any provider that reads stdin and writes stdout works. The same three-tier system governs pipelines, prompt templates, rule fragments, audit scopes, and security boundaries.

## [Failure and Recovery](failure-and-recovery.md)

Tasks that fail get retried. After 10 failures, Wolfcastle decomposes the task into smaller sub-tasks. Decomposition can recurse up to a configurable depth, and a hard cap prevents infinite iteration. The daemon self-heals after crashes. Blocked tasks can be unblocked three ways: a zero-cost status flip, an interactive model-assisted session, or a structured context dump for an external agent.

## [Audits and Quality](audits.md)

Every leaf gets verified by an automatic audit task that reviews breadcrumbs against defined criteria. Gaps escalate upward through the tree. A standalone `wolfcastle audit` command runs read-only codebase analysis against composable scopes, with an approval gate before findings become tasks. `wolfcastle doctor` validates and repairs the state tree itself.

## [Collaboration](collaboration.md)

Each engineer gets their own namespace under `.wolfcastle/system/projects/`. No merge conflicts, no coordination overhead. Wolfcastle commits to your current branch with safety checks, or isolates work in a separate git worktree. Completed projects move to the archive. Everything is tracked in structured NDJSON logs.

## [CLI Reference](cli.md)

39 commands across lifecycle, task management, auditing, diagnostics, and documentation. Every command accepts `--json` and `--node` for tree addressing. Covers the inbox, installation channels, project directory layout, and new engineer setup.
