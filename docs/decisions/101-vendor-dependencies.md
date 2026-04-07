# Vendor all dependencies

## Status
Accepted

## Date
2026-04-06

## Context

Wolfcastle's daemon runs with the user's shell permissions and has access to environment variables, including API keys and credentials. A compromised upstream dependency could inject code through a routine `go mod download`. The project has a minimal dependency surface (3 direct, 6 total modules), and adding Bubbletea for the TUI will expand that.

## Options Considered

1. **Normal module resolution.** Dependencies fetched from the module proxy on build. Simple, conventional. Vulnerable to upstream compromise or disappearance.

2. **Vendor all dependencies.** `go mod vendor` copies all dependency source into the repo. Builds use the vendored copy. Dependencies are auditable, pinned at the source level, and available offline.

## Decision

Vendor all dependencies. The `vendor/` directory is committed to the repo. `go mod vendor` is run after any dependency change. The `.gitignore` does not exclude `vendor/`.

## Consequences

- Builds require no network access and no module proxy trust.
- Dependency updates produce large diffs in the vendor directory. This is acceptable; the diff is the audit trail.
- The repo size increases by the total size of vendored source (currently small; will grow with Bubbletea's ecosystem).
- CI builds from the vendored source, matching what contributors build locally.
