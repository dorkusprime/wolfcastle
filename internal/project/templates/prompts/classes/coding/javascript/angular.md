# Angular (JavaScript)

When the codebase you're working in has established conventions that differ from what's described here, follow the codebase.

Angular projects overwhelmingly use TypeScript. A JavaScript Angular project is uncommon but valid. The framework's APIs, lifecycle hooks, and architectural patterns are the same regardless of language; the differences lie in how you express types, configure the build, and handle the absence of compile-time safety.

## Components

Prefer standalone components for all new code. Standalone is the default since Angular 17; NgModules add declaration ceremony that standalone components eliminate. Each component imports its own dependencies directly, enabling fine-grained tree-shaking and lazy loading.

Prefer signal-based inputs and outputs over decorators. `input()` and `input.required()` return signals; `output()` returns an `OutputEmitterRef` with a typed `.emit()` method; `model()` replaces the `@Input()` plus `@Output()` pair for two-way binding. Without TypeScript, the generic type parameters are unavailable, so signals carry no compile-time type information. Rely on JSDoc annotations and runtime validation to compensate.

Prefer the built-in control flow (`@if`, `@for`, `@switch`) over structural directives. `@for` requires a `track` expression; always track by a stable identity (`track item.id`), not by object reference or index.

## Signals and Reactivity

Prefer `signal()` for synchronous component state and `computed()` for derived values. Both are read by calling as functions (`count()`, `doubleCount()`). Use `.set()` for replacement and `.update(prev => ...)` for state that depends on the previous value.

Prefer `linkedSignal()` (Angular 19+) when a signal should reset to a computed default whenever a source changes, but still accept direct writes. This replaces the pattern of using `effect()` to reset a `signal()` in response to another signal's changes.

Prefer `resource()` and `rxResource()` (Angular 19+) for async data loading triggered by signal changes. `resource()` accepts a promise-based loader; `rxResource()` (from `@angular/core/rxjs-interop`) accepts an Observable-based loader. Both expose `.value()`, `.isLoading()`, and `.error()` signals.

Prefer `effect()` sparingly, for side effects that must react to signal changes (logging, localStorage sync, imperative DOM calls). Most derived state belongs in `computed()`. Signal reads after an `await` inside `effect()` or `computed()` lose tracking; read all dependencies synchronously before any async boundary.

Prefer `toSignal()` to bridge Observables into the signal graph, and `toObservable()` for the reverse. Call `toSignal()` once per Observable and reuse the returned signal.

## JavaScript-Specific Concerns

Prefer JSDoc type annotations on component classes, services, and public methods. Angular's tooling (language service, IDE support) reads JSDoc when TypeScript types are absent. Annotate signal values, input shapes, and service method signatures: `/** @param {string} name */`.

Prefer runtime validation at component boundaries. Without TypeScript's compile-time checks, a misspelled property name or wrong-typed input propagates silently until it causes a runtime error deep in the rendering tree. Use Angular's built-in validation where available and add defensive checks in `ngOnInit` or constructor logic for critical inputs.

Prefer `@angular/compiler-cli` configuration that targets JavaScript output. The Angular CLI generates TypeScript by default; for a JavaScript project, the build pipeline must transpile or the project must use a custom Webpack/esbuild configuration. Follow whatever build setup the project has established.

## Dependency Injection

Prefer the `inject()` function for services, guards, resolvers, and base classes. It works in field initializers and eliminates `super()` boilerplate. Constructor injection remains valid; pick one style per codebase and stay consistent.

Prefer `providedIn: 'root'` for application-wide singletons. Scope services to a component or route with the `providers` array when the service holds per-feature state that should not leak globally.

## Forms

Prefer reactive forms (`FormGroup`, `FormControl`, `FormArray`) for anything beyond trivial inputs. Validators live in JavaScript, making them unit-testable without rendering. Template-driven forms with `ngModel` suit simple, static fields where programmatic control adds no value.

Without TypeScript's typed forms (`FormGroup<{ name: FormControl<string> }>`), reactive form controls lose type inference. Access `.value` with care; it is always `any` at the type level. Prefer centralizing form-value extraction in helper functions with JSDoc-annotated return types.

## Routing

Prefer functional guards and resolvers over class-based equivalents (which are deprecated). A guard is a plain function that receives `ActivatedRouteSnapshot` and returns `boolean | UrlTree | Observable<...> | Promise<...>`. The `inject()` function works directly inside functional guards for service access.

Prefer lazy-loaded routes with `loadComponent` for standalone components. Group related routes behind a `loadChildren` call pointing at a route file when the feature has multiple sub-routes.

## Testing

Prefer `provideHttpClient()` followed by `provideHttpClientTesting()` over the older `HttpClientTestingModule`. Order matters: the testing provider must come second. Inject `HttpTestingController` to assert requests and flush responses.

Prefer component harnesses (`TestbedHarnessEnvironment.loader(fixture)`) for interaction tests. Harnesses abstract over DOM structure, making tests resilient to template refactors.

Prefer `await fixture.whenStable()` over manual `fixture.detectChanges()` calls for production-like async behavior. In zoneless configurations, `whenStable()` is the only reliable synchronization point.

Without TypeScript, test setup code cannot rely on type inference to catch mismatched mock shapes. Prefer documenting mock contracts with JSDoc and validating that mocks match the real service interface through integration tests.

## Common Pitfalls

Every raw `.subscribe()` in a component must be cleaned up. Prefer `toSignal()`, the `async` pipe, or `takeUntilDestroyed()` from `@angular/core/rxjs-interop`. A forgotten subscription silently leaks memory and can fire callbacks on destroyed components.

Without TypeScript, the Angular compiler cannot catch template binding errors at build time. A misspelled property binding (`[naem]` instead of `[name]`) produces a silent `undefined` at runtime rather than a compile error. The Angular language service provides some checking in IDEs, but it is less thorough without type information. Prefer running the full test suite as the primary safety net.

Tracking `@for` by object reference instead of a stable identity (`track item.id`) causes full re-renders on every data refresh. The performance cost compounds with large collections.

OnPush components with signal reads in the template automatically mark dirty when the signal changes. Mixing OnPush with zone-triggered Observables that mutate state outside signals causes missed re-renders unless you call `markForCheck()` or bridge through `toSignal()`.

Zoneless change detection (experimental since Angular 18, improved in 19) removes the zone.js dependency entirely, improving performance and debugging. In zoneless mode, change detection is driven entirely by signals and explicit `markForCheck()` calls.
