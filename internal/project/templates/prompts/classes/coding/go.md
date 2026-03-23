# Go

When the codebase you're working in has established conventions that differ from what's described here, follow the codebase.

## Version

Go 1.26 is the current release (February 2026). It enables the Green Tea garbage collector by default (10-40% GC overhead reduction), adds `errors.AsType[E]` for generic type-safe error checking, revamps `go fix` as the home of Go's modernizers, and allows `new(expr)` for pointer initialization. Go 1.24 introduced tool directives in `go.mod`, the `omitzero` JSON struct tag, Swiss Tables for maps, `testing.B.Loop()`, and iterator methods across the standard library. Set the `go` directive in `go.mod` to match the minimum Go version your project supports.

## Style

Prefer accepting interfaces and returning concrete types. A function that takes an `io.Reader` is more useful than one that takes `*os.File`, but a function that returns an interface hides information the caller might need.

Prefer short variable names in tight scopes. `r` for a reader used on three lines is clearer than `requestBodyReader`. In larger scopes or exported identifiers, be descriptive.

Prefer `fmt.Errorf("doing X: %w", err)` for error wrapping. Each layer of the call stack adds context about what it was doing when the error occurred. Avoid wrapping with `%v` unless you intentionally want to break the error chain.

Handle errors immediately after the call that produces them. The `if err != nil { return err }` pattern keeps the happy path at the left margin and error handling indented. Do not accumulate errors to check later.

Prefer `errors.Is` and `errors.As` over direct comparison or type assertions for error checking. Wrapped errors break equality checks but work with the `errors` package unwrapping functions. In Go 1.26+, prefer `errors.AsType[E](err)` over `errors.As`; it is generic, type-safe, avoids reflect calls, and cannot panic at runtime.

Prefer returning `error` over panicking. Reserve `panic` for genuinely unrecoverable situations (programmer bugs, impossible states after validation). Library code should never panic on bad input.

Prefer named return values only when they clarify the signature. Do not use named returns solely to enable bare `return` statements.

Prefer defining types close to where they're used. A type needed by one package belongs in that package, not in a shared `types` or `models` package.

Prefer meaningful zero values. Design structs so that the zero value is usable or at least safe.

## Build and Test

Prefer `go build ./...` to verify compilation across all packages. Prefer `go test ./...` with the `-race` flag when practical. Prefer `go vet ./...` as a baseline static check.

Prefer `gofmt` or `goimports` for formatting. All Go code should be formatted before committing; unformatted Go is a red flag.

Prefer `golangci-lint` (v2.x) when the project has a `.golangci.yml` configuration. It runs linters in parallel, caches results, and bundles staticcheck, govet, and dozens of other analyzers. When no linter configuration exists, `go vet` is sufficient.

Prefer `go fix ./...` to modernize code to current idioms and APIs. Go 1.26 revamped this command as a push-button modernizer built on the analysis framework.

Prefer tracking tool dependencies with `tool` directives in `go.mod` (Go 1.24+). This replaces the `tools.go` blank-import workaround. Run `go install tool` to install all tracked tools.

## Structured Logging

Prefer `log/slog` for structured logging. It is part of the standard library since Go 1.21, avoids external dependencies, and outputs JSON or text. Pass `slog.Logger` as an explicit dependency or retrieve it from context rather than using the global default. In Go 1.26, `slog.NewMultiHandler` dispatches log records to multiple handlers.

## Testing

Prefer table-driven tests for functions with multiple input/output scenarios. Name each case clearly so that `go test -v` output reads as documentation.

```go
tests := []struct {
    name    string
    input   string
    want    int
    wantErr bool
}{
    {name: "empty input", input: "", want: 0},
    {name: "single item", input: "a", want: 1},
}
for _, tt := range tests {
    t.Run(tt.name, func(t *testing.T) {
        // ...
    })
}
```

Prefer subtests (`t.Run`) for grouping related assertions. They give you granular failure output and let you run individual cases with `-run`.

Prefer `testdata/` directories for fixture files. Go tooling ignores `testdata/` during builds, and the convention is well understood.

Prefer the standard `testing` package for most tests. Testify's `require` and `assert` packages are fine when the project already uses them, but do not introduce testify into a project that uses stdlib assertions.

Prefer `t.Helper()` in test helper functions so that failure messages report the caller's line number. Prefer `t.Cleanup()` over `defer` in tests when cleanup must run after subtests complete. Use `t.Context()` (Go 1.24+) to get a context that is canceled when the test finishes. Use `testing.B.Loop()` (Go 1.24+) for benchmarks instead of the `for i := 0; i < b.N; i++` pattern.

## Iterators

Range-over-function iterators are stable since Go 1.23. Prefer them for custom container types, sequence generation, and lazy evaluation. The standard library now provides iterator-returning methods in `strings` (`Lines`, `SplitSeq`, `FieldsSeq`), `bytes`, `slices`, `maps`, and `go/types`. Prefer these over index-based loops when they express intent more clearly.

## Concurrency

Prefer `context.Context` as the first parameter for functions that do I/O or may block. Propagate cancellation faithfully; do not ignore context deadlines.

Prefer launching goroutines with clear ownership. Every goroutine should have a defined shutdown path. If a goroutine can outlive its caller, that's a leak. Use `sync.WaitGroup`, channels, or `errgroup.Group` to track completion.

Prefer `sync.Mutex` for protecting shared state and channels for signaling between goroutines. Do not use channels as locks or mutexes as signals.

Prefer `defer mu.Unlock()` immediately after `mu.Lock()` to prevent forgetting to unlock. Be aware that deferring inside a loop body defers until the function returns, not until the next iteration; extract the loop body into a function if you need per-iteration locking.

## Common Pitfalls

A nil pointer stored in an interface variable makes the interface non-nil. `var err *MyError = nil; var e error = err; e != nil` is true. Prefer returning explicit `nil` for interface types rather than typed nil pointers.

A `defer f.Close()` inside a loop defers every close until the function returns, potentially exhausting file descriptors. Extract the loop body into a separate function, or close explicitly within each iteration.

Variable shadowing with `:=` in nested scopes can silently create a new variable instead of assigning to the outer one. When `err` is declared in an outer scope, `result, err := someFunc()` in an `if` block creates a new `err` that shadows the outer. Use `=` or restructure the code to avoid this.

Goroutine closures that capture loop variables see the variable's final value, not the value at the time the goroutine was created. Since Go 1.22, each loop iteration gets its own copy of the variable, fixing this for `for` loops with a `go.mod` targeting 1.22 or later. In older module versions, pass the variable as a parameter to the goroutine function.
