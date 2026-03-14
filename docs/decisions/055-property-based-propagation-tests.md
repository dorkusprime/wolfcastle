# ADR-055: Property-Based Propagation Tests

## Status
Accepted

## Date
2026-03-14

## Context
State propagation is the most critical invariant in Wolfcastle — a parent's state must always be derivable from its children's states, and the root index must always reflect the current state of every node. The current test suite verifies propagation with hand-crafted scenarios, but these cover a finite set of tree shapes and mutation sequences.

Given the centrality of propagation to system correctness, the testing strategy should go beyond example-based cases. Property-based tests that generate random tree mutations and verify consistency invariants always hold can catch edge cases that no developer would think to write — unusual tree shapes, rapid state oscillations, deep nesting, and interleaved mutations.

## Decision

Add property-based tests using Go's `testing/quick` package (stdlib, no external dependency) to verify propagation invariants:

### Invariants to Verify

1. **Parent-child consistency** — For every orchestrator, its state is derivable from its children's states using `RecomputeState`. No parent can be `complete` while any child is `not_started`, `in_progress`, or `blocked`.

2. **Root index consistency** — Every entry in the root index matches the actual state in the corresponding node's `state.json`. No dangling references, no stale states.

3. **Idempotency** — Propagating the same state twice produces the same result. `Propagate(addr, state, idx, load, save)` called twice is equivalent to calling it once.

4. **Monotonic completion** — Once a node transitions to `complete` via propagation, no child mutation (other than unblock) can change it back. A complete parent stays complete until explicit intervention.

5. **Depth consistency** — `DecompositionDepth` of every child equals its parent's depth + 1.

### Test Structure

```go
func TestPropagationInvariantsRandom(t *testing.T) {
    f := func(seed int64) bool {
        rng := rand.New(rand.NewSource(seed))

        // Generate a random tree (2-5 levels, 1-4 children per node)
        tree := generateRandomTree(rng, maxDepth, maxBranching)

        // Apply a random sequence of mutations (10-50 mutations)
        for i := 0; i < rng.Intn(40)+10; i++ {
            mutation := randomMutation(rng, tree)
            applyMutation(tree, mutation)
            propagateAll(tree)
        }

        // Verify all invariants
        return verifyParentChildConsistency(tree) &&
            verifyRootIndexConsistency(tree) &&
            verifyDepthConsistency(tree)
    }

    if err := quick.Check(f, &quick.Config{MaxCount: 500}); err != nil {
        t.Error(err)
    }
}
```

### Random Mutation Types

| Mutation | Effect |
|----------|--------|
| `ClaimTask` | not_started → in_progress |
| `CompleteTask` | in_progress → complete |
| `BlockTask` | in_progress → blocked |
| `UnblockTask` | blocked → not_started |
| `AddChild` | Add a new leaf node to a random orchestrator |
| `AddTask` | Add a task to a random leaf |

Each mutation respects preconditions (only claims not_started tasks, only completes in_progress tasks, etc.) — the test generates *valid* mutation sequences and verifies that propagation maintains consistency.

### In-Memory Tree

The property tests operate on an in-memory tree (no filesystem) using the same `state.Propagate` function with mock load/save callbacks that read/write from a `map[string]*state.NodeState`. This keeps the tests fast (500 iterations in under 1 second).

## Consequences
- Catches propagation edge cases that hand-crafted tests miss
- Runs fast enough for CI (in-memory, no disk I/O)
- Uses stdlib `testing/quick` — no external dependency
- The random tree generator is reusable for other property tests
- 500 iterations with random seeds provides high confidence without being slow
