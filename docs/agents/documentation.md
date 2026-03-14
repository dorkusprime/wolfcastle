# Documentation

## Documentation Hierarchy

1. **ADRs** (Architecture Decision Records) — the authoritative record of *why* a design choice was made. Found in `docs/decisions/`. ADRs override specs when there's a conflict.
2. **Specs** — detailed implementation reference for major subsystems. Found in `docs/specs/`. Specs describe *how* things work and must be updated when behavior changes.
3. **AGENTS.md** — agent-facing guidance. Found in `docs/agents/` with a top-level index at `AGENTS.md`.
4. **Code doc comments** — Go doc comments on packages, types, and functions.

## ADR Conventions

- Numbered sequentially: `001-adr-format.md`, `002-json-config-and-state.md`, etc.
- Indexed in `docs/decisions/INDEX.md` — update this when adding an ADR
- An ADR is never deleted — if superseded, add a note at the top referencing the new ADR
- ADR format follows ADR-001

## Spec Conventions

- Timestamped filenames: `2026-03-12T00-00Z-state-machine.md`
- Each spec references governing ADRs at the top
- Indexed in `docs/specs/README.md`
- **Specs must track implementation.** If you change behavior, update the spec. A spec that describes a design that was never implemented (or was changed post-implementation) is a bug.

## ADR-036 Summary Change

The original specs (pipeline-stage-contract, archive-format) described the summary as a separate pipeline stage. ADR-036 changed this to inline generation via `WOLFCASTLE_SUMMARY:` marker. The specs have been updated to reflect this.

## Code Doc Comments

- Every exported function, type, method, and constant must have a doc comment
- Every package must have a package-level doc comment (on at least one file)
- Doc comments should explain *what* and *why*, not *how* (the code shows how)
- Use Go convention: `// FunctionName does X.` (starts with the name)

## When to Write What

| Change | Update |
|--------|--------|
| New design decision | ADR |
| New subsystem or major feature | Spec + ADR if design is novel |
| Behavioral change | Spec (and ADR if it's a design reversal) |
| New package or exported API | Code doc comments |
| Bug fix | Nothing (the fix is in the code, the commit message has context) |
| New CLI command | Spec (cli-commands), code doc comments, scriptref.go |
