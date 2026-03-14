# Test Strategy and Coverage

This spec defines the three-tier test strategy for Wolfcastle, covering unit tests, integration tests, and smoke tests. It details test infrastructure, coverage targets, and patterns for each tier. It is the implementation reference for ADR-044.

## Governing ADRs

- ADR-044: Test Strategy — Unit, Integration, and Smoke
- ADR-032: Go Project Structure and Cobra CLI Framework
- ADR-043: CI/CD Pipeline and Quality Gates

---

## 1. Test Tiers

| Tier | Location | Build tag | Runs in CI | Runs locally | Target |
|------|----------|-----------|-----------|-------------|--------|
| Unit | `*_test.go` alongside source | (none) | Always | `go test ./...` | Individual functions and packages |
| Integration | `test/integration/` | `integration` | Always | `go test -tags integration ./test/integration/...` | Full command sequences against a real .wolfcastle/ |
| Smoke | `test/smoke/` | `smoke` | Always | `go test -tags smoke ./test/smoke/...` | Binary compiles and runs basic commands |

---

## 2. Tier 1: Unit Tests

### Existing Coverage

These packages have strong unit test coverage and serve as the model for new tests:

| Package | Test files | Coverage area |
|---------|-----------|---------------|
| `internal/state` | 10 files | Mutations, navigation, propagation, I/O, audit lifecycle |
| `internal/tree` | 2 files | Address parsing, resolution |
| `internal/config` | 2 files | Loading, merging, validation |
| `internal/pipeline` | 3 files | Prompt assembly, fragments, context building |
| `internal/validate` | 3 files | Validation engine, auto-fix, categories |
| `internal/archive` | 1 file | Rollup generation |
| `internal/project` | 2 files | Scaffolding, project creation |
| `internal/inbox` | 1 file | I/O operations |
| `internal/review` | 1 file | Batch I/O |
| `internal/logging` | 1 file | Logger operations |
| `internal/output` | 2 files | Envelope formatting, printing |

### Coverage Gaps to Fill

#### `internal/invoke` (Priority: High)

The invoker handles subprocess execution, streaming, process groups, and exit code extraction. Test strategy:

1. **Test binary approach.** Build a small Go program in `internal/invoke/testdata/testcli/main.go` that:
   - Reads stdin and echoes it to stdout (with optional prefix)
   - Exits with a configurable exit code (via environment variable or flag)
   - Optionally writes to stderr
   - Optionally sleeps (for timeout testing)
   - Optionally produces very long lines (for buffer limit testing)

2. **Test cases:**
   - Successful invocation: stdin piped, stdout captured, exit 0
   - Non-zero exit: ExitError captured, exit code extracted
   - Streaming mode: logWriter receives lines as they're produced
   - Buffer limit: lines exceeding 1MB are handled (scanner buffer)
   - Context cancellation: command is killed when context expires
   - Empty output: model produces no stdout
   - Stderr capture: stderr is captured separately from stdout

3. **Build the test binary in TestMain:**

```go
func TestMain(m *testing.M) {
    // Build test CLI binary
    cmd := exec.Command("go", "build", "-o", filepath.Join(os.TempDir(), "testcli"), "./testdata/testcli")
    if err := cmd.Run(); err != nil {
        panic(err)
    }
    os.Exit(m.Run())
}
```

#### `cmd/cmdutil` (Priority: Medium)

- `App.FindWolfcastleDir()` — test upward directory walk, missing .wolfcastle/
- `App.LoadConfig()` — test with valid config, missing config, resolver failure
- `CompleteNodeAddresses()` — test completion against a seeded root index
- Overlap detection functions — `tokenize`, `bigrams`, `jaccardSimilarity` (pure functions, easy to test)

#### `internal/daemon` (Priority: Medium)

The daemon has a test file but only tests helper functions. Add:

- `RunOnce()` with a mock invoker (test the iteration lifecycle without actual subprocess execution)
- `applyModelMarkers()` — parse various marker combinations, verify state mutations
- `parseExpandedSections()` — test section splitting

### Shared Test Helpers (`internal/testutil`)

Create a new package with reusable helpers:

```go
package testutil

// WriteJSON writes v as indented JSON to path, failing the test on error.
func WriteJSON(t *testing.T, path string, v any)

// SetupWolfcastleDir creates a minimal .wolfcastle/ directory with config
// and returns its path. Cleans up on test completion.
func SetupWolfcastleDir(t *testing.T) string

// SetupNodeState creates a node state file at the expected path under
// the given projects directory. Returns the NodeState for further mutation.
func SetupNodeState(t *testing.T, projectsDir, addr string, nodeType state.NodeType) *state.NodeState

// NewTestConfig returns a minimal valid Config for testing.
func NewTestConfig() *config.Config
```

---

## 3. Tier 2: Integration Tests

### Directory Structure

```
test/
├── integration/
│   ├── init_test.go           # wolfcastle init workflow
│   ├── project_test.go        # project create + task lifecycle
│   ├── daemon_test.go         # daemon start/stop/status
│   ├── audit_test.go          # audit approve/reject workflow
│   ├── doctor_test.go         # doctor validation and fix
│   ├── helpers_test.go        # shared helpers for integration tests
│   └── testdata/
│       └── golden/            # golden files for output comparison
└── smoke/
    └── smoke_test.go          # binary build and basic invocation
```

### Test Pattern

Each integration test follows this pattern:

```go
func TestProjectLifecycle(t *testing.T) {
    // 1. Create a temp directory
    dir := t.TempDir()

    // 2. Initialize wolfcastle
    run(t, dir, "init")

    // 3. Execute the workflow under test
    run(t, dir, "project", "create", "my-feature")
    run(t, dir, "task", "add", "--node", "my-feature", "implement the thing")

    // 4. Verify state
    idx := loadRootIndex(t, dir)
    assert(t, idx.Nodes["my-feature"].State == "not_started")
}
```

### Helper Functions

```go
// run executes wolfcastle in the given directory and returns stdout.
// Fails the test if the exit code is non-zero.
func run(t *testing.T, dir string, args ...string) string

// runExpectError executes wolfcastle and expects a non-zero exit code.
func runExpectError(t *testing.T, dir string, args ...string) string

// runJSON executes wolfcastle with --json and unmarshals the response.
func runJSON(t *testing.T, dir string, args ...string) output.Response

// loadRootIndex reads the root index from the test directory.
func loadRootIndex(t *testing.T, dir string) *state.RootIndex
```

### Critical Paths to Cover

| Test | Commands exercised | Validates |
|------|-------------------|-----------|
| Init workflow | `init` | .wolfcastle/ created, config.json valid, base/ populated |
| Project lifecycle | `init` → `project create` → `task add` → `task claim` → `task complete` → `status` | State transitions, propagation, status output |
| Audit workflow | `init` → `project create` → `audit run` → `audit pending` → `audit approve` / `audit reject` | Review batch creation, decision recording, history archival |
| Doctor validation | `init` → manually corrupt state → `doctor` → `doctor --fix` | Issue detection, deterministic fix |
| Spec management | `init` → `spec create` → `spec link` → `spec list` | Spec creation, node linkage |
| JSON output | All major commands with `--json` | Envelope structure, action strings, data shape |
| Error handling | Commands with missing args, bad node addresses, uninitialized state | Error messages, exit codes |

### Golden File Testing

For commands with complex output (status, audit pending, doctor), use golden files:

1. Run the command and capture output
2. Compare against `testdata/golden/{testname}.txt`
3. Update golden files with `-update` flag: `go test -tags integration -run TestFoo -update`

---

## 4. Tier 3: Smoke Tests

### Purpose

Smoke tests verify that the compiled binary starts and responds to basic commands. They catch catastrophic build failures (missing init() calls, import cycles that somehow pass compilation but panic at runtime, etc.).

### Tests

```go
func TestBinaryBuilds(t *testing.T) {
    // Build the binary
    binary := buildBinary(t)

    // Verify it runs
    out := runBinary(t, binary, "version")
    assert(t, strings.Contains(out, "wolfcastle"))
}

func TestHelpOutput(t *testing.T) {
    binary := buildBinary(t)
    out := runBinary(t, binary, "--help")
    assert(t, strings.Contains(out, "Model-agnostic autonomous project orchestrator"))
}

func TestInitAndStatus(t *testing.T) {
    binary := buildBinary(t)
    dir := t.TempDir()
    runBinaryInDir(t, binary, dir, "init")
    out := runBinaryInDir(t, binary, dir, "status", "--json")
    // Verify JSON envelope
    var resp output.Response
    json.Unmarshal([]byte(out), &resp)
    assert(t, resp.OK)
}
```

### Performance

Smoke tests must complete in under 10 seconds. If they take longer, the binary build should be cached across test functions using `TestMain`.

---

## 5. Coverage Targets

| Scope | Target | Rationale |
|-------|--------|-----------|
| `internal/` | 85%+ | Core logic, high test value |
| `cmd/` | 60%+ | Covered indirectly by integration tests |
| Overall | 75%+ | Balanced — avoids gaming while ensuring meaningful coverage |

These are aspirational targets, not hard gates. Coverage is reported in CI but does not fail the build. The goal is visibility, not enforcement — coverage gaming (testing trivial paths to hit a number) is worse than an honest gap.

---

## 6. Test Conventions

- **Table-driven tests** for functions with multiple input/output cases
- **`t.TempDir()`** for all filesystem tests (auto-cleanup)
- **`t.Parallel()`** where tests are independent (most unit tests)
- **Descriptive subtest names** via `t.Run("description", func(t *testing.T) {...})`
- **No `testify`** — use standard library `testing` package. Helper functions return errors; test functions call `t.Fatal`/`t.Error`.
- **Test file naming:** `foo_test.go` tests `foo.go`. Additional test files use `foo_extra_test.go` or `foo_{aspect}_test.go` (e.g., `navigation_dfs_test.go`).
