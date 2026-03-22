# Angular

When the codebase you're working in has established conventions that differ from what's described here, follow the codebase.

## Components

Prefer standalone components for all new code. Standalone is the default since Angular 17; NgModules remain valid in existing codebases but add declaration ceremony that standalone components eliminate. Each component imports its own dependencies directly, enabling fine-grained tree-shaking and lazy loading.

Prefer signal-based inputs and outputs over decorators. `input<string>()` and `input.required<string>()` return signals, making reactive derivation with `computed` natural. `output<T>()` returns an `OutputEmitterRef` with a typed `.emit()` method. `model<T>()` replaces the `@Input()` plus `@Output()` pair for two-way binding.

Prefer the built-in control flow (`@if`, `@for`, `@switch`) over structural directives. `@for` requires a `track` expression; always track by a stable identity (`track item.id`), not by object reference or index.

## Signals and Reactivity

Prefer `signal()` for synchronous component state and `computed()` for derived values. Both are read by calling as functions (`count()`, `doubleCount()`). Use `.set()` for replacement and `.update(prev => ...)` for state that depends on the previous value.

Prefer `effect()` sparingly, for side effects that must react to signal changes (logging, localStorage sync, imperative DOM calls). Most derived state belongs in `computed()` instead. Signal reads after an `await` inside `effect()` or `computed()` lose tracking; read all dependencies synchronously before any async boundary.

Prefer `toSignal()` to bridge Observables into the signal graph, and `toObservable()` for the reverse. Call `toSignal()` once per Observable and reuse the returned signal rather than converting repeatedly.

## Dependency Injection

Prefer the `inject()` function for services, guards, resolvers, and base classes. It works in field initializers and eliminates `super()` boilerplate in class hierarchies. Constructor injection remains valid; pick one style per codebase and stay consistent.

Prefer `providedIn: 'root'` for application-wide singletons. Scope services to a component or route with the `providers` array when the service holds per-feature state that should not leak globally.

## Forms

Prefer reactive forms (`FormGroup`, `FormControl`, `FormArray`) for anything beyond trivial inputs. Validators live in TypeScript, making them unit-testable without rendering. Template-driven forms with `ngModel` suit simple, static fields where programmatic control adds no value.

## Routing

Prefer functional guards and resolvers over class-based equivalents (which are deprecated). A guard is a plain function that receives `ActivatedRouteSnapshot` and returns `boolean | UrlTree | Observable<...> | Promise<...>`. The `inject()` function works directly inside functional guards for service access.

Prefer lazy-loaded routes with `loadComponent` for standalone components. Group related routes behind a `loadChildren` call pointing at a route file when the feature has multiple sub-routes.

## Testing

Prefer `provideHttpClient()` followed by `provideHttpClientTesting()` over the older `HttpClientTestingModule`. Order matters: the testing provider must come second. Inject `HttpTestingController` to assert requests and flush responses.

Prefer component harnesses (`TestbedHarnessEnvironment.loader(fixture)`) for interaction tests. Harnesses abstract over DOM structure, making tests resilient to template refactors.

Prefer `await fixture.whenStable()` over manual `fixture.detectChanges()` calls for production-like async behavior. In zoneless configurations, `whenStable()` is the only reliable synchronization point.

Prefer testing signal-based inputs by setting them through the component harness or parent wrapper rather than reaching into the component instance. Test outputs by subscribing to the `OutputEmitterRef` or asserting on the parent's bound handler.

## Common Pitfalls

Every raw `.subscribe()` in a component must be cleaned up. Prefer `toSignal()`, the `async` pipe, or `takeUntilDestroyed()` from `@angular/core/rxjs-interop`. A forgotten subscription silently leaks memory and can fire callbacks on destroyed components.

Tracking `@for` by object reference instead of a stable identity (`track item.id`) causes full re-renders on every data refresh, even when the underlying data has not changed. The performance cost compounds with large collections.

Circular dependency injection (`NG0200`) surfaces at runtime, not compile time. When two services depend on each other, break the cycle with a mediator service, `forwardRef()`, or interface segregation. Architectural refactoring is preferable to `forwardRef()` as a long-term fix.

OnPush components with signal reads in the template automatically mark dirty when the signal changes. The trap is mixing OnPush with zone-triggered Observables that mutate state outside signals: the component will not re-render unless you call `markForCheck()` or bridge the Observable through `toSignal()`.
