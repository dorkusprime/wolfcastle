# ADR-072: Pre-blocking Not-Started Tasks

## Status
Accepted

## Date
2026-03-16

## Context

`TaskBlock` required the target task to be `in_progress`. This made sense when blocking was a self-report: a model working on a task hits a wall and blocks itself. But the discovery-first intake pipeline (ADR-071) introduces a new pattern: a discovery agent finishes its investigation, determines that a downstream task is infeasible, and needs to block that task before anyone claims it.

Under the old constraint, the discovery agent would have to claim the downstream task (moving it to `in_progress`), then immediately block it. This is a pointless state transition. The task was never worked on. Claiming it just to block it pollutes the state history and confuses the semantics of `in_progress`.

A second gap existed in `TaskComplete`. When a node's last non-blocked task completed, the node state was set to `complete` even if blocked tasks remained. This left blocked tasks orphaned under a "complete" node, invisible to the status view.

## Decision

`TaskBlock` now accepts tasks in the `not_started` state in addition to `in_progress`. A task can be blocked before any agent claims it. The block reason is recorded the same way, and the task transitions directly from `not_started` to `blocked`.

`TaskComplete` now checks remaining sibling tasks after marking one complete. If all remaining tasks under a node are in the `blocked` state, the node state is set to `blocked` rather than `complete`. This surfaces the blockage in the tree instead of burying it under a falsely complete node.

## Consequences

- Discovery agents can pre-block tasks they have identified as infeasible without the claim-then-block dance. The state transition is honest: the task was never in progress.
- `TaskBlock` validates against `not_started` and `in_progress`. Other states (`complete`, `blocked`, `failed`) still reject the block call.
- Nodes with only blocked tasks remaining show as `blocked`, propagating the signal upward through the tree. The operator sees blockage at the node level without having to inspect individual tasks.
- The unblock workflow (ADR-028) is unaffected. Unblocking a pre-blocked task returns it to `not_started`, ready for the daemon to pick up.
