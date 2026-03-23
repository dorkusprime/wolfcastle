# Next.js (JavaScript)

When the codebase you're working in has established conventions that differ from what's described here, follow the codebase.

## Server and Client Components

All components in the App Router are Server Components by default. Add `'use client'` only to the specific leaf components that need interactivity (state, effects, event handlers, browser APIs). Pushing the boundary down keeps the client bundle small.

Prefer passing Server Components as `children` to Client Components (the "donut" pattern) over marking entire subtrees as client code. The server-rendered content slots into the client wrapper without entering the client bundle.

Prefer `import 'server-only'` in files that use secrets, database clients, or other server-side resources. This produces a build error if a Client Component accidentally imports the file, preventing credential leakage and bundle bloat.

## Data Fetching

Prefer Server Actions for mutations. Define them in dedicated `'use server'` files when they need to be called from Client Components; inline `'use server'` works only inside Server Components. Server Actions are public HTTP endpoints, so always verify authentication and authorization inside each function.

Prefer `fetch()` with explicit caching options. Since Next.js 15, fetches, GET Route Handlers, and client navigations are not cached by default. Opt in with `{ cache: 'force-cache' }` or `{ next: { revalidate: seconds } }`. Use `{ next: { tags: ['key'] } }` combined with `revalidateTag('key')` in Server Actions for on-demand invalidation.

Next.js 16 introduces the `'use cache'` directive for explicit, granular caching of pages, components, and functions. The compiler generates cache keys automatically. This replaces the implicit caching model of earlier App Router versions. All dynamic code runs at request time by default; caching is entirely opt-in.

Prefer React's `cache()` function to deduplicate identical data-fetching calls within a single render pass (for example, sharing a query between `generateMetadata` and the page component).

## Routing Conventions

Prefer the standard file conventions: `page.jsx` makes a route accessible, `layout.jsx` wraps children and persists across navigations, `loading.jsx` provides an instant Suspense fallback, and `error.jsx` (which must be `'use client'`) catches runtime errors for the segment. Use `template.jsx` instead of `layout.jsx` only when the wrapper must remount on every navigation (animations, per-navigation effects).

Prefer `not-found.jsx` for custom 404 UI triggered by `notFound()`. Use `route.js` for API endpoints (Route Handlers); a route file cannot coexist with `page.jsx` in the same segment.

In Next.js 15+, `params` and `searchParams` are Promises. Await them in page and layout components: `const { slug } = await params`.

## Metadata

Prefer the static `metadata` export for pages with fixed titles and descriptions. Use `generateMetadata` (an async function receiving `params` and `parent`) for dynamic metadata that depends on route parameters or fetched data. Metadata merges from root layout down; more specific values override parents.

## Middleware

Prefer middleware (`middleware.js` at the project root) for cross-cutting concerns: redirects, rewrites, header manipulation, cookie-based routing. Use the `matcher` config to scope it to specific paths. Middleware should not be the sole authentication gate; always re-verify inside Server Actions and data-fetching functions.

## JavaScript-Specific Concerns

Without TypeScript, the boundary between Server and Client Components has no compile-time enforcement beyond the `'use client'` directive. A Server Component that accidentally imports a client-only library (or vice versa) fails at runtime. Prefer `import 'server-only'` and `import 'client-only'` as guardrails.

Prefer JSDoc annotations on Server Actions, page components, and layout components to document their parameter shapes. `/** @param {{ params: Promise<{ slug: string }> }} props */` gives editors useful autocompletion without TypeScript.

Prefer PropTypes on Client Components that receive props from Server Components. The serialization boundary between server and client is invisible in JavaScript; PropTypes provide the only development-time warning when a non-serializable value (function, class instance) crosses the boundary.

Next.js configuration (`next.config.js` or `next.config.mjs`) uses the same format regardless of language. Prefer `next.config.mjs` for ES module syntax consistency when the project uses `"type": "module"` in `package.json`.

Turbopack is stable and the default bundler in Next.js 16, providing significantly faster Fast Refresh and builds compared to Webpack. It supports filesystem caching in development, storing compiler artifacts on disk between runs.

## Testing

Prefer end-to-end tests (Playwright or Cypress) for pages that combine Server and Client Components; unit-testing async Server Components in isolation is still maturing. Use Vitest or Jest with `next/jest` for Client Components, hooks, and utility functions.

Prefer manual mocks for `next/navigation` (`useRouter`, `usePathname`, `useSearchParams`) in unit tests. Mock the module at the test-framework level (`vi.mock('next/navigation', ...)`) and return controlled values.

Prefer testing Server Actions as plain async functions: import them directly, pass arguments, and assert on return values and side effects (database writes, `revalidateTag` calls). Without TypeScript, assert on the shape of the return value to catch regressions when the action's contract changes.

## Common Pitfalls

Props passed from Server Components to Client Components must be serializable (strings, numbers, booleans, plain objects, arrays, Dates, Maps, Sets, FormData, Promises, React elements). Functions, class instances, and closures cannot cross the boundary. The error surfaces at runtime, not at build time. In JavaScript, there is no type checker to flag this; the first sign is a hydration error or a cryptic serialization failure.

Hydration mismatches arise when server and client renders produce different markup. Common causes: `Date.now()` or `Math.random()` called during render, locale-dependent formatting, browser extensions modifying the DOM. Prefer stable values or defer non-deterministic content to `useEffect`.

Importing a server-only module from a `'use client'` file pulls its entire dependency tree into the client bundle. The `server-only` package catches this at build, but without it the bloat is silent. Keep `'use client'` boundaries at the leaf level and audit imports when adding the directive to a file.

Prefer `next/dynamic` over `React.lazy` in Next.js applications. `next/dynamic` wraps `React.lazy` with chunk prefetching during SSR and an `ssr: false` option for components that must skip server rendering (Client Components only). In Server Components, async components work natively; neither `React.lazy` nor `next/dynamic` is needed.

Missing or non-unique `key` props on list items cause React to misidentify which items changed. Without TypeScript, no tooling warns about this at build time. Prefer the ESLint plugin `eslint-plugin-react` with the `react/jsx-key` rule enabled.
