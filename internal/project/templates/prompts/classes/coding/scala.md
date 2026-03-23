# Scala

When the codebase you're working in has established conventions that differ from what's described here, follow the codebase.

## Style

Prefer case classes for immutable data types. A `case class Point(x: Int, y: Int)` gives you `equals`, `hashCode`, `toString`, `copy`, and pattern matching support with no boilerplate. Reserve regular classes for types with mutable state or complex construction logic.

Prefer `enum` in Scala 3 for simple sum types and enumerations. `enum Color { case Red, Green, Blue }` and `enum Shape { case Circle(r: Double); case Rectangle(w: Double, h: Double) }` replace the Scala 2 pattern of sealed trait with case objects/classes. Fall back to sealed traits when enums hit their limitations (enums cannot nest other enums, and some advanced pattern matching scenarios work better with sealed traits).

Prefer `given`/`using` over `implicit` in Scala 3 code. `given` instances are easier to trace, produce clearer error messages, and have well-defined resolution rules. Reserve `implicit` for Scala 2 compatibility or when working in a mixed codebase. Keep `given` definitions in companion objects or clearly scoped objects to avoid resolution surprises.

Prefer pattern matching over chains of `if`/`else` or `isInstanceOf` checks. Match expressions are more readable, the compiler checks exhaustiveness on sealed types and enums, and they destructure values naturally: `case Person(name, age) if age >= 18 =>`.

Prefer immutable collections by default. `List`, `Vector`, `Map`, and `Set` from `scala.collection.immutable` (the default import) prevent accidental mutation. Reach for mutable collections only when profiling shows a measurable performance need, and scope them tightly.

Prefer for-comprehensions for composing monadic operations (chaining `Option`, `Either`, `Future`, or effect types). A for-comprehension with two or three generators reads more clearly than nested `flatMap`/`map` chains. For a single `map` or `filter`, use the method directly.

Prefer `val` over `var`. Immutable bindings reduce reasoning overhead and eliminate mutation bugs. Use `var` only when accumulation or in-place update is genuinely necessary and scoped tightly.

Prefer expressing errors with `Either[Error, A]` or domain-specific ADTs over throwing exceptions in pure code. Reserve exceptions for truly exceptional, unrecoverable conditions or interop with Java APIs that expect them.

Scala 3 supports optional braces (significant indentation). Either style is acceptable; follow the project's established convention. When using braceless syntax, use `end` markers on large blocks (classes, methods, matches spanning many lines) to make scope boundaries visible.

## Build and Test

Prefer the project's existing build tool, typically sbt (`build.sbt`). Look for `project/build.properties` and `project/plugins.sbt` to understand the sbt version and plugins in use. Prefer `sbt compile` for a quick compilation check and `sbt test` to run the full suite. Mill is a growing alternative that offers faster builds and a more direct configuration model; check for `build.mill` or `build.sc` at the project root.

Prefer scala-cli for single-file scripts, quick experiments, and small utilities outside the main build. It handles dependency resolution and compilation without project scaffolding. scala-cli is the official `scala` command as of SIP-46.

Prefer scalafmt for formatting. Check for `.scalafmt.conf` at the project root. Run `sbt scalafmtAll` or `scalafmt` before committing. Prefer scalafix for automated linting and refactoring; it handles import organization, unused code removal, and migration rewrites (including Scala 2 to 3 syntax changes). Prefer Wartremover when the project includes it as a compiler plugin for catching unsafe patterns (`Any`, `Null`, `Return`, `Var`).

## Scala Version

The current Scala 3 LTS line is 3.3.x (latest 3.3.7). The current Scala Next line is 3.8.x. For new projects, prefer the LTS line unless specific Scala Next features are required. The next LTS (3.9.x) will require JDK 17+; the 3.3.x LTS retains JDK 8 bytecode compatibility.

## Testing

Prefer whichever testing framework the project already uses. ScalaTest (FlatSpec, FunSuite, WordSpec styles), MUnit, and specs2 are all common. For new projects without an established convention, MUnit is lightweight and integrates well with sbt.

Prefer property-based testing with ScalaCheck for functions with well-defined input domains. Generator-driven tests catch edge cases that hand-picked examples miss. ScalaCheck integrates with ScalaTest (`GeneratorDrivenPropertyChecks`) and MUnit (`ScalaCheckSuite`).

Prefer test fixtures via traits mixed into test suites rather than complex setup/teardown methods. A `trait DatabaseFixture` mixed into the suite keeps resource management close to where it's used.

Prefer clear test names that describe the scenario and expected outcome. In ScalaTest FlatSpec: `it should "return None for missing keys"`. In MUnit: `test("lookup returns None for missing keys")`.

## Effect Systems

When the project uses an effect system, follow its conventions. ZIO (current line 2.1.x) provides a batteries-included ecosystem with `ZIO[R, E, A]` as the central type, built-in dependency injection via ZLayer, and integrated testing. Cats Effect (current line 3.7.x) takes a more modular approach with the Typelevel stack (fs2, http4s, doobie). For new projects, the choice depends on team preference: ZIO for a cohesive, opinionated ecosystem; Typelevel for composable, type-class-driven abstractions. Prefer a lazy effect type (`IO`, `ZIO`) over raw `Future` for new pure-functional code; `Future` is eagerly evaluated and harder to compose safely.

## Common Pitfalls

`given`/`using` resolution complexity grows with the number of given instances in scope. When the compiler reports ambiguous givens or fails to find an expected instance, check companion objects first (they have highest priority after local scope), then imported givens, then package objects. Prefer explicit imports of given instances (`import Foo.given`) over wildcard imports to keep resolution predictable.

Variance annotations (`+A`, `-A`) on type parameters control subtyping relationships but interact subtly with method signatures. A covariant container (`List[+A]`) cannot appear in contravariant position (as a method parameter type). When the compiler rejects a variance annotation, the fix is often a lower bound (`def add[B >: A](item: B)`), not removing the annotation.

Blocking calls inside a `Future` context starve the execution context's thread pool. Prefer wrapping blocking I/O with `scala.concurrent.blocking` or running it on a dedicated `ExecutionContext` sized for blocking work. Better yet, prefer a non-blocking alternative (async HTTP client, non-blocking database driver) when one exists.

Overusing inheritance over composition leads to rigid hierarchies. Prefer composing behavior through traits, type classes, or function parameters over deep class hierarchies. A trait mixed in is easier to replace than a base class baked into the type's identity.

Forgetting that `Future` is eagerly evaluated: `val f = Future { compute() }` starts executing immediately at the point of definition. For deferred computation, use `lazy val`, wrap in a function, or prefer a lazy effect type (`IO` from Cats Effect, `ZIO`) when the project uses one.
