# CI/CD Pipeline

This spec defines the GitHub Actions CI pipeline for Wolfcastle, covering workflow triggers, job stages, quality gates, and the release workflow. It is the implementation reference for ADR-043 and ADR-047.

## Governing ADRs

- ADR-043: CI/CD Pipeline and Quality Gates
- ADR-047: Release Automation via GoReleaser
- ADR-049: Lint Policy via golangci-lint

---

## 1. Workflow Files

Two workflow files live in `.github/workflows/`:

| File | Trigger | Purpose |
|------|---------|---------|
| `ci.yml` | Push to any branch, PR to main | Build, test, lint, cross-compile |
| `release.yml` | Push of a semver tag (`v*`) | Build release artifacts via GoReleaser |

---

## 2. CI Workflow (`ci.yml`)

### Trigger

```yaml
on:
  push:
    branches: ['**']
  pull_request:
    branches: [main]
```

### Go Version Matrix

```yaml
strategy:
  matrix:
    go-version: ['1.26.x', 'stable']
```

Tests run on `ubuntu-latest` only. Cross-compilation to darwin and windows is verified in a separate job without execution.

### Job: `build-and-test`

Steps in order:

1. **Checkout**: `actions/checkout@v4`
2. **Setup Go**: `actions/setup-go@v5` with the matrix Go version
3. **Build**: `go build -trimpath ./...`
4. **Vet**: `go vet ./...`
5. **Format check**: fails if any file is unformatted
6. **Unit tests**: `go test -race -coverprofile=coverage.out ./...`
7. **Upload coverage**: artifact upload for coverage.out (stable Go version only)

### Job: `cross-compile`

Separate job that builds for all target platforms (compilation only, no execution):

```bash
GOOS=linux GOARCH=amd64 go build -trimpath ./...
GOOS=linux GOARCH=arm64 go build -trimpath ./...
GOOS=darwin GOARCH=amd64 go build -trimpath ./...
GOOS=darwin GOARCH=arm64 go build -trimpath ./...
GOOS=windows GOARCH=amd64 go build -trimpath ./...
```

### Job: `smoke-tests`

Runs smoke tests with the `smoke` build tag: `go test -tags smoke -v ./test/smoke/...`

### Job: `integration-tests`

Runs integration tests with the `integration` build tag: `go test -tags integration -v -timeout 120s ./test/integration/...`

### Job: `lint`

Runs in parallel with `build-and-test`:

1. **Checkout**
2. **Setup Go**
3. **golangci-lint**: `golangci/golangci-lint-action@v6` with version pinned in the workflow

### Quality Gates

All of the following must pass for a PR to be mergeable:

| Gate | Enforcement |
|------|-------------|
| Build | `go build -trimpath ./...` exits 0 |
| Vet | `go vet ./...` exits 0 |
| Format | No files returned by `gofmt -l .` |
| Unit tests | All pass, race detector clean |
| Smoke tests | All pass |
| Integration tests | All pass |
| Lint | golangci-lint exits 0 |
| Cross-compile | All target platforms compile |

### Performance Target

The full CI pipeline should complete in under 3 minutes for a typical change. If it exceeds 5 minutes, investigate: slow tests, unnecessary rebuilds, or cache misses.

---

## 3. Release Workflow (`release.yml`)

### Trigger

```yaml
on:
  push:
    tags: ['v*']
```

### Steps

1. **Checkout**: with `fetch-depth: 0` (GoReleaser needs full history for changelog)
2. **Setup Go**
3. **GoReleaser**: `goreleaser/goreleaser-action@v6` with `args: release --clean`

### GoReleaser Configuration (`.goreleaser.yml`)

```yaml
version: 2

builds:
  - main: .
    binary: wolfcastle
    flags:
      - -trimpath
    goos:
      - linux
      - darwin
      - windows
    goarch:
      - amd64
      - arm64
    ignore:
      - goos: windows
        goarch: arm64
    ldflags:
      - -s -w
      - -X github.com/dorkusprime/wolfcastle/cmd.Version={{.Version}}
      - -X github.com/dorkusprime/wolfcastle/cmd.Commit={{.ShortCommit}}
      - -X github.com/dorkusprime/wolfcastle/cmd.Date={{.Date}}

archives:
  - formats:
      - tar.gz
    format_overrides:
      - goos: windows
        formats:
          - zip
    name_template: "{{ .ProjectName }}_{{ .Version }}_{{ .Os }}_{{ .Arch }}"

checksum:
  name_template: checksums.txt
  algorithm: sha256

changelog:
  sort: asc
  filters:
    exclude:
      - '^docs:'
      - '^test:'
      - '^ci:'
      - '^chore:'
```

### Release Artifacts

Each release produces:

| Artifact | Description |
|----------|-------------|
| `wolfcastle_{version}_{os}_{arch}.tar.gz` | Compiled binary (Unix) |
| `wolfcastle_{version}_windows_amd64.zip` | Compiled binary (Windows) |
| `checksums.txt` | SHA256 checksums for all artifacts |

### Version Injection

The binary reads version info from variables injected at build time via LDFLAGS. The `wolfcastle version` command displays:

```
wolfcastle v0.1.0 (commit: a1b2c3d, built: 2026-03-14T18:00:00Z)
```

---

## 4. Branch Protection

Recommended GitHub branch protection rules for `main`:

- Require status checks to pass (build-and-test, lint)
- Require branches to be up to date before merging
- Require pull request reviews (1 reviewer minimum)
- No force pushes
- No branch deletion

---

## 5. Versioning

Wolfcastle follows semantic versioning (semver):

- **Major** (1.0.0): Breaking changes to the CLI interface, config schema, or state file format
- **Minor** (0.x.0): New features, new commands, non-breaking config additions
- **Patch** (0.0.x): Bug fixes, documentation updates, dependency bumps

Pre-release versions use suffixes: `v0.1.0-alpha.1`, `v0.1.0-beta.1`, `v0.1.0-rc.1`.

The project starts at `v0.1.0`: major version 0 signals that the API is not yet stable.
