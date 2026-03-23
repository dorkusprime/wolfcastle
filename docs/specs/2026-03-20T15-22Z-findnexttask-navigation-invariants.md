# FindNextTask Navigation Invariants

Spec for property-based testing of `FindNextTask` (defined in `internal/state/navigation.go`). Each invariant below is a boolean predicate that must hold for every valid project tree, regardless of shape, depth, task count, or mutation history. A test author can translate each invariant directly into a `verify*` function following the pattern established in `propagation_property_test.go`.

## Definitions

**Reachable node**: A node whose index entry state is neither `StatusComplete` nor `StatusBlocked`, and whose ancestors in the DFS traversal are also reachable. `dfs()` skips complete/blocked nodes.

**Parent task**: A task whose ID is a proper prefix of at least one other task's ID (separated by `.`). For example, `task-0001` is a parent if `task-0001.0001` exists. Parent tasks have derived status computed by `DeriveParentStatus` and are not directly actionable.

**Audit task**: A task with `IsAudit == true`. Audit tasks are deferred until all non-audit work in the node is finished (complete or blocked, but not entirely blocked).

**Non-audit ancestor**: An ancestor task (by hierarchical ID prefix) that has `IsAudit == false` and `State == StatusNotStarted`. Audit ancestors are exempt because audit reset-to-not_started for re-verification must not block remediation children.

## Invariants

### INV-1: Returned task is actionable

If `result.Found == true`, then the task identified by `result.TaskID` in the node at `result.NodeAddress` must have state `StatusInProgress` or `StatusNotStarted`. It must never be `StatusComplete` or `StatusBlocked`.

```
assert(task.State == StatusInProgress || task.State == StatusNotStarted)
```

### INV-2: Non-audit tasks take priority over audit tasks (per-node)

If `result.Found == true` and the returned task has `IsAudit == true` and `State == StatusNotStarted`, then within the same node, `allNonAuditDone` must be true. Audit deferral is enforced per-node by `findActionableTask`, not tree-wide, because the DFS returns the first actionable task it finds and does not look ahead across nodes.

Two additional constraints: (1) `findActionableTask` also skips audit tasks when `allNonAuditBlocked` is true (all non-audit tasks blocked, no point running audit). (2) In-progress audit tasks bypass the deferral check entirely, because the self-healing loop returns any in_progress task without checking `IsAudit`.

```
if task.IsAudit && task.State == StatusNotStarted {
    assert(computeAllNonAuditDone(node) == true)
    assert(computeAllNonAuditBlocked(node) == false)
}
```

### INV-3: Parent tasks with children are never returned

If `result.Found == true`, then `TaskChildren(nodeState, result.TaskID)` must return `false`, unless the task is an audit task whose children are all complete (the re-verification exception). A parent task's status is derived; it is not directly claimable.

```
hasChildren := TaskChildren(ns, task.ID)
if hasChildren {
    assert(task.IsAudit && allChildrenComplete(ns, task.ID))
}
```

### INV-4: Child tasks with not_started non-audit ancestors are blocked

If `result.Found == true` and the returned task has `State == StatusNotStarted`, then `hasNotStartedAncestor(result.TaskID, nodeState)` must return `false`. A child task cannot run before its parent has been claimed. Audit ancestors are exempt per `hasNotStartedAncestor`'s semantics.

The ancestor check applies only to `StatusNotStarted` tasks. `StatusInProgress` tasks bypass it because the self-healing loop (lines 171-184) returns any in_progress non-parent task without checking ancestry; crashed work resumes regardless of ancestor state.

```
if task.State == StatusNotStarted {
    assert(!hasNotStartedAncestor(task.ID, ns))
}
```

### INV-5: All-complete yields `Found == false` with reason `all_complete`

If every node in `idx.Nodes` has `State == StatusComplete`, then `result.Found` must be `false` and `result.Reason` must be `"all_complete"`.

```
if allNodesComplete(idx) {
    assert(!result.Found)
    assert(result.Reason == "all_complete")
}
```

### INV-6: All-blocked yields `Found == false` with reason `all_blocked`

If no task in any reachable node is actionable and at least one node has `State == StatusBlocked`, then `result.Found` must be `false` and `result.Reason` must be `"all_blocked"`.

The current implementation sets `all_blocked` if any index entry has `StatusBlocked` and nothing was found. This is a simple heuristic, not a precise characterization: it checks nodes, not individual tasks. The property test should mirror this: after `FindNextTask` returns `Found == false`, scan `idx.Nodes` for any blocked entry to predict the reason.

```
if !result.Found && len(idx.Nodes) > 0 {
    hasBlocked := false
    for _, entry := range idx.Nodes {
        if entry.State == StatusBlocked {
            hasBlocked = true
            break
        }
    }
    if hasBlocked {
        assert(result.Reason == "all_blocked")
    } else {
        assert(result.Reason == "all_complete")
    }
}
```

### INV-7: Empty tree yields `Found == false` with reason `empty_tree`

If `len(idx.Nodes) == 0`, then `result.Found` must be `false` and `result.Reason` must be `"empty_tree"`.

```
if len(idx.Nodes) == 0 {
    assert(!result.Found)
    assert(result.Reason == "empty_tree")
}
```

## In-progress priority (supplemental)

When `findActionableTask` finds both `StatusInProgress` and `StatusNotStarted` tasks, it always returns the in-progress task first. This is the crash-recovery ("self-healing") behavior. A property test can verify:

```
if result.Found && task.State == StatusNotStarted {
    assert no in_progress non-parent task exists in the same node
}
```

## Scope of the property test

The random tree generator from `propagation_property_test.go` should be reused. After generating a tree and applying random mutations (claim, complete, block, unblock, add-child, add-task), call `FindNextTask` and assert all seven invariants plus the in-progress priority rule. The mutation set should also include adding audit tasks and hierarchical (parent/child) tasks to exercise INV-2 through INV-4.

## Source references

- `FindNextTask`: `internal/state/navigation.go:20-73`
- `findActionableTask`: `internal/state/navigation.go:130-240`
- `hasNotStartedAncestor`: `internal/state/navigation.go:276-293`
- `TaskChildren`: `internal/state/mutations.go:108-116`
- `DeriveParentStatus`: `internal/state/mutations.go:121-180`
- `allChildrenComplete`: `internal/state/navigation.go:297-314`
- Random tree generator: `internal/state/propagation_property_test.go:23-122`
