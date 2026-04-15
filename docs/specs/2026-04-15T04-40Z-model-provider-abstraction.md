# Model Provider Abstraction

## Status
Draft (v3 — post audit)

## Problem

The README promises: *"Agents are defined as CLI commands. Anything that reads stdin and writes stdout works: Claude Code, Cursor, Copilot, GPT, Gemini, Llama, a bash script wrapping curl. Your agents, your choice. Switch providers by editing a JSON file."*

That promise is partially true. `internal/invoke.ProcessInvoker` really will execute any CLI that reads stdin and writes stdout. But the surrounding daemon only produces useful behavior when the output happens to be **Claude Code's stream-json format**. Four codepaths parse that format today:

1. **`internal/invoke/format.go:FormatAssistantText`** parses Claude Code's `{"type":"assistant","message":{"content":[...]}}` envelopes into a human-readable line. Called from `cmd/unblock.go:234`.
2. **`internal/daemon/iteration.go:extractAssistantText`** parses the same envelope shape to extract the text content that `scanTerminalMarker` searches for `WOLFCASTLE_*` markers.
3. **`internal/logrender/thoughts.go:extractThoughtText`** (shared by `interleaved.go`) decodes assistant envelopes for NDJSON log rendering.
4. **`internal/tui/detail/logview.go:extractAssistantContent`** decodes assistant + `tool_use` + `tool_result` + `thinking` blocks for the TUI log modal. This is the *most complex* of the four — `tool_result.content` is handled as either a plain string or an array of typed blocks, a subtlety the new provider parser must replicate exactly.

On top of those four parsers, there's a **double-envelope** situation on disk. `internal/logging/logger.go:assistantLogWriter` wraps every raw byte from the model's stdout in a wolfcastle log record:

```json
// wolfcastle outer log envelope (written by assistantLogWriter)
{"level":"debug","type":"assistant","trace":"exec-0042",
 "timestamp":"2026-04-15T04:42:00Z",
 "text":"{\"type\":\"assistant\",\"message\":{\"content\":[{\"type\":\"text\",\"text\":\"hello\"}]}}\n"}
```

The outer `type: "assistant"` is wolfcastle's historical category name for "a line of streamed model output." The inner envelope (escaped as a JSON string inside `text`) is Claude Code's wire format. Downstream readers have to unwrap twice. **Agent A audit confirmed that `"assistant"` is the *only* fixed-type string produced by `assistantLogWriter`** — the ~80 other `"type"` values scattered through `internal/daemon/*.go` (`stage_start`, `terminal_marker`, `yield_decomposition`, `auto_commit`, etc.) are semantic daemon events, not model-stdout records. The rename question in Phase 4 is narrowly scoped to that one tag.

The config's `ModelDef` is minimal:

```go
type ModelDef struct {
    Command string   `json:"command"`
    Args    []string `json:"args"`
}
```

There's nowhere to declare *how the output should be parsed*. The daemon assumes one answer for every model.

## Goals

1. **A named provider per model** in config, orthogonal to the CLI command. `claude-code` is the default; `raw` ships alongside it. API-based providers (OpenAI, Anthropic API, Ollama HTTP, Gemini) are explicitly Non-goal'd.
2. **A `Provider` interface** under `internal/provider` that owns both invocation *and* stream parsing. Daemon code calls into the provider instead of reaching for raw stdout.
3. **Backwards compatibility.** Existing configs without a `provider` field keep working as Claude Code. Existing `.jsonl` logs on disk render correctly against the new code. No prompt changes, no config migration, no broken `jq` scripts tailing log files.
4. **Two shipping providers in v0.7:** `claude-code` (lifted from current code, identical behavior) and `raw` (opaque stdout, marker-only). The `raw` provider is the "bring your own agent" minimum and needs zero API credentials to build, test, or demo.
5. **Pluggable log rendering.** The TUI and log renderers consume `Event` from the provider instead of hand-parsing stream-json. Phase 1's `claudecode` parser is the **one true copy** — the four existing parser sites in the Problem statement are replaced by a single implementation.
6. **Minimum viable surface.** v0.7's public `Provider` interface and `ModelSpec` carry only what v0.7 providers consume. HTTP endpoint, auth, cost tracking, and tool-call normalization are deliberately out of scope — a follow-up spec adds them when an API provider concretely lands.

## Non-goals

- **API-based providers in v0.7.** OpenAI, Anthropic API, Ollama HTTP, Gemini. Each brings network mocking, auth key handling, per-call cost tracking, and rate limiting, none of which have design surface in v0.7. A follow-up spec picks one API-based provider and drives those concerns end-to-end.
- **HTTP-specific fields in `ModelSpec` / `ModelDef`.** No `Endpoint`, no `Model`, no `Extra map[string]any`. These were in v2 of this spec as speculative hooks for future HTTP providers and have been cut. The minimum viable `ModelSpec` is `Command []string`. When the API-provider spec lands, it adds the fields it needs with types it can defend.
- **A universal tool-call abstraction.** Each provider parses its own tool invocations into a common `ToolEvent` shape, but models still invoke whatever tools their native format exposes. We do not normalize tool schemas across providers.
- **Per-call provider selection by prompt instruction.** Providers are chosen at config time via the stage's `model` field, not dynamically by the model itself.
- **Replacing `internal/invoke` wholesale.** The package stays as the CLI-process toolkit (`ProcessInvoker`, `procattr_*`, `RestoreTerminal`). It shrinks by losing `FormatAssistantText`, the `Marker*` constants, and `RetryInvoker` — everything else is kept, some as type aliases through v0.7.
- **Breaking on-disk log format for scripting users.** v0.6's jq-style consumers (`.type == "assistant"`) keep working. The outer envelope's `type` field stays as `"assistant"`; a new `provider` field is added beside it. Rename would have been cleaner architecturally but breaks every external tailer; not worth it for a name.
- **Observability, cost tracking, secrets management, capability negotiation.** All deferred to the API-provider spec. v0.7 leaves them as open questions, not as fields or hooks on the `Provider` interface that would constrain the future design.
- **Third-party providers via plugins.** `internal/provider` is in the `internal/` tree; external Go packages cannot import it. Stating this so the question doesn't come up again.

## Design

### Package layout

```
internal/provider/
├── provider.go             // Provider interface, Event, EventKind, Result, Marker, Registry
├── testutil/
│   └── fake.go             // provider.Provider test double
├── claudecode/
│   ├── claudecode.go       // ClaudeCode implementation (composes invoke.ProcessInvoker)
│   ├── parse.go            // stream-json envelope parser (the One True Copy)
│   └── claudecode_test.go
└── raw/
    ├── raw.go              // opaque-stdout implementation
    └── raw_test.go
```

`internal/invoke/` stays in place. Its public surface shrinks: `FormatAssistantText`, `detectLineMarker`, `detectMarkers`, `Marker` type, `MarkerStringXXX` constants, and `RetryInvoker` are removed. `ProcessInvoker`, `Simple`, `LineCallback`, `Result`, `procattr_*`, `terminal_*`, and `RestoreTerminal` stay. The `Invoker` interface stays as a distinct deprecated type (not a type alias to `provider.Provider` — their signatures are incompatible, so an alias is literally impossible). `Invoker` is removed in v0.8 after call sites finish migrating.

### Core interface

```go
// Provider is the contract for running a model and interpreting its
// output. Implementations encapsulate both *how to invoke* the model
// and *how to read its stream* (envelope format, tool-call shape,
// marker positions). Every daemon, logrender, and TUI code path above
// this layer is provider-agnostic.
//
// Concurrency: all methods must be safe for concurrent use. The daemon
// may call Invoke from multiple goroutines simultaneously in parallel
// worker mode. Singletons holding shared state (HTTP clients, mutexes)
// are the provider's responsibility.
type Provider interface {
    // Name returns the canonical provider identifier used in config.
    // Stable across releases. If a provider's underlying wire format
    // changes incompatibly, ship it as a new Name (e.g., claude-code-v2).
    Name() string

    // Invoke runs the model with the given prompt and returns a Result.
    // Implementations may shell out to a CLI, make an HTTP call, or
    // anything else. See contract below.
    //
    // Contract:
    //   - workDir is an absolute filesystem path. Implementations may
    //     refuse relative paths or paths that don't exist.
    //   - stdin delivery: the prompt is made available to the model on
    //     stdin for CLI providers; the equivalent request body for API
    //     providers.
    //   - stderr: captured into Result.Stderr. Not streamed to logWriter.
    //   - logWriter, if non-nil, receives every raw line of the model's
    //     stdout before any parsing. One `fmt.Fprintln` per line.
    //   - onLine, if non-nil, is invoked once per line after the line
    //     is written to logWriter. Must not block.
    //   - ctx cancellation: implementations must abort in-flight work
    //     (SIGKILL for CLI, request cancellation for HTTP) and return
    //     whatever output was captured so far in Result.Stdout.
    //   - Partial-stream errors: Result is ALWAYS non-nil on return,
    //     even when error is non-nil. The Result carries whatever
    //     bytes the provider observed before the failure. Callers use
    //     the partial output for diagnosis and retry decisions.
    Invoke(
        ctx context.Context,
        spec ModelSpec,
        prompt string,
        workDir string,
        logWriter io.Writer,
        onLine LineCallback,
    ) (*Result, error)

    // ParseLine takes a raw line of provider output and returns the
    // structured Event it represents, or nil for a line that carries
    // no display-relevant content (blank, debug noise, unrecognized
    // envelope). Pure string → Event; never errors (no I/O, no
    // external state).
    //
    // Consistency: ParseLine(line) and ExtractText(line) must agree.
    // Specifically, for every line where ParseLine returns a non-nil
    // Event with Kind == EventText, ExtractText must return the same
    // Text value. The shared test helper provider.TestConsistency
    // enforces this in each provider's test suite.
    ParseLine(line string) *Event

    // ExtractText returns the human-readable text content of a raw
    // line, or "" if the line has no text content. Fast path for
    // the terminal-marker scanner, which runs over every line of
    // every iteration and cannot afford a full Event allocation per
    // line. Implementations may share a helper with ParseLine to
    // satisfy the consistency contract.
    ExtractText(line string) string
}
```

### Supporting types

```go
// ModelSpec is the per-call configuration a daemon hands to a provider.
// v0.7 carries only the two fields both shipping providers consume.
// Future providers that need more (endpoint, auth, timeouts) will add
// fields in their respective specs, not speculatively here.
type ModelSpec struct {
    Command string
    Args    []string
}

// Result is the output of a provider invocation. See Provider.Invoke's
// contract for guarantees on partial results during errors.
type Result struct {
    Stdout         string
    Stderr         string
    ExitCode       int
    TerminalMarker Marker
    Summary        string // set by claude-code from its "result" envelope; empty for raw
}

// Marker identifies a WOLFCASTLE_* terminal marker. The enum lives
// in internal/provider; internal/invoke.Marker becomes a type alias
// through v0.7 and is removed in v0.8.
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
    Raw        string          // original line, always populated
}

// EventKind identifies the content of an Event. The zero value is
// EventUnknown, so an uninitialized Event is never silently confused
// with a well-formed EventRaw. Providers that don't know how to
// classify a line set Kind = EventRaw explicitly.
type EventKind int

const (
    EventUnknown   EventKind = iota // zero-value sentinel; must not be returned
    EventText                       // human-readable model output
    EventThinking                   // model reasoning / scratchpad
    EventToolUse                    // model invoked a tool
    EventToolResult                 // tool returned a result
    EventRaw                        // line didn't parse as a known envelope
)

// LineCallback receives each line of model output during streaming.
// Implementations must not block; runs on the output-read goroutine.
type LineCallback func(line string)
```

**Notes on what's missing vs v2:**

- **No `Event.Level`**: was declared in v2, never set, never read. Deleted.
- **No `Event.Marker` (string)**: `Result.TerminalMarker` is the authoritative place for marker info. Having Event carry its own marker string was two representations of one fact.
- **No `EventMarker` kind**: same reason. Markers are a Result concern.
- **No `ModelSpec.Endpoint`, `ModelSpec.Model`, `ModelSpec.Extra`**: v2 had these as speculative hooks for future HTTP providers. v0.7 doesn't use them. Cut.
- **`ParseLine` returns `*Event`, not `(*Event, error)`**: nothing in a pure string parser can error.

### Registry

The registry is a package-level map written once at `init()`:

```go
// registry holds all providers keyed by Name(). Populated at init()
// time by each provider subpackage. Lookup returns singletons — one
// Provider per name for the life of the process.
var registry = map[string]Provider{}

// Register adds a Provider to the global registry. Panics on duplicate
// names; a duplicate means two providers fight over one identifier and
// silent override would be worse than a startup crash. Intended to be
// called only from init() in a provider subpackage.
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

// Names returns the registered provider names in alphabetical order,
// used by config validation to build deterministic error messages.
func Names() []string {
    names := make([]string, 0, len(registry))
    for n := range registry {
        names = append(names, n)
    }
    sort.Strings(names)
    return names
}
```

Each provider subpackage has an `init()` calling `Register(&ClaudeCode{})` / `Register(&Raw{})`. A top-level `internal/provider/all.go` blank-imports the built-in subpackages so callers get every built-in provider via one import. `all.go` is updated incrementally across phases — it imports `claudecode` in Phase 1, `raw` in Phase 5.

**Singletons.** `Lookup` returns the same instance every call. Providers may hold internal state (mutex-protected counters, future HTTP client pools), but per-model state like `Command` / `Args` lives in `ModelSpec` and is passed per call — never stored on the singleton. This rule keeps providers safe to share across concurrent iterations.

**Test fake.** `internal/provider/testutil.Fake` is a `provider.Provider` implementation designed for direct injection into test helpers. It does **not** register with the global registry; tests that want it import it and pass it to `runIteration` / `TryModelAssistedFix` / etc. explicitly. No build tags, no registration races, no parallel-universe side effects. See "Test fake" section below for the full surface.

### Config surface

`ModelDef` gains exactly one field:

```go
type ModelDef struct {
    Provider string   `json:"provider,omitempty"` // "claude-code" (default), "raw", ...
    Command  string   `json:"command,omitempty"`
    Args     []string `json:"args,omitempty"`
}
```

`config.Load()` gains a resolution pass. For each model entry:

- If `Provider == ""`, default to `"claude-code"`. Record that the default was applied; at the end of Load, emit **one** aggregated warning per config (`"8 models defaulted to provider=claude-code; add \"provider\":\"claude-code\" to each to silence"`) rather than one per model, to avoid flooding output in multi-model projects.
- If `Provider` is non-empty and unknown, return a config error that lists valid provider names. A new validation category `UNKNOWN_PROVIDER` (matching the existing `UNKNOWN_*` naming in `internal/validate`) catches this via `wolfcastle doctor`.

Helper on `ModelDef`:

```go
// ResolveProvider returns the Provider registered for this ModelDef.
// Must be called after config.Load() has defaulted empty Provider
// fields. Returns an error when the provider is unknown.
func (m ModelDef) ResolveProvider() (provider.Provider, error) {
    p, ok := provider.Lookup(m.Provider)
    if !ok {
        return nil, fmt.Errorf("unknown provider %q (known: %s)",
            m.Provider, strings.Join(provider.Names(), ", "))
    }
    return p, nil
}

// Spec returns a provider.ModelSpec built from this ModelDef.
func (m ModelDef) Spec() provider.ModelSpec {
    return provider.ModelSpec{Command: m.Command, Args: m.Args}
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

1. **`runIteration`**, **`runPlanningPass`**, **`invokeWithRetry`** stop accepting `config.ModelDef` and accept a `provider.Provider` plus a `provider.ModelSpec` instead. Resolution happens **once per stage invocation**, not once per iteration — each stage of an iteration (planning, worker, exploratory review) can have its own model with its own provider, and the resolution is on the hot path at stage start.

2. **`scanTerminalMarker`** in `iteration.go` stops embedding Claude Code envelope knowledge. It calls `provider.ExtractText(line)` per line to get the text content, then searches for `WOLFCASTLE_*` substrings. The scanner runs over `result.Stdout` captured in memory by `Invoke`, not over log records on disk, so the Phase 4 log-format work doesn't affect the scanner. Priority-ordered marker resolution (`COMPLETE > BLOCKED > YIELD`) stays in the scanner because it's provider-agnostic.

3. **`internal/validate/model_fix.go:TryModelAssistedFix`** currently takes `invoke.Invoker` + `config.ModelDef`. It's updated to take `provider.Provider` + `provider.ModelSpec`. Only caller: `cmd/doctor.go:135`. `doctor.go`'s migration is explicit in Phase 3 — it resolves the provider from config and passes both handles in.

4. **`cmd/unblock.go`** currently uses `invoke.Simple` and `invoke.FormatAssistantText`. Migration: `provider.Lookup("claude-code")` returns the singleton; `.Invoke(...)` replaces `invoke.Simple`; `.ParseLine(line).Text` replaces `invoke.FormatAssistantText`. No behavioral change.

5. **`cmd/cmdutil/app.go`** has `Invoker invoke.Invoker` for dependency injection in command tests. It becomes `Provider provider.Provider`, defaulting to `claude-code`.

### Log record format

The on-disk log envelope keeps its historical `type: "assistant"` tag. Renaming would break every external `jq` tailer matching `.type == "assistant"`, and "pay a documentation cost" is cheaper than "break all downstream scripts." Instead, Phase 4 **adds** a `provider` field alongside the existing `type`:

```json
{"level":"debug","type":"assistant","provider":"claude-code","trace":"exec-0042",
 "timestamp":"...","text":"{\"type\":\"assistant\",\"message\":..."}
```

```json
{"level":"debug","type":"assistant","provider":"raw","trace":"worker-leaf-task-0001",
 "timestamp":"...","text":"Working on the task...\n"}
```

The inner `text` is whatever the provider's stdout produced, byte-for-byte. Each renderer reads the outer envelope, finds the provider, and hands the inner text to `provider.ParseLine(...)` for display. Records without a `provider` field fall back to `claude-code`, which preserves every existing `.jsonl` on disk.

Implementation:

- `logging.Logger.ProviderStreamWriter(providerName string) io.Writer` is the new method. Returns a writer that tags records with both `type: "assistant"` (for compat) and `provider: <name>` (for routing).
- `AssistantWriter()` stays, unchanged, forwarding to `ProviderStreamWriter("claude-code")`. There is no deprecation; it remains a legitimate API for callers that genuinely want the historical behavior. In-tree daemon call sites (`iteration.go`, `planning.go`, `stages.go`) switch to `ProviderStreamWriter(prov.Name())`.
- `logrender.Record` gains a `Provider string` field. The reader at `internal/logrender/reader.go` populates it from the outer envelope if present, otherwise leaves it empty (renderers default empty to `claude-code`).
- Renderers in `logrender/thoughts.go` and `logrender/interleaved.go` drop their local `extractThoughtText` parser and call `provider.Lookup(rec.Provider).ParseLine(rec.Text)` instead.

### TUI integration

`internal/tui/detail/logview.go:extractAssistantContent(raw string) string` is replaced by `formatByProvider(rec logrender.Record) string`:

```go
func formatByProvider(rec logrender.Record) string {
    name := rec.Provider
    if name == "" {
        name = "claude-code" // legacy fallback for pre-v0.7 records
    }
    prov, ok := provider.Lookup(name)
    if !ok {
        return rec.Text
    }
    ev := prov.ParseLine(rec.Text)
    if ev == nil {
        return rec.Text
    }
    return renderEvent(ev) // existing icon/style logic, operates on Event
}
```

`renderEvent` keeps today's styling: `tool_use` → `[tool: name]`, `tool_result` → `[tool result] …`, `thinking` → `[thinking] …`, `text` → plain. It operates on `Event`, not on raw strings, so adding a new provider doesn't touch the TUI's rendering code.

**Mixed-provider sessions** work because each log record names its provider. The TUI never needs to know which stage ran which model — it reads the provider off each record.

### `internal/invoke` package fate

Concrete inventory:

| Symbol | Fate in v0.7 |
|---|---|
| `invoke.ProcessInvoker` | Stays. Internal primitive for CLI spawn + stall detection. Composed by `claudecode`. |
| `invoke.Simple` | Stays as a thin wrapper over `provider.Lookup("claude-code").Invoke(...)` for test compatibility. Deprecation in v0.8. |
| `invoke.LineCallback` | Stays as a type alias for `provider.LineCallback`. |
| `invoke.Result` | Stays as a type alias for `provider.Result`. |
| `invoke.Invoker` (interface) | Stays as a **distinct deprecated interface**. Cannot be a type alias because `provider.Provider` has different signatures and extra methods. Removed in v0.8. |
| `invoke.FormatAssistantText` | Removed in Phase 3. Copied into `claudecode/parse.go` first (Phase 1), then deleted from `invoke` once the one caller (`cmd/unblock.go`) migrates (Phase 3). |
| `invoke.Marker`, `MarkerStringXXX` constants | Removed in Phase 3. `provider.Marker` is the new source. |
| `invoke.detectLineMarker`, `detectMarkers` | Removed in Phase 3. Logic folded into `claudecode/parse.go`. |
| `invoke.RetryInvoker`, `NewRetryInvoker`, `RetryLogger` | **Deleted in Phase 3.** Defined but never instantiated outside its own tests; the daemon has its own retry loop at `daemon/retry.go:invokeWithRetry` which is the only production retry path. The unique feature of `RetryInvoker` (a separate `RetryLogger` interface) is already provided more directly by `daemon/retry.go`'s inline log calls. `retry_test.go` is deleted; anything it exercised that wasn't also covered in daemon tests becomes a daemon test. |
| `invoke.procattr_unix.go`, `procattr_windows.go` | Stay. CLI infrastructure. |
| `invoke.terminal_*.go`, `RestoreTerminal` | Stay. CLI infrastructure. |

**Non-CLI providers don't import `internal/invoke`.** `claudecode` composes `ProcessInvoker` internally for its CLI call. `raw` also composes `ProcessInvoker` (it's still a CLI provider, just one that doesn't parse output). A hypothetical future HTTP provider imports neither.

### Stream formats shipped

- **`claude-code`**: parses Claude Code's stream-json envelopes. Returns `EventText` for `text` content blocks, `EventThinking` for `thinking` blocks, `EventToolUse` for `tool_use` blocks (with `ToolInput` as the raw JSON of `content[i].input`), `EventToolResult` for `tool_result` blocks. `tool_result.content` handling replicates the current logic in `tui/detail/logview.go:extractAssistantContent`: content may be either a plain string or an array of `{type: "text", text: "..."}` blocks; both shapes decode to the same `EventToolResult.ToolOutput`. Unrecognized envelopes (`system`, `result`, any unknown) return `EventRaw` with the original line in `Raw`. `Result.Summary` is populated from the `result` envelope's `.result` field if one appears in stdout (matching `invoke.FormatAssistantText`'s current behavior).

- **`raw`**: `ParseLine` returns `&Event{Kind: EventText, Text: line, Raw: line}` for every non-blank line. `ExtractText(line) == line`. No tool extraction, no thinking blocks, no envelope awareness. Terminal-marker scanning works via substring match on the raw line. **False-positive risk**: if a model echoes its prompt and the prompt contains `WOLFCASTLE_COMPLETE` as literal text (e.g., documentation strings), the marker scanner triggers early. Acceptable tradeoff for v0.7 — users of `raw` are running minimal bring-your-own-agent setups and can work around with stricter prompts. Documented in the `raw` provider's package doc.

### Test fake

`internal/provider/testutil/fake.go`:

```go
// Fake is a programmable Provider for tests. Import and inject
// directly; it does not register with the global registry.
//
// Example:
//
//     fake := &testutil.Fake{
//         Results: []*provider.Result{
//             {Stdout: "WOLFCASTLE_COMPLETE\n", TerminalMarker: provider.MarkerComplete},
//         },
//     }
//     d.runIteration(ctx, fake, provider.ModelSpec{}, nav, idx)
//
type Fake struct {
    // Results are returned from successive Invoke calls, popped in
    // FIFO order. A nil entry returns (nil, errors.New("fake: no
    // results queued")).
    Results []*provider.Result

    // ParseFunc, if set, overrides the default ParseLine. The default
    // wraps every non-blank line in &Event{Kind: EventText, ...}.
    ParseFunc func(line string) *provider.Event

    // ExtractFunc, if set, overrides the default ExtractText. The
    // default returns the line unchanged.
    ExtractFunc func(line string) string

    // Calls captures each invocation for post-assertion.
    mu    sync.Mutex
    Calls []FakeCall
}

type FakeCall struct {
    Spec    provider.ModelSpec
    Prompt  string
    WorkDir string
}

func (f *Fake) Name() string                       { return "fake" }
func (f *Fake) Invoke(...) (*provider.Result, error) { ... }
func (f *Fake) ParseLine(line string) *provider.Event { ... }
func (f *Fake) ExtractText(line string) string      { ... }
```

Three existing mock invokers collapse into `testutil.Fake` during Phase 3:

- `cmd/unblock_interactive_test.go:fakeInvoker` (8 test cases)
- `cmd/doctor_invoker_test.go:mockInvoker` (8 test cases)
- `cmd/audit/codebase_invoker_test.go:mockInvoker` (5 test cases)

Each of those files currently reimplements the same "record calls, return canned results" pattern against `invoke.Invoker`. Phase 3 rewrites them as thin wrappers around `testutil.Fake` injected as a `provider.Provider`. The daemon's existing test-helper functions (`testDaemon`, `testConfig`, etc. in `internal/daemon/daemon_test.go`) do not carry an `Invoker` field on `Daemon` — invocation is threaded through call parameters — so the helpers themselves need minimal changes.

## Backwards compatibility

- **Configs without `provider`**: default to `claude-code`. One aggregated warning per config on load. Behavior unchanged.
- **`config.ModelDef{Command: ..., Args: ...}` literals**: non-test sources have **7 occurrences** (Agent A audit). Test sources have many more — Agent B counted ~146 files touching `ModelDef`. Phase 2 adds the `Provider` field where missing, all compiler-verified. The test-side work is tedious but mechanical, done file-by-file as Phase 3 migrates each consumer.
- **`invoke.Invoker`, `invoke.Result`, `invoke.LineCallback`, `invoke.Simple`**: remain through v0.7. `Result` and `LineCallback` are type aliases; `Invoker` is a distinct interface (alias impossible); `Simple` is a shim. All four removed in v0.8.
- **On-disk `.jsonl` logs** from v0.6.x continue to render correctly: records without a `provider` field fall back to `claude-code` in all renderers.
- **External jq/tailer scripts** keyed on `.type == "assistant"` keep working because the outer envelope tag is unchanged.

## Migration plan

Five phases, one PR each, all against `release/0.7`.

### Phase 1 — interface + `claudecode` (no daemon changes)
- New package `internal/provider` with `Provider`, `Event`, `Result`, `Marker`, `ModelSpec`, `LineCallback`, registry API.
- New subpackage `internal/provider/claudecode` with `claudecode.go` (composing `invoke.ProcessInvoker`) and `parse.go` (**copies**, not moves, `FormatAssistantText`/`detectLineMarker` logic plus the `tool_result`-string-or-array handling from `logview.go`). The original `invoke.FormatAssistantText` stays in place with its single caller, marked as deprecated.
- Consistency test helper `provider.TestConsistency(t, p)` that verifies `ExtractText(line) == ParseLine(line).Text` for every `EventText` case across a shared corpus of lines. Both `claudecode` and (in Phase 5) `raw` must pass it.
- `internal/provider/all.go` imports `claudecode`.
- Daemon and existing call sites untouched. Zero behavioral change.

### Phase 2 — config field + validation + test fake (dormant)
- `ModelDef.Provider` field with defaulting in `config.Load()` and one aggregated warning.
- `UNKNOWN_PROVIDER` validation category in `internal/validate`; `wolfcastle doctor` reports it.
- `internal/provider/testutil.Fake` ships, with no registry side effects. It's usable from tests but nothing injects it yet — the daemon entry points still take `invoke.Invoker`. Phase 3 wires it in.
- Test fixtures (source + tests) that construct `ModelDef{Command: ..., Args: ...}` literals get the `Provider: "claude-code"` field added. Compiler-verified.

### Phase 3 — daemon runtime through `Provider`
- `runIteration`, `runPlanningPass`, `invokeWithRetry`, `TryModelAssistedFix` swap `invoke.Invoker` + `config.ModelDef` for `provider.Provider` + `provider.ModelSpec`.
- `scanTerminalMarker` in `iteration.go` delegates per-line text extraction to `provider.ExtractText`.
- `cmd/unblock.go`, `cmd/doctor.go`, `cmd/cmdutil/app.go` migrate. `App.Invoker` becomes `App.Provider`.
- `invoke.FormatAssistantText`, `invoke.Marker*`, `invoke.detectLineMarker`, `invoke.detectMarkers` deleted (they now live in `claudecode/parse.go`).
- `invoke.RetryInvoker` deleted. Its test file (`retry_test.go`) is deleted; its unique coverage is already in `daemon/retry_test.go`.
- Three existing mock invokers (`fakeInvoker`, `mockInvoker` × 2) rewritten as wrappers around `testutil.Fake`.
- 18 `invoke.FormatAssistantText` call sites in `cmd/daemon/follow_test.go` migrate to `provider.Lookup("claude-code").ParseLine(...).Text`.

### Phase 4 — log record field + renderers + TUI
- `logging.Logger.ProviderStreamWriter(providerName)` ships.
- Daemon call sites in `iteration.go`, `planning.go`, `stages.go` switch from `d.Logger.AssistantWriter()` to `d.Logger.ProviderStreamWriter(prov.Name())`.
- Log records gain a `provider` field alongside the historical `type: "assistant"`.
- `logrender.Record` gains a `Provider string` field; the reader populates it from the outer envelope.
- `logrender/thoughts.go` and `logrender/interleaved.go` drop their local `extractThoughtText` parser and call `provider.Lookup(rec.Provider).ParseLine(rec.Text)`.
- `tui/detail/logview.go` migrates `extractAssistantContent` → `formatByProvider`.
- No on-disk format break: the outer `type` stays `"assistant"`, scripting consumers keep working.
- TUI acceptance test pass (`teatest`): log modal renders text, tool_use, tool_result, thinking correctly under claude-code.

### Phase 5 — `raw` provider + live smoke
- `internal/provider/raw` ships with unit tests and passes `provider.TestConsistency`.
- `internal/provider/all.go` imports `raw`.
- Daemon integration test under `raw`: mock shell script emits plain text ending in `WOLFCASTLE_COMPLETE`, iteration completes, daemon commits.
- Live smoke test: real wolfcastle project end-to-end with `raw` + `ollama run llama3:70b`. Success criterion: one leaf task completes, state propagates, TUI log modal shows output. Failure is a blocker.
- README and `docs/humans/configuration.md` updated. CHANGELOG entry.
- `release/0.7` opens its PR into `main`, combined 0.7 → v0.7.0 release is cut.

## Risks

- **Test migration scope underestimation.** Agent B counted ~146 files referencing `ModelDef{}` across tests — Phase 2 and Phase 3 touch every one. All compiler-verified and mechanical, but it's real hours. Mitigation: land `testutil.Fake` in Phase 2, so Phase 3 file-by-file migration swaps shell-out mocks for `Fake` at the same time it adds the `Provider` field.
- **`follow_test.go` as a hot spot.** 18 call sites to `invoke.FormatAssistantText` in one file. Migration is a mechanical replace-all but should ship as a single commit so reviewers can verify.
- **`ExtractText` / `ParseLine` drift.** Two methods that must agree on text content. Mitigated by `provider.TestConsistency`, which every provider must call in its `_test.go`. If we're paranoid, add a reflection-based test at the `provider` package level that asserts every registered provider passes consistency, running in CI.
- **Concurrent `Invoke` on a singleton.** `claudecode` is safe because each call spawns its own subprocess. Any future HTTP provider must declare itself safe for concurrent use or the daemon will break it. The interface comment is explicit; CI adds a smoke test that launches two parallel goroutines calling `claudecode.Invoke` against a mock CLI.
- **`raw` false-positive markers.** A chatty model echoing its prompt can trip `WOLFCASTLE_COMPLETE` matching. Documented in `raw`'s package doc; users of `raw` pick their prompt carefully or use `claude-code`.
- **Phase 4 rollback is clean.** The outer envelope stays `type: "assistant"`; only a new `provider` field is added. Rollback means reverting the Phase 4 commit, and pre-rollback logs still render (the new field is ignored, renderers default to claude-code). No data corruption risk.
- **Event drift on future kinds.** Multimodal models (images, audio, video) add event shapes we haven't modeled. Mitigation: `Event.Raw` is always populated, so renderers that don't know a new kind fall back to displaying the raw line. New kinds are additive; no renderer breaks.
- **Config hot-reload is not supported** for provider changes. Editing `provider` in a model mid-daemon requires a restart. This is consistent with all other config fields (models, prompts, planning) — wolfcastle has no hot-reload story in general. State in `docs/humans/configuration.md`.

## Verification

1. **Unit tests per provider.** `claudecode_test.go` and `raw_test.go` cover `ParseLine` (happy, empty, malformed, tool_use, tool_result as string and array), `ExtractText`, marker detection, and `Invoke` with a mock CLI. Both call `provider.TestConsistency(t, p)`.
2. **Registry tests.** `TestLookup_ReturnsSingleton`, `TestLookup_UnknownProvider`, `TestRegister_Duplicate_Panics`, `TestNames_Alphabetical`.
3. **Config resolution tests.** `TestConfigLoad_DefaultsEmptyProvider`, `TestConfigLoad_AggregatedWarning`, `TestConfigLoad_UnknownProvider_Errors`.
4. **Daemon integration tests.** Existing `TestDaemon_ExploratoryReview_CreatesRemediationLeaf` passes unchanged under `claude-code`. New `TestDaemon_RawProvider_CompletesTask` runs the daemon loop under `raw` with a shell-script mock model; both use `testutil.Fake` where possible.
5. **Concurrent invocation test.** `TestClaudeCode_Invoke_Concurrent` launches N goroutines calling `Invoke` on the singleton and asserts each completes with its own Result, no crosstalk.
6. **TUI acceptance tests.** `teatest` scripts verify the log modal under `claude-code` (text, tool_use, tool_result, thinking), `raw` (opaque text), and a **mixed-provider** session confirming per-record dispatch.
7. **Log backwards-compat test.** Load a v0.6.x-style `.jsonl` (no `provider` field) into `logrender`, confirm every record renders via the `claude-code` fallback, nothing drops, no errors.
8. **Scripting-user contract test.** A small test reads a v0.7-era log file with `jq '.type == "assistant"'` and confirms the result count matches the number of model-output records. This pins the scripting contract we committed to preserving.
9. **Live ollama smoke.** Final pre-merge step: real project, real ollama binary, one leaf task completes. Failure blocks the v0.7.0 merge.

## Implementation sequence

Matches Migration Plan 1:1. Phase numbering is consistent throughout the spec.

1. **Phase 1**: `internal/provider` + `claudecode` + `provider.TestConsistency`. No daemon changes.
2. **Phase 2**: `ModelDef.Provider` + defaulting + aggregated warning + `UNKNOWN_PROVIDER` validation + `testutil.Fake` (dormant).
3. **Phase 3**: daemon runtime migration (`runIteration`, `runPlanningPass`, `invokeWithRetry`, `TryModelAssistedFix`, `cmd/unblock.go`, `cmd/doctor.go`, `cmd/cmdutil/app.go`). `invoke.RetryInvoker`, `FormatAssistantText`, `Marker*`, `detectLineMarker`, `detectMarkers` deleted. Three mock invokers collapse into `Fake`.
4. **Phase 4**: `ProviderStreamWriter` + `logrender.Record.Provider` field + `logrender` renderers + `tui/detail/logview.go` migration. Outer envelope unchanged; `provider` field added. TUI acceptance tests.
5. **Phase 5**: `raw` provider + `all.go` adds `raw` import + daemon integration test + live ollama smoke + docs + CHANGELOG + PR into main + tag v0.7.0.
