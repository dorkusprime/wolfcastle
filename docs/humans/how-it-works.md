# How It Works

## Getting Work In

The most common path: you tell your coding agent what you want done. "Add OAuth2 PKCE support to the auth service." You go back and forth with the agent, refining scope, adding constraints, clarifying how you want things structured, until the plan feels right. Then the agent uses Wolfcastle's [CLI](cli.md) to inject the work, decompose it into tasks, and start [the daemon](#the-daemon). Wolfcastle handles the orchestration from there.

When you want direct control, the CLI has you covered:

**The inbox** is for quick capture. `wolfcastle inbox add "support OAuth2 PKCE"` drops an item into a queue. The daemon's [expand stage](#the-pipeline) picks it up on its next iteration, uses a model to decompose it into tasks, and the file stage organizes those tasks into the right place in the tree. You throw an idea at Wolfcastle. It figures out the rest. ([More on the inbox.](cli.md#the-inbox))

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

## The Daemon

`wolfcastle start` launches the daemon. It owns the pipeline loop.

```
wolfcastle start                          # foreground
wolfcastle start -d                       # background
wolfcastle start --node backend/auth      # scoped to a subtree
wolfcastle start --worktree feature/auth  # isolated git worktree
```

Each iteration walks the [configured pipeline stages](configuration.md#pipelines), invokes [models](configuration.md#models), and advances the tree. Between iterations, it checks for stop signals. On SIGTERM or SIGINT, it finishes the current stage, cleans up child processes, and shuts down. It does not leave a mess.

### The Pipeline

The daemon runs a pipeline of stages. Each stage invokes a model with a specific role. The default:

| Stage       | Model Tier | Mission                                                   |
| ----------- | ---------- | --------------------------------------------------------- |
| **expand**  | cheap      | Reads the inbox. Breaks new items into tasks.             |
| **file**    | mid        | Organizes tasks into the correct project nodes.           |
| **execute** | capable    | Claims a task. Does the work. Writes code. Makes commits. |

Summaries are generated inline during execution via the `WOLFCASTLE_SUMMARY:` marker (ADR-036), not as a separate stage.

Stages do not pass output to each other. They read the current state of the world and act on it. The expand stage creates tasks. The execute stage finds them. No coupling. No handoffs. Just [state on disk](#distributed-state) and models that know how to read it.

### Seven-Phase Execution

When the execute stage claims a task, the model follows a seven-phase protocol:

1. **Claim** the task.
2. **Study** the project description, [specs](collaboration.md#specs), [breadcrumbs](audits.md#breadcrumbs), and linked context.
3. **Implement** the work.
4. **Validate** by running configured checks (tests, lints, builds).
5. **Record** [breadcrumbs](audits.md#breadcrumbs) describing what was done and why.
6. **Commit** the changes.
7. **Signal** the outcome: COMPLETE, YIELD (needs another iteration), or BLOCKED.

The model communicates through [script calls](cli.md): `wolfcastle task claim`, `wolfcastle audit breadcrumb`, `wolfcastle task complete`. Every side effect goes through a deterministic command that validates inputs and enforces invariants. The model cannot corrupt the tree. It can only ask the scripts to make valid changes.

## Distributed State

State is stored as one `state.json` per node, co-located with its project description and task documents. Each engineer's tree lives in its own [namespace](collaboration.md#engineer-namespacing).

```
.wolfcastle/projects/wild-macbook/
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
