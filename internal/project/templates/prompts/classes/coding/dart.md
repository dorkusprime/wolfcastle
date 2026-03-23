# Dart

When the codebase you're working in has established conventions that differ from what's described here, follow the codebase.

## Style

Prefer sound null safety throughout. All new code should be fully null-safe. Use nullable types (`String?`) only when absence is meaningful, and prefer non-nullable types as the default. Use the null-aware operators (`?.`, `??`, `??=`, `...?`) to handle nullable values concisely rather than writing explicit null checks.

Prefer `final` for local variables that are assigned once and `const` for compile-time constants. Reserve `var` for variables that genuinely need reassignment. At the class level, prefer `final` fields initialized in the constructor; use `late final` only when deferred initialization is unavoidable and you can guarantee assignment before access.

Prefer cascade notation (`..`) when performing multiple operations on the same object. Cascades reduce repetition and make the chain of mutations read as a single logical statement. Avoid cascading when the intermediate return values matter.

Prefer named parameters with the `required` keyword for functions that take more than two or three arguments, or where positional arguments would be ambiguous. Named parameters are self-documenting at call sites. Use positional parameters for arguments whose role is obvious from context (e.g., a single callback, a clearly-typed value).

Prefer extension methods to add functionality to types you don't own rather than writing top-level utility functions. Extensions keep related behavior discoverable through auto-complete. Avoid extending overly broad types like `Object` or `dynamic`.

Prefer `lowerCamelCase` for variables, functions, methods, and parameters; `UpperCamelCase` for classes, enums, typedefs, and type parameters; `lowercase_with_underscores` for library prefixes and file names. Follow the Effective Dart naming conventions.

Prefer small, focused libraries. Use `part`/`part of` sparingly; prefer separate library files with explicit imports. Use `show` and `hide` combinators when importing to keep the namespace clean and make dependencies obvious.

## Build and Test

Prefer `dart analyze` to catch type errors, unused imports, and style issues before committing. It subsumes the older `dartanalyzer`. Check for an `analysis_options.yaml` at the project root; when one exists, respect its rules. When none exists, `package:lints/recommended.yaml` or `package:flutter_lints/flutter.yaml` provide sensible defaults.

Prefer `dart format` (or `dart format .`) to format code. All Dart code should be formatted before committing. The formatter is opinionated and non-configurable by design; do not fight it.

Prefer `dart test` for pure Dart projects and `flutter test` for Flutter projects. Run `dart pub get` or `flutter pub get` before testing to ensure dependencies are resolved. For Flutter widget tests, use `flutter test`; for integration tests, use `flutter test integration_test/`.

Prefer DCM (formerly `dart_code_metrics`) when the project has it configured, for additional static analysis covering cyclomatic complexity, lines of code, and maintainability metrics. Do not introduce it into projects that don't already use it.

Prefer `dart pub` for dependency management. Pin version constraints in `pubspec.yaml` using caret syntax (`^1.2.3`) for libraries and tighter constraints for applications. Run `dart pub upgrade --major-versions` deliberately, not as part of routine changes.

## Testing

Prefer `package:test` as the foundation. Organize tests with `group()` for logical grouping and `test()` for individual cases. Name test descriptions as plain English sentences that describe the expected behavior, not the implementation.

```dart
group('UserRepository', () {
  test('returns null when user is not found', () {
    // ...
  });
  test('caches repeated lookups by ID', () {
    // ...
  });
});
```

Prefer `setUp` and `tearDown` for shared fixture setup within a group. Use `setUpAll` and `tearDownAll` for expensive operations that can be shared across all tests in a group (database connections, server startup).

Prefer `testWidgets` for Flutter widget tests. Build the widget under test with `pumpWidget`, advance frames with `pump` or `pumpAndSettle`, and find widgets with `find.byType`, `find.text`, or `find.byKey`. Assert with `expect(find.text('Hello'), findsOneWidget)`.

Prefer golden tests (`matchesGoldenFile`) for verifying visual output of widgets that have stable, deterministic rendering. Keep golden files in a `goldens/` or `test/goldens/` directory. Regenerate with `--update-goldens` when intentional visual changes are made.

Prefer mocking with `package:mockito` and code generation (`@GenerateMocks`) or `package:mocktail` for mock objects. Mock at boundaries (HTTP clients, platform channels, repositories), not internal implementation details.

## Common Pitfalls

The `late` keyword defers initialization but crashes with `LateInitializationError` at runtime if the variable is read before assignment. Use `late` only when you can guarantee the assignment happens before any read path. Prefer nullable types with explicit null checks when initialization timing is uncertain.

In Flutter, passing `BuildContext` across an `async` gap is unsafe because the widget may have been unmounted during the await. Check `mounted` before using the context after any asynchronous operation. Prefer passing the data you need (a `ScaffoldMessengerState`, a `NavigatorState`) before the gap rather than the context itself.

Mutable state in `StatelessWidget` or stored directly on `State` fields without calling `setState` will silently fail to trigger rebuilds. All state mutations that should update the UI must go through `setState`, a state management solution, or an equivalent reactive mechanism.

Pubspec version constraints that are too loose (`any`, or bare `>=1.0.0`) invite dependency resolution surprises, especially in published packages. Prefer caret syntax (`^1.2.3`) to allow patch and minor updates while pinning the major version. For applications (not libraries), consider using a `pubspec.lock` committed to version control to ensure reproducible builds.

Dart `Future`s that are neither awaited nor assigned to a variable have their errors silently swallowed. The `unawaited_futures` lint catches this. Prefer awaiting every future, or use `unawaited()` from `dart:async` to signal deliberate fire-and-forget intent.

Equality comparisons on collections use identity by default, not structural equality. `[1, 2] == [1, 2]` is `false`. Use `ListEquality`, `SetEquality`, or `DeepCollectionEquality` from `package:collection`, or prefer `const` collections where possible (const collections with identical elements share identity).
