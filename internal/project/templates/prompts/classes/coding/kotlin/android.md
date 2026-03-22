# Android

When the codebase you're working in has established conventions that differ from what's described here, follow the codebase.

## Architecture and UI

Prefer Jetpack Compose for new UI work over the View system. Build screens as composable functions that take state down and emit events up (unidirectional data flow). Prefer `ViewModel` with `StateFlow` for screen state, exposed via `stateIn(SharingStarted.WhileSubscribed(5000))` so collectors survive configuration changes without leaking upstream work. Use `SharedFlow` for one-shot events (navigation triggers, snackbar messages) rather than stuffing transient signals into the UI state object. Collect flows in composables with `collectAsStateWithLifecycle()` from the lifecycle-runtime-compose artifact; it respects the lifecycle and stops collection when the UI is offscreen.

Prefer the single-Activity architecture with Jetpack Navigation (or Navigation Compose) for screen routing. Define destinations as routes with type-safe arguments. Avoid deep Activity back stacks; multiple activities make state restoration, deep linking, and predictive back gesture support harder to reason about.

## Dependency Injection

Prefer Hilt for dependency injection. Annotate the `Application` class with `@HiltAndroidApp`, activities with `@AndroidEntryPoint`, and inject into ViewModels with `@HiltViewModel`. Use `@Singleton`, `@ActivityRetainedScoped`, or `@ViewModelScoped` to control instance lifetimes. Define modules with `@Provides` for third-party types and `@Binds` for mapping interfaces to implementations. Prefer constructor injection over field injection in all classes that support it.

## Lifecycle and Coroutines

Prefer `viewModelScope` for coroutine work tied to a ViewModel and `lifecycleScope` for work tied to an Activity or Fragment. Use `repeatOnLifecycle(Lifecycle.State.STARTED)` in lifecycle owners to collect flows safely; bare `lifecycleScope.launch` continues running when the UI is in the background, wasting resources and risking updates to a detached view hierarchy. Never launch coroutines from `init` blocks or constructors in lifecycle-aware components without scoping them properly.

Prefer structured concurrency throughout. Avoid `GlobalScope` in Android code. When a coroutine must outlive a ViewModel (uploading a file, syncing data), prefer `WorkManager` for deferrable work or an application-scoped `CoroutineScope` injected via Hilt.

## Data Layer

Prefer Room for local persistence. Define DAOs as interfaces with `@Query`-annotated suspend functions that return `Flow<T>` for observable queries. Use `@Transaction` on methods that perform multiple writes. Provide the database instance via Hilt with a `@Singleton` scope. Prefer migrations over destructive recreation; `fallbackToDestructiveMigration()` loses user data silently.

Prefer Retrofit with a `kotlinx.serialization` converter (or Moshi) for REST networking. Define API interfaces with suspend functions returning the response body type directly; use `Response<T>` wrapper only when you need to inspect status codes or headers. For newer projects, Ktor client is a viable alternative with native coroutine support. Provide `OkHttpClient` and API service instances through Hilt modules.

## Gradle Configuration

Prefer the Kotlin DSL (`build.gradle.kts`) with a version catalog (`libs.versions.toml`) for dependency management. Use `buildFeatures { compose = true }` and set the Compose compiler version to match the Kotlin version via the Compose compiler Gradle plugin. Prefer convention plugins in `build-logic/` for shared configuration across modules rather than duplicating blocks in each module's build file. Enable `android.nonTransitiveRClass = true` to keep R class references scoped to their defining module.

## Testing

Prefer JUnit 4 with the AndroidX test libraries for unit tests; Android's testing ecosystem still centers on JUnit 4 runners. Use Robolectric for tests that touch Android framework classes (Context, SharedPreferences, Resources) without requiring a device. Prefer Compose testing APIs (`createComposeRule`, `onNodeWithText`, `performClick`) for UI component tests; they run on the JVM with Robolectric or on a device with the same assertions. Use Espresso only for instrumented end-to-end tests that must run on a real device or emulator.

Prefer Turbine for testing `Flow` emissions from ViewModels and repositories. Inject `TestDispatcher` via Hilt test modules so coroutine timing is deterministic. Use `StandardTestDispatcher` for explicit advancement and `UnconfinedTestDispatcher` when execution order doesn't matter.

## Common Pitfalls

Launching coroutines in `lifecycleScope.launch` without `repeatOnLifecycle` keeps the collection active while the app is backgrounded. On a flow that emits frequently (sensor data, location updates), this drains battery and may crash if the collector updates a composable or view that's no longer attached.

Configuration changes (rotation, locale switch, dark mode toggle) destroy and recreate the Activity. State held in Activity fields or companion objects vanishes. Prefer `ViewModel` for transient UI state and `rememberSaveable` in Compose (or `SavedStateHandle` in ViewModel) for state that must survive process death.

Compose recomposition skips composables whose parameters haven't changed, but only when parameter types are stable. List, Map, and other mutable collection types are unstable by default, triggering unnecessary recompositions. Prefer `kotlinx.collections.immutable` (`ImmutableList`, `PersistentMap`) or annotate custom types with `@Immutable`/`@Stable` when the contract genuinely holds. Profile recomposition counts with Layout Inspector before optimizing.

ProGuard and R8 strip classes, methods, and fields they consider unreferenced. Reflection-based libraries (serialization, Retrofit, Hilt, Room) break silently in release builds if their generated code or annotated classes get removed. Keep ProGuard rules in sync with dependencies; most Jetpack and Square libraries ship their own rules, but custom `@SerialName` mappings or Retrofit interfaces sometimes need explicit `-keep` directives. Test the release build, not just debug.
