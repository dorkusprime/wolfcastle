# Model Provider Abstraction

## Status
Draft

## Problem

The README promises: *"Agents are defined as CLI commands. Anything that reads stdin and writes stdout works: Claude Code, Cursor, Copilot, GPT, Gemini, Llama, a bash script wrapping curl. Your agents, your choice. Switch providers by editing a JSON file."*

That promise is partially true. `internal/invoke.ProcessInvoker` really will execute any CLI that reads stdin and writes stdout. But the surrounding daemon only produces useful behavior when the output happens to be **Claude Code's stream-json format**. Four codepaths are hard-coded to that format today:

1. **`internal/invoke/format.go:FormatAssistantText`** parses Claude Code's `{"type":"assistant","message":{"content":[{"type":"text","text":"..."},{"type":"tool_use","name":"...","input":{...}}]}}` envelopes to produce a human-readable line. Called from `cmd/unblock.go:234` and was historically called from `logrender` too.
2. **`internal/daemon/iteration.go:extractAssistantText`** parses the same envelope shape to extract the text content that the terminal-marker scanner will search.
3. **`internal/logrender/*`** (specifically `thoughts.go` and `interleaved.go`) decode assistant envelopes with their own local parser (`extractThoughtText`) for NDJSON log rendering.
4. **`internal/tui/detail/logview.go:extractAssistantContent`** decodes assistant envelopes plus `tool_use`, `tool_result`, and `thinking` content blocks for the TUI log modal.

On top of those parsers, there's a **double-envelope** situation on disk:

```json
// wolfcastle log record (outer)
{"level":"debug","type":"assistant","trace":"exec-0042",
 "timestamp":"2026-04-15T04:42:00Z",
 "text":"{\"type\":\"assistant\",\"message\":{\"content\":[{\"type\":\"text\",\"text\":\"hello\"}]}}\n"}
```

The outer envelope is a wolfcastle log record written by `internal/logging/logger.go`. Its `"type":"assistant"` is not the model's assistant message — it's wolfcastle's way of tagging a record as "a line of streamed model output." The inner envelope (escaped inside `text`) is Claude Code's wire format. Every downstream reader has to unwrap twice.

The `assistantLogWriter` that produces the outer envelope lives at `logging/logger.go:214`, tagging every streamed byte as `"type":"assistant"`. Daemon callers reach it via `d.Logger.AssistantWriter()`, passed as the `logWriter` parameter into `ProcessInvoker.Invoke`. In a multi-provider world, `"assistant"` as an outer tag is a misleading name — a raw stdout line from an OpenAI streaming response is not semantically an "assistant message" at the logging layer. This rename is part of the migration.

The config's `ModelDef` is minimal:

```go
type ModelDef struct {
    Command string   `json:"command"`
    Args    []string `json:"args"`
}
```

There's nowhere to declare *how the output should be parsed*. The daemon assumes one answer for every model.

## Goals

1. **A named provider per model** in config, orthogonal to the CLI command. `claude-code` is the default; `raw` ships alongside it. API-based providers (OpenAI, Anthropic API, Ollama HTTP) are explicitly deferred.
2. **A `Provider` interface** under `internal/provider` that owns both invocation *and* stream parsing. Daemon code calls into the provider instead of reaching for raw stdout.
3. **Backwards compatibility.** Existing configs without a `provider` field keep working as Claude Code. No prompt changes, no migration required for existing projects, no broken tests.
4. **Minimum two shipping providers in v0.7:** `claude-code` (lifted from current code, identical behavior) and `raw` (opaque stdout, marker-only). The `raw` provider is the "bring your own agent" minimum and needs zero API credentials to build, test, or demo.
5. **Pluggable log rendering.** The TUI and log renderers get a provider handle and ask it to format raw bytes into human-readable output. No more stream-json literals in `logrender` or `tui/detail/logview.go`.
6. **Single source of truth for envelope parsing.** `claudecode`'s parser replaces the four sites listed above. No drift between them. Test coverage consolidated in one package.

## Non-goals

- **API-based providers in v0.7.** OpenAI, Anthropic API, Ollama HTTP, Gemini — these are real providers we'll want eventually, but they add network-mocking infrastructure, auth key handling, per-call cost tracking, and rate limiting that deserve their own spec. v0.7 establishes the interface and proves it works with `raw` (zero network) and `claude-code` (current behavior). The follow-up spec picks one API-based provider and migrates this ground to production.
- **A universal tool-call abstraction.** Each provider parses its own tool invocations into a common `ToolEvent` shape, but models still invoke whatever tools their native format exposes. We're not normalizing tool schemas across providers.
- **Per-call provider selection at runtime by prompt instruction.** Providers are chosen at config time via the stage's `model` field, not dynamically mid-run by the model itself.
- **Replacing `internal/invoke`.** The package stays as the CLI-process toolkit (stall detection, stdout streaming, terminal restoration, platform-specific `procattr_*` files). Non-CLI providers don't use it; CLI-based providers (`claude-code`, `raw`) compose it internally.

## Design

### Package layout

```
internal/provider/
├── provider.go             // Provider interface, Event, Result, Registry
├── testutil/
│   └── fake.go             // provider.Provider test double for other packages
├── claudecode/
│   ├── claudecode.go       // ClaudeCode implementation (lifted from current parsers)
│   ├── parse.go            // stream-json envelope parser (the One True Copy)
│   └── claudecode_test.go
└── raw/
    ├── raw.go              // opaque-stdout implementation
    └── raw_test.go
```

`internal/invoke/` stays. Its public surface (`Invoker`, `Result`, `ProcessInvoker`, `Simple`, `FormatAssistantText`) is deprecated but not removed in v0.7 (see backwards compatibility).

### Core interface

```go
// Provider is the contract for running a model and interpreting its
// output. Implementations encapsulate both *how to invoke* the model
// (the CLI / API contract) and *how to read its stream* (envelope
// format, tool-call shape, marker positions). Every daemon, logrender,
// and TUI code path above this layer is provider-agnostic.
type Provider interface {
    // Name returns the canonical provider identifier used in config.
    // Stable across releases; required by the registry.
    Name() string

    // Invoke runs the model with the given prompt and returns a Result.
    // Implementations may shell out to a CLI, make an HTTP call, or
    // anything else. logWriter, if non-nil, receives every raw line
    // of stdout before any parsing. onLine, if non-nil, receives each
    // line in real time for callers that need incremental access.
    Invoke(
        ctx context.Context,
        spec ModelSpec,
        prompt string,
        workDir string,
        logWriter io.Writer,
        onLine LineCallback,
    ) (*Result, error)

    // ParseLine takes a raw line of provider output and returns the
    // structured event it represents, or (nil, nil) for a line that
    // carries no display-relevant content (blank line, debug noise,
    // unrecognized envelope). Every downstream renderer calls this
    // instead of parsing provider-specific JSON.
    ParseLine(line string) (*Event, error)

    // ExtractText returns the human-readable text embedded in a raw
    // line, or "" if none. Fast path for the terminal-marker scanner,
    // which only cares about text content — the scanner intentionally
    // does not construct a full Event per line during hot-loop scans.
    ExtractText(line string) string
}
```

`ExtractText` is not strictly necessary — `ParseLine(line).Text` would produce the same answer — but the scanner runs over every line of model output and a full `Event` allocation per line is measurable. Keep it as an optimization hook.

### Supporting types

```go
// ModelSpec replaces config.ModelDef inside the provider boundary. It
// carries the fields each provider might consume. Providers document
// which fields they require and ignore the rest.
type ModelSpec struct {
    // CLI providers (claude-code, raw)
    Command string
    Args    []string

    // HTTP providers (future, deferred)
    Endpoint string
    Model    string

    // Per-provider overflow. Opaque to the registry; providers
    // document any fields they consume.
    Extra map[string]any
}

// Result is the output of a provider invocation. Same shape as the
// legacy invoke.Result — this type IS the successor, not a re-export.
// The old invoke.Result becomes a type alias during migration and is
// removed in v0.8.
type Result struct {
    Stdout         string
    Stderr         string
    ExitCode       int
    TerminalMarker Marker
    Summary        string
}

// Marker identifies a WOLFCASTLE_* terminal marker. Copied into
// internal/provider so the invoke package can eventually be removed;
// until then invoke.Marker becomes a type alias for provider.Marker.
type Marker int

const (
    MarkerNone Marker = iota
    MarkerComplete
    MarkerYield
    MarkerBlocked
    MarkerSkip
    MarkerContinue
)

// Event is the common shape every provider's output is normalized
// into. Renderers (TUI log modal, logrender) consume Events, never
// raw lines, so adding a new provider only touches the parser.
type Event struct {
    Kind       EventKind
    Text       string          // populated for EventText, EventThinking
    ToolName   string          // populated for EventToolUse
    ToolInput  json.RawMessage // opaque provider-native JSON for the tool's input
    ToolOutput string          // populated for EventToolResult
    Marker     string          // populated for EventMarker
    Raw        string          // original line, always populated
    Level      string          // "debug" | "info" | "warn" | "error"
}

type EventKind int

const (
    EventRaw EventKind = iota // line didn't parse as a known envelope
    EventText                 // human-readable model output
    EventThinking             // model reasoning / scratchpad (Claude "thinking" blocks)
    EventToolUse              // model invoked a tool
    EventToolResult           // tool returned a result
    EventMarker               // a WOLFCASTLE_* terminal marker was on this line
)

// LineCallback receives each line of model output during streaming.
// Implementations should not block; this runs on the output-read
// goroutine and delays stall the process pipe.
type LineCallback func(line string)
```

`ToolInput` is `json.RawMessage` so providers can hand back arbitrary nested input without losing structure and renderers can pretty-print it at display time. String concatenation of structured input would lose information.

### Registry

The registry is a package-level map written at `init()` time:

```go
// Registry holds all providers registered at init() time. Lookup
// returns singletons — one Provider per name for the life of the
// process. This lets providers hold state (HTTP client pools, cache
// handles, mutex-protected counters) safely, and means the daemon
// doesn't pay a construction cost per iteration.
var registry = map[string]Provider{}

// Register makes a Provider available by name. Intended to be called
// from init() in a provider subpackage. Panics on duplicate names;
// a duplicate means two providers are fighting over one identifier
// and silent override would be worse than a startup crash.
func Register(p Provider) {
    name := p.Name()
    if _, ok := registry[name]; ok {
        panic(fmt.Sprintf("provider %q already registered", name))
    }
    registry[name] = p
}

// Lookup returns the provider registered under name, or (nil, false).
func Lookup(name string) (Provider, bool) {
    p, ok := registry[name]
    return p, ok
}

// Names returns the registered provider names in deterministic order,
// used by config validation to build error messages.
func Names() []string { ... }
```

Each provider subpackage has an `init()` that calls `Register(&ClaudeCode{})` or `Register(&Raw{})`. A top-level `internal/provider/all.go` imports the subpackages for side effects so callers get every built-in provider via one import:

```go
package provider

import (
    _ "github.com/dorkusprime/wolfcastle/internal/provider/claudecode"
    _ "github.com/dorkusprime/wolfcastle/internal/provider/raw"
)
```

Providers are **singletons**. The `Invoke` method takes a `ModelSpec` per call, so per-model state (CLI args, model identifier) is not stored on the Provider instance. Provider-internal state (stall-timer settings, HTTP client pools in future providers) IS stored on the instance and shared across calls.

### Config surface

`ModelDef` gains a `Provider` field:

```go
type ModelDef struct {
    Provider string   `json:"provider,omitempty"` // "claude-code" (default), "raw", ...
    Command  string   `json:"command,omitempty"`
    Args     []string `json:"args,omitempty"`
    // HTTP fields for future providers; ignored by CLI providers.
    Endpoint string   `json:"endpoint,omitempty"`
    Model    string   `json:"model,omitempty"`
}
```

`config.Load()` gets a resolution pass: for each model entry, if `Provider == ""`, defaults to `"claude-code"` and appends an informational warning to the returned `Warnings` slice (`"model 'heavy' has no provider field; defaulting to claude-code. Set \"provider\": \"claude-code\" to silence."`). If the provider name is unknown, `config.Load()` returns a config error — the daemon refuses to start. A new validation category `UNKNOWN_PROVIDER` (matching the existing `INVALID_*` / `UNKNOWN_*` naming style used in `internal/validate`) catches the same error via `wolfcastle doctor`.

Helper:

```go
// ResolveProvider returns the Provider registered for a ModelDef. Must
// be called after config.Load() has defaulted empty Provider fields.
// Returns an error when the provider is unknown.
func (m ModelDef) ResolveProvider() (provider.Provider, error) {
    p, ok := provider.Lookup(m.Provider)
    if !ok {
        return nil, fmt.Errorf("unknown provider %q for model (known: %s)",
            m.Provider, strings.Join(provider.Names(), ", "))
    }
    return p, nil
}

// Spec returns a provider.ModelSpec built from the ModelDef fields.
func (m ModelDef) Spec() provider.ModelSpec {
    return provider.ModelSpec{
        Command:  m.Command,
        Args:     m.Args,
        Endpoint: m.Endpoint,
        Model:    m.Model,
    }
}
```

Example config with two providers:

```json
{
  "models": {
    "heavy": {
      "provider": "claude-code",
      "command": "claude",
      "args": ["--output-format", "stream-json", "--model", "opus"]
    },
    "local": {
      "provider": "raw",
      "command": "ollama",
      "args": ["run", "llama3:70b"]
    }
  }
}
```

### Daemon integration

1. **`runIteration`**, **`runPlanningPass`**, **`invokeWithRetry`** stop accepting `config.ModelDef` and accept a `provider.Provider` plus a `provider.ModelSpec` instead. Resolution happens once per iteration (at the start) via `modelDef.ResolveProvider()`; the resolved handle is passed down. The daemon never calls into `internal/invoke` directly for model execution — it goes through the provider.

2. **`scanTerminalMarker`** in `iteration.go` stops embedding knowledge of Claude Code's envelope format. It calls `provider.ExtractText(line)` to get the text content, then searches for `WOLFCASTLE_*` markers. The priority-ordered resolution stays in `iteration.go` because it's provider-agnostic; the per-line text extraction moves into each provider.

3. **`internal/validate/model_fix.go`**, which currently takes an `invoke.Invoker` and a `config.ModelDef`, now takes a `provider.Provider` and a `provider.ModelSpec`. Its signature updates:

    ```go
    func TryModelAssistedFix(
        ctx context.Context,
        prov provider.Provider,
        spec provider.ModelSpec,
        issue Issue,
        projectsDir string,
        wolfcastleDirs ...string,
    ) (bool, error)
    ```

    Callers in `cmd/doctor.go` (and anywhere else) resolve the provider before calling in.

4. **`cmd/unblock.go`** uses `invoke.Simple` + `invoke.FormatAssistantText` to run a model synchronously and format its output. Both calls migrate: `provider.Lookup("claude-code").Invoke(...)` replaces `invoke.Simple`, and `provider.ClaudeCode().ParseLine(line).Text` replaces `invoke.FormatAssistantText`. The shape of `unblock` doesn't change.

5. **`cmd/cmdutil/app.go`** has an `Invoker invoke.Invoker` field on the `App` struct, used for dependency injection in tests. It becomes `Provider provider.Provider`, defaulting to `claude-code`. The `App.Invoker` field is removed in v0.8 after call sites finish migrating.

### Log record format

The daemon's `logging.Logger.AssistantWriter()` currently wraps every raw model line as `{"type":"assistant","text":"..."}`. That outer `"assistant"` tag is a wolfcastle log category, not Claude Code's message type, but the name is actively confusing in a multi-provider world.

Migration:

1. A new method `logging.Logger.ProviderStreamWriter()` returns a writer that wraps lines as `{"type":"model_output","provider":"<name>","text":"..."}`. The `provider` field is stamped from whichever Provider is driving the iteration.
2. `AssistantWriter()` is deprecated and kept as a thin wrapper that calls `ProviderStreamWriter("claude-code")` so existing test fixtures still work.
3. `logrender` reader: the record-filter switch case `case "assistant":` becomes `case "assistant", "model_output":` for backwards compat, then narrows to `"model_output"` only in v0.8.
4. Renderers dispatch by the `provider` field when present. When absent (legacy records), default to `claude-code`.

On-disk log records from a mixed-provider run look like:

```
{"type":"model_output","provider":"claude-code","trace":"plan-0030","text":"{\"type\":\"assistant\",\"message\":..."}
{"type":"model_output","provider":"raw","trace":"worker-leaf-task-0001","text":"Working on the task...\n"}
```

The inner `text` is whatever the provider's stdout produced, byte-for-byte. Each renderer reads the outer envelope, finds the provider, and hands the inner `text` to `provider.ParseLine(...)` for display.

### TUI integration

The TUI's `logview.go` `extractAssistantContent(raw string) string` becomes `formatByProvider(rec logrender.Record) string`:

```go
func formatByProvider(rec logrender.Record) string {
    prov, ok := provider.Lookup(rec.Provider)
    if !ok {
        prov, _ = provider.Lookup("claude-code") // legacy fallback
    }
    ev, err := prov.ParseLine(rec.Text)
    if err != nil || ev == nil {
        return rec.Text
    }
    return renderEvent(ev) // existing icon/style logic
}
```

`renderEvent` keeps the current styling: tool_use → `[tool: name]`, tool_result → `[tool result] …`, thinking → `[thinking] …`, text → plain. The styling lives in the TUI and is provider-agnostic because it operates on `Event`, not raw strings.

Mixed-provider sessions work because each log record names its provider. The TUI doesn't need to know which stage ran which model — it reads the provider off each record.

### `internal/invoke` package fate

Stays in place, shrunken:

- **Removed** (moved to `internal/provider/claudecode/`): `FormatAssistantText`, `detectMarkers`, `detectLineMarker`, `Marker` constants, `MarkerStringXXX` constants.
- **Deprecated with aliases** for v0.7: `Invoker`, `Result`, `LineCallback` become type aliases pointing at `provider.*` equivalents. `Simple` becomes a thin wrapper over `provider.Lookup("claude-code").Invoke(...)`.
- **Kept as primitives**: `ProcessInvoker` (CLI spawn + stall timer), `procattr_unix.go`, `procattr_windows.go`, `terminal_*.go`, `RestoreTerminal`. These are CLI infrastructure that `claudecode` composes internally. Non-CLI providers don't import any of them.
- **Removed entirely**: `RetryInvoker`. It's defined but never instantiated outside of its own tests (the daemon has its own retry loop in `daemon/retry.go`). v0.7 deletes it; tests that exercised `RetryInvoker` get rewritten to test the daemon's retry logic directly (where most of them belong anyway).

All the removals happen in phase 3 (see Migration). Aliasing preserves the old import paths through phase 3 so tests can be migrated file-by-file rather than in one atomic commit.

### Stream formats shipped

- **`claude-code`**: parses Claude Code's stream-json envelopes. Returns `EventText` for `text` content blocks, `EventThinking` for `thinking` blocks, `EventToolUse` for `tool_use` blocks (with `ToolInput` as the raw JSON of `content[i].input`), `EventToolResult` for `tool_result` blocks. Unrecognized envelope types (`system`, `result`, unknown) return `EventRaw`. This is the current parser's behavior preserved exactly.

- **`raw`**: `ParseLine` returns `&Event{Kind: EventText, Text: line, Raw: line}` for every non-blank line. No tool extraction, no thinking blocks, no envelope awareness. `ExtractText(line) == line`. Terminal-marker scanning works via substring match on the raw line, which is how the fallback path in the current `scanTerminalMarker` works when `extractAssistantText` returns empty.

### Test fake

`internal/provider/testutil/fake.go` ships a `Fake` provider for use in daemon / TUI / logrender tests:

```go
// Fake is a programmable provider.Provider for tests. Callers set
// fixed outputs and assert on captured calls.
type Fake struct {
    Results      []*provider.Result // popped in order per Invoke call
    ParseFunc    func(line string) (*provider.Event, error)
    ExtractFunc  func(line string) string

    // Capture
    Calls []FakeCall
}

type FakeCall struct {
    Spec   provider.ModelSpec
    Prompt string
}

func (f *Fake) Name() string { return "fake" }
func (f *Fake) Invoke(...) (*provider.Result, error) { ... }
func (f *Fake) ParseLine(line string) (*provider.Event, error) { ... }
func (f *Fake) ExtractText(line string) string { ... }
```

`Fake` is registered only in test builds (behind a `// +build testfake` or equivalent mechanism). Daemon tests that currently build a `ModelDef{Command: ..., Args: ...}` literal and rely on a mock CLI will be rewritten to inject a `Fake` provider directly, removing the shell-out from test paths and speeding up the daemon suite.

## Backwards compatibility

- **Configs without `provider`**: default to `claude-code`, informational warning on load, behavior unchanged.
- **Existing `config.ModelDef{Command: ..., Args: ...}` test literals** (29 sites across `internal/` + `cmd/`, enumerated below): each gains `Provider: "claude-code"` explicitly. Many will simultaneously migrate to injecting a `*testutil.Fake` instead of shelling out. Phase 3 does this mechanically; compiler-verified.
- **`invoke.Invoker`, `invoke.Result`, `invoke.LineCallback`** remain as type aliases through v0.7 so external/downstream code (if any — wolfcastle has no published importers) compiles unchanged.
- **`invoke.Simple`** remains as a thin shim: `provider.Lookup("claude-code").Invoke(ctx, provider.ModelSpec{Command: model.Command, Args: model.Args}, prompt, workDir, nil, nil)`.
- **On-disk log records**: existing `.jsonl` files with `"type":"assistant"` (no `provider` field) continue to render correctly — the TUI / logrender falls back to `claude-code` when the field is missing. The migration doesn't require rewriting old logs.

## Migration plan

Five phases, each a separate PR against `release/0.7`. Phase numbering is the same throughout this spec (no 5-vs-7 confusion).

### Phase 1 — Interface + registry + `claudecode` (no daemon changes)
- New package `internal/provider` with `Provider`, `Event`, `Result`, `Marker`, `ModelSpec`, `LineCallback`, `Registry` API.
- New subpackage `internal/provider/claudecode` wrapping current `ProcessInvoker` logic and lifting `FormatAssistantText` / `detectLineMarker` into a single `parse.go`.
- Daemon and existing call sites unchanged; they still import `internal/invoke`. Zero behavioral change.
- Full test coverage for `claudecode.ParseLine` / `ExtractText` mirroring the existing `invoke.FormatAssistantText` test cases.

### Phase 2 — Config field + validation + test fake
- `ModelDef.Provider` field with defaulting in `config.Load()`.
- `UNKNOWN_PROVIDER` validation category in `internal/validate`; `wolfcastle doctor` reports it.
- `internal/provider/testutil.Fake` ships and is usable from tests.
- Daemon tests that currently build a `ModelDef` literal get the `Provider: "claude-code"` field mechanically. Still no runtime behavior change.

### Phase 3 — Daemon runtime through `Provider`
- `runIteration`, `runPlanningPass`, `invokeWithRetry`, `TryModelAssistedFix` swap `invoke.Invoker` + `config.ModelDef` for `provider.Provider` + `provider.ModelSpec`.
- `scanTerminalMarker` in `iteration.go` delegates per-line text extraction to `provider.ExtractText`.
- `cmd/unblock.go` and `cmd/cmdutil/app.go` migrate; `App.Invoker` becomes `App.Provider`.
- `invoke.Simple` stays as a deprecated shim. `RetryInvoker` is deleted and its tests are either deleted or rewritten against the daemon's retry loop.
- Daemon tests continue to pass; any that still used `ModelDef` literals with mocked CLIs are rewritten to inject `testutil.Fake`.

### Phase 4 — Logging + log rendering + TUI through `Provider`
- `logging.Logger.ProviderStreamWriter(providerName)` ships. `AssistantWriter()` is deprecated to a thin wrapper.
- Daemon call sites in `iteration.go`, `planning.go`, `stages.go` (three files) switch from `d.Logger.AssistantWriter()` to `d.Logger.ProviderStreamWriter(prov.Name())`.
- Log records on disk gain a `provider` field. `logrender.Record` gets a `Provider` field populated from that.
- `logrender/thoughts.go` and `logrender/interleaved.go` drop their local `extractThoughtText` parsers and call `provider.Lookup(rec.Provider).ParseLine(rec.Text)` instead. Both renderers consume `Event.Text`.
- `tui/detail/logview.go` migrates `extractAssistantContent` → `formatByProvider` per the design above.
- Full acceptance test pass: `teatest` scripts covering the log modal under `claude-code`.

### Phase 5 — `raw` provider + live smoke test
- `internal/provider/raw` ships with unit tests.
- Daemon integration test under `raw`: a mock CLI emits a fixed plain-text sequence ending in `WOLFCASTLE_COMPLETE`, runs through a full iteration, and the daemon commits. No stream-json involved.
- Live smoke test with a real non-Claude model (target: `ollama run llama3:70b` with a trivial one-task project). Any issue found at this stage is a bug report against an earlier phase, not a new feature.
- README and `docs/humans/configuration.md` updated to describe providers, `claude-code` as default, `raw` as fallback. CHANGELOG entry.
- Finally, `release/0.7` opens its PR into `main` and the combined 0.7 → v0.7.0 release gets cut.

## Risks

- **Test fixture churn**: 29 sites (internal + cmd, non-test) reference `ModelDef{}` literals; considerably more test files (estimate ~45-50 based on `grep -l ModelDef{ **/*_test.go`) will need updates during Phase 2/3. Compiler-verified, mechanical, but tedious. Mitigation: land the `testutil.Fake` in Phase 2 before the migration starts, so Phase 3 edits swap shell-out mocks for `Fake` at the same time.
- **Event type drift**: once renderers consume `Event` instead of raw strings, adding a new event type (say, `EventImage` for multimodal models) is a cross-cutting change. Mitigation: keep `Event.Raw` always populated so a renderer that doesn't know about a new kind can still fall back to displaying the raw line.
- **`invoke.Simple` users in downstream code**: wolfcastle has no known external importers today, but the public surface should stay stable through 0.7 in case. Type aliases + shim keep compatibility. v0.8 can remove.
- **Double-envelope brittleness**: the outer `type=model_output` + inner provider-native JSON means every tool that tails `.jsonl` files still has to parse two layers. Normalizing the on-disk format (writing `Event`s directly instead of raw provider output) is tempting but would be a breaking change to the log format and require a data migration. v0.7 keeps the double envelope; a normalized format can be its own v0.8 spec if motivated.
- **`RetryInvoker` deletion**: theoretically could break an external caller, though there is none inside the repo. If concern arises, keep the type and stub its implementation to delegate to the daemon's retry logic. Default plan is deletion.

## Verification

1. **Unit tests per provider.** Each subpackage has a `_test.go` covering `ParseLine` happy/empty/malformed cases, `ExtractText` accuracy, marker detection, and tool-use round-trips (claude-code only).
2. **Registry round-trip test.** `TestLookup_ReturnsSameInstance`: confirms `provider.Lookup("claude-code")` returns the identical singleton across calls. `TestLookup_UnknownProvider`: confirms unknown names return `(nil, false)`. `TestRegister_Duplicate_Panics`.
3. **Config resolution test.** `TestConfigLoad_DefaultsEmptyProvider`: a config with no `provider` field loads with `provider="claude-code"` and an informational warning. `TestConfigLoad_UnknownProvider_Errors`: a config with `provider="oxnard"` fails to load with a clear error naming valid providers.
4. **Daemon integration tests.** The existing `TestDaemon_ExploratoryReview_CreatesRemediationLeaf` continues to pass unchanged (runs under `claude-code`). A new `TestDaemon_RawProvider_CompletesTask` runs the same daemon loop under `raw` with a shell-script mock model. Both use `testutil.Fake` where possible to avoid spawning subprocesses.
5. **TUI acceptance tests.** `teatest` scripts verify the log modal renders text, tool_use, tool_result, and thinking events correctly under `claude-code`, and renders opaque text under `raw`. A mixed-provider session test confirms the TUI dispatches per-record based on the `provider` field.
6. **Migration smoke test.** Load a v0.6.x config (no `provider` field) and confirm startup logs the defaulting warning, the daemon runs one task, and the log file carries `"provider":"claude-code"` records.
7. **Live test with ollama.** The final pre-merge step is a real wolfcastle project run end-to-end with `raw` pointed at `ollama run <some-small-model>`. Success criterion: one leaf task completes, state propagates, the TUI log modal shows the model's output as plain text. Failure is a blocker for the 0.7.0 merge.

## Implementation sequence

Matches the Migration Plan above 1:1. This section reiterates for readers skipping to the bottom:

1. **Phase 1**: `internal/provider` + `claudecode` + tests. No daemon changes.
2. **Phase 2**: `ModelDef.Provider` field, defaulting, validation, `UNKNOWN_PROVIDER`, `testutil.Fake`.
3. **Phase 3**: Daemon runtime migration (`runIteration`, `runPlanningPass`, `invokeWithRetry`, `TryModelAssistedFix`, `cmd/unblock.go`, `cmd/cmdutil/app.go`). `invoke.RetryInvoker` deleted. `invoke.Simple` shimmed.
4. **Phase 4**: `ProviderStreamWriter`, outer log envelope rename, `logrender` + `tui/detail/logview.go` migration, TUI acceptance tests.
5. **Phase 5**: `raw` provider, daemon test under `raw`, live ollama smoke, docs, CHANGELOG, PR into main, tag v0.7.0.
