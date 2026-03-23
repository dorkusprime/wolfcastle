# ADR-045: Daemon Package Decomposition

**Status:** Accepted

**Date:** 2026-03-14

## Context

`internal/daemon/daemon.go` is ~850 lines containing the main loop, pipeline
stage execution, inbox stage handlers (expand, file), model marker parsing,
retry logic, state propagation helpers, and inbox state checking. While the
functions within it are well-structured, the file itself has become a
grab-bag of concerns. As features accrete (new marker types, new stage
handlers, health checks), this file will become harder to navigate and
reason about.

## Decision

Split `daemon.go` into focused files within the same package:

- **daemon.go**: `Daemon` struct, `New()`, `Run()`, `RunWithSupervisor()`,
  `RunOnce()`, `selfHeal()`, `scopeLabel()`. The orchestration skeleton.
- **iteration.go**: `runIteration()` and the pipeline stage dispatch logic.
  The per-iteration execution path.
- **stages.go**: `runExpandStage()`, `runFileStage()`,
  `parseExpandedSections()`. Inbox-specific stage handlers.
- **markers.go**: `applyModelMarkers()`, `dedupPipe()`. Marker parsing and
  state mutation from model output.
- **retry.go**: `invokeWithRetry()`. Retry logic with exponential backoff.
- **propagate.go**: `propagateState()`, `checkInboxState()`. State
  propagation and inbox state helpers.

Existing files remain unchanged:

- **branch.go**: `currentBranch()` (already separate).
- **pid.go**. PID file operations (already separate).
- **daemon_test.go**: tests (already separate).

This is a pure file reorganization: no API changes, no new packages, no
behavior changes. All functions stay in package `daemon`. The split is by
responsibility, not by abstraction.

## Consequences

- Each file has a clear, single responsibility.
- Navigation is easier: "where's the marker parsing?" → `markers.go`.
- New stage handlers go in `stages.go`, new markers go in `markers.go` —
  clear homes for new code.
- No import changes anywhere: this is internal to the package.
- Git blame is affected but the commit message will note the
  reorganization.
