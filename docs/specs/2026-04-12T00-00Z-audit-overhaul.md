# Audit System Overhaul: Exploratory Review + Knowledge Pipeline

## Status
Accepted

## Problem

The daemon-driven audit verifies compliance: did each task deliver what it promised? It checks deliverables against acceptance criteria, runs the test suite, runs the linter. If the contract is met, the audit passes.

What it cannot do is find issues nobody thought to check for. A task that introduces stuttering type names, dead code, stale documentation, missing error handling, or architectural drift will pass the audit as long as its acceptance criteria are met. The audit verifies the plan. It doesn't question the plan.

A human-supervised audit session (or the `/audit` skill) asks a fundamentally different question: "what's wrong with this code?" without reference to any task's deliverables. It finds quality issues that no task was assigned to prevent. The daemon audit has no equivalent capability.

The knowledge system compounds this gap. Audit findings from manual sessions don't flow back into the automated pipeline. A human finds "this codebase uses typed constants, not raw strings" and fixes it, but nothing prevents the next agent from reintroducing raw strings because the daemon audit doesn't know to check.

## Design

### Exploratory review at the orchestrator level

The orchestrator audit (`plan-review.md`) already runs when all children complete. It already uses the heavy model. It already has the wide view: all children, all deliverables, all scope. It is the natural place for an exploratory quality review.

Add an exploratory phase to plan-review.md between the existing verification (phase C) and the decision (phase D). The new phase asks: "what quality issues exist in the code these children produced, beyond what the acceptance criteria cover?"

The exploratory review is unconstrained by task deliverables. It reads all files in scope, not just deliverables. It looks for:

- Naming inconsistencies (stuttering, casing violations, convention drift)
- Dead code (unused functions, unreferenced files, stale imports)
- Missing or inaccurate documentation
- Error handling gaps (unchecked errors, missing context wrapping)
- Architectural violations (circular dependencies, wrong-layer access, leaked abstractions)
- Test gaps (uncovered code paths, tests that test implementation rather than behavior)
- Security issues (injection vectors, hardcoded secrets, unsafe deserialization)
- Performance concerns (unbounded allocations, missing context cancellation, goroutine leaks)

The review is not a checklist. The model uses its judgment to identify what matters in the specific codebase it's looking at. The knowledge system provides accumulated context (see below), but the review is free to find issues the knowledge system hasn't seen before.

### Remediation through new leaves

When the exploratory review finds issues, the orchestrator creates a new leaf under itself with tasks scoped to the findings. Each task targets specific files and issues from the review, with a task class appropriate to the work (e.g., `coding/go` for Go fixes, `refactor` for structural changes).

The orchestrator emits WOLFCASTLE_CONTINUE instead of WOLFCASTLE_COMPLETE, signaling that new work exists. The daemon processes the remediation leaf: its tasks execute, its leaf audit runs (compliance check on the fixes), and the orchestrator re-enters plan-review when the leaf completes.

The second orchestrator review sees:
- The original children (unchanged, already complete)
- The remediation leaf (newly complete)
- The previous review's breadcrumbs and findings

It verifies the remediation addressed the findings and runs another exploratory pass. If clean, it emits WOLFCASTLE_COMPLETE. If not, it creates another remediation leaf. The loop converges because each pass has less to find.

A depth limit prevents infinite loops. The orchestrator tracks how many remediation passes it has created via a counter in its audit state. After `max_review_passes` (default 3), the orchestrator completes regardless, logging any remaining findings as informational breadcrumbs rather than creating more work.

### Knowledge pipeline: findings become persistent checks

When the exploratory review finds a pattern violation (e.g., "raw string literals used for state values instead of typed constants"), the finding should persist as a knowledge entry so future audits check for it automatically.

The flow:

1. Orchestrator exploratory review finds "3 files use raw string `"not_started"` instead of `state.StatusNotStarted`"
2. Orchestrator creates remediation task to fix the instances
3. Orchestrator also writes a knowledge entry: `wolfcastle knowledge add "State status values must use typed constants from internal/state/types.go, never raw string literals. Found and fixed in v0.6.0 audit."`
4. Future execution tasks see this knowledge entry in their context and avoid the pattern
5. Future leaf audits see this knowledge entry (already implemented, #182) and verify compliance
6. Future orchestrator reviews see this knowledge entry and can verify codebase-wide compliance

The knowledge system becomes the project's accumulating immune system: each infection the audit fights teaches the system to recognize the next one.

### Changes to plan-review.md

Add phase C.5 between existing phases C and D:

```markdown
### C.5 Exploratory Quality Review

Step back from the acceptance criteria and deliverables. Read the actual code 
these children produced. Ask: what quality issues exist that nobody checked for?

Review all files in scope, not just deliverables. Look for problems that 
acceptance criteria wouldn't catch: naming violations, dead code, missing error 
handling, test gaps, documentation drift, architectural issues, security 
concerns. Use your judgment. The knowledge entries below describe patterns this 
project has encountered before; verify those, but don't limit yourself to them.

For each finding:
1. Record it as a breadcrumb with location and description
2. Assess whether it warrants a remediation task or is informational

If findings warrant remediation:
1. Create a new leaf: `wolfcastle project create --node <your-node>/<remediation-slug>`
2. Add tasks targeting the specific files and issues
3. Write a knowledge entry for any pattern-level finding that future work should 
   avoid: `wolfcastle knowledge add "<description of the pattern to avoid>"`
4. Emit WOLFCASTLE_CONTINUE

If findings are informational only (cosmetic, low-impact, or would require 
disproportionate effort to fix), record them as breadcrumbs and proceed to 
phase D.
```

### Changes to execute.md (leaf audit)

No changes to the leaf audit procedure. It remains a compliance check. The exploratory review happens at the orchestrator level where the model has the full feature context.

Knowledge entries are already injected into audit context (#182). As the knowledge system grows from orchestrator findings, leaf audits automatically gain new verification criteria without prompt changes.

### Configuration

```json
{
  "planning": {
    "max_review_passes": 3
  }
}
```

Default 3. Set to 1 for a single exploratory pass with no remediation loop. Set to 0 to disable exploratory review entirely (existing behavior).

### Review pass tracking

The orchestrator's `NodeState` gains a `review_pass` integer field (default 0). Each time plan-review creates remediation work and emits WOLFCASTLE_CONTINUE, the daemon increments `review_pass`. The prompt template includes the current pass number so the model knows where it is in the loop. At `max_review_passes`, the prompt instructs the model to log remaining findings as breadcrumbs and emit WOLFCASTLE_COMPLETE.

### Orchestrator review visibility

The TUI has no indication that a planning pass (including the review) is running. Leaf tasks show `→` in the tree and update the dashboard's "Current target" field. Orchestrator planning passes are invisible: the model runs, the orchestrator sits there, and the user sees nothing happening.

The daemon already writes `current_node` to the activity file during execution. During planning passes, it should do the same: write the orchestrator's address and the planning trigger ("completion_review", "initial", "remediate") to the activity file. The TUI already reads this file and displays the current target.

Additionally, the daemon should log a structured `planning_start` event that the TUI's log stream can display, so the user sees "reviewing warzone/backend (completion_review)" in the activity feed.

Changes:
1. In `runPlanningPass`, write `current_node` and `current_task` (set to the planning trigger, e.g., "completion_review") to the daemon activity file before invoking the model. Clear them after.
2. The TUI dashboard already reads the activity file and displays "Current: warzone/backend/auth/task-1". During planning, it would show "Current: warzone/backend (completion_review)".
3. The header status remains "hunting" (unchanged).

## What this does NOT change

- **Leaf audit procedure**: unchanged. Compliance check against deliverables.
- **Model tiers**: planning already uses heavy. No new stages or models.
- **Daemon loop**: the orchestrator already emits WOLFCASTLE_CONTINUE when it creates work. The daemon already re-enters plan-review when the new work completes. No loop changes needed.
- **State machine**: creating a new leaf under an orchestrator is already supported. The orchestrator transitions back to in_progress when it has incomplete children. No state changes needed.

## Implementation sequence

1. Add `review_pass` field to `NodeState` and `max_review_passes` to `PlanningConfig`
2. Update plan-review.md with phase C.5 (exploratory review)
3. Update the daemon's planning pass to increment `review_pass` on WOLFCASTLE_CONTINUE and inject the counter into prompt context
4. Update the daemon's planning pass to switch to a "final review" prompt variant when `review_pass >= max_review_passes`
5. Write `current_node` and planning trigger to the daemon activity file during `runPlanningPass` so the TUI shows orchestrator review activity
6. Test with a project that has known quality issues and verify the loop finds, remediates, and converges

## Verification

1. Create a project with intentional quality issues (raw string literals, dead code, stuttering names, missing error handling)
2. Run the daemon and verify the orchestrator review finds the issues, creates remediation leaves, and the fixes land
3. Verify the knowledge entries are created and appear in future audit contexts
4. Verify the loop terminates at `max_review_passes`
5. Verify that a clean project completes in one pass with no remediation
