# Improvement Specification: Claude

Based on the comparative evaluation of the three Wolfcastle implementations, your implementation is the strongest architecturally and algorithmically. You clearly understand Go idioms, prioritizing isolated domain models, robust tests, and strict invariants. However, there are areas where you can learn from the other models to achieve 100% completeness on the CLI surface.

## 1. Complete the CLI Command Surface (Reference: Gemini)

**Issue:** While your core domain logic and validation engines are robust, your CLI surface area is incomplete or heavily stubbed for secondary commands compared to the specification's 21 commands.

**Actionable Advice:**
Look at **Gemini's** implementation in `../wolfcastle-gemini/internal/cli/`. Gemini successfully mapped out almost all 21 CLI commands using a clean, separated structure. 

Specifically, observe how Gemini organizes and implements its commands:
- `../wolfcastle-gemini/internal/cli/follow.go`
- `../wolfcastle-gemini/internal/cli/unblock.go`
- `../wolfcastle-gemini/internal/cli/audit.go`
- `../wolfcastle-gemini/internal/cli/navigate.go`
- `../wolfcastle-gemini/internal/cli/archive.go`
- `../wolfcastle-gemini/internal/cli/inbox.go`
- `../wolfcastle-gemini/internal/cli/project.go`
- `../wolfcastle-gemini/internal/cli/doctor.go`

Your current `cmd/` directory has many files, but some secondary commands (like deep navigation, full archive generation, and inbox management) remain stubbed or less fleshed out than Gemini's. You should fully implement the logic for these peripheral commands. You have the advantage of using your excellent `internal/state` and `internal/validate` packages as the backend, so wiring these commands up should be straightforward.

## 2. Maintain Your Strengths

- Do not change your `RecomputeState` logic in `internal/state/propagation.go`. It flawlessly handles the tricky "mixed blocked/not_started" state propagation case.
- Do not change your `internal/validate/engine.go` composable `Check` interface. It was evaluated as the best-in-class approach across all three models for building an extensible structural validation engine.
- Keep your excellent unit tests (`internal/state/propagation_test.go`, `internal/config/merge_test.go`, etc.).

By filling in the missing CLI implementation details using Gemini's structure as inspiration, your implementation will be flawless.