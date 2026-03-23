# ADR-057: All Prompts Externalized to Overridable Markdown Files

## Status
Accepted

## Date
2026-03-14

## Context
Wolfcastle's three-tier file layering system (ADR-009, ADR-018) allows prompts in `base/prompts/` to be overridden by `custom/prompts/` or `local/prompts/`. Most stage prompts (execute.md, expand.md, file.md, audit.md, summary.md, unblock.md) are already externalized to `internal/project/templates/prompts/` and deployed to `base/prompts/` during `wolfcastle init`.

However, several pieces of model-facing text remain hardcoded in Go source:

1. **Doctor model-assisted fix prompt** (`internal/validate/model_fix.go:28-40`). A system prompt for the "Wolfcastle Doctor" repair agent. Hardcoded with `fmt.Sprintf`, not overridable.

2. **Unblock session preamble** (`cmd/unblock.go:156-159`). Inline instructions appended to the diagnostic context. The `unblock.md` template exists but the code constructs its own preamble instead of using it.

3. **Iteration context instructional text** (`internal/pipeline/context.go`). Markdown headers, decomposition guidance ("Break this leaf into smaller sub-tasks..."), and summary instructions ("This is the last incomplete task...") are hardcoded in the context builder.

4. **Inbox stage context headers** (`internal/daemon/daemon.go:489, 594`): "# Inbox Items to Expand" and "# Expanded Inbox Items to File" are hardcoded.

This means teams cannot customize model behavior for these scenarios without modifying Go source: violating the principle that all model-facing text should be overridable through the tier system.

## Decision

Externalize all model-facing instructional text to `.md` files in `base/prompts/`, loadable via `pipeline.ResolveFragment()` and overridable through `custom/` and `local/` tiers.

### New Prompt Files

| File | Replaces | Content |
|------|----------|---------|
| `doctor.md` | `model_fix.go:28-40` | Doctor repair agent system prompt with `{{.Node}}`, `{{.Issue}}`, `{{.Description}}` template variables |
| `decomposition.md` | `context.go:47-52` | Decomposition instructions shown when `NeedsDecomposition` is true, with `{{.NodeAddr}}` variable |
| `summary-required.md` | `context.go:87-90` | Summary marker instructions for the last task in a node |
| `context-headers.md` | `context.go:40-44` | Failure history section header and field descriptions |
| `expand-context.md` | `daemon.go:489` | Expand stage context preamble |
| `file-context.md` | `daemon.go:594` | File stage context preamble |

### Template Variables

Prompt files use Go `text/template` syntax for dynamic content. The resolver loads the file, then executes it as a template with a context struct:

```go
type PromptContext struct {
    NodeAddr           string
    TaskID             string
    FailureCount       int
    DecompThreshold    int
    MaxDecompDepth     int
    CurrentDepth       int
    HardCap            int
    // ... fields as needed per template
}
```

Simple prompts with no dynamic content (e.g., the expand-context preamble) are loaded as plain strings: no template execution needed.

### Existing Prompts (Already Externalized)

These remain unchanged: they already follow the pattern:

| File | Stage | Purpose |
|------|-------|---------|
| `execute.md` | Execute | Execution agent role and workflow |
| `expand.md` | Expand | Inbox expansion instructions |
| `file.md` | File | Filing agent instructions |
| `audit.md` | Audit | Codebase audit instructions |
| `summary.md` | Summary | Summary generation instructions |
| `unblock.md` | Unblock | Unblock assistant role |
| `script-reference.md` | All stages | CLI command reference |

### Unblock Fix

The `cmd/unblock.go` code currently ignores the existing `unblock.md` template and constructs its own preamble. After this change, it loads `unblock.md` via `pipeline.ResolveFragment()` and appends the diagnostic context, allowing teams to customize the unblock assistant's personality and instructions.

### Loading Pattern

```go
// Load a prompt template, execute with context, return the rendered string.
func ResolvePromptTemplate(wolfcastleDir, promptFile string, ctx any) (string, error) {
    raw, err := ResolveFragment(wolfcastleDir, "prompts/"+promptFile)
    if err != nil {
        return "", err
    }
    // If ctx is nil, return raw content (no template execution)
    if ctx == nil {
        return raw, nil
    }
    tmpl, err := template.New(promptFile).Parse(raw)
    if err != nil {
        return "", fmt.Errorf("parsing prompt template %s: %w", promptFile, err)
    }
    var buf strings.Builder
    if err := tmpl.Execute(&buf, ctx); err != nil {
        return "", fmt.Errorf("executing prompt template %s: %w", promptFile, err)
    }
    return buf.String(), nil
}
```

### Override Example

A team wants to customize the decomposition instructions to reference their own project structure conventions:

```
.wolfcastle/system/custom/prompts/decomposition.md
```

This file completely replaces `base/prompts/decomposition.md` (per ADR-018's file-level replacement semantics), and the model sees the team's custom decomposition guidance instead of the default.

## Consequences
- All model-facing text is overridable through the three-tier system: no Go source changes needed to customize model behavior
- `wolfcastle init` and `wolfcastle update` regenerate `base/prompts/` with defaults, but never touch `custom/` or `local/`
- Template variables enable dynamic content while keeping the prose in Markdown
- Teams can tune model personality, instructions, and output format per stage without forking
- The doctor prompt, decomposition guidance, and summary instructions become first-class configurable surfaces
- Existing prompts (execute.md, expand.md, etc.) are unchanged: this is purely additive

