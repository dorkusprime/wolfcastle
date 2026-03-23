---
status: IMPLEMENTED
---

# Orchestrator Planning Pipeline

## Problem Statement

Wolfcastle's current pipeline separates work into two stages: intake (decompose inbox items into a project tree) and execute (implement one task per iteration). Intake performs all planning in a single pass, creating the full tree of orchestrators, leaves, and tasks before any code is written. This front-loaded planning model has three systemic weaknesses:

1. **Planning quality degrades with scope.** Intake must decompose an entire spec into tasks in one invocation. For large specs, it runs out of context and produces coarse task descriptions ("Implement PromptRepository") that leave too much to the executor's interpretation. The domain repository refactor's Group E tasks were too coarse, leading to scope creep and a package-move incident.

2. **No feedback loop.** Once intake creates the tree, no planning agent re-examines it. When a task blocks, the tree freezes. When work reveals that the plan was wrong, there's no mechanism to re-plan. The domain refactor's task-0007 (delete Resolver) blocked because its prerequisites weren't met, and no agent could create the missing migration work.

3. **Documentation is an afterthought.** Specs, ADRs, and research happen (or don't) at the executor's discretion. The execute prompt says "ADRs are mandatory" but the executor can ignore it. The domain refactor produced 25 commits with zero ADRs until prompt enforcement was added, and even then compliance is probabilistic.

This spec redesigns the pipeline so that planning is recursive, happens at every level of the tree, and produces the artifacts (specs, ADRs, task definitions) that implementation depends on.

## Design

### Overview

Orchestrators become active planning agents. Instead of being inert containers whose state derives from their children, orchestrators receive planning invocations from the daemon. During a planning pass, an orchestrator studies its scope, examines the codebase, and creates its children using wolfcastle CLI commands, producing rich task definitions, deliverables, success criteria, and documentation tasks.

Planning runs in the main daemon loop between task executions. It is never concurrent with task execution. The daemon's loop becomes: complete a task, check if any orchestrator needs planning, run the planning pass if so, pick the next task.

The orchestrator does not write application code. It plans, reviews, and remediates. It has full codebase access (reads files, greps, explores) and calls wolfcastle CLI commands to create structure, but it never creates or modifies source files, test files, or other project artifacts. That is the executor's job. This separation means plan quality and code quality can be evaluated independently: if the output is wrong, the cause is traceable to either a bad plan (orchestrator) or a bad implementation (executor).

### Intake

Intake remains a parallel loop in the daemon, separate from the main execution loop. Its role narrows:

1. Read new inbox items.
2. Examine the existing project tree for overlap with the new item.
3. If no overlap: create one or more root orchestrators with a scope description. Multiple inbox items that belong together become a single orchestrator. A single inbox item with distinct goals may become multiple orchestrators.
4. If overlap with an existing orchestrator: append the new scope to that orchestrator's pending scope queue. The orchestrator absorbs it during its next planning pass.
5. File the inbox item.

Intake no longer creates leaves, tasks, or any structure below the root orchestrator. It does not write specs, define deliverables, or set success criteria. Its only structural output is root orchestrators.

Intake does not append to `PendingScope` while a planning pass is running. Pending scope items are buffered in the inbox goroutine and delivered to the orchestrator's state only between planning passes (when the main loop checks for re-planning triggers). This eliminates race conditions between the intake goroutine and the planning model's state mutations.

### Overlap Detection and Routing

When a new inbox item arrives, intake must determine whether it overlaps with an existing project. Overlap means the new work would modify the same packages, files, or architectural concerns as an in-progress project.

Intake performs a two-stage assessment:

**Stage 1: Heuristic pre-filter.** Compare the new item's text against each existing orchestrator's scope description using bigram similarity (the existing overlap advisory). If similarity is below a low threshold (e.g., 0.1), skip the orchestrator. This eliminates obviously unrelated projects cheaply.

**Stage 2: Model-based assessment.** For orchestrators that pass the pre-filter, include them in the intake prompt context. The intake model sees the new item alongside each candidate orchestrator's scope and children, and makes the routing decision: create new, merge into existing, or flag for human review.

The model-based assessment is necessary because text similarity misses semantic overlap. "Add caching to config loading" and "Implement ConfigRepository" have low bigram similarity but high architectural overlap.

If intake cannot determine overlap with confidence, it creates a new root orchestrator and logs an advisory. The human can merge orchestrators manually if needed.

### Orchestrator Lifecycle

An orchestrator progresses through these phases:

**Created.** Intake or a parent orchestrator creates the orchestrator with a scope description. The orchestrator has no children yet. It is marked as `needs_planning`.

**Planning.** The daemon detects `needs_planning` and invokes the orchestrator's planning model. The orchestrator has full codebase access. It reads files, explores the codebase, and calls wolfcastle CLI commands to build its subtree. In a single invocation, the orchestrator both decides the plan and executes it structurally.

The planning model receives (via prompt context):
- The orchestrator's scope description
- Any pending scope items (new work routed by intake or a parent)
- The parent orchestrator's success criteria for this node (if any)
- Current state of children (for re-planning passes)
- Planning history (prior passes for this orchestrator)
- Linked specs and ADRs

The orchestrator calls CLI commands during its invocation to produce:
- **Children.** `wolfcastle project create` for child orchestrators, leaf creation for concrete work.
- **Task definitions** for each leaf via `wolfcastle task add` with rich metadata (see Task Definition Structure and CLI Surface below).
- **Specs needed.** Spec-writing tasks ordered before implementation tasks.
- **Decisions needed.** ADR-writing tasks for architectural choices the orchestrator identified.
- **Research needed.** Discovery tasks for areas where the orchestrator lacks information to plan.
- **Success criteria** via `wolfcastle orchestrator criteria`.
- **Leaf audit enrichment** via `wolfcastle audit enrich`.

After the planning pass, the orchestrator's state transitions from `needs_planning`. Its children are created and ready for execution.

Planning passes use the same invocation timeout as task execution (configured via `daemon.invocation_timeout_seconds`). If a planning pass times out mid-creation (some children created, others not), the daemon treats it as a partial plan. Self-heal detects the incomplete state on next startup. The orchestrator is re-invoked with `plan-initial` trigger, sees its partially-created children, and completes the structure.

**Active.** The daemon works through the orchestrator's children. Leaves execute tasks sequentially. Child orchestrators receive their own planning invocations when reached by navigation.

**Re-planning.** The orchestrator is re-invoked when:
- New scope arrives (intake buffered a new item for this orchestrator).
- A child blocks or fails its audit.
- All children complete (triggers completion review, if the orchestrator has success criteria).

During re-planning, the orchestrator can:
- Create new children (leaves or child orchestrators).
- Modify tasks in its direct leaves, but only tasks that have not started. In-progress and complete tasks are immutable.
- Create new tasks in its direct leaves (appended after existing tasks).
- Delete children it just created during the current planning pass (rollback of its own partial work).
- It cannot modify children of child orchestrators. If a child orchestrator's subtree needs changes, the orchestrator marks the child orchestrator as `needs_planning` with amended scope, and the child orchestrator re-plans itself.

The orchestrator has a cumulative re-planning budget tracked by `TotalReplans`. Each planning pass that fails to produce a recognized terminal marker increments `TotalReplans`. If `TotalReplans` reaches `MaxReplans` (default 3, configurable per orchestrator or globally via `pipeline.planning.max_replans`), the orchestrator blocks itself and escalates. The budget is global across all trigger types, not per-trigger. The original `ReplanCount` (per-trigger map) is deprecated in favor of the simpler cumulative counter.

**Completion Review.** When all children are complete (or blocked/skipped), if the orchestrator has `SuccessCriteria` defined, it receives a final invocation with the `plan-review.md` prompt. It reviews:
- Whether its success criteria are met.
- Whether its children's outputs integrate correctly.
- Whether any audit gaps remain open.
- Whether the codebase is in the state the orchestrator intended.

If the review passes, the orchestrator completes. If it finds gaps, it creates new leaves to address them and transitions back to Active.

Orchestrators without `SuccessCriteria` auto-complete when all children are complete, preserving the current behavior for simple pass-through orchestrators. The `RecomputeState()` propagation logic continues to derive state from children for these orchestrators.

**Completion.** All success criteria met (or none defined and all children complete), review passed (or not required).

### Planning Prompt Variants

The orchestrator's invocation uses different prompt templates depending on the trigger. Each trigger is a different cognitive task requiring different instructions and a different phase structure:

**`plan-initial.md`** (trigger: `needs_planning`, no existing children).

Phase structure: Study → Decide → Structure → Record → Signal.
- **Study:** Read the scope description and any referenced specs. Explore the codebase to understand current state.
- **Decide:** Identify the concerns, dependencies, and ordering. Determine what needs research, what needs specs, what needs ADRs, and what can proceed directly to implementation.
- **Structure:** Call CLI commands to create children, tasks, success criteria, and audit enrichments.
- **Record:** Write a planning breadcrumb summarizing what was created and why.
- **Signal:** Emit `WOLFCASTLE_COMPLETE` when planning is done. Emit `WOLFCASTLE_BLOCKED` if the scope cannot be planned (missing information that can't be obtained from the codebase).

Guardrails:
- Maximum 10 direct children per orchestrator. If more are needed, create child orchestrators to group them.
- Maximum 8 tasks per leaf. If more are needed, split into multiple leaves.
- Spec and ADR tasks must precede implementation tasks within a leaf.
- Discovery tasks must precede spec tasks when the orchestrator lacks information.

**`plan-amend.md`** (trigger: `needs_planning` with pending scope, existing children).

Phase structure: Review → Assess → Amend → Record → Signal.
- **Review:** Read pending scope items and current children's state.
- **Assess:** Determine where the new work fits in the existing structure.
- **Amend:** Create new children or amend unstarted tasks. Do not disrupt in-progress work.
- **Record:** Write a planning breadcrumb summarizing the amendment.
- **Signal:** Emit `WOLFCASTLE_COMPLETE`.

**`plan-remediate.md`** (trigger: child blocked or audit failed).

Phase structure: Diagnose → Plan → Execute → Record → Signal.
- **Diagnose:** Read the block reason or audit findings. Understand why the child failed.
- **Plan:** Determine the remediation strategy (create prerequisites, amend plan, escalate, or skip).
- **Execute:** Call CLI commands to create remediation work, unblock children, or restructure.
- **Record:** Write a planning breadcrumb summarizing the remediation.
- **Signal:** Emit `WOLFCASTLE_COMPLETE` if remediated. Emit `WOLFCASTLE_BLOCKED` to escalate.

**`plan-review.md`** (trigger: all children complete, orchestrator has success criteria).

Phase structure: Assess → Verify → Decide → Record → Signal.
- **Assess:** Read success criteria and all children's final state, breadcrumbs, and audit results.
- **Verify:** Check the codebase: do the deliverables exist? Do tests pass? Do the pieces integrate?
- **Decide:** Complete if criteria met. Create new leaves if gaps found.
- **Record:** Write a planning breadcrumb summarizing the review outcome.
- **Signal:** Emit `WOLFCASTLE_COMPLETE` if criteria met and no new work created. Emit `WOLFCASTLE_CONTINUE` if new work was created and the orchestrator should transition back to Active.

Planning prompts use a subset of terminal markers with one addition:
- `WOLFCASTLE_COMPLETE`: Planning/review finished successfully.
- `WOLFCASTLE_BLOCKED`: Planning cannot proceed (escalate).
- `WOLFCASTLE_CONTINUE`: Review found gaps and created new work; transition back to Active. (Planning-only marker, not used in execution.)

Planning does not use `WOLFCASTLE_YIELD` or `WOLFCASTLE_SKIP`. These are execution-only markers with decomposition and already-done semantics that don't apply to planning. The daemon's marker handler branches by pass type: execution passes call `scanTerminalMarker` with an explicit marker list (`COMPLETE`, `SKIP`, `BLOCKED`, `YIELD`) that excludes `CONTINUE`; planning passes call `scanTerminalMarker` without a restricted set, defaulting to all five markers (which includes `CONTINUE`). This asymmetry is intentional: the execution side must never see `CONTINUE` (it would fall through to the failure path), while the planning side handles `CONTINUE` explicitly alongside `COMPLETE` and `BLOCKED`. If a planning model emits `YIELD` or `SKIP`, these are technically detected but fall through to the default "no recognized marker" handler, which increments the replan count.

The daemon selects the prompt variant based on the trigger type recorded in the orchestrator's state.

### Planning Context Assembly

Planning invocations use a dedicated context builder (`BuildPlanningContext()` in `internal/pipeline/planning_context.go`) separate from the execution context builder. The planning context currently includes:

1. **Orchestrator address and trigger type** (always included).
2. **Remediation attempt counter** (included when replans > 0): shows current attempt vs maximum.
3. **Scope** (always included, never truncated): the orchestrator's scope description.
4. **Pending scope items** (included when present): new work items to integrate, truncated to first 5.
5. **Success criteria** (included when defined): what the parent expects.
6. **Children state** (included for re-planning): each child's ID, address, and state. Currently rendered as ID + address + state; extended metadata (type, deliverables, constraints, acceptance criteria) is available on the `Task` struct but not yet surfaced in the planning context builder.
7. **Task state** (included when tasks exist): each task's ID, description, state, and block reason.
8. **Planning history** (included, last 3 passes): timestamp, trigger, and outcome summary.
9. **Open audit gaps** (included when gaps exist): gap ID and description for open gaps.
10. **Linked specs** (included by reference): paths listed, content not inlined.
11. **Codebase** (available via agent access, not inlined): the orchestrator explores the codebase during its invocation, same as the executor.

> **Note on extended task metadata:** The `state.Task` struct includes `Body`, `TaskType`, `Constraints`, `AcceptanceCriteria`, `References`, `Integration`, and `Deliverables` fields (added by the planning pipeline spec). These are set by the planning model during task creation and consumed by the executor. However, `BuildPlanningContext()` does not yet surface these extended fields in the children/task state sections of the planning context. The planning model only sees task ID, description, state, and block reason. Surfacing extended metadata in the planning context is a future enhancement that would improve re-planning quality.

Truncation priority when context exceeds budget:
1. Planning history beyond the last 3 passes (drop oldest first).
2. Children state detail (reduce to ID + state only, drop task summaries).
3. Pending scope items beyond the first 5 (drop oldest, log a warning).
4. Scope description is never truncated (it's the orchestrator's reason for existence).

### Orchestrator as Auditor

The orchestrator's completion review (when triggered) serves as the strategic audit for its subtree. It has full context: the scope it was given, the plan it made, the success criteria it defined, and the state of every child including their breadcrumbs, audit results, and code changes. It has codebase access and reads selectively rather than receiving full diffs in context; this keeps the review pass within context limits regardless of how much code the children produced.

Leaf-level audits still exist. They perform the detail check: code compiles, tests pass, files are in the right place, ADRs exist for technology choices. The orchestrator enriches each leaf's audit prompt with criteria specific to that leaf's role in the plan.

When a leaf audit fails or blocks, the orchestrator is responsible for remediation. It reads the audit's findings and decides: create a remediation leaf, amend the failed leaf's unstarted tasks, or accept the finding as non-blocking.

The separation:
- **Leaf audit:** "Did this leaf produce correct, complete code?"
- **Orchestrator review:** "Did this subtree achieve its goal? Do the pieces fit together?"

### Orchestrator as Unblocker

When a child blocks, the orchestrator receives a re-planning invocation with the `plan-remediate.md` prompt. It reads the block reason and the child's state, then decides:

1. **Create prerequisite work.** If the block is "can't delete X because Y still references it," the orchestrator creates a leaf to migrate Y, then unblocks the original task.
2. **Amend the plan.** If the block reveals the plan was wrong, the orchestrator restructures: replace the blocked child with a different approach, split it into smaller pieces, or remove it if the work isn't needed.
3. **Escalate.** If the orchestrator can't resolve the block (e.g., it requires human input, external dependency, or a decision outside its scope), it blocks itself with the reason. Its parent orchestrator (if any) then gets a re-planning invocation to handle the escalation. If there is no parent, the block surfaces to the human via `wolfcastle status` and daemon log.
4. **Skip.** If the blocked work is no longer necessary (other children achieved the goal by a different path), the orchestrator marks the child as skipped.

### Task Definition Structure

Orchestrators define tasks with significantly more detail than the current one-line descriptions. A task definition includes:

```
Title: short identifier
Description: what to build, concretely
  - Files to create or modify
  - Types, interfaces, or functions to implement
  - Behavior and error handling requirements
Integration: how this connects to other work
  - What existing code it touches
  - What other tasks depend on its output
  - What APIs or interfaces it must conform to
Deliverables: exact file paths this task will produce
Acceptance Criteria: verifiable conditions for "done"
  - Specific tests that must pass
  - Interfaces that must be implemented
  - Behavioral requirements
Constraints: what not to do
  - Files to leave alone
  - Patterns to avoid
  - Scope boundaries
Reference Material: what the executor should read
  - Spec paths
  - ADR paths
  - Existing code paths to study
  - Output from prior discovery tasks
```

The executor receives this full definition in its iteration context. It follows the definition rather than interpreting a vague title.

### CLI Surface for Planning

The orchestrator uses wolfcastle CLI commands during its planning invocation. Existing commands cover basic structure creation. New commands (or flags on existing commands) are needed for the rich metadata:

**Extended `wolfcastle task add`:**
```
wolfcastle task add --node <node> "title" \
  --body "rich description with implementation details" \
  --deliverable "path/to/file" \
  --type discovery|spec|adr|implementation|integration|cleanup \
  --constraint "do not modify X" \
  --acceptance "tests in X_test.go pass" \
  --reference "docs/specs/some-spec.md"
```

Flags `--body`, `--type`, `--constraint`, `--acceptance`, and `--reference` are new. Multiple `--deliverable`, `--constraint`, `--acceptance`, and `--reference` flags can be provided.

**New orchestrator commands:**
```
wolfcastle orchestrator criteria --node <node> "success criterion"
wolfcastle orchestrator criteria --node <node> --list
wolfcastle audit enrich --node <node> "additional audit criterion"
```

Multiple criteria/enrichments can be set by calling the command multiple times.

**Task modification (unstarted tasks only):**
```
wolfcastle task amend --node <node/task-id> --body "updated description"
wolfcastle task amend --node <node/task-id> --add-deliverable "path"
wolfcastle task amend --node <node/task-id> --add-constraint "constraint"
```

The `amend` command refuses to modify tasks that are in_progress or complete.

### Task Types

Orchestrators create tasks of different types, executed in the order they appear in the leaf:

**Discovery.** Read code, catalog patterns, inventory callers, assess scope. Output is breadcrumbs and/or a report file. Feeds into spec and implementation tasks.

**Spec.** Write a specification document via `wolfcastle spec create`. The orchestrator defines what the spec must cover: interfaces, error behavior, contracts, usage patterns, test strategy. The executor writes the spec content.

**ADR.** Document an architectural decision via `wolfcastle adr create`. The orchestrator identifies the decision, the context, and the alternatives to consider. The executor writes the ADR content with its analysis.

**Implementation.** Write code per the spec and task definition. The orchestrator defines deliverables, acceptance criteria, and constraints. The executor implements.

**Integration.** Wire new code into existing systems. Migrate callers, update imports, modify configuration. The orchestrator defines which callers to migrate and what the target API looks like.

**Cleanup.** Delete deprecated code, remove backward-compatibility shims, clean up temporary scaffolding. Always ordered last within a leaf. The orchestrator defines preconditions (what must be true before deletion is safe).

These types are stored on the Task struct and included in the executor's iteration context. The executor prompt can adjust its instructions per type (e.g., discovery tasks focus on reading and reporting, cleanup tasks verify preconditions before deleting). The daemon does not enforce different completion behavior per type.

### Task Classes Integration

When an orchestrator creates a task, it can assign a task class (e.g., `lang-go`, `discipline-audit`). Task classes (defined in the task-classes spec, `2026-03-15T00-04Z-task-classes.md`) inject behavioral prompts specific to the task's language, framework, or discipline. The orchestrator is the right place to assign classes because it understands what each task involves.

Class assignment is optional. If the orchestrator doesn't assign a class, the executor inherits the default class resolution (language detection, project-level defaults). Orchestrator assignment overrides automatic detection.

```
wolfcastle task add --node <node> "title" --class lang-go
```

### Hierarchical Task IDs

Tasks created by decomposition use hierarchical IDs reflecting their parent-child relationship:

```
task-0001           (original task)
task-0001.0001      (first subtask of task-0001)
task-0001.0002      (second subtask)
task-0001.0002.0001 (subtask of the second subtask)
```

Nesting depth is unbounded. Each level of decomposition costs an invocation, providing a natural practical limit.

Navigation is depth-first: all children of `task-0001` complete before `task-0002` starts. Within a level, tasks execute in order. This eliminates the current bug where siblings execute before children of decomposed parents. Crash recovery is preserved: `in_progress` tasks are always picked first regardless of position in the sorted order (see Navigation and Crash Recovery in the Testing Strategy section).

**Implementation: flat list with hierarchical ID strings.** Navigation with ancestor checks is O(n*d) where n is task count and d is nesting depth. For realistic sizes (< 100 tasks, < 4 levels), this is negligible. If large projects surface performance issues, an index of parent-child relationships can be built at state load time (O(n) build, O(1) lookup).

The state file keeps a flat `[]Task` slice. IDs are hierarchical strings. Lexicographic sort of hierarchical IDs produces depth-first order naturally (`task-0001`, `task-0001.0001`, `task-0001.0002`, `task-0002`). Parent-child relationships are determined by prefix matching. No nested struct or recursive serialization needed. This is backward compatible with existing state files (old `task-0001` IDs are valid hierarchical IDs with depth 1).

A parent task's status derives from its children:
- All children complete: parent is complete.
- Any child in progress: parent is in progress.
- Any child blocked (and none in progress): parent is blocked.
- All children not started: parent is not started.

There is no "decomposed" status. A task either has children (detected by prefix scan of the task list) and its status is derived, or it doesn't and its status is managed directly by the daemon. The `wolfcastle status` display distinguishes parent tasks from leaf tasks visually, but the underlying state model is the same.

Hierarchical task IDs replace the current leaf-to-orchestrator promotion mechanism. When an executor needs to decompose (>8 files), it creates child tasks under its own ID using `--parent` rather than spawning child nodes. The executor's task gains children in the flat list; the leaf's type doesn't change. This simplifies the decomposition model: tasks decompose into tasks, nodes don't change type.

The CLI's `wolfcastle task add` accepts a `--parent` flag to create a child with the next hierarchical ID under the specified parent:
```
wolfcastle task add --node <node> --parent task-0001 "subtask title"
# creates task-0001.0001
```

### Planning Model Selection

Orchestrator planning passes use a higher-capability model than task execution by default. Planning requires architectural judgment, scope assessment, and multi-concern reasoning. Implementation requires code generation within well-defined boundaries.

Default configuration:
- **Planning model (orchestrators):** `heavy` (Opus)
- **Execution model (tasks):** configurable per stage, defaults to `heavy`
- **Leaf audit model:** configurable, defaults to `heavy`
- **Intake model:** `mid` (Sonnet), same as today

These are configurable in the pipeline config. An orchestrator can override the planning model for its child orchestrators if it determines they need more or less capability.

### Daemon Loop Changes

The current daemon loop:
1. Navigate to the next task.
2. Invoke the executor.
3. Handle the terminal marker.
4. Repeat.

The new daemon loop:
1. Deliver any buffered pending scope from intake to orchestrator state files.
2. Navigate depth-first through the tree. The first actionable item is either:
   - An orchestrator with `needs_planning` set, or
   - A task that is `not_started` (with no `not_started` ancestors) or `in_progress` (crash recovery).
   Orchestrator planning is checked at the same priority as task readiness. The daemon processes whichever it encounters first in depth-first order. This means planning is just-in-time: the root orchestrator plans, its first child orchestrator plans, the first leaf's tasks execute, all before sibling orchestrators are planned.
3. If the actionable item is an orchestrator needing planning: run the planning pass.
4. If the actionable item is a task: invoke the executor and handle the terminal marker.
5. After the iteration (planning or execution), check for re-planning triggers:
   - Did a child just complete? Check if the parent orchestrator's children are all complete. If the orchestrator has `SuccessCriteria`, set `needs_planning` with `completion_review` trigger. Otherwise, auto-complete.
   - Did a child just block? Set parent orchestrator `needs_planning` with `plan-remediate` trigger.
   - Did intake buffer pending scope? Will be delivered at step 1 of the next iteration.
6. Repeat.

Planning passes are iterations in the daemon's log, just like task executions. They consume an iteration number and produce log output. The daemon's `max_iterations` limit applies to both planning and execution iterations. Planning passes appear in `wolfcastle status` as "Planning: [orchestrator name]" and in `wolfcastle daemon follow` as logged iterations with a `"stage": "plan"` tag.

### Error Recovery During Planning

If a planning invocation crashes or times out mid-pass (some children created, others not), the daemon treats it the same as a crashed task execution:

1. The orchestrator remains in `needs_planning` state.
2. On the next daemon iteration (or after restart via self-heal), the daemon re-invokes the orchestrator.
3. The orchestrator sees its partially-created children in the planning context and completes the structure.
4. Children created during the crashed pass that are `not_started` can be modified or deleted by the re-invocation.

No special rollback mechanism is needed. The orchestrator's re-invocation sees the current state and adjusts.

### NeedsPlanning State Transitions

`NeedsPlanning` is a flag on orchestrator `NodeState` that the daemon checks during navigation. The following table defines every transition:

| Event | Sets `NeedsPlanning` | Sets `PlanningTrigger` | Clears `NeedsPlanning` |
|-------|---------------------|----------------------|----------------------|
| Intake creates orchestrator | yes | `initial` |: |
| Parent orchestrator creates child orchestrator | yes | `initial` |: |
| Intake delivers buffered pending scope | yes | `new_scope` |: |
| Child blocks or audit fails | yes | `child_blocked` |: |
| All children complete (orchestrator has SuccessCriteria) | yes | `completion_review` |: |
| All children complete (orchestrator has no SuccessCriteria) |: |: |: (auto-completes instead) |
| Planning pass emits WOLFCASTLE_COMPLETE |: |: | yes |
| Planning pass emits WOLFCASTLE_BLOCKED |: |: | yes (orchestrator blocks itself) |
| Planning pass emits WOLFCASTLE_CONTINUE |: |: | yes (but new triggers may re-set it on next iteration) |
| Planning pass crashes/times out | remains set | unchanged |: (re-invoked on next iteration) |

`NeedsPlanning` and `NodeStatus` interact as follows:
- An orchestrator can have `NeedsPlanning=true` while its `State` is `in_progress` (normal: active orchestrator receiving new work or handling a blocked child).
- An orchestrator should never have `NeedsPlanning=true` while its `State` is `complete`. If this occurs, self-heal should log a warning and clear the flag.
- An orchestrator with `State=blocked` and `NeedsPlanning=true` means the orchestrator blocked itself but a new trigger (e.g., pending scope from intake) arrived. The daemon should re-invoke it, which may unblock it.

### Orchestrator State

The `NodeState` type for orchestrators gains new fields:

```go
type NodeState struct {
    // Existing fields
    Version            int
    ID                 string
    Name               string
    Type               NodeType       // "orchestrator" or "leaf"
    State              NodeStatus
    DecompositionDepth int
    Children           []ChildRef
    Tasks              []Task
    Audit              AuditState

    // New fields for active orchestrators
    Scope              string         // What this orchestrator is responsible for
    PendingScope       []string       // New scope items awaiting next planning pass
    SuccessCriteria    []string       // Conditions for completion (if set, triggers review)
    PlanningModel      string         // Model override for this orchestrator's planning
    NeedsPlanning      bool           // Daemon checks this to trigger planning
    PlanningTrigger    string         // "initial", "new_scope", "child_blocked", "completion_review"
    ReplanCount        map[string]int // deprecated: use TotalReplans
    TotalReplans       int            // cumulative replan count across all triggers
    MaxReplans         int            // Budget (default 3, configurable per orchestrator)
    PlanningHistory    []PlanningPass // Record of each planning invocation (last 5 kept)
    AuditEnrichment    []string       // Additional criteria injected into leaf audits
}

type PlanningPass struct {
    Timestamp   time.Time
    Trigger     string    // "initial", "new_scope", "child_blocked", "completion_review"
    Summary     string    // What the planning model decided (e.g., "marker=WOLFCASTLE_COMPLETE")
}
```

`PlanningHistory` is capped at 5 entries. Older entries are dropped when new ones are appended. The last 3 are included in planning context; the remaining 2 provide buffer for review without bloating state files.

The `Task` type gains new fields for rich definitions:

```go
type Task struct {
    // Existing fields
    ID                 string
    Title              string
    Description        string
    State              NodeStatus
    IsAudit            bool
    BlockedReason      string
    FailureCount       int
    NeedsDecomposition bool
    Deliverables       []string
    LastFailureType    string

    // New fields
    Body               string     // Rich description (markdown)
    TaskType           string     // "discovery", "spec", "adr", "implementation", "integration", "cleanup"
    Class              string     // Task class override (e.g., "lang-go")
    Constraints        []string   // What not to do
    AcceptanceCriteria []string   // Verifiable conditions for done
    References         []string   // Spec paths, ADR paths, code paths to study
    Integration        string     // How this connects to other work
}
```

All new fields use `omitempty` JSON tags, making the schema change backward compatible with existing state files. Old state files without the new fields deserialize cleanly; the fields default to zero values.

### Single Daemon Policy

One wolfcastle daemon runs globally at a time. This is an intentional constraint. The current system has no mechanism for coordinating multiple daemons operating on different worktrees of the same git repository, and the risk of conflicting commits, file mutations, and state corruption is too high. Multi-daemon support may be revisited once the orchestrator pipeline is stable and a coordination protocol exists.

Enforcement uses a global lock file at `~/.wolfcastle/daemon.lock` containing:

```json
{
    "pid": 12345,
    "repo": "/Users/wild/repository/dorkusprime/wolfcastle",
    "worktree": "/Users/wild/repository/dorkusprime/wolfcastle/refactor/domains",
    "started": "2026-03-17T06:11:00Z"
}
```

`wolfcastle start` checks this file before starting:
1. If the file doesn't exist, create it and proceed.
2. If the file exists, read the PID and check if the process is alive.
3. If the process is alive, refuse to start. Print: "Daemon already running in [worktree] (PID [pid], started [time])."
4. If the process is dead (stale lock), remove the file and proceed.

Non-daemon commands (`wolfcastle status`, `wolfcastle task add`, etc.) always operate on the CWD's `.wolfcastle/` directory. They do not read the lock file for routing. If CWD has no `.wolfcastle/`, they error with "no .wolfcastle directory found." They do not walk up the directory tree. This fixes the stray-init problem where commands silently operated on a `.wolfcastle/` in an ancestor directory.

The lock file lives in `~/.wolfcastle/` (the user's global wolfcastle config directory, separate from any project's `.wolfcastle/`). The `~/.wolfcastle/` directory is created on first use if it doesn't exist.

### Migration Path

This spec represents a significant structural change. The migration from the current system:

**Phase 1: Rich task definitions.** Add the new fields to `state.Task` (Body, TaskType, Constraints, AcceptanceCriteria, References, Integration). Extend `wolfcastle task add` with the new flags. Update the executor's iteration context to render the rich fields. No pipeline changes; intake and executors can start using richer tasks immediately. This improves task quality without any orchestrator changes.

**Phase 2: Orchestrator planning pass and intake narrowing.** Add the planning invocation for orchestrators. Add the four planning prompt variants. Intake narrows to creating root orchestrators. Orchestrators create their own children during planning using CLI commands. These must ship together: narrowing intake without orchestrator planning leaves no one to create the tree structure.

**Phase 3: Re-planning, remediation, and completion review.** Add re-planning triggers: new scope, child blocked, completion review. Orchestrators can now amend plans, create remediation work, and resolve blockers. This ships with or immediately after Phase 2.

**Phase 4: Hierarchical task IDs.** This phase is split:
- **4a: Hierarchical IDs (independent).** Add hierarchical ID support to `state.Task`, update navigation to depth-first via lexicographic sort, add `--parent` flag to `wolfcastle task add`. The old flat numbering continues to work (depth-1 hierarchical IDs). The decomposition block-reason hack and auto-complete logic remain functional for backward compatibility.
- **4b: Decomposition mechanism removal (depends on Phases 2-3).** Remove `autoCompleteDecomposedParents`, remove the "decomposed into subtasks" block reason convention, remove leaf-to-orchestrator promotion. Orchestrator planning and hierarchical task IDs fully replace these mechanisms.

**Phase 5: Single daemon enforcement.** Add global lock file. Update `wolfcastle start` to check the lock. Update all non-daemon commands to require `.wolfcastle/` in CWD (no directory walking).

Phase 1, Phase 4a, and Phase 5 are independent and can ship in any order. Phases 2 and 3 ship together. Phase 4b ships after Phases 2-3.

## Success Criteria

- Orchestrators produce task definitions with: concrete deliverables, acceptance criteria, constraints, reference material, and integration notes.
- Spec-writing and ADR-writing tasks are created by orchestrators before implementation tasks, not left to executor discretion.
- Discovery tasks are created when the orchestrator lacks information to plan.
- Blocked children trigger orchestrator re-planning, not tree freezes.
- Orchestrator completion review catches gaps that leaf audits miss (integration issues, missing migration work, scope not satisfied).
- The domain repository architecture spec, when re-run through this pipeline, produces: richer task definitions than the current run, specs before implementation, ADRs before implementation, and no blocked tasks from incomplete prerequisites.
- One daemon globally, enforced by lock file.
- No concurrent planning and execution.
- Orchestrators re-plan at most N times per trigger type (configurable, default 3) before escalating.
- Planning passes appear in status output and daemon logs with clear identification.

## Testing Strategy

Each phase introduces testable behavior that must be verified with both unit tests and live daemon evaluations.

### Unit Test Infrastructure

The existing test helpers (`testDaemon`, `setupLeafNode`, `writePromptFile`, `initTestGitRepo`) cover task execution. Planning requires new helpers:

- **`setupOrchestrator(t, d, nodeAddr, scope, children)`**: Creates an orchestrator node with scope, optional children, and optional success criteria.
- **`mockPlanningModel(t, commands []string)`**: Returns a model definition whose script calls the specified wolfcastle CLI commands in order, then emits `WOLFCASTLE_COMPLETE`. Used to test that the daemon correctly handles planning output (children created, tasks defined, criteria set).
- **`assertTreeStructure(t, d, expected)`**: Verifies the project tree matches an expected shape (node types, task counts, task states, hierarchical IDs).

### Live Evaluation Criteria

Each phase needs a concrete live daemon scenario that proves the behavior works end-to-end, run 3 consecutive times.

**Phase 1 (Rich task definitions):**
- Scenario: Intake creates tasks with `--body`, `--type`, `--constraint`, `--acceptance`, `--reference` flags. Executor receives the rich fields in its iteration context. Verify: the model's output respects the constraints and references.
- Eval: Create a project with a constrained task ("implement X but do not modify Y"). Run 3 times. Verify Y is unmodified in all 3 runs.

**Phase 2+3 (Orchestrator planning + re-planning):**
- Scenario: Inject the domain repository architecture spec. The orchestrator plans the full tree with rich task definitions, specs before implementation, ADRs before implementation. Compare output against the `refactor/domains` run.
- Eval: Run 3 times. Each run must produce: (a) spec-writing tasks before implementation tasks, (b) at least one ADR task, (c) task definitions with acceptance criteria, (d) 0 task failures through Group D.
- Remediation eval: Inject a task that will block (e.g., "delete X" where X has callers). Verify the orchestrator creates prerequisite migration work rather than leaving the tree frozen. Run 3 times.

**Phase 4a (Hierarchical task IDs):**
- Scenario: Executor decomposes a task mid-execution (>8 files). Verify: child tasks have hierarchical IDs, navigation processes children before siblings, parent auto-completes when children finish.
- Eval: Use the F6 mock model (creates subtasks and yields). Run 3 times. Verify parent task-0001's children (task-0001.0001, task-0001.0002) execute before task-0002.

**Phase 5 (Single daemon):**
- Scenario: Start a daemon, then attempt to start a second. Verify: second start fails with a clear message. Kill the first daemon, verify the second starts. Run `wolfcastle status` from a directory without `.wolfcastle/` and verify it errors (no walking up).

### Navigation and Crash Recovery

Depth-first navigation via lexicographic sort must preserve crash recovery semantics. The current system picks `in_progress` tasks before `not_started` tasks so that a crashed daemon resumes the interrupted task.

With hierarchical IDs, navigation sorts the task list lexicographically for ordering, but still prioritizes `in_progress` tasks within that order:

1. Scan for any `in_progress` task (crash recovery). If found, return it regardless of position. There should be at most one (self-heal enforces this).
2. If no `in_progress` task, scan the lexicographically sorted list for the first `not_started` task that has no `not_started` ancestors (i.e., its parent, if any, is `in_progress` or `complete`, not `not_started`). This prevents starting a child when the parent hasn't been claimed yet.

This preserves crash recovery (step 1) while achieving depth-first ordering (step 2).

### Code Coverage Gaps

Modifying these systems creates coverage gaps that must be backfilled:

| System | Existing Coverage | Gap Introduced |
|--------|------------------|----------------|
| `state.Task` | Field-level tests, mutation tests | New fields need rendering tests (iteration context includes Body, Constraints, etc.) |
| Navigation | Flat scan tests | Hierarchical ordering tests, depth-first verification, crash recovery with nested tasks |
| Daemon loop | RunOnce/RunIteration tests | Planning pass interleaving, re-planning triggers, buffered scope delivery |
| Prompt assembly | BuildIterationContextFull tests | BuildPlanningContext tests, truncation priority verification |
| CLI `task add` | Existing flag tests | New flags (--body, --type, --constraint, --acceptance, --reference, --parent, --class) |
| CLI new commands | None | `orchestrator criteria`, `audit enrich`, `task amend` need full test suites |
| Intake | Intake stage tests | Narrowed role tests, overlap detection (heuristic + model), scope buffering |
| Self-heal | Interrupted task recovery | Interrupted planning recovery, partial tree detection |

## Superseded and Amended Documents

When this spec is implemented, the following documents need updating:

### Specs

- **`2026-03-12T00-03Z-pipeline-stage-contract.md`** (Pipeline Stage Contract). Partially superseded. The stage lifecycle, prompt assembly, and iteration handling sections still apply to the execute stage. The intake stage section needs rewriting to reflect intake's narrowed role (root orchestrator creation only, no leaf/task creation). A new "plan" stage section must be added describing the orchestrator planning invocation as a stage type.

- **`2026-03-12T00-07Z-orchestrator-prompt.md`** (Orchestrator Prompt Structure). Substantially superseded. This spec describes prompt assembly for a passive orchestrator (prompt is assembled *about* the orchestrator for the executor). The new model has orchestrators receiving their own prompt and running as active agents. The prompt structure for orchestrators is now defined by the four planning prompt variants in this spec. The old spec should be marked superseded and reference this one.

- **`2026-03-12T00-00Z-state-machine.md`** (State Machine). Amended. The state transitions for orchestrators change: orchestrators gain a `needs_planning` flag and the planning lifecycle (created → planning → active → re-planning → completion review → complete). The automatic "all children complete = parent complete" rule is replaced by the orchestrator's completion review for orchestrators with success criteria; orchestrators without success criteria continue to auto-complete. Task states gain hierarchical ID semantics (parent status derived from children). The spec's section on node state propagation needs rewriting.

- **`2026-03-16T00-00Z-domain-repository-architecture.md`** (Domain Repository Architecture). Unaffected structurally, but the `state.Task` type changes (new fields: Body, TaskType, Class, Constraints, AcceptanceCriteria, References, Integration) need to be reflected. The `state.NodeState` type changes (new orchestrator fields) need to be reflected. The Store and navigation interfaces may need new methods.

### ADRs

- **ADR-014** (Serial Execution). Affirmed and strengthened. This spec adds planning passes to the serial execution model. Planning and execution are never concurrent. The ADR's principle holds; its scope expands.

- **ADR-019** (Failure/Decomposition/Retry). Partially superseded. Decomposition is no longer primarily a failure recovery mechanism triggered by repeated task failures. It's a first-class planning output created by orchestrators. The failure-driven decomposition path (fail N times → set NeedsDecomposition → decompose on next invocation) remains as a fallback for executors that hit the 8-file threshold, but the primary decomposition path is orchestrator planning. The ADR should be updated to distinguish planned decomposition (orchestrator) from reactive decomposition (executor under pressure).

- **ADR-064** (Intake Stage and Parallel Inbox). Amended. Intake remains parallel but its output changes from "full project tree" to "root orchestrators with scope descriptions." The ADR's architectural decision (separate goroutine for inbox processing) still holds. The scope of intake's work narrows. New constraint: intake buffers pending scope delivery between planning passes.

- **ADR-020** (Daemon Lifecycle). Amended. The daemon loop gains planning pass checks between task executions. The self-heal, shutdown, and restart logic is unaffected but extends to cover incomplete planning passes. The iteration model expands to include planning iterations alongside execution iterations.

### Backlog Items Resolved

These backlog items are addressed by this spec and should be moved to Done when implemented:

- "Hierarchical task IDs for decomposition": addressed by Hierarchical Task IDs section.
- "Decomposed tasks show as blocked": addressed by derived parent status (no decomposed status needed).
- "Navigation doesn't prioritize subtasks of a decomposed parent": superseded by hierarchical IDs.
- "Decomposition should scope by concern, not by directory": addressed by orchestrator planning (orchestrator inventories concerns during planning pass).
- "Deletion tasks should auto-decompose on block": addressed by orchestrator-as-unblocker (orchestrator re-plans when a child blocks).
- "Task descriptions need more detail": addressed by Task Definition Structure.
- "Intake decomposition should reference spec granularity": addressed by orchestrator planning (orchestrator reads the spec and creates tasks at the right granularity).

## Design Decisions

1. **Planning pass cost for simple projects.** Accept the overhead. A simple project's orchestrator planning pass will be fast (small scope, one leaf, a few tasks). The cost of consistency (every project flows through the same pipeline) outweighs the cost of one extra invocation. If profiling shows this is material for common workflows, add an optimization later (intake bypass for single-scope items). Do not add the optimization preemptively.

2. **Child orchestrator autonomy vs. parent control.** Parent scope amendments are authoritative. The child orchestrator incorporates them into its next planning pass. If the child cannot reconcile the amendment with its existing plan, it blocks itself and escalates back to the parent. The parent adjudicates. This matches organizational hierarchy: scope flows down, escalations flow up.

3. **State schema migration.** All new `NodeState` and `Task` fields use `omitempty` JSON tags, making deserialization backward compatible. Rich task definition fields (Phase 1), hierarchical IDs (Phase 4a), and single daemon enforcement (Phase 5) are always-on once shipped. Orchestrator planning invocations (Phases 2-3) are gated by a `pipeline.planning.enabled` config flag, defaulting to `false`. When false, orchestrators behave as inert containers (current behavior). When true, the daemon runs planning passes. This allows incremental rollout without affecting existing projects.
