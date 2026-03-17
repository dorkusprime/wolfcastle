# Remediation Plan

Work branch: `fix/remediation` (based on `main` at `21ca134`)
Worktree: `/Users/wild/repository/dorkusprime/wolfcastle/fix/remediation`
Remediation specs: `~/Desktop/remediation/failure-{1..6}-*.md`
Run report: `docs/domain-refactor-run-report.md` (on main)
Backlog: `docs/backlog.md` (on main)

## Context for a new session

This plan remediates 6 failures discovered during Wolfcastle's autonomous implementation of the Domain Repository Architecture spec on the `refactor/domains` branch. The failures are daemon infrastructure bugs that affect all Wolfcastle runs, not just the refactor. Fixes go on `fix/remediation` (off `main`). The `refactor/domains` branch is a test case only; its commit SHAs are used for regression verification via throwaway worktrees.

Key files to read before starting:
- This file (`REMEDIATION-PLAN.md` in the worktree root)
- `~/Desktop/remediation/failure-{1..6}-*.md` (detailed per-failure plans with exact SHAs and test steps)
- `internal/daemon/iteration.go` (marker scanning, progress check, YIELD handler)
- `internal/daemon/deliverables.go` (git progress check)
- `internal/invoke/invoker.go` (marker constants, detection)
- `internal/project/templates/prompts/execute.md` (model instructions)
- `docs/backlog.md` (on main, for actionable items beyond the 6 failures)

## Worktree layout

| Worktree | Branch | Purpose |
|----------|--------|---------|
| `wolfcastle/fix/remediation` | `fix/remediation` | Active fixes. All commits go here. |
| `wolfcastle/refactor/domains` | `refactor/domains` | Test case. Read-only reference for SHAs. |
| Throwaway `/tmp/regression-fN` | detached HEAD | Created from `refactor/domains` SHAs for regression replay. Deleted after each test. |

## Task list

### Phase 1: Code fixes (sequential, each depends on the previous)

| # | Task | Files | Depends on | Remediation spec |
|---|------|-------|------------|-----------------|
| 1.1 | Strip markdown from markers | `internal/daemon/iteration.go` | — | `failure-3-markdown-marker.md` |
| 1.2 | Add WOLFCASTLE_SKIP marker | `internal/invoke/invoker.go`, `internal/daemon/iteration.go` | 1.1 (scanner changes) | `failure-5-work-already-done.md` |
| 1.3 | Skip progress check for IsAudit | `internal/daemon/iteration.go`, `internal/daemon/deliverables.go` | 1.2 (marker dispatch) | `failure-1-audit-progress-check.md` |
| 1.4 | Downgrade deliverable missing to warning | `internal/daemon/iteration.go`, `cmd/task/deliverable.go` | 1.3 | `failure-4-deliverable-path.md` |
| 1.5 | YIELD blocks parent when subtasks created | `internal/daemon/iteration.go` | 1.2 (SKIP needed for parent completion) | `failure-6-yield-self-decomposition.md` |
| 1.6 | Auto-complete decomposed parents | `internal/daemon/iteration.go` or `internal/state/mutations.go` | 1.5 | `failure-6-yield-self-decomposition.md` |

### Phase 2: Prompt changes

| # | Task | Files | Depends on |
|---|------|-------|------------|
| 2.1 | Add "no formatting" to marker instructions | `internal/project/templates/prompts/execute.md` | 1.1 |
| 2.2 | Add WOLFCASTLE_SKIP to marker documentation | `internal/project/templates/prompts/execute.md` | 1.2 |
| 2.3 | Add "do not move packages" constraint | `internal/project/templates/prompts/execute.md` | — |
| 2.4 | Add "list files, decompose if >8" trigger | `internal/project/templates/prompts/execute.md` | — |
| 2.5 | Add YIELD + decompose documentation | `internal/project/templates/prompts/execute.md` | 1.5 |

### Phase 3: Backlog code fixes (independent of Phase 1/2)

These are directly actionable code items from the backlog that don't require specs.

| # | Task | Files | Backlog item |
|---|------|-------|-------------|
| 3.1 | Auto-commit partial work on failure | `internal/daemon/iteration.go` | "Model scope creep across task boundaries" |
| 3.2 | Failure context in retry prompt | `internal/pipeline/context.go` | "Failure context in iteration prompt" |
| 3.3 | Validate deliverable paths at declaration | `cmd/task/deliverable.go` | "task deliverable doesn't validate paths" |

### Phase 4: Unit tests for all fixes

Write tests AFTER each fix is proven with 3 manual passes.

| # | Test | Package | Covers |
|---|------|---------|--------|
| 4.1 | TestScanTerminalMarker markdown variants | `internal/daemon` | 1.1 |
| 4.2 | TestScanTerminalMarker SKIP variants | `internal/daemon` | 1.2 |
| 4.3 | TestRunIteration_SkipBypassesProgressCheck | `internal/daemon` | 1.2 |
| 4.4 | TestRunIteration_AuditSkipsProgressCheck | `internal/daemon` | 1.3 (already exists, verify) |
| 4.5 | TestRunIteration_MissingDeliverables_WarnsButCompletes | `internal/daemon` | 1.4 |
| 4.6 | TestRunIteration_YieldWithSubtasks_BlocksParent | `internal/daemon` | 1.5 |
| 4.7 | TestRunIteration_YieldWithoutSubtasks_StaysInProgress | `internal/daemon` | 1.5 |
| 4.8 | TestAutoCompleteDecomposedParent | `internal/daemon` | 1.6 |
| 4.9 | TestPartialWorkCommittedOnFailure | `internal/daemon` | 3.1 |
| 4.10 | TestFailureContextInRetryPrompt | `internal/pipeline` | 3.2 |
| 4.11 | TestDeliverablePathWarning | `cmd/task` | 3.3 |

### Phase 5: Remediation verification (3 passes each)

Run each failure's test steps from `~/Desktop/remediation/failure-N-*.md`. Each must pass 3 consecutive times. Use throwaway worktrees from `refactor/domains` SHAs for regression replay.

| # | Failure | Regression SHA | Method |
|---|---------|---------------|--------|
| 5.1 | Audit progress check | `ef3095b` | Unit test + live daemon |
| 5.2 | Package move | `2283d18` | Prompt verification + supplementary live test |
| 5.3 | Markdown marker | `e5df644` | Unit test (primary) + live daemon (supplementary) |
| 5.4 | Deliverable path | `2283d18` | Unit test + live daemon with pre-declared wrong deliverable |
| 5.5 | Work already done | `a055e49` | Unit test (primary) + live daemon (supplementary) |
| 5.6 | YIELD decomposition | `7b6e463` | Unit test + mock model script |

### Phase 6: Final regression suite

```bash
cd /Users/wild/repository/dorkusprime/wolfcastle/fix/remediation
gofmt -l .                           # formatting
go vet ./...                         # vet
go build ./...                       # build
go test -race -count=1 ./...         # unit + package tests
go test -race -tags integration -count=1 ./test/integration/  # integration
go test -race -tags smoke -count=1 ./test/smoke/              # smoke
```

All must pass. If regressions appear, fix them before proceeding.

### Phase 7: Commit and push

Once all phases pass:
```bash
cd /Users/wild/repository/dorkusprime/wolfcastle/fix/remediation
git push -u origin fix/remediation
# Then create PR from wolfcastle/main:
# gh pr create --title "..." --body "..."
```

## Execution order

```
1.1 → 1.2 → 1.3 → 1.4 → 1.5 → 1.6  (code fixes, sequential)
        ↓
2.1 → 2.2 → 2.3 → 2.4 → 2.5         (prompt changes, after relevant code fix)
        ↓
3.1, 3.2, 3.3                         (backlog fixes, independent)
        ↓
4.1 → 4.11                            (unit tests for all fixes)
        ↓
5.1 → 5.6                             (remediation verification, 3 passes each)
        ↓
6                                      (final regression suite)
        ↓
7                                      (commit and push)
```

Phases 1-3 can overlap with Phase 4 (write tests as fixes are applied). Phase 5 must wait until all fixes are in. Phase 6 must be the last thing before Phase 7.

## Commit strategy

One commit per logical fix. Do not squash. The commit history should tell the story:

1. `Strip markdown formatting from terminal marker scanner`
2. `Add WOLFCASTLE_SKIP terminal marker`
3. `Skip git progress check for audit tasks`
4. `Downgrade missing deliverables to warning`
5. `Block parent task on YIELD with subtask creation`
6. `Auto-complete decomposed parents when subtasks finish`
7. `Prompt: no formatting on markers, SKIP docs, decomposition trigger, package constraint`
8. `Auto-commit partial work on task failure`
9. `Include failure context in retry iteration prompt`
10. `Validate and suggest corrections for deliverable paths`
11. `Tests for all remediation fixes`
