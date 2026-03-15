# ADR-064: Consolidated Intake Stage and Parallel Inbox Processing

## Status
Accepted

## Date
2026-03-14

## Context
The original pipeline had three stages: expand (cheap model breaks inbox items into structured descriptions), file (mid-tier model creates projects/tasks from those descriptions), and execute (capable model does the work). This two-phase inbox flow introduced unnecessary complexity: an intermediate "expanded" status, parsed model output that needed section-matching logic, and a filing-priority gate that blocked execution while expanded items awaited filing.

The expand stage parsed model output by splitting on `##` headings and matching sections to inbox items by position. This was fragile. The file stage then consumed the `expanded` field to create projects and tasks via CLI commands. Two model invocations to do what one could handle directly.

Inbox processing also ran synchronously before each iteration in the main loop, blocking task execution while inbox items were being processed.

## Decision

### Consolidation: expand + file becomes intake

The two inbox stages are replaced by a single "intake" stage that uses a mid-tier model. The model reads raw inbox items and calls `wolfcastle project create` and `wolfcastle task add` directly, the same way the file stage already did. No intermediate "expanded" status. No section parsing. No position matching.

The default pipeline changes from `[expand, file, execute]` to `[intake, execute]`.

The `Expanded` field is removed from `InboxItem`. The "expanded" status is removed from the inbox lifecycle. Items go directly from "new" to "filed".

### Parallel inbox processing

Inbox processing moves to a background goroutine that runs independently of the main execution loop. The goroutine polls `inbox.json` at a configurable interval (`daemon.inbox_poll_interval_seconds`, default 5) and runs the intake stage when new items are found.

The main loop no longer calls `processInboxIfNeeded`. `RunOnce` handles navigation and execution only. Both goroutines share the existing file-locking mechanism for state safety. Context cancellation from SIGINT/SIGTERM stops both.

### Supersedes

This decision supersedes the expand/file design described in ADR-034 (Inbox Format and Lifecycle). The inbox format remains a JSON file with the same path, but the lifecycle simplifies to `new -> filed`.

## Consequences

- One model call handles the full inbox-to-tree flow instead of two. Fewer tokens, fewer failure modes, simpler code.
- The `parseExpandedSections` function and its position-matching logic are removed. The model calls CLI commands directly, which is more reliable than parsing structured text output.
- The filing-priority gate (skip execute when expanded items are pending) is no longer needed because intake runs in a separate goroutine.
- Inbox processing no longer blocks task execution. The daemon can execute tasks while new inbox items are being filed in parallel.
- The `InboxItem.Expanded` field and "expanded" status are removed, simplifying the state model.
- The `inbox clear` command no longer needs to handle "expanded" items.
