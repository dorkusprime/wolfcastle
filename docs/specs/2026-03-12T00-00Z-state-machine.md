# State Machine for Nodes and Tasks

This spec defines the state machine governing the Wolfcastle project tree. It covers state definitions, valid transitions, propagation rules, failure tracking, decomposition, distributed state file structure, and cross-file consistency validation.

**Governing ADRs**: ADR-002 (JSON state), ADR-003 (deterministic scripts), ADR-007 (audit invariants), ADR-008 (tree addressing), ADR-009 (project layout), ADR-014 (serial execution), ADR-019 (failure/decomposition), ADR-020 (self-healing), ADR-021 (CLI commands), ADR-024 (distributed state files and task docs), ADR-025 (structural validation and doctor command).

---

## 1. Node Types

The project tree is composed of two node types:

| Node Type | Contains | Role |
|-----------|----------|------|
| **Orchestrator** | Child nodes (orchestrators or leaves) | Groups related work into sub-projects. State is derived from children. |
| **Leaf** | Tasks (ordered list) | Represents a unit of plannable work. Each leaf has an ordered task list ending with an audit task. |

An orchestrator's children may be a mix of orchestrators and leaves. A leaf contains only tasks -- it has no child nodes.

---

## 2. States

Every node (orchestrator or leaf) has exactly one of four states:

| State | Meaning |
|-------|---------|
| **Not Started** | No work has begun. All tasks (for a leaf) or all children (for an orchestrator) are untouched. This is the initial state for every newly created node. |
| **In Progress** | Work is actively happening or has been attempted. For a leaf, at least one task has been claimed. For an orchestrator, at least one child is In Progress or Complete. |
| **Complete** | All work is done and verified. For a leaf, all tasks including the audit task have completed. For an orchestrator, all children are Complete. |
| **Blocked** | Work cannot continue. The node has hit a failure threshold, an environmental issue, or a dependency that prevents progress. Requires explicit user intervention to unblock. |

There are no other states. "Failed", "Cancelled", "Paused", and similar are not valid. Work that cannot proceed is Blocked.

---

## 3. State Definitions by Node Type

### 3.1 Leaf Nodes

A leaf node's state reflects the aggregate progress of its task list.

| Leaf State | Condition |
|------------|-----------|
| Not Started | All tasks are Not Started. |
| In Progress | At least one task is In Progress or Complete, but not all tasks are Complete. Also applies when the node itself is actively being worked (a task has been claimed). |
| Complete | All tasks, including the audit task, are Complete. |
| Blocked | Explicitly set by `wolfcastle task block` or auto-blocked by failure thresholds. At least one task cannot proceed. |

### 3.2 Orchestrator Nodes

An orchestrator's state is **derived** from the states of its children. It is never set directly -- it is computed by propagation rules (Section 5).

| Orchestrator State | Condition |
|--------------------|-----------|
| Not Started | All children are Not Started. |
| In Progress | At least one child is In Progress or Complete, but not all children are Complete. Also: at least one child is Blocked but others remain workable. |
| Complete | All children are Complete. |
| Blocked | All non-Complete children are Blocked. No forward progress is possible without intervention. |

---

## 4. Valid State Transitions

### 4.1 Task-Level Transitions

Tasks are the atomic unit of work within a leaf. Each task has its own state.

```
                    +-----------+
                    | Not       |
               +--->| Started   |
               |    +-----+-----+
               |          |
               |          | claim
               |          v
               |    +-----------+
               |    | In        |
               |    | Progress  |
               |    +-----+-----+
               |          |
               |    +-----+-----+
               |    |           |
               |  complete    block
               |    |           |
               |    v           v
               | +------+ +-------+
               | |Compl.| |Blocked|
               | +------+ +---+---+
               |               |
               |             unblock
               +---------------+
                  (reset to
                  Not Started)
```

| From | To | Trigger | Command |
|------|----|---------|---------|
| Not Started | In Progress | Task is claimed for work | `wolfcastle task claim` |
| In Progress | Complete | Task passes validation | `wolfcastle task complete` |
| In Progress | Blocked | Manual block or auto-block from failure threshold / hard cap | `wolfcastle task block` (manual) or auto-block by daemon |
| Blocked | Not Started | User resolves external issue and unblocks (ADR-028: requires re-claim) | `wolfcastle task unblock` |

**Invalid transitions:**
- Not Started to Complete (must pass through In Progress)
- Not Started to Blocked (must be claimed first)
- Complete to any state (completion is terminal)
- In Progress to Not Started (no undo of claim)

### 4.2 Leaf Node Transitions

Leaf transitions are a consequence of task transitions within the leaf.

| From | To | Trigger |
|------|----|---------|
| Not Started | In Progress | First task in the leaf is claimed |
| In Progress | Complete | All tasks (including audit) complete |
| In Progress | Blocked | Active task is blocked (manual or auto) |
| Blocked | Not Started | Blocked task is unblocked (resets to Not Started per ADR-028) |

### 4.3 Orchestrator Node Transitions

Orchestrator transitions are computed, never directly commanded.

| From | To | Trigger |
|------|----|---------|
| Not Started | In Progress | First child transitions to In Progress |
| In Progress | Complete | Last non-Complete child transitions to Complete |
| In Progress | Blocked | All non-Complete children become Blocked |
| Blocked | In Progress | Any Blocked child is unblocked |

---

## 5. State Propagation

State propagates **upward** from children to parents. It never propagates downward. The rules are deterministic and enforced by scripts, not by the model.

### 5.1 Propagation Algorithm

When any node's state changes, its parent's `state.json` is recomputed, and the change is reflected in the root index:

```
function recompute_parent(parent):
    child_states = [child.state for child in parent.children]

    if all(s == "not_started" for s in child_states):
        parent.state = "not_started"
    elif all(s == "complete" for s in child_states):
        parent.state = "complete"
    elif all(s in ("complete", "blocked") for s in child_states)
         and any(s == "blocked" for s in child_states):
        parent.state = "blocked"
    else:
        parent.state = "in_progress"

    write parent's state.json
    update root index entry for parent

    if parent.parent exists:
        recompute_parent(parent.parent)
```

### 5.2 Propagation Across Distributed State Files

Per ADR-024, state is distributed across per-node `state.json` files. When a child's state changes, propagation must update multiple files:

1. **Child's `state.json`** -- the originating state change is written.
2. **Parent's `state.json`** -- the parent's `children` array is updated with the child's new state, and the parent's own state is recomputed.
3. **Root index `state.json`** -- the node registry entry for both the child and the parent (and any ancestors whose state changed) is updated.

All three writes happen within the same script invocation. If the process is interrupted mid-propagation, the root index may be stale -- this is a condition that `wolfcastle doctor` detects and repairs (see Section 14).

### 5.3 Implementation Pattern

The `propagateState` helper (in `cmd/helpers.go`) implements a two-pass approach used by all state-mutating commands (`task claim`, `task complete`, `task block`, `task unblock`):

1. **Update the originating node** in the root index
2. **Walk up the tree** via `state.PropagateUp()`:
   - For each parent: load its `state.json`, update the child ref's state, recompute parent state via `RecomputeState()`, save
   - Collect all ancestor addresses that were touched
3. **Update root index entries** for all ancestors that changed state
4. **Save root index once** at the end (single atomic write)

This pattern ensures:
- Node state files are always consistent with their children
- The root index reflects the current state of every node
- A single root index write at the end minimizes corruption risk
- Every command uses the same propagation logic (no duplication)

### 5.4 Propagation Examples

| Children States | Computed Parent State | Reasoning |
|-----------------|-----------------------|-----------|
| All Not Started | Not Started | No work has begun anywhere. |
| One In Progress, rest Not Started | In Progress | Work has begun. |
| One Complete, rest Not Started | In Progress | Some work is done, more remains. |
| One Blocked, rest Not Started | In Progress | One path is stuck, but others are available. |
| All In Progress | In Progress | All actively being worked. |
| One Blocked, rest In Progress | In Progress | Progress is possible on non-blocked children. |
| One Blocked, one Complete, one Not Started | In Progress | Not Started child can still make progress. |
| All Complete | Complete | All work is done. |
| One Blocked, rest Complete | Blocked | No forward progress possible without unblocking. |
| All Blocked | Blocked | Nothing can proceed. |

### 5.4 Propagation Is Recursive

When a leaf completes, its parent orchestrator recomputes. If that causes the parent to become Complete, the grandparent recomputes, and so on up to the root. Each ancestor's `state.json` is updated in turn, and the root index is updated once at the end with all changed entries.

---

## 6. Failure Tracking and Decomposition

Per ADR-019, each node tracks failure information to govern escalation behavior.

### 6.1 Failure Counter

Every leaf node (and every task within it) tracks a `failure_count` integer. This counter increments each time the model fails to make the task's validation pass within an iteration.

- The counter is **per-task**, not per-node.
- The counter resets to 0 on `wolfcastle task unblock`.
- The counter does NOT reset when the model succeeds at a partial fix that still fails validation.

### 6.2 Decomposition Depth

Every node tracks a `decomposition_depth` integer:

| Depth | Meaning |
|-------|---------|
| 0 | Original task, created by the user or a filing stage. |
| 1 | Created by decomposing a depth-0 task that hit the failure threshold. |
| N | N levels of recursive decomposition. |

When a task decomposes, child tasks inherit `decomposition_depth + 1`. The parent depth is unchanged.

### 6.3 Escalation Thresholds

All thresholds are configurable (see ADR-019). Defaults:

| Threshold | Default | Behavior |
|-----------|---------|----------|
| `decomposition_threshold` | 10 | At this failure count, the model is prompted to consider decomposing the task. |
| `max_decomposition_depth` | 5 | At this depth, decomposition is no longer offered. Hitting the failure threshold auto-blocks. |
| `hard_cap` | 50 | At this failure count, the task is auto-blocked regardless of depth. |

### 6.4 Escalation Decision Table

| Failure Count | Decomposition Depth | Action |
|---------------|---------------------|--------|
| < threshold (default 10) | Any | Keep fixing. Model iterates normally. |
| = threshold | < max (default 5) | Model is prompted to decompose. It may choose to decompose or block. |
| = threshold | = max (5) | Auto-blocked. No decomposition option. |
| = hard cap (default 50) | Any | Auto-blocked. Safety net against unbounded iteration. |

---

## 7. Decomposition and Tree Structure

When a task decomposes, the leaf node that contains it transforms into an orchestrator with new child leaves.

### 7.1 Decomposition Mechanics

1. The model determines that a task should be decomposed (prompted at failure threshold).
2. The model calls `wolfcastle project create` to create child nodes under the current node.
3. The model calls `wolfcastle task add` to populate the new child leaves with subtasks.
4. The original task that triggered decomposition is marked as superseded -- its work is now represented by the child nodes.
5. The former leaf node becomes an orchestrator. Its state is now derived from its new children.
6. Each new child node gets its own directory with its own `state.json` (per ADR-024). The parent's `state.json` is rewritten to reflect the type change from leaf to orchestrator.

### 7.2 Structural Change

```
BEFORE decomposition:
    orchestrator/
        leaf-A/            (leaf, contains tasks including stuck task X)
            state.json     (tasks, audit, failure counts)
            leaf-A.md      (project description)
            task-1: Complete
            task-X: In Progress (failure_count = 10)
            audit:  Not Started

AFTER decomposition:
    orchestrator/
        leaf-A/            (now an orchestrator, state derived from children)
            state.json     (rewritten: children list, no tasks)
            leaf-A.md      (project description, preserved)
            subtree-X/     (new orchestrator or leaf, depth = original + 1)
                state.json
                subtree-X.md
                part-1/    (leaf, depth = original + 1)
                    state.json  (tasks, audit, failure_count = 0)
                    part-1.md
                part-2/    (leaf, depth = original + 1)
                    state.json  (tasks, audit, failure_count = 0)
                    part-2.md
```

### 7.3 Invariants During Decomposition

- The audit task of the decomposing node remains last and is not moved or deleted.
- Completed tasks within the node are preserved -- only the stuck task is replaced by subtasks.
- Child nodes inherit `decomposition_depth + 1`.
- Each new child's failure counter starts at 0.
- The decomposing node's type changes from leaf to orchestrator.
- Each new child gets its own directory and `state.json`.
- The root index is updated to reflect all new nodes and the type change.

---

## 8. Audit Task Invariant

Per ADR-007, every leaf node has an audit task that is always the last task in its ordered list.

### 8.1 Rules

1. **Always last**: The audit task is always the final task in a leaf's task list. No task may be added after it.
2. **Cannot be moved**: Reordering operations must preserve the audit task's terminal position.
3. **Cannot be deleted**: The audit task cannot be removed from a leaf.
4. **Created automatically**: When a leaf is created, the audit task is appended automatically by the script.
5. **Executes last**: The audit task is only claimed after all other tasks in the leaf are Complete.
6. **Verification scope**: The audit verifies the work done by preceding tasks in the leaf. For orchestrators, the audit verifies integration across children.

### 8.2 Script Enforcement

All task-mutating scripts (`wolfcastle task add`, reorder operations) enforce the audit-last invariant. Adding a task inserts it before the audit task, never after. Attempting to delete or move the audit task returns an error.

---

## 9. Self-Healing: In Progress Recovery

Per ADR-020, if Wolfcastle starts and finds a task in `In Progress` state, this indicates a previous crash or hard kill interrupted work.

### 9.1 Recovery Behavior

1. On startup, the daemon scans for any task in `In Progress` state (using the root index for fast lookup, then confirming against the node's `state.json`).
2. If found, the daemon navigates to that task (using `wolfcastle navigate`).
3. The model is invoked on that task with whatever working directory state exists (possibly uncommitted changes from the interrupted run).
4. The model decides what to do: continue the work, discard partial changes, or complete/block the task.

### 9.2 Why This Works

State is committed alongside code, not on every mutation (ADR-020). This means:
- If the task was interrupted before committing, the state file still shows `In Progress` and the working directory may have uncommitted code changes.
- The model sees the same context it would have seen mid-task and can pick up where it left off.
- No special recovery logic is needed -- the normal execution path handles it.

### 9.3 Constraint

At most one task can be `In Progress` at any time, because execution is serial (ADR-014). If the root index or any `state.json` shows multiple tasks as `In Progress`, this is a corruption condition. The daemon should halt with an error, and `wolfcastle doctor` can diagnose and repair it (ADR-025).

---

## 10. Distributed State File Structure

Per ADR-024, state is distributed as one `state.json` per node, co-located with the node's project definition and task documents. The engineer's root `state.json` serves as a centralized index for fast navigation.

### 10.1 File Layout

```
.wolfcastle/projects/wild-macbook/
  state.json                          # Root index (tree structure, node registry)
  attunement-tree/
    state.json                        # Orchestrator node state
    attunement-tree.md                # Project description (Markdown)
    fire-impl/
      state.json                      # Leaf node state (tasks, audit, failures)
      fire-impl.md                    # Project description (Markdown)
      task-3.md                       # Task working doc (optional, model-created)
    water-impl/
      state.json                      # Leaf node state
      water-impl.md                   # Project description
```

### 10.2 Root Index Structure

The root `state.json` is an index of the entire tree. It provides fast navigation and status overview without requiring a filesystem walk. It contains the tree structure (node IDs, types, states, addresses) but not the detailed task lists, audit trails, or failure counters that live in each node's own `state.json`.

```json
{
  "version": 1,
  "root_id": "project-root",
  "root_name": "My Project",
  "root_state": "in_progress",
  "nodes": {
    "attunement-tree": {
      "name": "Attunement Tree Implementation",
      "type": "orchestrator",
      "state": "in_progress",
      "address": "attunement-tree",
      "decomposition_depth": 0,
      "children": ["attunement-tree/fire-impl", "attunement-tree/water-impl"]
    },
    "attunement-tree/fire-impl": {
      "name": "Implement Fire Attunement",
      "type": "leaf",
      "state": "in_progress",
      "address": "attunement-tree/fire-impl",
      "decomposition_depth": 0,
      "parent": "attunement-tree"
    },
    "attunement-tree/water-impl": {
      "name": "Implement Water Attunement",
      "type": "leaf",
      "state": "not_started",
      "address": "attunement-tree/water-impl",
      "decomposition_depth": 0,
      "parent": "attunement-tree"
    }
  }
}
```

The root index contains:
- **version**: Schema version for forward compatibility.
- **root_id, root_name, root_state**: Top-level project identity and aggregate state.
- **nodes**: A flat registry keyed by tree address. Each entry has the node's name, type, current state, decomposition depth, parent address, and (for orchestrators) child addresses.

The root index does NOT contain: task lists, failure counters, audit breadcrumbs, escalations, or blocked reasons. Those live in each node's own `state.json`.

### 10.3 Orchestrator Node State File

An orchestrator's `state.json` contains its own state, its children summary, and its audit trail.

```json
{
  "version": 1,
  "id": "attunement-tree",
  "name": "Attunement Tree Implementation",
  "type": "orchestrator",
  "state": "in_progress",
  "decomposition_depth": 0,
  "children": [
    {
      "id": "fire-impl",
      "address": "attunement-tree/fire-impl",
      "state": "in_progress"
    },
    {
      "id": "water-impl",
      "address": "attunement-tree/water-impl",
      "state": "not_started"
    }
  ],
  "audit": {
    "scope": { "description": "", "files": [], "systems": [], "criteria": [] },
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

The orchestrator's `children` array contains each child's ID, tree address, and current state -- enough to recompute the orchestrator's own state without reading child files. Full child details are in each child's own `state.json`.

### 10.4 Leaf Node State File

A leaf's `state.json` contains the task list, failure counters, and audit trail.

```json
{
  "version": 1,
  "id": "fire-impl",
  "name": "Implement Fire Attunement",
  "type": "leaf",
  "state": "in_progress",
  "decomposition_depth": 0,
  "tasks": [
    {
      "id": "task-1",
      "description": "Create fire attunement data model",
      "state": "complete",
      "failure_count": 0
    },
    {
      "id": "task-2",
      "description": "Wire stamina cost for fire abilities",
      "state": "in_progress",
      "failure_count": 3
    },
    {
      "id": "audit",
      "description": "Verify fire attunement implementation",
      "state": "not_started",
      "failure_count": 0,
      "is_audit": true
    }
  ],
  "audit": {
    "scope": {
      "description": "Verify fire attunement implementation",
      "files": [],
      "systems": [],
      "criteria": []
    },
    "breadcrumbs": [
      {
        "timestamp": "2026-03-12T18:30:00Z",
        "task": "attunement-tree/fire-impl/task-1",
        "text": "Created FireAttunement struct with base stats and validation"
      }
    ],
    "gaps": [],
    "escalations": [],
    "status": "pending",
    "started_at": null,
    "completed_at": null,
    "result_summary": null
  }
}
```

Task descriptions: the brief `description` field in the leaf's `state.json` is set by `wolfcastle task add` and is always included in model context. Per ADR-024, an optional companion Markdown file (e.g., `task-2.md`) can provide rich working context -- findings, learnings, partial results. The Markdown file is model-created and model-updated during execution. Go code controls context injection: only the active task's companion Markdown is loaded to prevent runaway context growth.

### 10.5 What Lives Where

| Data | Location | Rationale |
|------|----------|-----------|
| Tree structure (all node IDs, types, states, addresses) | Root index `state.json` | Fast navigation, status overview, `wolfcastle status` |
| Node registry (parent/child relationships) | Root index `state.json` | Tree traversal without filesystem walks |
| Orchestrator's computed state, children summary | Orchestrator's `state.json` | Self-contained recomputation of derived state |
| Leaf's task list, failure counters | Leaf's `state.json` | All task execution data co-located with the node |
| Audit breadcrumbs, escalations | Each node's `state.json` | Audit data lives with the node it describes |
| Blocked reasons | Each node's `state.json` | Contextual to the blocked node |
| Brief task description | Leaf's `state.json` (`tasks[].description`) | Always available, script-managed |
| Rich task context | Optional companion Markdown (e.g., `task-3.md`) | Model-created working document, loaded only for active task |
| Project description | Sibling Markdown (e.g., `fire-impl.md`) | Human and model readable, committed to git |

### 10.6 Full JSON Schema: Root Index

```json
{
  "$schema": "https://json-schema.org/draft/2020-12/schema",
  "title": "Wolfcastle Root Index",
  "type": "object",
  "required": ["version", "root_id", "root_name", "root_state", "nodes"],
  "properties": {
    "version": {
      "type": "integer",
      "const": 1,
      "description": "Schema version for forward compatibility"
    },
    "root_id": {
      "type": "string",
      "description": "ID of the root project node"
    },
    "root_name": {
      "type": "string",
      "description": "Human-readable name of the root project"
    },
    "root_state": {
      "type": "string",
      "enum": ["not_started", "in_progress", "complete", "blocked"],
      "description": "Aggregate state of the entire project tree"
    },
    "nodes": {
      "type": "object",
      "additionalProperties": { "$ref": "#/$defs/index_entry" },
      "description": "Flat registry of all nodes, keyed by tree address"
    }
  },
  "$defs": {
    "index_entry": {
      "type": "object",
      "required": ["name", "type", "state", "address", "decomposition_depth"],
      "properties": {
        "name": {
          "type": "string",
          "description": "Human-readable name"
        },
        "type": {
          "type": "string",
          "enum": ["orchestrator", "leaf"]
        },
        "state": {
          "type": "string",
          "enum": ["not_started", "in_progress", "complete", "blocked"]
        },
        "address": {
          "type": "string",
          "description": "Tree address (slash-separated path from root)"
        },
        "decomposition_depth": {
          "type": "integer",
          "minimum": 0
        },
        "parent": {
          "type": "string",
          "description": "Tree address of parent node. Absent for top-level children of root."
        },
        "children": {
          "type": "array",
          "items": { "type": "string" },
          "description": "Tree addresses of child nodes. Present only for orchestrators."
        }
      }
    }
  }
}
```

### 10.7 Full JSON Schema: Node State File

```json
{
  "$schema": "https://json-schema.org/draft/2020-12/schema",
  "title": "Wolfcastle Node State File",
  "type": "object",
  "required": ["version", "id", "name", "type", "state", "decomposition_depth"],
  "properties": {
    "version": {
      "type": "integer",
      "const": 1,
      "description": "Schema version for forward compatibility"
    },
    "id": {
      "type": "string",
      "description": "URL-safe slug, unique among siblings. Used in tree addresses."
    },
    "name": {
      "type": "string",
      "description": "Human-readable name"
    },
    "type": {
      "type": "string",
      "enum": ["orchestrator", "leaf"]
    },
    "state": {
      "type": "string",
      "enum": ["not_started", "in_progress", "complete", "blocked"]
    },
    "decomposition_depth": {
      "type": "integer",
      "minimum": 0,
      "description": "How many levels of decomposition produced this node. 0 = original."
    },
    "blocked_reason": {
      "type": "string",
      "description": "Present only when state is blocked. Explains why."
    },
    "children": {
      "type": "array",
      "items": { "$ref": "#/$defs/child_ref" },
      "description": "Present only for orchestrator nodes. Summary of each child's identity and state."
    },
    "tasks": {
      "type": "array",
      "items": { "$ref": "#/$defs/task" },
      "description": "Present only for leaf nodes. Last item must have is_audit=true."
    },
    "audit": { "$ref": "#/$defs/audit" }
  },
  "if": { "properties": { "type": { "const": "orchestrator" } } },
  "then": { "required": ["children"] },
  "else": { "required": ["tasks"] },
  "$defs": {
    "child_ref": {
      "type": "object",
      "required": ["id", "address", "state"],
      "properties": {
        "id": {
          "type": "string",
          "description": "Child node's ID (directory name)"
        },
        "address": {
          "type": "string",
          "description": "Child node's full tree address"
        },
        "state": {
          "type": "string",
          "enum": ["not_started", "in_progress", "complete", "blocked"],
          "description": "Child's current state (mirrored from child's state.json)"
        }
      }
    },
    "task": {
      "type": "object",
      "required": ["id", "description", "state", "failure_count"],
      "properties": {
        "id": {
          "type": "string",
          "description": "Unique task identifier within the leaf"
        },
        "description": {
          "type": "string",
          "description": "Brief description of what this task accomplishes. Rich context goes in the optional companion Markdown file (ADR-024)."
        },
        "state": {
          "type": "string",
          "enum": ["not_started", "in_progress", "complete", "blocked"]
        },
        "failure_count": {
          "type": "integer",
          "minimum": 0,
          "description": "Number of failed validation attempts for this task"
        },
        "is_audit": {
          "type": "boolean",
          "default": false,
          "description": "True only for the audit task (must be last in array)"
        },
        "blocked_reason": {
          "type": "string",
          "description": "Present only when state is blocked"
        }
      }
    },
    "audit": {
      "type": "object",
      "required": ["scope", "breadcrumbs", "gaps", "escalations", "status"],
      "properties": {
        "scope": {
          "type": "object",
          "properties": {
            "description": { "type": "string", "description": "What this audit must verify" },
            "files": { "type": "array", "items": { "type": "string" }, "description": "File paths or globs touched by this node" },
            "systems": { "type": "array", "items": { "type": "string" }, "description": "Subsystems or integration points" },
            "criteria": { "type": "array", "items": { "type": "string" }, "description": "Specific verification conditions" }
          },
          "description": "Contract defining what the audit must check"
        },
        "breadcrumbs": {
          "type": "array",
          "items": {
            "type": "object",
            "required": ["timestamp", "task", "text"],
            "properties": {
              "timestamp": {
                "type": "string",
                "format": "date-time"
              },
              "task": {
                "type": "string",
                "description": "Full tree address of the task that wrote this breadcrumb"
              },
              "text": {
                "type": "string",
                "description": "What was done or observed"
              }
            }
          },
          "description": "Append-only record of work performed. Feeds archive."
        },
        "gaps": {
          "type": "array",
          "items": {
            "type": "object",
            "required": ["id", "timestamp", "description", "source", "status"],
            "properties": {
              "id": { "type": "string", "description": "Deterministic ID: gap-{node-slug}-{sequential-int}" },
              "timestamp": { "type": "string", "format": "date-time" },
              "description": { "type": "string", "description": "What is missing or broken" },
              "source": { "type": "string", "description": "Tree address of the task or audit that found the gap" },
              "status": { "type": "string", "enum": ["open", "fixed"] },
              "fixed_by": { "type": ["string", "null"], "description": "Tree address of the task that resolved this gap" },
              "fixed_at": { "type": ["string", "null"], "format": "date-time" }
            }
          },
          "description": "Issues found during audit. Status transitions from open to fixed."
        },
        "escalations": {
          "type": "array",
          "items": {
            "type": "object",
            "required": ["id", "timestamp", "description", "source_node", "status"],
            "properties": {
              "id": { "type": "string", "description": "Deterministic ID: escalation-{node-slug}-{sequential-int}" },
              "timestamp": { "type": "string", "format": "date-time" },
              "description": { "type": "string", "description": "What needs attention at the parent level" },
              "source_node": { "type": "string", "description": "Tree address of the child that escalated" },
              "source_gap_id": { "type": ["string", "null"], "description": "ID of the originating gap, if from a gap" },
              "status": { "type": "string", "enum": ["open", "resolved"] },
              "resolved_by": { "type": ["string", "null"], "description": "Tree address of the task that resolved this" },
              "resolved_at": { "type": ["string", "null"], "format": "date-time" }
            }
          },
          "description": "Gaps escalated upward from children"
        },
        "status": {
          "type": "string",
          "enum": ["pending", "in_progress", "passed", "failed"],
          "description": "Lifecycle of the audit itself"
        },
        "started_at": {
          "type": ["string", "null"],
          "format": "date-time",
          "description": "When the audit task began execution"
        },
        "completed_at": {
          "type": ["string", "null"],
          "format": "date-time",
          "description": "When the audit task finished"
        },
        "result_summary": {
          "type": ["string", "null"],
          "description": "Brief model-written summary of audit outcome"
        }
      }
    }
  }
}
```

---

## 11. State Mutation Rules

All state mutations pass through deterministic scripts (ADR-003). The model never edits `state.json` directly. Because state is distributed across files (ADR-024), each mutation may write to multiple `state.json` files.

### 11.1 Command-to-State Mapping

| Command | State Mutation | Files Written |
|---------|---------------|---------------|
| `wolfcastle task add --node <path> "desc"` | Inserts task before audit task with state `not_started`, `failure_count: 0` | Leaf's `state.json` only (adding a Not Started task doesn't change node state) |
| `wolfcastle task claim --node <path>` | Sets task state to `in_progress` | Leaf's `state.json`, parent's `state.json` (child state summary), root index |
| `wolfcastle task complete --node <path>` | Sets task state to `complete` | Leaf's `state.json`, parent's `state.json`, root index; if all tasks complete, propagation continues up the tree |
| `wolfcastle task block --node <path> "reason"` | Sets task state to `blocked`, records `blocked_reason` | Leaf's `state.json`, parent's `state.json`, root index |
| `wolfcastle task unblock --node <path>` | Sets task state to `not_started` (ADR-028), clears `blocked_reason`, resets `failure_count` to 0 | Leaf's `state.json`, parent's `state.json`, root index |
| `wolfcastle project create --node <parent> "name"` | Creates child directory with `state.json` (state `not_started`), updates parent's children list | New child's `state.json`, parent's `state.json`, root index |
| `wolfcastle audit breadcrumb --node <path> "text"` | Appends to `audit.breadcrumbs` | Node's `state.json` only (breadcrumbs don't affect state) |
| `wolfcastle audit escalate --node <path> "gap"` | Appends to parent's `audit.escalations` | Parent's `state.json` only (escalations are informational) |

### 11.2 Commit Strategy

State accumulates locally across the distributed `state.json` files during task execution. It is committed to git alongside the task's code changes when the task completes or when the daemon decides to checkpoint. State is NOT committed on every mutation -- this avoids polluting git history with intermediate state changes.

---

## 12. Navigation and Task Selection

The daemon uses the root index for fast lookup, then reads individual node `state.json` files for task-level detail (ADR-014, ADR-024).

### 12.1 Navigation Algorithm

```
function find_next_task(node_address):
    index = read root index
    node_entry = index.nodes[node_address]

    if node_entry.type == "leaf":
        leaf_state = read node's state.json
        for task in leaf_state.tasks:
            if task.state == "in_progress":
                return task              // Resume interrupted work (self-healing)
            if task.state == "not_started":
                return task              // Next unclaimed task
        return null                      // All complete or blocked

    if node_entry.type == "orchestrator":
        for child_address in node_entry.children:
            child_entry = index.nodes[child_address]
            if child_entry.state in ("not_started", "in_progress"):
                result = find_next_task(child_address)
                if result != null:
                    return result
        return null                      // All children complete or blocked
```

The root index enables the orchestrator-level traversal without reading every node's `state.json`. Only the target leaf's `state.json` is read for task-level detail.

### 12.2 Priority

1. Any task in `In Progress` state (self-healing recovery).
2. First `Not Started` task in the first `Not Started` or `In Progress` leaf, depth-first.
3. If no actionable tasks exist, the daemon emits a completion or blocked signal.

---

## 13. Summary of Invariants

These invariants must hold at all times. Scripts enforce them; violations indicate bugs in Wolfcastle.

1. **Single In Progress**: At most one task across the entire tree is `In Progress` at any time.
2. **Audit Last**: The last task in every leaf's task list has `is_audit: true`.
3. **Audit Immovable**: The audit task cannot be deleted, reordered, or moved.
4. **State Consistency**: An orchestrator's state always equals the result of `recompute_parent()` over its children.
5. **Depth Monotonicity**: A child's `decomposition_depth` is always >= its parent's `decomposition_depth`.
6. **Failure Counter Non-Negative**: `failure_count` is always >= 0.
7. **Blocked Requires Reason**: If `state == "blocked"`, `blocked_reason` must be present and non-empty.
8. **Complete Is Terminal**: A task or node in `complete` state never transitions to another state.
9. **Breadcrumbs Append-Only**: Entries in `audit.breadcrumbs` are never modified or removed.
10. **Valid State Values**: State is always one of: `not_started`, `in_progress`, `complete`, `blocked`.
11. **Index-Node Consistency**: Every node's state in the root index matches the state in that node's `state.json`. Every node directory on disk has a corresponding entry in the root index, and vice versa.
12. **Parent-Child State Mirror**: Each orchestrator's `children` array mirrors the current state of each child's own `state.json`.

---

## 14. Structural Validation

Per ADR-025, the validation engine checks consistency across distributed state files. This is core infrastructure, not just a doctor feature.

### 14.1 Checks Performed

| Check | Severity | Description |
|-------|----------|-------------|
| Index-node state match | Error | Every node's state in the root index matches the state in the node's own `state.json`. |
| Index completeness | Error | Every node directory on disk has a root index entry. No index entries point to missing directories. |
| Parent-child consistency | Error | Each orchestrator's `children` array in its `state.json` matches the actual child directories and their states. |
| Orchestrator state derivation | Error | Each orchestrator's state equals `recompute_parent()` over its children's states. |
| Single In Progress | Error | At most one task across the entire tree is `In Progress`. |
| Audit-last invariant | Error | Every leaf's last task has `is_audit: true`. |
| Blocked reason presence | Warning | Every blocked node/task has a non-empty `blocked_reason`. |
| Orphaned directories | Warning | Node directories that exist on disk but have no index entry. |
| Stale index entries | Warning | Index entries that point to non-existent directories. |
| Schema version | Error | All `state.json` files have a recognized `version` field. |

### 14.2 When Validation Runs

- **Daemon startup**: A subset of checks (single In Progress, index completeness, orchestrator derivation) runs automatically to catch obvious issues before work begins.
- **`wolfcastle doctor`**: The full suite runs, reports findings, and offers fixes.
- **After propagation**: Scripts that propagate state changes can run a lightweight consistency check on the affected path from leaf to root.

### 14.3 Repair

Deterministic fixes (missing audit task, stale index entry, orphaned files) are applied directly by Go code. Ambiguous fixes (conflicting states, unclear intent) are resolved by a configurable model with Go-code validation before applying. See ADR-025 for the full doctor flow.
