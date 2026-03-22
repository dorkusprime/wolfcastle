# Next.js (App Router)

When the codebase you're working in has established conventions that differ from what's described here, follow the codebase.

## Server and Client Components

All components in the App Router are Server Components by default. Add `'use client'` only to the specific leaf components that need interactivity (state, effects, event handlers, browser APIs). Pushing the boundary down keeps the client bundle small.

Prefer passing Server Components as `children` to Client Components (the "donut" pattern) over marking entire subtrees as client code. The server-rendered content slots into the client wrapper without entering the client bundle.

Prefer `import 'server-only'` in files that use secrets, database clients, or other server-side resources. This produces a build error if a Client Component accidentally imports the file, preventing credential leakage and bundle bloat.

## Data Fetching

Prefer Server Actions for mutations. Define them in dedicated `'use server'` files when they need to be called from Client Components; inline `'use server'` works only inside Server Components. Server Actions are public HTTP endpoints, so always verify authentication and authorization inside each function.

Prefer `fetch()` with explicit caching options. Next.js 15 does not cache fetches by default; opt in with `{ cache: 'force-cache' }` or `{ next: { revalidate: seconds } }`. Use `{ next: { tags: ['key'] } }` combined with `revalidateTag('key')` in Server Actions for on-demand invalidation.

Prefer React's `cache()` function to deduplicate identical data-fetching calls within a single render pass (for example, sharing a query between `generateMetadata` and the page component).

## Routing Conventions

Prefer the standard file conventions: `page.tsx` makes a route accessible, `layout.tsx` wraps children and persists across navigations, `loading.tsx` provides an instant Suspense fallback, and `error.tsx` (which must be `'use client'`) catches runtime errors for the segment. Use `template.tsx` instead of `layout.tsx` only when the wrapper must remount on every navigation (animations, per-navigation effects).

Prefer `not-found.tsx` for custom 404 UI triggered by `notFound()`. Use `route.ts` for API endpoints (Route Handlers); a route file cannot coexist with `page.tsx` in the same segment.

In Next.js 15+, `params` and `searchParams` are Promises. Await them in page and layout components: `const { slug } = await params`.

## Metadata

Prefer the static `metadata` export for pages with fixed titles and descriptions. Use `generateMetadata` (an async function receiving `params` and `parent`) for dynamic metadata that depends on route parameters or fetched data. Metadata merges from root layout down; more specific values override parents.

## Middleware

Prefer middleware (`middleware.ts` at the project root) for cross-cutting concerns: redirects, rewrites, header manipulation, cookie-based routing. Use the `matcher` config to scope it to specific paths. Middleware should not be the sole authentication gate; always re-verify inside Server Actions and data-fetching functions.

## Testing

Prefer end-to-end tests (Playwright or Cypress) for pages that use async Server Components; unit-testing tooling for async server components is still maturing. Use Vitest or Jest with `next/jest` for Client Components, hooks, and utility functions.

Prefer manual mocks for `next/navigation` (`useRouter`, `usePathname`, `useSearchParams`) in unit tests, since no official mocking utilities exist. Mock the module at the test-framework level (`vi.mock('next/navigation', ...)`) and return controlled values.

Prefer testing Server Actions as plain async functions: import them directly, pass arguments, and assert on return values and side effects (database writes, `revalidateTag` calls).

## Common Pitfalls

Props passed from Server Components to Client Components must be serializable (strings, numbers, booleans, plain objects, arrays, Dates, Maps, Sets, FormData, Promises, React elements). Functions, class instances, and closures cannot cross the boundary. The error surfaces at runtime, not at build time.

Hydration mismatches arise when server and client renders produce different markup. Common causes: `Date.now()` or `Math.random()` called during render, locale-dependent formatting, browser extensions modifying the DOM. Prefer stable values or defer non-deterministic content to `useEffect`.

Importing a server-only module from a `'use client'` file pulls its entire dependency tree into the client bundle. The `server-only` package catches this at build, but without it the bloat is silent. Keep `'use client'` boundaries at the leaf level and audit imports when adding the directive to a file.

Prefer `next/dynamic` over `React.lazy` in Next.js applications. `next/dynamic` wraps `React.lazy` with chunk prefetching during SSR and an `ssr: false` option for components that must skip server rendering (Client Components only). In Server Components, async components work natively; neither `React.lazy` nor `next/dynamic` is needed.
