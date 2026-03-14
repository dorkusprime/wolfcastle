# Wolfcastle Claude Implementation — Improvement Specification

This document specifies concrete improvements to bring the Claude implementation to full spec compliance, drawing on patterns and solutions from the Codex and Gemini implementations. Each section includes the problem, the reference implementation to learn from, and the exact changes needed.

---

## Priority 1: Critical Algorithm & Spec Fixes

### 1.1 Expand Doctor Validation Engine to All 17 Issue Categories

**Problem**: `/Volumes/git/dorkusprime/wolfcastle-claude/cmd/doctor.go` currently checks ~7 issue categories. The spec requires 17 distinct issue categories with specific severity assignments and fix strategies (deterministic, model-assisted, manual).

**Reference**: Gemini's `/Volumes/git/dorkusprime/wolfcastle-gemini/internal/doctor/doctor.go:17-38` defines all 17 categories as typed constants. Codex's `/Volumes/git/dorkusprime/wolfcastle-codex/doctor.go:15-510` implements 20+ categories with inline fix logic.

**Changes needed in `/Volumes/git/dorkusprime/wolfcastle-claude/cmd/doctor.go`**:

Add the following issue categories (currently missing):

```go
// Add these category constants near the top of doctor.go
const (
    catRootIndexDanglingRef              = "ROOTINDEX_DANGLING_REF"           // existing as "orphan_index"
    catRootIndexMissingEntry             = "ROOTINDEX_MISSING_ENTRY"          // existing as "orphan_state"
    catOrphanState                       = "ORPHAN_STATE"                     // existing
    catOrphanDefinition                  = "ORPHAN_DEFINITION"               // NEW
    catPropagationMismatch               = "PROPAGATION_MISMATCH"            // NEW
    catMissingAuditTask                  = "MISSING_AUDIT_TASK"              // existing as "missing_audit"
    catAuditNotLast                      = "AUDIT_NOT_LAST"                  // existing as "audit_position"
    catMultipleAuditTasks                = "MULTIPLE_AUDIT_TASKS"            // NEW
    catInvalidStateValue                 = "INVALID_STATE_VALUE"             // existing as "invalid_state"
    catCompleteWithIncomplete            = "INVALID_TRANSITION_COMPLETE_WITH_INCOMPLETE" // NEW
    catBlockedWithoutReason              = "INVALID_TRANSITION_BLOCKED_WITHOUT_REASON"   // NEW
    catStaleInProgress                   = "STALE_IN_PROGRESS"               // NEW
    catMultipleInProgress                = "MULTIPLE_IN_PROGRESS"            // NEW
    catDepthMismatch                     = "DEPTH_MISMATCH"                  // NEW
    catNegativeFailureCount              = "NEGATIVE_FAILURE_COUNT"          // NEW
    catMissingRequiredField              = "MISSING_REQUIRED_FIELD"          // NEW
    catMalformedJSON                     = "MALFORMED_JSON"                  // NEW
)
```

Add a `FixType` field to the `doctorIssue` struct:

```go
type doctorIssue struct {
    Severity    string `json:"severity"`
    Category    string `json:"category"`
    Node        string `json:"node,omitempty"`
    Description string `json:"description"`
    CanAutoFix  bool   `json:"can_auto_fix"`
    FixType     string `json:"fix_type,omitempty"` // "deterministic", "model-assisted", "manual"
}
```

Add these new checks inside the `for addr, entry := range idx.Nodes` loop (after line 170):

1. **PROPAGATION_MISMATCH**: Recompute parent state and compare
   ```go
   if ns.Type == state.NodeOrchestrator {
       expected := state.RecomputeState(ns.Children)
       if ns.State != expected {
           issues = append(issues, doctorIssue{
               Severity: "error", Category: catPropagationMismatch, Node: addr,
               Description: fmt.Sprintf("Computed state is %s but stored is %s", expected, ns.State),
               CanAutoFix: true, FixType: "deterministic",
           })
       }
   }
   ```

2. **MULTIPLE_AUDIT_TASKS**: Count audit tasks
   ```go
   auditCount := 0
   for _, t := range ns.Tasks {
       if t.ID == "audit" { auditCount++ }
   }
   if auditCount > 1 {
       issues = append(issues, doctorIssue{
           Severity: "error", Category: catMultipleAuditTasks, Node: addr,
           Description: fmt.Sprintf("Leaf has %d audit tasks, expected 1", auditCount),
           FixType: "model-assisted",
       })
   }
   ```

3. **INVALID_TRANSITION_COMPLETE_WITH_INCOMPLETE**: Check leaves marked complete that have incomplete tasks
   ```go
   if ns.Type == state.NodeLeaf && ns.State == state.StatusComplete {
       for _, t := range ns.Tasks {
           if t.State != state.StatusComplete {
               issues = append(issues, doctorIssue{
                   Severity: "error", Category: catCompleteWithIncomplete, Node: addr,
                   Description: "Leaf is complete but has incomplete tasks",
                   FixType: "model-assisted",
               })
               break
           }
       }
   }
   ```

4. **BLOCKED_WITHOUT_REASON**: Check blocked nodes/tasks without reasons
   ```go
   if ns.State == state.StatusBlocked {
       // Check node-level block reason (not currently tracked — add to NodeState if needed)
   }
   for _, t := range ns.Tasks {
       if t.State == state.StatusBlocked && t.BlockReason == "" {
           issues = append(issues, doctorIssue{
               Severity: "error", Category: catBlockedWithoutReason, Node: addr,
               Description: fmt.Sprintf("Task %s is blocked without a reason", t.ID),
               CanAutoFix: true, FixType: "deterministic",
           })
       }
   }
   ```

5. **STALE_IN_PROGRESS / MULTIPLE_IN_PROGRESS**: Track in_progress tasks globally
   ```go
   // Outside the per-node loop, collect all in_progress tasks:
   var inProgressTasks []string
   // Inside per-node loop, for leaves:
   for _, t := range ns.Tasks {
       if t.State == state.StatusInProgress {
           inProgressTasks = append(inProgressTasks, addr+"/"+t.ID)
       }
   }
   // After the loop:
   if len(inProgressTasks) > 1 {
       issues = append(issues, doctorIssue{
           Severity: "error", Category: catMultipleInProgress,
           Description: fmt.Sprintf("Multiple tasks in progress: %s", strings.Join(inProgressTasks, ", ")),
           FixType: "model-assisted",
       })
   } else if len(inProgressTasks) == 1 {
       issues = append(issues, doctorIssue{
           Severity: "warning", Category: catStaleInProgress,
           Description: "Task in progress — may be stale if no daemon is running",
           FixType: "manual",
       })
   }
   ```

6. **DEPTH_MISMATCH**: Check child depth >= parent depth
   ```go
   if entry.Parent != "" {
       if parentEntry, ok := idx.Nodes[entry.Parent]; ok {
           parentStatePath := filepath.Join(resolver.ProjectsDir(), filepath.Join(tree.ParseAddressUnchecked(entry.Parent).Parts...), "state.json")
           parentNS, parentErr := state.LoadNodeState(parentStatePath)
           if parentErr == nil && ns.DecompositionDepth < parentNS.DecompositionDepth {
               issues = append(issues, doctorIssue{
                   Severity: "error", Category: catDepthMismatch, Node: addr,
                   Description: fmt.Sprintf("Child depth %d < parent depth %d", ns.DecompositionDepth, parentNS.DecompositionDepth),
                   CanAutoFix: true, FixType: "deterministic",
               })
           }
       }
   }
   ```

7. **NEGATIVE_FAILURE_COUNT**: Check for negative failure counts
   ```go
   for _, t := range ns.Tasks {
       if t.FailureCount < 0 {
           issues = append(issues, doctorIssue{
               Severity: "error", Category: catNegativeFailureCount, Node: addr,
               Description: fmt.Sprintf("Task %s has negative failure count: %d", t.ID, t.FailureCount),
               CanAutoFix: true, FixType: "deterministic",
           })
       }
   }
   ```

8. **MISSING_REQUIRED_FIELD**: Check for empty required fields
   ```go
   if ns.ID == "" || ns.Name == "" || string(ns.Type) == "" || string(ns.State) == "" {
       issues = append(issues, doctorIssue{
           Severity: "error", Category: catMissingRequiredField, Node: addr,
           Description: "Missing required field(s) in state.json",
           CanAutoFix: true, FixType: "deterministic",
       })
   }
   ```

9. **ORPHAN_DEFINITION**: Check for `.md` files without corresponding nodes (add to `checkOrphanedFiles`).

Add corresponding fix logic for each new auto-fixable category in the `--fix` section.

### 1.2 Add Model-Assisted Doctor Fix Strategy

**Problem**: Doctor only supports deterministic fixes. Spec requires three fix strategies.

**Reference**: Gemini's `/Volumes/git/dorkusprime/wolfcastle-gemini/internal/doctor/doctor.go:510-572` implements `tryModelAssistedFix()` that shells out to the configured doctor model.

**Changes needed**:

Add to `/Volumes/git/dorkusprime/wolfcastle-claude/cmd/doctor.go` after the deterministic fix section (around line 325):

```go
// Model-assisted fixes
if issue.FixType == "model-assisted" && cfg.Doctor.Model != "" {
    model, ok := cfg.Models[cfg.Doctor.Model]
    if ok {
        fixed, err := tryModelAssistedFix(model, issue, resolver.ProjectsDir())
        if err == nil && fixed {
            fixedItems = append(fixedItems, fmt.Sprintf("model-assisted: %s at %s", issue.Category, issue.Node))
        }
    }
}
```

Create a new function (can be in the same file or a new `/Volumes/git/dorkusprime/wolfcastle-claude/cmd/doctor_model.go`):

```go
func tryModelAssistedFix(model config.ModelDef, issue doctorIssue, projectsDir string) (bool, error) {
    prompt := fmt.Sprintf(`You are Wolfcastle Doctor, a structural repair agent.
An ambiguous state conflict has been found.
Node: %s
Issue: %s (%s)

Output a JSON object with your resolution:
{"resolution": "not_started|in_progress|complete|blocked", "reason": "explanation"}`,
        issue.Node, issue.Description, issue.Category)

    result, err := invoke.Invoke(context.Background(), model, prompt, projectsDir)
    if err != nil {
        return false, err
    }
    // Parse JSON response and apply (see Gemini doctor.go:537-570 for parsing pattern)
    // ...
    return true, nil
}
```

### 1.3 Add Explicit Self-Healing Startup Phase to Daemon

**Problem**: The daemon relies on navigation ordering for self-healing but lacks an explicit scan-and-resume phase per ADR-020.

**Reference**: Gemini's `/Volumes/git/dorkusprime/wolfcastle-gemini/internal/daemon/daemon.go:114-185` has a dedicated `selfHeal()` method.

**Changes needed in `/Volumes/git/dorkusprime/wolfcastle-claude/internal/daemon/daemon.go`**:

Add a `selfHeal` method and call it at the start of `Run()` (before line 80):

```go
func (d *Daemon) selfHeal(ctx context.Context) error {
    fmt.Println("Running self-healing check...")
    idx, err := d.Resolver.LoadRootIndex()
    if err != nil {
        return nil // No index yet — nothing to heal
    }

    var inProgress []struct{ addr, taskID string }
    for addr, entry := range idx.Nodes {
        if entry.Type != state.NodeLeaf { continue }
        a, err := tree.ParseAddress(addr)
        if err != nil { continue }
        ns, err := state.LoadNodeState(filepath.Join(d.Resolver.ProjectsDir(), filepath.Join(a.Parts...), "state.json"))
        if err != nil { continue }
        for _, t := range ns.Tasks {
            if t.State == state.StatusInProgress {
                inProgress = append(inProgress, struct{ addr, taskID string }{addr, t.ID})
            }
        }
    }

    if len(inProgress) > 1 {
        return fmt.Errorf("state corruption: %d tasks in progress (serial execution requires at most 1)", len(inProgress))
    }
    if len(inProgress) == 1 {
        fmt.Printf("Found interrupted task: %s/%s — will resume on next iteration\n",
            inProgress[0].addr, inProgress[0].taskID)
    } else {
        fmt.Println("No interrupted tasks found.")
    }
    return nil
}
```

Call it in `Run()` before the main loop:

```go
func (d *Daemon) Run(ctx context.Context) error {
    // ... signal setup (existing) ...

    // Self-healing phase (ADR-020)
    if err := d.selfHeal(ctx); err != nil {
        return fmt.Errorf("self-healing failed: %w", err)
    }

    // ... rest of loop ...
}
```

### 1.4 Add Daemon Supervisor with Restart Logic

**Problem**: If the daemon crashes mid-iteration, there is no automatic restart. The daemon exits on any error.

**Reference**: Codex's `/Volumes/git/dorkusprime/wolfcastle-codex/daemon.go:185-207` implements `runDaemonSupervisor()` with configurable `max_restarts` and `restart_delay_seconds`.

**Changes needed**:

Add supervisor config fields to `/Volumes/git/dorkusprime/wolfcastle-claude/internal/config/types.go`:

```go
type DaemonConfig struct {
    // ... existing fields ...
    MaxRestarts         int `json:"max_restarts"`
    RestartDelaySeconds int `json:"restart_delay_seconds"`
}
```

Add defaults in `/Volumes/git/dorkusprime/wolfcastle-claude/internal/config/config.go`:

```go
Daemon: DaemonConfig{
    // ... existing ...
    MaxRestarts:         3,
    RestartDelaySeconds: 2,
}
```

Wrap the main `Run()` loop in a supervisor in `/Volumes/git/dorkusprime/wolfcastle-claude/cmd/start.go` or a new `/Volumes/git/dorkusprime/wolfcastle-claude/internal/daemon/supervisor.go`:

```go
func (d *Daemon) RunWithSupervisor(ctx context.Context) error {
    maxRestarts := d.Config.Daemon.MaxRestarts
    delay := time.Duration(d.Config.Daemon.RestartDelaySeconds) * time.Second

    for restart := 0; ; restart++ {
        err := d.Run(ctx)
        if err == nil || ctx.Err() != nil {
            return err
        }
        if restart >= maxRestarts {
            return fmt.Errorf("daemon exceeded max restarts (%d): %w", maxRestarts, err)
        }
        fmt.Printf("Daemon crashed (attempt %d/%d): %v — restarting in %v\n", restart+1, maxRestarts, err, delay)
        time.Sleep(delay)
    }
}
```

---

## Priority 2: Missing Features from Codex

### 2.1 Add State Value Normalization to Doctor

**Problem**: If state.json is manually edited with typos like "completed", "done", or "in-progress", the current doctor just reports an invalid state.

**Reference**: Codex's `/Volumes/git/dorkusprime/wolfcastle-codex/doctor.go:647-660` has `normalizeStateValue()` that maps common typos.

**Changes needed**: Add to `/Volumes/git/dorkusprime/wolfcastle-claude/cmd/doctor.go` fix logic:

```go
func normalizeStateValue(s string) (state.NodeStatus, bool) {
    switch strings.ToLower(strings.TrimSpace(s)) {
    case "complete", "completed", "done":
        return state.StatusComplete, true
    case "not_started", "not-started", "pending", "todo":
        return state.StatusNotStarted, true
    case "in_progress", "in-progress", "started", "doing":
        return state.StatusInProgress, true
    case "blocked", "stuck":
        return state.StatusBlocked, true
    default:
        return "", false
    }
}
```

### 2.2 Add Full Audit Lifecycle Commands

**Problem**: `/Volumes/git/dorkusprime/wolfcastle-claude/cmd/audit_breadcrumb.go` and `/Volumes/git/dorkusprime/wolfcastle-claude/cmd/audit_escalate.go` exist but are stubs. Missing: gap management (open/fix), scope setting, result summary, escalation resolution, audit show.

**Reference**: Codex's `/Volumes/git/dorkusprime/wolfcastle-codex/auditcmd.go` and `/Volumes/git/dorkusprime/wolfcastle-codex/runtime_mutation.go` implement all of these. Codex's `/Volumes/git/dorkusprime/wolfcastle-codex/app.go` dispatches `audit` subcommands at lines 56-59.

**Changes needed**:

Create `/Volumes/git/dorkusprime/wolfcastle-claude/cmd/audit_gap.go`:
```go
// wolfcastle audit gap --node <addr> "description"
// Appends a new gap to the node's audit.gaps array
// Reference: /Volumes/git/dorkusprime/wolfcastle-codex/runtime_mutation.go:303-323 (appendAuditGap)
```

Create `/Volumes/git/dorkusprime/wolfcastle-claude/cmd/audit_fix_gap.go`:
```go
// wolfcastle audit fix-gap --node <addr> <gap-id>
// Marks a gap as fixed with fixed_by and fixed_at
// Reference: /Volumes/git/dorkusprime/wolfcastle-codex/runtime_mutation.go:325-345 (fixAuditGap)
```

Create `/Volumes/git/dorkusprime/wolfcastle-claude/cmd/audit_scope.go`:
```go
// wolfcastle audit scope --node <addr> --description "..." [--files "a|b"] [--systems "x|y"] [--criteria "c|d"]
// Sets structured audit scope on the node
// Reference: /Volumes/git/dorkusprime/wolfcastle-codex/runtime_mutation.go:237-267 (setAuditScopeDescription, setAuditScopeList)
```

Create `/Volumes/git/dorkusprime/wolfcastle-claude/cmd/audit_show.go`:
```go
// wolfcastle audit show --node <addr>
// Returns the full audit state (scope, breadcrumbs, gaps, escalations, status)
// Reference: /Volumes/git/dorkusprime/wolfcastle-codex/auditcmd.go
```

Create `/Volumes/git/dorkusprime/wolfcastle-claude/cmd/audit_resolve.go`:
```go
// wolfcastle audit resolve --node <addr> <escalation-id>
// Marks an escalation as resolved
// Reference: /Volumes/git/dorkusprime/wolfcastle-codex/runtime_mutation.go:351-370 (resolveAuditEscalation)
```

Implement the breadcrumb and escalate commands (currently stubs):
```go
// cmd/audit_breadcrumb.go — call state.AddBreadcrumb() (already exists in internal/state/mutations.go:131-138)
// cmd/audit_escalate.go — call state.AddEscalation() (already exists in internal/state/mutations.go:140-151)
```

### 2.3 Add Detached Daemon Mode and Stop File

**Problem**: `/Volumes/git/dorkusprime/wolfcastle-claude/cmd/start.go` supports `-d` for background mode but lacks the stop-file mechanism and stale PID recovery that Codex implements.

**Reference**: Codex's `/Volumes/git/dorkusprime/wolfcastle-codex/daemon.go:90-135` (cmdStart with stop file), `/Volumes/git/dorkusprime/wolfcastle-codex/daemon.go:363-433` (cmdStop with timeout and force), `/Volumes/git/dorkusprime/wolfcastle-codex/daemon.go:655-676` (recoverStaleDaemonState).

**Changes needed in `/Volumes/git/dorkusprime/wolfcastle-claude/cmd/start.go`**:

Add stop-file check in the daemon loop (currently at `/Volumes/git/dorkusprime/wolfcastle-claude/internal/daemon/daemon.go:80-87`):

```go
// After the signal check, add:
stopFilePath := filepath.Join(d.WolfcastleDir, "stop")
if _, err := os.Stat(stopFilePath); err == nil {
    os.Remove(stopFilePath)
    fmt.Println("=== Wolfcastle stopped by stop file ===")
    return nil
}
```

Add `recoverStaleDaemonState` to `/Volumes/git/dorkusprime/wolfcastle-claude/cmd/start.go` (call before starting daemon):

```go
func recoverStaleDaemonState(wolfcastleDir string) error {
    pidPath := filepath.Join(wolfcastleDir, "daemon.pid")
    data, err := os.ReadFile(pidPath)
    if os.IsNotExist(err) { return nil }
    if err != nil { return err }
    pid, err := strconv.Atoi(strings.TrimSpace(string(data)))
    if err != nil { os.Remove(pidPath); return nil }
    process, err := os.FindProcess(pid)
    if err != nil { os.Remove(pidPath); return nil }
    if err := process.Signal(syscall.Signal(0)); err != nil {
        // Process is dead — clean up stale files
        os.Remove(pidPath)
        os.Remove(filepath.Join(wolfcastleDir, "daemon.meta.json"))
        os.Remove(filepath.Join(wolfcastleDir, "stop"))
    }
    return nil
}
```

### 2.4 Add WOLFCASTLE_* Marker Parsing in Daemon Output

**Problem**: The daemon checks for WOLFCASTLE_YIELD, WOLFCASTLE_BLOCKED, WOLFCASTLE_COMPLETE but doesn't process WOLFCASTLE_BREADCRUMB, WOLFCASTLE_GAP, WOLFCASTLE_DECOMPOSE, etc.

**Reference**: Codex's `/Volumes/git/dorkusprime/wolfcastle-codex/runtime_mutation.go:16-79` (applyModelMarkers) parses 13 marker types from model output.

**Changes needed in `/Volumes/git/dorkusprime/wolfcastle-claude/internal/daemon/daemon.go`**:

After processing the model output (around line 253), add marker parsing:

```go
// After checking for terminal markers, parse mutation markers:
lines := strings.Split(result.Stdout, "\n")
for _, line := range lines {
    line = strings.TrimSpace(line)
    switch {
    case strings.HasPrefix(line, "WOLFCASTLE_BREADCRUMB:"):
        text := strings.TrimSpace(strings.TrimPrefix(line, "WOLFCASTLE_BREADCRUMB:"))
        state.AddBreadcrumb(ns, nav.NodeAddress+"/"+nav.TaskID, text)
    case strings.HasPrefix(line, "WOLFCASTLE_GAP:"):
        // Add gap to audit...
    case strings.HasPrefix(line, "WOLFCASTLE_FIX_GAP:"):
        // Fix gap...
    case strings.HasPrefix(line, "WOLFCASTLE_SCOPE:"):
        // Set scope description...
    case strings.HasPrefix(line, "WOLFCASTLE_DECOMPOSE:"):
        // Trigger decomposition...
    }
}
```

### 2.5 Add Overlap Advisory

**Problem**: `/Volumes/git/dorkusprime/wolfcastle-claude/cmd/overlap.go` is a stub. The spec requires cross-engineer overlap detection.

**Reference**: Codex's `/Volumes/git/dorkusprime/wolfcastle-codex/overlap.go:17-176` implements `computeOverlapAdvisories()` with token-based and bigram similarity scoring.

**Changes needed**: Implement `/Volumes/git/dorkusprime/wolfcastle-claude/cmd/overlap.go` following Codex's approach:
1. Scan all engineer namespaces in `.wolfcastle/projects/`
2. Compare project names using token overlap and slug similarity
3. Return sorted advisory list
4. Call from `/Volumes/git/dorkusprime/wolfcastle-claude/cmd/project_create.go` after successful creation

### 2.6 Add Immutable Config Merge (Clone-Before-Merge)

**Problem**: `/Volumes/git/dorkusprime/wolfcastle-claude/internal/config/merge.go` mutates the `dst` map in place. If the same base config map is reused, later merges could see stale state.

**Reference**: Codex's `/Volumes/git/dorkusprime/wolfcastle-codex/config.go:71-114` uses `cloneJSONObject`/`cloneJSONValue` to deep-clone before merging.

**Changes needed in `/Volumes/git/dorkusprime/wolfcastle-claude/internal/config/merge.go`**:

```go
func cloneMap(in map[string]any) map[string]any {
    out := make(map[string]any, len(in))
    for k, v := range in {
        out[k] = cloneValue(v)
    }
    return out
}

func cloneValue(v any) any {
    switch typed := v.(type) {
    case map[string]any:
        return cloneMap(typed)
    case []any:
        out := make([]any, len(typed))
        for i, item := range typed {
            out[i] = cloneValue(item)
        }
        return out
    default:
        return typed
    }
}

// Update DeepMerge to clone first:
func DeepMerge(dst, src map[string]any) map[string]any {
    result := cloneMap(dst) // Clone to avoid mutating original
    for k, sv := range src {
        if sv == nil {
            delete(result, k)
            continue
        }
        dv, exists := result[k]
        if !exists {
            result[k] = cloneValue(sv)
            continue
        }
        dMap, dIsMap := dv.(map[string]any)
        sMap, sIsMap := sv.(map[string]any)
        if dIsMap && sIsMap {
            result[k] = DeepMerge(dMap, sMap)
            continue
        }
        result[k] = cloneValue(sv)
    }
    return result
}
```

---

## Priority 3: Testing Gaps

### 3.1 Add Tests for New Doctor Categories

For each new category added in 1.1, add corresponding test cases in a new `/Volumes/git/dorkusprime/wolfcastle-claude/cmd/doctor_test.go` or extend existing tests:

```go
func TestDoctor_DetectsPropagationMismatch(t *testing.T) { ... }
func TestDoctor_DetectsMultipleAuditTasks(t *testing.T) { ... }
func TestDoctor_DetectsCompleteWithIncomplete(t *testing.T) { ... }
func TestDoctor_DetectsBlockedWithoutReason(t *testing.T) { ... }
func TestDoctor_DetectsMultipleInProgress(t *testing.T) { ... }
func TestDoctor_DetectsDepthMismatch(t *testing.T) { ... }
func TestDoctor_DetectsNegativeFailureCount(t *testing.T) { ... }
func TestDoctor_DetectsMissingRequiredField(t *testing.T) { ... }
func TestDoctor_FixesPropagationMismatch(t *testing.T) { ... }
func TestDoctor_FixesDepthMismatch(t *testing.T) { ... }
func TestDoctor_FixesNegativeFailureCount(t *testing.T) { ... }
```

### 3.2 Add Failure Escalation Tests

Test the decomposition threshold logic in the daemon. Currently no tests exist for this path.

```go
func TestDaemon_DecompositionThresholdAtMaxDepth(t *testing.T) { ... }
func TestDaemon_HardCapAutoBlocks(t *testing.T) { ... }
func TestDaemon_BelowThresholdKeepsIterating(t *testing.T) { ... }
```

### 3.3 Add Single-In-Progress Invariant Test

The invariant is enforced at navigation level but never explicitly tested for the violation case:

```go
func TestTaskClaim_RejectsSecondInProgress(t *testing.T) {
    // Create two leaves, claim in leaf-1, try to claim in leaf-2
    // Should fail or not find work for leaf-2
}
```

Reference: Codex `/Volumes/git/dorkusprime/wolfcastle-codex/main_test.go` has `TestTaskClaimRejectsSecondInProgressTask` at approximately line 796.

---

## Priority 4: Minor Improvements

### 4.1 Add `Root` Array to Root Index

**Problem**: Finding root nodes requires iterating all entries and checking for empty `Parent`. Codex adds a `Root []string` array for O(1) root discovery.

**Reference**: Codex's `/Volumes/git/dorkusprime/wolfcastle-codex/types.go:91-95` (`RootIndex` has `Root []string`).

### 4.2 Add `--force` to Stop Command

Implement `/Volumes/git/dorkusprime/wolfcastle-claude/cmd/stop.go` with both graceful (SIGTERM + timeout) and force (SIGKILL) modes. Reference: Codex's `/Volumes/git/dorkusprime/wolfcastle-codex/daemon.go:363-433`.

### 4.3 Add Daemon Startup Doctor Check

Run a subset of doctor checks before starting the daemon and block on errors.

**Reference**: Codex's `/Volumes/git/dorkusprime/wolfcastle-codex/daemon.go:76-84` runs `runDoctor` at startup and fails if any error-severity issues exist.

### 4.4 Add Audit Task Lifecycle Sync

When the audit task completes, check for open gaps and auto-block if any exist.

**Reference**: Codex's `/Volumes/git/dorkusprime/wolfcastle-codex/state_tree.go:210-260` (`syncAuditLifecycle`) maps audit task state to audit status and handles the gap-blocking logic.

### 4.5 Implement Remaining CLI Stubs

The following `/Volumes/git/dorkusprime/wolfcastle-claude/cmd/` files are stubs that need implementation:
- `/Volumes/git/dorkusprime/wolfcastle-claude/cmd/status.go` — aggregate progress stats (reference: Codex `/Volumes/git/dorkusprime/wolfcastle-codex/status.go`)
- `/Volumes/git/dorkusprime/wolfcastle-claude/cmd/stop.go` — graceful daemon stop (reference: Codex `/Volumes/git/dorkusprime/wolfcastle-codex/daemon.go:363-433`)
- `/Volumes/git/dorkusprime/wolfcastle-claude/cmd/follow.go` — tail daemon logs (reference: Codex `/Volumes/git/dorkusprime/wolfcastle-codex/daemon.go:435-511`)
- `/Volumes/git/dorkusprime/wolfcastle-claude/cmd/archive_add.go` — trigger archive generation (reference: Codex `/Volumes/git/dorkusprime/wolfcastle-codex/archive.go`)
- `/Volumes/git/dorkusprime/wolfcastle-claude/cmd/update.go` — regenerate base assets (reference: Codex /Volumes/git/dorkusprime/wolfcastle-codex/app.go `cmdUpdate`)
- `/Volumes/git/dorkusprime/wolfcastle-claude/cmd/install.go` — install skill bundle (reference: Codex `/Volumes/git/dorkusprime/wolfcastle-codex/daemon.go:524-559` `installSkillBundle`)
