# ADR-062: Realistic Model Mocks for Integration Testing

## Status
Accepted

## Date
2026-03-14

## Context

The initial integration tests (ADR-050) used trivial mocks: a shell script that emitted a terminal marker and exited. This was enough to verify that the daemon loop ran at all, but it missed an entire class of bugs that only surface when the model behaves like a real model. Prompt echo false positives, YIELD/COMPLETE confusion, task state transition ordering, JSON stream envelope parsing, multi-iteration counter drift: all of these slipped through because the simple mocks never read stdin, never created files, never called `wolfcastle` CLI commands, and never varied their behavior across invocations.

Real model execution is a conversation between the daemon and a subprocess. The daemon pipes an assembled prompt on stdin, the subprocess does work (reading code, creating files, calling CLI commands), and then signals completion via a terminal marker embedded in Claude Code's stream-json format. Testing this contract requires mocks that participate in it faithfully.

## Decision

Build a configurable mock model system using shell scripts that replicate the essential behaviors of a real model subprocess:

**Prompt validation.** Mock scripts read stdin (the assembled prompt) and use `grep` to check for expected content: node addresses, task descriptions, task IDs, prompt tier markers. Results are written to an assertion file that the test reads after the iteration completes.

**Side effects.** Scripts can create files in the working directory, simulating real model output. The `create-file` behavior, for instance, touches a file and verifies the daemon's working directory is set correctly.

**Terminal markers in stream-json format.** All mock output uses the Claude Code JSON envelope (`{"type":"result","text":"WOLFCASTLE_COMPLETE"}`), exercising the same parsing path that real model output takes. This catches envelope format regressions that raw-text mocks would miss.

**Multi-invocation scenarios via counter files.** A counter file on disk tracks how many times the script has been called. The script reads the counter, increments it, and chooses its behavior accordingly: yield for the first N calls, complete on call N+1. This pattern tests the daemon's iteration lifecycle without timing dependencies.

**Assertion files for test verification.** Rather than parsing script stdout (which the daemon consumes), mocks write structured assertion data to sidecar files. Tests read these files after the daemon exits to verify prompt content, invocation counts, and execution ordering.

The system provides two layers of mock infrastructure:

1. **`internal/daemon/` integration tests** use `testDaemon()` to construct a `Daemon` struct directly, configure a model definition pointing at a shell script, and call `RunOnce()` in a loop. This tests the daemon's internal iteration logic without subprocess overhead.

2. **`test/integration/` tests** use helper functions (`createMockModel`, `createCounterMock`, `createNoMarkerStopAfterMock`) that generate shell scripts at runtime and configure them via `configureMockModels`. These tests exercise the full CLI path: `wolfcastle start` runs the daemon as it would in production, with the mock script standing in for the real model.

## Consequences

- Integration tests exercise the full daemon loop with realistic model behavior, completing in milliseconds rather than the minutes a real model invocation would take.
- New model behaviors are added by writing a new shell script body in the mock helper functions or by configuring existing helpers with different parameters. No new test binaries or complex fixtures required.
- The mock protocol serves as executable documentation of the contract between the daemon and any model adapter: read stdin, do work, emit stream-json with a terminal marker.
- Prompt echo rejection, JSON envelope parsing, multi-iteration state transitions, and failure escalation are all covered by tests that would have been impractical with trivial mocks.
- Counter-based multi-invocation mocks eliminate timing-dependent tests. The daemon runs at full speed; the mock's behavior is determined by invocation count, not wall-clock time.
- The assertion file pattern decouples test verification from the daemon's stdout consumption, making it possible to validate what the model "saw" without intercepting the daemon's I/O pipeline.
