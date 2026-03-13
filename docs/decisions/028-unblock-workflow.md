# ADR-028: Three-Tier Unblock Workflow

## Status
Accepted

## Date
2026-03-13

## Context
When a task is blocked — whether by hitting the failure threshold, environmental issues, or model judgment — unblocking often requires more than flipping a status bit. The autonomous Wolfcastle loop already failed to resolve the issue, so autonomous retry is not the answer. Different situations call for different levels of assistance.

## Decision

### Three Tiers of Unblocking

**Tier 1: Status flip**
```
wolfcastle task unblock --node <path>
```
The user has already fixed the issue externally. This resets the failure counter and sets the task state back to Not Started (requiring re-claim). No model involvement.

**Tier 2: Interactive model-assisted fix**
```
wolfcastle unblock --node <path>
```
Starts a multi-turn chat session with a configurable model, pre-loaded with:
- Block reason and failure history
- Task breadcrumbs and audit context
- Relevant code areas
- Previous fix attempts

The human works through the fix conversationally with the model. This is explicitly NOT autonomous — anything blocked has already been surfaced to the autonomous loop which couldn't resolve it. Human judgment is the missing ingredient.

When the fix is applied, the human runs `wolfcastle task unblock --node <path>` (tier 1) to flip the status.

**Tier 3: Agent context dump**
```
wolfcastle unblock --agent --node <path>
```
Outputs rich structured diagnostic context for consumption by an already-running interactive agent (e.g. Claude Code). The output includes:
- Full block diagnostic (reason, failure count, decomposition depth, history)
- Breadcrumbs and audit state
- Relevant file paths and code areas
- Suggested approaches based on failure patterns
- Instructions on how to flip the status when done: `wolfcastle task unblock --node <path>`

The agent and human take it from there. Wolfcastle is not involved in the fix — it just provides the context and the final command to run.

### Configuration
```json
{
  "unblock": {
    "model": "heavy",
    "prompt_file": "unblock.md"
  }
}
```

The model tier for tier 2 is configurable. Tier 1 and tier 3 do not invoke a model.

### Unblock Target State
Unblocking always sets the task to Not Started (not In Progress). The task must be re-claimed. This ensures unblocking is a deliberate, focused effort — the model re-examines the task fresh rather than blindly resuming.

## Consequences
- Tier 1 is zero-cost for simple cases (user already fixed it)
- Tier 2 brings human judgment into the loop for genuinely hard problems
- Tier 3 integrates with existing interactive agent workflows (CC, etc.)
- Not autonomous by design — respects that autonomous already failed
- Configurable model tier allows using the strongest model for hard unblocks
- Re-claim requirement after unblock forces fresh evaluation of the task
