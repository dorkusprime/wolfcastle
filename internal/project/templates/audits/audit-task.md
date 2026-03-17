# Audit

Verify all work in this node is complete and correct.

## Checklist

- [ ] All tasks marked complete actually did what they claimed
- [ ] Deliverables exist and contain meaningful content
- [ ] No files were left in a broken state
- [ ] Any validation commands pass
- [ ] Breadcrumbs describe what was done and why
- [ ] No gaps remain open
- [ ] Specs are in `.wolfcastle/docs/specs/` (not `docs/` or other locations). If a spec is in the wrong place, read its content, then run `wolfcastle spec create "Spec Title" --body "content" --node <node>`, then delete the misplaced file.
- [ ] **Decisions are documented.** Read the code changes. Identify decisions where alternatives existed: a library was chosen over another, an interface was defined instead of a concrete type, a concurrency strategy was selected, a structural pattern was adopted. For each such decision, check whether an ADR exists in `.wolfcastle/docs/decisions/`. If a non-trivial decision is undocumented, the audit verdict is REMEDIATE. A "non-trivial decision" is one where a reasonable developer might have chosen differently. Do not flag forced choices or standard patterns.
- [ ] **Contracts are specified.** If a task created an interface or a type that other packages depend on, a spec should exist in `.wolfcastle/docs/specs/` describing the contract. If missing for a public interface, verdict is REMEDIATE.
- [ ] Research documents are in `.wolfcastle/artifacts/` (not `docs/`). If research is in the wrong place, move it to `.wolfcastle/artifacts/` and update the deliverable path.
