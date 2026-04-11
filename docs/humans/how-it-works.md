# How It Works

## Getting Work In

The most common path: you tell your coding agent what you want done. "Add OAuth2 PKCE support to the auth service." You go back and forth with the agent, refining scope, adding constraints, clarifying how you want things structured, until the plan feels right. Then the agent uses Wolfcastle's [CLI](cli.md) to inject the work, decompose it into tasks, and start [the daemon](#the-daemon). Wolfcastle handles the orchestration from there.

When you want direct control, the CLI has you covered:

**The inbox** is for quick capture. `wolfcastle inbox add "support OAuth2 PKCE"` drops an item into a queue. The daemon's [intake stage](#the-pipeline) picks it up in a background goroutine, uses a model to create projects and tasks directly in the tree. You throw an idea at Wolfcastle. It figures out the rest. ([More on the inbox.](cli.md#the-inbox))

**Project creation** is for structured planning. `wolfcastle project create --node backend/auth` creates an orchestrator or leaf node at a specific point in the tree. You define the shape of the work; the daemon fills in the tasks, or you add them manually.

**Task addition** is for precision. `wolfcastle task add --node backend/auth/session-tokens "Implement token rotation"` places a single task exactly where you want it. No model involved. No decomposition. You know what needs doing and where it belongs.

All of these compose freely. Your agent might create a project, you might add a few tasks by hand, then drop something in the inbox while the daemon is running. The tree accepts work from either direction.

## The Project Tree

Work is organized as a tree. Two node types. No depth limit.

**Orchestrator nodes** contain child nodes (other orchestrators or leaves). Their state is computed from their children. You do not set it. You do not touch it. The children report upward and the orchestrator obeys the math.

**Leaf nodes** contain an ordered list of tasks. The last task in every leaf is an [audit task](audits.md#the-audit-system), auto-created, immovable, non-negotiable. Every piece of work gets verified.

Orchestrators can contain orchestrators. Those can contain more orchestrators. The tree goes as deep as the work demands.

```
goal/                       <- orchestrator (root)
  backend/                  <- orchestrator
    auth/                   <- orchestrator
      session-tokens/       <- leaf: tasks live here
      oauth-provider/       <- leaf
    database/               <- orchestrator
      migrations/           <- leaf
      connection-pool/      <- leaf
  frontend/                 <- orchestrator
    login-flow/             <- orchestrator
      form-validation/      <- leaf
      error-states/         <- leaf
```

Traversal is depth-first. Top-to-bottom, left-to-right, one task at a time. One target. One model.

### Hierarchical Task IDs

Tasks can decompose into subtasks. `task-0001` can spawn `task-0001.0001`, `task-0001.0002`, and so on. When a task has children, its status derives from them: all children complete means the parent completes. This nesting goes as deep as needed. The ID structure mirrors the hierarchy; the dots tell you lineage at a glance.

## Four States

Every node and task has exactly one state.

| State         | Meaning                                |
| ------------- | -------------------------------------- |
| `not_started` | Waiting. Its time will come.           |
| `in_progress` | Under attack.                          |
| `complete`    | Destroyed. Terminal. Never comes back. |
| `blocked`     | Cannot proceed. Waiting for a human.   |

There is no `failed`. There is no `cancelled`. There is no `paused`. Work that cannot continue is blocked. Work that is done is complete. Everything else is in progress or waiting. (Blocked tasks have [three ways out](failure-and-recovery.md#the-unblock-workflow).)

### State Propagation

State flows upward. Only upward. When a task completes, its leaf recomputes. When a leaf completes, its parent orchestrator recomputes. This continues to the root.

- All children not started: parent is not started
- All children complete: parent is complete
- All non-complete children blocked: parent is blocked
- Anything else: parent is in progress

No node sets its own state. State is a consequence of the work below it.

## Lazy Planning

The daemon executes first. Planning only fires when navigation finds no actionable tasks in a subtree. Each orchestrator gets planned right before its subtree needs work, not before. This keeps the tree lean: you never have a plan for work the daemon hasn't reached yet, and replanning happens naturally when the shape of the work changes.

## Orchestrator Audits

Every node gets an audit task, not just leaves. Orchestrator audits are deferred until all children complete. Once the last child finishes, the orchestrator's audit fires and verifies the subtree as a whole. This catches integration gaps that leaf-level audits miss.

## Remediation

When an audit finds gaps, the audit task moves to BLOCKED and the block propagates upward to the root index. The parent orchestrator sees the block, triggers re-planning, and creates remediation tasks to address the gaps. Those tasks execute, the fixes land, and the audit re-runs. This cycle repeats until the audit passes or the problem escalates to a human.

## Spec Review

When a spec task completes, the daemon automatically creates a review task. The review checks the spec for logical gaps, missing method signatures, contradictions, and unclear requirements before implementation begins. If the review finds issues, it blocks and feeds the feedback back to the original spec task, which re-enters the work queue for revision. This loop repeats until the spec passes review. Review tasks are inserted before the audit task so they run in the natural execution order.

## Orchestrator Reconciliation

At the start of each daemon iteration, the daemon reconciles every orchestrator's persisted state against its children. If a parent says "in progress" but all its children are complete, the parent gets corrected. This handles edge cases where state mutations from a previous iteration (or a crash) left the tree inconsistent. The reconciliation runs before planning or navigation, so the daemon always operates on an accurate picture.

## Auto-Archive

When a root-level project completes, the daemon can automatically archive it. Auto-archive runs inline in the main daemon loop ([Architecture Decision Record](../decisions/INDEX.md) ADR-064), after the execute and planning stages find nothing to do. The daemon polls for eligible nodes at a configurable interval, and each completed project must sit idle for a configurable delay (default 24 hours) before archival triggers. One node is archived per poll cycle, generating an archive rollup entry and moving the project tree to archived state. Archived nodes can be restored or permanently deleted via the CLI. The feature is enabled by default; disable it in [configuration](config-reference.md#archive):

```json
{
  "archive": {
    "auto_archive_enabled": true,
    "auto_archive_delay_hours": 24,
    "archive_poll_interval_seconds": 300
  }
}
```

## The Daemon

`wolfcastle start` launches the daemon. It owns the pipeline loop.

```
wolfcastle start                          # foreground
wolfcastle start -d                       # background
wolfcastle start --node backend/auth      # scoped to a subtree
wolfcastle start --worktree feature/auth  # isolated git worktree
```

Each iteration walks the [configured pipeline stages](configuration.md#pipeline-stages), invokes [models](configuration.md#model-configuration), and advances the tree. Between iterations, it checks for stop signals. On SIGTERM or SIGINT, it finishes the current stage, cleans up child processes, and shuts down. It does not leave a mess.

### The Pipeline

The daemon runs a pipeline of stages. Each stage invokes a model with a specific role. The default:

| Stage       | Model Tier | Mission                                                   |
| ----------- | ---------- | --------------------------------------------------------- |
| **intake**  | mid        | Reads the inbox. Creates projects and tasks directly.     |
| **execute** | capable    | Claims a task. Does the work. Writes code. Makes commits. |

The intake stage runs in a parallel background goroutine, polling for new inbox items independently of the main execution loop. The execute stage runs in the main loop.

Summaries are generated inline during execution via the `WOLFCASTLE_SUMMARY:` marker (ADR-036), not as a separate stage.

Stages do not pass output to each other. They read the current state of the world and act on it. The intake stage creates projects and tasks. The execute stage finds them. No coupling. No handoffs. Just [state on disk](#distributed-state) and models that know how to read it.

### Execution Protocol

When the execute stage claims a task, the model follows a ten-phase protocol:

1. **Claim** the task.
2. **Study** the project description, [specs](collaboration.md#specs), [breadcrumbs](audits.md#breadcrumbs), and linked context.
3. **Implement** the work.
4. **Validate** by running configured checks (tests, lints, builds).
5. **Record** [breadcrumbs](audits.md#breadcrumbs) describing what was done and why.
6. **Document** decisions (ADRs) and contracts (specs).
7. **Capture** codebase knowledge files for future context.
8. **Signal** the outcome: COMPLETE, YIELD (needs another iteration), or BLOCKED.
9. **Pre-block** downstream tasks that cannot succeed, when applicable.
10. **Create follow-up tasks** based on findings, when applicable.

The model communicates through [script calls](cli.md): `wolfcastle task claim`, `wolfcastle audit breadcrumb`, `wolfcastle task complete`. Every side effect goes through a deterministic command that validates inputs and enforces invariants. The model cannot corrupt the tree. It can only ask the scripts to make valid changes.

#### Daemon Commits

The agent never runs git commands. After each iteration, the daemon commits the changes on the agent's behalf. This happens on both success and failure: a completed task produces a commit with the message `wolfcastle: <task-id> complete`, while a failed or yielded task produces `wolfcastle: <task-id> partial (attempt N)`. Every iteration leaves a commit in the history, so progress is never lost and the timeline of work is fully recoverable.

Four [configuration fields](config-reference.md#git) control this behavior. `auto_commit` is the master switch; when `false`, the daemon writes no commits at all. `commit_on_success` and `commit_on_failure` toggle commits for their respective outcomes independently. `commit_state` controls whether `.wolfcastle/` state files are included in the commit or left out so that only code changes land.

The daemon commits by running `git add .` followed by `git commit` against the default index. If `.wolfcastle/` state files should be excluded (when `commit_state` is `false`), only code changes are staged.

## Codebase Knowledge Files

ADRs record why a decision was made. Specs record what a component does. AARs record what happened during a single task. None of these serve as a living document of the things developers learn by working in a codebase: build quirks, undocumented conventions, things that look wrong but are intentional, test patterns that work, dependencies between modules that the code alone doesn't make obvious.

Codebase knowledge files fill that gap. They're Markdown documents that accumulate codebase-specific knowledge across tasks, stored at `.wolfcastle/docs/knowledge/<namespace>.md` (one per engineer namespace). The `ContextBuilder` injects the knowledge file into every task's execution context, right alongside specs, ADRs, and AARs. The file is read fresh every iteration, so knowledge recorded by one task is immediately available to the next.

The lifecycle is simple. An agent works on a task and discovers something non-obvious ("the integration tests require a running Redis," "the config loader silently drops null values," "`daemon.RunOnce` has an intentional goto"). It records the discovery with `wolfcastle knowledge add "<entry>"`, which appends a bullet to the namespace's knowledge file. Future agents read it automatically as part of their context and benefit from the accumulated wisdom.

Entries should be concrete ("the `make test` target runs with `-short` by default"), durable (true next week, not just today), and non-obvious (not already in the README or derivable from the code). The file is free-form Markdown with no rigid schema.

Because the entire knowledge file is injected into every task's context, size matters. A configurable token budget (`knowledge.max_tokens`, default 2000) caps how large the file can grow. When an entry would push the file over budget, the CLI rejects it and asks the engineer (or daemon) to prune. Pruning is an auditable operation: the daemon creates a maintenance task that reviews the file, removes stale entries, consolidates related ones, and brings it under budget. Git history preserves anything removed, so nothing is lost permanently.

## Distributed State

State is stored as one `state.json` per node, co-located with its project description and task documents. Each engineer's tree lives in its own [namespace](collaboration.md#engineer-namespacing).

```
.wolfcastle/system/projects/wild-macbook/
  state.json                        <- root index
  backend/
    state.json                      <- orchestrator state
    backend.md                      <- project description
    auth/
      state.json                    <- orchestrator state
      auth.md
      session-tokens/
        state.json                  <- leaf state (tasks, audit, failures)
        session-tokens.md           <- project description
        task-3.md                   <- task working document (optional)
```

Every state mutation writes to the affected node, its parent chain, and the root index in the same operation. Task descriptions live in the leaf's `state.json`. Rich working documents (findings, context, research) go in optional Markdown files next to the state. Only the active task's working document gets injected into the model's context. (See the [full project layout](cli.md#project-layout) for directory structure.)
