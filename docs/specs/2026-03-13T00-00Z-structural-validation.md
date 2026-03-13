# Structural Validation Engine

This spec defines the structural validation engine that powers `wolfcastle doctor` and the daemon startup health checks. It is the core infrastructure for detecting, classifying, and repairing inconsistencies in the distributed state tree.

**Governing ADRs**: ADR-002 (JSON state), ADR-003 (deterministic scripts), ADR-007 (audit invariants), ADR-008 (tree addressing), ADR-009 (project layout), ADR-014 (serial execution), ADR-019 (failure/decomposition), ADR-020 (daemon lifecycle, self-healing, stale PID), ADR-024 (distributed state files, per-node state.json, root index), ADR-025 (wolfcastle doctor, validation as infrastructure).

---

## 1. Validation Categories

The validation engine checks for every category of structural issue that can arise in the distributed state tree. Each category has an identifier (used in reports and the API), a human-readable description, a severity level, and a fix strategy.

### 1.1 Root Index Inconsistencies

The root `state.json` (at `.wolfcastle/projects/{identity}/state.json`) is the centralized index of the full tree structure. It must be consistent with the per-node state files that exist on disk.

#### ROOTINDEX_DANGLING_REF

**Description**: The root index references a node, but no corresponding `state.json` exists on disk at the expected directory path.

**Detection**: For every node registered in the root index, check that `{node-dir}/state.json` exists on the filesystem.

**Severity**: Error

**Example**: Root index lists `attunement-tree/fire-impl` as a child, but `.wolfcastle/projects/wild-macbook/attunement-tree/fire-impl/state.json` does not exist.

#### ROOTINDEX_MISSING_ENTRY

**Description**: A per-node `state.json` exists on disk, but no entry in the root index references it.

**Detection**: Walk the filesystem under the engineer namespace directory. For every directory containing a `state.json` (excluding the root itself), verify that the root index contains a corresponding entry for that path.

**Severity**: Error

### 1.2 Orphaned State Files

#### ORPHAN_STATE

**Description**: A `state.json` exists in a node directory, but no parent node's `children` array references the node.

**Detection**: For every per-node `state.json` found on disk, verify that its parent node (derived from the directory path) lists it in its `children` array. The root node is exempt (it has no parent).

**Severity**: Error

**Example**: `.wolfcastle/projects/wild-macbook/attunement-tree/ice-impl/state.json` exists, but `attunement-tree`'s `state.json` does not list `ice-impl` in its `children` array.

### 1.3 Orphaned Definition Files

#### ORPHAN_DEFINITION

**Description**: A Markdown definition file (e.g., `fire-impl.md`) exists in the engineer namespace, but no corresponding node exists in the state tree (neither root index nor per-node state).

**Detection**: Walk the filesystem for `.md` files under the engineer namespace. For each file, derive the node address from the file path and check whether a corresponding node exists in state. Exclude task working documents (`task-*.md`) which are optional companions, not node definitions. A task working document is orphaned only if the parent leaf node does not exist.

**Severity**: Warning

**Example**: `.wolfcastle/projects/wild-macbook/attunement-tree/ice-impl.md` exists, but there is no `ice-impl` node under `attunement-tree` in any state file.

### 1.4 State Propagation Errors

#### PROPAGATION_MISMATCH

**Description**: An orchestrator node's state does not match the result of recomputing it from its children's states using the propagation algorithm (state machine spec, Section 5.1).

**Detection**: For every orchestrator node, read all children's states and run `recompute_parent()`. Compare the result to the orchestrator's stored state. Any difference is a propagation error.

**Severity**: Error

**Example**: An orchestrator's state is `complete`, but one of its children is `in_progress`.

### 1.5 Missing Audit Tasks

#### MISSING_AUDIT_TASK

**Description**: A leaf node has no task with `is_audit: true`, or its task list is empty.

**Detection**: For every leaf node, check that its `tasks` array is non-empty and that exactly one task has `is_audit: true`.

**Severity**: Error

### 1.6 Audit Task Position Violations

#### AUDIT_NOT_LAST

**Description**: A leaf node has an audit task (`is_audit: true`), but it is not the last element in the `tasks` array.

**Detection**: For every leaf node, check that the last element of the `tasks` array has `is_audit: true`, and that no other element has `is_audit: true`.

**Severity**: Error

#### MULTIPLE_AUDIT_TASKS

**Description**: A leaf node has more than one task with `is_audit: true`.

**Detection**: Count tasks where `is_audit: true` in each leaf. If count > 1, report.

**Severity**: Error

### 1.7 Invalid State Values

#### INVALID_STATE_VALUE

**Description**: A node or task has a `state` field that is not one of the four valid values: `not_started`, `in_progress`, `complete`, `blocked`.

**Detection**: For every node and every task, validate that `state` is one of the four valid enum values.

**Severity**: Error

### 1.8 Invalid State Transitions

#### INVALID_TRANSITION_COMPLETE_WITH_INCOMPLETE

**Description**: A node is marked `complete`, but it contains tasks or children that are not `complete`.

**Detection**: For leaf nodes in `complete` state, verify all tasks are `complete`. For orchestrator nodes in `complete` state, verify all children are `complete`.

**Severity**: Error

**Example**: A leaf node's state is `complete`, but `task-2` is `in_progress`.

#### INVALID_TRANSITION_BLOCKED_WITHOUT_REASON

**Description**: A node or task has `state: "blocked"` but no `blocked_reason` field, or the field is empty.

**Detection**: For every node and task with `state == "blocked"`, verify that `blocked_reason` is present and non-empty.

**Severity**: Error

### 1.9 Stale In Progress

#### STALE_IN_PROGRESS

**Description**: A task is in `in_progress` state, but no daemon process is running. This indicates a crash or hard kill interrupted work.

**Detection**: Find any task with `state == "in_progress"`. Check whether a Wolfcastle daemon PID file exists at `.wolfcastle/wolfcastle.pid` and whether the process at that PID is actually running. If no daemon is running and a task is `in_progress`, the state is stale.

Additionally, if more than one task across the entire tree is `in_progress`, this is always an error regardless of daemon status (serial execution invariant, ADR-014).

**Severity**: Warning (single stale task, expected recovery path), Error (multiple `in_progress` tasks)

**Note**: A single stale `in_progress` task is a Warning because the daemon's self-healing mechanism (state machine spec, Section 9) handles this on restart by navigating to the task and letting the model decide how to proceed. The doctor reports it but does not auto-fix it, since the self-healing path is the intended recovery. Multiple `in_progress` tasks are an Error because this violates the serial execution invariant and cannot be self-healed.

#### MULTIPLE_IN_PROGRESS

**Description**: More than one task across the entire tree has `state: "in_progress"`. This violates the serial execution invariant (ADR-014).

**Detection**: Count all tasks with `state == "in_progress"` across every leaf in the tree. If count > 1, report all of them.

**Severity**: Error

### 1.10 Failure Counter Inconsistencies

#### DEPTH_MISMATCH

**Description**: A child node's `decomposition_depth` is less than its parent's `decomposition_depth`. Per the state machine spec (Section 6.2), a child's depth must be >= its parent's depth.

**Detection**: For every parent-child relationship, verify `child.decomposition_depth >= parent.decomposition_depth`.

**Severity**: Error

#### NEGATIVE_FAILURE_COUNT

**Description**: A task's `failure_count` is negative.

**Detection**: For every task in every leaf, verify `failure_count >= 0`.

**Severity**: Error

### 1.11 Missing Required Fields

#### MISSING_REQUIRED_FIELD

**Description**: A `state.json` file is valid JSON but is missing one or more fields required by the schema (state machine spec, Section 10.4).

**Detection**: Validate every `state.json` against the required fields for its node type:
- All nodes require: `id`, `name`, `type`, `state`, `decomposition_depth`
- Orchestrator nodes additionally require: `children` (array)
- Leaf nodes additionally require: `tasks` (array)
- All tasks require: `id`, `description`, `state`, `failure_count`
- The root `state.json` requires: `version`, `root`

**Severity**: Error

### 1.12 Malformed State Files

#### MALFORMED_JSON

**Description**: A `state.json` file exists but cannot be parsed as valid JSON.

**Detection**: Attempt to parse every `state.json` found under the engineer namespace. If parsing fails, report the file path and the parse error.

**Severity**: Error

---

## 2. Severity Levels

Every issue found by the validation engine is classified into one of three severity levels. Severity determines the urgency of the fix and whether the daemon can proceed.

| Severity | Meaning | Daemon behavior | Doctor behavior |
|----------|---------|-----------------|-----------------|
| **Error** | Structural invariant violated. The tree is in an inconsistent state that may cause incorrect behavior. Must be fixed before the daemon can safely operate. | Daemon refuses to start if any Error-severity issue exists (startup subset). | Reports the issue and offers a fix (deterministic or model-assisted). |
| **Warning** | Potential issue that should be addressed but does not prevent correct operation. May indicate a crash recovery scenario or cosmetic problem. | Daemon starts but logs a warning. | Reports the issue and offers a fix if available. |
| **Info** | Cosmetic observation. No functional impact. | Daemon starts normally, no log entry. | Reports the issue for awareness. No fix offered. |

### Severity Assignment Table

| Issue ID | Severity |
|----------|----------|
| `ROOTINDEX_DANGLING_REF` | Error |
| `ROOTINDEX_MISSING_ENTRY` | Error |
| `ORPHAN_STATE` | Error |
| `ORPHAN_DEFINITION` | Warning |
| `PROPAGATION_MISMATCH` | Error |
| `MISSING_AUDIT_TASK` | Error |
| `AUDIT_NOT_LAST` | Error |
| `MULTIPLE_AUDIT_TASKS` | Error |
| `INVALID_STATE_VALUE` | Error |
| `INVALID_TRANSITION_COMPLETE_WITH_INCOMPLETE` | Error |
| `INVALID_TRANSITION_BLOCKED_WITHOUT_REASON` | Error |
| `STALE_IN_PROGRESS` | Warning |
| `MULTIPLE_IN_PROGRESS` | Error |
| `DEPTH_MISMATCH` | Error |
| `NEGATIVE_FAILURE_COUNT` | Error |
| `MISSING_REQUIRED_FIELD` | Error |
| `MALFORMED_JSON` | Error |

---

## 3. Deterministic vs Model-Assisted Fixes

Every issue type has a fix strategy. Deterministic fixes are applied directly by Go code without model involvement. Model-assisted fixes require the doctor's configured model to reason about the correct resolution because the fix is ambiguous or context-dependent.

Some issues are unfixable by the engine and require manual intervention.

### Fix Strategy Table

| Issue ID | Fix Strategy | Fix Description |
|----------|-------------|-----------------|
| `ROOTINDEX_DANGLING_REF` | **Deterministic** | Remove the dangling entry from the root index. The referenced node does not exist, so the reference is invalid. |
| `ROOTINDEX_MISSING_ENTRY` | **Deterministic** | Add the missing entry to the root index. The node exists on disk with valid state, so register it. |
| `ORPHAN_STATE` | **Model-assisted** | Ambiguous: the orphan may be from a partially completed decomposition, a manual filesystem operation, or a node that was removed from its parent but whose directory was not cleaned up. The model examines the orphan's state and its potential parent to decide whether to re-register it as a child or delete it. |
| `ORPHAN_DEFINITION` | **Deterministic** | Delete the orphaned Markdown file. Definition files without a corresponding node serve no purpose. Alternatively, if the user wants to keep orphaned definitions, doctor can skip this with `--skip-orphan-defs`. |
| `PROPAGATION_MISMATCH` | **Deterministic** | Recompute the orchestrator's state from its children using the propagation algorithm. The children's states are the source of truth; the parent's stored state is derived. |
| `MISSING_AUDIT_TASK` | **Deterministic** | Append an audit task to the leaf's task list with `is_audit: true`, `state: "not_started"`, `failure_count: 0`, and a default description derived from the node name: `"Verify {node-name} implementation"`. |
| `AUDIT_NOT_LAST` | **Deterministic** | Move the audit task to the last position in the task array. Preserve the relative order of all other tasks. |
| `MULTIPLE_AUDIT_TASKS` | **Model-assisted** | Ambiguous: which audit task is the "real" one? The model examines the descriptions, states, and breadcrumbs of each audit task to determine which to keep and which to merge or remove. |
| `INVALID_STATE_VALUE` | **Model-assisted** | The model examines the invalid value, the node's context (children states, task states, breadcrumbs), and infers the intended valid state. Common typos (e.g., `"completed"` -> `"complete"`, `"pending"` -> `"not_started"`) may be resolved deterministically as a fast path before falling through to model assistance. |
| `INVALID_TRANSITION_COMPLETE_WITH_INCOMPLETE` | **Model-assisted** | Ambiguous: is the node actually complete (and the child/task state is stale), or is the node not actually complete (and the node state is wrong)? The model examines breadcrumbs, task history, and audit results to determine which direction to correct. |
| `INVALID_TRANSITION_BLOCKED_WITHOUT_REASON` | **Deterministic** | Set `blocked_reason` to `"Blocked reason missing — added by wolfcastle doctor"`. This preserves the blocked state while making the invariant hold. The user can edit the reason later. |
| `STALE_IN_PROGRESS` | **No fix** | The daemon's self-healing mechanism handles this on startup (state machine spec, Section 9). Doctor reports it for awareness but does not change the state, because the model needs to inspect the working directory and decide how to proceed. |
| `MULTIPLE_IN_PROGRESS` | **Model-assisted** | The model examines the timestamps, breadcrumbs, and task positions of all `in_progress` tasks to determine which one was the genuinely active task. All others are reset to their previous valid state (typically `not_started` if no breadcrumbs exist, or left as `in_progress` for the one legitimate task). |
| `DEPTH_MISMATCH` | **Deterministic** | Set the child's `decomposition_depth` to `max(child.decomposition_depth, parent.decomposition_depth)`. The depth can only increase through decomposition, so the parent's depth is the lower bound. |
| `NEGATIVE_FAILURE_COUNT` | **Deterministic** | Set `failure_count` to `0`. Negative failure counts are always invalid and zero is the safe default. |
| `MISSING_REQUIRED_FIELD` | **Deterministic** (for fields with obvious defaults) / **Model-assisted** (for fields like `name`, `id`) | Fields with schema defaults (`decomposition_depth: 0`, `failure_count: 0`, `state: "not_started"`) are filled deterministically. Fields that require semantic content (`name`, `description`) use the model to infer from context (directory name, sibling nodes, definition file content). |
| `MALFORMED_JSON` | **Manual** | Cannot be auto-fixed. Doctor reports the parse error with file path and byte offset. The user must repair or delete the file. If a backup exists (from a prior successful write), doctor may offer to restore from git history. |

### Fix Strategy Summary

| Strategy | Count | Description |
|----------|-------|-------------|
| Deterministic | 9 | Go code fixes directly. No model tokens spent. |
| Model-assisted | 5 | Requires model reasoning. Bounded by guardrails (Section 8). |
| No fix | 1 | Intentionally deferred to daemon self-healing. |
| Manual | 1 | Requires human intervention. |

---

## 4. Validation API

The validation engine is a Go package (`pkg/validate`) that provides composable check functions and a top-level runner. Individual checks can be run in isolation (for testing, CI, or targeted diagnostics) or composed into suites.

### 4.1 Core Types

```go
package validate

import "time"

// Severity classifies the urgency of a validation issue.
type Severity int

const (
    SeverityInfo    Severity = iota
    SeverityWarning
    SeverityError
)

// FixStrategy describes how an issue can be resolved.
type FixStrategy int

const (
    FixDeterministic FixStrategy = iota
    FixModelAssisted
    FixManual
    FixNone // Intentionally deferred (e.g., stale in-progress)
)

// Issue represents a single structural problem found during validation.
type Issue struct {
    // ID is the machine-readable issue identifier (e.g., "ROOTINDEX_DANGLING_REF").
    ID string

    // Severity classifies urgency.
    Severity Severity

    // Message is a human-readable description of the specific instance.
    Message string

    // NodePath is the tree address of the affected node, if applicable.
    // Empty for issues that are not node-specific (e.g., malformed root index).
    NodePath string

    // FilePath is the filesystem path of the affected file, if applicable.
    FilePath string

    // Field is the JSON field name within the file, if applicable.
    Field string

    // FixStrategy indicates how this issue can be resolved.
    FixStrategy FixStrategy

    // FixDescription is a human-readable explanation of the proposed fix.
    FixDescription string
}

// Report is the result of a validation run.
type Report struct {
    // Timestamp is when the validation ran.
    Timestamp time.Time

    // EngineerNamespace is the identity namespace that was validated.
    EngineerNamespace string

    // Issues is the list of all issues found, in detection order.
    Issues []Issue

    // Duration is how long the validation took.
    Duration time.Duration
}

// Counts returns issue counts by severity.
func (r *Report) Counts() map[Severity]int

// HasErrors returns true if any issue has Error severity.
func (r *Report) HasErrors() bool

// HasWarnings returns true if any issue has Warning severity.
func (r *Report) HasWarnings() bool

// ByCategory groups issues by their ID prefix.
func (r *Report) ByCategory() map[string][]Issue

// Deterministic returns only issues with deterministic fixes.
func (r *Report) Deterministic() []Issue

// ModelAssisted returns only issues requiring model reasoning.
func (r *Report) ModelAssisted() []Issue
```

### 4.2 Check Interface

Each validation category is implemented as a `Check`. Checks are composable: you can run one, several, or all.

```go
// Check is a single validation check that examines one category of issue.
type Check interface {
    // ID returns the check identifier (e.g., "root_index", "audit_position").
    ID() string

    // Description returns a human-readable description of what this check validates.
    Description() string

    // Run executes the check against the given state tree and returns any issues found.
    // The TreeState parameter provides read access to the full tree and filesystem.
    Run(ctx context.Context, tree TreeState) ([]Issue, error)
}

// TreeState provides read-only access to the state tree and filesystem for validation.
type TreeState interface {
    // RootIndex returns the parsed root state.json.
    RootIndex() (*RootState, error)

    // NodeState returns the parsed state.json for a specific node path.
    NodeState(nodePath string) (*NodeState, error)

    // NodeExists checks whether a state.json exists at the given node path.
    NodeExists(nodePath string) bool

    // WalkNodes calls fn for every node directory found on disk under the namespace.
    WalkNodes(fn func(nodePath string, state *NodeState) error) error

    // WalkDefinitions calls fn for every .md file found under the namespace.
    WalkDefinitions(fn func(filePath string, nodePath string) error) error

    // DaemonRunning checks whether a daemon process is currently running.
    DaemonRunning() bool

    // NamespacePath returns the absolute filesystem path to the engineer namespace.
    NamespacePath() string
}
```

### 4.3 Built-In Checks

The engine ships with these checks, each implementing the `Check` interface:

| Check ID | Issues detected | Used in startup | Used in doctor |
|----------|----------------|:---:|:---:|
| `root_index` | `ROOTINDEX_DANGLING_REF`, `ROOTINDEX_MISSING_ENTRY` | Yes | Yes |
| `orphan_state` | `ORPHAN_STATE` | Yes | Yes |
| `orphan_definition` | `ORPHAN_DEFINITION` | No | Yes |
| `state_propagation` | `PROPAGATION_MISMATCH` | Yes | Yes |
| `audit_task` | `MISSING_AUDIT_TASK`, `AUDIT_NOT_LAST`, `MULTIPLE_AUDIT_TASKS` | Yes | Yes |
| `state_values` | `INVALID_STATE_VALUE` | Yes | Yes |
| `state_transitions` | `INVALID_TRANSITION_COMPLETE_WITH_INCOMPLETE`, `INVALID_TRANSITION_BLOCKED_WITHOUT_REASON` | Yes | Yes |
| `in_progress` | `STALE_IN_PROGRESS`, `MULTIPLE_IN_PROGRESS` | Yes | Yes |
| `failure_counters` | `DEPTH_MISMATCH`, `NEGATIVE_FAILURE_COUNT` | No | Yes |
| `required_fields` | `MISSING_REQUIRED_FIELD` | Yes | Yes |
| `json_integrity` | `MALFORMED_JSON` | Yes | Yes |

### 4.4 Runner

The `Runner` composes checks into suites and executes them.

```go
// Runner executes validation checks and produces a report.
type Runner struct {
    checks []Check
}

// NewRunner creates a runner with the given checks.
func NewRunner(checks ...Check) *Runner

// AllChecks returns a Runner with every built-in check registered.
func AllChecks() *Runner

// StartupChecks returns a Runner with only the fast, critical checks
// suitable for daemon startup (see Section 5).
func StartupChecks() *Runner

// Run executes all registered checks against the given tree state.
// Checks run sequentially in registration order.
// If a check returns an error (not issues — an operational error like I/O failure),
// the runner logs the error and continues with the remaining checks.
func (r *Runner) Run(ctx context.Context, tree TreeState) (*Report, error)
```

### 4.5 Fix Application Interface

Fixes are applied through a separate `Fixer` that takes a `Report` and applies fixes atomically.

```go
// Fixer applies fixes for validation issues.
type Fixer struct {
    tree     TreeState
    model    ModelInvoker // For model-assisted fixes; nil if deterministic-only mode
}

// FixResult describes the outcome of applying a fix.
type FixResult struct {
    Issue       Issue
    Applied     bool
    Description string
    Error       error
}

// FixDeterministic applies all deterministic fixes from the report.
// Returns results for each attempted fix.
// All fixes within a single state file are applied atomically (Section 7).
func (f *Fixer) FixDeterministic(ctx context.Context, report *Report) ([]FixResult, error)

// FixWithModel applies model-assisted fixes from the report.
// Each ambiguous issue is sent to the model with context (Section 8).
// The model's proposed fix is validated by Go code before application.
func (f *Fixer) FixWithModel(ctx context.Context, report *Report) ([]FixResult, error)

// ModelInvoker abstracts model invocation for doctor fixes.
type ModelInvoker interface {
    // Invoke sends a prompt to the configured doctor model and returns the response.
    Invoke(ctx context.Context, prompt string) (string, error)
}
```

---

## 5. Startup Subset

When the daemon starts (`wolfcastle start`), it runs a subset of validation checks before entering the main loop. These checks are fast (no filesystem walk for orphan definitions, no deep failure counter analysis) and focus on issues that would cause incorrect behavior during execution.

### Startup Check Set

| Check | Why at startup |
|-------|---------------|
| `json_integrity` | Cannot operate on corrupt state files. |
| `required_fields` | Missing fields cause nil dereferences or incorrect defaults. |
| `root_index` | Dangling refs cause navigation failures; missing entries hide work. |
| `orphan_state` | Orphaned state files indicate a tree that is out of sync. |
| `state_values` | Invalid state values break the state machine. |
| `state_transitions` | Inconsistent states cause wrong navigation decisions. |
| `state_propagation` | Wrong derived states cause the daemon to skip or repeat work. |
| `audit_task` | Missing or misplaced audit tasks break the execution invariant. |
| `in_progress` | Detects crash recovery scenario; single stale task is expected and logged. Multiple `in_progress` is fatal. |

### Checks NOT Run at Startup

| Check | Why deferred to doctor |
|-------|----------------------|
| `orphan_definition` | Requires a full filesystem walk for `.md` files. Cosmetic issue that does not affect correctness. |
| `failure_counters` | Depth mismatches and negative counts are edge cases that do not affect navigation or execution safety. Worth checking but not blocking startup. |

### Startup Behavior

```go
func (d *Daemon) startup(ctx context.Context) error {
    tree := loadTreeState(d.namespacePath)
    runner := validate.StartupChecks()
    report, err := runner.Run(ctx, tree)
    if err != nil {
        return fmt.Errorf("validation failed: %w", err)
    }

    if report.HasErrors() {
        // Log each error-severity issue
        for _, issue := range report.Issues {
            if issue.Severity == validate.SeverityError {
                d.log.Error("structural issue", "id", issue.ID, "path", issue.NodePath, "msg", issue.Message)
            }
        }
        return fmt.Errorf("found %d structural errors — run 'wolfcastle doctor' to fix", report.Counts()[validate.SeverityError])
    }

    // Log warnings but proceed
    for _, issue := range report.Issues {
        if issue.Severity == validate.SeverityWarning {
            d.log.Warn("structural issue", "id", issue.ID, "path", issue.NodePath, "msg", issue.Message)
        }
    }

    return nil
}
```

**Key behavior**: Error-severity issues block startup. Warning-severity issues are logged but do not prevent the daemon from starting. The daemon's log message directs the user to run `wolfcastle doctor` for repair.

---

## 6. Report Format

The validation engine produces reports in two formats: human-readable (default for terminal output) and JSON (for programmatic consumption, CI, and `--json` flag).

### 6.1 Human-Readable Format

```
Wolfcastle Doctor — Structural Validation Report
=================================================

Engineer: wild-macbook
Checked:  2026-03-13T14:30:00Z
Duration: 45ms

Errors (3)
----------

  ERROR  ROOTINDEX_DANGLING_REF
         Node:  attunement-tree/fire-impl
         File:  .wolfcastle/projects/wild-macbook/state.json
         Root index references node, but no state.json exists on disk.
         Fix:   Remove dangling entry from root index. [deterministic]

  ERROR  PROPAGATION_MISMATCH
         Node:  attunement-tree
         File:  .wolfcastle/projects/wild-macbook/attunement-tree/state.json
         Orchestrator state is "complete" but child "water-impl" is "in_progress".
         Fix:   Recompute orchestrator state from children. [deterministic]

  ERROR  MISSING_AUDIT_TASK
         Node:  attunement-tree/water-impl
         File:  .wolfcastle/projects/wild-macbook/attunement-tree/water-impl/state.json
         Leaf node has no audit task.
         Fix:   Append default audit task. [deterministic]

Warnings (1)
------------

  WARN   ORPHAN_DEFINITION
         File:  .wolfcastle/projects/wild-macbook/attunement-tree/ice-impl.md
         Markdown definition file has no corresponding node in state.
         Fix:   Delete orphaned file, or register node. [deterministic]

Summary: 3 errors, 1 warning, 0 info
```

### 6.2 JSON Format

The JSON report is emitted when `wolfcastle doctor --json` is used. It follows the standard JSON envelope used by all Wolfcastle commands.

```json
{
  "ok": true,
  "action": "doctor",
  "report": {
    "timestamp": "2026-03-13T14:30:00Z",
    "engineer_namespace": "wild-macbook",
    "duration_ms": 45,
    "counts": {
      "error": 3,
      "warning": 1,
      "info": 0
    },
    "issues": [
      {
        "id": "ROOTINDEX_DANGLING_REF",
        "severity": "error",
        "message": "Root index references node, but no state.json exists on disk.",
        "node_path": "attunement-tree/fire-impl",
        "file_path": ".wolfcastle/projects/wild-macbook/state.json",
        "field": "root.children",
        "fix_strategy": "deterministic",
        "fix_description": "Remove dangling entry from root index."
      },
      {
        "id": "PROPAGATION_MISMATCH",
        "severity": "error",
        "message": "Orchestrator state is \"complete\" but child \"water-impl\" is \"in_progress\".",
        "node_path": "attunement-tree",
        "file_path": ".wolfcastle/projects/wild-macbook/attunement-tree/state.json",
        "field": "state",
        "fix_strategy": "deterministic",
        "fix_description": "Recompute orchestrator state from children."
      },
      {
        "id": "MISSING_AUDIT_TASK",
        "severity": "error",
        "message": "Leaf node has no audit task.",
        "node_path": "attunement-tree/water-impl",
        "file_path": ".wolfcastle/projects/wild-macbook/attunement-tree/water-impl/state.json",
        "field": "tasks",
        "fix_strategy": "deterministic",
        "fix_description": "Append default audit task."
      },
      {
        "id": "ORPHAN_DEFINITION",
        "severity": "warning",
        "message": "Markdown definition file has no corresponding node in state.",
        "node_path": "",
        "file_path": ".wolfcastle/projects/wild-macbook/attunement-tree/ice-impl.md",
        "field": "",
        "fix_strategy": "deterministic",
        "fix_description": "Delete orphaned file."
      }
    ]
  }
}
```

### 6.3 Report Stability

The JSON report schema is versioned. If the report structure changes in a future version, the version field will increment. Consumers can check the version before parsing.

Issue IDs are stable identifiers. Once an issue ID is introduced, it is never renamed or repurposed. Deprecated issue IDs are documented but continue to be recognized.

---

## 7. Fix Application

Fixes are applied atomically to prevent partial repairs that leave the tree in a worse state than before. The fix pipeline follows this sequence:

### 7.1 Fix Pipeline

```
1. Validate report          — Ensure the report is current (re-run checks on the specific files)
2. Group by file            — Collect all fixes that target the same state.json
3. Apply per-file atomically:
   a. Read the current file contents
   b. Re-validate that the issues still exist (stale report guard)
   c. Apply all fixes for this file in memory
   d. Validate the resulting state (run checks on the modified in-memory state)
   e. Write to a temporary file ({file}.doctor.tmp)
   f. Rename temporary file to target (atomic on POSIX)
4. Record each fix in the doctor log
```

### 7.2 Atomicity Guarantees

**Per-file atomicity**: All fixes targeting the same `state.json` are applied as a single write. Either all succeed or none are written. This prevents a scenario where fixing one issue introduces another (e.g., removing a dangling index entry but leaving the propagation state stale).

**Cross-file consistency**: Fixes that span multiple state files (e.g., re-registering an orphan requires updating both the parent's `children` array and the root index) are applied in dependency order:

1. Leaf state files first (most specific)
2. Parent orchestrator state files next
3. Root index last

If any write in the sequence fails, previously written files are rolled back from their pre-fix contents (stored in memory before the fix began).

### 7.3 Rollback

```go
type FixTransaction struct {
    // backups maps file paths to their original contents, captured before any fix is applied.
    backups map[string][]byte
}

// Begin captures the current contents of all files that will be modified.
func (tx *FixTransaction) Begin(filePaths []string) error

// Rollback restores all files to their pre-fix contents.
func (tx *FixTransaction) Rollback() error

// Commit discards the backups (fix is final).
func (tx *FixTransaction) Commit()
```

### 7.4 Post-Fix Validation

After all fixes are applied (and before committing the transaction), the runner re-executes the relevant checks against the modified state. If new issues are detected that were not present before the fix, the entire transaction is rolled back and the user is informed:

```
Fix application introduced new issues — rolling back all changes.
New issue: INVALID_TRANSITION_COMPLETE_WITH_INCOMPLETE at attunement-tree
Run 'wolfcastle doctor' again after investigating.
```

This prevents the fixer from making things worse.

### 7.5 Doctor Log

Every fix application is logged to `.wolfcastle/logs/doctor.jsonl` as NDJSON:

```json
{
  "timestamp": "2026-03-13T14:31:00Z",
  "issue_id": "ROOTINDEX_DANGLING_REF",
  "node_path": "attunement-tree/fire-impl",
  "file_path": ".wolfcastle/projects/wild-macbook/state.json",
  "fix_strategy": "deterministic",
  "fix_description": "Removed dangling entry from root index.",
  "applied": true,
  "error": null
}
```

---

## 8. The Doctor Prompt

When the validation engine encounters issues that require model reasoning (fix strategy: model-assisted), it invokes the configured doctor model with a structured prompt. The prompt provides the model with precise context and constraints.

### 8.1 Configuration

From `config.json` (or `config.local.json` override):

```json
{
  "doctor": {
    "model": "mid",
    "prompt_file": "doctor.md"
  }
}
```

The model key references the `models` dictionary. The prompt file is resolved through the standard three-tier merge (base/custom/local).

### 8.2 Prompt Structure

The doctor prompt is assembled by Go code and sent to the model as a single invocation. It contains:

```markdown
# Wolfcastle Doctor — Structural Repair

You are assisting with structural repair of a Wolfcastle project tree.
You will be given one or more structural issues that require judgment to resolve.
For each issue, you must propose a specific fix as a JSON patch.

## Rules

1. You may only modify the specific fields identified in each issue.
2. You may not create new nodes, tasks, or files.
3. You may not delete nodes that contain completed work (breadcrumbs exist).
4. Your proposed fix must result in a valid state per the Wolfcastle state machine.
5. Valid states are: not_started, in_progress, complete, blocked.
6. An orchestrator's state must match the recompute_parent() result over its children.
7. A leaf marked complete must have all tasks complete.
8. The audit task must remain last in every leaf's task list.
9. Respond with ONLY the JSON fix document. No explanation, no markdown fences.

## Issue {N}

**ID**: {issue_id}
**Severity**: {severity}
**Description**: {message}
**Node**: {node_path}
**File**: {file_path}
**Field**: {field}

### Current State

{JSON contents of the relevant portion of the state file, pretty-printed}

### Context

{Additional context depending on issue type — see Section 8.3}

## Fix Format

Respond with a JSON array of fix operations:

```json
[
  {
    "issue_id": "ISSUE_ID",
    "file_path": "path/to/state.json",
    "operations": [
      {
        "op": "set",
        "path": "$.state",
        "value": "in_progress"
      }
    ]
  }
]
```

Supported operations:
- `set`: Set a field to a value. Path is a JSON path.
- `delete`: Remove a field or array element. Path is a JSON path.
- `append`: Append a value to an array. Path is a JSON path to the array.
- `move`: Move an array element. Path is source, value is destination index.
```

### 8.3 Context Injection Per Issue Type

The model receives different context depending on the issue type:

| Issue ID | Context provided |
|----------|-----------------|
| `ORPHAN_STATE` | The orphan's full state, the potential parent's `children` array, sibling nodes' names and states. |
| `MULTIPLE_AUDIT_TASKS` | All audit tasks in the node with their descriptions, states, and any associated breadcrumbs. |
| `INVALID_STATE_VALUE` | The invalid value, the node's children/task states, recent breadcrumbs. |
| `INVALID_TRANSITION_COMPLETE_WITH_INCOMPLETE` | The node's state, all children/task states, audit results, recent breadcrumbs. |
| `MULTIPLE_IN_PROGRESS` | All `in_progress` tasks with their node paths, timestamps of last breadcrumb (if any), and failure counts. |
| `MISSING_REQUIRED_FIELD` (semantic fields) | The node's directory name, definition file contents (if exists), sibling node names. |

### 8.4 Guardrails

The model's proposed fix is validated by Go code before application. The following constraints are enforced:

1. **Schema validation**: Every proposed state change must result in a valid `state.json` per the schema.
2. **Scope restriction**: The model may only modify the specific files and fields identified in the issue. Modifications outside the issue scope are rejected.
3. **No creation**: The model cannot create new nodes, tasks, or files through the fix. Fixes repair existing state; they do not extend the tree.
4. **No deletion of work**: The model cannot delete nodes that have breadcrumbs or completed tasks. Completed work is preserved.
5. **Invariant preservation**: After applying the proposed fix, the validation engine re-checks the affected nodes. If the fix introduces new issues, it is rejected.
6. **Single invocation**: The model gets one shot per issue batch. If the proposed fix is rejected, the issue is reported as "unfixable by model — manual intervention required" and the doctor moves on.

### 8.5 Deterministic Fast Paths

Before invoking the model, the engine checks for common typo corrections that can be resolved deterministically:

| Invalid value | Deterministic correction |
|---------------|------------------------|
| `"completed"` | `"complete"` |
| `"pending"` | `"not_started"` |
| `"in-progress"` | `"in_progress"` |
| `"started"` | `"in_progress"` |
| `"not-started"` | `"not_started"` |

If the invalid value matches a known typo, the fix is applied deterministically without model invocation. This saves tokens for the most common case of `INVALID_STATE_VALUE`.

---

## 9. wolfcastle doctor Command Integration

This section describes how the validation engine integrates with the `wolfcastle doctor` CLI command.

### 9.1 Command Flow

```
wolfcastle doctor [--fix] [--fix-all] [--json] [--check <id>]
```

1. **Load state tree** — Read the engineer namespace, parse all state files.
2. **Run validation** — Execute `AllChecks()` runner (or a specific check if `--check` is provided).
3. **Display report** — Render in human-readable or JSON format.
4. **If `--fix` is specified**:
   a. Display issues with proposed fixes.
   b. Prompt the user: "Fix all deterministic issues? [y/N]"
   c. If confirmed, apply deterministic fixes.
   d. If model-assisted issues exist, prompt: "Invoke model for ambiguous issues? [y/N]"
   e. If confirmed, invoke the doctor model and apply validated fixes.
5. **If `--fix-all` is specified**:
   a. Apply all deterministic fixes without prompting.
   b. Invoke the model for all model-assisted fixes without prompting.
   c. Report results.

### 9.2 Exit Codes

| Code | Meaning |
|------|---------|
| 0 | No issues found, or all issues fixed successfully. |
| 1 | Issues found but not fixed (report-only mode, or `--fix` declined). |
| 2 | Fix attempted but some fixes failed or were rejected. |
| 3 | Operational error (cannot read state, cannot invoke model, etc.). |

### 9.3 Filtering

`wolfcastle doctor --check root_index` runs only the `root_index` check. Multiple `--check` flags can be provided to compose a custom suite. This is useful for CI pipelines that want to verify specific invariants.

`wolfcastle doctor --severity error` reports only error-severity issues. Accepted values: `error`, `warning`, `info`.

---

## 10. Invariants Verified

For reference, this is the complete list of invariants from the state machine spec (Section 13) and how the validation engine checks each one:

| # | Invariant | Check ID(s) |
|---|-----------|-------------|
| 1 | Single In Progress | `in_progress` |
| 2 | Audit Last | `audit_task` |
| 3 | Audit Immovable | `audit_task` |
| 4 | State Consistency (orchestrator derived from children) | `state_propagation` |
| 5 | Depth Monotonicity | `failure_counters` |
| 6 | Failure Counter Non-Negative | `failure_counters` |
| 7 | Blocked Requires Reason | `state_transitions` |
| 8 | Complete Is Terminal | `state_transitions` |
| 9 | Breadcrumbs Append-Only | Not checked (append-only is enforced by the script layer, not detectable after the fact) |
| 10 | Valid State Values | `state_values` |

Additionally, the validation engine checks structural integrity issues (root index consistency, orphans, malformed JSON, required fields) that are not captured by the state machine invariants but are necessary for correct operation of the distributed state system introduced in ADR-024.

---

## References

- ADR-002: JSON for Configuration and State
- ADR-003: Deterministic Scripts with Static Documentation
- ADR-007: Audit Model Preserved, Mechanics via Scripts
- ADR-008: Tree-Addressed Operations
- ADR-009: Distribution, Project Layout, and Three-Tier File Layering
- ADR-014: Serial Execution with Node Scoping
- ADR-019: Failure Handling, Decomposition, and Retry Thresholds
- ADR-020: Daemon Lifecycle and Process Management
- ADR-024: Distributed State Files, Task Working Documents, and Runtime Aggregation
- ADR-025: Wolfcastle Doctor — Structural Validation and Repair
- Spec: State Machine for Nodes and Tasks (state machine spec)
- Spec: Tree Addressing Scheme
- Spec: Config Schema
- Spec: CLI Commands
