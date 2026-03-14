# Wolfcastle Improvement Specification: Claude Implementation

## Overview
Your implementation of Wolfcastle has the strongest architectural foundation, with excellent package boundaries (`/Volumes/git/dorkusprime/wolfcastle-claude/cmd/`, `/Volumes/git/dorkusprime/wolfcastle-claude/internal/state/`, `/Volumes/git/dorkusprime/wolfcastle-claude/internal/config/`, etc.), rigorous core state/config testing (134 tests), and correct state algorithms (like `PropagateUp`). However, some critical features were left as stubs or deviated from the spec schema. To reach production readiness, you need to implement the full validation engine, refine your schema, and improve daemon lifecycle management.

## 1. Complete the Validation Engine (`doctor`)
Your current `/Volumes/git/dorkusprime/wolfcastle-claude/cmd/doctor.go` only implements 7 of the 17 required validation categories and lacks a model-assisted fix strategy.
*   **Actionable Fix:** Implement the remaining 10 categories (e.g., `MULTIPLE_AUDIT_TASKS`, `DEPTH_MISMATCH`, `NEGATIVE_FAILURE_COUNT`, `MALFORMED_JSON`). 
*   **Reference implementation:** Look at Gemini's `/Volumes/git/dorkusprime/wolfcastle-gemini/internal/doctor/doctor.go` (lines 15-38). Gemini defines all 17 issue categories as typed constants and implements a robust three-tier fix strategy.
*   **Model-Assisted Fixes:** You must implement model-assisted structural repairs. Reference Gemini's `tryModelAssistedFix` function in `/Volumes/git/dorkusprime/wolfcastle-gemini/internal/doctor/doctor.go` which invokes the model with a clear prompt to resolve ambiguous state conflicts.
*   **State Normalization:** Reference Codex's `normalizeStateValue` function in `/Volumes/git/dorkusprime/wolfcastle-codex/doctor.go` which gracefully handles common typos (e.g., parsing "done" as "complete", "stuck" as "blocked").
*   **Rollback Semantics:** Ensure that fixes are applied atomically. Post-fix re-validation must occur, and if new issues are introduced, the fix must be rolled back.

## 2. Implement Explicit Daemon Self-Healing
Your daemon relies on implicit self-healing via navigation ordering but lacks the explicit self-healing phase on startup required by ADR-020.
*   **Actionable Fix:** Add an explicit startup phase to `/Volumes/git/dorkusprime/wolfcastle-claude/cmd/start.go` that scans the tree for stale `in_progress` tasks and resumes them before entering the daemon loop.
*   **Reference implementation:** Look at Gemini's `/Volumes/git/dorkusprime/wolfcastle-gemini/internal/daemon/daemon.go`, specifically the `selfHeal` method, and Codex's `/Volumes/git/dorkusprime/wolfcastle-codex/daemon.go` `recoverStaleDaemonState` function for examples of explicit daemon state recovery.

## 3. Correct Schema Mismatches & Prompt Assembly Bugs
Your data structures diverge from the spec, and the pipeline drops crucial context.
*   **Actionable Fix (Schema):** Update `/Volumes/git/dorkusprime/wolfcastle-claude/internal/state/types.go`. `RootIndex` must match the spec's flat registry shape, not use `root_id`/`root_name`. Tasks must include `is_audit` and use `blocked_reason` instead of `block_reason`.
*   **Actionable Fix (Prompt Assembly):** In `/Volumes/git/dorkusprime/wolfcastle-claude/internal/pipeline/prompt.go`, your `skip_prompt_assembly` logic drops the iteration context entirely. The spec requires retaining iteration context even when assembly is skipped. Reference Codex's `/Volumes/git/dorkusprime/wolfcastle-codex/runtime_stage.go` which handles this correctly.
*   **Actionable Fix (Init):** Your `/Volumes/git/dorkusprime/wolfcastle-claude/internal/project/scaffold.go` writes an empty `config.json`. Update it to write a fully populated default shared config as required by the CLI spec.

## 4. Unify PID Management and Add Daemon Supervision
PID file logic is currently split between `/Volumes/git/dorkusprime/wolfcastle-claude/internal/daemon/pid.go` and `/Volumes/git/dorkusprime/wolfcastle-claude/cmd/start.go`.
*   **Actionable Fix:** Consolidate PID management. Implement a robust daemon supervisor loop that handles detached mode and crash recovery.
*   **Reference implementation:** Review Codex's `/Volumes/git/dorkusprime/wolfcastle-codex/daemon.go` (`runDaemonSupervisor` and `startDetached` functions) which implements production-grade daemon supervision, PID management, and forced-stop semantics.