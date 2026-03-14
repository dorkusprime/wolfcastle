# ADR-058: Small Package Consolidation

## Status
Accepted

## Date
2026-03-14

## Context
The codebase has 21 packages. Three of them — `internal/inbox` (2 source files + 1 test), `internal/review` (2 source files + 1 test), and `internal/archive` (1 source file + 1 test) — are too small to justify their own package namespace. Each contains a types file and an I/O file, totaling roughly 150-250 lines of source per package. They exist as separate packages not because they have complex internal APIs to encapsulate, but because they were created to hold domain-adjacent types during the initial package decomposition.

The cognitive overhead of 21 packages is real: a newcomer must understand the package graph before contributing, and Go's strict import rules mean that moving a type between packages is a ripple-effect refactor. Reducing the package count where separation isn't earning its keep simplifies onboarding and reduces import churn.

## Decision

### Option A: Merge into `internal/subsystems`

Create a single `internal/subsystems` package containing all three domains. This keeps them out of `internal/state` (which is already the largest internal package) and gives them a shared home without conflating their types with state management.

**Pros:** Clean separation from state; one new import path replaces three.
**Cons:** "subsystems" is a vague package name; the three domains have no real cohesion beyond being small.

### Option B: Merge inbox and review into `internal/state`, keep archive separate

Inbox items and review batches are state-adjacent — they're JSON files read/written by the daemon alongside node state, and they follow the same I/O patterns (load, save, atomic write). Archive rollup is different: it reads state but generates Markdown output, and its rollup logic has distinct dependencies (`config` for identity, `git` for metadata). Keeping archive as its own package respects this boundary.

**Pros:** inbox and review types live alongside the state types they're most associated with; archive retains its distinct identity; package names remain meaningful.
**Cons:** `internal/state` grows by ~400 lines (from ~700 to ~1100), but remains well within manageable size.

### Chosen: Option B

Merge `internal/inbox` and `internal/review` into `internal/state`. Keep `internal/archive` as-is.

Rationale: Option B reduces the package count from 21 to 19 while preserving meaningful package boundaries. The "subsystems" name in Option A is too vague — it would become a dumping ground for any package deemed "too small." Option B's criterion is clear: state-adjacent I/O types belong in the state package, rollup logic does not.

### Migration

| Current | After | Files moved |
|---------|-------|-------------|
| `internal/inbox/types.go` | `internal/state/inbox_types.go` | Types: `InboxData`, `Item` |
| `internal/inbox/io.go` | `internal/state/inbox_io.go` | Functions: `Load`, `Save` (renamed to `LoadInbox`, `SaveInbox` to avoid collision) |
| `internal/inbox/io_test.go` | `internal/state/inbox_io_test.go` | Tests |
| `internal/review/types.go` | `internal/state/review_types.go` | Types: `Batch`, `Finding`, `Decision`, `HistoryEntry`, constants |
| `internal/review/io.go` | `internal/state/review_io.go` | Functions: `LoadBatch`, `SaveBatch`, `LoadHistory`, `SaveHistory`, `RemoveBatch`, `EnforceRetention` |
| `internal/review/io_test.go` | `internal/state/review_io_test.go` | Tests |
| `internal/archive/` | (unchanged) | — |

### Import Updates

All importers change from `inbox.Load(...)` to `state.LoadInbox(...)` and from `review.LoadBatch(...)` to `state.LoadBatch(...)`. The affected files:

- `internal/daemon/daemon.go` — `inbox.Load` → `state.LoadInbox`
- `cmd/inbox/add.go`, `cmd/inbox/list.go`, `cmd/inbox/clear.go` — `inbox.*` → `state.*`
- `cmd/audit/approve.go`, `cmd/audit/reject.go`, `cmd/audit/codebase.go`, `cmd/audit/pending.go`, `cmd/audit/history.go` — `review.*` → `state.*`

## Consequences
- Package count drops from 21 to 19
- Two import paths eliminated, reducing the package graph's surface area
- `internal/state` becomes the comprehensive home for all persistent data types and I/O — node state, root index, inbox, and review batches
- File prefixes (`inbox_`, `review_`) keep the merged files navigable within the state package
- `internal/archive` retains its distinct identity and rollup logic
- Function renames (`Load` → `LoadInbox`) prevent name collisions and improve clarity at call sites
