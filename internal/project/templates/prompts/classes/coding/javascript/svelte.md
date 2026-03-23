# Svelte (JavaScript, Svelte 5 / SvelteKit 2)

When the codebase you're working in has established conventions that differ from what's described here, follow the codebase.

## Runes and Reactivity

Prefer `$state` for reactive declarations. For objects and arrays, `$state` creates a deeply reactive proxy; mutations to nested properties and array methods like `push` trigger granular updates. Prefer `$state.raw` for large immutable data structures (API responses, lookup tables) where deep proxying adds overhead without benefit; values must be reassigned wholesale, not mutated.

Prefer `$derived` for computed values. Use `$derived.by(() => { ... })` when the computation requires multiple statements. Using `$effect` to write computed results into a second `$state` variable is an anti-pattern: it creates unnecessary intermediate state and risks infinite loops. Reserve `$effect` for side effects that synchronize with systems outside Svelte (DOM APIs, subscriptions, timers).

Prefer returning a cleanup function from `$effect` for subscriptions, intervals, and event listeners. The cleanup runs before each re-execution and on component destruction. Only values read synchronously inside the effect body are tracked; anything read after an `await`, inside `setTimeout`, or in a callback passed to an external library will not be a dependency.

## Components and Props

Prefer `$props()` with destructuring for component inputs: `let { title, count = 0, ...rest } = $props()`. Without TypeScript, type annotations on the destructuring pattern are unavailable. Use JSDoc comments above the declaration to document expected types: `/** @type {{ title: string, count?: number }} */`.

Prefer `$bindable()` as the default value for props that support `bind:` from the parent.

Prefer snippets over the legacy slot API. Declare reusable template fragments with `{#snippet name(params)}` and render them with `{@render name(args)}`. Content placed inside a component tag without an explicit `{#snippet}` wrapper becomes the implicit `children` snippet; render it with `{@render children?.()}`.

## SvelteKit Routing and Data Loading

Prefer `+page.server.js` for data that requires secrets, cookies, or database access. Prefer `+page.js` (universal load) when the load function must also run on the client during navigation, or when it returns non-serializable data like component constructors. When both exist, server load runs first and its result is available as the `data` parameter of the universal load.

Prefer form actions in `+page.server.js` for mutations. Use `fail()` to return validation errors to the same page and `redirect()` for navigation after success. Apply `use:enhance` on forms for progressive enhancement; without a custom callback it handles invalidation and form reset automatically. When providing a custom callback, call `update()` to preserve the default behavior.

Prefer route groups `(groupname)` for organizing layouts without affecting URLs. Use `+layout.svelte` with `{@render children()}` for shared layout structure across route segments.

## Server-Side Rendering

Prefer `$effect` for code that must run only in the browser (DOM measurement, `window` access, third-party browser SDKs). Effects never execute during SSR. For values that differ between server and client (timestamps, randomness), defer them to `$effect` to avoid hydration mismatches.

Prefer `$lib/server/` or `*.server.js` naming for server-only modules. SvelteKit enforces at build time that these cannot be imported by client code. Access secrets through `$env/static/private` or `$env/dynamic/private`, which carry the same import restriction.

## Stores

Prefer runes over stores for new code. Cross-component shared state belongs in `.svelte.js` files exporting objects or functions that use `$state` internally. Stores (`svelte/store`) remain useful for interop with store-contract libraries and for `readable` start/stop lifecycle semantics that runes do not replicate.

## JavaScript-Specific Concerns

Without TypeScript, SvelteKit load functions lose their inferred return types. The `data` prop in `+page.svelte` is untyped; destructure it with JSDoc annotations for editor support: `/** @type {{ items: Array<{id: string, name: string}> }} */ let { data } = $props()`.

Prefer JSDoc `@typedef` blocks in shared files for shapes used across multiple load functions and components. Import them with `/** @type {import('./types.js').Item} */` to keep documentation consistent without a build step.

Prop validation in Svelte is minimal by design. There is no equivalent to React's PropTypes or Vue's validator option. Runtime type checks at the top of the component script (`if (typeof title !== 'string') throw ...`) serve as the safety net in JavaScript projects. Prefer validating at the data boundary (load functions, form actions) rather than inside every component.

## Testing

Prefer Svelte Testing Library with Vitest and the `svelteTesting()` vite plugin, which handles automatic cleanup and browser condition resolution. Use `render(Component, { props: { ... } })` to mount, `screen.getByRole` and `screen.getByText` for queries, and `userEvent` over `fireEvent` for realistic interaction simulation. Interaction helpers are async; `await` them to let Svelte flush reactive updates before asserting.

Prefer testing SvelteKit load functions and form actions as plain async functions: import them, pass mock `RequestEvent` objects, and assert on return values and side effects. Load functions are regular JavaScript functions; they do not require the Svelte compiler or DOM to test.

Without TypeScript, mock objects for `RequestEvent`, `Cookies`, and other SvelteKit types must be constructed manually. Prefer creating factory functions (`createMockEvent()`, `createMockCookies()`) in a test utility file to keep mock construction consistent across tests.

## Common Pitfalls

Destructuring a `$state` object extracts snapshot values, severing reactivity. `let { name } = $state({ name: 'x' })` gives a plain, non-reactive `name`. Keep the object intact and access properties through it, or use `$derived` to project individual fields reactively.

`$state` on class fields compiles to getter/setter pairs, making them non-enumerable. `Object.keys()`, `JSON.stringify()`, and spread will not see them. Use `$state.snapshot()` to obtain a plain copy for serialization or logging.

Cross-module `$state` cannot use `export let`. The reactive file must have a `.svelte.js` extension, and the state should be exported as an object property or through accessor functions, not as a bare `let` binding.

`use:enhance` without a custom callback provides automatic invalidation, form reset, and focus management. Passing a callback that omits the `update()` call silently drops all of that default behavior, which surfaces as forms that submit but never visually respond.

Without TypeScript, typos in load function return keys (`{ itmes: [] }` instead of `{ items: [] }`) propagate silently until the component reads `data.items` and gets `undefined`. Prefer naming tests after load function return shapes to catch these mismatches early.
