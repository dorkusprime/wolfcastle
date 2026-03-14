# ADR-040: Daemon Artifact Cleanup in Doctor

**Status:** Accepted

**Date:** 2026-03-14

## Context

ADR-025 established `wolfcastle doctor` as the structural validation and
repair tool for the project tree. It validates 17+ categories of structural
issues — dangling references, state mismatches, missing audit tasks — and
applies deterministic fixes via `--fix`.

However, when the daemon crashes or is killed without cleanup, operational
artifacts are left behind:

1. **Stale PID file** (`daemon.pid`) — points to a dead process or a
   recycled PID belonging to something else. The next `wolfcastle start`
   may refuse to start or behave unpredictably.
2. **Stale stop file** (`stop`) — left behind if `wolfcastle stop` was
   issued after the daemon had already exited. The next `wolfcastle start`
   would immediately see it and shut down.

These are not structural state issues, but they block normal daemon
operation and require manual intervention to resolve.

## Decision

Extend the validation engine with two new categories:

- **`STALE_PID_FILE`** (warning, deterministic fix) — PID file exists but
  the referenced process is not alive (checked via signal 0). Fix: remove
  the PID file.
- **`STALE_STOP_FILE`** (warning, deterministic fix) — stop file exists but
  no daemon process is running. Fix: remove the stop file.

Both checks reuse the existing `isDaemonAlive()` method. Both are classified
as warnings (not errors) because they don't represent data corruption — they
are operational artifacts. Both are deterministic fixes: the repair action
(file removal) requires no model or human judgment.

`ApplyDeterministicFixes` accepts an optional `wolfcastleDir` parameter to
locate the artifacts. This preserves backward compatibility with callers
that don't have access to the wolfcastle directory.

## Consequences

- `wolfcastle doctor` now detects and reports stale daemon artifacts.
- `wolfcastle doctor --fix` removes them automatically.
- The validation engine's scope expands from "tree structure" to
  "workspace health," which is a natural evolution of the doctor concept.
- No behavioral change to startup validation — these categories are not
  in the startup gate set, so the daemon won't block on stale artifacts.
