# Java

When the codebase you're working in has established conventions that differ from what's described here, follow the codebase.

## Style

Prefer records for immutable value types. A `record Point(int x, int y) {}` replaces a class with fields, constructor, `equals`, `hashCode`, and `toString` in one declaration. Use records when a type exists to carry data, not to carry behavior.

Prefer sealed interfaces and sealed classes to express closed type hierarchies. `sealed interface Shape permits Circle, Rectangle` tells the compiler (and the reader) every possible subtype, enabling exhaustive pattern matching in `switch`.

Prefer pattern matching throughout. Use `switch` expressions with record patterns for type-safe deconstruction (`case Circle(var r) -> Math.PI * r * r`), guarded patterns for conditional logic, and `instanceof` patterns to eliminate explicit casts. Exhaustive `switch` over sealed types catches missing cases at compile time.

Prefer `Optional<T>` as a return type when absence is a normal outcome, not an error. Do not use `Optional` as a field type, method parameter, or collection element. For methods that might fail, prefer throwing an exception or returning a domain-specific result type.

Prefer the Streams API for transformations over manual loops when the pipeline reads clearly. A `stream().filter().map().collect()` chain that fits one screen is preferable to a loop with an accumulator. When the pipeline grows complex or needs index access, a loop is clearer. Use Stream Gatherers (finalized in Java 24) for custom intermediate operations that the built-in operators cannot express, such as windowing, folding with state, or short-circuiting transforms.

Prefer `List.of()`, `Map.of()`, and `Set.of()` factory methods for small unmodifiable collections. Prefer `Collections.unmodifiableList()` or `List.copyOf()` when wrapping or copying a mutable source.

Prefer `var` for local variables when the type is obvious from the right-hand side. `var users = new ArrayList<User>()` is clear; `var result = service.process(input)` hides the type and hurts readability.

Prefer virtual threads for I/O-bound concurrent work. Create them with `Thread.ofVirtual().start(runnable)` or through an `ExecutorService` from `Executors.newVirtualThreadPerTaskExecutor()`. Virtual threads are cheap to create (a few kilobytes each) and unmount from their carrier thread on blocking I/O, so thread-per-request designs scale without thread pool tuning. Reserve platform threads for CPU-bound work or when pinning to a carrier is required (synchronized blocks pin the carrier; prefer `ReentrantLock` in hot paths used by virtual threads).

Prefer package-private visibility as the default. Use `public` for API surfaces, `private` for internals, and `protected` sparingly. Minimize the number of public types per package.

Prefer statements before `super()` when the constructor needs to validate or transform arguments before passing them to the parent. This feature (finalized in Java 25) eliminates the old workaround of static helper methods called inside `super()` invocations.

## Build and Test

Prefer the project's existing build tool (Maven `mvn` or Gradle `gradle`/`gradlew`). Look for `pom.xml` or `build.gradle`/`build.gradle.kts` at the project root. Use the wrapper script (`mvnw`, `gradlew`) when present.

Prefer `mvn verify` or `gradle build` to compile, test, and run static checks in one pass. Prefer `mvn compile` or `gradle compileJava` for a quick compilation check before running the full suite.

Prefer google-java-format or Spotless for formatting. Check for a `.editorconfig`, Spotless plugin configuration in the build file, or a formatter profile in the IDE settings directory. Run the formatter before committing.

Prefer Error Prone for catching common mistakes at compile time through compiler plugins. Prefer SpotBugs for bytecode-level analysis that finds null dereference, resource leaks, and concurrency bugs. Both tools complement each other: Error Prone catches patterns during compilation, SpotBugs finds issues in the compiled bytecode that source analysis misses.

## Testing

Prefer JUnit 5 (`org.junit.jupiter`) for new tests. Use `@Test`, `@ParameterizedTest`, `@Nested` for grouping, and `@DisplayName` when the method name alone doesn't communicate intent.

Prefer AssertJ for assertions. `assertThat(result).isEqualTo(expected)` reads fluently and produces clear failure messages. When the project uses Hamcrest instead, match that convention.

Prefer `@ParameterizedTest` with `@MethodSource` or `@CsvSource` for table-driven testing. Each row should test one scenario, named clearly so test output reads as documentation.

Prefer Mockito for mocking external boundaries (HTTP clients, database connections, third-party services). Do not mock types you own; use real instances or test doubles. Prefer constructor injection so dependencies are explicit and tests don't need reflection.

Prefer `@TempDir` for filesystem tests and `@Timeout` to catch hanging tests. Both are built into JUnit 5.

## Common Pitfalls

Checked exceptions force callers to handle failure but spread `throws` clauses across the call stack. Prefer unchecked exceptions (`RuntimeException` subtypes) for programming errors and unrecoverable failures. Reserve checked exceptions for recoverable conditions the caller can meaningfully handle.

Mutable collections returned from APIs invite callers to corrupt internal state. Prefer returning unmodifiable views or defensive copies. `Collections.unmodifiableList(internal)` prevents modification without copying; `List.copyOf(internal)` creates an independent snapshot.

The `equals`/`hashCode` contract requires that equal objects produce equal hash codes. Breaking this contract corrupts `HashMap` and `HashSet` behavior silently. Prefer records (which generate both correctly) or IDE-generated implementations that include all significant fields.

Resource leaks from unclosed `InputStream`, `Connection`, `PreparedStatement`, or `ResultSet` objects accumulate until the application runs out of handles or memory. Prefer try-with-resources for every `AutoCloseable`. Nest resources in a single `try` block rather than closing them manually in `finally`.

`NullPointerException` is the most common runtime failure in Java. Prefer `Objects.requireNonNull()` at API boundaries to fail fast with a clear message. Prefer `Optional` for method returns where null would otherwise be the convention. When working with Spring Framework 7+ or libraries annotated with JSpecify, treat unannotated types as non-null and use `@Nullable` explicitly where null is expected.

String concatenation in loops creates a new `String` object per iteration. Prefer `StringBuilder` for loops, or `String.join()` / `StringJoiner` when assembling a delimited sequence. For simple expressions, `+` concatenation is fine; the compiler optimizes it.

Virtual threads pin their carrier when executing inside a `synchronized` block or calling a native method. In high-throughput code paths used by many virtual threads, prefer `ReentrantLock` over `synchronized` to avoid starving the carrier thread pool.
