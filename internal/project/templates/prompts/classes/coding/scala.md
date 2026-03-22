# Scala

When the codebase you're working in has established conventions that differ from what's described here, follow the codebase.

## Style

Prefer case classes for immutable data types. A `case class Point(x: Int, y: Int)` gives you `equals`, `hashCode`, `toString`, `copy`, and pattern matching support with no boilerplate. Reserve regular classes for types with mutable state or complex construction logic.

Prefer sealed traits and sealed abstract classes for closed type hierarchies. `sealed trait Shape` with case class subtypes enables exhaustive pattern matching, and the compiler warns when a `match` is missing a case.

Prefer pattern matching over chains of `if`/`else` or `isInstanceOf` checks. Match expressions are more readable, the compiler checks exhaustiveness on sealed types, and they destructure values naturally: `case Person(name, age) if age >= 18 =>`.

Prefer immutable collections by default. `List`, `Vector`, `Map`, and `Set` from `scala.collection.immutable` (the default import) prevent accidental mutation. Reach for mutable collections only when profiling shows a measurable performance need, and scope them tightly.

Prefer for-comprehensions for composing monadic operations (chaining `Option`, `Either`, `Future`, or collection transforms). A for-comprehension with two or three generators reads more clearly than nested `flatMap`/`map` chains. For a single `map` or `filter`, use the method directly.

Prefer type classes (via implicit parameters or context bounds in Scala 2, `using`/`given` in Scala 3) over inheritance when you need ad-hoc polymorphism. Type classes let you add behavior to types you don't own without subclassing or wrapper types. Keep implicit definitions in companion objects or clearly scoped implicit objects to avoid resolution surprises.

Prefer `val` over `var`. Immutable bindings reduce reasoning overhead and eliminate mutation bugs. Use `var` only when accumulation or in-place update is genuinely necessary and scoped tightly.

Prefer expressing errors with `Either[Error, A]` or domain-specific ADTs over throwing exceptions in pure code. Reserve exceptions for truly exceptional, unrecoverable conditions or interop with Java APIs that expect them.

## Build and Test

Prefer the project's existing build tool, typically sbt (`build.sbt`). Look for `project/build.properties` and `project/plugins.sbt` to understand the sbt version and plugins in use. Prefer `sbt compile` for a quick compilation check and `sbt test` to run the full suite.

Prefer scala-cli for single-file scripts, quick experiments, and small utilities outside the main build. It handles dependency resolution and compilation without project scaffolding.

Prefer scalafmt for formatting. Check for `.scalafmt.conf` at the project root. Run `sbt scalafmtAll` or `scalafmt` before committing. Prefer scalafix for automated linting and refactoring; it handles import organization, unused code removal, and migration rewrites. Prefer Wartremover when the project includes it as a compiler plugin for catching unsafe patterns (`Any`, `Null`, `Return`, `Var`).

## Testing

Prefer whichever testing framework the project already uses. ScalaTest (FlatSpec, FunSuite, WordSpec styles), MUnit, and specs2 are all common. For new projects without an established convention, MUnit is lightweight and integrates well with sbt.

Prefer property-based testing with ScalaCheck for functions with well-defined input domains. Generator-driven tests catch edge cases that hand-picked examples miss. ScalaCheck integrates with ScalaTest (`GeneratorDrivenPropertyChecks`) and MUnit (`ScalaCheckSuite`).

Prefer test fixtures via traits mixed into test suites rather than complex setup/teardown methods. A `trait DatabaseFixture` mixed into the suite keeps resource management close to where it's used.

Prefer clear test names that describe the scenario and expected outcome. In ScalaTest FlatSpec: `it should "return None for missing keys"`. In MUnit: `test("lookup returns None for missing keys")`.

## Common Pitfalls

Implicit resolution complexity grows with the number of implicit definitions in scope. When the compiler reports ambiguous implicits or fails to find an expected implicit, check companion objects first (they have highest priority after local scope), then imported implicits, then package objects. In Scala 3, prefer `given`/`using` over `implicit` for clearer scoping and error messages.

Variance annotations (`+A`, `-A`) on type parameters control subtyping relationships but interact subtly with method signatures. A covariant container (`List[+A]`) cannot appear in contravariant position (as a method parameter type). When the compiler rejects a variance annotation, the fix is often a lower bound (`def add[B >: A](item: B)`), not removing the annotation.

Blocking calls inside a `Future` context starve the execution context's thread pool. Prefer wrapping blocking I/O with `scala.concurrent.blocking` or running it on a dedicated `ExecutionContext` sized for blocking work. Better yet, prefer a non-blocking alternative (async HTTP client, non-blocking database driver) when one exists.

Overusing inheritance over composition leads to rigid hierarchies. Prefer composing behavior through traits, type classes, or function parameters over deep class hierarchies. A trait mixed in is easier to replace than a base class baked into the type's identity.

Forgetting that `Future` is eagerly evaluated: `val f = Future { compute() }` starts executing immediately at the point of definition. For deferred computation, use `lazy val`, wrap in a function, or prefer a lazy effect type (`IO` from Cats Effect, `ZIO`) when the project uses one.
