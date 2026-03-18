# Backlog

Items accumulate here as they surface. Don't process unless directed.

## Pipeline Architecture

These shape how Wolfcastle plans, executes, and learns from its work.

- **Planning pipeline doesn't populate task References.** Tasks created by the planner have bodies but no References linking back to the originating spec. The executor has to discover the spec by searching. The planner should set References on every task. When no spec exists, the planner should create a spec-writing discovery task first.

- **Spec stubs pass through the pipeline.** During the domains eval, 5 of 7 spec files contained only `[Spec content goes here.]`. The audit model created placeholder files to satisfy its own "spec exists" check. The PromptRepository audit caught and fixed its own stub, but earlier audits didn't. The planning prompt should require populated specs as deliverables, not just file creation.

- **Spec review pipeline.** Specs currently go from draft to implementation with no structured review. The domain repository spec needed 4 revision passes to reach quality (internal audit, external audit, existential review). Wolfcastle should have a review stage: after a spec is created, a separate model audits it for logical gaps, missing method signatures, contradictions, and under-specified behavior before it drives implementation.

- **Structured audit reports with PASS/REMEDIATE verdicts.** Audit tasks should produce a markdown report with a clear verdict. PASS is the expected outcome. REMEDIATE requires concrete, verifiable findings with file:line evidence. The daemon parses the verdict: PASS completes, REMEDIATE spawns tasks plus a re-audit. The template must make "no issues found" a first-class outcome to prevent hallucinated remediation.

- **Audit quality beyond structural checks.** The current audit prompt only verifies files exist and are in the right place. It should also check error wrapping, doc comments on exports, test coverage, Go idiom, naming conventions, and dead code. The task-classes spec defines a `discipline-audit` class whose behavioral prompt should carry these criteria. Implement as part of task-classes, not separately.

- **After Action Reviews per task.** Every task should produce an AAR following the standard template: objective, what happened, what went well, what can be improved, action items. The next task reads prior AARs for full context. Replaces terse breadcrumbs with actionable narrative. AARs accumulate in the leaf directory and become the audit's primary input.

- **Eval framework using task commits as regression tests.** The domain repository runs produced both failures and successes. Roll back to the commit before a known-bad decision, apply a fix, replay the task, verify the fix prevents the original mistake. Regression testing for the AI pipeline itself.

## Daemon Mechanics

Operational improvements to the daemon's core loop and resilience.

- **Stall detector for model invocations.** When the model produces no output for N seconds (e.g., 120s), kill the process and retry. The current 3600s invocation timeout is far too generous when the model has already committed its work and is hung on post-commit steps. Caught during eval when a claude process hung for 10+ minutes due to API instability.

- **NeedsPlanning inference for re-planning triggers.** The daemon infers initial planning from structure (childless orchestrator), but re-planning (new scope, child blocked, completion review) still depends on explicitly setting a flag. Could be made structural: check whether pending scope exists, whether any child is blocked, or whether all children are complete, rather than relying on a flag set at the right time.

- **CWD resolution for wolfcastle commands.** Commands should require `.wolfcastle/` in CWD and refuse to operate if it isn't there. No walking up the directory tree. Caught when `wolfcastle start` was accidentally run from `~/` with a stray `.wolfcastle/` and silently operated on the wrong project.

## User Experience

How the tool feels to use.

- **`wolfcastle log` design pass.** The log command needs a full spec. Current issues: intake log (10001-*) sorts after execute logs, follow mode doesn't know which iteration is active, no stage filtering, no human-readable formatting. Design goals: multiple verbosity levels (daemon output, info, debug, raw NDJSON), `--follow` at all levels, unified log stream with stage tags, iteration addressing (`--iteration 24`), stage filtering (`--stage execute`).

- **`wolfcastle status` detail.** Show task descriptions (not just titles), failure reasons from the last attempt, deliverable declarations, and breadcrumbs. A user watching the daemon should understand what's happening without reading raw logs.

- **README files in every `.wolfcastle/` directory.** `system/` should explain the three-tier system. `system/base/prompts/` should explain how to override prompts. `docs/` should explain what goes there. These are the discoverability layer for humans browsing the directory.

- **Prompt subdirectories for human navigation.** The flat `system/base/prompts/` mixes stage prompts, class prompts, audit prompts, and templates. Better: `prompts/stages/`, `prompts/classes/`, `prompts/audits/`, `prompts/templates/`. Users should browse and understand what's available without reading source code.

- **Fully user-configurable prompts via the tier system.** Prompts are currently embedded in the binary via go:embed and extracted at scaffold time. Users can override in custom/local but can't easily see the defaults. Make base/ prompts the authoritative reference. `init` populates, `init --force` regenerates.

## Code Quality

Things that should be better in the implementation.

- **Domain refactor: migrate remaining backward-compat callers.** 17 files still use `app.WolfcastleDir`, 9 use `app.Cfg`, 15 use `app.Store`. These need migration to repository methods before `tree.Resolver` can be deleted. Old code paths (`AssemblePrompt`, `project.Scaffold`) exist alongside new ones and will drift.

- **ContextBuilder null-safety.** Docstring claims nil repositories degrade gracefully, but `findTask()` returns nil silently causing context truncation. Should error. Template re-parsing on every `Build()` call with no caching.

- **git.CreateWorktree loses the first error.** When the primary `git worktree add -b` fails, the fallback runs without `-b`. If both fail, only the fallback error surfaces. The original (potentially more informative) error is discarded.

- **ScaffoldService.Reinit silently ignores migration errors.** Three `_ = migrator.Migrate...()` calls. If migration fails for a real reason (permissions, corruption), the operator never knows.

- **TTL-based caching in tierfs.** Short TTL (1-5s) for custom/local tiers (rapid-fire reads during a single iteration hit cache, human edits between iterations are fresh). Long/infinite TTL for base (only changes on rescaffold). All repositories benefit from the same caching layer.

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
- ~~Audit progress check~~ (skip for IsAudit tasks, PR #35)
- ~~Markdown terminal markers~~ (strip formatting before matching, PR #35)
- ~~Work-already-done concept~~ (WOLFCASTLE_SKIP marker, PR #35)
- ~~YIELD + self-decomposition~~ (block parent, auto-complete, PR #35)
- ~~Deliverable missing = failure loop~~ (downgraded to warning, PR #35)
- ~~Deliverable path validation~~ (warns and suggests at declaration, PR #35)
- ~~Failure context in retry prompt~~ (LastFailureType injected, PR #35)
- ~~Decomposition trigger~~ ("list files, decompose if >8", PR #35)
- ~~Scope creep across task boundaries~~ (auto-commit partial work, PR #35)
- ~~ADR/spec audit enforcement~~ (reframed as descriptive, PR #35 + PR #36)
- ~~Rich task definitions~~ (Body, TaskType, Constraints, AcceptanceCriteria, References, PR #36)
- ~~Hierarchical task IDs~~ (depth-first navigation, derived parent status, PR #36)
- ~~Single daemon enforcement~~ (global lock, CWD-only resolution, PR #36)
- ~~Orchestrator planning pipeline~~ (active planning agents, re-planning, completion review, PR #36)
- ~~Task descriptions need more detail~~ (rich fields on Task struct, PR #36)
- ~~Decomposition by concern~~ (orchestrator planning handles this, PR #36)
- ~~Auto-decompose on block~~ (orchestrator-as-unblocker, PR #36)
- ~~Intake decomposition granularity~~ (orchestrator reads spec, plans at right granularity, PR #36)
- ~~Planning enabled by default~~ (was dead code at false, fix/low-hanging)
- ~~Execute prompt: --parent for hierarchical decomposition~~ (fix/low-hanging)
- ~~Execute prompt: branch/worktree guardrail~~ (fix/low-hanging)
- ~~Audit prompt: verify spec content not just existence~~ (fix/low-hanging)
- ~~Execute prompt spec reference~~ (inline .md references in iteration context, fix/low-hanging)
- ~~Planning prompts require --reference when scope has a spec~~ (fix/low-hanging)
