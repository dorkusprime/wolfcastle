# Audit Propagation Rules

## Overview

Every node in the Wolfcastle work tree carries an audit. The audit verifies that the node's work is complete, correct, and integrated. Audits propagate upward: leaf audits verify local work, orchestrator audits verify integration across children, and gaps found at any level escalate to the parent for resolution.

This spec defines the audit state schema, breadcrumb format, escalation mechanics, the audit task invariant, scope definition, orchestrator vs. leaf audit behavior, archive integration, and the CLI operations that drive the system.

### Governing ADRs

- **ADR-002**. All state is JSON, mutated only by deterministic scripts
- **ADR-003**. Models call scripts; scripts enforce invariants
- **ADR-007**. Audit concept preserved from Ralph, mechanics via scripts/JSON
- **ADR-008**. Tree-addressed operations
- **ADR-016**. Archive rollup includes audit results
- **ADR-021**. CLI commands: `wolfcastle audit breadcrumb`, `wolfcastle audit escalate`

---

## 1. Audit State Schema

Each node in the work tree contains an `audit` object in its JSON state. This is Wolfcastle-owned state (ADR-002): modified only by deterministic scripts, never by the model directly.

### Schema

```json
{
  "audit": {
    "scope": {
      "description": "string: what this audit must verify",
      "files": ["string: file paths or globs touched by this node"],
      "systems": ["string: subsystems, modules, or integration points"],
      "criteria": ["string: specific verification conditions that must pass"]
    },
    "breadcrumbs": [
      {
        "timestamp": "time.Time. Go time.Time serialized as ISO 8601 (precision determined by clock implementation; typically nanosecond in Go's JSON marshaler)",
        "task": "string: tree address of the task that wrote this breadcrumb",
        "text": "string: what changed, what was done, what to verify"
      }
    ],
    "gaps": [
      {
        "id": "string: deterministic ID: gap-{node-internal-id}-{sequential-int}, where node-internal-id is the node's `id` field (e.g. gap-fire-impl-1)",
        "timestamp": "string. ISO 8601",
        "description": "string: what is missing or broken",
        "source": "string: tree address of the task or audit that found the gap",
        "status": "string: open | fixed",
        "fixed_by": "string | null: tree address of the task that resolved this gap",
        "fixed_at": "string | null. ISO 8601 timestamp of resolution"
      }
    ],
    "escalations": [
      {
        "id": "string: escalation-{node-slug}-{sequential-int}",
        "timestamp": "string. ISO 8601",
        "description": "string: what needs attention at the parent level",
        "source_node": "string: tree address of the child that escalated",
        "source_gap_id": "string | null. ID of the originating gap, if this came from a gap",
        "status": "string: open | resolved",
        "resolved_by": "string | null: tree address of the task that resolved this",
        "resolved_at": "string | null. ISO 8601"
      }
    ],
    "status": "string: pending | in_progress | passed | failed",
    "started_at": "string | null. ISO 8601",
    "completed_at": "string | null. ISO 8601",
    "result_summary": "string | null: brief model-written summary of audit outcome"
  }
}
```

### Field Semantics

- **scope**. Defined when the node is created or during the discovery/expansion phase. Describes what the audit must check. This is the contract: the audit task reads the scope and verifies every item.
- **breadcrumbs**. Append-only log of changes made by tasks in this node. Written during task execution via `wolfcastle audit breadcrumb`. The audit task reads these to understand what happened.
- **gaps**. Issues found during the audit. A gap is something that should have been done but was not, or something that was done incorrectly. Gaps can be fixed by the audit task itself (status transitions to `fixed`) or escalated to the parent.
- **escalations**. Gaps that could not be resolved at this level and were pushed to the parent node's audit. These appear in the *parent's* `escalations` array, not this node's. The child calls `wolfcastle audit escalate`, which writes to the parent.
- **status**. Lifecycle of the audit itself: `pending` (not started), `in_progress` (audit task is executing), `passed` (all scope verified, no open gaps), `failed` (open gaps remain after audit task completed: triggers escalation).

### Initial State

When a node is created, the audit object is initialized to:

```json
{
  "audit": {
    "scope": {
      "description": "",
      "files": [],
      "systems": [],
      "criteria": []
    },
    "breadcrumbs": [],
    "gaps": [],
    "escalations": [],
    "status": "pending",
    "started_at": null,
    "completed_at": null,
    "result_summary": null
  }
}
```

The scope fields are populated either by the model during task expansion/discovery or by the user directly. Scope is part of the node's configuration and can be set before execution begins.

---

## 2. Breadcrumb Format

Breadcrumbs are the primary communication channel between executing tasks and the audit. Every task that modifies code, configuration, or project state should leave a breadcrumb describing what it did and what the audit should verify.

### What Gets Recorded

A breadcrumb captures:
- **What changed**: files modified, functions added, dependencies introduced
- **Why it changed**: the intent behind the change
- **What to verify**: what the audit should check as a consequence

### Who Writes Breadcrumbs

The executing model writes breadcrumbs by calling `wolfcastle audit breadcrumb` during task execution. This is a convention enforced by the system prompt, not a hard gate. Models are instructed to leave breadcrumbs after meaningful changes.

### When Breadcrumbs Are Written

- After completing a logically coherent unit of work within a task (not after every line change)
- Before marking a task as complete: the final breadcrumb should summarize the task's outcome
- When encountering something unexpected that the audit should know about

### Breadcrumb Quality

Breadcrumbs feed directly into the archive (ADR-016). Terse or cryptic breadcrumbs degrade the permanent record. The system prompt instructs the model to write breadcrumbs that a different engineer (or a different model) could read and understand without additional context.

### Example Breadcrumbs

```json
{
  "timestamp": "2026-03-12T18:45:32Z",
  "task": "skill-system/fire-impl/wire-stamina-cost",
  "text": "Added stamina cost calculation to FireSkill.execute(). Cost scales with skill level via the formula in skills.md. Modified: src/skills/fire.ts, src/skills/types.ts. Verify: stamina is deducted before damage is applied, cost cannot exceed current stamina."
}
```

```json
{
  "timestamp": "2026-03-12T18:47:15Z",
  "task": "skill-system/fire-impl/wire-stamina-cost",
  "text": "Found that SkillBase.validateCost() was not called in the execute path: added the call. This affects all skills, not just fire. The audit for skill-system should verify that other skill implementations also pass through validateCost()."
}
```

The second example illustrates a breadcrumb that hints at a cross-cutting concern. The leaf audit may choose to escalate this to the parent orchestrator's audit if verification requires checking sibling nodes.

---

## 2.5. After Action Reviews (AARs)

AARs are a structured retrospective format that complements breadcrumbs. While breadcrumbs capture real-time observations during task execution, AARs capture structured reflection after a task completes.

### AAR Schema

```json
{
  "task_id": "string. ID of the task this AAR covers",
  "timestamp": "time.Time: when the AAR was recorded",
  "objective": "string: what the task set out to do",
  "what_happened": "string: what actually happened",
  "went_well": ["string: things that went well (repeatable)"],
  "improvements": ["string: things that could be improved (repeatable)"],
  "action_items": ["string: follow-up items for subsequent tasks (repeatable)"]
}
```

AARs are stored in the node's `aars` field (a map keyed by task ID, at the top level of the node state, not nested under `audit`) and recorded via `wolfcastle audit aar`. They flow into the iteration context for subsequent tasks, so the executing model can learn from prior task outcomes within the same node. They also feed into archive entries alongside breadcrumbs.

### When AARs Are Written

The executing model writes an AAR after completing a task, particularly when:
- The task involved unexpected challenges or course corrections
- There are lessons that subsequent tasks in the same node should know about
- The approach taken differs from what was originally planned

AARs are richer than breadcrumbs and more structured. A breadcrumb says "did X." An AAR says "tried to do X, Y actually happened, Z went well, W needs improvement."

---

## 3. Escalation Mechanics

Escalation is the mechanism by which a gap found at a lower level propagates upward to the parent's audit. This ensures that integration issues, cross-cutting concerns, and problems that require a broader scope are handled at the appropriate level.

### When Escalation Happens

A gap escalates when:
1. The audit task at a child node finds an issue it cannot resolve because resolution requires changes outside its scope (e.g., a sibling node's code, a shared interface, a project-level configuration).
2. The audit task at a child node finds an issue that is technically fixable locally but has implications that the parent should verify (e.g., a behavior change that might break integration with siblings).
3. A breadcrumb explicitly identifies a cross-cutting concern that the leaf audit cannot fully verify.

### Escalation Flow

```
Leaf Node (fire-impl)
  1. Audit task runs, finds gap: "validateCost() not called in ice-impl"
  2. Gap is local to fire-impl but resolution is in ice-impl (a sibling)
  3. Model calls: wolfcastle audit escalate --node skill-system/fire-impl "validateCost() not called in ice-impl: needs fix in sibling"
  4. Script writes an escalation to the PARENT node (skill-system)'s audit.escalations array
  5. fire-impl's audit can still pass (it verified its own scope): the escalation is recorded at the parent level

Parent Node (skill-system)
  6. When skill-system's audit task runs, it sees the escalation in its escalations array
  7. It verifies whether ice-impl's audit addressed this, or fixes it directly
  8. It marks the escalation as resolved
```

### What the Script Does

`wolfcastle audit escalate --node <child-path> "description"`:
1. Validates that `<child-path>` exists and has a parent node
2. Generates a deterministic escalation ID
3. Appends an escalation record to the **parent** node's `audit.escalations` array
4. Returns the escalation ID and parent node path as JSON

The script does NOT modify the child node's audit state. The child's gap remains as-is. The escalation is a new record on the parent.

### Escalation Chains

If the parent's audit also cannot resolve an escalated issue (because it requires an even broader scope), the parent can escalate further to its own parent. There is no depth limit on escalation chains. In practice, most issues resolve within one or two levels.

### Escalation vs. Blocking

Escalation is not the same as task blocking (ADR-019). A blocked task cannot proceed due to an external dependency. An escalation is an audit finding: the work may be complete, but verification identified something that needs attention at a higher level. A node can complete its audit (status: `passed`) while still having outbound escalations, because the escalations are written to the parent, not to the node itself.

---

## 4. The Audit Task

### Invariant Position

Every node in the work tree has an audit task as its **last** task. This is a structural invariant enforced by the scripts:

- The audit task **cannot be moved** from the last position
- The audit task **cannot be deleted**
- When new tasks are added to a node via `wolfcastle task add`, they are inserted **before** the audit task
- The audit task is created automatically when a node is created

### How It Differs from Regular Tasks

| Property | Regular Task | Audit Task |
|----------|-------------|------------|
| Position | Any position before audit | Always last |
| Deletable | Yes | No |
| Movable | Yes (within pre-audit range) | No |
| Purpose | Produce work | Verify work |
| Scope | Defined by task description | Defined by `audit.scope` + breadcrumbs + escalations |
| Can fix code | Yes | Yes: audit tasks fix issues, not just report them |
| Can add tasks | Via decomposition | No: audit runs after all tasks are done |
| Can escalate | No (tasks do work, not verification) | Yes: `wolfcastle audit escalate` |

### What the Audit Task Does

When the audit task is claimed and executed:

1. **Reads the audit scope**: understands what must be verified
2. **Reads all breadcrumbs**: understands what was done by preceding tasks
3. **Reads inbound escalations** (for orchestrator nodes): understands what children flagged
4. **Verifies each scope criterion**: checks files, runs tests, inspects integration points
5. **Fixes issues it can fix**: the audit task has full code modification capability. If a test is failing due to a trivial bug, the audit task fixes it rather than reporting it. This is a key distinction from a passive audit.
6. **Records gaps**: issues found are recorded as gaps in the node's `audit.gaps` array via script calls
7. **Fixes gaps where possible**: transitions gap status from `open` to `fixed`
8. **Escalates unresolvable gaps**: calls `wolfcastle audit escalate` for issues outside its scope
9. **Writes result summary**: a brief description of what was verified, what was fixed, what was escalated
10. **Completes**: if all scope criteria are met and no open gaps remain, audit status is `passed`. If open gaps remain that could not be fixed or escalated, status is `failed`.

### Audit Task Lifecycle

```
pending → in_progress → passed
                      → failed
```

A `failed` audit prevents the node from being marked complete. The orchestrator (or user) must decide how to proceed: either unblock and retry, or address the underlying issues.

---

## 5. Scope Definition

### Where Scope Lives

Audit scope is part of the node's audit state (the `audit.scope` object). It is set during node creation or during the expansion/discovery phase of the pipeline.

### How Scope Is Defined

Scope can be populated in two ways:

1. **During expansion**. The expansion pipeline stage (or the model during discovery tasks) calls a script to set the audit scope when creating or elaborating a node. This is the typical path for automatically decomposed work.

2. **Via task description**. For nodes created manually or with minimal expansion, the node's task description Markdown may include audit scope hints. The audit task reads the task description and interprets scope from context. This is a fallback, not the primary mechanism.

### Scope Fields

- **description**. Plain-language statement of what the audit covers. Example: "Verify that all fire skill implementations correctly deduct stamina, apply damage, and respect cooldowns."
- **files**. File paths or globs that this node's work should touch. The audit verifies these were modified appropriately. Example: `["src/skills/fire.ts", "src/skills/fire.test.ts"]`
- **systems**. Subsystems or modules involved. Broader than files: captures architectural boundaries. Example: `["skill-system", "stamina-system"]`
- **criteria**. Specific, verifiable conditions. These are the audit's checklist. Example: `["All fire skill tests pass", "Stamina is deducted before damage application", "Cost cannot exceed current stamina"]`

### Scope for Orchestrator vs. Leaf Nodes

- **Leaf nodes**. Scope focuses on the specific work: files modified, tests passing, implementation matching specification.
- **Orchestrator nodes**. Scope focuses on integration: do children compose correctly, are interfaces consistent across children, do cross-cutting concerns hold. Orchestrator scope should NOT duplicate children's scope: it covers what falls between children.

---

## 6. Orchestrator Audits vs. Leaf Audits

### Leaf Audit

A leaf node's audit verifies its own work product:

- Did the tasks in this node accomplish what was specified?
- Do the modified files compile, pass linting, and pass tests?
- Are there edge cases the tasks missed?
- Are breadcrumbs consistent with the actual changes?

The leaf audit has a narrow, concrete scope. It reads the node's breadcrumbs and scope, inspects the code, runs validation, and fixes issues. Most gaps found at the leaf level are fixable by the audit task directly.

### Orchestrator Audit

An orchestrator node's audit verifies integration across its children:

- Do children's outputs compose correctly?
- Are interfaces between children consistent (types, contracts, data formats)?
- Are cross-cutting concerns handled (error handling conventions, logging, naming)?
- Were escalations from children addressed?

The orchestrator audit runs **after all children have completed** (including their own audits). It has access to:
- Its own `audit.scope` (integration-focused)
- Its own `audit.breadcrumbs` (if any: orchestrators may have their own tasks beyond managing children)
- Its `audit.escalations` array (populated by children's `wolfcastle audit escalate` calls)
- The completed state of all child nodes (their audit results, breadcrumbs, and resolved gaps)

### Orchestrator Audit Flow

```
All children complete (including their audits)
  ↓
Orchestrator's audit task is the only remaining task
  ↓
Audit task claims and executes:
  1. Read own scope (integration criteria)
  2. Read escalations from children
  3. Inspect children's audit results for patterns
  4. Verify integration points
  5. Fix integration issues
  6. Resolve escalations (mark as resolved)
  7. Escalate anything it cannot handle to its own parent
  8. Write result summary
  ↓
If passed: orchestrator node can complete
If failed: orchestrator node is blocked until issues are resolved
```

### What Orchestrator Audits Should NOT Do

- Re-verify work that a child audit already verified (avoid duplication)
- Modify code within a child's scope unless fixing an integration issue that spans children
- Ignore escalations: every escalation must be explicitly resolved or re-escalated

---

## 7. Integration with Archive

When a node completes and is archived (ADR-016), the archive rollup includes audit data. The `wolfcastle archive add --node <path>` command extracts audit information from the node's JSON state and writes it into the Markdown archive entry.

### What the Archive Captures

The archive entry's **Audit results** section includes:

1. **Scope summary**: the `audit.scope.description` field
2. **Verification status**: `passed` or `failed`
3. **Criteria checked**: the `audit.scope.criteria` list with pass/fail per item
4. **Gaps found and fixed**: each gap from `audit.gaps` with its description, status, and resolution
5. **Escalations sent**: escalations this node sent to its parent (from the parent's `audit.escalations` where `source_node` matches this node)
6. **Escalations received and resolved** (orchestrator nodes only): escalations from children and their resolution
7. **Result summary**: the model-written `audit.result_summary`

### Archive Entry Structure (Audit Section)

```markdown
## Audit Results

**Status**: Passed
**Scope**: Verify that all fire skill implementations correctly deduct stamina, apply damage, and respect cooldowns.

### Criteria
- [x] All fire skill tests pass
- [x] Stamina is deducted before damage application
- [x] Cost cannot exceed current stamina

### Gaps Found
1. **gap-fire-impl-1** (fixed): `validateCost()` was not called in the execute path. Fixed by adding the call in `src/skills/fire.ts:42`.

### Escalations Sent
1. **escalation-fire-impl-1**: `validateCost()` not called in ice-impl: needs fix in sibling. Sent to `skill-system` audit.

### Summary
All fire skill implementations verified. Stamina deduction logic is correct. One gap found and fixed locally (missing validateCost call). One cross-cutting concern escalated to parent regarding ice-impl.
```

### Rollup for Orchestrator Archives

When an orchestrator node is archived, its audit section additionally includes:

- **Escalations received**: with source child and resolution status
- **Integration verification**: what was checked across children

This provides a complete audit trail in the permanent record: what was verified, what broke, how it was fixed, and what got pushed upward.

---

## 8. Script Operations

### `wolfcastle audit breadcrumb`

Appends a breadcrumb to a node's audit trail.

**Syntax:**
```
wolfcastle audit breadcrumb --node <path> "text"
```

**Input:**
- `--node <path>`. Tree address of the node to append the breadcrumb to (required)
- `"text"`. The breadcrumb content (required, positional argument)

**Behavior:**
1. Validates that `<path>` exists in the work tree
2. Generates a timestamp (current UTC time, ISO 8601 with second precision)
3. Sets the `task` field to the `--node` address provided by the caller (the CLI does not resolve execution context; the model passes whichever address it is working on)
4. Appends a breadcrumb object to `audit.breadcrumbs`:
   ```json
   {
     "timestamp": "2026-03-12T18:45:32Z",
     "task": "<the --node argument value>",
     "text": "<provided text>"
   }
   ```
5. Writes the updated state atomically

**Output (JSON, stdout):**
```json
{
  "ok": true,
  "node": "skill-system/fire-impl",
  "breadcrumb_index": 3,
  "timestamp": "2026-03-12T18:45:32Z"
}
```

**Errors:**
- Exit code 1 if `<path>` does not exist
- Exit code 1 if text is empty

**Invariants enforced:**
- Breadcrumbs are append-only. There is no command to edit or delete a breadcrumb.
- The `task` field is set from the `--node` argument provided by the caller. The model supplies the node address it is working on. (The original design intended resolution from daemon execution context, but the implementation uses the caller-supplied address for simplicity.)

---

### `wolfcastle audit escalate`

Escalates a gap to the parent node's audit.

**Syntax:**
```
wolfcastle audit escalate --node <path> "description"
```

**Input:**
- `--node <path>`. Tree address of the **child** node that is escalating (required). The escalation is written to this node's parent.
- `"description"`. What needs attention at the parent level (required, positional argument)

**Behavior:**
1. Validates that `<path>` exists in the work tree
2. Validates that `<path>` has a parent node (cannot escalate from root)
3. Resolves the parent node's tree address
4. Generates a deterministic escalation ID: `escalation-{child-slug}-{next-sequential-int}`
5. Generates a timestamp (current UTC time)
6. Appends an escalation record to the **parent** node's `audit.escalations`:
   ```json
   {
     "id": "escalation-fire-impl-1",
     "timestamp": "2026-03-12T18:50:00Z",
     "description": "<provided description>",
     "source_node": "skill-system/fire-impl",
     "source_gap_id": null,
     "status": "open",
     "resolved_by": null,
     "resolved_at": null
   }
   ```
7. Writes the updated parent state atomically

**Output (JSON, stdout):**
```json
{
  "ok": true,
  "escalation_id": "escalation-fire-impl-1",
  "source_node": "skill-system/fire-impl",
  "target_node": "skill-system",
  "timestamp": "2026-03-12T18:50:00Z"
}
```

**Errors:**
- Exit code 1 if `<path>` does not exist
- Exit code 1 if `<path>` is the root node (no parent to escalate to)
- Exit code 1 if description is empty

**Invariants enforced:**
- Escalations are append-only on the parent. There is no command to delete an escalation.
- Resolution is a separate operation (setting `status: "resolved"`, `resolved_by`, `resolved_at`) performed by the parent's audit task during its execution.
- The `source_node` is set by the script from the `--node` argument, not from execution context, because the escalating audit task operates within the child node's scope and explicitly identifies itself.

---

### Gap Recording (Internal to Audit Task Execution)

Gaps are recorded during audit task execution. Explicit CLI commands now exist for gap management:

- `wolfcastle audit gap --node <path> "description"`: records a gap
- `wolfcastle audit fix-gap --node <path> --gap <gap-id>`: marks a gap as fixed
- `wolfcastle audit scope --node <path>`: sets or displays audit scope
- `wolfcastle audit resolve --node <path> --escalation <id>`: resolves an escalation
- `wolfcastle audit show --node <path>`: displays audit state

The model uses these commands during audit task execution. The daemon's marker protocol (`WOLFCASTLE_GAP:`, `WOLFCASTLE_FIX_GAP:`, `WOLFCASTLE_SCOPE:`) provides a parallel path for inline gap management during non-audit task execution.

---

## Appendix: Full Audit State Example

A completed leaf node with one gap found and fixed, and one escalation sent:

```json
{
  "audit": {
    "scope": {
      "description": "Verify fire skill implementation: stamina cost, damage application, cooldown behavior",
      "files": ["src/skills/fire.ts", "src/skills/fire.test.ts", "src/skills/types.ts"],
      "systems": ["skill-system", "stamina-system"],
      "criteria": [
        "All fire skill tests pass",
        "Stamina is deducted before damage application",
        "Cost cannot exceed current stamina",
        "Cooldown timer resets after use"
      ]
    },
    "breadcrumbs": [
      {
        "timestamp": "2026-03-12T18:30:00Z",
        "task": "skill-system/fire-impl/implement-base",
        "text": "Implemented FireSkill class extending SkillBase. Added execute(), getCost(), and getCooldown() methods. Modified src/skills/fire.ts."
      },
      {
        "timestamp": "2026-03-12T18:35:00Z",
        "task": "skill-system/fire-impl/wire-stamina-cost",
        "text": "Wired stamina cost into execute path. Cost = baseCost * (1 + level * 0.1). Added validateCost() call before damage application. Modified src/skills/fire.ts, src/skills/types.ts."
      },
      {
        "timestamp": "2026-03-12T18:40:00Z",
        "task": "skill-system/fire-impl/add-tests",
        "text": "Added test suite for FireSkill. 12 tests covering: normal execution, insufficient stamina, cooldown enforcement, level scaling. All passing. Modified src/skills/fire.test.ts."
      },
      {
        "timestamp": "2026-03-12T18:45:00Z",
        "task": "skill-system/fire-impl/wire-stamina-cost",
        "text": "Discovered that SkillBase.validateCost() was not being called in ice-impl or lightning-impl. This is outside fire-impl scope but affects the stamina-system contract."
      }
    ],
    "gaps": [
      {
        "id": "gap-fire-impl-1",
        "timestamp": "2026-03-12T18:50:00Z",
        "description": "Cooldown timer was not resetting after failed execution (insufficient stamina). Timer should only reset on successful execute().",
        "source": "skill-system/fire-impl/audit",
        "status": "fixed",
        "fixed_by": "skill-system/fire-impl/audit",
        "fixed_at": "2026-03-12T18:52:00Z"
      }
    ],
    "escalations": [],
    "status": "passed",
    "started_at": "2026-03-12T18:48:00Z",
    "completed_at": "2026-03-12T18:55:00Z",
    "result_summary": "All four scope criteria verified. One gap found (cooldown reset on failed execution) and fixed. Cross-cutting concern escalated to parent: validateCost() missing in sibling skill implementations."
  }
}
```

The parent node (`skill-system`) would have this in its escalations:

```json
{
  "audit": {
    "escalations": [
      {
        "id": "escalation-fire-impl-1",
        "timestamp": "2026-03-12T18:53:00Z",
        "description": "validateCost() not called in ice-impl or lightning-impl execute paths. Found during fire-impl audit. All skill implementations must call validateCost() before damage application per the stamina-system contract.",
        "source_node": "skill-system/fire-impl",
        "source_gap_id": null,
        "status": "open",
        "resolved_by": null,
        "resolved_at": null
      }
    ]
  }
}
```
