# ADR-071: Discovery-First Intake Pipeline

## Status
Accepted

## Date
2026-03-16

## Context

The intake model receives an inbox item and creates projects and tasks. But not all inbox items are ready for implementation. Some reference technologies the codebase has never used. Some are vague enough that jumping straight to code would produce garbage. The intake model had no structured way to distinguish between these cases; it would either guess at task breakdowns or produce overly broad tasks that the execute model would fail on repeatedly.

The failure pattern was predictable: vague request goes in, underspecified tasks come out, the execute model thrashes against them, hits the retry cap, and the tasks end up blocked. The work that should have happened first (research, specification, feasibility checking) was skipped entirely.

## Decision

The intake model follows a decision tree when processing inbox items:

1. **Unknown technology.** If the request involves libraries, frameworks, or tools not present in the codebase, the intake model creates a discovery task only. The discovery task investigates feasibility, identifies the right approach, and produces a findings document. Downstream implementation tasks are not created until the discovery task completes.

2. **Vague requirements.** If the request is underspecified (no clear acceptance criteria, ambiguous scope, multiple possible interpretations), the intake model creates a specification task first. The spec task produces a concrete specification document that subsequent implementation tasks can reference.

3. **Known and specific.** If the technology is already in use and the requirements are clear, the intake model creates implementation tasks directly, same as before.

Pre-blocking is the mechanism that connects these stages. A discovery agent that determines something is infeasible can block downstream tasks via `wolfcastle task block` without claiming them. This prevents the daemon from wasting model invocations on tasks that cannot succeed.

## Consequences

- The intake model makes an explicit routing decision for each inbox item instead of always producing implementation tasks. The decision tree is documented in the intake prompt.
- Discovery tasks produce knowledge artifacts (findings documents) that feed into subsequent task creation. The system learns before it builds.
- Vague requests get a specification pass before implementation, reducing thrash at the execute stage.
- Pre-blocking connects the discovery and implementation phases: if discovery finds a dead end, downstream tasks are blocked immediately without burning execute-model tokens.
- The intake prompt grows to include the decision tree criteria. The model needs clear guidance on what counts as "unknown technology" versus "known" and "vague" versus "specific."
