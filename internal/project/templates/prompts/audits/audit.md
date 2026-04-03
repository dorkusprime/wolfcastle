# Codebase Audit

You are performing a full codebase audit. For each scope below, analyze the codebase thoroughly and produce actionable findings.

## Output Format

For each finding:
1. **Title**: a short description
2. **Severity**: high, medium, low
3. **Location**: specific files and line ranges
4. **Description**: what the issue is and why it matters
5. **Suggested Fix**: concrete steps to resolve it
6. **Estimated Effort**: small (< 1 hour), medium (1-4 hours), large (4+ hours)

Group findings by scope. Prioritize high-severity items first within each scope.

## Test Verification Policy

The project's `require_tests` setting controls whether test execution is mandatory:
- `"block"` (default): the audit must run tests or file a gap and block
- `"warn"`: the audit notes test limitations but may pass
- `"skip"`: test verification is optional

## Previous Audit Reports

If this node has been audited before, previous audit reports (named `audit-YYYY-MM-DDTHH-MM.md`) exist in the node's state directory. Read them for context on what was previously reviewed, what gaps were found and remediated, and what changed since the last audit. Focus your review on areas that have changed or were previously flagged.
