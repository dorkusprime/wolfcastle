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
    go-version: ['1.26.x']
    os: [ubuntu-latest, macos-latest]
```

Test on both Linux and macOS to catch platform-specific issues (syscall behavior, signal handling, flock semantics). Windows is built but not tested — Wolfcastle's `SysProcAttr` usage requires platform-specific attention before Windows is a supported target.

### Job: `build-and-test`

Steps in order:

1. **Checkout** — `actions/checkout@v4`
2. **Setup Go** — `actions/setup-go@v5` with the matrix Go version
3. **Cache** — Go module and build cache (`~/go/pkg/mod`, `~/.cache/go-build`)
4. **Build** — `go build ./...`
5. **Vet** — `go vet ./...`
6. **Format check** — `gofmt -l . | tee /tmp/gofmt.out && test ! -s /tmp/gofmt.out` (fails if any file is unformatted)
7. **Unit tests** — `go test -race -coverprofile=coverage.out ./...`
8. **Integration tests** — `go test -race -tags integration ./test/integration/...`
9. **Upload coverage** — artifact upload for coverage.out (no external coverage service initially)
10. **Cross-compile check** — Build for all target platforms (compilation only, no execution):

```bash
GOOS=linux GOARCH=amd64 go build -o /dev/null ./...
GOOS=linux GOARCH=arm64 go build -o /dev/null ./...
GOOS=darwin GOARCH=amd64 go build -o /dev/null ./...
GOOS=darwin GOARCH=arm64 go build -o /dev/null ./...
GOOS=windows GOARCH=amd64 go build -o /dev/null ./...
```

### Job: `lint`

Runs in parallel with `build-and-test`:

1. **Checkout**
2. **Setup Go**
3. **golangci-lint** — `golangci/golangci-lint-action@v6` with version pinned in the workflow

### Quality Gates

All of the following must pass for a PR to be mergeable:

| Gate | Enforcement |
|------|-------------|
| Build | `go build ./...` exits 0 |
| Vet | `go vet ./...` exits 0 |
| Format | No files returned by `gofmt -l .` |
| Unit tests | All pass, race detector clean |
| Integration tests | All pass |
| Lint | golangci-lint exits 0 |
| Cross-compile | All target platforms compile |

### Performance Target

The full CI pipeline should complete in under 3 minutes for a typical change. If it exceeds 5 minutes, investigate — slow tests, unnecessary rebuilds, or cache misses.

---

## 3. Release Workflow (`release.yml`)

### Trigger

```yaml
on:
  push:
    tags: ['v*']
```

### Steps

1. **Checkout** — with `fetch-depth: 0` (GoReleaser needs full history for changelog)
2. **Setup Go**
3. **GoReleaser** — `goreleaser/goreleaser-action@v6` with `args: release --clean`

### GoReleaser Configuration (`.goreleaser.yml`)

```yaml
version: 2

builds:
  - main: .
    binary: wolfcastle
    env:
      - CGO_ENABLED=0
    goos:
      - linux
      - darwin
      - windows
    goarch:
      - amd64
      - arm64
    ldflags:
      - -s -w
      - -X github.com/dorkusprime/wolfcastle/cmd.version={{.Version}}
      - -X github.com/dorkusprime/wolfcastle/cmd.commit={{.Commit}}
      - -X github.com/dorkusprime/wolfcastle/cmd.date={{.Date}}

archives:
  - format: tar.gz
    format_overrides:
      - goos: windows
        format: zip
    name_template: "wolfcastle_{{ .Version }}_{{ .Os }}_{{ .Arch }}"

checksum:
  name_template: 'checksums.txt'

changelog:
  sort: asc
  filters:
    exclude:
      - '^docs:'
      - '^test:'
      - '^ci:'
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

The project starts at `v0.1.0` — major version 0 signals that the API is not yet stable.
