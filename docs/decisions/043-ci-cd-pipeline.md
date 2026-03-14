# ADR-043: CI/CD Pipeline and Quality Gates

**Status:** Accepted

**Date:** 2026-03-14

## Context

Wolfcastle has no CI/CD pipeline. Code quality is enforced manually
(developer runs `go vet`, `gofmt`, `go test`). This is acceptable for a
single developer but fragile — a single missed `gofmt` or a broken test can
merge unnoticed. As the project approaches production, automated quality
gates become non-negotiable.

## Decision

Adopt GitHub Actions as the CI platform, matching the GitHub-hosted
repository:

1. **Trigger.** Push to any branch, pull request to `main`.
2. **Pipeline stages** (in order): checkout → Go setup → build → vet →
   gofmt check → test → lint (`golangci-lint`, see ADR-049) →
   cross-compile verification.
3. **Go version matrix.** Test against the minimum supported version
   (from `go.mod`) and latest stable.
4. **Test stage.** Runs `go test -race -coverprofile ./...` — race detector
   on, coverage collected.
5. **Coverage.** Reported but not gated (no minimum threshold yet — see
   ADR-044 for the plan).
6. **gofmt check.** Hard gate — any unformatted file fails the pipeline.
7. **Cross-compilation check.** Build `linux/amd64`, `linux/arm64`,
   `darwin/amd64`, `darwin/arm64` to catch platform-specific compilation
   errors.
8. **No deployment stage.** Release automation is handled separately
   (ADR-047).
9. **Pipeline definition.** Lives in `.github/workflows/ci.yml`.
10. **README badge.** Build status badge in `README.md` signals project
    health to contributors.

## Consequences

- Every push and PR gets automated quality verification.
- Race conditions are caught before they reach production.
- Cross-platform builds are verified without manual testing.
- Developers get fast feedback on breakage (target: pipeline completes in
  under 3 minutes).
- README badge signals project health to contributors.
