# Auto-archive runs inline in RunOnce, not as a separate goroutine

## Status
Accepted

## Date
2026-03-21

## Status
Accepted

## Context
The auto-archive scanner needs a home in the daemon loop. Two patterns exist in the codebase: inline work within RunOnce (task execution, planning) and a parallel goroutine (inbox polling via ADR-064). The archive operation involves directory moves and index updates with no model invocations.

## Options Considered
1. Separate goroutine with its own timer (like the inbox goroutine)
2. Inline check in RunOnce, gated by a poll interval field on the Daemon struct

## Decision
Option 2: inline in RunOnce. Archive checking slots in after task finding and planning, before the idle report. A `lastArchiveCheck` timestamp on the Daemon struct enforces the 5-minute poll interval without a timer goroutine. At most one node is archived per RunOnce call, keeping each iteration bounded.

## Consequences
No new goroutine, no new synchronization. Archive work is lightweight enough that it doesn't block the main loop meaningfully. The tradeoff: archive checks only happen when the daemon is idle (no tasks, no planning), so a constantly-busy daemon won't archive until it runs out of work. This is acceptable because archival is not time-sensitive; the delay threshold already provides a generous window.
