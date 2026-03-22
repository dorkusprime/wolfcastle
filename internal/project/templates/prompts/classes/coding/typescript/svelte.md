# Svelte (Svelte 5 / SvelteKit 2)

When the codebase you're working in has established conventions that differ from what's described here, follow the codebase.

## Runes and Reactivity

Prefer `$state` for reactive declarations. For objects and arrays, `$state` creates a deeply reactive proxy; mutations to nested properties and array methods like `push` trigger granular updates. Prefer `$state.raw` for large immutable data structures (API responses, lookup tables) where deep proxying adds overhead without benefit; values must be reassigned wholesale, not mutated.

Prefer `$derived` for computed values. Use `$derived.by(() => { ... })` when the computation requires multiple statements. Using `$effect` to write computed results into a second `$state` variable is an anti-pattern: it creates unnecessary intermediate state and risks infinite loops. Reserve `$effect` for side effects that synchronize with systems outside Svelte (DOM APIs, subscriptions, timers).

Prefer returning a cleanup function from `$effect` for subscriptions, intervals, and event listeners. The cleanup runs before each re-execution and on component destruction. Only values read synchronously inside the effect body are tracked; anything read after an `await`, inside `setTimeout`, or in a callback passed to an external library will not be a dependency.

## Components and Props

Prefer `$props()` with destructuring for component inputs: `let { title, count = 0, ...rest } = $props()`. Type props directly on the destructuring pattern: `let { title, count = 0 }: { title: string; count?: number } = $props()`. Mark props that support `bind:` with `$bindable()` as the default value.

Prefer snippets over the legacy slot API. Declare reusable template fragments with `{#snippet name(params)}` and render them with `{@render name(args)}`. Content placed inside a component tag without an explicit `{#snippet}` wrapper becomes the implicit `children` snippet; render it with `{@render children?.()}`. Type snippet props with `Snippet<[ParamTypes]>` from `svelte`.

## SvelteKit Routing and Data Loading

Prefer `+page.server.ts` for data that requires secrets, cookies, or database access. Prefer `+page.ts` (universal load) when the load function must also run on the client during navigation, or when it returns non-serializable data like component constructors. When both exist, server load runs first and its result is available as the `data` parameter of the universal load.

Prefer form actions in `+page.server.ts` for mutations. Use `fail()` to return validation errors to the same page and `redirect()` for navigation after success. Apply `use:enhance` on forms for progressive enhancement; without a custom callback it handles invalidation and form reset automatically. When providing a custom callback, call `update()` to preserve the default behavior.

Prefer route groups `(groupname)` for organizing layouts without affecting URLs. Use `+layout.svelte` with `{@render children()}` for shared layout structure across route segments.

## Server-Side Rendering

Prefer `$effect` for code that must run only in the browser (DOM measurement, `window` access, third-party browser SDKs). Effects never execute during SSR. For values that differ between server and client (timestamps, randomness), defer them to `$effect` to avoid hydration mismatches.

Prefer `$lib/server/` or `*.server.ts` naming for server-only modules. SvelteKit enforces at build time that these cannot be imported by client code. Access secrets through `$env/static/private` or `$env/dynamic/private`, which carry the same import restriction.

## Stores

Prefer runes over stores for new code. Cross-component shared state belongs in `.svelte.ts` files exporting objects or functions that use `$state` internally. Stores (`svelte/store`) remain useful for interop with store-contract libraries and for `readable` start/stop lifecycle semantics that runes do not replicate.

## Testing

Prefer Svelte Testing Library with Vitest and the `svelteTesting()` vite plugin, which handles automatic cleanup and browser condition resolution. Use `render(Component, { props: { ... } })` to mount, `screen.getByRole` and `screen.getByText` for queries, and `userEvent` over `fireEvent` for realistic interaction simulation. Interaction helpers are async; `await` them to let Svelte flush reactive updates before asserting.

Prefer testing SvelteKit load functions and form actions as plain async functions: import them, pass mock `RequestEvent` objects, and assert on return values and side effects.

## Common Pitfalls

Destructuring a `$state` object extracts snapshot values, severing reactivity. `let { name } = $state({ name: 'x' })` gives a plain, non-reactive `name`. Keep the object intact and access properties through it, or use `$derived` to project individual fields reactively.

`$state` on class fields compiles to getter/setter pairs, making them non-enumerable. `Object.keys()`, `JSON.stringify()`, and spread will not see them. Use `$state.snapshot()` to obtain a plain copy for serialization or logging.

Cross-module `$state` cannot use `export let`. The reactive file must have a `.svelte.ts` extension, and the state should be exported as an object property or through accessor functions, not as a bare `let` binding.

`use:enhance` without a custom callback provides automatic invalidation, form reset, and focus management. Passing a callback that omits the `update()` call silently drops all of that default behavior, which surfaces as forms that submit but never visually respond.
