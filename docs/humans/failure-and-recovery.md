# Failure and Recovery

## Task Failure Escalation

Tasks fail. Wolfcastle does not take it personally. It takes it systematically.

In the [TUI](the-tui.md), blocked tasks show the `☢` glyph in the tree. Toast notifications announce auto-blocks as they happen. Press `Enter` on a blocked task to see the block reason, failure count, and last failure type in the [task detail](the-tui.md#task-detail) view.

Each task tracks a failure counter. All thresholds are [configurable](config-reference.md#failure):

| Failures | Depth OK | Action |
|----------|----------|--------|
| < 10 | n/a | Retry. |
| = 10 | Yes | Decompose. The task becomes an orchestrator with smaller child tasks. |
| = 10 | No (depth limit) | Auto-block. Decomposition cannot recurse forever. |
| = 50 | n/a | Hard block. Safety net. The task is done fighting. |

All thresholds are configurable:

```json
{
  "failure": {
    "decomposition_threshold": 10,
    "max_decomposition_depth": 5,
    "hard_cap": 50
  }
}
```

## Decomposition

When a task hits the decomposition threshold, the model breaks it into smaller problems. The [leaf node transforms into an orchestrator node](how-it-works.md#the-project-tree) with new child leaves. Each child inherits `decomposition_depth + 1`. Each child's failure counter starts at zero.

Decomposition can recurse. A decomposed task's children can themselves decompose. The `max_decomposition_depth` setting prevents infinite recursion. The `hard_cap` prevents infinite iteration. Between them, Wolfcastle always stops eventually.

## API Failure Handling

Model API failures (timeouts, rate limits, server errors) get exponential backoff (see [retries reference](config-reference.md#retries)):

```json
{
  "api_retry": {
    "initial_delay_seconds": 30,
    "max_delay_seconds": 600,
    "max_retries": -1
  }
}
```

`max_retries: -1` means unlimited. Wolfcastle will wait as long as it takes.

## Stall Detection

Model processes sometimes hang: the API stops responding, the subprocess deadlocks, output just stops flowing. The stall detector watches for this. When a model invocation produces no stdout output for a [configurable timeout](config-reference.md#daemon) period, Wolfcastle kills the entire process group and returns `ErrStallTimeout`. Any output at all (a single line, a partial token) resets the timer, so a slow but active model is left alone. Only truly silent processes get killed. The stall timeout is configured per-invocation; a zero value disables detection entirely.

## Partial Work Preservation

When a task fails or yields, the daemon commits whatever work the agent produced, provided `commit_on_failure` is enabled (it is by default). The commit message includes the attempt number: `wolfcastle: <task-id> partial (attempt N)`. This means partial progress is never lost to a retry, and the full timeline of attempts is visible in the git log. Combined with [stall detection](#stall-detection), even a hung model invocation results in a committed snapshot of the work completed before the stall.

See [`git` configuration](config-reference.md#git) for the `auto_commit`, `commit_on_failure`, and related fields.

## Git Revert Recovery

Every daemon commit is atomic: code changes and `.wolfcastle/` state land together in a single commit. One commit, one complete snapshot of the project at that point in time. This is what makes git-based recovery possible. If a task produces bad output, you don't need to untangle which files belong to the task and which don't. The commit boundary already drew that line for you.

### Reading the commit log

The daemon writes commit messages in a predictable format. Successful tasks produce `wolfcastle: <task-id> complete`. Failed or yielded tasks produce `wolfcastle: <task-id> partial (attempt N)`. A typical stretch of history looks like this:

```
$ git log --oneline -6
f4a2c1e wolfcastle: api-gateway/auth/task-0003 complete
b8d91a7 wolfcastle: api-gateway/auth/task-0002 complete
3e0f5c4 wolfcastle: api-gateway/auth/task-0001 partial (attempt 2)
a1c7b30 wolfcastle: api-gateway/auth/task-0001 partial (attempt 1)
91d4e82 wolfcastle: api-gateway/rate-limit/task-0002 complete
c5f0a19 wolfcastle: api-gateway/rate-limit/task-0001 complete
```

The task IDs and attempt numbers tell the story. Here, `task-0001` under `auth` took two attempts before succeeding (the `partial` commits preserve its intermediate work, as described in [Partial Work Preservation](#partial-work-preservation)). Tasks `task-0002` and `task-0003` each completed on their first try.

### Reverting a bad task

Suppose `task-0003` introduced a regression. You want to undo its work and let the daemon try again from a clean state. The commit you want to revert is `f4a2c1e`:

```
$ git revert f4a2c1e
```

This creates a new commit that inverts the changes from `task-0003`, including both the code and the `.wolfcastle/` state updates. The task's state reverts to whatever it was before that commit, which means the daemon will see it as incomplete on its next iteration.

If the bad outcome spans multiple commits (say the task failed once and then completed, producing two commits), revert them in reverse chronological order:

```
$ git revert f4a2c1e b8d91a7
```

For cases where you'd rather erase the commits entirely instead of adding revert commits, `git reset` works too:

```
$ git reset --hard b8d91a7
```

This moves HEAD back to just before the bad commit. Use this only on branches where rewriting history is safe (feature branches, worktrees). On shared branches, prefer `git revert` so the history stays linear.

### Restarting the daemon

After the revert, restart the daemon. It reads task state from the `.wolfcastle/` files on disk, so the reverted state is the state it sees. The task that produced the bad output will be back in its pre-completion state, and the daemon will pick it up in the next iteration. If the task's state was `in_progress`, [self-healing](#self-healing) applies: the daemon hands the situation to the model, which decides whether to resume, roll back, or block.

No manual state editing required. The atomic commit structure means reverting the git history is sufficient to rewind the project state.

See [`git` configuration](config-reference.md#git) for the `auto_commit`, `commit_on_success`, `commit_on_failure`, and `commit_state` fields that control how the daemon writes these commits.

## Self-Healing

If [the daemon](how-it-works.md#the-daemon) crashes mid-task (power failure, OOM, act of god), it recovers on restart. It finds the task left `in_progress`, hands the situation to the model, and lets it decide: resume, roll back, or block. Stale PID files are detected and ignored. Because the daemon [commits after every iteration](collaboration.md#daemon-side-commits), the last iteration's output is already in the history, giving the recovering model a clean baseline to work from.

Self-healing also handles two structural problems that a crash can leave behind:

**Blocked audits with orphaned gaps.** When an audit task finds gaps, it blocks and the daemon creates remediation subtasks. If the daemon crashes between the block and the subtask creation, the audit is stuck: blocked with open gaps but no remediation work queued. On restart, self-healing detects this state, creates the missing remediation subtasks, and resets the audit to `not_started` so the cycle can resume.

**CHILDREF_STATE_MISMATCH.** When an orchestrator's recorded child state diverges from the child's actual state (another crash artifact), the [structural validator](audits.md#structural-validation) detects it as a deterministic fix. The repair rewrites the orchestrator's state on disk, correcting both the stale child reference and recomputing the orchestrator's own state. This runs as part of the startup validation checks, before the daemon begins its first iteration.

## The Unblock Workflow

Tasks block. It happens. Wolfcastle provides three escalating tiers to deal with it.

### Tier 1: Status Flip

```
wolfcastle task unblock --node backend/auth/session-tokens
```

Zero cost. You already fixed the problem externally. This resets the failure counter and sets the task back to [`not_started`](how-it-works.md#four-states). No model involved.

### Tier 2: Interactive Model-Assisted

```
wolfcastle unblock --node backend/auth/session-tokens
```

Multi-turn conversation with a model, pre-loaded with the block reason, failure history, [breadcrumbs](audits.md#breadcrumbs), [audit context](audits.md#the-audit-system), and previous attempts. You and the model work through the fix together. This is not autonomous; the human drives.

### Tier 3: Agent Context Dump

```
wolfcastle unblock --agent --node backend/auth/session-tokens
```

Rich structured diagnostic output for consumption by an external agent. Full block diagnostic, [breadcrumbs](audits.md#breadcrumbs), audit state, file paths, suggested approaches, and instructions. Feed it to whatever agent you're running.

All tiers reset the task to [`not_started`](how-it-works.md#four-states). Fresh evaluation, no blind resumption.
