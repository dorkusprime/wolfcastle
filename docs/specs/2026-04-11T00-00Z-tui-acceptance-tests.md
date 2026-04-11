# TUI Acceptance Tests

## Status
Draft

## Problem

The TUI has unit tests that call `Update()` directly on isolated models and verify returned state. These tests never run a real `tea.Program`, never exercise the event loop, never render to a terminal. A wiring bug, a dropped message, a rendering regression, or a broken key binding that the help overlay claims exists would all pass the unit tests and ship to users.

The existing `test/smoke/` suite verifies the binary builds and `wolfcastle version` runs. Nothing tests that launching `wolfcastle` produces a functioning interactive interface.

## Approach

Use `charmbracelet/x/exp/teatest/v2`, a test harness for Bubbletea v2 applications. It runs a real `tea.Program` in a headless goroutine with controlled I/O: the test sends keystrokes and messages, reads rendered output, and inspects the final model state. No terminal required. No tmux. No external dependencies beyond Go test infrastructure.

Tests live in `test/acceptance/`, gated behind `//go:build acceptance`. CI runs them as a separate job, after build and unit tests pass. They are slower than unit tests (real event loops, real I/O buffers, real timing) but faster than integration tests (no daemon, no model invocation, no filesystem state beyond scaffolding).

## Design

### Package structure

```
test/acceptance/
  tui_test.go          # All acceptance tests
  helpers_test.go       # Shared setup, assertion utilities
```

Single package, `package acceptance`. Build tag `//go:build acceptance`. Tests construct a `TUIModel` with a scaffolded `.wolfcastle/` directory, wrap it in `teatest.NewTestModel`, drive it with keystrokes, and assert on rendered output.

### teatest API usage

```go
tm := teatest.NewTestModel(t, model,
    teatest.WithInitialTermSize(120, 40),
)

// Send keystrokes
tm.Type("i")              // press i (inbox)
tm.Type("a")              // press a (add item)
tm.Type("Build something")
tm.Send(tea.KeyPressMsg{Code: tea.KeyEnter})

// Assert on rendered output
teatest.WaitFor(t, tm.Output(), func(b []byte) bool {
    return bytes.Contains(b, []byte("Build something"))
}, teatest.WithDuration(3*time.Second))

// Quit and inspect final state
tm.Type("q")
final := tm.FinalModel(t).(app.TUIModel)
```

`WaitFor` polls the program's output buffer until a condition matches or a timeout expires. Default timeout is 1 second, configurable per call. This replaces fragile `time.Sleep` waits.

### Test model construction

Tests need a `TUIModel` with real-enough state to render and respond to input, without a running daemon or live filesystem watcher.

```go
func newTestTUI(t *testing.T, entryState app.EntryState) app.TUIModel {
    t.Helper()
    dir := t.TempDir()
    // Scaffold a minimal .wolfcastle/ if needed
    if entryState != app.StateWelcome {
        scaffoldProject(t, dir)
    }
    store := state.NewStore(filepath.Join(dir, ".wolfcastle", "system",
        "projects", "test-machine"), 0)
    daemonRepo := daemon.NewRepository(filepath.Join(dir, ".wolfcastle"))
    m := app.NewTUIModel(store, daemonRepo, dir, "test")
    m.SetEntryState(entryState)
    return m
}
```

The `SetEntryState` method (or equivalent) may need to be exported for test use. If `TUIModel` fields are unexported, a `testing`-only constructor or a `WithEntryState` option is preferable to exporting internal state.

### Test scenarios

**Welcome screen flow:**
1. Launch in `StateWelcome`
2. Assert "WOLFCASTLE" title renders
3. Press `I` to initialize
4. Assert transition to cold state (dashboard renders)

**Dashboard navigation:**
1. Launch in `StateCold` with a populated tree
2. Assert dashboard renders with node counts
3. Press `j` to move cursor in tree
4. Press `Enter` to expand a node
5. Assert node detail renders with scope/criteria
6. Press `d` to return to dashboard
7. Assert dashboard is visible again

**Inbox flow:**
1. Launch in `StateCold`
2. Press `i` to open inbox modal
3. Assert inbox modal renders
4. Press `a`, type text, press `Enter`
5. Assert item appears in inbox list
6. Press `Esc` to close modal

**Search flow:**
1. Launch with a populated tree
2. Press `/`, type a query, press `Enter`
3. Assert search highlights appear in output
4. Press `n` to advance to next match
5. Press `Esc` to dismiss

**Help overlay:**
1. Press `?`
2. Assert help overlay renders with all keybinding sections (Global, Tree Navigation, Daemon Control, Inbox, Log Stream, Search)
3. Press `?` again to dismiss
4. Assert overlay gone

**Key binding coverage:**
1. Every key listed in the help overlay (`help/model.go`) has a test that sends it and verifies a visible effect. This is the acceptance criterion: if the help overlay says a key does something, the acceptance test proves it.

### Assertion style

String matching on the raw output buffer. No ANSI parsing. The output contains escape sequences, but `bytes.Contains` on the text content works because Bubbletea renders the text interspersed with escape codes. For glyphs (`●`, `◐`, `◯`, `☢`), match on the UTF-8 bytes directly.

For negative assertions (verifying something is NOT rendered), use a short `WaitFor` with the inverse condition and expect the timeout. Or read the output at a point in time and assert absence.

### What acceptance tests do NOT cover

- Daemon lifecycle (start/stop with real processes). That's integration testing.
- Model invocation (LLM calls). That's integration testing.
- Filesystem watching (fsnotify events). The unit tests cover the watcher; acceptance tests inject state messages directly.
- Platform-specific rendering (Windows console vs Unix terminal). Acceptance tests run headless.

### Vendoring

Add `github.com/charmbracelet/x/exp/teatest/v2` to `go.mod`. Its dependency tree is minimal: it wraps `tea.Program` with controlled I/O and pulls in `github.com/charmbracelet/x/exp/golden` for snapshot testing. Both are small. Most transitive deps (ansi, term, termios) are already vendored.

### CI integration

Add an `acceptance-tests` job to `.github/workflows/ci.yml`:

```yaml
acceptance-tests:
  needs: build-and-test
  runs-on: ubuntu-latest
  steps:
    - uses: actions/checkout@v4
    - uses: actions/setup-go@v5
      with:
        go-version: stable
    - run: go test -tags acceptance -v -timeout 120s ./test/acceptance/...
```

Runs after unit tests pass. 120-second timeout (generous for headless TUI tests). Failure blocks merge.

## Resolved questions

1. **Entry state construction**: Acceptance tests scaffold real `.wolfcastle/` directories (or omit them for the welcome screen) and let `detectEntryState` run naturally. No exported `SetEntryState`. The whole point is to exercise the real path; hiding it behind a test shortcut defeats the purpose.
2. **Golden file snapshots**: Deferred. The TUI is young and layout changes are frequent. Golden files become a maintenance burden when every spacing tweak breaks them. Start with `bytes.Contains` assertions on key content. Add golden files later once the UI stabilizes.
