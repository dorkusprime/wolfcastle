# ADR-080: Sequential Inbox Intake

## Status
Accepted

## Date
2026-03-21

## Context
The intake stage batched all new inbox items into a single model invocation. When multiple inbox items described work related to the same feature, the model had no way to see projects it had just created within that same invocation: the root index on disk still reflected the state before the invocation began. The result was duplicate root projects, one per inbox item, for work that should have been filed under a single project.

The root cause is a visibility gap. The model reads the root index at prompt-assembly time, but any `wolfcastle project create` calls it makes during execution only land on disk after the CLI returns. A batched invocation sees a stale snapshot for the entire batch.

## Decision
Process inbox items sequentially, one model invocation per item. Before each invocation, re-read the root index so the model's context includes any projects created by previous invocations in the same intake pass. On success, mark that single item as filed before moving on to the next.

This trades throughput for correctness. A batch of N items now requires N model invocations instead of one, but each invocation operates against an accurate view of the project tree. The throughput cost is acceptable because inbox items arrive infrequently (human-paced, not machine-paced), and correctness of deduplication is a hard requirement for the integrity of the project tree.

A failed invocation (non-zero exit) skips that item and continues to the next. The skipped item remains in "new" status for retry on the next intake pass. The `workAvailable` signal is only sent if at least one item was successfully filed.

## Consequences
- Duplicate root projects from related inbox items are eliminated.
- Intake latency scales linearly with the number of pending items. For typical workloads (1-3 items per pass) this is negligible.
- Each item's prompt includes a fresh root index, which means slightly more disk I/O per intake pass.
- A single item's failure no longer blocks the entire batch; items that succeed are filed immediately, and failures are retried independently.
