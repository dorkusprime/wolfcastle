# Backlog

Items accumulate here as they surface. Don't process unless directed.

## Pipeline Architecture

These shape how Wolfcastle plans, executes, and learns from its work.

- **Harden audit unblock after remediation.** In eval #6, the remediation task successfully unblocked the original audit, but it wasn't obvious whether that would happen reliably. Investigate: does the remediation prompt (`plan-remediate.md`) consistently instruct the model to include an unblock step? Should the daemon auto-unblock as a fallback? A prompt change in the remediate template or the remediation task body may be enough to make this reliable without daemon-side logic.

- **Spec review pipeline.** Specs go from draft to implementation with no structured review. The domain repository spec needed 4 revision passes to reach quality. Wolfcastle should have a review stage: after a spec is created, a separate model audits it for logical gaps, missing method signatures, contradictions, and under-specified behavior before it drives implementation.

- **After Action Reviews per task.** Every task should produce an AAR following the standard template: objective, what happened, what went well, what can be improved, action items. The next task reads prior AARs for full context. Replaces terse breadcrumbs with actionable narrative. AARs accumulate in the leaf directory and become the audit's primary input.

- **Eval framework using task commits as regression tests.** Roll back to the commit before a known-bad decision, apply a fix, replay the task, verify the fix prevents the original mistake. Regression testing for the AI pipeline itself. Multiple eval runs have produced ideal regression cases.

## Daemon Mechanics

Operational improvements to the daemon's core loop and resilience.

- **Stall detector for model invocations.** When the model produces no output for N seconds (e.g., 120s), kill the process and retry. The current 3600s invocation timeout is far too generous. Caught during eval when a claude process hung for 10+ minutes due to API instability.

- **CWD resolution for wolfcastle commands.** Commands should require `.wolfcastle/` in CWD and refuse to operate if it isn't there. No walking up the directory tree. Caught when `wolfcastle start` was accidentally run from `~/` with a stray `.wolfcastle/`.

## User Experience

How the tool feels to use.

- **`wolfcastle log` design pass.** Current issues: intake log (10001-\*) sorts after execute logs, follow mode doesn't know which iteration is active, no stage filtering, no human-readable formatting. Design goals: multiple verbosity levels, `--follow` at all levels, unified log stream with stage tags, iteration addressing, stage filtering.

- **`wolfcastle status` detail.** Show task descriptions (not just titles), failure reasons from the last attempt, deliverable declarations, and breadcrumbs. A user watching the daemon should understand what's happening without reading raw logs.

- **README files in every `.wolfcastle/` directory.** `system/` should explain the three-tier system. `system/base/prompts/` should explain how to override prompts. `docs/` should explain what goes there.

- **Prompt subdirectories for human navigation.** The flat `system/base/prompts/` mixes stage prompts, class prompts, audit prompts, and templates. Better: `prompts/stages/`, `prompts/classes/`, `prompts/audits/`, `prompts/templates/`.

- **Fully user-configurable prompts via the tier system.** Prompts are embedded via go:embed and extracted at scaffold time. Users can override in custom/local but can't easily see the defaults. Make base/ prompts the authoritative reference. `init` populates, `init --force` regenerates.

- **`wolfcastle status -w` refresh is jumpy.** Each refresh hops the screen for a millisecond or two. Should clear and redraw smoothly (terminal alternate screen buffer or cursor repositioning).

- **`wolfcastle status -w` should show the interval.** Display the refresh interval at the top, like `watch` does ("Every 5.0s: wolfcastle status").

- **`wolfcastle status -w --interval` should accept `-n`.** Match `watch` convention: `-n 2` as shorthand for `--interval 2`.

- **`wolfcastle status` should collapse completed nodes.** Show completed leaves/orchestrators as a single line with a count of collapsed children. Flag to expand (e.g., `--all`). Keeps the tree readable as it grows.

- **`wolfcastle status` should indent subtasks by depth.** Hierarchical task IDs (task-0001.0001) should be visually nested under their parent task, not displayed at the same level.

## Code Quality

Things that should be better in the implementation.

- **ContextBuilder null-safety.** `findTask()` returns nil silently causing context truncation. Should error. Template re-parsing on every `Build()` call with no caching.

- **git.CreateWorktree loses the first error.** When the primary `git worktree add -b` fails and the fallback also fails, only the fallback error surfaces.

- **ScaffoldService.Reinit silently ignores migration errors.** Three `_ = migrator.Migrate...()` calls. If migration fails for a real reason, the operator never knows.

- **TTL-based caching in tierfs.** Short TTL for custom/local tiers, long/infinite for base. All repositories benefit from the same caching layer.

## Completed

- ~~Requirements in README~~ (Go 1.26+, Git, a coding agent)
- ~~Terminal marker detection~~ (deliverable check was clearing valid markers)
- ~~Deliverable globs don't recurse~~ (`globRecursive` walks subdirs)
- ~~Deliverable unchanged false failures~~ (replaced hashing with git-diff progress)
- ~~Git progress fails on committed work~~ (`checkGitProgress` checks HEAD moved OR uncommitted)
- ~~Spinner during execution~~ (stop before RunOnce)
- ~~.wolfcastle/system/ restructure (ADR-077)~~ (system internals under system/)
- ~~Boundary rules in prompts~~ (execute and intake forbid writing to system/)
- ~~CLI header cleanup~~ ([INFO] suppressed, version deduped)
- ~~Status detail for completed tasks~~ (summary shown)
- ~~Stale ADR-070~~ (marked superseded)
- ~~Stale daemon agent guide~~ (updated for new patterns)
- ~~Audit progress check~~ (skip for IsAudit, PR #35)
- ~~Markdown terminal markers~~ (strip formatting, PR #35)
- ~~Work-already-done concept~~ (WOLFCASTLE_SKIP, PR #35)
- ~~YIELD + self-decomposition~~ (block parent, auto-complete, PR #35)
- ~~Deliverable missing = failure loop~~ (downgraded to warning, PR #35)
- ~~Deliverable path validation~~ (warns at declaration, PR #35)
- ~~Failure context in retry prompt~~ (LastFailureType, PR #35)
- ~~Decomposition trigger~~ ("list files, decompose if >8", PR #35)
- ~~Scope creep across task boundaries~~ (auto-commit partial work, PR #35)
- ~~ADR/spec audit enforcement~~ (reframed as descriptive, PR #35 + PR #36)
- ~~Rich task definitions~~ (Body, TaskType, Constraints, References, PR #36)
- ~~Hierarchical task IDs~~ (depth-first navigation, derived parent status, PR #36)
- ~~Single daemon enforcement~~ (global lock, CWD-only resolution, PR #36)
- ~~Orchestrator planning pipeline~~ (active planners, re-planning, completion review, PR #36)
- ~~Task descriptions need more detail~~ (rich fields on Task struct, PR #36)
- ~~Decomposition by concern~~ (orchestrator planning, PR #36)
- ~~Auto-decompose on block~~ (orchestrator-as-unblocker, PR #36)
- ~~Intake decomposition granularity~~ (orchestrator plans at spec granularity, PR #36)
- ~~Planning enabled by default~~ (PR #40)
- ~~Execute prompt: --parent for hierarchical decomposition~~ (PR #40)
- ~~Execute prompt: branch/worktree guardrail~~ (PR #40, strengthened PR #45)
- ~~Audit prompt: verify spec content~~ (PR #40)
- ~~Execute prompt spec reference~~ (inline .md references in context, PR #40)
- ~~Planning prompts require --reference~~ (PR #40)
- ~~Duplicate planning bug~~ (inference only, no flag from project create, PR #42)
- ~~OVERLAP marker parsing~~ (daemon delivers scope to target orchestrator, PR #42)
- ~~Audit quality: build, test, correctness, modularity~~ (language-agnostic checklist, PR #43 + PR #44)
- ~~Orchestrator audits~~ (all nodes get audits, deferred until children complete, PR #44 + PR #46 + PR #47)
- ~~Lazy planning~~ (execute first, plan when no tasks available, PR #44 + PR #49)
- ~~PendingScope cleared prematurely~~ (only clear pre-pass scope items, PR #44)
- ~~Default Clock on App~~ (prevent nil pointer, PR #44)
- ~~Flat structure guidance~~ (planning prompt: sub-orchestrators >4 children, PR #44)
- ~~Remediate prompt strategies~~ (spec-fix, code-fix, prerequisite, escalate, PR #44)
- ~~Planning deadlock with audit tasks~~ (inference ignores audit tasks, PR #47)
- ~~Planners only create direct children~~ (not grandchildren, PR #48)
- ~~.gitignore not tracking state files~~ (unignore intermediate directories, PR #48)
- ~~Daemon loop cleanup~~ (three clean steps: execute, plan, idle, PR #49)
- ~~Blocked audit propagation~~ (BLOCKED propagates to index for remediation, PR #50)
- ~~ADR/spec prompt rewrite~~ (WHY/WHAT/HOW framing, missing ADRs = REMEDIATE, PR #50)
- ~~NeedsPlanning inference~~ (structural detection of childless orchestrators, PR #42 + PR #47)
- ~~Spec stubs pass through pipeline~~ (audit checks content not existence, PR #40 + PR #43)
- ~~Structured audit PASS/REMEDIATE~~ (verdicts, daemon handles BLOCKED, PR #43 + PR #44 + PR #50)
- ~~Domain refactor: migrate backward-compat callers~~ (App Refactor in eval #6, test/domains run)
- ~~Spec-first ordering codified~~ (planning prompt, PR #52)
- ~~Remediation unblock step~~ (remediate prompt, PR #52)
