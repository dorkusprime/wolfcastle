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
- [ ] **MANDATORY**: Technology choices have ADRs. If any task in this leaf introduced a new package, chose a concurrency strategy (mutex, channel, atomic), defined an interface, selected an architecture pattern, or rejected an alternative, an ADR MUST exist. If it does not, the audit verdict is REMEDIATE. List each missing ADR as a finding: "Missing ADR: [decision description]". Create the ADR as a remediation task.
- [ ] **MANDATORY**: New packages with interfaces have specs. If a task created a package that exports an interface or a type other packages depend on, a spec MUST exist describing the contract. If missing, verdict is REMEDIATE.
- [ ] Research documents are in `.wolfcastle/artifacts/` (not `docs/`). If research is in the wrong place, move it to `.wolfcastle/artifacts/` and update the deliverable path.
