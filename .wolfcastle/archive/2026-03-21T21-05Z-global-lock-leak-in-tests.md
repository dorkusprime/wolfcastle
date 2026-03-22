# Archive: global-lock-leak-in-tests

## Breadcrumbs

- **global-lock-leak-in-tests** [2026-03-21T01:55Z]: Created 1 leaf child: fix-leaking-tests (4 tasks). Covers 5 leaking tests across 3 files: start_test.go (3 tests), start_coverage_test.go (1 test), start_deep_coverage_test.go (1 test). All need the same GlobalLockDir = t.TempDir() pattern already established in start_validation_test.go. No specs or ADRs needed; this is a mechanical fix applying an existing pattern.

## Audit

**Status:** passed

### Scope



### Escalations

- [OPEN] TestStartCmd_AlreadyRunning in cmd/daemon/daemon_test.go:672 reaches dmn.AcquireGlobalLock without redirecting GlobalLockDir, leaking the real daemon.lock during tests. Same pattern as the tests fixed in this node. (from global-lock-leak-in-tests/fix-leaking-tests)

## Metadata

| Field | Value |
|-------|-------|
| Node | global-lock-leak-in-tests |
| Completed | 2026-03-21T03:07Z |
| Archived | 2026-03-21T21:05Z |
| Engineer | wild-macbook-pro |
| Branch | feat/log-design |
