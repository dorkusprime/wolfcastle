# Task.RenderContext parameterless refactoring

## Status
Accepted

## Date
2026-03-18

## Status
Accepted

## Context
Task.RenderContext originally accepted two parameters: nodeAddr (the slash-delimited node path) and nodeDir (a filesystem path for reading per-task `.md` files). This coupled the domain type to two concerns it had no business owning: full address composition (a responsibility of ContextBuilder, which knows the node hierarchy) and filesystem I/O (reading per-task markdown from the node directory). The method lived in `internal/state/render.go`, a pure domain package, yet it imported `path/filepath` and called `os.ReadFile`, meaning tests required real files on disk and the domain layer carried a transitive dependency on the operating system.

This was identified as audit gap `gap-render-context-methods-1`.

## Options Considered
1. **Keep parameters, mock the filesystem.** Introduce an `fs.FS` parameter or interface so tests could supply an in-memory filesystem. This would remove the real-file dependency but still leave address composition on the domain type, and it would widen the method signature further.
2. **Move the method to the pipeline package entirely.** Eliminate the domain method and keep all rendering in `pipeline/context.go`. Straightforward, but it collapses the useful separation between "what a task knows about itself" and "what the pipeline assembles from external sources." Other domain types (NodeState, AuditState) already have their own RenderContext methods; removing Task's would break the symmetry.
3. **Strip parameters, defer external concerns to ContextBuilder.** Make RenderContext() parameterless. The task renders only what it owns: its own ID, description, body, deliverables, acceptance criteria, constraints, references (listed but not inlined), failure metadata. The full node-qualified address and per-task `.md` file inlining become ContextBuilder's responsibility, since ContextBuilder already has nodeAddr, nodeDir, and config access.

## Decision
Option 3. Task.RenderContext now takes no parameters. It renders the task ID alone (not the full `nodeAddr/taskID` path), and it no longer reads `.md` files from the filesystem. The `path/filepath` import is removed; `os` remains only for reference file inlining (which reads from absolute paths already present in the References slice, a pre-existing behavior unrelated to nodeDir).

The ContextBuilder in `internal/pipeline/context.go` retains its own inline rendering that composes the full address and handles per-task `.md` reads. Over time, ContextBuilder will be refactored to delegate to the domain RenderContext methods, but that migration is a separate concern.

## Consequences
- The `state` package no longer imports `path/filepath`. Its only remaining `os` usage is for reference file inlining, which operates on absolute paths stored in the task's References field.
- Tests for Task.RenderContext no longer need temporary directories or real files for the nodeDir/.md reading path. Three such tests were removed in the refactoring commit.
- NodeState.RenderContext and AuditState.RenderContext follow the same parameterless pattern, giving all three domain render methods a consistent shape.
- The pipeline's `buildIterationContext` still duplicates some rendering logic. Consolidation (having the pipeline call the domain methods and layer on its own concerns) is a planned follow-up, not part of this decision.
