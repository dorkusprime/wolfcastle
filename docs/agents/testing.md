# Integration Testing

This guide covers how to run, write, and debug daemon integration tests using Wolfcastle's mock model system.

## Running Integration Tests

Unit tests run without tags:

```sh
go test ./...
```

Integration tests require the `integration` build tag:

```sh
go test -tags integration ./test/integration/...
```

The `internal/daemon/` package also has integration-tagged tests that exercise `RunOnce()` directly:

```sh
go test -tags integration ./internal/daemon/...
```

Run both in one shot:

```sh
go test -tags integration ./internal/daemon/... ./test/integration/...
```

## Two Layers of Daemon Tests

**Layer 1: `internal/daemon/integration_test.go`.** These tests construct a `Daemon` struct via `testDaemon(t)`, configure a model definition pointing at a shell script, and call `RunOnce()` directly. They test the daemon's internal iteration logic: marker parsing, state transitions, prompt echo rejection, failure escalation, self-healing. No subprocess, no CLI dispatch.

**Layer 2: `test/integration/daemon_test.go`.** These tests exercise the full CLI path. They call `run(t, dir, "start")`, which runs `wolfcastle start` as a real process. The daemon picks up work, invokes the mock model script, and exits when it hits a stop file or iteration cap. These tests validate the entire pipeline from command parsing through state persistence.

## Writing a New Daemon Integration Test

### Layer 1 (internal/daemon)

```go
func TestIntegration_MyScenario(t *testing.T) {
    d := testDaemon(t)
    d.Config.Git.VerifyBranch = false

    // Create a node with tasks
    setupLeafNode(t, d, "my-node", []state.Task{
        {ID: "task-1", Description: "do the thing", State: state.StatusNotStarted},
        {ID: "audit", Description: "audit", State: state.StatusNotStarted, IsAudit: true},
    })

    // Configure the mock model
    d.Config.Models["echo"] = config.ModelDef{
        Command: "sh",
        Args:    []string{"-c", `cat > /dev/null; echo "WOLFCASTLE_COMPLETE"`},
    }
    writePromptFile(t, d.WolfcastleDir, "execute.md")

    // Run one iteration
    _ = d.Logger.StartIteration()
    result, err := d.RunOnce(context.Background())
    d.Logger.Close()
    if err != nil {
        t.Fatalf("iteration error: %v", err)
    }
    if result != IterationDidWork {
        t.Fatalf("expected DidWork, got %v", result)
    }

    // Verify state
    projDir := d.Resolver.ProjectsDir()
    ns, _ := state.LoadNodeState(filepath.Join(projDir, "my-node", "state.json"))
    if ns.Tasks[0].State != state.StatusComplete {
        t.Errorf("expected complete, got %s", ns.Tasks[0].State)
    }
}
```

Every Layer 1 test file must start with `//go:build integration`.

### Layer 2 (test/integration)

```go
func TestDaemon_MyScenario(t *testing.T) {
    dir := t.TempDir()
    run(t, dir, "init")

    scriptPath := createMockModel(t, dir, "my-mock", "complete")
    configureMockModels(t, dir, scriptPath)

    run(t, dir, "project", "create", "my-project")
    run(t, dir, "task", "add", "--node", "my-project", "do the thing")

    setMaxIterations(t, dir, 10)
    run(t, dir, "start")

    ns := loadNode(t, dir, "my-project")
    for _, task := range ns.Tasks {
        if task.ID == "task-1" && task.State != state.StatusComplete {
            t.Errorf("expected complete, got %s", task.State)
        }
    }
}
```

## Mock Model Behaviors

The `createMockModel` helper accepts a behavior string. Each behavior generates a different shell script:

| Behavior | What it does | Creates stop file |
|----------|-------------|-------------------|
| `complete` | Emits `WOLFCASTLE_COMPLETE` in a JSON result envelope | Yes |
| `yield` | Emits `WOLFCASTLE_YIELD` in a JSON result envelope | No |
| `blocked` | Emits `WOLFCASTLE_BLOCKED` in a JSON result envelope | Yes |
| `no-marker` | Emits assistant text with no terminal marker | No |
| `create-file` | Creates `mock-created-file.txt` in the working directory, then completes | Yes |

All scripts consume stdin (the prompt) before emitting output. All JSON output uses the Claude Code stream-json envelope format: `{"type":"result","text":"WOLFCASTLE_COMPLETE"}`.

## Validating Prompt Content

To verify what the daemon sends to the model, use the assertion file pattern. The mock script reads stdin into a variable, greps for expected content, and writes results to a file:

```go
func TestIntegration_PromptContainsTaskDescription(t *testing.T) {
    d := testDaemon(t)
    d.Config.Git.VerifyBranch = false

    setupLeafNode(t, d, "check-node", []state.Task{
        {ID: "task-1", Description: "implement frobulator", State: state.StatusNotStarted},
        {ID: "audit", Description: "audit", State: state.StatusNotStarted, IsAudit: true},
    })

    assertFile := filepath.Join(t.TempDir(), "assertions.txt")
    scriptFile := filepath.Join(t.TempDir(), "validate.sh")

    script := fmt.Sprintf(`#!/bin/sh
PROMPT=$(cat)
echo "$PROMPT" | grep -q "implement frobulator" && printf "FOUND" > %s
echo "WOLFCASTLE_COMPLETE"
`, assertFile)

    os.WriteFile(scriptFile, []byte(script), 0755)
    d.Config.Models["echo"] = config.ModelDef{Command: "sh", Args: []string{scriptFile}}
    writePromptFile(t, d.WolfcastleDir, "execute.md")

    _ = d.Logger.StartIteration()
    d.RunOnce(context.Background())
    d.Logger.Close()

    data, _ := os.ReadFile(assertFile)
    if string(data) != "FOUND" {
        t.Error("prompt did not contain expected task description")
    }
}
```

## Multi-Iteration Scenarios

Use `createCounterMock` for tests that need the daemon to run through multiple iterations:

```go
func TestDaemon_ThreeYieldsThenComplete(t *testing.T) {
    dir := t.TempDir()
    run(t, dir, "init")

    // Yield 3 times, then complete on the 4th invocation
    scriptPath, counterFile := createCounterMock(t, dir, 3)
    configureMockModels(t, dir, scriptPath)

    run(t, dir, "project", "create", "multi-test")
    run(t, dir, "task", "add", "--node", "multi-test", "iterative work")

    setMaxIterations(t, dir, 10)
    run(t, dir, "start")

    // Verify invocation count
    data, _ := os.ReadFile(counterFile)
    count, _ := strconv.Atoi(strings.TrimSpace(string(data)))
    if count != 4 {
        t.Errorf("expected 4 invocations (3 yields + 1 complete), got %d", count)
    }
}
```

The counter mock tracks invocations via a file on disk. It yields for the first N calls and completes on call N+1. The counter file is readable after the test for verification.

For failure escalation testing, `createNoMarkerStopAfterMock` creates a script that never emits a terminal marker but places the stop file after a specified number of invocations. This lets you test decomposition thresholds and hard cap auto-blocking:

```go
scriptPath := createNoMarkerStopAfterMock(t, dir, 5)
configureMockModels(t, dir, scriptPath)
setFailureAndIterationConfig(t, dir, 3, 5, 50, 20)
```

## Testing CLI-Calling Models

Models that call `wolfcastle` CLI commands (breadcrumbs, gaps, task blocking) can be simulated by including those calls in the mock script:

```sh
#!/bin/sh
cat > /dev/null
wolfcastle audit breadcrumb --node my-node "did some work"
printf '{"type":"result","text":"WOLFCASTLE_COMPLETE"}\n'
touch /path/to/stop
```

For this to work in integration tests, the `wolfcastle` binary must be on `PATH`. Layer 2 tests (which invoke `wolfcastle start` via `run()`) already have this, since the test binary is built and placed on `PATH` by the test harness.

## Configuration Helpers

| Helper | Purpose |
|--------|---------|
| `configureMockModels(t, dir, scriptPath)` | Sets all three model tiers to use the given script; disables git branch verification, sets fast poll intervals |
| `configureWithArgs(t, dir, scriptPath, args)` | Same as above but passes extra args to the model command |
| `setMaxIterations(t, dir, n)` | Merges a `max_iterations` override into `local/config.json` |
| `setFailureAndIterationConfig(t, dir, decomp, maxDepth, hardCap, maxIter)` | Merges both failure thresholds and iteration cap |
| `mergeLocalConfig(t, dir, overrides)` | Generic shallow-merge into `local/config.json` |

## testutil Environment Builder

The `internal/testutil` package provides a structured `Environment` builder for tests that need a full `.wolfcastle/` directory with repositories wired up. This is the preferred way to set up test state for unit tests in `internal/` packages.

### Basic Usage

```go
env := testutil.NewEnvironment(t)
```

This creates a temp directory with a complete `.wolfcastle/` structure, three-tier config, empty root index, and all repository objects pre-wired (`env.State`, `env.Config`, `env.Prompts`, `env.Classes`, `env.Daemon`).

### Building Trees with NodeSpec

Use `Leaf` and `Orchestrator` helpers to describe tree shape declaratively:

```go
env := testutil.NewEnvironment(t).
    WithProject("My Project", testutil.Orchestrator("root",
        testutil.Leaf("auth", "task-0001", "task-0002"),
        testutil.Leaf("api", "task-0001"),
    ))
```

Each leaf automatically receives an audit task. Task descriptions are auto-generated from IDs.

### Chaining Configuration

```go
env := testutil.NewEnvironment(t).
    WithConfig(map[string]any{"max_iterations": 5}).
    WithPrompt("stages/execute.md", "# Execute\n{{.Task}}").
    WithRule("no-force-push.md", "Never force push.").
    WithClasses(map[string]config.ClassDef{"research": {Model: "opus"}}).
    WithGit(myGitProvider)
```

### Extracting App Fields

For tests in `cmd/` that need a `cmdutil.App`, use `ToAppFields()`:

```go
fields := env.ToAppFields()
app := cmdutil.App{
    Config: fields.Config,
    State:  fields.State,
    // ...
}
```

### When to Use Environment vs SetupWolfcastle

Use `NewEnvironment(t)` when your test needs repository objects (config.Repository, Store, PromptRepository). Use `SetupWolfcastle(t)` or `SetupTree(t)` when you only need raw directory paths and are constructing state files yourself.

## Common Patterns and Gotchas

**Always set `d.Config.Git.VerifyBranch = false` in Layer 1 tests.** The daemon checks that the current git branch matches the expected branch. Temp directories aren't git repos, so this check will fail unless disabled.

**Always include an audit task.** The daemon expects every node to have an audit task as its last task. If you set up tasks without one, the daemon may behave unexpectedly. Use `{ID: "audit", Description: "audit", State: state.StatusNotStarted, IsAudit: true}` as the final task.

**The stop file is the clean shutdown mechanism.** Mock scripts that represent terminal states (complete, blocked) should create `.wolfcastle/stop` so the daemon exits. Scripts that represent non-terminal states (yield, no-marker) should not, unless you want the daemon to stop after that iteration for testing purposes.

**Call `d.Logger.StartIteration()` before and `d.Logger.Close()` after each `RunOnce()` in Layer 1 tests.** The daemon writes structured logs, and the logger panics if an iteration isn't started.

**Use `setMaxIterations` as a safety net.** Even if your mock creates a stop file, setting a max iteration count prevents runaway tests if the mock fails to create it.

**Consume stdin in mock scripts.** Every mock script must read stdin (even if it discards it with `cat > /dev/null`). If the script exits without reading, the daemon's pipe write may fail with a broken pipe error, causing spurious test failures.

**Counter files must be initialized.** `createCounterMock` handles this automatically, but if you write a custom counter script, initialize the counter file to `0` before the test runs.
