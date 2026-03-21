# ContextBuilder Contract

## Overview

`ContextBuilder` in `internal/pipeline` composes the full iteration context string that the daemon feeds to the execute-stage model. It replaces the monolithic `buildIterationContext()` function in `pipeline/context.go` by delegating rendering to domain-owned `RenderContext()` methods on `state.NodeState`, `state.Task`, and `state.AuditState`, then layering in class guidance and prompt-template-backed sections (summary requirements, failure context) that require repository access.

The struct holds two repository dependencies and no mutable state of its own; it is safe for concurrent use without synchronization.

## Type

```go
type ContextBuilder struct {
    prompts *PromptRepository
    classes *ClassRepository
}
```

`prompts` resolves three-tier prompt templates (`summary-required.md`, `context-headers.md`, `decomposition.md`). `classes` resolves behavioral guidance prompts keyed by task class. Neither field is optional; both must be provided at construction.

## Constructor

### NewContextBuilder(prompts *PromptRepository, classes *ClassRepository) *ContextBuilder

Returns a ready-to-use `ContextBuilder`. Panics if either argument is nil: both repositories are required for correct context assembly.

## Methods

### Build(nodeAddr string, nodeDir string, ns *state.NodeState, taskID string, cfg *config.Config) string

Assembles the complete iteration context for a single task within a node. The returned string is Markdown-formatted text ready for inclusion in the execute-stage prompt.

**Parameters:**

- `nodeAddr`: slash-delimited node address (e.g. `"my-project/auth"`), rendered at the top of the output
- `nodeDir`: filesystem path to the node directory; when non-empty, per-task `.md` files (`{taskID}.md`) are read from this directory and their content is injected into the task context after the description line. May be empty to skip per-task file loading.
- `ns`: the node's persistent state, supplying node metadata, task list, audit trail, and linked specs
- `taskID`: identifies the active task within `ns.Tasks`
- `cfg`: daemon configuration, used for failure policy thresholds and decomposition settings; may be nil for backward compatibility (failure context is skipped when nil)

**Assembly order:**

The output is built by appending sections in this fixed order. Each section is emitted only when its data is non-empty.

1. **Node address header.** `**Node:** {nodeAddr}` on its own line.

2. **Node context.** Calls `ns.RenderContext(taskID)`, which emits node type, node state, and linked specs. The node address is not part of `RenderContext`; it comes from step 1.

3. **Task context.** Locates the task matching `taskID` in `ns.Tasks`. Calls `task.RenderContext()`, which emits the task ID, description, task type (if set), body (if set), integration notes (if set), deliverables, acceptance criteria, constraints, references with inline spec content for small `.md` files (<8000 bytes), task state, failure count, and last failure type with human-readable explanation. When `nodeDir` is non-empty and a file `{taskID}.md` exists in that directory, its trimmed content is injected after the description line but before the remaining task metadata. If the task is not found, this section is empty and downstream sections that depend on the task (class guidance, failure context, summary check) are skipped.

4. **Class guidance.** If the task has a `Class` field set, calls `classes.Resolve(task.Class)`. On success, the returned prompt content is appended under a `## Class Guidance` header. Resolution errors are silently ignored (the class prompt is advisory, not required).

5. **Audit context.** Calls `ns.Audit.RenderContext()`, which emits the last 10 breadcrumbs and audit scope when present. Returns empty string when both are absent.

6. **Summary requirement.** Calls `shouldIncludeSummary(ns, taskID, cfg)`. When true, calls `renderSummaryRequired()` and appends the result under a `## Summary Required` header. This section tells the model to run `wolfcastle audit summary` before emitting `WOLFCASTLE_COMPLETE`.

7. **Failure context.** Emitted only when `task.FailureCount > 0` and `cfg` is non-nil. Calls `renderFailureContext(task, cfg)`, which loads the `context-headers.md` template with failure policy variables and, when `task.NeedsDecomposition` is true, appends the `decomposition.md` template with the node address.

**Return value:** the concatenated Markdown string. Sections are separated by newlines to produce readable output. If `taskID` is not found in `ns.Tasks`, the output contains only the node address header and node context (type, state, specs).

## Internal Helpers

These methods are unexported. They encapsulate template resolution with hardcoded fallbacks so that `Build` remains a clean composition of domain render calls and template-backed sections.

### shouldIncludeSummary(ns *state.NodeState, taskID string) bool

Returns true when `taskID` is the only non-complete task remaining in `ns.Tasks`. Iterates `ns.Tasks` once: if any task other than the one matching `taskID` has a state other than `state.StatusComplete`, returns false. Returns false if `taskID` is not found at all.

### renderSummaryRequired() string

Loads `summary-required.md` via `prompts.Resolve("summary-required", nil)`. On success, returns the rendered template content. On failure (file missing or template error), returns a hardcoded fallback:

```
This is the last incomplete task in this node. When you complete it,
include a summary of all work done in this node:

`wolfcastle audit summary --node <your-node> "one-paragraph summary of what was accomplished"`

Run this command before emitting WOLFCASTLE_COMPLETE.
```

The fallback ensures summary guidance is always emitted regardless of prompt tier availability.

### renderFailureContext(nodeAddr string, task *state.Task, currentDepth int, cfg *config.Config) string

Produces the failure history and optional decomposition guidance sections. `nodeAddr` is passed through to the decomposition template; `currentDepth` is the node's current decomposition depth (`ns.DecompositionDepth`).

**Failure header:** loads `context-headers.md` via `prompts.Resolve("context-headers", ctx)` where `ctx` is a `FailureHeaderContext` struct:

```go
type FailureHeaderContext struct {
    FailureCount    int    // task.FailureCount
    DecompThreshold int    // cfg.Failure.DecompositionThreshold
    MaxDecompDepth  int    // cfg.Failure.MaxDecompositionDepth
    CurrentDepth    int    // ns.DecompositionDepth
    HardCap         int    // cfg.Failure.HardCap
}
```

On template resolution failure, falls back to a hardcoded block listing the failure count, decomposition threshold, max decomposition depth with current depth, and hard failure cap.

**Decomposition guidance:** appended only when `task.NeedsDecomposition` is true. Loads `decomposition.md` via `prompts.Resolve("decomposition", ctx)` where `ctx` is a `DecompositionContext{NodeAddr: nodeAddr}`. On failure, falls back to hardcoded instructions for creating child nodes and tasks via the wolfcastle CLI.

```go
type DecompositionContext struct {
    NodeAddr string
}
```

## Error Behavior

`Build` does not return an error. All template resolution failures fall back to hardcoded defaults. Class resolution failures are silently swallowed. The method always produces a valid context string, even if degraded.

This design reflects the operational reality: a failed template lookup should never prevent an iteration from running. The hardcoded fallbacks are functionally equivalent to the templates; the templates exist for customizability, not correctness.

## Thread Safety

`ContextBuilder` holds no mutable state. Both `PromptRepository` and `ClassRepository` are individually goroutine-safe (the class repository guards its map with `sync.RWMutex`; the prompt repository's base-tier cache uses `sync.RWMutex`). `Build` may be called concurrently from multiple goroutines without external synchronization.

## Migration Path

`ContextBuilder` replaces `BuildIterationContext`, `BuildIterationContextWithDir`, and `BuildIterationContextFull` in `pipeline/context.go`. The migration proceeds in two steps:

1. **Introduce `ContextBuilder`** with the `Build` method, wiring it through the daemon's pipeline. The existing `buildIterationContext` function and its public wrappers remain temporarily for callers that haven't migrated.

2. **Remove legacy functions** once all call sites use `ContextBuilder.Build`. The `FailureHeaderContext`, `DecompositionContext`, `renderFailureHeader`, `renderDecomposition`, `renderSummaryRequired`, and `isLastIncompleteTask` functions move into `ContextBuilder` as unexported methods or are inlined.

The `ResolvePromptTemplate` function in `pipeline/` (used by the legacy `renderFailureHeader` and `renderDecomposition`) is replaced by `PromptRepository.Resolve`, which provides the same three-tier template resolution through a cleaner interface.
