# Rust

When the codebase you're working in has established conventions that differ from what's described here, follow the codebase.

## Edition

Rust 2024 is the current edition, stable since Rust 1.85 (February 2025). It changes RPIT lifetime capture rules (all in-scope generics are captured by default), requires `unsafe` on extern blocks, reserves the `gen` keyword, adds `Future` and `IntoFuture` to the prelude, and supports async closures (`async || {}`). Prefer `edition = "2024"` in `Cargo.toml` for new projects. Use `cargo fix --edition` to migrate existing crates.

## Style

Prefer borrowing over ownership transfer when the function only needs to read data. Accept `&str` rather than `String`, `&[T]` rather than `Vec<T>`, and `&Path` rather than `PathBuf` at API boundaries. Return owned types (`String`, `Vec<T>`, `PathBuf`) when the caller needs to store the result.

Prefer the `?` operator for propagating errors. Each call site gets a clean one-liner instead of a match block. Reserve explicit `match` for cases where you need to handle specific variants differently.

Prefer `Result<T, E>` for operations that can fail and `Option<T>` for values that may be absent. Avoid `.unwrap()` and `.expect()` in library code; they panic on failure. In binaries, `.expect("descriptive message")` is acceptable at the top level or during initialization where recovery is impossible.

Prefer enums for modeling closed sets of states. Rust enums with data are more expressive and safer than stringly-typed alternatives or boolean flags.

Prefer `impl Trait` in function signatures when the concrete type is an implementation detail. Use explicit generics with trait bounds when the caller needs to name or constrain the type.

Prefer `Clone` only when copying is semantically meaningful. Cloning to appease the borrow checker usually signals a design issue; restructure ownership instead.

## Error Handling

Prefer `thiserror` for library crates and `anyhow` for application binaries. Libraries need clear, matchable error types that callers can inspect; applications need ergonomic error aggregation for display and logging.

Define error enums with `#[derive(thiserror::Error)]`. Use `#[from]` to convert from underlying library errors and `#[source]` to preserve the error chain. Keep variant counts manageable; if callers always handle 20 variants the same way, group them or use `#[error(transparent)]`.

Prefer lowercase error messages without trailing punctuation. Each error describes itself without recursively formatting its source.

## Build and Test

Prefer `cargo build` to verify compilation. Prefer `cargo test` to run the full test suite, including doctests and integration tests. Prefer `cargo clippy` as the primary linter; treat its warnings seriously, as they catch real bugs and non-idiomatic patterns. Prefer `cargo fmt` (backed by `rustfmt`) for formatting; check for a `rustfmt.toml` in the project root for project-specific style overrides.

When the project uses a workspace (`[workspace]` in the root `Cargo.toml`), run cargo commands from the workspace root so all members are covered.

Prefer `cargo test -- --nocapture` when you need to see `println!` output during debugging. Remove debug prints before committing.

## Testing

Prefer unit tests in the same file as the code they test, inside a `#[cfg(test)] mod tests` block. This keeps tests close to the implementation and lets them access private items.

Prefer integration tests in the `tests/` directory at the crate root for testing public API behavior. Each file in `tests/` compiles as a separate crate and can only access the public interface.

Prefer `assert_eq!` and `assert_ne!` over plain `assert!` for comparisons, because they print both values on failure.

Prefer the `#[should_panic(expected = "message fragment")]` attribute for testing that code panics with the right message. For testing `Result`-returning functions, prefer asserting on the `Err` variant directly.

## Ownership and Lifetimes

Prefer structuring data so that ownership flows in one direction. Parent owns children; children borrow from parent. Cyclic ownership requires `Rc<RefCell<T>>` or `Arc<Mutex<T>>`, both of which add runtime cost and complexity.

Prefer elided lifetimes when the compiler can infer them. Add explicit lifetime annotations only when required by the compiler or when they clarify the relationship between inputs and outputs. In the 2024 edition, return-position `impl Trait` captures all in-scope lifetimes by default; use `+ use<'x, T>` syntax when you need to restrict which generics the hidden type captures.

Prefer splitting large structs that trigger borrow checker conflicts. If a method needs mutable access to one field while borrowing another, splitting into separate structs (or using helper methods that borrow individual fields) resolves the conflict without unsafe code.

## Async

Prefer the async runtime the project already uses (tokio, async-std, smol). Do not introduce a second runtime. When starting a new project, prefer tokio; it has the largest ecosystem and the most library support.

`async fn` in traits is stable since Rust 1.75. Return-position `impl Trait` in traits is also stable. Use `Pin<Box<dyn Future>>` at trait boundaries only when dynamic dispatch or object safety requires it. The `async_trait` proc macro is no longer needed for most use cases.

Async closures (`async || {}`) are stable in the 2024 edition. They capture values from the enclosing scope and return futures when called. The `AsyncFn`, `AsyncFnMut`, and `AsyncFnOnce` traits enable generic higher-order async functions.

Prefer `tokio::spawn` (or equivalent) with structured error handling over detached tasks. Every spawned task should have a join handle that something monitors.

## Common Pitfalls

Implicit `Deref` coercions (`String` to `&str`, `Vec<T>` to `&[T]`) make function calls convenient but can obscure what's happening. Be aware of them when reading code, and prefer explicit borrows in contexts where clarity matters.

Turbofish syntax (`::<Type>`) is needed when the compiler cannot infer a generic type from context. `"42".parse::<i32>()` is clearer than relying on type inference from a distant binding.

`to_string()`, `to_owned()`, and `clone()` on string types all produce a `String`, but they signal different intent. Prefer `to_owned()` when converting `&str` to `String`, `to_string()` when invoking a `Display` implementation, and `clone()` when duplicating an existing `String`.

Moving a value into a closure or thread transfers ownership permanently. If you need the value afterward, clone before the move or use `Arc` for shared ownership across threads.
