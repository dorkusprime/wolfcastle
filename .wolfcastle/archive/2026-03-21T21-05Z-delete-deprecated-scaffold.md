# Archive: delete-deprecated-scaffold

## Breadcrumbs

- **delete-deprecated-scaffold** [2026-03-21T02:02Z]: Created 2 children: verify-no-production-callers (leaf, 2 tasks), remove-deprecated-files (leaf, 3 tasks). Ordering: verification must complete before deletion proceeds. Key finding during planning: the scope lists 4 test files for deletion but the codebase contains 4 additional scaffold test files (scaffold_chmod_test.go, scaffold_coverage_test.go, scaffold_extra_test.go, scaffold_readme_test.go) that also call deprecated functions. Also, reinit_coverage_test.go appears to test ScaffoldService.Reinit (the replacement), not the deprecated Scaffold function -- the verification leaf will confirm before the deletion leaf acts on it.

## Audit

**Status:** passed

### Scope



### Escalations

- [OPEN] scaffold.go contains two non-deprecated production functions: WriteAuditTaskMD (called from cmd/project/create.go, cmd/audit/approve.go) and CreateProject (called from cmd/project/create.go, cmd/audit/approve.go). The sibling remove-deprecated-files node cannot delete scaffold.go wholesale; these functions must be extracted first or the file must be edited to remove only the deprecated code. (from delete-deprecated-scaffold/verify-no-production-callers)

## Metadata

| Field | Value |
|-------|-------|
| Node | delete-deprecated-scaffold |
| Completed | 2026-03-21T03:07Z |
| Archived | 2026-03-21T21:05Z |
| Engineer | wild-macbook-pro |
| Branch | feat/log-design |
