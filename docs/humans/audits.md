# Audits and Quality

## The Audit System

Every [leaf](how-it-works.md#the-project-tree) ends with an audit task. Auto-created. Cannot be moved. Cannot be deleted. Runs last, after every other task in the leaf has completed. Its job: verify that the work actually happened and actually works.

### Breadcrumbs

As tasks execute, they write timestamped breadcrumbs via `wolfcastle audit breadcrumb` (during the [record phase](how-it-works.md#seven-phase-execution) of execution). Each breadcrumb describes what was done, why, and what changed. These are not terse commit messages. They are rich, explanatory records: the raw material for verification.

### Audit Execution

The audit task reviews all breadcrumbs against the leaf's defined criteria:

- Did the implementation match the requirements?
- Are there gaps between what was planned and what was done?
- Do the validation results confirm the work?

### Gap Escalation

If the audit finds gaps it cannot resolve locally, it escalates them upward to the parent [orchestrator](how-it-works.md#the-project-tree) via `wolfcastle audit escalate`. The parent's audit scope now includes cross-cutting verification of those gaps. Escalation can propagate all the way to the root if necessary.

### Audit Status

| Status | Meaning |
|--------|---------|
| `pending` | Audit has not started. |
| `in_progress` | Audit is running. |
| `passed` | All criteria met. No gaps. |
| `failed` | Gaps found. Escalation may follow. |

Gaps are tracked individually with deterministic IDs, open/fixed status, and full traceability.

## Codebase Audit

A standalone command for auditing your codebase against composable, discoverable scopes:

```
wolfcastle audit run                              # all scopes
wolfcastle audit run --scope dry,modularity       # specific scopes
wolfcastle audit list                             # show available scopes
```

Strictly read-only. The model reads your code, analyzes it against the requested scopes, and produces a Markdown report. It does not modify files, create branches, or touch your codebase. The only output is the report.

### Scopes

Scopes are enum-like IDs backed by prompt fragments. Base scopes ship with Wolfcastle (`dry`, `modularity`, `decomposition`, `comments`, etc.). Add custom scopes in `custom/audits/` or personal scopes in `local/audits/`. All [three tiers](configuration.md#three-tiers) are discovered at runtime.

### The Approval Gate

Audit findings do not become tasks automatically. The model generates prioritized findings in its report. You review them. Approve all, review individually, or reject all. Approved findings become projects and tasks in your [project tree](how-it-works.md#the-project-tree). Rejected findings disappear. Nothing changes until you say so.

## Structural Validation

The validation engine checks the entire [distributed state tree](how-it-works.md#distributed-state) for consistency. It classifies 17 distinct issue types by severity:

- **9 deterministic fixes**: Missing audit task, stale index entry, orphaned files. Go code fixes these directly.
- **5 ambiguous fixes**: Conflicting state, unclear intent. A configurable model reasons about the fix with strict guardrails.
- **1 daemon [self-healing](failure-and-recovery.md#self-healing)**: Crash recovery, handled on next startup.
- **1 manual**: Requires human judgment.
- **1 cross-engineer**: Overlap or conflict across [namespaces](collaboration.md#engineer-namespacing).

### wolfcastle doctor

Interactive validation and repair:

```
wolfcastle doctor
```

Scans the tree. Reports findings with locations and severity. You choose: fix all, fix selected, or abort. Deterministic fixes are applied by Go code. Ambiguous fixes are reasoned by a [model you configure](configuration.md#models):

```json
{
  "doctor": {
    "model": "mid",
    "prompt_file": "doctor.md"
  }
}
```

The validation engine also runs a subset of checks on [daemon](how-it-works.md#the-daemon) startup. If the tree is corrupted, the daemon refuses to start.
