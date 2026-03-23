# ADR-047: Release Automation via GoReleaser

**Status:** Accepted

**Date:** 2026-03-14

## Context

The Makefile provides cross-compilation targets but no release automation.
Building, signing, checksumming, and publishing releases is a manual process.
For production readiness, release artifacts need to be consistently built,
cryptographically verifiable, and automatically published.

## Decision

Use GoReleaser for release automation (widely adopted in the Go ecosystem,
integrates with GitHub Actions):

1. **Release trigger.** Pushing a semver git tag (e.g., `v0.1.0`).
2. **Artifacts per release.** Compiled binaries for linux/amd64,
   linux/arm64, darwin/amd64, darwin/arm64, windows/amd64: each as a
   compressed tarball (tar.gz for Unix, zip for Windows).
3. **LDFLAGS.** Inject version, commit SHA, and build date into the binary
   (the `version` command already reads these).
4. **Checksums.** GoReleaser generates SHA256 checksums (checksums.txt) for
   all artifacts.
5. **GitHub Release.** Created automatically with artifacts attached and a
   changelog generated from conventional commit messages between tags.
6. **Configuration.** Lives in `.goreleaser.yml` at the project root.
7. **CI integration.** A separate GitHub Actions workflow
   (`.github/workflows/release.yml`) triggers on tag push, runs GoReleaser.
8. **No Homebrew tap initially.** Can be added later when there's enough
   user demand.
9. **No Docker images initially.** Wolfcastle is a CLI tool, not a service.
10. **Versioning.** Follows semver: major.minor.patch with pre-release
    suffixes (e.g., v0.1.0-alpha.1).
11. **Version command.** `wolfcastle version` displays the injected version,
    commit, and build date.

## Consequences

- Releases are one `git tag` + `git push` away: no manual build steps.
- Every release artifact has a verifiable checksum.
- Users can download platform-appropriate binaries from GitHub Releases.
- Changelog is generated automatically: no separate CHANGELOG.md to
  maintain.
- GoReleaser is well-maintained and widely used: minimal maintenance
  burden.
