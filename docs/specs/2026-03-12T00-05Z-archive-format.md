# Archive Entry Format and Rollup Process

This spec defines the structure of archive entries in `.wolfcastle/archive/`, the filename convention, the rollup process that generates them, and the optional summary stage. It is the implementation reference for `wolfcastle archive add`.

## Related ADRs

- ADR-002: JSON for Configuration and State
- ADR-007: Audit Model Preserved, Mechanics via Scripts
- ADR-008: Tree-Addressed Operations
- ADR-009: Distribution, Project Layout, and Three-Tier File Layering
- ADR-011: ISO 8601 Timestamp Filenames for ADRs and Specs
- ADR-013: Model Invocation via CLI Shell-Out with Pipeline Configuration
- ADR-016: Archive Format with Deterministic Rollup and Model Summary
- ADR-021: CLI Command Surface

---

## 1. Filename Format

Archive entries live in `.wolfcastle/archive/` and follow the ISO 8601 timestamp convention from ADR-011.

### Pattern

```
{timestamp}-{slug}.md
```

### Timestamp

ISO 8601 with minute precision in UTC. Colons are replaced with hyphens for filesystem safety.

Format: `YYYY-MM-DDTHH-MMZ`

The timestamp is the moment `wolfcastle archive add` executes, not the moment the node completed or the summary was generated.

### Slug Derivation

The slug is derived from the leaf node path in the tree:

1. Take the full tree-addressed node path (e.g., `attunement-tree/fire-impl/wire-stamina-cost`).
2. Replace path separators (`/`) with hyphens (`-`).
3. Lowercase the entire string (tree paths should already be lowercase, but normalize defensively).
4. Collapse any consecutive hyphens into a single hyphen.
5. Truncate to 80 characters if longer, trimming at the last full word boundary before the limit.

### Examples

| Node path | Timestamp | Filename |
|-----------|-----------|----------|
| `attunement-tree/fire-impl` | `2026-03-12T18-45Z` | `2026-03-12T18-45Z-attunement-tree-fire-impl.md` |
| `skill-system/passive-skills/regen-aura` | `2026-03-13T02-10Z` | `2026-03-13T02-10Z-skill-system-passive-skills-regen-aura.md` |

Because each entry has a unique timestamp and is scoped to a specific node, filename collisions are only possible if two archive operations for different nodes execute within the same UTC minute. In practice this cannot happen because Wolfcastle executes serially (ADR-014).

---

## 2. Archive Entry Template

Every archive entry is a Markdown file with the following sections, in order. The summary section is present only when the summary stage is enabled.

### With Summary (default)

```markdown
# Archive: {node path}

## Summary

{Model-generated plain-language summary of what the node accomplished and why it matters.}

## Breadcrumbs

{Chronological list of breadcrumbs recorded during task execution.}

## Audit

{Audit results: scope verified, gaps found, fixes applied, escalations raised.}

## Metadata

| Field | Value |
|-------|-------|
| Node | `{tree-addressed node path}` |
| Completed | `{ISO 8601 timestamp of node completion}` |
| Archived | `{ISO 8601 timestamp of archive generation}` |
| Engineer | `{user-machine identity}` |
| Branch | `{git branch at archive time}` |
| Commit | `{full git commit SHA at archive time}` |
```

The Commit field contains the full 40-character SHA from `git rev-parse HEAD`, resolved by the archive service at the moment the entry is generated.

### Without Summary (opt-out)

When summary generation is disabled via the `summary.enabled` config gate (set to `false`), the entry omits the Summary section entirely. The heading order becomes: Breadcrumbs, Audit, Metadata. No placeholder text or "summary disabled" marker is inserted.

```json
{
  "summary": {
    "enabled": false
  }
}
```

```markdown
# Archive: {node path}

## Breadcrumbs

{Chronological list of breadcrumbs recorded during task execution.}

## Audit

{Audit results: scope verified, gaps found, fixes applied, escalations raised.}

## Metadata

| Field | Value |
|-------|-------|
| Node | `{tree-addressed node path}` |
| Completed | `{ISO 8601 timestamp of node completion}` |
| Archived | `{ISO 8601 timestamp of archive generation}` |
| Engineer | `{user-machine identity}` |
| Branch | `{git branch at archive time}` |
| Commit | `{full git commit SHA at archive time}` |
```

---

## 3. Section Details

### 3.1 Title

```markdown
# Archive: attunement-tree/fire-impl
```

The H1 uses the full tree-addressed node path exactly as it appears in JSON state. This makes archive entries grep-friendly when searching for a specific node.

### 3.2 Summary

The summary is a model-generated paragraph (or short set of paragraphs) explaining what the node accomplished and why it matters. Per ADR-036, it is generated inline by the executing model (via the `WOLFCASTLE_SUMMARY:` marker) and stored in the node's JSON state before rollup (see Section 5).

The archive script copies the summary text verbatim from JSON state into this section. It does not reformat, truncate, or modify the model's output.

If the summary field in JSON state is empty or null (because the summary stage was disabled or failed), this entire section is omitted.

### 3.3 Breadcrumbs

Breadcrumbs are chronological entries recorded by the model during task execution via `wolfcastle audit breadcrumb --node <path> "text"`. Each breadcrumb is stored in JSON state with a timestamp and the text.

The archive script renders breadcrumbs as a Markdown list, one per line, with timestamps:

```markdown
## Breadcrumbs

- **2026-03-12T18-20Z**. Reviewed existing fire damage calculations in `combat/damage.go`. Current implementation uses flat values; need to convert to percentage-based scaling.
- **2026-03-12T18-25Z**. Added `StaminaCost` field to `FireAttunement` struct. Wired into the combat loop via `ApplyAttunementCost()`.
- **2026-03-12T18-32Z**. Tests passing. `TestFireStaminaCost` covers base case, zero-stamina edge case, and attunement level scaling.
- **2026-03-12T18-38Z**. Audit complete. Verified scope covers stamina cost integration. No gaps found.
```

Breadcrumbs should be richer and more explanatory than terse status notes. They serve as the raw material for the permanent record and for the summary model. The executing model is instructed (via prompt rules) to write breadcrumbs that explain reasoning, reference specific files and functions, and note decisions made.

### 3.4 Audit

The audit section captures the results of the audit task that runs after the node's work tasks complete. The archive script reads the audit results from JSON state and renders them.

Audit data in JSON state includes:

- **Scope**: The `audit.scope` object: `description`, `files`, `systems`, `criteria`.
- **Gaps found**: The `audit.gaps` array: each gap has `description`, `status` (`open` or `fixed`), `fixed_by`, `fixed_at`. Gaps with `status: "fixed"` are rendered as "Fixes applied" in the archive.
- **Escalations**: The `audit.escalations` array: gaps escalated to the parent node via `wolfcastle audit escalate`.
- **Status**: The `audit.status` field: `pending`, `in_progress`, `passed`, or `failed`.
- **Result summary**: The `audit.result_summary` field: model-written summary of audit outcome.

The actual rendering (generated by `state.GenerateAuditReport`) uses a combined "Findings" section with status badges rather than separate "Gaps found" and "Fixes applied" sections:

```markdown
## Audit

**Verdict:** PASSED

## Scope

Verify that all fire skill implementations correctly deduct stamina.

**Criteria:**
- All fire skill tests pass
- Cost cannot exceed current stamina

## Findings

1 total (1 remediated, 0 open)

### Remediated

- **gap-fire-impl-1**: Missing edge case test for zero-stamina scenario. (fixed by fire-impl/audit)

## Escalations

- **escalation-fire-impl-1** [OPEN] from `skill-system/fire-impl`: validateCost() not called in ice-impl
```

When there are no findings or escalations, those sections are omitted entirely (the report includes only what is present).

### 3.5 Metadata

The metadata table captures context for traceability. All values are determined at archive generation time by reading JSON state and querying git.

| Field | Source | Description |
|-------|--------|-------------|
| Node | JSON state: node path | Full tree-addressed path (e.g., `attunement-tree/fire-impl`) |
| Completed | JSON state: node completion timestamp | ISO 8601 UTC timestamp when the node transitioned to Complete |
| Archived | System clock | ISO 8601 UTC timestamp when `wolfcastle archive add` executed |
| Engineer | `local/config.json`: identity | `{user}-{machine}` concatenation (e.g., `wild-macbook`) |
| Branch | `git rev-parse --abbrev-ref HEAD` | Current git branch at archive time |
| Commit | `git rev-parse HEAD` | Full SHA of the current HEAD commit at archive time |

---

## 4. The Rollup Process

`wolfcastle archive add --node <path>` is a daemon-internal command (ADR-021). It generates the archive entry deterministically from JSON state. No model invocation occurs during rollup.

### Step-by-Step

1. **Validate the node**. Read JSON state and confirm the target node exists and is in `complete` state. Exit with an error if the node is not complete.

2. **Read the summary** (if present). Check the node's JSON state for a `summary` field. This field is populated by the summary stage (Section 5) before `archive add` is called. If the field is null, empty, or absent, the summary section will be omitted from the archive entry.

3. **Read breadcrumbs**. Extract the ordered list of breadcrumb entries from the node's JSON state. Each entry has a `timestamp` (ISO 8601) and `text` (string).

4. **Read audit results**. Extract audit data from the node's JSON state: scope verification status, list of gaps found, list of fixes applied, and list of escalations.

5. **Resolve metadata**.
   - **Node path**: from the `--node` argument.
   - **Completed timestamp**: from the node's JSON state (`completed_at` field).
   - **Archived timestamp**: current UTC time.
   - **Engineer identity**: read from resolved config (three-tier merge of `base/config.json`, `custom/config.json`, and `local/config.json`), using `identity.user` and `identity.machine`.
   - **Branch**: execute `git rev-parse --abbrev-ref HEAD`.
   - **Commit**: execute `git rev-parse HEAD`.

6. **Generate the filename**. Derive the slug from the node path (per Section 1). Combine with the archived timestamp to produce the filename.

7. **Render the Markdown**. Assemble the archive entry using the template (Section 2), populating each section from the data gathered above. This is string templating, not model generation.

8. **Write the file**. Write the rendered Markdown to `.wolfcastle/archive/{filename}`.

9. **Return the path**. Output the path of the created archive entry as structured JSON so the daemon can log it and optionally commit it.

### What the Rollup Does NOT Do

- It does not invoke a model. The summary was already generated in a prior stage.
- It does not modify JSON state. It reads state but never writes to it.
- It does not commit to git. The daemon's iteration commit handles that.
- It does not delete or clean up the node's project state. That is a separate concern.

---

## 5. Summary Generation (Inline via ADR-036)

> **Note:** The original design described the summary as a separate pipeline stage with its own model invocation. ADR-036 superseded this approach: summaries are now generated inline by the executing model, eliminating an extra model call.

### How It Works

When the executing model completes the last task in a node, the daemon's `BuildIterationContext` function appends a "Summary Required" section to the prompt. This instructs the model to emit a `WOLFCASTLE_SUMMARY:` marker in its output alongside `WOLFCASTLE_COMPLETE`. The daemon's `applyModelMarkers` function parses this marker and stores the text in the node's `audit.result_summary` field.

No separate model invocation occurs for summarization.

### Where the Summary Lives Before Rollup

The summary is stored in the node's `audit.result_summary` field in its `state.json`:

```json
{
  "id": "fire-impl",
  "state": "complete",
  "audit": {
    "result_summary": "Implemented fire attunement stamina cost...",
    ...
  }
}
```

It lives here until `wolfcastle archive add` reads it and renders it into the Markdown archive entry. The summary is part of the node's committed state (`.wolfcastle/system/projects/` is committed per ADR-009), so it survives daemon restarts.

### Opt-Out

Summary generation is enabled by default. To disable it, set `summary.enabled` to `false` in config:

```json
{
  "summary": {
    "enabled": false
  }
}
```

When summary is disabled, the "Summary Required" section is not appended to the prompt, the `audit.result_summary` field in JSON state remains null, and the archive entry omits the Summary section entirely.

---

## 6. Data Read from JSON State

This section enumerates every field the archive script reads from JSON state and where each field ends up in the rendered entry.

### Node State Fields

| JSON field | Type | Archive section | Notes |
|------------|------|-----------------|-------|
| (node address) | string | Title, Metadata (Node) | Tree-addressed node path, from the `--node` argument |
| `state` | string | (validation only) | Must be `"complete"` or rollup fails |
| `audit.completed_at` | string (ISO 8601) | Metadata (Completed) | Set when the audit task completes |
| `audit.breadcrumbs` | array of objects | Breadcrumbs | Each has `timestamp`, `task` (tree address), and `text` |
| `audit.scope` | object | Audit (Scope) | `description`, `files`, `systems`, `criteria` |
| `audit.gaps` | array of objects | Audit (Gaps found) | Each has `description`, `status` (`open` or `fixed`), `fixed_by`, `fixed_at` |
| `audit.escalations` | array of objects | Audit (Escalations) | Each has `description`, `source_node`, `status` (`open` or `resolved`) |
| `audit.status` | string | Audit (Status) | `pending`, `in_progress`, `passed`, or `failed` |
| `audit.result_summary` | string or null | Summary, Audit (Summary) | Written inline by the executing model via `WOLFCASTLE_SUMMARY:` marker (ADR-036); null if summary is disabled. Used for both the Summary archive section and the Audit summary. |

### External Sources

| Source | Archive section | How resolved |
|--------|-----------------|--------------|
| System clock (UTC) | Metadata (Archived), Filename | `time.Now().UTC()` |
| `local/config.json` → `identity` | Metadata (Engineer) | `{identity.user}-{identity.machine}` |
| `git rev-parse --abbrev-ref HEAD` | Metadata (Branch) | Shell out |
| `git rev-parse HEAD` | Metadata (Commit) | Shell out |

---

## 7. Breadcrumb Quality Expectations

Per ADR-016, breadcrumbs should be richer than Ralph's terse notes. The executing model's prompt rules should instruct it to write breadcrumbs that:

- Explain reasoning, not just actions ("Chose percentage-based scaling because flat values don't account for attunement level", not "Updated damage calc").
- Reference specific files and functions by name.
- Note decisions made and alternatives rejected.
- Record what was verified during audit, not just "audit passed".

This is enforced at the prompt level, not by the archive script. The script renders whatever breadcrumbs are in state. But breadcrumb quality directly affects archive value, because the breadcrumbs are the authoritative chronological record and the primary input to the summary model.

---

## 8. Example Archive Entry

A complete, realistic archive entry with all sections populated.

```markdown
# Archive: attunement-tree/fire-impl

## Summary

Implemented the fire attunement stamina cost system. Fire-attuned attacks now deduct stamina proportional to attunement level, with costs scaling from 5% at level 1 to 15% at level 5. The implementation integrates with the existing combat loop via a new `ApplyAttunementCost()` function called after damage calculation. Edge cases for zero stamina (attack still fires but at reduced damage) and attunement level overflow are handled. All existing combat tests continue to pass, and three new test cases cover the stamina cost logic.

## Breadcrumbs

- **2026-03-12T18-20Z**. Reviewed existing fire damage calculations in `combat/damage.go`. Current implementation uses flat damage values per attunement level. The cost system needs to integrate after `CalculateFireDamage()` but before `ApplyDamage()` in the combat loop.
- **2026-03-12T18-23Z**. Decided on percentage-based stamina cost rather than flat cost. Flat costs would make low-level attunements disproportionately expensive relative to damage output. Percentage-based scaling (5% at L1 to 15% at L5) keeps the cost/damage ratio consistent.
- **2026-03-12T18-28Z**. Added `StaminaCost` field to `FireAttunement` struct in `combat/attunement.go`. Created `ApplyAttunementCost()` in `combat/stamina.go`: takes attunement level, returns stamina deduction as a float. Wired into combat loop in `combat/loop.go` between damage calc and damage application.
- **2026-03-12T18-32Z**. Handled zero-stamina edge case: when stamina reaches zero, fire attacks still execute but deal 50% reduced damage. This matches the design doc in `docs/design/attunement-costs.md`. Added `StaminaDepletedModifier` constant.
- **2026-03-12T18-35Z**. Tests passing. `TestFireStaminaCost` covers: base cost at each attunement level, zero-stamina reduced damage, and attunement level exceeding max (clamped to level 5 cost). Ran full `go test ./combat/...`: 47 tests pass, 0 failures.
- **2026-03-12T18-38Z**. Audit complete. Verified all three tasks (implement cost calc, wire into loop, add tests) are done. Checked that `ApplyAttunementCost()` is only called for fire attunement: other elements are unaffected. No gaps found; implementation matches scope.

## Audit

**Scope**: Verified. All tasks completed within scope.

**Gaps found**: None

**Fixes applied**: None

**Escalations**: None

## Metadata

| Field | Value |
|-------|-------|
| Node | `attunement-tree/fire-impl` |
| Completed | `2026-03-12T18-38Z` |
| Archived | `2026-03-12T18-45Z` |
| Engineer | `wild-macbook` |
| Branch | `feature/attunement-system` |
| Commit | `a1b2c3d4e5f6a7b8c9d0e1f2a3b4c5d6e7f8a9b0` |
```
