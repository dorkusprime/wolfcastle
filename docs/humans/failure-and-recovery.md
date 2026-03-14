# Failure and Recovery

## Task Failure Escalation

Tasks fail. Wolfcastle does not take it personally. It takes it systematically.

Each task tracks a failure counter. All thresholds are [configurable](configuration.md):

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

Model API failures (timeouts, rate limits, server errors) get exponential backoff:

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

## Self-Healing

If [the daemon](how-it-works.md#the-daemon) crashes mid-task (power failure, OOM, act of god), it recovers on restart. It finds the task left `in_progress`, hands the situation to the model, and lets it decide: resume, roll back, or block. Stale PID files are detected and ignored.

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
