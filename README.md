# Wolfcastle

A model-agnostic autonomous project orchestrator. Wolfcastle breaks complex work into a persistent tree of projects, sub-projects, and tasks, then executes them through configurable multi-model pipelines.

## Status

Pre-alpha — architecture and design phase. See [docs/decisions/](docs/decisions/INDEX.md) for accepted ADRs.

## Core Concepts

- **Tree-structured work** — Orchestrator nodes (contain sub-projects) and Leaf nodes (contain tasks), traversed depth-first
- **JSON state, deterministic scripts** — All state mutations happen through validated scripts, never by the model directly
- **Three-tier configuration** — `base/` (Wolfcastle defaults) → `custom/` (team overrides) → `local/` (personal overrides)
- **Configurable pipelines** — Define multi-stage, multi-model workflows in JSON
- **Audit propagation** — Scoped verification at every tree level with upward gap escalation
- **Engineer-namespaced projects** — Multiple engineers work concurrently without merge conflicts

## Installation

Coming soon. Target distribution: `curl` installer, Homebrew tap, optional npm wrapper.

## Documentation

- [Architecture Decision Records](docs/decisions/INDEX.md)
- [Specs](docs/specs/README.md)
