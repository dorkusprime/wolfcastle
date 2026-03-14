# Wolfcastle Improvement Spec for Claude

This document is a concrete implementation plan for improving the Claude Wolfcastle implementation based on the comparative evaluation across Claude, Gemini, and Codex.

The goal is not to redesign Wolfcastle. The goal is to close the specific gaps between the current Claude implementation and the project specs in `/Volumes/git/dorkusprime/wolfcastle/docs/`, while borrowing proven ideas from the other two implementations where they are stronger.

Primary references:

- Current implementation: `wolfcastle-claude`
- Canonical specs: `/Volumes/git/dorkusprime/wolfcastle/docs/specs/`
- Canonical ADRs: `/Volumes/git/dorkusprime/wolfcastle/docs/decisions/`
- Reference implementation strengths:
  - Codex for operational coverage, archive/audit workflow, and end-to-end tests
  - Gemini for validation taxonomy breadth

## Priorities

Implement these in order:

1. Fix schema mismatches in state/config/init.
2. Fix prompt assembly contract for `skip_prompt_assembly`.
3. Upgrade `doctor` to the full structural validation engine.
4. Make daemon startup, self-healing, and lifecycle match the CLI spec and ADR-020.
5. Fill in missing command-contract details and broaden tests.

---

## 1. Fix Distributed State Schema Mismatches

### Problem

Your core algorithms are good, but the persisted schema diverges from the spec in ways that will cause downstream incompatibility:

- `/Volumes/git/dorkusprime/wolfcastle-claude/internal/state/types.go:33-40` defines `RootIndex` with `root_id`, `root_name`, and `root_state`, but the state-machine and validation specs treat the root `state.json` as a registry keyed by tree address with a root list/registry model, not a root metadata wrapper.
- `/Volumes/git/dorkusprime/wolfcastle-claude/internal/state/types.go:77-85` defines `Task` without `is_audit`, and uses `block_reason` instead of `blocked_reason`.
- `/Volumes/git/dorkusprime/wolfcastle-claude/internal/project/scaffold.go:83-90` creates the root index using the current non-spec shape.

### Required changes

#### 1.1 Update root index schema

Update:

- `/Volumes/git/dorkusprime/wolfcastle-claude/internal/state/types.go`
- `/Volumes/git/dorkusprime/wolfcastle-claude/internal/state/io.go`
- `/Volumes/git/dorkusprime/wolfcastle-claude/internal/project/scaffold.go`
- Any tests that assume the old root index shape

Target behavior:

- Match the canonical state-file requirements from:
  - `/Volumes/git/dorkusprime/wolfcastle/docs/specs/2026-03-12T00-00Z-state-machine.md:163-171`
  - `/Volumes/git/dorkusprime/wolfcastle/docs/specs/2026-03-13T00-00Z-structural-validation.md:173-181`

Use Codex as the closest concrete reference for the root registry shape:

- `/Volumes/git/dorkusprime/wolfcastle-codex/types.go:91-104`
- `/Volumes/git/dorkusprime/wolfcastle-codex/state_tree.go:87-96`
- `/Volumes/git/dorkusprime/wolfcastle-codex/state_tree.go:98-136`

Codex is not perfect overall, but its root index shape is closer to the intended distributed-state model than Claude’s current `root_id/root_name/root_state` struct.

Implementation requirements:

- Root state file should carry:
  - `version`
  - a collection of root addresses
  - a flat `nodes` map keyed by tree address
- Each node entry should include:
  - `address`
  - `name`
  - `type`
  - `state`
  - `parent`
  - child metadata as needed
- Eliminate `root_id/root_name/root_state` from the persisted root schema.

#### 1.2 Update task schema

Update `/Volumes/git/dorkusprime/wolfcastle-claude/internal/state/types.go:77-85`.

Required changes:

- Rename `BlockReason` JSON tag from `block_reason` to `blocked_reason`.
- Add `IsAudit bool 'json:"is_audit,omitempty"'`.
- Keep `failure_count`.

Use these as references:

- Spec: `/Volumes/git/dorkusprime/wolfcastle/docs/specs/2026-03-13T00-00Z-structural-validation.md:173-181`
- Codex task schema: `/Volumes/git/dorkusprime/wolfcastle-codex/types.go:131-138`
- Gemini task schema: `/Volumes/git/dorkusprime/wolfcastle-gemini/internal/state/state.go:114-123`

#### 1.3 Make audit-task identity explicit

Today many Claude codepaths infer the audit task by `ID == "audit"`:

- `/Volumes/git/dorkusprime/wolfcastle-claude/cmd/doctor.go:105-128`
- `/Volumes/git/dorkusprime/wolfcastle-claude/internal/state/mutations.go:33-41`

That is weaker than the spec. The spec requires the audit task to be the task with `is_audit: true`, always last, undeletable, and unique.

Required changes:

- Persist `is_audit: true` on the audit task.
- Update all logic that currently checks `ID == "audit"` to prefer `IsAudit`.
- Keep `ID == "audit"` as a compatibility fallback only if needed during migration.

Reference implementations:

- Gemini audit-task handling:
  - `/Volumes/git/dorkusprime/wolfcastle-gemini/internal/cli/task.go:98-123`
  - `/Volumes/git/dorkusprime/wolfcastle-gemini/internal/doctor/doctor.go:198-259`
- Codex audit-task handling:
  - `/Volumes/git/dorkusprime/wolfcastle-codex/app.go:1156-1163`
  - `/Volumes/git/dorkusprime/wolfcastle-codex/doctor.go:247-270`

---

## 2. Fix `init` and Config Scaffolding

### Problem

`/Volumes/git/dorkusprime/wolfcastle-claude/internal/project/scaffold.go:51-60` writes an empty `config.json`, which contradicts the CLI spec:

- `/Volumes/git/dorkusprime/wolfcastle/docs/specs/2026-03-12T00-06Z-cli-commands.md:78-87`

The spec explicitly requires default shared config content to be written at init time.

### Required changes

#### 2.1 Write a real default `config.json`

Replace the empty config scaffold in:

- `/Volumes/git/dorkusprime/wolfcastle-claude/internal/project/scaffold.go:51-60`

with a populated default config derived from your existing hardcoded defaults in:

- `/Volumes/git/dorkusprime/wolfcastle-claude/internal/config/config.go:9-92`

Do not invent another default source. Reuse the existing config defaults and serialize them.

Good reference patterns:

- Codex:
  - `/Volumes/git/dorkusprime/wolfcastle-codex/defaults.go:9-45`
  - `/Volumes/git/dorkusprime/wolfcastle-codex/app.go:117-118`
- Gemini:
  - `/Volumes/git/dorkusprime/wolfcastle-gemini/internal/config/config.go:184-269`

Codex is the better reference here because it cleanly separates default shared config from default local identity config.

#### 2.2 Preserve `config.local.json` semantics

Your `config.local.json` handling is already broadly aligned, but after schema updates ensure:

- `identity` is only populated in local config.
- Force-mode init updates identity fields without overwriting user overrides.

Codex has a good pattern for this:

- `/Volumes/git/dorkusprime/wolfcastle-codex/app.go:121-139`

Bring that preservation behavior into Claude’s `init`.

---

## 3. Fix Prompt Assembly Contract

### Problem

`/Volumes/git/dorkusprime/wolfcastle-claude/internal/pipeline/prompt.go:13-19` returns only stage prompt content when `skip_prompt_assembly` is enabled.

That is incorrect. The spec requires:

- stage prompt only
- plus iteration context

See:

- `/Volumes/git/dorkusprime/wolfcastle/docs/specs/2026-03-12T00-03Z-pipeline-stage-contract.md:134-170`

### Required changes

Update:

- `/Volumes/git/dorkusprime/wolfcastle-claude/internal/pipeline/prompt.go`

Required behavior:

- For normal stages:
  - rule fragments
  - script reference
  - stage prompt
  - iteration context
- For `skip_prompt_assembly: true` stages:
  - stage prompt
  - iteration context

Use Codex as the best concrete reference:

- `/Volumes/git/dorkusprime/wolfcastle-codex/runtime_stage.go:78-130`

Codex gets the conditional assembly shape right, even though its broader runtime has other issues.

Do not follow Gemini here:

- `/Volumes/git/dorkusprime/wolfcastle-gemini/internal/pipeline/pipeline.go:112-118`

Gemini reads script reference from `base/scripts/script-reference.md`, which is contrary to the spec path. Claude already uses the correct `prompts/script-reference.md` path in:

- `/Volumes/git/dorkusprime/wolfcastle-claude/internal/pipeline/prompt.go:34-44`

Keep that path.

### Additional prompt work

Add tests covering:

- normal assembly includes all four layers
- `skip_prompt_assembly` excludes rules/script reference but still includes iteration context
- missing stage prompt is fatal
- script-reference file resolution comes from `base/prompts/script-reference.md`

Use your existing test style from:

- `/Volumes/git/dorkusprime/wolfcastle-claude/internal/pipeline/prompt_test.go`
- `/Volumes/git/dorkusprime/wolfcastle-claude/internal/pipeline/context_test.go`
- `/Volumes/git/dorkusprime/wolfcastle-claude/internal/pipeline/fragments_test.go`

---

## 4. Replace the Current `doctor` with a Real Validation Engine

### Problem

Claude’s current `doctor` in:

- `/Volumes/git/dorkusprime/wolfcastle-claude/cmd/doctor.go:23-380`

is far too shallow relative to the structural validation spec:

- `/Volumes/git/dorkusprime/wolfcastle/docs/specs/2026-03-13T00-00Z-structural-validation.md:9-266`

It mostly checks:

- missing state files
- state mismatch between index and node
- missing/misordered audit task
- some orphaned files

That misses the majority of required categories and does not model deterministic vs model-assisted fixes cleanly.

### Required changes

#### 4.1 Create a dedicated validation package

Add a package under `/Volumes/git/dorkusprime/wolfcastle-claude/internal/validate/` or `/Volumes/git/dorkusprime/wolfcastle-claude/internal/doctor/validate/`.

Do not keep the entire engine inside Cobra command code.

Use Gemini as the category-coverage reference:

- `/Volumes/git/dorkusprime/wolfcastle-gemini/internal/doctor/doctor.go:15-339`

Use Codex as the workflow and repair-depth reference:

- `/Volumes/git/dorkusprime/wolfcastle-codex/doctor.go:1-500`

Target design:

- Command layer only handles CLI flags/output.
- Validation package handles:
  - tree loading
  - issue generation
  - startup subset
  - deterministic repair execution
  - model-assisted repair interface

#### 4.2 Implement all 17 required issue categories

Implement the exact categories from:

- `/Volumes/git/dorkusprime/wolfcastle/docs/specs/2026-03-13T00-00Z-structural-validation.md:13-227`

Minimum required issue IDs:

- `ROOTINDEX_DANGLING_REF`
- `ROOTINDEX_MISSING_ENTRY`
- `ORPHAN_STATE`
- `ORPHAN_DEFINITION`
- `PROPAGATION_MISMATCH`
- `MISSING_AUDIT_TASK`
- `AUDIT_NOT_LAST`
- `MULTIPLE_AUDIT_TASKS`
- `INVALID_STATE_VALUE`
- `INVALID_TRANSITION_COMPLETE_WITH_INCOMPLETE`
- `INVALID_TRANSITION_BLOCKED_WITHOUT_REASON`
- `STALE_IN_PROGRESS`
- `MULTIPLE_IN_PROGRESS`
- `DEPTH_MISMATCH`
- `NEGATIVE_FAILURE_COUNT`
- `MISSING_REQUIRED_FIELD`
- `MALFORMED_JSON`

Gemini has the cleanest category inventory:

- `/Volumes/git/dorkusprime/wolfcastle-gemini/internal/doctor/doctor.go:21-39`

Codex has stronger handling for some audit-specific consistency checks not fully called out in Gemini:

- `/Volumes/git/dorkusprime/wolfcastle-codex/doctor.go:93-200`
- `/Volumes/git/dorkusprime/wolfcastle-codex/doctor.go:247-360`

Borrow those as additional checks, but keep the canonical 17 as the required minimum.

#### 4.3 Implement deterministic/model-assisted/manual fix strategy explicitly

Gemini already models fix types:

- `/Volumes/git/dorkusprime/wolfcastle-gemini/internal/doctor/doctor.go:47-54`
- `/Volumes/git/dorkusprime/wolfcastle-gemini/internal/doctor/doctor.go:341-507`

You should adopt that separation, but tighten it:

- Deterministic fix code must not live inline in Cobra command handlers.
- Model-assisted fixes should be routed through a narrow interface with explicit guardrails.
- Manual/no-fix categories must be reported but not mutated.

#### 4.4 Add startup validation subset

The daemon start flow should run a fast subset before execution begins.

Required integration targets:

- `/Volumes/git/dorkusprime/wolfcastle-claude/cmd/start.go`
- `/Volumes/git/dorkusprime/wolfcastle-claude/internal/daemon/daemon.go`

Codex has the cleanest current startup behavior here:

- `/Volumes/git/dorkusprime/wolfcastle-codex/daemon.go:76-83`

It is not a full validation engine, but it correctly blocks startup on doctor errors. Claude currently does not do this.

#### 4.5 Implement atomic fix application

The spec requires grouped writes and rollback-aware behavior. Neither Claude nor Gemini currently do this well.

Codex is still non-atomic:

- `/Volumes/git/dorkusprime/wolfcastle-codex/app.go:1052-1058`

So do not copy that directly. Instead:

- Use your existing atomic node-state save pattern as the base:
  - `/Volumes/git/dorkusprime/wolfcastle-claude/internal/state/io.go`
- Extend it so doctor repairs:
  - stage changes in memory
  - write leaf -> parent -> root in deterministic order
  - re-validate
  - only commit if the repair does not introduce new errors

That work will be net-new in Claude and should be treated as first-class infrastructure.

---

## 5. Bring Daemon Startup and Lifecycle to Spec

### Problem

`/Volumes/git/dorkusprime/wolfcastle-claude/cmd/start.go:58-75` and `/Volumes/git/dorkusprime/wolfcastle-claude/internal/daemon/daemon.go:56-155` are solid beginnings, but the lifecycle is still missing required contract pieces:

- explicit startup validation phase
- explicit stale in-progress self-healing before normal traversal
- spec-complete background PID behavior
- stop semantics aligned with process-group handling

### Required changes

#### 5.1 Add startup validation

Before creating/running the daemon, run the startup validation subset.

Update:

- `/Volumes/git/dorkusprime/wolfcastle-claude/cmd/start.go`

Reference:

- Codex startup validation gate: `/Volumes/git/dorkusprime/wolfcastle-codex/daemon.go:76-83`
- Spec: `/Volumes/git/dorkusprime/wolfcastle/docs/specs/2026-03-12T00-06Z-cli-commands.md:167-184`

#### 5.2 Add explicit stale `in_progress` resume path

Your navigation function already prefers stale `in_progress` tasks correctly:

- `/Volumes/git/dorkusprime/wolfcastle-claude/internal/state/navigation.go:83-103`

What is missing is explicit startup framing that treats this as self-healing and logs it as such per ADR-020:

- `/Volumes/git/dorkusprime/wolfcastle/docs/decisions/020-daemon-lifecycle.md:32-37`

Gemini has an explicit `selfHeal()` phase:

- `/Volumes/git/dorkusprime/wolfcastle-gemini/internal/daemon/daemon.go:114-185`

Do not copy its exact execution behavior, because Gemini’s navigation/task model has other problems. But do copy the explicit startup-phase shape:

- detect stale in-progress tasks
- if more than one: fail startup
- if exactly one: log self-healing and resume that task first

#### 5.3 Improve background startup confirmation

Current background start:

- `/Volumes/git/dorkusprime/wolfcastle-claude/cmd/start.go:78-117`

writes the PID immediately after `proc.Start()`, without a real health handshake.

Codex handles this better:

- `/Volumes/git/dorkusprime/wolfcastle-codex/daemon.go:137-183`

Borrow the concept:

- background child writes healthy startup metadata after initialization succeeds
- parent waits for confirmation or timeout
- parent reports startup failure if child exits early

#### 5.4 Tighten signal and child-process ownership

Your model invocation already uses process groups correctly:

- `/Volumes/git/dorkusprime/wolfcastle-claude/internal/invoke/invoker.go:32-40`

That is better than Codex. Preserve it.

What still needs work:

- ensure stop/interrupt propagates to the child process group in all daemon modes
- verify graceful shutdown waits for current stage exit before quitting

Audit:

- `/Volumes/git/dorkusprime/wolfcastle-claude/internal/daemon/daemon.go`
- `/Volumes/git/dorkusprime/wolfcastle-claude/cmd/stop.go`

Use the process-group ownership expectations from:

- `/Volumes/git/dorkusprime/wolfcastle/docs/specs/2026-03-12T00-03Z-pipeline-stage-contract.md:105-112`
- `/Volumes/git/dorkusprime/wolfcastle/docs/decisions/020-daemon-lifecycle.md:18-31`

Codex is weaker here, so do not copy its force-stop approach:

- `/Volumes/git/dorkusprime/wolfcastle-codex/daemon.go:404-432`

---

## 6. Expand Command-Contract Compliance

### Problem

Claude has the broadest command surface, but several commands still need stricter conformance to the CLI spec.

### Required changes

#### 6.1 Ensure all commands support `--json`

Your root command advertises:

- `/Volumes/git/dorkusprime/wolfcastle-claude/cmd/root.go:33-34`

but audit each command to ensure:

- success output uses the common envelope
- error output goes to stderr
- JSON mode prints `{ ok, error, code }` envelope

Reference patterns:

- Claude envelope helper:
  - `/Volumes/git/dorkusprime/wolfcastle-claude/internal/output/envelope.go:9-42`
- Gemini handled exit/error envelope:
  - `/Volumes/git/dorkusprime/wolfcastle-gemini/internal/cli/adr.go:111-120`

Gemini is better about explicit exit-code-carrying handled errors; that pattern is worth importing into Claude’s command layer.

#### 6.2 Audit command/address contract

Confirm that every command using `--node` accepts true tree addresses as required by:

- `/Volumes/git/dorkusprime/wolfcastle/docs/specs/2026-03-12T00-06Z-cli-commands.md:9-16`

Your current address parsing in `/Volumes/git/dorkusprime/wolfcastle-claude/internal/tree/address.go` is good. Use it consistently and add tests wherever command parsing still relies on loose string behavior.

#### 6.3 Match help and command surface to the current spec

Compare your command surface against:

- `/Volumes/git/dorkusprime/wolfcastle/docs/specs/2026-03-12T00-06Z-cli-commands.md`

Codex help text is a good checklist for breadth:

- `/Volumes/git/dorkusprime/wolfcastle-codex/help.go:5-245`

Not everything there is guaranteed spec-perfect, but it is a useful command inventory.

---

## 7. Borrow Codex’s Stronger Audit and Archive Workflow

### Problem

Claude has archive support, but Codex has significantly more complete operational handling around:

- audit scope mutation
- gap lifecycle
- escalation resolution
- result summary persistence
- archive rollup inputs

### Required changes

Audit these Claude areas:

- `/Volumes/git/dorkusprime/wolfcastle-claude/cmd/audit_breadcrumb.go`
- `/Volumes/git/dorkusprime/wolfcastle-claude/cmd/audit_escalate.go`
- archive-related commands
- state audit structs

Then compare to Codex:

- `/Volumes/git/dorkusprime/wolfcastle-codex/auditcmd.go`
- `/Volumes/git/dorkusprime/wolfcastle-codex/archive.go`
- `/Volumes/git/dorkusprime/wolfcastle-codex/doctor.go:271-284`

Specific features to pull in:

- explicit audit scope mutation commands and structured persistence
- gap open/fixed lifecycle with timestamps and ownership
- escalation resolve command support
- stronger archive rendering from audit scope + summary

Archive formatting references:

- Claude current archive rollup:
  - `/Volumes/git/dorkusprime/wolfcastle-claude/internal/archive/rollup.go`
- Codex archive behavior:
  - `/Volumes/git/dorkusprime/wolfcastle-codex/archive.go`
  - `/Volumes/git/dorkusprime/wolfcastle-codex/main_test.go:383-415`

---

## 8. Expand Tests Aggressively

### Problem

Claude already has the best test shape, but the missing features above need corresponding coverage or they will regress.

### Required test additions

#### 8.1 Validation engine tests

Add dedicated tests for each required validation category.

Gemini’s doctor code gives you a category matrix; Codex’s `main_test.go` gives you fixture-driven repair scenarios.

Primary references:

- `/Volumes/git/dorkusprime/wolfcastle-gemini/internal/doctor/doctor.go:21-39`
- `/Volumes/git/dorkusprime/wolfcastle-codex/main_test.go:790-1521`

#### 8.2 Init/config tests

Add tests to verify:

- `config.json` is populated with defaults
- `config.local.json` preserves user overrides on force reinit
- `identity` remains local-only

References:

- `/Volumes/git/dorkusprime/wolfcastle-claude/internal/project/scaffold_test.go`
- `/Volumes/git/dorkusprime/wolfcastle-codex/main_test.go:1656-1664`

#### 8.3 Prompt-assembly tests

Add tests covering:

- `skip_prompt_assembly` still includes iteration context
- missing `script-reference.md` behavior
- tier override precedence

Reference:

- `/Volumes/git/dorkusprime/wolfcastle-codex/main_test.go:1850-1939`

#### 8.4 Daemon lifecycle tests

Add tests for:

- stale PID cleanup
- startup validation failure
- stale `in_progress` self-healing
- background startup confirmation

References:

- `/Volumes/git/dorkusprime/wolfcastle-codex/status_test.go:322`
- `/Volumes/git/dorkusprime/wolfcastle-codex/main_test.go:845-850`
- `/Volumes/git/dorkusprime/wolfcastle-codex/daemon.go:137-183`

---

## 9. What Not to Copy

- Do not copy Codex’s `recomputeLeafState()` behavior from `/Volumes/git/dorkusprime/wolfcastle-codex/state_tree.go:174-208`. It is wrong for blocked-task handling.
- Do not copy Gemini’s orchestrator audit-task generation from `/Volumes/git/dorkusprime/wolfcastle-gemini/internal/state/state.go:250-266`. It contradicts the core node model.
- Do not copy Codex’s non-atomic `writeJSON()` persistence from `/Volumes/git/dorkusprime/wolfcastle-codex/app.go:1052-1058`.
- Do not copy Gemini’s extra config-layer interpretation from `/Volumes/git/dorkusprime/wolfcastle-gemini/internal/config/config.go:153-172`.

---

## 10. Concrete File-by-File Work List

### Must modify

- `/Volumes/git/dorkusprime/wolfcastle-claude/internal/state/types.go`
- `/Volumes/git/dorkusprime/wolfcastle-claude/internal/state/io.go`
- `/Volumes/git/dorkusprime/wolfcastle-claude/internal/project/scaffold.go`
- `/Volumes/git/dorkusprime/wolfcastle-claude/internal/pipeline/prompt.go`
- `/Volumes/git/dorkusprime/wolfcastle-claude/cmd/start.go`
- `/Volumes/git/dorkusprime/wolfcastle-claude/internal/daemon/daemon.go`
- `/Volumes/git/dorkusprime/wolfcastle-claude/cmd/doctor.go` or replace with a thin wrapper around a new validation package

### Likely add

- `/Volumes/git/dorkusprime/wolfcastle-claude/internal/validate/*.go`
- `/Volumes/git/dorkusprime/wolfcastle-claude/internal/validate/*_test.go`
- new daemon lifecycle tests
- new init/config tests
- new prompt assembly tests

### Must update tests

- `/Volumes/git/dorkusprime/wolfcastle-claude/internal/project/scaffold_test.go`
- `/Volumes/git/dorkusprime/wolfcastle-claude/internal/pipeline/*_test.go`
- state IO / schema tests
- new doctor/validation test suite

---

## End State

Claude should remain the most maintainable implementation, but after these changes it should also become the most spec-complete one.

If you execute this plan correctly, the resulting implementation should combine:

- Claude’s current strengths in core state and package design
- Gemini’s validation breadth
- Codex’s workflow completeness and test realism

That combination is the strongest path to a first-place implementation on a re-evaluation.
