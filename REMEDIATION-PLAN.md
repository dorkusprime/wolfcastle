# Remediation Plan

Work branch: `fix/remediation` (based on `main` at `21ca134`)
Worktree: `/Users/wild/repository/dorkusprime/wolfcastle/fix/remediation`
Remediation specs: `~/Desktop/remediation/failure-{1..7}-*.md`
Run report: `/Users/wild/repository/dorkusprime/wolfcastle/main/docs/domain-refactor-run-report.md`
Backlog: `/Users/wild/repository/dorkusprime/wolfcastle/main/docs/backlog.md`
Domain repo spec (reference only): `/Users/wild/repository/dorkusprime/wolfcastle/main/docs/specs/2026-03-16T00-00Z-domain-repository-architecture.md`
Spec audits (reference only): `/Users/wild/repository/dorkusprime/wolfcastle/main/fs-spec-audit.md` and `fs-spec-audit-v2.md`

## Context for a new session

### What is Wolfcastle?

Wolfcastle is a Go CLI project orchestrator. You give it a goal, it decomposes it into a tree of tasks, and sends AI coding agents (Claude Code by default) to implement them one by one. The daemon runs a pipeline: intake (decompose inbox items into projects/tasks) and execute (claim a task, invoke a model, verify the result). The model communicates completion via terminal markers on stdout: `WOLFCASTLE_COMPLETE`, `WOLFCASTLE_BLOCKED`, `WOLFCASTLE_YIELD`. The daemon checks that real work was done (git progress), deliverables exist, and then advances to the next task. Each leaf in the project tree ends with an audit task that reviews the work.

### What happened?

Wolfcastle was used to implement its own Domain Repository Architecture (a 13-step refactoring spec). The run completed but revealed 7 systemic failures in the daemon infrastructure that affect all Wolfcastle runs. These failures are documented in `~/Desktop/remediation/failure-{1..7}-*.md` with exact commit SHAs for regression replay.

### What does this plan do?

Apply code fixes and prompt changes to the wolfcastle daemon on the `fix/remediation` branch (based on `main`). Then verify each fix passes 3 times. Then run the full test suite to catch regressions. Then push and PR.

### Key architecture details the executor needs to know

- **Terminal markers**: Model emits `WOLFCASTLE_COMPLETE`, `WOLFCASTLE_BLOCKED`, or `WOLFCASTLE_YIELD` on stdout. The daemon scans output via `scanTerminalMarker` in `internal/daemon/iteration.go`. The invoke package also detects markers during streaming via `detectLineMarker` in `internal/invoke/invoker.go`.
- **Git progress check**: After COMPLETE, the daemon verifies work was done via `checkGitProgress` in `internal/daemon/deliverables.go`. Checks HEAD moved OR uncommitted changes outside `.wolfcastle/system/`.
- **Deliverable check**: Before progress check, the daemon verifies declared deliverable files exist via `checkDeliverables` in `internal/daemon/deliverables.go`.
- **IsAudit**: Tasks with `IsAudit: true` are audit tasks that review work rather than writing code.
- **YIELD**: Leaves the task `in_progress` for resumption. Used for crash recovery and decomposition.
- **State files**: Per-node JSON in `.wolfcastle/system/projects/<namespace>/<address>/state.json`. Mutated via `StateStore.MutateNode`.
- **Prompts**: Embedded Go templates extracted to `.wolfcastle/system/base/prompts/` at scaffold time. Execute prompt is `execute.md`, audit prompt is in `audits/audit-task.md`.

### Prerequisites

- Go 1.26+ installed
- Claude Code CLI (`claude`) installed and configured with API key (for live daemon tests)
- Git configured globally (or the test steps use `-c user.name=test -c user.email=test@test.com`)

## Worktree layout

| Worktree | Branch | Purpose |
|----------|--------|---------|
| `wolfcastle/fix/remediation` | `fix/remediation` | Active fixes. All commits go here. |
| `wolfcastle/main` | `main` | Reference. Read backlog and run report from here. Do NOT modify. |
| `wolfcastle/refactor/domains` | `refactor/domains` | Test case. Read-only reference for regression SHAs. |
| `/tmp/regression-fN` | detached HEAD | Throwaway worktrees from `refactor/domains` SHAs. Created for regression replay, deleted after. |

## Task list

### Phase 1: Code fixes

All code fixes touch `internal/daemon/iteration.go` as the primary file. Apply them sequentially to avoid merge conflicts. Run `go build ./...` and `go test ./internal/daemon/` after EACH fix to catch breakage immediately.

| # | Task | Files | Depends on | Spec |
|---|------|-------|------------|------|
| 1.1 | Strip markdown from markers | `internal/daemon/iteration.go` | — | `failure-3` |
| 1.2 | Add WOLFCASTLE_SKIP marker | `internal/invoke/invoker.go`, `internal/daemon/iteration.go` | 1.1 | `failure-5` |
| 1.3 | Skip progress check for IsAudit | `internal/daemon/iteration.go`, `internal/daemon/deliverables.go` | 1.2 | `failure-1` |
| 1.4 | Downgrade deliverable missing to warning | `internal/daemon/iteration.go` | 1.3 | `failure-4` |
| 1.5 | YIELD blocks parent when subtasks created | `internal/daemon/iteration.go` | 1.4 | `failure-6` |
| 1.6 | Auto-complete decomposed parents | `internal/daemon/iteration.go` | 1.5 | `failure-6` |
| 1.7 | Auto-commit partial work on failure | `internal/daemon/iteration.go` | 1.6 | backlog |
| 1.8 | Failure context in retry prompt | `internal/daemon/iteration.go`, `internal/pipeline/context.go` | 1.7 | backlog |
| 1.9 | Validate deliverable paths at declaration | `cmd/task/deliverable.go` | — (independent file) | backlog |

**Why Phase 3 items moved here:** Tasks 1.7-1.8 modify `iteration.go`, the same file as 1.1-1.6. Applying them in a separate phase creates unnecessary merge risk. Task 1.9 is an independent file and can be done at any point.

**After EACH task:**
```bash
cd /Users/wild/repository/dorkusprime/wolfcastle/fix/remediation
go build ./...
go test -count=1 ./internal/daemon/  # or ./cmd/task/ for 1.9
# If either fails, fix before proceeding

# Then run the live daemon test from the failure spec 3 times:
make build  # build the binary
# Follow "Verify with live daemon" steps in ~/Desktop/remediation/failure-N-*.md
# Must pass 3 consecutive times before committing

git add -A && git commit -m "<commit message>"
```

**Live test reminders:**
- Use temp directories (`/tmp/test-N-*`), not the worktree
- `git -c user.name=test -c user.email=test@test.com` for all git commands in temp dirs
- Create `.wolfcastle/system/local/` before writing config
- `max_iterations` 5+ for single-task leaves (includes audit)
- F6 uses a mock model script (see `failure-6-yield-self-decomposition.md`)
- F7 success is tiered: model creates ADR proactively OR audit flags missing ADR as REMEDIATE

#### 1.8 detail: Failure context in retry prompt

The backlog says "daemon reads the last log entry for the task and injects it into the iteration context." The implementation:

1. In `iteration.go`, before invoking the model, check if the task has `FailureCount > 0`.
2. If so, read the last log file for this task (the daemon knows the log directory and the current iteration number; the previous iteration is `iteration - 1`).
3. Extract the failure type from the log (`no_terminal_marker`, `no_progress`, `deliverable_warning`, etc.).
4. Pass this as a string to `pipeline.BuildIterationContext` (or `ContextBuilder.Build`), which includes it in the context under a "Previous attempt failed" header.
5. The pipeline/context.go change: accept an optional `failureReason string` parameter and include it in the output when non-empty.

This is a daemon + pipeline change, not just pipeline.

### Phase 2: Prompt and audit changes

All prompt changes can be done in a single commit since they're all text edits to template files. Apply after Phase 1 is complete.

| # | Task | File |
|---|------|------|
| 2.1 | Marker: "emit as plain text, no formatting" | `execute.md` |
| 2.2 | Marker: document WOLFCASTLE_SKIP and when to use it | `execute.md` |
| 2.3 | Constraint: "do not move, rename, or delete packages" | `execute.md` |
| 2.4 | Decomposition: "list files first, decompose if >8" | `execute.md` |
| 2.5 | Decomposition: "after creating subtasks, emit YIELD" | `execute.md` |
| 2.6 | ADR: pre-completion verification step | `execute.md` |
| 2.7 | ADR/spec: mandatory REMEDIATE on absence in audit | `audit-task.md` |
| 2.8 | Spec: create spec for new interfaces | `execute.md` |

Full file paths:
- `internal/project/templates/prompts/execute.md`
- `internal/project/templates/audits/audit-task.md`

**After all prompt changes:**
```bash
go build ./...  # prompts are embedded, verify build
git add -A && git commit -m "Prompt and audit enforcement changes"
```

### Phase 3: Regression verification (regression tests from `refactor/domains`)

Verify the fixes don't break the scenarios that originally worked. Create throwaway worktrees from `refactor/domains` SHAs.

```bash
# Verify the old audit-progress fix still works
git worktree add /tmp/regression-audit ef3095b
cd /tmp/regression-audit && make build
# ... run the regression test from failure-1 spec ...
cd /Users/wild/repository/dorkusprime/wolfcastle/fix/remediation
git worktree remove /tmp/regression-audit

# Repeat for other regression SHAs as needed
```

### Phase 4: Unit tests for all fixes

Write tests that codify what the live tests proved. These are permanent regression tests.

| # | Test | Package | Covers |
|---|------|---------|--------|
| 4.1 | TestScanTerminalMarker: markdown bold, italic, backtick, underscore, mixed | `internal/daemon` | 1.1 |
| 4.2 | TestScanTerminalMarker: SKIP standalone, with reason, in JSON, priority | `internal/daemon` | 1.2 |
| 4.3 | TestRunIteration_SkipBypassesProgressCheck | `internal/daemon` | 1.2 |
| 4.4 | TestRunIteration_AuditSkipsProgressCheck | `internal/daemon` | 1.3 |
| 4.5 | TestRunIteration_MissingDeliverables_WarnsButCompletes | `internal/daemon` | 1.4 |
| 4.6 | TestRunIteration_YieldWithSubtasks_BlocksParent | `internal/daemon` | 1.5 |
| 4.7 | TestRunIteration_YieldWithoutSubtasks_StaysInProgress | `internal/daemon` | 1.5 |
| 4.8 | TestAutoCompleteDecomposedParent | `internal/daemon` | 1.6 |
| 4.9 | TestPartialWorkCommittedOnFailure | `internal/daemon` | 1.7 |
| 4.10 | TestFailureContextInRetryPrompt | `internal/daemon` or `internal/pipeline` | 1.8 |
| 4.11 | TestDeliverablePathValidation | `cmd/task` | 1.9 |

```bash
go test -race -count=1 ./internal/daemon/ ./internal/pipeline/ ./cmd/task/
git add -A && git commit -m "Tests for all remediation fixes"
```

### Phase 5: Final test suite

```bash
cd /Users/wild/repository/dorkusprime/wolfcastle/fix/remediation
gofmt -l .                                                        # formatting clean
go vet ./...                                                      # vet clean
go build ./...                                                    # build clean
go test -race -count=1 ./...                                      # all unit tests
go test -race -count=1 -tags integration ./test/integration/      # integration tests
go test -race -count=1 -tags smoke ./test/smoke/                  # smoke tests
```

ALL must pass. If regressions appear, fix and re-run from Phase 1 live tests.

### Phase 6: Push and PR

```bash
cd /Users/wild/repository/dorkusprime/wolfcastle/fix/remediation
git push -u origin fix/remediation

# Create PR from wolfcastle/main directory (not the worktree):
cd /Users/wild/repository/dorkusprime/wolfcastle/main
gh pr create --head fix/remediation --title "Remediate 7 daemon failures from domain refactor run" --body "$(cat <<'EOF'
## Summary
- Strip markdown formatting from terminal markers
- Add WOLFCASTLE_SKIP for already-completed tasks
- Skip git progress check for audit tasks
- Downgrade missing deliverables to warning
- Block parent on YIELD with subtask creation, auto-complete when done
- Auto-commit partial work on failure
- Include failure context in retry prompts
- Validate deliverable paths at declaration
- Strengthen ADR/spec enforcement in execute and audit prompts

## Test plan
- 11 new unit tests covering all daemon fixes
- 3-pass deterministic verification for each fix
- Live daemon verification for audit, deliverable, skip, yield, and ADR scenarios
- Full regression suite (unit, integration, smoke)

Run report: docs/domain-refactor-run-report.md
Remediation specs: ~/Desktop/remediation/failure-{1..7}-*.md
EOF
)" --auto --merge
```

## Execution order

```
Phase 1: Code fixes (sequential, with build+test after each)
  For EACH fix:
    1. Apply code change
    2. Build (go build ./...)
    3. Run existing tests (go test ./internal/daemon/)
    4. Run live daemon test from failure spec, 3 consecutive passes
    5. If live test fails → iterate on fix → rebuild → retest
    6. Commit only after 3 consecutive live passes
  1.1 → 1.2 → 1.3 → 1.4 → 1.5 → 1.6 → 1.7 → 1.8
  1.9 (independent, can be done anytime)
    ↓
Phase 2: Prompt and audit changes (single commit)
  Then live-test prompt-dependent fixes (F2, F7): 3 passes each
    ↓
Phase 3: Regression verification (throwaway worktrees from refactor/domains SHAs)
    ↓
Phase 4: Unit tests for all fixes (codify what the live tests proved)
    ↓
Phase 5: Final test suite (go test -race ./...)
    ↓
Phase 6: Push and PR
```

Live daemon tests are the primary gate. Unit tests codify what the live tests already proved. Regression tests ensure fixes don't break scenarios that previously worked.

## Commit strategy

One commit per code fix. Prompt changes in one commit. Tests in one commit. Do not squash. The commit history tells the story:

1. `Strip markdown formatting from terminal marker scanner`
2. `Add WOLFCASTLE_SKIP terminal marker`
3. `Skip git progress check for audit tasks`
4. `Downgrade missing deliverables to warning`
5. `Block parent task on YIELD with subtask creation`
6. `Auto-complete decomposed parents when subtasks finish`
7. `Auto-commit partial work on task failure`
8. `Include failure context in retry iteration prompt`
9. `Validate and suggest corrections for deliverable paths`
10. `Strengthen prompt and audit enforcement for markers, ADRs, specs, decomposition`
11. `Tests for all remediation fixes`

## If something goes wrong

- **A fix breaks existing tests:** Fix the regression before moving to the next task. Run `go test ./internal/daemon/` after every change.
- **Live daemon test fails (model doesn't cooperate):** Iterate on the fix. If the model consistently fails to use the right marker format or follow instructions, that's a signal the prompt needs strengthening, not that the test is inconclusive.
- **Phase 6 reveals a regression:** Identify which commit introduced it (`git bisect`), fix it, re-run Phases 4-6.
- **Context window running low:** The plan is designed for autonomous execution. If compacting is needed, this file plus `~/Desktop/remediation/failure-{1..7}-*.md` contain everything needed to resume.
