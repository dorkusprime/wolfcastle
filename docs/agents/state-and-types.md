# State & Types

The authoritative specification for the state machine is [docs/specs/2026-03-12T00-00Z-state-machine.md](../specs/2026-03-12T00-00Z-state-machine.md). This guide covers the practical details for developers working with the code.

## State Files

Wolfcastle persists all state as JSON files under `.wolfcastle/`:

### Root Index (`projects/{namespace}/state.json`)

```json
{
  "version": 1,
  "root_id": "...",
  "root_name": "...",
  "root_state": "not_started",
  "root": ["child-1", "child-2"],
  "archived_root": ["old-project"],
  "nodes": {
    "child-1": { "name": "...", "type": "leaf", "state": "not_started", "address": "child-1", "decomposition_depth": 0, ... },
    "child-1/sub": { "name": "...", "type": "leaf", "state": "complete", "address": "child-1/sub", "parent": "child-1", ... },
    "old-project": { "name": "...", "type": "leaf", "state": "complete", "archived": true, "archived_at": "2026-03-20T...", ... }
  }
}
```

### Node State (`projects/{namespace}/{node-path}/state.json`)

```json
{
  "version": 1,
  "id": "node-id",
  "name": "Node Name",
  "type": "leaf",
  "state": "in_progress",
  "decomposition_depth": 0,
  "tasks": [ { "id": "task-1", "title": "...", "description": "...", "state": "not_started", ... } ],
  "scope": "...",
  "pending_scope": [],
  "success_criteria": [],
  "needs_planning": false,
  "aars": {},
  "specs": [],
  "audit": {
    "status": "pending",
    "breadcrumbs": [],
    "gaps": [],
    "escalations": [],
    "scope": { "description": "...", "files": [], "systems": [], "criteria": [] },
    "result_summary": ""
  }
}
```

## Status Constants

Always use typed constants from `internal/state/types.go`:

| Domain | Constants | File |
|--------|-----------|------|
| Node/Task status | `StatusNotStarted`, `StatusInProgress`, `StatusComplete`, `StatusBlocked` | `types.go` |
| Audit lifecycle | `AuditPending`, `AuditInProgress`, `AuditPassed`, `AuditFailed` | `types.go` |
| Node type | `NodeOrchestrator`, `NodeLeaf` | `types.go` |
| Gap status | `GapOpen`, `GapFixed` | `types.go` |
| Escalation status | `EscalationOpen`, `EscalationResolved` | `types.go` |
| Inbox item status | `InboxNew`, `InboxFiled` | `inbox_types.go` |
| Review batch status | `BatchPending`, `BatchCompleted` | `review_types.go` |
| Finding status | `FindingPending`, `FindingApproved`, `FindingRejected` | `review_types.go` |

## Hierarchical Task IDs

Task IDs use a dot-separated hierarchy. `task-0001.0001` is a child of `task-0001`. Parent ID is derived by stripping the last `.NNNN` segment (see `parentTaskID()` in `navigation.go`). `TaskAddChild(ns, parentID, description)` creates a child task with the next available sequence number under the parent.

Navigation skips parent tasks that have children (their status is derived, not directly actionable). Child tasks whose parent is not_started are also skipped; the parent must be claimed first.

## DeriveParentStatus

`DeriveParentStatus(ns, taskID)` in `internal/state/mutations.go` computes a parent task's status from its immediate children (one level deep). Returns `(derivedStatus, true)` if children exist, or `(task.State, false)` if the task has no children.

The derivation rules: all children complete → complete; any child in_progress → in_progress; any child blocked (none in_progress) → blocked; otherwise → not_started.

Used by `selfHeal`, `PreStartSelfHeal`, `TaskComplete` (auto-completes parent when siblings finish), and navigation (skips parent tasks, uses derived status for audit deferral).

## State Mutations

All mutations go through functions in `internal/state/mutations.go`:

- `TaskAdd(ns, description) (*Task, error)`: appends a new task to the node
- `TaskAddChild(ns, parentID, description) (*Task, error)`: creates a child task under a parent
- `TaskChildren(ns, taskID) bool`: returns whether the task has children
- `TaskClaim(ns, taskID) error`: not_started → in_progress
- `TaskComplete(ns, taskID) error`: in_progress → complete (auto-completes parent when all siblings finish)
- `TaskBlock(ns, taskID, reason) error`: → blocked
- `TaskUnblock(ns, taskID) error`: blocked → not_started (resets failure counter)
- `DeriveParentStatus(ns, taskID) (NodeStatus, bool)`: computes parent status from immediate children
- `AddBreadcrumb(ns, taskAddr, text, clk)`: records a timestamped breadcrumb
- `AddEscalation(parent, sourceNode, description, sourceGapID, clk)`: creates an escalation on the parent node
- `AddAAR(ns, aar)`: stores an After Action Review keyed by task ID
- `IncrementFailure(ns, taskID) (int, error)`: bumps failure count, returns new count
- `SetNeedsDecomposition(ns, taskID, needs)`: flags a task for decomposition

## State Propagation

When a node's state changes, it must propagate up through all ancestors to the root index. This is handled by `state.Propagate()`:

1. Walk from the changed node to the root
2. At each parent, recompute state from children
3. Update the root index entry
4. Save both the parent state file and root index

**Propagation is automatic.** `Store.MutateNode` propagates state changes to all ancestors within the same lock. No manual propagation calls are needed.

## Audit Lifecycle

`SyncAuditLifecycle(ns)` in `internal/state/audit_lifecycle.go` keeps the audit status consistent with node state:

- Node not_started → audit pending
- Node in_progress → audit in_progress
- Node complete + no open gaps → audit passed
- Node complete + open gaps → audit failed
- Node blocked → audit failed

## In-Progress Tracking

In-progress tracking applies to all node types. Orchestrators have tasks too (audit tasks, at minimum), and those tasks follow the same in_progress rules as leaf tasks. The serial execution invariant (at most one in_progress task globally) applies across both orchestrators and leaves.

## Orchestrator Audit Deferral

Audit tasks on orchestrators are actionable only when all children are complete AND at least one child exists. If no children exist, the orchestrator hasn't been planned yet, so the audit has nothing to verify. Navigation enforces this in `findActionableTask`: it checks `len(ns.Children) > 0` and every child's state before allowing the orchestrator's audit task into the work queue.

## Navigation

`FindNextTask(idx, scope, loader)` in `internal/state/navigation.go` uses depth-first traversal to find the next actionable task (not_started or in_progress).

## Atomic I/O

All state writes use `atomicWriteJSON()` (write to temp file, then `os.Rename`). This prevents corruption from partial writes or crashes.
