# Haskell

When the codebase you're working in has established conventions that differ from what's described here, follow the codebase.

## Style

Prefer explicit type signatures on all top-level definitions. GHC can infer them, but signatures serve as documentation, catch errors earlier, and prevent accidental polymorphism that bloats specialization.

Prefer algebraic data types over stringly-typed or boolean-heavy designs. A `data Direction = North | South | East | West` is self-documenting and exhaustively checkable; a `String` is neither.

Prefer pattern matching over manual deconstruction. Match on constructors directly in function arguments and `case` expressions. Enable `-Wincomplete-patterns` to catch unhandled cases at compile time.

Prefer pure functions wherever possible. Push IO and side effects to the edges of the program. A function with a pure signature is easier to test, reason about, and compose.

Prefer `do` notation for monadic composition when the pipeline has more than two or three binds. For short chains, `>>=` and `<$>`/`<*>` applicative style can be clearer. Match the prevailing style in the file.

Prefer type classes for ad-hoc polymorphism, but define new classes sparingly. Before creating a type class, check whether a plain function, a record of functions, or an existing class (`Foldable`, `Traversable`, `MonadIO`) already covers the need.

Prefer qualified imports or explicit import lists over unqualified blanket imports. `import qualified Data.Map.Strict as Map` avoids name collisions and makes call sites self-documenting.

Prefer `Text` (from `text`) for human-readable strings and `ByteString` (from `bytestring`) for binary data. The built-in `String` type (`[Char]`) is a linked list per character; reserve it for prototyping or when interfacing with APIs that require it.

Prefer `camelCase` for functions and values, `PascalCase` for types, constructors, and type classes. Module names follow the hierarchical `Data.Map.Strict` convention.

## Build and Test

Prefer Cabal as the build tool for new projects. Cabal has gained significant momentum and development velocity, while Stack has slowed. Stack's HLS integration has historically been frustrating. The Haskell Foundation now administers Stackage, so both ecosystems remain viable, but Cabal is the safer default for new work. When the project already uses Stack, continue with Stack; don't mix the two.

Prefer `ghc -Wall -Werror` (or the equivalent in `ghc-options` in the `.cabal` file) to surface unused binds, incomplete patterns, missing signatures, and other warnings. Treat warnings as errors in CI. GHC 9.12 introduced `OrPatterns` (combining multiple pattern clauses), `MultilineStrings`, and type syntax in expressions. GHC 9.14 improves specialization with type application syntax in the `SPECIALISE` pragma.

Prefer Ormolu or Fourmolu for deterministic formatting. Both produce consistent output without configuration debates. Run the formatter before committing.

Prefer HLint for style suggestions: redundant brackets, eta reduction opportunities, and library function alternatives. Not every suggestion is worth taking, but review each one.

Prefer Weeder to detect dead code: unused exports, unreachable definitions, and unnecessary dependencies. Run it periodically to keep the dependency footprint honest.

## Testing

Prefer Hspec or Tasty as the test framework. Hspec provides `describe`/`it` BDD-style structure. Tasty unifies HUnit, QuickCheck, and other providers under a single tree. Use whichever the project already has; both produce clear failure output.

```haskell
describe "parseConfig" $ do
  it "parses valid YAML into a Config" $
    parseConfig validYaml `shouldBe` Right expectedConfig

  it "returns Left for malformed input" $
    parseConfig "{{garbage" `shouldSatisfy` isLeft
```

Prefer QuickCheck or Hedgehog for property-based testing alongside example-based tests. Properties catch edge cases that hand-picked examples miss. Write `Arbitrary` instances for domain types, or use Hedgehog's integrated generators.

Prefer doctest for lightweight executable examples in Haddock comments. Add `-- >>> expression` / `-- result` pairs to document and test pure functions simultaneously.

Prefer separating pure logic tests (fast, no IO) from integration tests (database, file system, network). Run pure tests with every build; gate integration tests behind a flag or test suite.

## Common Pitfalls

Space leaks from lazy evaluation are the classic Haskell trap. Unevaluated thunks accumulate silently until memory is exhausted. Prefer strict data fields (`!` or `{-# UNPACK #-}`) in types that will hold many values, strict left folds (`Data.List.foldl'` over `foldl`), and `BangPatterns` in accumulators. Profile with `+RTS -hT` when memory usage surprises you.

`String` is `[Char]`, a linked list with one constructor per character. It is catastrophically slow for anything beyond toy inputs. Prefer `Data.Text` for text processing and `Data.ByteString` for binary or ASCII-heavy work. Use `OverloadedStrings` to write string literals that work with `Text` and `ByteString` without explicit conversion.

Partial functions (`head`, `tail`, `fromJust`, `read`, `!!`) crash on empty or invalid input with no recovery path. Prefer their total alternatives: pattern matching, `Data.Maybe.fromMaybe`, `readMaybe`, `Data.List.uncons`, and safe indexing. Enable `-Wincomplete-uni-patterns` to catch partial pattern matches.

Orphan instances (type class instances defined in a module that owns neither the class nor the type) create incoherence risks and confuse dependency resolution. Define instances in the module that defines the type or the class. When you cannot avoid an orphan, isolate it in a clearly named module and document why.

Over-abstracted monad transformer stacks (`ReaderT Config (StateT AppState (ExceptT AppError (LoggingT IO)))`) become difficult to reason about and compose. Prefer a concrete application monad with `newtype AppM a = AppM (ReaderT Env IO a)` and `Has`-style constraints or `MonadReader`/`MonadIO` classes for testability without the tower of transformers. For new projects that want a more principled approach, consider `effectful` (type-level effect tracking with good performance) or `bluefin` (value-level effect handles that make disambiguating multiple effects of the same type straightforward). Both outperform the older `polysemy` and `fused-effects` libraries. Prefer whichever the project already uses; do not introduce an effect system into a codebase that manages fine without one.

Strictness annotations and evaluation strategy mismatches between lazy and strict variants of containers (`Data.Map.Lazy` vs `Data.Map.Strict`) can produce subtle performance differences. Prefer the strict variants (`Data.Map.Strict`, `Data.HashMap.Strict`) unless laziness in values is specifically needed.
