# ADR-035: Model-Driven Decomposition via CLI

## Status

Accepted

## Date

2026-03-13

## Context

When a task's failure count reaches the decomposition threshold and the node's decomposition depth is below the maximum, the system needs to break the failing leaf into smaller subtasks. Three approaches were considered:

1. **Code-driven decomposition**. The daemon invokes a separate model call to generate a decomposition plan, then programmatically restructures the tree.
2. **Marker-driven decomposition**. The model emits a custom `WOLFCASTLE_DECOMPOSE` marker with subtask definitions that the daemon parses and executes.
3. **CLI-driven decomposition**. The model uses existing `wolfcastle project create` and `wolfcastle task add` CLI commands directly.

## Decision

Use approach 3: model-driven decomposition via the CLI.

The model already has tool access and can run CLI commands. When the decomposition threshold is reached, `BuildIterationContext` includes guidance telling the model to use `wolfcastle project create --node <parent>` and `wolfcastle task add --node <child>` to restructure the tree.

The `project create` command auto-promotes a leaf parent to an orchestrator when children are created under it, eliminating the chicken-and-egg problem.

## Consequences

- **No new markers or parsers needed**: decomposition uses the same CLI infrastructure as manual operations.
- **Model makes all judgment calls**: only the model understands the domain context well enough to decide subtask boundaries.
- **Daemon remains mechanical**: it flags `NeedsDecomposition` and includes prompt guidance; the model executes.
- **CLI must handle leaf→orchestrator promotion**: `project create` now auto-converts a leaf parent when creating children under it.
