# Improvement Specification for Wolfcastle-Claude (Round 2)

This document describes specific improvements to the Claude implementation of Wolfcastle, informed by comparative evaluation against the Gemini and Codex implementations. Each section identifies the problem, references the superior implementation, and provides exact file/line guidance for the fix.

---

## Priority 1: Critical Algorithm Fixes

### 1.1 Fix Failure Escalation Threshold Check (`==` → `>=`)

**Problem**: In `internal/daemon/daemon.go` line 367, the failure threshold check uses `==` (exact equality) instead of `>=`. This means the decomposition prompt fires exactly once when `failure_count` hits the threshold, then never again as the count climbs toward the hard cap. If the model doesn't decompose on that single iteration, the opportunity is permanently lost until hard cap.

**Fix**: Change the `==` to `>=` on line 367.

**Reference**: Codex uses `>=` for both threshold and hard cap checks in `runtime_mutation.go` lines 172-177. Gemini also uses `>=` in `internal/daemon/daemon.go` lines 500-503.

**Before** (`internal/daemon/daemon.go` ~line 367):
```go
if failCount == threshold && depth < max {
```

**After**:
```go
if failCount >= threshold && depth < max {
```

### 1.2 Implement Decomposition Action at Failure Threshold

**Problem**: When `failure_count >= threshold AND depth < max_depth`, the daemon logs a "decomposition_threshold" message but takes no action. The spec says to "prompt decomposition" — the model should be informed via the next iteration's prompt that decomposition is recommended.

**What to implement**: When the threshold is reached at allowable depth, instead of just logging, set a flag or field on the task/node (e.g., `needs_decomposition: true`) that the prompt assembly picks up. The next iteration's prompt should include guidance that the model should decompose this leaf into sub-tasks using `wolfcastle project create`.

**Reference**: Codex has a `describeFailurePolicy` function (not in the main mutation path, but used in prompt assembly in `runtime_stage.go` lines 79-133) that communicates the failure policy to the model. While Codex doesn't trigger automatic decomposition either, it at least provides the model with the necessary context in every prompt.

**Implementation approach**:

1. In `internal/state/types.go`, add a `NeedsDecomposition bool` field to the `Node` struct (or use the existing failure metadata).

2. In `internal/daemon/daemon.go` around line 367-375, when `failCount >= threshold && depth < maxDepth`:
   ```go
   // Instead of just logging, mark the node for decomposition
   node.NeedsDecomposition = true
   // Save the node state
   ```

3. In `internal/pipeline/context.go` `BuildIterationContext` (lines 11-58), check for `NeedsDecomposition` and append guidance text:
   ```
   ## Decomposition Recommended

   This task has failed {N} times (threshold: {threshold}). The current
   decomposition depth is {depth} (max: {maxDepth}). You should decompose
   this leaf into smaller sub-tasks using `wolfcastle project create --node {addr} "subtask-name"`.
   ```

4. If the model successfully decomposes (detected by checking if the node type changed from leaf to orchestrator after the iteration), clear `NeedsDecomposition`.

### 1.3 Fix Non-Deterministic Navigation Fallback

**Problem**: In `internal/state/navigation.go` lines 34-39, when `idx.Root` is empty, the code falls back to iterating `idx.Nodes` (a Go map). Go map iteration order is non-deterministic, meaning which root node is processed first varies between runs.

**Reference**: Codex solves this in `state_tree.go` line 351 by explicitly sorting children by slug before traversal:
```go
sort.Slice(node.Children, func(i, j int) bool {
    return node.Children[i].Slug < node.Children[j].Slug
})
```

**Fix**: In `internal/state/navigation.go`, replace the map iteration fallback (lines 34-39) with a sorted slice:

```go
// Collect top-level node addresses, sort for determinism
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

Also, in the `dfs` function (lines 57-112), when iterating over an orchestrator's children, ensure children are processed in a deterministic order. If `Children` is a slice stored in order, this is fine. If it could be unordered, sort by address/slug before traversal.

---

## Priority 2: Missing Audit Lifecycle State Machine

### 2.1 Implement `syncAuditLifecycle`

**Problem**: Claude stores audit data (breadcrumbs, gaps, escalations) but does not implement the audit lifecycle state machine. The audit `Status` field (pending/in_progress/passed/failed) is never automatically synchronized with task state, and there's no mechanism to block audit task completion when open gaps exist.

**Reference**: Codex implements this comprehensively in `state_tree.go` lines 213-263 (`syncAuditLifecycle`). This function:

1. When a task transitions to `not_started` → sets audit status to `"pending"`
2. When a task transitions to `in_progress` → sets audit status to `"in_progress"`, records `StartedAt`
3. When a task completes AND open gaps exist → sets audit status to `"failed"`, blocks the task
4. When a task completes AND no open gaps → sets audit status to `"passed"`, records `CompletedAt`
5. When a task is blocked → sets audit status to `"failed"`

**Implementation**:

Create a new function in `internal/state/mutations.go` (or a new file `internal/state/audit_lifecycle.go`):

```go
func SyncAuditLifecycle(node *Node) {
    if node.Type != "leaf" || node.Audit == nil {
        return
    }

    // Find the audit task
    var auditTask *Task
    for i := range node.Tasks {
        if node.Tasks[i].IsAudit {
            auditTask = &node.Tasks[i]
            break
        }
    }
    if auditTask == nil {
        return
    }

    now := time.Now().UTC().Format(time.RFC3339)

    switch auditTask.State {
    case "not_started":
        node.Audit.Status = "pending"
    case "in_progress":
        node.Audit.Status = "in_progress"
        if node.Audit.StartedAt == "" {
            node.Audit.StartedAt = now
        }
    case "complete":
        if hasOpenGaps(node.Audit) {
            node.Audit.Status = "failed"
            // Block the audit task — can't complete with open gaps
            auditTask.State = "blocked"
            auditTask.BlockedReason = "Open audit gaps remain"
        } else {
            node.Audit.Status = "passed"
            node.Audit.CompletedAt = now
        }
    case "blocked":
        node.Audit.Status = "failed"
    }
}

func hasOpenGaps(audit *AuditState) bool {
    for _, gap := range audit.Gaps {
        if gap.Status == "open" {
            return true
        }
    }
    return false
}
```

Then call `SyncAuditLifecycle(node)` at the end of every mutation function that changes task state: `TaskClaim` (line 60), `TaskComplete` (line 85), `TaskBlock` (line 111), `TaskUnblock` (line 129).

Also call it after gap creation/fixing in the daemon marker processing (`internal/daemon/daemon.go` `applyModelMarkers` lines 645-736) — when a gap is added or fixed, re-sync the audit lifecycle.

### 2.2 Add Audit Status Fields to Types

**Problem**: The `AuditState` type in `internal/state/types.go` may not have all the lifecycle fields.

**Reference**: Codex's `AuditState` in `types.go` lines 181-190 includes:
```go
type AuditState struct {
    Scope        AuditScopeState  `json:"scope"`
    Breadcrumbs  []Breadcrumb     `json:"breadcrumbs"`
    Gaps         []AuditGap       `json:"gaps"`
    Escalations  []AuditEscalation `json:"escalations"`
    Status       string           `json:"status"`        // pending, in_progress, passed, failed
    StartedAt    string           `json:"started_at"`
    CompletedAt  string           `json:"completed_at"`
    ResultSummary string          `json:"result_summary"`
}
```

Verify that Claude's `AuditState` type includes `Status`, `StartedAt`, `CompletedAt`, and `ResultSummary` fields. Add any that are missing.

---

## Priority 3: Validation Engine Enhancements

### 3.1 Add Audit-Specific Validation Categories

**Problem**: Claude's validation engine implements exactly the 17 spec categories, which is correct but minimal. Several audit-related structural issues are not caught.

**Reference**: Codex's `doctor.go` implements 25+ categories including these audit-specific checks that Claude should adopt:

| Category | Codex location | Description |
|----------|---------------|-------------|
| `MISSING_AUDIT_OBJECT` | `doctor.go` lines 92-98 | Node has no audit object at all |
| `INVALID_AUDIT_SCOPE` | `doctor.go` lines 99-107 | Audit scope missing required fields |
| `INVALID_AUDIT_STATUS` | `doctor.go` lines 108-114 | Audit status not in {pending, in_progress, passed, failed} |
| `INVALID_AUDIT_GAP` | `doctor.go` lines 115-158 | Gap missing ID/description/status; fixed gap missing fixed_at/fixed_by; open gap with stale fixed metadata |
| `INVALID_AUDIT_ESCALATION` | `doctor.go` lines 159-201 | Escalation missing ID/description/status/source_node; resolved escalation missing resolved_at/resolved_by |
| `AUDIT_STATUS_TASK_MISMATCH` | `doctor.go` lines 271-286 | Audit status "passed" but task not complete, or audit status "failed" but task complete with no gaps |

**Implementation**: Add these as new constants in `internal/validate/types.go` after line 22:

```go
CatMissingAuditObject    = "MISSING_AUDIT_OBJECT"
CatInvalidAuditScope     = "INVALID_AUDIT_SCOPE"
CatInvalidAuditStatus    = "INVALID_AUDIT_STATUS"
CatInvalidAuditGap       = "INVALID_AUDIT_GAP"
CatInvalidAuditEscalation = "INVALID_AUDIT_ESCALATION"
CatAuditStatusTaskMismatch = "AUDIT_STATUS_TASK_MISMATCH"
```

Then add check logic in `internal/validate/engine.go`. The cleanest approach is to add a new `checkAuditIntegrity` function (similar to the existing `checkLeafAudit` at lines 266-308) that validates all audit-specific invariants.

For each node:
1. If `node.Audit == nil`, emit `MISSING_AUDIT_OBJECT` (deterministic fix: create empty audit with status "pending")
2. If `node.Audit.Status` not in `{"pending", "in_progress", "passed", "failed", ""}`, emit `INVALID_AUDIT_STATUS` (deterministic fix: set to "pending")
3. For each gap in `node.Audit.Gaps`:
   - If `gap.ID == ""` or `gap.Description == ""` or `gap.Status == ""`, emit `INVALID_AUDIT_GAP`
   - If `gap.Status == "fixed"` and (`gap.FixedAt == ""` or `gap.FixedBy == ""`), emit `INVALID_AUDIT_GAP`
   - If `gap.Status == "open"` and (`gap.FixedAt != ""` or `gap.FixedBy != ""`), emit `INVALID_AUDIT_GAP` (deterministic fix: clear stale fixed metadata)
4. For each escalation in `node.Audit.Escalations`:
   - If `escalation.ID == ""` or `escalation.Description == ""` or `escalation.SourceNode == ""`, emit `INVALID_AUDIT_ESCALATION`
   - If `escalation.Status == "resolved"` and (`escalation.ResolvedAt == ""` or `escalation.ResolvedBy == ""`), emit `INVALID_AUDIT_ESCALATION`
5. If leaf: find audit task. If audit task complete but `node.Audit.Status != "passed"` and no open gaps, emit `AUDIT_STATUS_TASK_MISMATCH` (deterministic fix: set to "passed")

Add these new categories to `StartupCategories` in `internal/validate/types.go` (after line 92) where appropriate — `MISSING_AUDIT_OBJECT` and `INVALID_AUDIT_STATUS` should be startup checks.

### 3.2 Add Deterministic Fixes for New Categories

In `internal/validate/fix.go` `ApplyDeterministicFixes` (lines 22-258), add cases to the switch statement for each new category:

```go
case CatMissingAuditObject:
    node.Audit = &AuditState{Status: "pending"}
    modified[issue.NodePath] = true

case CatInvalidAuditStatus:
    node.Audit.Status = "pending"
    modified[issue.NodePath] = true

case CatInvalidAuditGap:
    // Clear stale fixed metadata on open gaps
    for i := range node.Audit.Gaps {
        if node.Audit.Gaps[i].Status == "open" {
            node.Audit.Gaps[i].FixedAt = ""
            node.Audit.Gaps[i].FixedBy = ""
        }
    }
    modified[issue.NodePath] = true

case CatAuditStatusTaskMismatch:
    // Re-sync via SyncAuditLifecycle
    SyncAuditLifecycle(node)
    modified[issue.NodePath] = true
```

---

## Priority 4: Test Coverage Improvements

### 4.1 Add Audit Lifecycle Tests

**Problem**: No tests for audit lifecycle state machine (since it doesn't exist yet — add alongside the implementation from 2.1).

**Reference**: Codex tests audit lifecycle in `audit_test.go` (3,208 lines of tests total) covering:
- Gap creation and lifecycle
- Escalation creation and resolution
- Audit task blocking when gaps are open
- Doctor detecting and fixing audit mismatches

**Tests to add** in a new file `internal/state/audit_lifecycle_test.go`:

```go
func TestSyncAuditLifecycle_NotStartedSetsPending(t *testing.T)
func TestSyncAuditLifecycle_InProgressSetsInProgress(t *testing.T)
func TestSyncAuditLifecycle_CompleteWithNoGapsSetsPassed(t *testing.T)
func TestSyncAuditLifecycle_CompleteWithOpenGapsBlocksTask(t *testing.T)
func TestSyncAuditLifecycle_BlockedSetsFailed(t *testing.T)
func TestSyncAuditLifecycle_FixingLastGapAllowsCompletion(t *testing.T)
```

### 4.2 Add Failure Escalation Tests

**Problem**: No tests for the failure escalation path in the daemon. The `==` bug (1.1) would have been caught by tests.

**Tests to add** in `internal/daemon/daemon_test.go` (new file):

```go
func TestFailureEscalation_BelowThreshold_ContinuesIteration(t *testing.T)
func TestFailureEscalation_AtThreshold_BelowMaxDepth_PromptsDecomposition(t *testing.T)
func TestFailureEscalation_AtThreshold_AtMaxDepth_AutoBlocks(t *testing.T)
func TestFailureEscalation_AboveThreshold_BelowMaxDepth_StillPromptsDecomposition(t *testing.T)
func TestFailureEscalation_AtHardCap_AutoBlocks(t *testing.T)
func TestFailureEscalation_AboveHardCap_AutoBlocks(t *testing.T)
```

These tests should exercise the `runIteration` function (or the extracted failure-handling logic) with mock state, verifying the correct task/node state after each scenario.

### 4.3 Add Navigation Determinism Test

**Tests to add** in `internal/state/navigation_test.go`:

```go
func TestFindNextTask_DeterministicOrder(t *testing.T) {
    // Create a root index with multiple top-level nodes
    // Run FindNextTask 100 times
    // Verify the same node is returned every time
}
```

### 4.4 Add Daemon Integration Tests

**Reference**: Codex has integration-style tests in `main_test.go` (3,208 lines) that exercise the full App lifecycle: init → project create → task add → task claim → task complete → status. These catch issues that unit tests miss.

Consider adding integration tests that exercise:
- Init → create project → add tasks → verify state files on disk
- Navigate → claim → complete → verify propagation
- Block → unblock → verify failure counter reset
- Doctor → verify fix application

---

## Priority 5: Structural Improvements

### 5.1 Add `moveAuditLast` Enforcement in TaskAdd

**Problem**: `TaskAdd` in `internal/state/mutations.go` lines 33-41 inserts new tasks before the audit task. This is correct. However, there's no re-enforcement if something else modifies the task list out of order.

**Reference**: Codex has a `moveAuditLast` function in `doctor.go` lines 613-624 that can be called as a repair:

```go
func moveAuditLast(node *NodeState) {
    idx := -1
    for i, t := range node.Tasks {
        if t.IsAudit {
            idx = i
            break
        }
    }
    if idx >= 0 && idx != len(node.Tasks)-1 {
        audit := node.Tasks[idx]
        node.Tasks = append(node.Tasks[:idx], node.Tasks[idx+1:]...)
        node.Tasks = append(node.Tasks, audit)
    }
}
```

Add a similar utility function in `internal/state/mutations.go` and call it at the end of any function that modifies the task list, as a safety invariant.

### 5.2 Improve Prompt Assembly with Failure Context

**Problem**: When a task has failed multiple times, the model receives no context about the failure history or decomposition policy.

**Reference**: Codex's `runtime_stage.go` `buildPrompt` (lines 79-133) includes failure context in the prompt. Specifically, it calls a function that describes the failure policy (thresholds, current count, depth) so the model can make informed decisions.

**Implementation**: In `internal/pipeline/context.go` `BuildIterationContext` (lines 11-58), add a section after the task description that includes:

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

### 5.3 Add Status Command Enhancements

**Reference**: Codex's `status.go` (lines 14-289) provides a rich status output including:
- Progress counts (total, complete, in_progress, blocked, not_started) — `ProgressCounts` struct lines 25-32
- Active task display — `ActiveTaskInfo` struct lines 34-37
- Blocked task summary — `BlockedTask` struct lines 39-42
- Audit counts (failed nodes, open gaps, escalations) — `AuditCounts` struct lines 44-49
- Daemon status (PID, mode, branch) — `DaemonStatus` struct lines 51-60

Verify that Claude's status command provides comparable detail. If not, enhance `cmd/status.go` to include audit counts and daemon status information.

---

## Priority 6: Robustness Improvements

### 6.1 Add PID File Atomicity

**Problem**: PID file creation should be atomic to prevent race conditions between daemon instances.

**Reference**: Codex's `ensureNoLivePID` in `daemon.go` lines 650-672 checks for existing PID file, reads the PID, and verifies the process is still running before writing a new one. This prevents two daemons from starting simultaneously.

Verify that Claude's PID management in `internal/daemon/daemon.go` follows the same check-then-create pattern. If it uses a simple write without checking for an existing live process, add the check.

### 6.2 Add Branch Verification in Daemon Loop

**Reference**: Both Gemini (`internal/daemon/daemon.go` lines 331-341) and Codex (`daemon.go` line 228-232, `verifyBranch` lines 796-805) verify that the git branch hasn't changed between iterations. If the branch changes, the daemon stops to prevent working on the wrong branch.

Verify this exists in Claude's daemon. If not, add a branch check at the start of each iteration in `internal/daemon/daemon.go` `Run` (around line 172-177).

### 6.3 Add Worktree Support

**Reference**: Gemini has the most complete worktree implementation in `internal/git/git.go`:
- `SetupWorktree` (lines 19-36): Creates a new git worktree for isolated execution
- `RemoveWorktree` (lines 39-44): Cleans up when done
- Start command integration in `internal/cli/start.go` lines 64-96

And Codex in `daemon.go` `ensureWorktree` (lines 729-747).

If Claude's `cmd/start.go` supports `--worktree`, verify the implementation. If not, add support for `wolfcastle start --worktree <branch>` that:
1. Creates a git worktree in a temp directory
2. Changes the daemon's working directory to the worktree
3. Cleans up the worktree on daemon stop

---

## Priority 7: Minor Fixes and Polish

### 7.1 Config Validation: Hard Cap >= Threshold

**Reference**: Claude already tests this in `internal/config/config_test.go` (`TestValidate_CatchesHardCapBelowDecompositionThreshold`). Verify the validation itself exists in `internal/config/validate.go`.

### 7.2 Add `normalizeAuditState` for Legacy Compatibility

**Reference**: Codex's `state_tree.go` lines 46-62 `normalizeAuditState` syncs between top-level and nested audit fields, handling cases where breadcrumbs or gaps might be stored at the wrong level. This is defensive programming for state files that might have been written by earlier versions.

Consider adding a similar normalization step in the state loading path (`internal/state/io.go`) to handle any legacy or malformed state files gracefully.

### 7.3 Ensure JSON Error Envelope Consistency

**Reference**: The spec requires JSON error output with `ok`, `error`, `code` fields. Claude's `internal/output/envelope.go` has `Ok` (line 19) and `Err` (line 24) functions.

Verify that ALL CLI commands use this envelope for JSON output, and that error output goes to stderr. Check each command in `cmd/` for direct `fmt.Fprintf(os.Stderr, ...)` calls that should use the envelope instead.

---

## Summary of Changes

| Priority | Change | Files Affected | Estimated Complexity |
|----------|--------|---------------|---------------------|
| P1 | Fix `==` to `>=` in failure threshold | `internal/daemon/daemon.go` | Trivial |
| P1 | Implement decomposition action | `internal/daemon/daemon.go`, `internal/pipeline/context.go`, `internal/state/types.go` | Medium |
| P1 | Fix non-deterministic navigation | `internal/state/navigation.go` | Small |
| P2 | Implement `syncAuditLifecycle` | New: `internal/state/audit_lifecycle.go`, modify `internal/state/mutations.go` | Medium |
| P2 | Add audit status fields to types | `internal/state/types.go` | Small |
| P3 | Add 6 audit validation categories | `internal/validate/types.go`, `internal/validate/engine.go`, `internal/validate/fix.go` | Medium |
| P4 | Add audit lifecycle tests | New: `internal/state/audit_lifecycle_test.go` | Medium |
| P4 | Add failure escalation tests | New: `internal/daemon/daemon_test.go` | Medium |
| P4 | Add navigation determinism test | `internal/state/navigation_test.go` | Small |
| P5 | Add `moveAuditLast` safety | `internal/state/mutations.go` | Small |
| P5 | Add failure context to prompts | `internal/pipeline/context.go` | Small |
| P6 | PID file atomicity | `internal/daemon/daemon.go` | Small |
| P6 | Branch verification | `internal/daemon/daemon.go` | Small |
| P7 | Normalize audit state on load | `internal/state/io.go` | Small |
