# NodeState.RenderContext phantom taskID parameter

## Status
Accepted

## Date
2026-03-18

## Status
Accepted

## Date
2026-03-18

## Context
NodeState.RenderContext accepts a `taskID string` parameter (internal/state/render.go:97) that appears nowhere in the method body. The method renders node type, node state, and linked specs, none of which vary by task. All six call sites in the test suite pass a taskID string, but the value has no effect on output.

This parameter was introduced during the initial RenderContext extraction to mirror the anticipated calling convention in `pipeline/context.go`, where `buildIterationContext` already knows the active task ID and passes it alongside the node state when composing the iteration prompt. The intent: when ContextBuilder is eventually refactored to delegate to the domain RenderContext methods (rather than duplicating their logic inline), it will call `ns.RenderContext(taskID)` and `task.RenderContext()` as a pair. At that point, NodeState may need the task ID to render task-scoped node information (for example, per-task escalation markers or task-specific spec annotations that the node state surfaces on behalf of the task).

Meanwhile, Task.RenderContext was refactored to a parameterless signature (see ADR 078), and AuditState.RenderContext has always been parameterless. NodeState is the only domain render method that carries a parameter it does not yet use.

## Options Considered
1. **Remove the parameter now, add it back when needed.** Keeps the API honest: every parameter is used. The cost is a breaking signature change on the day the parameter becomes load-bearing, requiring updates to every caller.
2. **Keep the phantom parameter.** Preserves the target signature for the planned ContextBuilder consolidation. Callers already pass the value, so no migration is needed when the method starts using it. The tradeoff is a slightly misleading API surface in the interim: a reader examining the method body sees an unused parameter and must read the doc comment to understand why it exists.
3. **Accept an options struct instead.** Future-proof against additional parameters, but over-engineered for a method that renders a handful of markdown lines. Adds allocation overhead and ceremony for no current benefit.

## Decision
Option 2. The `taskID` parameter stays. The doc comment on the method explicitly states the parameter's purpose ("identifies the active task so the caller can pair this output with the corresponding Task.RenderContext and AuditState.RenderContext") and signals that it is reserved for future use.

The parameter is expected to become load-bearing when ContextBuilder consolidation begins: specifically, when `pipeline/context.go` stops duplicating node-level rendering and instead calls `ns.RenderContext(taskID)` directly. At that point, the method may use taskID to filter or annotate node-level information that is relevant only to the active task (escalation markers, task-scoped spec links, or similar per-task context).

## Consequences
- The NodeState.RenderContext signature diverges from the parameterless pattern established by Task.RenderContext and AuditState.RenderContext. This asymmetry is intentional: node-level rendering has a known future dependency on task identity that the other two methods do not.
- Static analysis tools or linters that flag unused parameters will report this method. The doc comment serves as the canonical justification; a linter-suppress directive is acceptable if the project adopts such tooling.
- If ContextBuilder consolidation does not happen, or if the design evolves so that taskID is never needed, the parameter should be removed at that time to resolve the API dishonesty. This ADR does not create a permanent entitlement to unused parameters.
