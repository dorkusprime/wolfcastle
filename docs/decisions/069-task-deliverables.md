# ADR-069: Task Deliverables

## Status
Accepted

## Date
2026-03-15

## Context

Tasks in Wolfcastle declare what they intend to do, but nothing in the system enforces that they actually produce the artifacts they promise. A model can emit WOLFCASTLE_COMPLETE without having created the files it was supposed to write. The daemon accepts the completion at face value, the task transitions to complete, and a missing deliverable goes unnoticed until the audit stage (if it catches it at all).

This gap is structural. The daemon already verifies terminal markers and manages failure counts, but it has no mechanism to verify that the work product exists before signing off on a completion signal.

## Decision

Add a `Deliverables []string` field to the Task struct. Each entry is a repo-relative file path that the task is expected to produce.

Deliverables are set at task creation time via `wolfcastle task add --deliverable "path/to/file"` (repeatable flag) or appended later via `wolfcastle task deliverable "path" --node <task-address>`. The intake model sets deliverables when creating tasks. The execute model can add deliverables it discovers during work.

The daemon verifies deliverable existence after scanning for WOLFCASTLE_COMPLETE but before calling TaskComplete. For each declared deliverable, the daemon checks that the file exists on disk and has non-zero size. If any deliverable is missing or empty, the completion is rejected: the marker is cleared, the failure count increments, and the model is re-invoked on the next iteration.

Tasks with no declared deliverables pass the check unconditionally, preserving backward compatibility.

The iteration context includes the deliverables list so the execute model knows what files it must produce. The execute prompt instructs the model to verify deliverables before signaling completion.

## Consequences

- Tasks that declare deliverables cannot complete without producing them. The daemon enforces this structurally rather than relying on model self-discipline.
- The intake model gains the `--deliverable` flag in its command reference, enabling it to declare expected outputs at task creation time.
- The execute model sees deliverables in its iteration context and is instructed to verify them before signaling completion.
- Existing tasks without deliverables are unaffected. The `omitempty` JSON tag keeps state files clean.
- The failure-count/decomposition machinery handles deliverable rejections the same way it handles any other non-completion: increment the counter, re-invoke, and eventually decompose or hard-cap if the model cannot produce the required files.
