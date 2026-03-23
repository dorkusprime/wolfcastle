# ADR-059: Pre-Commit Hooks via .githooks/

**Status:** Accepted

**Date:** 2026-03-14

## Context

CI catches formatting, vet, and lint issues, but the feedback loop is slow: push, wait for CI, read the failure, fix, push again. Developers waste time on round-trips for issues that could be caught locally in seconds. A pre-commit hook provides instant feedback before the commit is even created.

The Go ecosystem has no dominant hook manager (unlike JavaScript's husky/lint-staged). Options include lefthook, pre-commit (Python), husky, or a plain shell script. Given that Wolfcastle has exactly one hook with four checks, a framework adds dependency overhead with no practical benefit.

## Decision

Use a plain shell script at `.githooks/pre-commit`, committed to the repo. Developers activate it with:

```
git config core.hooksPath .githooks
```

The hook runs four checks in order, failing fast on the first error:

1. **gofmt**: formatting check (instant)
2. **go vet**: static analysis (1-2 seconds)
3. **go build**: compilation check (2-3 seconds)
4. **golangci-lint**: full lint suite (5-10 seconds, skipped if not installed)

golangci-lint is optional because it requires a separate install. The first three checks use only the Go toolchain, which every developer already has.

The `core.hooksPath` approach was chosen over symlinks into `.git/hooks/` because it's a single command, works on all platforms, and doesn't require copying files on clone.

## Consequences

- Developers catch formatting and lint issues before they commit, not after CI runs.
- No new dependencies. The hook is a POSIX shell script.
- Opt-in: developers must run the git config command once after cloning. This is documented in the new engineer setup guide.
- The hook is committed to the repo, so updates propagate through git pull.
- Total hook runtime is under 15 seconds with golangci-lint, under 5 without.
