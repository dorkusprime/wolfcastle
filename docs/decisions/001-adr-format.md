# ADR-001: Architecture Decision Record Format

## Status
Accepted

## Date
2026-03-12

## Context
Wolfcastle needs to track architectural decisions both for its own development and as a first-class feature for projects that use it. We need a lightweight, consistent format that is easy for both humans and AI models to read and reference.

## Decision
We adopt a simplified ADR format with these sections:
- **Status**: Accepted, Superseded, or Deprecated
- **Date**: When the decision was made
- **Context**: Why the decision was needed
- **Decision**: What was decided and why
- **Consequences**: What follows from this decision

ADRs are numbered sequentially (`NNN-slug.md`) and indexed in `docs/decisions/INDEX.md`.

## Consequences
- All architectural decisions for Wolfcastle itself are recorded here
- Wolfcastle-the-system will support ADRs as a configurable feature for user projects, using this same format as the default
- Future conversations have full reasoning context without relying on memory
