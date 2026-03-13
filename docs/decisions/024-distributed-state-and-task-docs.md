# ADR-024: Distributed State Files, Task Working Documents, and Runtime Aggregation

## Status
Accepted

## Date
2026-03-13

## Context
During the spec phase, the tree addressing spec chose a single `state.json` per engineer namespace. Review revealed two issues (recorded in ADR-023): (1) task descriptions should be richer than a JSON field allows, and (2) a single monolithic state file doesn't align well with the co-located project structure. Additionally, aggregating state across engineers for commands like `wolfcastle status` raised questions about shared vs. independent state.

## Decision

### Per-Node State Files
State is distributed as one `state.json` per node, co-located with the node's project definition and task documents:

```
.wolfcastle/projects/wild-macbook/
  state.json                          # Root index: tree structure, node registry
  attunement-tree/
    state.json                        # Orchestrator node state
    attunement-tree.md                # Project description
    fire-impl/
      state.json                      # Leaf node state (tasks, audit, failures)
      fire-impl.md                    # Project description
      task-3.md                       # Task working doc (model-created, optional)
```

Each node's `state.json` contains only that node's data (children list for orchestrators, task list for leaves, audit state, failure counters). The engineer's root `state.json` is a centralized index of the full tree structure for fast navigation.

### Task Descriptions: Hybrid JSON + Markdown
- **Brief description** lives in the leaf's `state.json` as a field (set by `wolfcastle task add`, script-managed)
- **Rich working document** is an optional companion Markdown file (e.g. `task-3.md`) that the model creates and updates during execution with findings, learnings, and context
- Go code controls context injection: the brief JSON description is always included; the Markdown file is included only for the active task to prevent runaway context

### No Shared Root Index
Each engineer maintains their own independent tree in their namespace. There is no shared state file across engineers.

- `wolfcastle status` shows only the current engineer's tree (default)
- `wolfcastle status --all` aggregates across all engineer directories at runtime
- No merge conflicts on state files, ever

### Project Discovery
When engineer A creates a project and commits the definition files (Markdown), engineer B gets them on pull but they don't automatically appear in B's state. Projects appear in an engineer's state when they start working on them, or `wolfcastle doctor` can detect orphaned definitions and offer to register them.

## Consequences
- Everything about a node is in one directory — state, description, task docs
- Root index enables fast navigation without filesystem walks
- Task working documents give humans visibility and models a native writing surface
- Context budget is controlled — only active task's Markdown is loaded
- Independent engineer trees eliminate all shared-state concerns
- Runtime aggregation for `--all` is slightly more work but avoids conflict entirely
