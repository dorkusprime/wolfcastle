# Archive: documentation-review

## Breadcrumbs

- **documentation-review** [2026-03-21T14:51Z]: Created 4 children: specs-currency-audit, agent-docs-audit, human-docs-audit, root-docs-audit. Ordering: specs first (foundational truth source), then agent docs (technical accuracy), then human docs (user-facing), then root docs (summary-level). Each leaf independently verifiable. Major drift risks identified: stages-as-dict in config spec, 5 missing packages in CONTRIBUTING.md, new CLI commands undocumented, auto-archive ADR not yet reflected in goroutine spec.

## Audit

**Status:** passed

### Scope



### Escalations

- [OPEN] findnexttask-invariants.md spec exists in docs/specs/ but is missing from the README index. Pre-existing omission, not introduced by this node. (from documentation-review/specs-currency-audit)
- [OPEN] PreStartSelfHeal is still referenced in docs/agents/state-and-types.md:82 and docs/agents/daemon.md:69-71 but the function does not exist in the codebase. These references were introduced by commits 51038fa and ebfc370 (a different node), not by root-docs-audit. (from documentation-review/root-docs-audit)
- [OPEN] internal/archive/service_test.go has a gofmt formatting violation (map literal alignment). Introduced by commit 2a31bf0 from a different node. (from documentation-review/root-docs-audit)

## Metadata

| Field | Value |
|-------|-------|
| Node | documentation-review |
| Completed | 2026-03-21T21:04Z |
| Archived | 2026-03-21T21:05Z |
| Engineer | wild-macbook-pro |
| Branch | feat/log-design |
