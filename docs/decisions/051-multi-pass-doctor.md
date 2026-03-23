# ADR-051: Multi-Pass Doctor with Fix Verification

## Status
Accepted

## Date
2026-03-14

## Context
The current `ApplyDeterministicFixes` runs a single pass: validate, fix, done. But some fixes create new issues: a cascading effect that a single pass cannot resolve. For example, relinking an orphaned node may create a propagation mismatch that requires recomputation. Or adding a missing audit task may trigger an audit-status-mismatch that needs syncing. A single pass misses these cascading effects, forcing users to run `doctor --fix` multiple times to reach a clean state.

A multi-pass approach: running validation and fixing in a loop until the tree stabilizes: handles these cascading repairs automatically while a pass cap prevents runaway loops.

## Decision

Implement a multi-pass fix loop in the validation engine:

### Loop Structure

```go
const maxFixPasses = 5

func (e *Engine) FixWithVerification(idx *state.RootIndex, categories []Category) (*Report, error) {
    var lastReport *Report
    for pass := 0; pass < maxFixPasses; pass++ {
        // Re-read state from disk on each pass (fixes from prior pass are persisted)
        idx, err = e.reloadIndex()
        if err != nil { return nil, err }

        report := e.Validate(idx, categories)
        if !report.HasAutoFixable() {
            return report, nil // Nothing to fix: done
        }

        fixes := e.ApplyDeterministicFixes(idx, report)
        lastReport = report
        if len(fixes) == 0 {
            break // Applied no fixes: further passes won't help
        }
    }

    // Final validation-only pass: report residual issues
    idx, _ = e.reloadIndex()
    finalReport := e.Validate(idx, categories)
    return finalReport, nil
}
```

### Early Exit

If a pass finds no fixable issues or applies no fixes, the loop exits immediately. This prevents unnecessary disk I/O for already-clean trees.

### Cascading Fix Examples

| Pass 1 Fix | Cascading Issue | Pass 2 Fix |
|-----------|----------------|-----------|
| Add missing audit task | Audit status doesn't match task state | Sync audit lifecycle |
| Relink orphaned node | Parent's child-ref state is stale | Propagation recomputation |
| Remove dangling index entry | Root state no longer matches children | Root state recomputation |
| Fix depth mismatch | Child depth no longer tracks parent | Depth recalculation |

### Reporting

Each pass appends its fixes to a cumulative list. The final report includes:
- All issues found (including those fixed in prior passes)
- All fixes applied (with pass number)
- Residual issues from the final validation-only pass

## Consequences
- Cascading fixes are handled automatically: no manual re-run needed
- The 5-pass cap prevents infinite loops from fix cycles
- Early exit keeps performance comparable to single-pass for clean trees
- The final validation-only pass provides confidence that fixes were effective
- Existing single-pass callers (`doctor` without `--fix`) are unaffected
