# State & Types

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

## State Mutations

All mutations go through functions in `internal/state/mutations.go`:

- `TaskClaim(ns, taskID)` — not_started → in_progress
- `TaskComplete(ns, taskID)` — in_progress → complete
- `TaskBlock(ns, taskID, reason)` — → blocked
- `TaskUnblock(ns, taskID)` — blocked → not_started
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

**Always propagate after mutations.** Use `app.PropagateState()` in commands or `d.propagateState()` in the daemon.

## Audit Lifecycle

`SyncAuditLifecycle(ns)` in `internal/state/audit_lifecycle.go` keeps the audit status consistent with node state:

- Node not_started → audit pending
- Node in_progress → audit in_progress
- Node complete + no open gaps → audit passed
- Node complete + open gaps → audit failed
- Node blocked → audit failed

## Navigation

`FindNextTask(idx, scope, loader)` in `internal/state/navigation.go` uses depth-first traversal to find the next actionable task (not_started or in_progress).

## Atomic I/O

All state writes use `atomicWriteJSON()` (write to temp file, then `os.Rename`). This prevents corruption from partial writes or crashes.
