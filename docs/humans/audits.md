# Audits and Quality

## The Audit System

Every [leaf](how-it-works.md#the-project-tree) ends with an audit task. Auto-created. Cannot be moved. Cannot be deleted. Runs last, after every other task in the leaf has completed. Its job: verify that the work actually happened and actually works.

### Breadcrumbs

As tasks execute, they write timestamped breadcrumbs via `wolfcastle audit breadcrumb` (during the [record phase](how-it-works.md#execution-protocol) of execution). Each breadcrumb describes what was done, why, and what changed. These are not terse commit messages. They are rich, explanatory records: the raw material for verification.

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

### After Action Reviews

After Action Reviews (AARs) are structured post-mortems recorded when a task completes. Each AAR captures the objective, what actually happened, what went well, what could improve, and follow-up action items. The model writes them via [`wolfcastle audit aar`](cli/audit-aar.md).

AARs are richer than breadcrumbs. Breadcrumbs are timestamped notes written during execution. AARs are retrospectives written after. Both feed into the audit, but AARs also flow forward: the next task in the leaf receives the previous task's AAR as context, so lessons compound rather than evaporate.

When the audit task runs, it reads every AAR in the node alongside the breadcrumbs. Patterns across AARs (repeated improvement suggestions, recurring action items) signal systemic issues that the audit can escalate.

### Audit Reports

Audit reports are Markdown summaries generated when an audit completes. They contain the audit verdict (passed, failed, in progress), scope definitions, breadcrumb summaries, gap details, and escalation records. Reports are saved as files in the node's directory for permanent reference.

View a report with [`wolfcastle audit report`](cli/audit-report.md). If no report file exists yet, the command generates a preview from the current audit state. With `--path`, it prints just the file path for scripting.

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
