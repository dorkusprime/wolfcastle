# ADR-070: Deliverable Change Detection

## Status
Accepted

## Date
2026-03-16

## Context

ADR-069 introduced deliverable verification: the daemon checks that declared files exist and are non-empty before accepting a WOLFCASTLE_COMPLETE signal. This catches the case where a model claims victory without producing any artifacts. But there is a second failure mode it does not catch: a model that opens a deliverable file, reads it, changes nothing, and signals completion. The file exists. It is non-empty. The check passes. No work was done.

This is not a theoretical concern. Models under retry pressure will sometimes emit the completion marker without making meaningful changes, especially on tasks where the initial attempt failed and the model is unsure what to do differently. The deliverable exists because it existed before the task started.

## Decision

At task claim time, the daemon captures SHA-256 hashes of all declared deliverable files that already exist on disk. These baseline hashes are stored in the task's runtime state.

When the model signals WOLFCASTLE_COMPLETE, the daemon re-hashes each deliverable and compares against the baseline. If every deliverable's hash matches its baseline (or the file still does not exist), the completion is rejected. At least one deliverable must have changed, been created, or been deleted for the completion to stand.

Glob patterns in deliverables are supported. The daemon expands globs at both claim time and completion time, hashing all matched files. New files matching the glob that did not exist at claim time count as changes.

Files that did not exist at claim time and still do not exist at completion time are treated as unchanged. The existing non-empty size check from ADR-069 still applies independently.

## Consequences

- Models cannot complete a task by doing nothing. The daemon enforces that deliverables were actually modified, not just present.
- Baseline snapshots add a small amount of state per task, stored only for the duration of the task's execution. Hashing is fast; SHA-256 on source files is measured in microseconds.
- Glob deliverables work naturally: the daemon expands them at both endpoints and diffs the full set. New files matching the pattern are detected as changes.
- The existing deliverable existence check and the new change detection check are independent. A task must pass both: files must exist with non-zero size, and at least one must differ from its baseline.
- Retry iterations get a clean signal. If the daemon rejects a false completion, the failure count increments and the model is re-invoked with context about what went wrong, same as any other rejection.
