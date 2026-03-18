# RenderContext Rendering Contract

## Overview

Three domain types each expose a `RenderContext` method that renders their portion of an iteration prompt as Markdown. These methods live in `internal/state/render.go`. The pipeline's `ContextBuilder` (`internal/pipeline/context.go`) composes their outputs into a complete iteration context, adding filesystem-dependent sections and instructional guidance that domain types cannot (and should not) produce.

This spec codifies the output format, section ordering, edge-case behavior, and the division of responsibility between these two layers.

## Method Signatures

```go
func (t *Task) RenderContext() string
func (ns *NodeState) RenderContext(taskID string) string
func (a *AuditState) RenderContext() string
```

`NodeState.RenderContext` accepts a `taskID` parameter reserved for future ContextBuilder consolidation. The parameter is not used in the method body today (see ADR on the phantom parameter).

## Task.RenderContext

Renders task metadata in isolation. Does not perform filesystem I/O for per-task `.md` files (that responsibility belongs to ContextBuilder).

### Section Ordering

1. `**Task:** {ID}` (task ID only, no node address prefix)
2. `**Description:** {Description}`
3. `**Task Type:** {TaskType}` (omitted when empty)
4. `## Task Details` section with Body content (omitted when Body is empty)
5. `## Integration` section (omitted when Integration is empty)
6. `**Deliverables:**` bulleted list, each path in backtick code spans (omitted when slice is empty)
7. `**Acceptance Criteria:**` bulleted list (omitted when empty)
8. `**Constraints:**` bulleted list (omitted when empty)
9. `**Reference Material:**` bulleted list of paths, followed by inlined `.md` content (omitted when References is empty)
10. `**Task State:** {State}` (always present)
11. `**Failure Count:** {N}` (only when FailureCount > 0)
12. `## Previous Attempt Failed` section with failure explanation (only when FailureCount > 0 and LastFailureType is non-empty)

### Reference Inlining

After listing all reference paths, the method makes a second pass over References. For each entry ending in `.md`:

- Reads the file with `os.ReadFile`
- Trims whitespace
- Inlines the content under a `### Reference: {path}` header **only if** the file is readable, non-empty after trimming, and strictly less than 8000 characters (bytes, measured on the trimmed string)
- Files that fail any of these conditions are silently skipped; they still appear in the bulleted list above

### Failure Type Messages

Three recognized failure types produce specific guidance text:

| LastFailureType | Message |
|---|---|
| `no_terminal_marker` | Reminds the agent to emit exactly one terminal marker |
| `no_progress` | States that WOLFCASTLE_COMPLETE requires committed git changes |
| (anything else) | Echoes the raw failure reason string |

### Empty-Field Omission Rule

Every optional section is guarded by a zero-value check. If the backing field is the zero value for its type (empty string, nil/empty slice, zero int), the section is not emitted at all: no header, no blank line, nothing. This keeps the prompt compact and avoids confusing agents with empty placeholders.

## NodeState.RenderContext

Renders node-level metadata: type, state, and linked specs.

### Section Ordering

1. `**Node Type:** {Type}` (always present)
2. `**Node State:** {State}` (always present)
3. `## Linked Specs` bulleted list (omitted when Specs is empty)

The method does not render the node address. ContextBuilder provides that.

## AuditState.RenderContext

Renders the audit trail. Returns the empty string when both Breadcrumbs and Scope are absent.

### Section Ordering

1. `## Recent Breadcrumbs` (omitted when Breadcrumbs is empty)
2. `## Audit Scope` with scope description (omitted when Scope is nil)

### Breadcrumb Cap

When more than 10 breadcrumbs exist, only the most recent 10 are rendered. The slice is windowed from `len(Breadcrumbs) - 10` onward. Each breadcrumb is formatted as:

```
- [{timestamp}] {task}: {text}
```

Timestamp format: `2006-01-02T15:04Z` (Go reference time, minute precision, UTC).

## ContextBuilder Assembly

`buildIterationContext` (the private core function behind three public entry points) composes the final iteration context. It does not call the domain `RenderContext` methods directly; instead it duplicates their rendering logic, augmenting it with filesystem access, config-driven failure policy, and instructional sections that domain types cannot produce.

### Public Entry Points

| Function | wolfcastleDir | nodeDir | Use Case |
|---|---|---|---|
| `BuildIterationContext` | empty | empty | Tests, backward compatibility |
| `BuildIterationContextWithDir` | provided | empty | Standard iteration (no per-task .md) |
| `BuildIterationContextFull` | provided | provided | Full iteration with per-task .md files |

### Composite Section Ordering

1. **Node header** (always):
   - `**Node:** {nodeAddr}`
   - `**Node Type:** {Type}`
   - `**Node State:** {State}`
   - Blank line

2. **Task block** (single-pass search for matching taskID):
   - `**Task:** {nodeAddr}/{taskID}` (full qualified address, unlike domain method)
   - `**Description:**`
   - Per-task `.md` content (only when nodeDir is provided; reads `{nodeDir}/{taskID}.md`, trims, includes if non-empty)
   - `**Task Type:**` through `**Reference Material:**` with inlining (same logic as Task.RenderContext)
   - `**Task State:**`, `**Failure Count:**`, `## Previous Attempt Failed` (same logic)
   - `## Failure History` section with policy thresholds (only when FailureCount > 0 AND config is provided)
   - Decomposition instructions (only when `NeedsDecomposition` is true, nested under failure context)

3. **Audit breadcrumbs** (same cap-of-10 logic as AuditState.RenderContext, reading from `ns.Audit.Breadcrumbs`)

4. **Audit scope** (same logic as AuditState.RenderContext, reading from `ns.Audit.Scope`)

5. **Linked Specs** bulleted list (from `ns.Specs`)

6. **Summary Required** section (conditional: only when the task was found AND it is the last incomplete task in the node, determined by `isLastIncompleteTask`)

### Template Resolution

Three instructional sections use externalized templates with hardcoded fallbacks:

| Section | Template File | Fallback Behavior |
|---|---|---|
| Failure History | `context-headers.md` | Shows failure count, decomposition threshold, max depth (current), hard cap |
| Decomposition | `decomposition.md` | Step-by-step CLI instructions for breaking the leaf into child nodes |
| Summary Required | `summary-required.md` | Instructs agent to run `wolfcastle audit summary` before WOLFCASTLE_COMPLETE |

Templates are resolved via `ResolvePromptTemplate(wolfcastleDir, name, ctx)`. When `wolfcastleDir` is empty or resolution fails, the hardcoded fallback is used. This makes the builder functional without a `.wolfcastle/` directory (useful in tests).

### Last-Incomplete-Task Detection

`isLastIncompleteTask(ns, taskID)` returns true when every task in the node other than `taskID` has state `StatusComplete`. This triggers the Summary Required section, prompting the agent to write a node-level audit summary before finishing.

## Division of Responsibility

| Concern | Domain RenderContext | ContextBuilder |
|---|---|---|
| Task metadata fields | Yes | Duplicated (augmented) |
| Node address in task line | No (ID only) | Yes (full path) |
| Per-task .md file content | No | Yes (requires nodeDir) |
| Reference inlining | Yes (os.ReadFile) | Yes (duplicated) |
| Failure policy context | No | Yes (requires config) |
| Decomposition guidance | No | Yes (requires config) |
| Breadcrumb cap of 10 | Yes | Yes (duplicated) |
| Audit scope rendering | Yes | Yes (duplicated) |
| Linked specs | Yes | Yes (duplicated) |
| Summary guidance | No | Yes (requires task scan) |
| Template resolution | No | Yes (requires wolfcastleDir) |

The domain methods provide a self-contained, dependency-free rendering path suitable for testing and simple consumers. ContextBuilder provides the full, production-grade composition with filesystem and config access. The duplication is intentional: domain types stay pure (no config, no filesystem beyond reference inlining), while ContextBuilder owns the orchestration concerns.

## Edge Cases Summary

| Behavior | Threshold / Condition | Effect |
|---|---|---|
| Empty field omission | Field is zero value | Section not rendered |
| Breadcrumb cap | > 10 breadcrumbs | Only last 10 shown |
| Reference inlining | `.md` suffix, readable, non-empty, < 8000 chars | Content inlined under `### Reference:` |
| Reference inlining skip | Non-`.md`, unreadable, empty, or >= 8000 chars | Silently skipped (path still listed) |
| AuditState empty return | No breadcrumbs and no scope | Returns empty string |
| Failure history | FailureCount > 0 and config present | Renders thresholds and policy |
| Summary guidance | Last incomplete task in node | Renders summary-required instructions |
| Per-task .md | nodeDir provided and file exists | Content inserted after Description |
