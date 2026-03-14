# ADR-036: Summaries via Inline Marker, Not Separate Stage

## Status

Accepted

## Context

The spec requires node summaries — a recap of what was accomplished — stored in `Audit.ResultSummary`. The original design included a `runSummaryStage` that makes a separate model call using a cheaper/faster model to generate the summary from breadcrumbs and audit state.

All three implementations struggled with when to trigger this separate stage.

## Decision

Summaries are generated inline by the executing model, not by a separate stage.

When `BuildIterationContext` detects the current task is the last incomplete task in the node, it adds a "Summary Required" section to the prompt instructing the model to emit a `WOLFCASTLE_SUMMARY:` marker alongside `WOLFCASTLE_COMPLETE`.

The `runSummaryStage` method is retained as dead code for potential future use (e.g., backfilling summaries for nodes that completed without one) but is not called automatically.

## Consequences

- **No extra model call** — zero additional latency and cost.
- **Better context** — the model that did the work has richer context than a separate model working from breadcrumbs alone.
- **Simpler trigger logic** — no need to decide when to invoke a separate stage.
- **Trade-off** — if the model fails to emit the marker, the node completes without a summary. This is acceptable since summaries are informational, not structural.
