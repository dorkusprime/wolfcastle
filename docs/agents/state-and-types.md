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
  "nodes": {
    "child-1": { "name": "...", "type": "leaf", "state": "not_started", ... },
    "child-1/sub": { "name": "...", "type": "leaf", "state": "complete", "parent": "child-1", ... }
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
  "tasks": [ { "id": "task-1", "description": "...", "state": "not_started", ... } ],
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

| Domain | Constants |
|--------|-----------|
| Node/Task status | `StatusNotStarted`, `StatusInProgress`, `StatusComplete`, `StatusBlocked` |
| Audit lifecycle | `AuditPending`, `AuditInProgress`, `AuditPassed`, `AuditFailed` |
| Node type | `NodeOrchestrator`, `NodeLeaf` |
| Gap status | `GapOpen`, `GapFixed` |
| Escalation status | `EscalationOpen`, `EscalationResolved` |

## Hierarchical Task IDs

Task IDs use a dot-separated hierarchy. `task-0001.0001` is a child of `task-0001`. Parent ID is derived by stripping the last `.NNNN` segment (see `parentTaskID()` in `navigation.go`). `TaskAddChild(ns, parentID, description)` creates a child task with the next available sequence number under the parent.

Navigation skips parent tasks that have children (their status is derived, not directly actionable). Child tasks whose parent is not_started are also skipped; the parent must be claimed first.

## DeriveParentStatus

`DeriveParentStatus(ns, taskID)` in `internal/state/mutations.go` computes a parent task's status from its immediate children (one level deep). Returns `(derivedStatus, true)` if children exist, or `(task.State, false)` if the task has no children.

The derivation rules: all children complete → complete; any child in_progress → in_progress; any child blocked (none in_progress) → blocked; otherwise → not_started.

Used by `selfHeal`, `PreStartSelfHeal`, `TaskComplete` (auto-completes parent when siblings finish), and navigation (skips parent tasks, uses derived status for audit deferral).

## State Mutations

All mutations go through functions in `internal/state/mutations.go`:

- `TaskClaim(ns, taskID)`: not_started → in_progress
- `TaskComplete(ns, taskID)`: in_progress → complete
- `TaskBlock(ns, taskID, reason)`: → blocked
- `TaskUnblock(ns, taskID)`: blocked → not_started
- `TaskAddChild(ns, parentID, description)`: creates a child task under a parent
- `DeriveParentStatus(ns, taskID)`: computes parent status from immediate children
- `AddBreadcrumb(ns, task, text)`
- `AddEscalation(ns, id, desc, source, gapID)`
- `IncrementFailure(ns, taskID)`
- `SetNeedsDecomposition(ns, taskID, needs)`

## State Propagation

When a node's state changes, it must propagate up through all ancestors to the root index. This is handled by `state.Propagate()`:

1. Walk from the changed node to the root
2. At each parent, recompute state from children
3. Update the root index entry
4. Save both the parent state file and root index

**Propagation is automatic.** `StateStore.MutateNode` propagates state changes to all ancestors within the same lock. No manual propagation calls are needed.

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
