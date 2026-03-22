# Documentation Review

Comprehensive documentation review and update pass. Verify all docs/humans/ pages match current reality, all docs/specs/ are current or marked superseded, CONTRIBUTING.md package map is accurate, README feature list is complete, AGENTS.md critical rules are correct, docs/agents/architecture.md reflects current dependency graph.

Major changes since last review that must be reflected: config schema rewritten (stages as dict), 10+ new CLI commands (config show, audit aar, audit report), prompt subdirectory restructure, AARs and spec review as new pipeline concepts, status --detail flag, unknown-field detection, stall detector, selfHeal blocked audit remediation, CHILDREF_STATE_MISMATCH in doctor, archive lifecycle.

Each doc file should be audited independently: flag outdated content, update or supersede as appropriate.
