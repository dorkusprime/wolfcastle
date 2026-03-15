# ADR-019: Failure Handling, Decomposition, and Retry Thresholds

## Status
Accepted

## Date
2026-03-12

## Context
Autonomous execution needs guardrails against infinite loops where the model repeatedly fails at a task, but also enough room for the model to iterate on genuine fixes. Ralph used a simple "3 failures = blocked" rule which was too aggressive. Wolfcastle needs a more nuanced escalation path that uses the model's ability to restructure work when direct fixing isn't working.

## Decision

### Model Invocation Failures (API errors, crashes, empty output)
Exponential backoff with a configurable maximum backoff delay. No retry cap by default (`-1` = unlimited). The daemon waits patiently and resumes when the API recovers.

```json
{
  "retries": {
    "initial_delay_seconds": 30,
    "max_delay_seconds": 600,
    "max_retries": -1
  }
}
```

### Task-Level Failures (validation fails, tests don't pass)
The model uses its judgment — keep fixing if the issue is fixable, block if it's environmental (broken dependency, infrastructure issue, etc.). Three escalation thresholds govern the behavior:

1. **0 to N failures (default N=10)**: Keep fixing. The model iterates on the problem.
2. **N failures (default 10)**: Prompted to decompose. The model is told to consider breaking the task into smaller pieces. This is a prompt, not a mandate — the model can choose to decompose or to block if decomposition doesn't make sense.
3. **Hard cap failures (default 50)**: Forced block regardless of depth or model judgment. Safety net against unbounded iteration.

### Decomposition Depth
Each node tracks a `decomposition_depth` integer:
- Depth 0 = original task
- Depth 1 = decomposed from a task that hit the failure threshold
- Depth N = N levels of decomposition

When a task is decomposed, child tasks inherit `decomposition_depth + 1`. Each child's failure counter starts at zero.

### Maximum Decomposition Depth (default 5)
At max depth, decomposition is no longer offered as an option. Hitting the failure threshold at max depth results in automatic blocking.

### Escalation Summary

| Condition | Behavior |
|-----------|----------|
| Failures < decomposition threshold | Keep fixing |
| Failures = decomposition threshold, depth < max | Prompted to decompose (model may choose to block instead) |
| Failures = decomposition threshold, depth = max | Auto-blocked |
| Failures = hard cap (any depth) | Auto-blocked |

### Unblocking
`wolfcastle task unblock --node <path>` allows the user to signal that an external issue has been resolved. This resets the failure counter and allows execution to resume.

### Configuration

All thresholds are configurable in `config.json`:

```json
{
  "failure": {
    "decomposition_threshold": 10,
    "max_decomposition_depth": 5,
    "hard_cap": 50
  }
}
```

## Consequences
- The model gets real room to iterate (10 attempts before escalation, 50 hard cap)
- Decomposition is a prompted option, not a forced action — model judgment is preserved
- Depth tracking prevents infinite decomposition chains
- The hard cap catches edge cases where decomposition produces tasks that each fail just under the threshold
- Users can tune thresholds per project — tighter for CI, looser for exploratory work
- `task unblock` gives users a clean way to intervene without restarting the daemon
