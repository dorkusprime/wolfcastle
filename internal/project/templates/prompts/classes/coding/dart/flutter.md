# Flutter

When the codebase you're working in has established conventions that differ from what's described here, follow the codebase.

## Widget Architecture

Prefer small, focused widgets over deeply nested build methods. Extract subtrees into their own widget classes when a build method exceeds roughly 30-40 lines or when a subtree has its own logical identity. Prefer `StatelessWidget` by default; reach for `StatefulWidget` only when the widget owns lifecycle-dependent state (animation controllers, scroll controllers, text editing controllers). Dispose controllers in `dispose()` without exception.

Prefer composition over inheritance for widget reuse. Build complex screens by combining small widgets that each manage one concern, rather than subclassing existing widgets or using mixins to share build logic.

## State Management

Prefer Riverpod (`flutter_riverpod`) for state management in new projects. Define providers at the top level as global finals; use `ref.watch` in widgets for reactive rebuilds and `ref.read` for one-shot access in callbacks. Prefer `NotifierProvider` and `AsyncNotifierProvider` (Riverpod 2.x code-gen style) over the legacy `StateNotifierProvider`. For projects already using Bloc, prefer Cubit for simple state with `emit()` and full Bloc with event classes only when you need event-driven traceability or complex event transformations like `debounce` or `throttle`. Do not mix state management solutions within the same feature.

## Navigation

Prefer `go_router` for declarative routing. Define routes with `GoRoute` and use `context.go()` for navigation that replaces the stack and `context.push()` for pushing onto it. Keep route paths as constants to avoid string duplication. Use `ShellRoute` for persistent scaffolds (bottom nav bars, side drawers) that wrap multiple child routes. Handle deep links by declaring path parameters and query parameters in route definitions rather than parsing raw URIs manually.

## Platform Channels and Native Interop

Prefer `MethodChannel` for one-off calls between Dart and native code and `EventChannel` for continuous streams (sensor data, location updates). Serialize arguments as simple types (strings, maps, lists of primitives) to avoid codec mismatches. For projects with substantial native interop, prefer Pigeon to generate type-safe platform channel bindings rather than maintaining string-keyed method calls by hand. Prefer FFI (`dart:ffi` with `ffigen`) for direct C library calls where latency matters.

## Theming and Design

Prefer `ThemeData` with `ColorScheme.fromSeed()` for Material 3 theming. Define a single `ThemeData` for light mode and one for dark mode; pass both to `MaterialApp`'s `theme` and `darkTheme` parameters. Override individual component themes (`ElevatedButtonThemeData`, `InputDecorationTheme`) within the `ThemeData` rather than styling widgets inline. For apps targeting both Android and iOS fidelity, use `platform`-aware checks or the `flutter_platform_widgets` package to switch between Material and Cupertino components where the design calls for it.

## Build Flavors and Environments

Prefer Dart `--dart-define` or `--dart-define-from-file` for compile-time environment configuration (API base URLs, feature flags). Access values with `String.fromEnvironment`. Use `flutter build` with `--flavor` and corresponding `productFlavors` in Android and schemes in Xcode for builds that need distinct app IDs, icons, or signing configurations. Keep environment switching out of runtime code; compile-time constants let the tree shaker remove unused branches.

## Testing

Prefer `flutter_test` for widget tests. Build the widget under test inside `pumpWidget` wrapped in `MaterialApp` or the relevant ancestor providers so that theme, navigation, and media query contexts resolve correctly. Use `pumpAndSettle` for animations and `pump(duration)` for time-dependent behavior. Prefer `find.byKey` with stable `ValueKey` identifiers over `find.text` for assertions that shouldn't break when copy changes.

Prefer the `integration_test` package for end-to-end tests running on a real device or emulator. Use `patrol` when tests need to interact with OS-level UI (permission dialogs, notifications, system settings). Keep integration tests in `integration_test/` and run with `flutter test integration_test/`.

Mock services and repositories at the provider level (override Riverpod providers in `ProviderScope.overrides` or inject mock Cubits) rather than mocking widget internals. Prefer `mocktail` over `mockito` in new Flutter projects; it requires no code generation.

## Common Pitfalls

Calling `setState` after `dispose` throws a `FlutterError`. Any async callback (timer, stream subscription, future completion) that outlives the widget must check `mounted` before calling `setState`, or be cancelled in `dispose()`. Prefer cancelling subscriptions over checking `mounted`.

Placing unbounded-height children (`ListView`, `GridView`, `Column` with `Expanded`) inside a parent that also has unbounded height (another `ListView`, a `SingleChildScrollView`) causes layout failures. Constrain the inner list with `shrinkWrap: true` (with caution for performance) or a fixed `SizedBox` height, or prefer `CustomScrollView` with slivers to compose multiple scrollable regions into a single viewport.

Omitting `Key` on widgets in dynamic lists, or using index-based keys, causes Flutter to reuse state from the wrong element when items are reordered, inserted, or removed. Prefer `ValueKey` based on a stable identifier from the data model. Conversely, assigning new `UniqueKey()` on every rebuild forces unnecessary state destruction; only do this when you intentionally want to reset a widget.

Platform rendering differs between Android (Material, Skia/Impeller) and iOS (Cupertino, Impeller). Pixel-perfect golden tests generated on one platform will fail on the other. Run goldens on a single CI platform and tag them by OS. Test visual fidelity on both platforms through integration tests rather than cross-platform golden matching.
