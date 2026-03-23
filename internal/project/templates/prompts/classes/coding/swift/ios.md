# iOS

When the codebase you're working in has established conventions that differ from what's described here, follow the codebase.

## SwiftUI and Architecture

Prefer SwiftUI with MVVM for new screens. Views should be thin declarative descriptions of UI; push logic, formatting, and side effects into an `@Observable` view model (iOS 17+) or `ObservableObject` with `@Published` properties for older deployment targets. Use `@State` for view-local ephemeral state (toggle flags, text field bindings), `@Binding` to pass write access down the view tree, and `@Environment` for app-wide dependencies (color scheme, managed object context, custom services). Prefer `@Observable` over `ObservableObject` when the minimum target allows it; `@Observable` tracks property access at the individual field level instead of firing on every `objectWillChange`, which eliminates accidental whole-view recomputations.

Prefer `NavigationStack` with `NavigationPath` for programmatic navigation over the deprecated `NavigationView`. Define navigation destinations with `.navigationDestination(for:)` on value types, keeping the router state in the view model so deep links and back-stack manipulation are testable outside the view layer. For tab-based apps, use `TabView` with one `NavigationStack` per tab to keep navigation stacks independent.

## UIKit Interop

Prefer `UIViewRepresentable` and `UIViewControllerRepresentable` when wrapping UIKit components for use in SwiftUI. Implement `makeCoordinator()` for delegate callbacks, and perform updates in `updateUIView(_:context:)` rather than recreating the UIKit view. Avoid storing SwiftUI state in the coordinator; let the representable bridge values from the SwiftUI side into UIKit on each update pass.

## Data Persistence

Prefer SwiftData for new projects targeting iOS 17+. Define models with `@Model`, use `@Query` in views for automatic observation, and inject the `ModelContainer` through the environment. For projects on older targets or with complex migration histories, prefer Core Data with `NSPersistentContainer`. Perform writes on a background `NSManagedObjectContext` created via `newBackgroundContext()` and merge changes to the view context through `automaticallyMergesChangesFromParent`. Never pass `NSManagedObject` instances across context boundaries; use `NSManagedObjectID` and re-fetch.

## Structured Concurrency on iOS

Prefer `@MainActor` on view models that update UI-bound state, so callers never need to remember to dispatch. Use `Task {}` from SwiftUI's `.task` modifier for work tied to a view's lifetime; the task cancels automatically when the view disappears. Prefer `TaskGroup` or `async let` for parallel network calls. Avoid `Task.detached` unless you genuinely need to escape the current actor's isolation; detached tasks lose structured cancellation and priority inheritance.

## SPM and Dependencies

Prefer Swift Package Manager for dependency management. Declare dependencies in `Package.swift` for libraries or through Xcode's package resolution for app targets. Pin versions with `.upToNextMinor(from:)` for dependencies that follow semver, and exact version pins for dependencies with a history of breaking changes. Prefer local packages within the workspace for modularizing app code into feature, domain, and shared layers.

## Testing

Prefer XCTest for iOS projects. Use `ViewInspector` to unit-test SwiftUI view hierarchies without launching a host application; it lets you inspect `@State` bindings and trigger actions on buttons and text fields. Prefer mock protocols over concrete subclass mocking for dependencies (network clients, repositories, services); Swift's lack of runtime reflection makes protocol-based test doubles the cleanest approach. Test async view model methods with `async` test functions and assert state changes after `await`. Use `XCUIApplication` for end-to-end UI tests that exercise real navigation, accessibility identifiers, and OS-level interactions. Prefer Swift Testing's `@Test` with `#expect` for new test targets.

## Common Pitfalls

Capturing `self` strongly in escaping closures passed to `Task {}` or `URLSession` callbacks creates retain cycles when the closure is stored on the capturing object. In SwiftUI view models, `Task` closures inside `.task` are fine (the view manages the lifetime), but long-lived closures registered on publishers or notification observers need `[weak self]`.

Updating UI-bound state from a non-main-actor context triggers runtime warnings in iOS 17+ strict concurrency checking and purple runtime warnings in Instruments. Annotate the view model with `@MainActor` or dispatch explicitly with `await MainActor.run {}` for isolated state mutations from background work.

SwiftUI diffs the view tree by structural identity. Changing a view's position in a conditional branch (wrapping it in a new container, reordering `if/else` arms) resets its state even when the underlying data hasn't changed. Prefer stable view identity through `.id()` modifiers on data-driven collections and consistent conditional structure. Excessive `body` recomputation from reading `@Observable` properties you don't display also wastes frames; split complex views so each subview reads only the properties it renders.

Core Data and SwiftData context operations are not thread-safe. Accessing a managed object from a thread that doesn't own its context corrupts internal state silently, surfacing later as crashes or data loss. Confine objects to their owning context's queue, use `perform {}` or `perform(schedule:)` for cross-queue work, and pass object IDs rather than live objects between actors.
