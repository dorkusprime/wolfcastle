# Kotlin

When the codebase you're working in has established conventions that differ from what's described here, follow the codebase.

## Style

Prefer data classes for types whose purpose is carrying data. A `data class Point(val x: Int, val y: Int)` gives you `equals`, `hashCode`, `toString`, `copy`, and destructuring for free. Reserve regular classes for types with significant behavior or mutable internal state.

Prefer sealed classes and sealed interfaces for closed type hierarchies. `sealed interface Shape` with subclasses `Circle`, `Rectangle` enables exhaustive `when` expressions, and the compiler enforces completeness.

Prefer guard conditions in `when` branches (stable in Kotlin 2.2) to combine pattern matching with additional boolean checks. `is Circle if radius > 0 ->` is clearer than nesting an `if` inside the branch body.

Prefer Kotlin's null safety system over null checks. Use nullable types (`String?`) to express optionality, the safe call operator (`?.`) for chaining, and the Elvis operator (`?:`) for defaults. Prefer `requireNotNull()` or `checkNotNull()` at API boundaries to fail fast with a clear message. Avoid `!!` except in tests or where a preceding check makes the non-null guarantee obvious.

Prefer extension functions for operations that read naturally as methods but don't need access to private state. An extension on a type you don't own keeps calling code clean without subclassing or utility classes. Keep extensions close to where they're used; avoid scattering unrelated extensions across the codebase.

Prefer scope functions with discipline. `let` for null-conditional chains (`value?.let { process(it) }`), `apply` for configuring an object after construction, `run` for scoped transformations, `also` for side effects like logging. When nesting scope functions or chaining more than two, extract a named function instead.

Prefer expression bodies for single-expression functions. `fun square(n: Int): Int = n * n` is clearer than a block body with an explicit `return`. For multi-statement functions, use a block body.

Prefer `val` over `var`. Immutable bindings reduce cognitive load and eliminate a class of mutation bugs. Use `var` when accumulation or reassignment is genuinely necessary.

Prefer object declarations for singletons and companion objects for factory methods. Avoid using companion objects as a dumping ground for unrelated static-style functions; top-level functions are often a better fit.

Prefer context parameters (preview in Kotlin 2.2) over manual parameter threading when a dependency is shared across many functions in a call chain. Context parameters replace the older experimental context receivers feature. Unlike context receivers, context parameters require explicit naming to access members (`context(logger: Logger)` uses `logger.info()`, not bare `info()`).

## Build and Test

Prefer the project's existing build tool, typically Gradle with Kotlin DSL (`build.gradle.kts`). Use the wrapper script (`gradlew`) when present. Prefer `gradlew build` to compile, test, and run checks in one pass. Prefer `gradlew compileKotlin` for a quick compilation check.

Prefer ktlint for formatting and style enforcement. Check for a `.editorconfig` or ktlint configuration in the build file. Prefer detekt for static analysis; it catches complexity, naming violations, and common code smells. Run both before committing.

The K2 compiler is the default since Kotlin 2.0 and provides roughly 2x faster compilation. Ensure the project targets Kotlin 2.x in its build configuration. The K2 compiler is also the default for IDE analysis in IntelliJ IDEA 2025.1+.

## Testing

Prefer JUnit 5 for test structure when the project uses it. `@Test`, `@ParameterizedTest`, `@Nested`, and `@DisplayName` work the same as in Java. When the project uses kotest, follow its style (string spec, fun spec, or whichever spec style the project has adopted).

Prefer MockK over Mockito for mocking in Kotlin code. MockK handles Kotlin's final-by-default classes, coroutines, and extension functions without workarounds. Mock external boundaries (HTTP clients, databases, third-party services); use real instances for types you own.

Prefer `@ParameterizedTest` with `@MethodSource` or `@CsvSource` for table-driven testing. In kotest, prefer `forAll` with a data-driven approach. Name scenarios clearly so test output reads as documentation.

Prefer kotlinx-coroutines-test (`runTest`, `TestDispatcher`) for testing coroutine code. Inject dispatchers rather than hardcoding `Dispatchers.IO` or `Dispatchers.Default`, so tests can control timing and execution.

## Kotlin Multiplatform

Kotlin Multiplatform (KMP) is stable for sharing business logic across JVM, iOS, Android, JS, and Wasm targets. Compose Multiplatform for iOS reached stable status in 2025. When structuring a KMP project, put platform-agnostic code in `commonMain` and use `expect`/`actual` declarations for platform-specific implementations. Prefer Kotlin-first libraries that support all targets (kotlinx.serialization, kotlinx.coroutines, Ktor) over platform-specific alternatives in shared modules.

## Common Pitfalls

Platform types from Java interop (`String!`) bypass Kotlin's null safety. When calling Java APIs that lack nullability annotations, treat return values as nullable unless the library is annotated with `@Nullable`/`@NotNull` (JSR 305, JetBrains annotations, JSpecify, or Jakarta). A bare `getString()` returning platform type `String!` can NPE in Kotlin if the Java side returns null.

Coroutine cancellation requires cooperation. Long-running computations that don't check `isActive` or call suspending functions won't respond to cancellation. Prefer structured concurrency with `coroutineScope` or `supervisorScope` to ensure child coroutines are properly scoped. Avoid `GlobalScope` except in top-level application bootstrapping.

`lateinit var` properties throw `UninitializedPropertyAccessException` if accessed before assignment. Prefer constructor parameters, `by lazy`, or nullable types with an explicit null check over `lateinit` when initialization timing is uncertain. Reserve `lateinit` for dependency injection frameworks and test setup where initialization is guaranteed.

Overusing scope functions makes code harder to follow. Nested `let`/`run`/`apply` chains, especially with `it` shadowing across levels, read like a puzzle. If a scope function doesn't make the code measurably more readable than a local variable or an explicit call, skip it.

Kotlin classes are final by default. If the code is designed for extension, mark classes and methods `open` explicitly. When using frameworks that generate proxies (Spring, Hibernate), prefer the `kotlin-allopen` or `kotlin-spring` compiler plugin over scattering `open` annotations by hand.
