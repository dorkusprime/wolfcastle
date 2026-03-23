# Elixir

When the codebase you're working in has established conventions that differ from what's described here, follow the codebase.

## Style

Prefer pattern matching in function heads over conditional logic in function bodies. Multiple function clauses with distinct patterns are clearer than a single clause with nested `case` or `cond`. Use guard clauses (`when`) to refine matches on type or range.

Prefer the pipe operator `|>` for data transformation chains. Each step should take the accumulator as its first argument and return the next value. Break the chain when a step needs the intermediate value in a non-first position; bind it to a variable instead of contorting the pipeline.

Prefer `with` for sequences of pattern matches that can fail at any step. `with` reads top to bottom and the `else` clause handles all failure shapes in one place. For simple two-match cases, nested `case` is fine.

Prefer GenServer for stateful processes that serialize access, Agent for trivial state wrappers, and Task for fire-and-forget or awaitable async work. Reach for GenServer when you need `handle_info` or custom message handling; reach for Agent when the state logic is a single function.

Prefer protocols for polymorphism across types you don't control. Use behaviours (`@callback`) for contracts within your own codebase where modules must implement a known interface.

Prefer `defstruct` with `@enforce_keys` for domain types. Structs catch typos at compile time and make pattern matching on map shape explicit. Tag structs with `@type t()` for the type system.

Prefer `snake_case` for functions and variables, `PascalCase` for modules. Prefix unused variables with `_`. Use `@moduledoc` and `@doc` with markdown for public API documentation.

## Type System

Elixir 1.17+ includes a gradual set-theoretic type system integrated into the compiler. Types compose with unions (`atom() or integer()`), intersections, and negations. The compiler infers types from patterns and return values, emitting warnings for type mismatches. As of Elixir 1.19, the compiler also type-checks protocol dispatch and function captures. Full type inference is expected in Elixir 1.20. Prefer `@spec` annotations on public functions. Dialyzer (via `dialyxir`) remains useful for checking contracts across module boundaries until the built-in type system reaches full coverage.

## Build and Test

Prefer `mix compile --warnings-as-errors` to catch unused variables, imports, and deprecated calls. Prefer `mix test` for the test suite and `mix test --stale` during development to run only tests affected by recent changes.

Prefer `mix format` before committing. Elixir ships a deterministic formatter configured by `.formatter.exs`; no debates about style. Prefer `mix credo --strict` for static analysis covering consistency, readability, and complexity. The compiler's built-in type checking (Elixir 1.17+) catches many issues that previously required Dialyzer. Prefer Dialyzer (via `mix dialyzer` through the `dialyxir` dependency) for cross-module contract checking and specs verification; start from `@spec` annotations on public functions and expand.

Prefer `mix docs` (via `ex_doc`) for generating documentation. Doctests embedded in `@doc` attributes serve double duty as documentation examples and executable tests.

## Testing

Prefer ExUnit with `describe`/`test` blocks. Group related tests under `describe "function_name/arity"` and write test names that read as behavior statements.

```elixir
describe "parse/1" do
  test "returns structured data for valid input" do
    assert {:ok, %Config{timeout: 30}} = Parser.parse("timeout=30")
  end

  test "returns error tuple for malformed input" do
    assert {:error, :invalid_format} = Parser.parse("garbage")
  end
end
```

Prefer `setup` and `setup_all` callbacks for shared fixtures. `setup` runs before each test; `setup_all` runs once per describe block. Return a map from setup to inject values into the test context.

Prefer `async: true` on test modules that have no shared state (no database writes, no named process dependencies). Async tests run concurrently across cores and dramatically cut suite time.

Prefer Mox for mocking. Define behaviours for external dependencies, then `Mox.defmock(MockService, for: ServiceBehaviour)` and set expectations per test. Mox enforces that mocks respect the behaviour contract and that all expectations are fulfilled.

Prefer doctests for pure functions with simple inputs and outputs. Add `doctest MyModule` in the test file to execute all `iex>` examples in that module's documentation.

## Common Pitfalls

Process mailbox overflow happens when a process receives messages faster than it handles them. Memory grows unbounded until the node crashes. Monitor mailbox length in production with `:erlang.process_info(pid, :message_queue_len)` and design back-pressure mechanisms for high-throughput paths.

GenServer bottlenecks emerge when all callers serialize through a single process. If the GenServer's work is read-heavy and state is immutable between writes, consider ETS tables for concurrent reads. For CPU-bound work, distribute across a pool (e.g., `poolboy`, `NimblePool`).

Dynamic atom creation from user input (`String.to_atom/1`) risks exhausting the atom table, which is fixed-size and never garbage collected. Prefer `String.to_existing_atom/1` when the atom must already exist, or keep user-controlled values as strings.

Large binaries (over 64 bytes) live on the shared binary heap, referenced by pointers on the process heap. A process that holds a reference to a sub-binary of a large binary prevents the entire original from being garbage collected. Copy the needed slice with `:binary.copy/1` if the parent binary is large and the reference long-lived.

Supervision tree design determines fault tolerance. Place processes that can crash independently under separate supervisors with appropriate restart strategies (`:one_for_one`, `:rest_for_one`, `:one_for_all`). Avoid putting unrelated processes under the same supervisor, since `:one_for_all` restarts everything when one child fails.

Bare `try/rescue` in Elixir is a code smell. The convention is `{:ok, value}` / `{:error, reason}` tuples for expected failures, with exceptions reserved for genuinely unexpected conditions. Prefer matching on return tuples over rescuing; `with` handles multi-step error propagation cleanly.
