# Model Provider Abstraction

## Status
Draft

## Problem

The README promises: *"Agents are defined as CLI commands. Anything that reads stdin and writes stdout works: Claude Code, Cursor, Copilot, GPT, Gemini, Llama, a bash script wrapping curl. Your agents, your choice. Switch providers by editing a JSON file."*

That promise is half-true. `internal/invoke.ProcessInvoker` really will execute any CLI that reads stdin and writes stdout. But the surrounding daemon only produces useful behavior when that CLI happens to emit **Claude Code's stream-json format**. Three codepaths are hard-coded to that format:

1. **`internal/daemon/iteration.go:extractAssistantText`** parses `{"type":"assistant","message":{"content":[{"type":"text","text":"..."}]}}` envelopes to pull out human-readable text for terminal-marker scanning and log rendering.
2. **`internal/logrender/*`** (summary, interleaved, thoughts renderers) all extract assistant text, tool-call names, thinking blocks from the same envelope shape.
3. **`internal/tui/detail/logview.go`** decodes `tool_use` / `tool_result` / `thinking` content blocks for display.

Any non-Claude agent that produces different output either (a) ships no visible text through the TUI log modal, or (b) fails to trigger terminal markers because the markers are buried inside unparseable JSON and never reach `scanTerminalMarker`. The practical effect is that "switch providers by editing a JSON file" is only true for agents that impersonate Claude Code's wire format — which is a very short list.

The config's `ModelDef` is minimal:

```go
type ModelDef struct {
    Command string   `json:"command"`
    Args    []string `json:"args"`
}
```

There's nowhere to declare *how the output should be parsed*. The daemon assumes one answer for every model.

## Goals

1. **A named provider per model** in config, orthogonal to the CLI command. `claude-code` is the default; `openai`, `ollama`, `anthropic-api`, `raw` are plausible additions.
2. **A `Provider` interface** under `internal/provider` that owns both invocation *and* stream parsing. Daemon code calls into the provider instead of reaching for raw stdout.
3. **Backwards compatibility.** Existing configs without a `provider` field keep working as Claude Code. No prompt changes, no migration required for existing projects.
4. **At least two shipping providers in v0.7:** `claude-code` (lifted from current code) and one other. Most likely `raw` (which treats stdout as opaque text and only scans for markers) — it's the minimum viable "bring your own agent" provider and needs zero API credentials to build and test.
5. **Pluggable log rendering.** The TUI and log renderers get a provider handle and ask it to format a line/envelope/block. No more hard-coded stream-json parsing in `logrender` or `tui/detail/logview.go`.

## Non-goals

- **OpenAI / Anthropic API clients in v0.7.** Those are real providers we'll want eventually, but they add real-network test infrastructure, auth plumbing, and cost controls that deserve their own spec. v0.7 establishes the interface; v0.8 can fill in one more concrete adapter if demand is there.
- **A universal tool-call format.** Each provider parses its own tool invocations into a common `ToolEvent` shape, but we're not building a cross-provider tool-calling abstraction on top of that — models invoke whatever tools their native format exposes.
- **Per-stage provider selection at runtime via prompt instruction.** Providers are chosen at config time, not per-call by the model.

## Design

### Package layout

New package `internal/provider`:

```
internal/provider/
├── provider.go          // Provider interface + Registry + Event types
├── claudecode/
│   ├── claudecode.go    // ClaudeCode implements Provider; stream-json parser
│   └── claudecode_test.go
├── raw/
│   ├── raw.go           // Raw implements Provider; opaque stdout, marker-only
│   └── raw_test.go
└── provider_test.go     // Registry round-trip, config resolution
```

### Core interface

```go
// Provider is the contract for running a model and interpreting its output.
// Implementations encapsulate both *how to invoke* the model (the CLI / API
// contract) and *how to read its stream* (envelope format, tool-call shape,
// marker positions). Everything in the daemon above this layer should be
// provider-agnostic.
type Provider interface {
    // Name returns the canonical provider identifier as it appears in
    // config (e.g., "claude-code", "raw", "openai").
    Name() string

    // Invoke runs the model with the given prompt and returns a Result.
    // Implementations may shell out to a CLI, make an HTTP call, or
    // anything else. logWriter, if non-nil, receives each NDJSON-ready
    // Event as it streams. onLine, if non-nil, receives the raw line
    // before any envelope parsing so legacy callers can still tail.
    Invoke(ctx context.Context, spec ModelSpec, prompt string, workDir string, logWriter io.Writer, onLine LineCallback) (*Result, error)

    // ParseLine extracts a structured Event from a single raw line of
    // output. The daemon calls this during streaming so logrender and
    // the TUI have a uniform shape to render across providers.
    // Returns (nil, nil) for lines that are not envelope-bearing
    // (blank lines, debug noise).
    ParseLine(line string) (*Event, error)

    // ExtractText returns the human-readable text from a raw line, if
    // any. Equivalent to ParseLine(line).Text when the event is a text
    // block. Retained as a fast path for the marker scanner, which
    // only cares about text content.
    ExtractText(line string) string
}

// ModelSpec replaces ModelDef for provider-aware invocation. It carries
// the raw config fields; each provider documents which it consumes.
type ModelSpec struct {
    Command  string            // CLI command (CLI providers)
    Args     []string          // CLI args (CLI providers)
    Endpoint string            // API endpoint (HTTP providers)
    Model    string            // model identifier (HTTP providers)
    Extra    map[string]any    // provider-specific overflow
}

// Event is the common shape every provider's output is normalized into.
// The daemon and logrender and the TUI all consume Events, never raw
// lines, so adding a new provider only touches the parser.
type Event struct {
    Kind      EventKind // text | thinking | tool_use | tool_result | marker | raw
    Text      string    // populated for text / thinking
    ToolName  string    // populated for tool_use
    ToolInput string    // populated for tool_use; provider-formatted
    ToolOutput string   // populated for tool_result
    Marker    string    // populated for marker
    Raw       string    // original line, always populated
    Level     string    // "debug" | "info" | "warn" | "error"
}

type EventKind int

const (
    EventRaw EventKind = iota
    EventText
    EventThinking
    EventToolUse
    EventToolResult
    EventMarker
)
```

`Result` is the same shape as `invoke.Result` today (stdout / stderr / exit code / terminal marker / summary). The daemon already owns that type; we re-export or move it into `internal/provider`.

### Registry

```go
// Registry maps provider names to constructors. Providers register
// themselves via init() in their respective subpackages. The daemon
// calls Lookup(name) when resolving a model from config.
var Registry = map[string]Constructor{}

type Constructor func() Provider

func Register(name string, ctor Constructor) { Registry[name] = ctor }
func Lookup(name string) (Provider, bool)    { ... }
```

`init()` in `claudecode` and `raw` registers each. The daemon imports the subpackages for side effects in a single `internal/provider/all.go` so callers get every built-in provider without touching individual imports.

### Config surface

`ModelDef` grows a `Provider` field with a safe default:

```go
type ModelDef struct {
    Provider string   `json:"provider,omitempty"` // "claude-code", "raw", ...
    Command  string   `json:"command,omitempty"`
    Args     []string `json:"args,omitempty"`
    Endpoint string   `json:"endpoint,omitempty"`
    Model    string   `json:"model,omitempty"`
}
```

`config.Load()` resolves each entry: if `Provider == ""`, defaults to `"claude-code"`; if the provider name is unknown, returns a loud config error (not a silent fallback). A new validation category `UNKNOWN_PROVIDER` catches typos.

Example config:

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

The `raw` provider in this example runs `ollama run llama3:70b` and treats the output as plain text — marker scanning still works, but there's no structured tool-call extraction. That's a deliberate first-rung option: "my model emits text, leave it alone."

### Daemon integration

1. **`runIteration` + `runPlanningPass` + `invokeWithRetry`** stop accepting a `config.ModelDef` and start accepting a resolved `provider.Provider`. The iteration's chosen model is looked up at the start of the iteration via `provider.Lookup(modelDef.Provider).Build(modelDef)`.

2. **`invoke.ProcessInvoker`** moves into `internal/provider/claudecode/` as an implementation detail. The generic `Invoker` interface shrinks to whatever is truly CLI-shared (probably just the stall-timer wrapper). Other providers can reuse or ignore it as they see fit.

3. **`scanTerminalMarker`** in `iteration.go` delegates to `provider.ExtractText(line)` instead of hard-coding `extractAssistantText`. Moves out of daemon and into the claude-code provider; the daemon just calls `provider.ScanTerminalMarker(output, markers)`.

4. **`logrender.extractAssistantContent`** moves into claudecode as well; the daemon passes the provider handle when constructing a renderer.

### TUI integration

1. The log modal's `extractAssistantContent` in `internal/tui/detail/logview.go` becomes `provider.FormatLine(line)` — a display-only transformation the provider owns.
2. The TUI tab's active provider is stashed on the tab struct when the tab connects, so every log line the watcher emits can be formatted correctly.

### Stream format support

v0.7 ships two providers:

- **`claude-code`**: parses `{"type":"assistant","message":{"content":[...]}}` envelopes with text / tool_use / tool_result / thinking blocks. Lifted from current code, no behavioral change.
- **`raw`**: treats every output line as opaque text. Every line becomes an `EventText` with `Raw == Text`. Terminal markers are scanned as substring matches over the line, same as the current fallback path in `scanTerminalMarker` when `extractAssistantText` returns empty. Works with any stdin/stdout command.

### Backwards compatibility

- Configs without a `provider` field: resolve to `claude-code`. Loud warning on load: `"model 'heavy' has no provider field; defaulting to claude-code. Add \"provider\": \"claude-code\" to silence."` The warning is informational, not fatal. v0.8 can upgrade to an error.
- Test fixtures across `internal/daemon`, `internal/validate`, `internal/pipeline`: audit for any that construct a `ModelDef{Command: ..., Args: ...}` directly and backfill `Provider: "claude-code"`.
- `internal/invoke.Simple` and `Invoker` interface: kept for compatibility with `validate/model_fix.go` (which uses an ad-hoc invocation). They become thin wrappers over the claude-code provider's CLI path.

### Migration plan

1. **Phase 1 — introduce the interface, lift claude-code.** New package `internal/provider` with the interface, registry, and `claudecode` subpackage. Daemon still uses `config.ModelDef` everywhere; providers are constructed at `Run()` entry from the config. Zero behavioral change.
2. **Phase 2 — thread `Provider` through the daemon.** `runIteration` / `runPlanningPass` / `invokeWithRetry` take a `Provider` instead of `ModelDef`. `scanTerminalMarker` and `extractAssistantText` move into `claudecode`. Still one provider; everything still works the same.
3. **Phase 3 — thread `Provider` through log rendering.** `logrender` and `tui/detail/logview.go` consume `Event` instead of parsing stream-json. Claude-code continues to emit the same Events; visually nothing changes.
4. **Phase 4 — add the `raw` provider.** With no existing model producing `raw` output, this phase is purely additive: new code, new test, new docs entry. A local `ollama run llama3` smoke test verifies a non-claude model can complete a task.
5. **Phase 5 — config validation + docs.** The `UNKNOWN_PROVIDER` validator, updated `docs/humans/configuration.md`, updated README.

Each phase is its own PR against `release/0.7`. Phase 1 can be minutes after merging the spec. Phase 5 is the last commit before `release/0.7` opens its PR back into `main`.

## Risks

- **Test fixture churn.** Every daemon / validate / pipeline test that uses a `ModelDef` literal needs a `Provider` field. That's ~40-60 test files based on a rough grep. Mechanical, compiler-verified, but tedious. Mitigate by writing a small `testutil.DefaultModelSpec()` helper early.
- **Interface surface creep.** Providers that need to do extra things (rate limiting, per-call auth, cost reporting) will want to push state through the interface. Keep the v0.7 interface minimal: Name, Invoke, ParseLine, ExtractText. Everything else is provider-internal.
- **Registering providers at init() time vs. dependency injection.** Go's package `init()` is fine for a small finite set and matches how `git.Provider` is done. For v0.7 we use init-registration; if we add network-configured providers later (API keys, endpoints per tenant) we revisit.
- **Log format churn on disk.** The NDJSON records on disk are still whatever the provider writes line-by-line. `Event` is an in-memory abstraction, not a wire format. Existing tooling that tails the `.jsonl` files continues to see provider-native JSON. If later we want a provider-normalized on-disk format that's a follow-up decision.

## Verification

1. **Unit tests per provider.** Each provider subpackage has `_test.go` covering: ParseLine handles happy/empty/malformed lines, ExtractText returns the right substring, marker detection, tool_use round-trip.
2. **Daemon integration test.** Existing `TestDaemon_ExploratoryReview_CreatesRemediationLeaf` runs under `claude-code` (today's behavior); a parallel test under `raw` using a mock CLI that echoes a fixed sequence verifies the daemon handles a provider that can't emit structured events.
3. **TUI acceptance tests.** `teatest` scripts covering the log modal under both providers — claude-code renders tool calls and text, raw renders everything as text.
4. **Migration smoke test.** Load an existing 0.6.x config (no `provider` field) and confirm the daemon starts, processes a task, and emits a `provider_default` info record to the daemon log.
5. **Live test with ollama.** The branch's final pre-PR step is to run a real wolfcastle project end-to-end with `raw` + a local Llama model and watch it complete at least one leaf task. If that fails we know the spec was too optimistic about what "raw" can do.

## Implementation sequence

Each phase is a separate commit/PR on `release/0.7`:

1. Add `internal/provider` package with `Provider` interface, `Registry`, and `Event` types. Add `claudecode` subpackage that is literally `invoke.ProcessInvoker` under a new name. No daemon changes.
2. Add `Provider` field to `ModelDef`. Default to `claude-code` on load. Validation category `UNKNOWN_PROVIDER`.
3. Thread `Provider` through `runIteration`, `runPlanningPass`, `invokeWithRetry`. Move `scanTerminalMarker` and `extractAssistantText` into `claudecode`. Daemon tests updated.
4. Thread `Provider` through `logrender` and `tui/detail/logview.go`. TUI tests updated.
5. Add `internal/provider/raw`. Minimal unit tests. Integration test under `raw`.
6. Update `docs/humans/configuration.md` and README to describe providers explicitly. Update CHANGELOG.
7. Live smoke test with a non-claude model (ollama or similar). Fix whatever breaks.
