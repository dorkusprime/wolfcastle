# Prompt Externalization

This spec details the full inventory of model-facing text in Wolfcastle, the externalization plan for each, and the template variable system that enables dynamic content in overridable prompt files. It is the implementation reference for ADR-057.

## Governing ADRs

- ADR-005: Composable Rule Fragments with Sensible Defaults
- ADR-009: Distribution, Project Layout, and Three-Tier File Layering
- ADR-017: Script Reference via Prompt Injection
- ADR-018: Merge Semantics for Config and Prompt Layering
- ADR-057: All Prompts Externalized to Overridable Markdown Files

---

## 1. Current State: Prompt Inventory

### Already Externalized (no changes needed)

| File | Location | Stage | Lines |
|------|----------|-------|-------|
| `execute.md` | `templates/prompts/` | Execute | 35 |
| `expand.md` | `templates/prompts/` | Expand | 33 |
| `file.md` | `templates/prompts/` | File | 46 |
| `audit.md` | `templates/prompts/` | Codebase audit | 15 |
| `summary.md` | `templates/prompts/` | Summary (inline) | 3 |
| `unblock.md` | `templates/prompts/` | Unblock | 11 |
| `script-reference.md` | `templates/prompts/` | All stages | 361 |

### Rule Fragments (already externalized)

| File | Location | Purpose |
|------|----------|---------|
| `git-conventions.md` | `templates/rules/` | Git commit and branch conventions |
| `adr-policy.md` | `templates/rules/` | ADR writing policy |

### Audit Scopes (already externalized)

| File | Location | Purpose |
|------|----------|---------|
| `comments.md` | `templates/audits/` | Code comment quality audit |
| `decomposition.md` | `templates/audits/` | Task decomposition quality audit |
| `dry.md` | `templates/audits/` | DRY principle audit |
| `modularity.md` | `templates/audits/` | Module boundaries audit |

### Still Hardcoded (to be externalized)

| Source Location | Content | New File |
|----------------|---------|----------|
| `validate/model_fix.go:28-40` | Doctor repair agent system prompt | `doctor.md` |
| `cmd/unblock.go:156-159` | Unblock session preamble (bypasses existing unblock.md) | Fix: use existing `unblock.md` |
| `pipeline/context.go:40-52` | Failure history + decomposition instructions | `decomposition-guidance.md` |
| `pipeline/context.go:87-90` | Summary required instructions | `summary-required.md` |
| `daemon/daemon.go:489` | Expand stage context header | `expand-context.md` |
| `daemon/daemon.go:594` | File stage context header | `file-context.md` |

---

## 2. New Prompt Files

### 2.1 `doctor.md`

**Replaces:** `validate/model_fix.go:28-40`

```markdown
# Wolfcastle Doctor

You are Wolfcastle Doctor, a structural repair agent for an autonomous project orchestrator.

An ambiguous state conflict has been found that cannot be resolved deterministically.

## Conflict Details

- **Node:** {{.Node}}
- **Issue Category:** {{.Category}} ({{.FixType}})
- **Description:** {{.Description}}

## Valid States

The four valid node/task states are: `not_started`, `in_progress`, `complete`, `blocked`.

## Instructions

Analyze the conflict and determine the correct state. Consider:
1. What state is most consistent with the node's children (if any)?
2. What state avoids data loss?
3. When in doubt, prefer `not_started` over `in_progress` (safer to re-do work than to skip it).

## Output Format

Output a single JSON object with your resolution. Nothing else.

```json
{"resolution": "not_started|in_progress|complete|blocked", "reason": "explanation"}
```
```

**Template variables:**

| Variable | Type | Source |
|----------|------|--------|
| `{{.Node}}` | string | `issue.Node` |
| `{{.Category}}` | string | `issue.Category` |
| `{{.FixType}}` | string | `issue.FixType` |
| `{{.Description}}` | string | `issue.Description` |

### 2.2 `decomposition-guidance.md`

**Replaces:** `pipeline/context.go:46-52`

```markdown
## Decomposition Required

This task has failed too many times to continue as-is. Break this leaf node into smaller, more focused sub-tasks.

Use the wolfcastle CLI to decompose:

1. Create child nodes:
   `wolfcastle project create --node {{.NodeAddr}} --type leaf "<descriptive-name>"`

2. Add tasks to each child:
   `wolfcastle task add --node {{.NodeAddr}}/<child-slug> "<task description>"`

3. Emit `WOLFCASTLE_YIELD` when decomposition is complete.

The parent node will automatically convert from leaf to orchestrator when the first child is created.
```

**Template variables:**

| Variable | Type | Source |
|----------|------|--------|
| `{{.NodeAddr}}` | string | Current node address |

### 2.3 `summary-required.md`

**Replaces:** `pipeline/context.go:87-90`

```markdown
## Summary Required

This is the last incomplete task in this node. When you complete it, include a summary of all work done in this node.

Emit the summary on its own line using this marker:

`WOLFCASTLE_SUMMARY: <one-paragraph summary of what was accomplished and why it matters>`

Emit the summary line before `WOLFCASTLE_COMPLETE`.
```

**Template variables:** None — this is static text.

### 2.4 `expand-context.md`

**Replaces:** `daemon/daemon.go:489`

```markdown
# Inbox Items to Expand

The following inbox items need to be broken down into concrete projects and tasks. For each item, determine:
- What projects/sub-projects should be created
- What tasks should be added to each project
- What audit scopes should be defined
```

**Template variables:** None — item details are appended after this preamble.

### 2.5 `file-context.md`

**Replaces:** `daemon/daemon.go:594`

```markdown
# Expanded Inbox Items to File

The following items have been expanded and need to be filed into the project tree. For each item:
- Create the appropriate project nodes if they don't exist
- Add tasks with clear descriptions and acceptance criteria
- Define audit scopes for each node
- Ensure tasks are properly ordered with audit tasks last
```

**Template variables:** None — item details are appended after this preamble.

---

## 3. Template Variable System

### Resolver Function

```go
// ResolvePromptTemplate loads a prompt file through the three-tier system,
// then executes it as a Go text/template with the given context.
// If ctx is nil, the raw file content is returned without template execution.
func ResolvePromptTemplate(wolfcastleDir, promptFile string, ctx any) (string, error) {
    raw, err := ResolveFragment(wolfcastleDir, "prompts/"+promptFile)
    if err != nil {
        return "", fmt.Errorf("resolving prompt %s: %w", promptFile, err)
    }
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

### Context Structs

Each prompt that uses template variables defines a context struct:

```go
// DoctorPromptContext provides variables for doctor.md.
type DoctorPromptContext struct {
    Node        string
    Category    string
    FixType     string
    Description string
}

// DecompositionPromptContext provides variables for decomposition-guidance.md.
type DecompositionPromptContext struct {
    NodeAddr string
}
```

### Backward Compatibility

Existing prompts (execute.md, expand.md, etc.) contain no `{{` template syntax and are loaded via `ResolveFragment()` as before. The new `ResolvePromptTemplate()` function is only called for prompts that need template variables. This means existing custom/local overrides continue to work unchanged.

---

## 4. Override Examples

### Custom Doctor Prompt

A team wants the doctor model to be more conservative — always choosing `not_started`:

```
.wolfcastle/system/custom/prompts/doctor.md
```

```markdown
# Wolfcastle Doctor (Conservative)

Node {{.Node}} has a state conflict: {{.Description}}

Always resolve ambiguous state conflicts to `not_started`. It is safer to re-execute work than to skip it.

Output: {"resolution": "not_started", "reason": "conservative policy — re-execute rather than skip"}
```

### Custom Decomposition Guidance

A team uses a specific project structure convention:

```
.wolfcastle/system/custom/prompts/decomposition-guidance.md
```

```markdown
## Decomposition Required

This task needs to be broken into smaller pieces following our team's convention:
- Each sub-task should be scoped to a single file or module
- Use the naming pattern: `<parent>-<aspect>` (e.g., `auth-api`, `auth-tests`, `auth-docs`)
- Never create more than 5 sub-tasks from a single decomposition

Use the wolfcastle CLI:
1. `wolfcastle project create --node {{.NodeAddr}} --type leaf "<name>"`
2. `wolfcastle task add --node {{.NodeAddr}}/<slug> "<description>"`
3. Emit `WOLFCASTLE_YIELD` when done.
```

### Local Summary Override

A developer wants more detailed summaries:

```
.wolfcastle/system/local/prompts/summary-required.md
```

```markdown
## Summary Required

This is the last task. Write a detailed summary covering:
1. What was implemented and the approach taken
2. What alternatives were considered and rejected
3. What edge cases were handled
4. What tests were added

Format: `WOLFCASTLE_SUMMARY: <your detailed summary>`

Emit before `WOLFCASTLE_COMPLETE`.
```

---

## 5. Deployment

### `wolfcastle init`

Writes all prompt files to `base/prompts/`:

```
.wolfcastle/system/base/prompts/
├── audit.md
├── decomposition-guidance.md    # NEW
├── doctor.md                    # NEW
├── execute.md
├── expand.md
├── expand-context.md            # NEW
├── file.md
├── file-context.md              # NEW
├── script-reference.md
├── summary.md
├── summary-required.md          # NEW
└── unblock.md
```

### `wolfcastle update`

Regenerates `base/prompts/` (including new files) without touching `custom/` or `local/`.

### Embedded Templates

All new prompt files are added to `internal/project/templates/prompts/` and embedded via `go:embed` (ADR-033). The `scaffold.go` function copies them to `base/prompts/` during init/update.

---

## 6. Migration

### Code Changes

| File | Change |
|------|--------|
| `validate/model_fix.go` | Replace `fmt.Sprintf` prompt with `pipeline.ResolvePromptTemplate("doctor.md", ctx)` |
| `cmd/unblock.go` | Replace hardcoded preamble with `pipeline.ResolveFragment("prompts/unblock.md")` + diagnostic context |
| `pipeline/context.go` | Load `decomposition-guidance.md` and `summary-required.md` via `ResolvePromptTemplate` instead of inline strings |
| `daemon/daemon.go` | Load `expand-context.md` and `file-context.md` via `ResolveFragment` instead of inline strings |
| `pipeline/prompt.go` | Add `ResolvePromptTemplate()` function |
| `internal/project/scaffold.go` | Include new prompt files in embedded template set |

### Iteration Context Headers

The metadata headers in `context.go` (`**Node:**`, `**Task:**`, etc.) are data formatting, not instructional text. These remain in Go code — they are structural, not behavioral, and overriding them would break the daemon's parsing expectations.

The boundary: **instructional text** (tells the model what to do) goes to `.md` files. **Structural formatting** (formats data for the model to read) stays in Go code.
