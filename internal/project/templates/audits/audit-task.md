# Audit

Verify all work in this node is complete and correct.

## Checklist

- [ ] All tasks marked complete actually did what they claimed
- [ ] Deliverables exist and contain meaningful content
- [ ] No files were left in a broken state
- [ ] Any validation commands pass
- [ ] Breadcrumbs describe what was done and why
- [ ] No gaps remain open
- [ ] Specs are in `.wolfcastle/docs/specs/` (not `docs/` or other locations). If a spec is in the wrong place, move it: create it via `wolfcastle spec create --body "content" --node <node>` and delete the misplaced file.
- [ ] Technology choices have ADRs in `.wolfcastle/docs/decisions/`. If a framework, library, or architecture was chosen without an ADR, create one via `wolfcastle adr create --stdin "Decision Title"`.
- [ ] Research documents are in `.wolfcastle/artifacts/` (not `docs/`). If research is in the wrong place, move it to `.wolfcastle/artifacts/` and update the deliverable path.
