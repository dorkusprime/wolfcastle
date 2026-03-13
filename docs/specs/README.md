# Specs

Living system specifications for Wolfcastle. These describe the current design and are the implementation reference.

## Specs

| Spec | Description |
|------|-------------|
| [State Machine](2026-03-12T00-00Z-state-machine.md) | Node lifecycle, four states, valid transitions, failure tracking, decomposition, state propagation across distributed files |
| [Config Schema](2026-03-12T00-01Z-config-schema.md) | Full JSON schema for config.json and config.local.json with defaults, merge semantics, doctor, audit, and overlap advisory |
| [Tree Addressing](2026-03-12T00-02Z-tree-addressing.md) | Address format, per-node filesystem mapping, root index, navigation algorithm, hybrid task descriptions |
| [Pipeline Stage Contract](2026-03-12T00-03Z-pipeline-stage-contract.md) | Stage definition, invocation, prompt assembly, default pipeline, error handling |
| [Audit Propagation](2026-03-12T00-04Z-audit-propagation.md) | Breadcrumbs, gaps, escalation mechanics, audit task invariant, scope definition |
| [Archive Format](2026-03-12T00-05Z-archive-format.md) | Entry template, filename convention, rollup process, summary stage |
| [CLI Commands](2026-03-12T00-06Z-cli-commands.md) | Detailed spec for all CLI commands including doctor, install, spec, inbox, status --all |
| [Orchestrator Prompt](2026-03-12T00-07Z-orchestrator-prompt.md) | Prompt assembly, phase structure, guardrails, terminal markers, per-stage differences |
| [Structural Validation](2026-03-13T00-00Z-structural-validation.md) | Validation engine, 17 issue types, severity levels, deterministic vs model-assisted fixes, Go API |

## Naming

Specs use ISO 8601 timestamp prefixes per [ADR-011](../decisions/011-timestamp-filenames-for-docs.md): `2026-03-12T18-45Z-title.md`.
