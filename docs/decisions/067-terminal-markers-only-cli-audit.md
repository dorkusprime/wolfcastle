# ADR-067: Terminal Markers Only; Audit Mutations via CLI

## Status
Accepted

## Date
2026-03-15

## Context
The daemon previously parsed a dozen data-carrying markers from model stdout (WOLFCASTLE_BREADCRUMB, WOLFCASTLE_GAP, WOLFCASTLE_SCOPE, WOLFCASTLE_SUMMARY, etc.) and applied the mutations to in-memory state before saving. This created a subtle bug: the model's CLI subprocesses could write to state.json during execution, but the daemon's in-memory copy would overwrite those mutations on its post-invocation save.

The data-carrying markers were also redundant with the CLI commands that already existed for the same operations (audit breadcrumb, audit gap, audit scope, etc.). Maintaining two paths for the same mutations doubled the surface area without adding value.

## Decision
Remove all data-carrying markers from the daemon. The only markers the daemon parses are the three terminal signals: WOLFCASTLE_COMPLETE, WOLFCASTLE_YIELD, and WOLFCASTLE_BLOCKED. These remain as stdout markers because they control iteration flow rather than mutating audit state.

All audit mutations (breadcrumbs, gaps, fix-gap, scope, summary, resolve-escalation) are now performed exclusively via CLI commands that the model invokes as subprocesses. These commands load state.json, mutate it, and save it back, making each mutation atomic and durable.

After the model invocation returns, the daemon reloads state.json from disk before processing terminal markers. This ensures that CLI-driven mutations survive the daemon's own state transitions.

A new `wolfcastle audit summary` CLI command was added to replace the WOLFCASTLE_SUMMARY marker.

The execute stage's AllowedCommands list was expanded to include: audit gap, audit fix-gap, audit scope, audit summary, audit resolve-escalation.

## Consequences
- The daemon no longer needs markers.go (ParseMarkers, MarkerCallbacks, applyModelMarkers, dedupPipe). Deleted.
- The SyncAuditLifecycle call after marker parsing was removed from iteration.go (it runs on task state transitions instead).
- The "persist marker mutations immediately" save was removed; the daemon reloads from disk instead.
- Integration tests that previously emitted stdout markers for breadcrumbs and gaps now use CLI calls.
- Prompt templates (execute.md, summary-required.md, script-reference.md) were updated to instruct the model to use CLI commands.
- Historical ADRs (054, 045) and specs reference the removed code but are left as-is since they document past decisions.
