# Consolidated Improvement Specification

This document combines findings from three independent evaluations (Claude self-review, Gemini review, and Codex review) into a single prioritized improvement plan for the Claude implementation of Wolfcastle.

Sources:
- `improvement-spec-claude-2.md` (Claude self-review)
- `improvement-spec-gemini-2.md` (Gemini review)
- `improvement-spec-codex-2.md` (Codex review)

---

## Strengths to Preserve

All three evaluations agree on these strengths — do not regress them:

- **`RecomputeState` propagation logic** (`internal/state/propagation.go`) — flawlessly handles the mixed blocked/not_started case. Best-in-class.
- **Composable `Check` interface** (`internal/validate/engine.go`) — best validation architecture across all three implementations.
- **Package discipline** — config, state, pipeline, and project packages are cleanly separated. Do not flatten while fixing runtime issues.
- **Unit test coverage** (`internal/state/propagation_test.go`, `internal/config/merge_test.go`, etc.) — keep and extend.
- **Deterministic fix staging** (`internal/validate/fix.go`) — strong foundation for self-healing.
- **Embedded templates and scaffold** (`internal/project/embedded.go`, `internal/project/scaffold.go`).

---

## Priority 1: Fix Daemon-Side Propagation (ADR-024 Compliance)

**Sources**: Codex review (P1), Claude self-review (related to P1)

### Problem

The CLI propagation path is correct (`cmd/helpers.go:10-86`), but the daemon bypasses it:

- Claim path: `internal/daemon/daemon.go:246-259`
- Post-model mutation/save path: `internal/daemon/daemon.go:328-390`

The daemon mutates the leaf, writes its `state.json`, partially updates the root index entry, but does **not** recompute and persist parent `state.json` files. This violates the distributed-state contract in ADR-024 and is the single biggest correctness issue.

### Required Changes

#### 1.1 Centralize daemon mutations onto the shared propagation path

Extract the shared logic from `cmd/helpers.go:10-86` into a reusable internal package. Replace the manual claim/update paths in the daemon so both CLI and daemon call the same propagation primitive.

**Reference**: Codex's runtime mutation flow (`runtime_mutation.go:148-193`, `state_tree.go:122-170`) always writes the leaf, propagates to ancestors, and rebuilds the root index.

#### 1.2 Propagate after every state-affecting marker

`d.applyModelMarkers(result.Stdout, ns, nav)` at `internal/daemon/daemon.go:328-329` mutates `ns` in memory but only writes the leaf afterward. If a marker changes task state, audit status, gaps/escalations, or node shape, the full save+propagate+root-index update path must run before returning from the iteration.

### Tests to Add

- Daemon claim transitions a parent from `not_started` to `in_progress`
- Daemon completion of last child transitions ancestors to `complete`
- Daemon block transitions an ancestor to `blocked` only when all non-complete siblings are blocked
- Root index entries for both leaf and ancestors are updated in the same iteration

---

## Priority 2: Fix Failure Threshold Handling (`==` → `>=`) and Decomposition

**Sources**: Claude self-review (P1: 1.1, 1.2), Codex review (P4)

### 2.1 Fix threshold comparison

In `internal/daemon/daemon.go` line 367, change `==` to `>=`:

```go
// Before
if failCount == threshold && depth < max {
// After
if failCount >= threshold && depth < max {
```

**Reference**: Codex uses `>=` in `runtime_mutation.go:153-178`. Gemini uses `>=` in `internal/daemon/daemon.go:500-503`.

### 2.2 Implement decomposition action at failure threshold

When `failure_count >= threshold AND depth < max_depth`, the daemon logs but takes no action. The model should be informed via the next iteration's prompt that decomposition is recommended.

**Implementation**:

1. In `internal/state/types.go`, add `NeedsDecomposition bool` to the `Node` struct.
2. In `internal/daemon/daemon.go:367-375`, set `node.NeedsDecomposition = true` when condition is met.
3. In `internal/pipeline/context.go` `BuildIterationContext`, check for `NeedsDecomposition` and append decomposition guidance text.
4. Clear `NeedsDecomposition` when the model successfully decomposes (node type changes from leaf to orchestrator).

### 2.3 Add failure context to prompt assembly

In `internal/pipeline/context.go` `BuildIterationContext` (lines 11-58), add failure history and decomposition policy context when `task.FailureCount > 0`:

```go
if task.FailureCount > 0 {
    fmt.Fprintf(&buf, "\n## Failure History\n\n")
    fmt.Fprintf(&buf, "This task has failed %d times.\n", task.FailureCount)
    fmt.Fprintf(&buf, "- Decomposition threshold: %d\n", cfg.Failure.DecompositionThreshold)
    fmt.Fprintf(&buf, "- Max decomposition depth: %d (current: %d)\n", cfg.Failure.MaxDecompositionDepth, node.DecompositionDepth)
    fmt.Fprintf(&buf, "- Hard failure cap: %d\n", cfg.Failure.HardCap)
    if task.FailureCount >= cfg.Failure.DecompositionThreshold && node.DecompositionDepth < cfg.Failure.MaxDecompositionDepth {
        fmt.Fprintf(&buf, "\n**Decomposition recommended.** Consider breaking this leaf into smaller sub-tasks.\n")
    }
}
```

**Reference**: Codex's `runtime_stage.go:79-133` includes failure policy in prompts. Gemini's `internal/pipeline/pipeline.go:185-206` does the same.

### Tests to Add

- Below threshold: continues iteration normally
- At threshold, below max depth: prompts decomposition
- Above threshold, below max depth: still prompts decomposition
- At threshold, at max depth: auto-blocks
- At hard cap: auto-blocks
- Above hard cap: auto-blocks

---

## Priority 3: Fix Non-Deterministic Navigation

**Source**: Claude self-review (P1: 1.3)

### Problem

In `internal/state/navigation.go` lines 34-39, when `idx.Root` is empty, the code falls back to iterating a Go map (`idx.Nodes`). Map iteration order is non-deterministic.

### Fix

Replace the map iteration fallback with a sorted slice:

```go
var topAddrs []string
for addr, entry := range idx.Nodes {
    if entry.Parent == "" {
        topAddrs = append(topAddrs, addr)
    }
}
sort.Strings(topAddrs)

for _, addr := range topAddrs {
    // ... existing DFS logic
}
```

Also ensure the `dfs` function (lines 57-112) processes orchestrator children in deterministic order.

**Reference**: Codex sorts children by slug in `state_tree.go:351`.

### Tests to Add

```go
func TestFindNextTask_DeterministicOrder(t *testing.T) {
    // Create a root index with multiple top-level nodes
    // Run FindNextTask 100 times
    // Verify the same node is returned every time
}
```

---

## Priority 4: Implement Audit Lifecycle State Machine

**Source**: Claude self-review (P2: 2.1, 2.2)

### Problem

Audit data (breadcrumbs, gaps, escalations) is stored but the lifecycle state machine is not implemented. The audit `Status` field is never automatically synchronized with task state, and there's no mechanism to block audit task completion when open gaps exist.

### 4.1 Implement `SyncAuditLifecycle`

Create `internal/state/audit_lifecycle.go`:

| Task State | Audit Status | Additional Action |
|-----------|-------------|-------------------|
| `not_started` | `"pending"` | — |
| `in_progress` | `"in_progress"` | Record `StartedAt` |
| `complete` + open gaps | `"failed"` | Block the audit task |
| `complete` + no open gaps | `"passed"` | Record `CompletedAt` |
| `blocked` | `"failed"` | — |

Call `SyncAuditLifecycle(node)` at the end of `TaskClaim`, `TaskComplete`, `TaskBlock`, `TaskUnblock`, and after gap creation/fixing in `applyModelMarkers`.

**Reference**: Codex's `state_tree.go:213-263`.

### 4.2 Verify audit status fields on types

Ensure `AuditState` in `internal/state/types.go` includes `Status`, `StartedAt`, `CompletedAt`, and `ResultSummary` fields.

### Tests to Add

- `TestSyncAuditLifecycle_NotStartedSetsPending`
- `TestSyncAuditLifecycle_InProgressSetsInProgress`
- `TestSyncAuditLifecycle_CompleteWithNoGapsSetsPassed`
- `TestSyncAuditLifecycle_CompleteWithOpenGapsBlocksTask`
- `TestSyncAuditLifecycle_BlockedSetsFailed`
- `TestSyncAuditLifecycle_FixingLastGapAllowsCompletion`

---

## Priority 5: Make Stale `in_progress` Detection PID-Aware

**Source**: Codex review (P2)

### Problem

The validator emits `STALE_IN_PROGRESS` whenever exactly one task is `in_progress` (`internal/validate/engine.go:253-259`). This fires during normal healthy daemon runs — it's a "there is one active task" detector, not a stale-task detector.

### Required Changes

#### 5.1 Redefine `STALE_IN_PROGRESS`

Only emit when:
1. Exactly one task is `in_progress`, **and**
2. There is no live daemon PID for this workspace.

#### 5.2 Add daemon liveness helpers to validation

Add a helper that checks daemon PID artifacts before classifying a task as stale. Can live in `internal/validate/engine.go` or a small daemon-liveness helper package.

**Reference**: Codex's `ensureNoLivePID` (`daemon.go:650-671`) and `recoverStaleDaemonState` (`daemon.go:684-727`).

### Tests to Add

- One `in_progress` task + live daemon PID → no `STALE_IN_PROGRESS`
- One `in_progress` task + dead/missing daemon PID → `STALE_IN_PROGRESS`
- Malformed PID file → warning plus stale-task classification

---

## Priority 6: Fix Daemon Shutdown and Signal Handling

**Source**: Codex review (P3)

### Problem

Signal handling in `Run` closes an internal shutdown channel (`internal/daemon/daemon.go:121-127`) but does not cancel the in-flight model invocation context. The daemon loop is structurally sound but shutdown behavior is weaker than the spec requires.

### Required Changes

#### 6.1 Root the daemon in a cancelable signal context

Replace "close shutdown channel and hope the loop notices" with a parent `context.Context` canceled by SIGINT/SIGTERM. Pass that context through to every model invocation path.

**Reference**: Codex uses `signal.NotifyContext` (`daemon.go:129-134`).

#### 6.2 Ensure stop semantics terminate the full invocation subtree

If the detached daemon or model subprocess is a process-group leader, stopping should signal the process group, not just the immediate process.

**Reference**: Codex's process-group handling (`daemon.go:137-170`, `daemon.go:422-462`). Gemini's model invocation process-group handling (`internal/pipeline/pipeline.go:245-301`).

### Tests to Add

- SIGTERM causes the active invocation context to cancel
- Detached stop kills the full process group
- Stop waits for clean exit when child honors cancellation

---

## Priority 7: Validation Engine — Audit-Specific Categories

**Source**: Claude self-review (P3: 3.1, 3.2)

### New Validation Categories

Add to `internal/validate/types.go`:

| Category | Description | Deterministic Fix |
|----------|-------------|-------------------|
| `MISSING_AUDIT_OBJECT` | Node has no audit object | Create empty audit with status "pending" |
| `INVALID_AUDIT_SCOPE` | Audit scope missing required fields | — |
| `INVALID_AUDIT_STATUS` | Status not in {pending, in_progress, passed, failed} | Set to "pending" |
| `INVALID_AUDIT_GAP` | Gap missing ID/description/status; stale metadata | Clear stale fixed metadata on open gaps |
| `INVALID_AUDIT_ESCALATION` | Escalation missing required fields | — |
| `AUDIT_STATUS_TASK_MISMATCH` | Audit status inconsistent with task state | Re-sync via `SyncAuditLifecycle` |

### Additional Runtime Validation (from Codex review P5)

- Daemon/runtime propagation drift detection
- Stale daemon artifacts vs live PID
- Root index state mismatches caused by daemon mutation paths

Add checks in the existing engine rather than growing a second validation mechanism.

---

## Priority 8: Complete the CLI Command Surface

**Source**: Gemini review

### Problem

Core domain logic and validation are robust, but some secondary CLI commands are incomplete or stubbed compared to the spec's 21 commands. Deep navigation, full archive generation, and inbox management are less fleshed out.

### Approach

Use Gemini's CLI structure as reference:
- `../wolfcastle-gemini/internal/cli/follow.go`
- `../wolfcastle-gemini/internal/cli/unblock.go`
- `../wolfcastle-gemini/internal/cli/audit.go`
- `../wolfcastle-gemini/internal/cli/navigate.go`
- `../wolfcastle-gemini/internal/cli/archive.go`
- `../wolfcastle-gemini/internal/cli/inbox.go`
- `../wolfcastle-gemini/internal/cli/project.go`
- `../wolfcastle-gemini/internal/cli/doctor.go`

The existing `internal/state` and `internal/validate` packages provide the backend — wiring these commands should be straightforward.

---

## Priority 9: Robustness and Polish

### 9.1 PID file atomicity

Verify daemon PID management follows check-then-create pattern to prevent race conditions between daemon instances.

**Reference**: Codex's `ensureNoLivePID` (`daemon.go:650-672`).

### 9.2 Branch verification in daemon loop

Verify the git branch hasn't changed between iterations. If the branch changes, the daemon should stop.

**Reference**: Gemini (`internal/daemon/daemon.go:331-341`), Codex (`daemon.go:228-232`, `verifyBranch` at `daemon.go:796-805`).

### 9.3 Worktree support

Add support for `wolfcastle start --worktree <branch>` if not already implemented: create worktree in temp dir, change daemon working directory, clean up on stop.

**Reference**: Gemini (`internal/git/git.go:19-44`, `internal/cli/start.go:64-96`), Codex (`daemon.go:729-747`).

### 9.4 `moveAuditLast` enforcement

Add a utility function to ensure the audit task is always last in the task list. Call at the end of any function that modifies the task list.

**Reference**: Codex's `moveAuditLast` (`doctor.go:613-624`).

### 9.5 Normalize audit state on load

Add a normalization step in `internal/state/io.go` to handle legacy or malformed state files, syncing between top-level and nested audit fields.

**Reference**: Codex's `normalizeAuditState` (`state_tree.go:46-62`).

### 9.6 JSON error envelope consistency

Verify all CLI commands use the `internal/output/envelope.go` `Ok`/`Err` functions for JSON output. Check for direct `fmt.Fprintf(os.Stderr, ...)` calls that should use the envelope.

### 9.7 Config validation: hard cap >= threshold

Verify the validation exists in `internal/config/validate.go` (test exists in `internal/config/config_test.go`).

### 9.8 Status command enhancements

Verify the status command provides audit counts and daemon status information comparable to Codex's `status.go:14-289`.

---

## Summary

| Priority | Change | Complexity | Sources |
|----------|--------|-----------|---------|
| P1 | Fix daemon-side propagation (ADR-024) | High | Codex |
| P2 | Fix `==` → `>=` threshold + decomposition action + prompt context | Medium | Claude, Codex |
| P3 | Fix non-deterministic navigation | Small | Claude |
| P4 | Implement audit lifecycle state machine | Medium | Claude |
| P5 | Make stale `in_progress` PID-aware | Medium | Codex |
| P6 | Fix daemon shutdown / signal handling | Medium | Codex |
| P7 | Add audit-specific validation categories | Medium | Claude, Codex |
| P8 | Complete CLI command surface | Medium | Gemini |
| P9 | Robustness and polish (9 items) | Small each | All |

**If you do only one thing, do Priority 1.** That is the difference between "architecturally impressive" and "operationally correct."
