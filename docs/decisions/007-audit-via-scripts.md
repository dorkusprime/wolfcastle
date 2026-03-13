# ADR-007: Audit Model Preserved, Mechanics via Scripts

## Status
Accepted

## Date
2026-03-12

## Context
Ralph's audit propagation model is one of its strongest features: every node has an audit scope, leaf audits verify the node's work, orchestrator audits verify integration, and gaps escalate upward. However, in Ralph the model directly edited audit.md files.

## Decision
The audit *concept* carries forward unchanged — scoped verification at every tree level with upward gap escalation. The *mechanics* move to deterministic scripts operating on JSON state: claiming audit tasks, recording breadcrumbs, escalating gaps, and marking audit completion are all script operations.

## Consequences
- Audit state is JSON, consistent with ADR-002
- Scripts enforce audit invariants (e.g. audit task always last, breadcrumbs append-only)
- The model decides what to audit and what gaps exist; scripts record it reliably
- Audit reports can be generated deterministically from state
