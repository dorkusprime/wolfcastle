# ADR-031: In-Flight Specs with State-Based Linkage

## Status
Accepted

## Date
2026-03-13

## Context
Projects working on complex features need design specs that live above individual tasks. These specs need to be shared across engineers and machines, discoverable by both humans and models, but not all loaded into every model invocation's context.

We considered per-node spec directories, engineer-level directories, and the existing `docs/specs/` directory. Each had trade-offs around discoverability, isolation, and cross-machine access.

## Decision

### Specs Live in `docs/specs/`
All specs go in the single committed `docs/specs/` directory. They're flat, browsable, and shared across all engineers. Timestamp filenames (ADR-011) prevent conflicts.

### State Files Reference Relevant Specs
Each node's `state.json` has a `specs` array of filenames. This is the deterministic linkage between a node and its design context. Only referenced specs are injected into the model's context when working on that node.

### Commands Manage Linkage
- `wolfcastle spec create --node <path> "title"` — creates the spec file in `docs/specs/` AND adds the reference to the node's state
- `wolfcastle spec link --node <path> <filename>` — links an existing spec to a node
- `wolfcastle spec list [--node <path>]` — lists all specs, or specs linked to a specific node

### No Index File
There is no shared index file (avoids merge conflicts). `wolfcastle spec list` scans the directory and reads state files at runtime. Models use `wolfcastle spec list` as their index.

### Context Injection Strategy
The prompt tells the model:
- "These specs are assigned to your current task" — loaded directly in context
- "Other specs exist in `docs/specs/` — use standard tools (grep, find, etc.) to discover and read them if you need broader context"

The model pulls additional specs on demand. Context stays minimal by default.

### Cross-Node Sharing
Multiple nodes can reference the same spec file. A design spec for "new auth system" can be linked to both the implementation node and the testing node.

## Consequences
- Specs are always findable by humans (browse the directory)
- Context injection is deterministic and minimal (state references only)
- No merge conflicts on index files
- Cross-node and cross-engineer sharing works naturally
- Models have agency to explore beyond their assigned specs
- The directory may grow large but it's just Markdown files — cheap and browsable
- `wolfcastle spec list` gives models a runtime index without a file to maintain
