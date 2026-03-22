# C#

When the codebase you're working in has established conventions that differ from what's described here, follow the codebase.

## Style

Prefer nullable reference types enabled (`<Nullable>enable</Nullable>` in the project file). Annotate nullability explicitly: `string` means non-null, `string?` means nullable. Use the null-forgiving operator (`!`) sparingly and only when the compiler can't see a guarantee you've already established.

Prefer records (`record` or `record struct`) for immutable data types. A `record Point(int X, int Y);` gives you value equality, `ToString`, deconstruction, and `with`-expressions for nondestructive mutation. Use classes when the type has significant mutable state or complex behavior.

Prefer pattern matching for control flow involving type checks, value decomposition, and complex conditions. `switch` expressions with property patterns, relational patterns, and `when` guards replace chains of `if`/`else if` with something the compiler can verify for exhaustiveness. Prefer `is` patterns for simple type checks over explicit casts.

Prefer LINQ for collection transformations when the pipeline reads clearly. A `.Where().Select().ToList()` chain that fits one screen beats a manual loop with an accumulator. When the pipeline needs index access, side effects, or spans multiple screens, a `foreach` loop is clearer. Prefer method syntax over query syntax unless the query involves multiple `from` clauses or `join`.

Prefer `async`/`await` for all I/O-bound operations. Suffix async methods with `Async` by convention. Use `ConfigureAwait(false)` in library code to avoid capturing the synchronization context; omit it in application code (ASP.NET Core has no sync context, so it's a no-op there, but older frameworks still do).

Prefer file-scoped namespaces (`namespace Foo;`) over block-scoped namespaces to reduce nesting. Prefer top-level statements in console applications and tools where a `Main` method would be ceremonial.

Prefer `sealed` on classes that aren't designed for inheritance. Sealing enables devirtualization optimizations and communicates intent.

## Build and Test

Prefer the `dotnet` CLI for build operations. `dotnet build` compiles, `dotnet test` runs tests, `dotnet publish` creates deployment artifacts. Check for a solution file (`.sln`) at the repository root and build at that level to catch cross-project issues.

Prefer NuGet for package management. Packages are declared in `.csproj` files as `<PackageReference>` elements. Use `dotnet add package` to add dependencies and `dotnet restore` when package state is inconsistent.

Prefer dotnet-format (`dotnet format`) or CSharpier for formatting. Check for an `.editorconfig` at the repository root, which dotnet-format uses for style rules. Run the formatter before committing.

Prefer Roslyn analyzers for static analysis. Many ship with the SDK (`Microsoft.CodeAnalysis.NetAnalyzers`). Check the project's `<AnalysisLevel>` and `<EnforceCodeStyleInBuild>` settings. Treat analyzer warnings in code you touch as errors.

## Testing

Prefer xUnit for new test projects. `[Fact]` for single-case tests, `[Theory]` with `[InlineData]` or `[MemberData]` for parameterized tests. When the project uses NUnit or MSTest, match that convention.

Prefer FluentAssertions for readable assertions. `result.Should().Be(expected)` produces clear failure messages with context. When the project uses a different assertion library, follow its style.

Prefer NSubstitute or Moq for mocking external boundaries (HTTP clients, database connections, third-party services). Do not mock types you own; use real instances or hand-written fakes. Prefer constructor injection so dependencies are explicit and tests don't need reflection.

Prefer `IAsyncDisposable` and `await using` in test fixtures that manage async resources. Prefer `IClassFixture<T>` in xUnit for expensive shared setup (database containers, HTTP servers) rather than recreating per-test.

## Common Pitfalls

`async void` methods swallow exceptions silently and can't be awaited by the caller. Prefer `async Task` for all async methods except event handlers, where `async void` is the only option. An `async void` method that throws will crash the process with an unobserved exception.

Forgetting to dispose `HttpClient`, `DbConnection`, `Stream`, and other `IDisposable` objects leaks handles and sockets. Prefer `using` declarations (`using var stream = ...`) or `using` blocks. For `HttpClient` specifically, prefer `IHttpClientFactory` to avoid socket exhaustion from repeated instantiation.

Calling `.Result` or `.Wait()` on a `Task` in code that has a synchronization context (WinForms, WPF, legacy ASP.NET) deadlocks. The `await` continuation needs the sync context, but `.Result` is blocking it. Prefer `await` end-to-end. If synchronous code must call async code, use `Task.Run(() => AsyncMethod()).GetAwaiter().GetResult()` as a last resort, not `.Result`.

Mutable structs behave unexpectedly when stored in collections or accessed through interfaces. Modifying a struct retrieved from a `List<T>` modifies a copy, not the original. Prefer `readonly struct` for value types, and prefer records or classes when mutation is needed.

String concatenation in loops creates a new `string` per iteration. Prefer `StringBuilder` for loops, `string.Join()` for assembling delimited sequences, and string interpolation (`$"..."`) for simple expressions.
