# ADR-061: MIT License

**Status:** Accepted

**Date:** 2026-03-14

## Context

The project had no license file, which under copyright law means no one can legally use, modify, or distribute the code. A license is required before any public release.

The two most common choices for Go CLI tools are MIT and Apache 2.0. Both are permissive. Apache 2.0 adds an explicit patent grant and requires attribution in derivative works. MIT is simpler and shorter.

## Decision

MIT. It is the most widely used license in the Go ecosystem (Cobra, Gin, Hugo, Bubbletea all use MIT), has no patent clause complexity, and imposes minimal obligations on users.

## Consequences

- Anyone can use, modify, and distribute Wolfcastle with no restrictions beyond preserving the copyright notice.
- No patent grant. This is acceptable for a CLI tool that does not implement patentable algorithms.
- The LICENSE file lives at the repo root and is detected by GitHub, pkg.go.dev, and license badge services.
