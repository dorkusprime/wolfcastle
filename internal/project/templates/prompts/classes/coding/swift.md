# Swift

When the codebase you're working in has established conventions that differ from what's described here, follow the codebase.

## Style

Prefer value types (`struct`, `enum`) over reference types (`class`) unless you need identity semantics, inheritance, or interop with Objective-C APIs. Value types eliminate an entire class of shared-mutable-state bugs because copies are independent. Use `class` when you need reference identity, deinitializers, or participation in Objective-C runtime features.

Prefer `guard` for early exits and precondition checks. `guard let user = optionalUser else { return }` keeps the happy path at the left margin and makes the unwrapped binding available for the rest of the scope. Use `if let` when you only need the unwrapped value inside a narrow branch.

Prefer protocol-oriented design over deep class hierarchies. Define behavior through protocols with default implementations in extensions. Protocols compose where classes cannot: a type can conform to many protocols but inherit from only one class. Use protocol extensions to provide shared behavior, and constrained extensions (`extension Collection where Element: Comparable`) for type-specific defaults.

Prefer `Result<Success, Failure>` for operations that can fail in ways the caller should handle explicitly. Use `throws` when the function's failure mode is truly exceptional and callers will typically propagate with `try`. Prefer typed throws (`throws(MyError)`) to give callers exhaustive `catch` patterns.

Prefer `Codable` for serialization. Implement `CodingKeys` only when JSON field names diverge from Swift property names. For complex decoding logic, write a custom `init(from:)` rather than reaching for third-party mapping libraries.

Prefer Swift's naming conventions: `lowerCamelCase` for functions, methods, properties, and variables; `UpperCamelCase` for types and protocols. Boolean properties read as assertions (`isEmpty`, `canBecomeFirstResponder`). Methods that perform actions use verb phrases (`append`, `removeAll`); methods that return transformed values use noun or past-participle phrases (`sorted`, `distance(to:)`).

Prefer `async`/`await` over completion handlers for new asynchronous code. Structured concurrency with `TaskGroup` and `async let` makes concurrency relationships explicit. Use actors to protect mutable state accessed from multiple tasks. Mark types `@Sendable` only when they are genuinely safe to cross isolation boundaries.

## Build and Test

Prefer `swift build` and `swift test` for Swift Package Manager projects. Check for a `Package.swift` at the project root. When the project is an Xcode workspace, use `xcodebuild -scheme <SchemeName> -destination <platform> build test` instead.

Prefer SwiftLint for style enforcement. Check for a `.swiftlint.yml` in the project root and run `swiftlint` before committing. When the project uses SwiftFormat, run `swiftformat .` as well; the two tools are complementary (SwiftLint checks rules, SwiftFormat rewrites formatting).

Prefer Swift Package Manager for dependency management in libraries and server-side projects. For iOS/macOS apps, follow whatever the project already uses (SPM, CocoaPods, or Carthage). Do not introduce a second dependency manager into a project that already has one.

## Testing

Prefer XCTest as the baseline testing framework. Organize test cases as subclasses of `XCTestCase`, one per unit under test. Name test methods `test_<scenario>_<expectedBehavior>` or the project's existing convention.

Prefer `XCTAssertEqual`, `XCTAssertTrue`, `XCTAssertNil`, and `XCTAssertThrowsError` over bare `XCTAssert` with string interpolation. Specific assertions produce clearer failure messages.

Prefer `async` test methods for testing asynchronous code (Xcode 13.2+ / Swift 5.5+). Write `func testFetch() async throws` and `await` directly inside the test body, rather than using `XCTestExpectation` and `wait(for:timeout:)`. Use expectations only when testing delegate callbacks or Combine publishers that don't bridge to async.

Prefer Swift Testing (`@Test`, `#expect`, `@Suite`) for new test targets. Swift Testing provides parameterized tests, traits for configuration, and a cleaner assertion syntax than XCTest. When the project uses XCTest, continue with XCTest; do not mix frameworks within a single test target without a migration plan.

Prefer test doubles at boundaries (network, persistence, system clock) rather than mocking internal types. Inject dependencies through initializer parameters or protocol-typed properties. Avoid mocking types you own unless testing interaction behavior at an architectural seam.

## Common Pitfalls

Retain cycles in closures are Swift's most common memory leak. When a closure is stored by the object it captures (`self`), neither can be deallocated. Use `[weak self]` in escaping closures stored on `self`, and `guard let self` at the top of the closure body. Non-escaping closures (the default) cannot cause retain cycles.

Force unwrapping (`!`) crashes at runtime when the value is `nil`. Prefer `guard let`, `if let`, `??` with a default, or `Optional.map`. Reserve `!` for situations where `nil` is a programmer error (e.g., loading a storyboard resource that ships with the app). Never force-unwrap values received from network responses, user input, or external data.

Implicitly unwrapped optionals (`var name: String!`) leak into APIs and infect callers with hidden crash risk. They exist primarily for Interface Builder outlets and two-phase initialization patterns. Do not use them in function signatures, return types, or public properties. Convert to proper optionals or non-optional types during initialization.

`@MainActor` annotations are contagious. Once a type is `@MainActor`, all its methods and properties run on the main thread, including those called from background tasks. This can create unexpected serialization and deadlocks. Prefer isolating only the properties and methods that genuinely touch UI state, or use an actor with `nonisolated` methods for the computation-heavy parts.

ABI stability (Swift 5.0+) means compiled frameworks can be used across Swift versions, but source-level breaking changes still happen between compiler releases. When distributing binary frameworks, use `@frozen` on enums and structs that must remain layout-compatible, and be aware that adding cases to a non-frozen public enum is a source-compatible but binary-breaking change.
