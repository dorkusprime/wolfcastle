# ADR-049: Lint Policy via golangci-lint

**Status:** Accepted

**Date:** 2026-03-14

## Context

The current code quality bar is `go vet` + `gofmt`: both catch real issues
but represent a minimal surface. Go has a rich ecosystem of static analysis
tools (errcheck, ineffassign, staticcheck, gosimple, gocritic, etc.) that
catch bugs, performance issues, and style problems that vet alone misses.
Without a linter configuration, each developer makes their own judgment about
code quality, leading to inconsistency.

## Decision

Adopt `golangci-lint` as the project linter, configured via `.golangci.yml`
at the project root:

1. **Enabled linters** (conservative, high-signal set):
   - `errcheck`: unchecked error returns (the single most common Go bug).
   - `ineffassign`: assignments to variables that are never used.
   - `staticcheck`: the gold standard Go static analyzer.
   - `gosimple`: simplifications (e.g., `if x == true` to `if x`).
   - `govet`: already used, included for completeness.
   - `unused`: unused code detection.
   - `gofmt`: formatting check (already enforced, included for
     completeness).
   - `misspell`: typos in comments and strings.
   - `nolintlint`: ensures //nolint directives have justifications.
2. **Disabled** (explicitly, with comments explaining why):
   - `gocritic`: too opinionated, generates noise.
   - `lll`: line length limits are counterproductive for Go.
   - `wsl`: whitespace linting is too aggressive.
   - `funlen`: function length limits are context-dependent.
   - `gocognit`: cognitive complexity limits are too simplistic.
3. **//nolint directives.** Allowed but require a justification comment
   (enforced by nolintlint).
4. **CI integration.** golangci-lint runs in CI as part of the pipeline
   (ADR-043): a lint failure fails the build.
5. **Local development.** golangci-lint is NOT a hard requirement for local
   development: developers can run it optionally, but CI enforces it.
6. **Version.** Pinned in CI to prevent surprise breakage from linter
   updates.
7. **Philosophy.** Configuration is intentionally conservative: we'd
   rather have a small set of high-signal linters than a large set that
   generates false positives and encourages //nolint proliferation.

## Consequences

- Catches the most common classes of Go bugs (unchecked errors, unused
  variables, inefficient patterns) automatically.
- Consistent code quality across all contributors.
- //nolint with justification prevents both suppression abuse and
  false-positive frustration.
- Conservative linter set means almost no false positives: developers
  trust the linter output.
- CI enforcement means lint issues are caught before merge, not in code
  review.
