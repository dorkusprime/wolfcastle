# ADR-023: Decisions Emerging from Spec Phase

## Status
Accepted

## Date
2026-03-12

## Context
During the spec-writing phase, several implementation-level decisions were made that go beyond what the original ADRs covered. These are captured here as a consolidated record for review.

## Decisions

### 1. Single state.json per engineer namespace
The tree addressing spec chose a single `state.json` file per engineer namespace (`.wolfcastle/system/projects/wild-macbook/state.json`) rather than one file per node. This keeps the state atomic — one file to read, one file to write, no partial-update risks.

**Status: Resolved by ADR-024** — state is now distributed as one state.json per node.

### 2. Task descriptions inline in state.json
The tree addressing spec put task descriptions as `description` fields inside the state JSON rather than as separate Markdown files. This contradicts the earlier discussion where we agreed task descriptions should be Markdown files.

**Status: Resolved by ADR-024** — hybrid approach: brief description in state.json, optional Markdown working document per task.

### 3. Project definition files as sibling Markdown
Project/sub-project descriptions are stored as Markdown files alongside the state, mirroring the tree path: `.wolfcastle/system/projects/wild-macbook/attunement-tree/fire-impl.md`. These are model-written documentation, not state.

### 4. Pipeline stage `enabled` and `skip_prompt_assembly` fields
The pipeline spec added two optional fields to stage definitions:
- `enabled` (boolean, default true) — static opt-out of a stage
- `skip_prompt_assembly` (boolean, default false) — lightweight stages skip the full system prompt

### 5. Arrays replace entirely in config merge
The config spec formalized that arrays in a higher tier completely replace arrays from lower tiers. No element-level merging. This applies to `pipeline.stages`, model `args`, `prompts.fragments`, etc.

### 6. Null deletion in config merge
Setting a key to `null` in a higher tier (e.g. `local/config.json`) removes it from the resolved config. Allows local config to explicitly disable team settings.

### 7. Validation commands formalized
The config spec defined `validation.commands` as an ordered list of objects with `name`, `run`, and `timeout_seconds`. These run after task completion before marking done.

### 8. Audit has its own status lifecycle
The audit spec introduced a separate status for the audit itself: `pending`, `in_progress`, `passed`, `failed`. This is distinct from the task status of the audit task.

### 9. Gap tracking with open/fixed status
Audit gaps have their own lifecycle: found during audit (open), fixed during audit (fixed), or escalated to parent. Gaps have deterministic IDs for traceability.

### 10. Orchestrator prompt execution protocol
The execute stage follows: Claim → Study → Implement → Validate → Record → Document → Commit → Signal → Pre-block → Follow-up. All within a single model invocation per iteration. Originally seven phases; expanded to ten as documentation, pre-blocking, and follow-up creation were formalized.

### 11. JSON envelope for model-facing commands
CLI commands that the model calls return a consistent JSON envelope: `{ok: boolean, action: string, ...}` for reliable parsing.

### 12. Commit message template with placeholders
Default format: `wolfcastle: {action} [{node}]` with `{action}`, `{node}`, `{user}`, `{machine}` placeholders.

### 13. Fragment ordering
If `prompts.fragments` is empty (default), all fragments are auto-discovered and included in alphabetical order. Explicit ordering via the array overrides this.

## Consequences
Items previously marked "Needs review" have been resolved by ADR-024. All other decisions are consistent with the ADR architecture and can proceed.
