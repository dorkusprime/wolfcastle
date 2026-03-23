# ContextBuilder Contract

## Overview

`ContextBuilder` in `internal/pipeline` composes the full iteration context string that the daemon feeds to the execute-stage model. It replaces the monolithic `buildIterationContext()` function in `pipeline/context.go` by delegating rendering to domain-owned `RenderContext()` methods on `state.NodeState`, `state.Task`, and `state.AuditState`, then layering in class guidance and prompt-template-backed sections (summary requirements, failure context) that require repository access.

The struct holds two repository dependencies and no mutable state of its own; it is safe for concurrent use without synchronization.

## Type

```go
type ContextBuilder struct {
    prompts       *PromptRepository
    classes       *ClassRepository
    wolfcastleDir string

    // Cached parsed templates, resolved once at construction time.
    // nil when the corresponding prompt file is missing (fallback text is used).
    tmplSummary       *template.Template
    tmplFailHeader    *template.Template
    tmplDecomposition *template.Template
}
```

`prompts` resolves three-tier prompt templates (`summary-required.md`, `context-headers.md`, `decomposition.md`). `classes` resolves behavioral guidance prompts keyed by task class. Neither field is optional; both must be provided at construction. `wolfcastleDir` is the root `.wolfcastle/` directory, used to locate codebase knowledge files; pass `""` to disable knowledge injection. The three `tmpl*` fields cache parsed templates at construction time for the builder's lifetime.

## Constructor

### NewContextBuilder(prompts *PromptRepository, classes *ClassRepository, wolfcastleDir string) *ContextBuilder

Returns a ready-to-use `ContextBuilder`. Panics if either repository argument is nil: both are required for correct context assembly. Templates (`summary-required`, `context-headers`, `decomposition`) are parsed eagerly via `cacheTemplate` and stored for the builder's lifetime; missing prompt files are tolerated (fallback text is used at render time). `wolfcastleDir` is stored for knowledge file lookup; pass `""` to disable knowledge injection.

## Methods

### Build(nodeAddr string, nodeDir string, ns *state.NodeState, taskID string, namespace string, cfg *config.Config) (string, error)

Assembles the complete iteration context for a single task within a node. Returns the Markdown-formatted context string and an error. Returns an error when `taskID` does not match any task in the node.

**Parameters:**

- `nodeAddr`: slash-delimited node address (e.g. `"my-project/auth"`), rendered at the top of the output
- `nodeDir`: filesystem path to the node directory; when non-empty, per-task `.md` files (`{taskID}.md`) are read from this directory and their content is injected into the task context after the description line. May be empty to skip per-task file loading.
- `ns`: the node's persistent state, supplying node metadata, task list, audit trail, and linked specs
- `taskID`: identifies the active task within `ns.Tasks`
- `namespace`: the engineer namespace for knowledge file lookup; pass `""` to skip knowledge injection
- `cfg`: daemon configuration, used for failure policy thresholds and decomposition settings; may be nil for backward compatibility (failure context is skipped when nil)

**Assembly order:**

The output is built by appending sections in this fixed order. Each section is emitted only when its data is non-empty.

1. **Node address header.** `**Node:** {nodeAddr}` on its own line.

2. **Node context.** Calls `ns.RenderContext(taskID)`, which emits node type, node state, and linked specs. The node address is not part of `RenderContext`; it comes from step 1.

3. **Task context.** Locates the task matching `taskID` in `ns.Tasks` via `findTask()`. If the task is not found, `Build` returns an error (not an empty context). The task ID line is rendered with the full node address prefix: `**Task:** {nodeAddr}/{taskID}`. Calls `task.RenderContext()` for the remaining task metadata (description, task type, body, integration notes, deliverables, acceptance criteria, constraints, references, task state, failure count, last failure type). The duplicate `**Task:**` line from `RenderContext` is stripped to avoid redundancy. When `nodeDir` is non-empty and a file `{taskID}.md` exists in that directory, its trimmed content is injected after the description line but before the remaining task metadata.

4a. **Universal guidance.** Calls `prompts.ResolveRaw("prompts/classes", "universal.md")`. On success, the returned content is appended under a `## Universal Guidance` header. If the file is missing, this section is silently skipped. Universal guidance is always injected regardless of the task's class.

4b. **Class guidance.** If the task has a `Class` field set, calls `classes.Resolve(task.Class)`. On success, the returned prompt content is used. If the class is empty or resolution fails, falls back to `prompts.ResolveRaw("prompts/classes", "coding/default.md")`. If class guidance content was obtained (from either path), it is appended under a `## Class Guidance` header.

4c. **Codebase knowledge.** Emitted only when both `wolfcastleDir` and `namespace` are non-empty. Calls `knowledge.Read(wolfcastleDir, namespace)`. On success (and non-empty content), the result is appended under a `## Codebase Knowledge` header. Knowledge content is read fresh every iteration, never cached. Missing or empty knowledge files are silently skipped.

5. **Prior task AARs.** Calls `state.RenderAARs(ns.AARs)`, which emits After Action Reviews from previously completed tasks in the node. Returns empty string when no AARs are present.

6. **Audit context.** Calls `ns.Audit.RenderContext()`, which emits the last 10 breadcrumbs and audit scope when present. Returns empty string when both are absent.

7. **Summary requirement.** Calls `shouldIncludeSummary(ns, taskID)`. When true, calls `renderSummaryRequired()` and appends the result under a `## Summary Required` header. This section tells the model to run `wolfcastle audit summary` before emitting `WOLFCASTLE_COMPLETE`.

8. **Failure context.** Emitted only when `task.FailureCount > 0` and `cfg` is non-nil. Calls `renderFailureContext(nodeAddr, task, ns.DecompositionDepth, cfg)`, which loads the cached `context-headers` template with failure policy variables and, when `task.NeedsDecomposition` is true, appends the cached `decomposition` template with the node address.

**Return value:** the concatenated Markdown string and nil error on success. Sections are separated by newlines to produce readable output. If `taskID` is not found in `ns.Tasks`, an error is returned.

## Internal Helpers

These methods are unexported. They encapsulate template resolution with hardcoded fallbacks so that `Build` remains a clean composition of domain render calls and template-backed sections.

### cacheTemplate(name string) *template.Template

Loads and parses a prompt template by name via `prompts.Resolve(name, nil)`. Returns the parsed `*template.Template` on success, or nil when the prompt file is missing or fails to parse. Called once at construction time for each of the three template slots.

### shouldIncludeSummary(ns *state.NodeState, taskID string) bool

Returns true when `taskID` is the only non-complete task remaining in `ns.Tasks`. Iterates `ns.Tasks` once: if any task other than the one matching `taskID` has a state other than `state.StatusComplete`, returns false. Returns false if `taskID` is not found at all. Note: this method does not take a `cfg` parameter.

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

**Failure header:** renders the cached `tmplFailHeader` template (or falls back to hardcoded text when nil) with a `FailureHeaderContext` struct:

```go
type FailureHeaderContext struct {
    FailureCount    int    // task.FailureCount
    DecompThreshold int    // cfg.Failure.DecompositionThreshold
    MaxDecompDepth  int    // cfg.Failure.MaxDecompositionDepth
    CurrentDepth    int    // ns.DecompositionDepth
    HardCap         int    // cfg.Failure.HardCap
}
```

On template execution failure (or when the cached template is nil), falls back to a hardcoded block listing the failure count, decomposition threshold, max decomposition depth with current depth, and hard failure cap.

**Decomposition guidance:** appended only when `task.NeedsDecomposition` is true. Renders the cached `tmplDecomposition` template with a `DecompositionContext{NodeAddr: nodeAddr}`. On failure (or when the cached template is nil), falls back to hardcoded instructions for creating child nodes and tasks via the wolfcastle CLI.

```go
type DecompositionContext struct {
    NodeAddr string
}
```

## Error Behavior

`Build` returns an error when `taskID` is not found in the node's task list. All template resolution failures fall back to hardcoded defaults. Class resolution failures fall back to `coding/default.md`. Universal guidance and codebase knowledge failures are silently skipped. When the task is found, `Build` always produces a valid context string, even if some optional sections are degraded.

This design reflects the operational reality: a failed template lookup should never prevent an iteration from running. The hardcoded fallbacks are functionally equivalent to the templates; the templates exist for customizability, not correctness. However, a missing task is a programming error that the caller should handle.

## Thread Safety

`ContextBuilder` holds no mutable state after construction. The cached `*template.Template` fields are set once during `NewContextBuilder` and only read thereafter. Both `PromptRepository` and `ClassRepository` are individually goroutine-safe (the class repository guards its map with `sync.RWMutex`; the prompt repository's base-tier cache uses `sync.RWMutex`). `Build` may be called concurrently from multiple goroutines without external synchronization. Knowledge files are read fresh each call via `knowledge.Read`, which performs stateless filesystem reads.

## Migration Path

`ContextBuilder` replaces `BuildIterationContext`, `BuildIterationContextWithDir`, and `BuildIterationContextFull` in `pipeline/context.go`. The migration proceeds in two steps:

1. **Introduce `ContextBuilder`** with the `Build` method, wiring it through the daemon's pipeline. The existing `buildIterationContext` function and its public wrappers remain temporarily for callers that haven't migrated.

2. **Remove legacy functions** once all call sites use `ContextBuilder.Build`. The `FailureHeaderContext`, `DecompositionContext`, `renderFailureHeader`, `renderDecomposition`, `renderSummaryRequired`, and `isLastIncompleteTask` functions move into `ContextBuilder` as unexported methods or are inlined.

The `ResolvePromptTemplate` function in `pipeline/` (used by the legacy `renderFailureHeader` and `renderDecomposition`) is replaced by `PromptRepository.Resolve`, which provides the same three-tier template resolution through a cleaner interface.
