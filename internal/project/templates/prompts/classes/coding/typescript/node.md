# Node.js (TypeScript)

When the codebase you're working in has established conventions that differ from what's described here, follow the codebase.

## TypeScript Configuration

Prefer `"strict": true` in `tsconfig.json`. This enables `strictNullChecks`, `noImplicitAny`, `strictFunctionTypes`, and related checks. Do not weaken strict settings to make code compile; fix the types instead.

Prefer `"module": "NodeNext"` and `"moduleResolution": "NodeNext"` for projects targeting Node.js. This setting respects the `"type"` field in `package.json` and enforces correct ESM/CJS interop rules at the type level. Import paths must include the `.js` extension even when the source file is `.ts`, because TypeScript emits to JavaScript and the runtime resolves the emitted paths.

Prefer `"target": "ES2022"` or later. Node.js 20+ supports top-level `await`, private class fields, `Array.prototype.at()`, `structuredClone`, and `Error.cause` natively. There is no reason to downlevel to ES2015 unless the output must run on an older runtime.

Prefer `"verbatimModuleSyntax": true` to enforce that type-only imports use the `import type` syntax. This prevents runtime side effects from modules imported solely for their types.

Prefer `"isolatedModules": true` when using a bundler or transpiler (esbuild, swc, tsx) alongside `tsc`. It disallows TypeScript constructs that require whole-program analysis to emit correctly, ensuring each file can be transpiled independently.

## ESM vs CJS

Prefer ES modules when starting a new project. Set `"type": "module"` in `package.json` and use `import`/`export` syntax. CommonJS (`require`/`module.exports`) remains the default when `"type"` is absent.

Prefer `"moduleResolution": "NodeNext"` over `"node"` (the legacy resolution). NodeNext requires explicit file extensions in import specifiers (`import { foo } from './bar.js'`), which aligns with how Node.js actually resolves modules at runtime. The legacy `"node"` resolution accepts extensionless imports, but those fail at runtime in ESM mode.

When publishing a library that must support both ESM and CJS consumers, prefer the `"exports"` field in `package.json` with conditional exports: `{ ".": { "import": "./dist/index.js", "require": "./dist/index.cjs" } }`. Build tools like `tsup` and `tshy` automate dual-format output from a single TypeScript source.

Prefer dynamic `import()` at the boundary when a CJS module must consume an ESM-only dependency. Node.js 22+ allows `require()` of ESM modules under certain conditions, but `import()` works consistently across versions.

## Runtime Patterns

Prefer `node:` prefixed imports (`node:fs`, `node:path`, `node:crypto`, `node:test`) to distinguish built-in modules from npm packages with colliding names.

Prefer `async`/`await` throughout the request lifecycle. When wrapping callback-based APIs, use `node:util` `promisify` or construct a `new Promise` at the boundary rather than mixing callbacks with awaited code.

Prefer Node.js streams with async iteration (`for await (const chunk of stream)`) over manual `.on('data')` listeners. Use `node:stream/promises` `pipeline()` for connecting transform chains; it handles cleanup and error propagation across the full pipe.

Prefer the `--env-file` flag (Node 20.6+) for loading `.env` files without external dependencies. For projects already using `dotenv`, follow the existing pattern. Never commit `.env` files.

## Type-Safe HTTP Frameworks

### Fastify

Prefer Fastify's generic type parameters on route definitions for request/reply typing. `fastify.get<{ Params: { id: string }; Querystring: { page: number } }>('/users/:id', handler)` gives the handler typed access to `request.params.id` and `request.query.page`.

Prefer Fastify's JSON Schema validation with `@fastify/type-provider-typebox` or `@fastify/type-provider-json-schema-to-ts`. Define the schema once; the provider infers TypeScript types from it, eliminating the need to maintain both a schema and a type definition.

Prefer the plugin system for encapsulating related routes, hooks, and decorators. Fastify catches async errors from route handlers automatically; explicit `.catch()` wiring is unnecessary. Use `fastify-plugin` (the wrapper) only when state must be shared across encapsulation boundaries.

Prefer declaration merging to extend Fastify's built-in interfaces when adding custom decorators: `declare module 'fastify' { interface FastifyRequest { user: User } }`. This gives typed access to the decorator throughout the plugin scope.

### Express

Prefer `@types/express` for type definitions. Express middleware and route handlers use `(req: Request, res: Response, next: NextFunction)` signatures from the `express` package's types.

Prefer extending the `Request` interface via declaration merging for custom properties set by middleware: `declare global { namespace Express { interface Request { user: User } } }`. Place this in a `.d.ts` file included by `tsconfig.json`.

Register error-handling middleware (the four-argument `(err: Error, req: Request, res: Response, next: NextFunction)` signature) after all routes. Wrap async route handlers so rejected promises reach the error middleware: `app.get('/path', (req, res, next) => handleAsync(req, res).catch(next))`, or use `express-async-errors` to patch this automatically.

## Environment and Configuration

Prefer typing environment variables at the application boundary. Define a config module that reads `process.env`, validates, parses, and exports a typed configuration object. Code outside this module should import the typed config, never read `process.env` directly.

A common pattern: use a validation library (Zod, Valibot, Ajv) to parse environment variables at startup. The schema serves as both documentation and runtime validation. A missing `DATABASE_URL` that crashes on first request is harder to diagnose than one that crashes on boot.

Prefer declaring `process.env` types with a `.d.ts` file or module augmentation: `declare namespace NodeJS { interface ProcessEnv { DATABASE_URL: string; PORT?: string } }`. This gives type-checked access at `process.env.DATABASE_URL` without per-access assertions.

## Testing

Prefer the project's established test runner. Vitest handles TypeScript natively without separate transform configuration and is the current standard for ESM projects. Jest with `ts-jest` or `@swc/jest` remains common in existing codebases. The built-in `node:test` runner (stable since Node 20) works with TypeScript via `--import tsx` or `--experimental-strip-types` (Node 22+).

Prefer `supertest` (or Fastify's `app.inject()`) for HTTP-level tests that exercise middleware, routing, and serialization together without binding a port.

Prefer typing mock functions to match the real interface. `vi.fn<Parameters<typeof realFn>, ReturnType<typeof realFn>>()` (or the equivalent Jest signature) catches mock drift when the real function's signature changes. A loosely typed `vi.fn()` silently ignores signature mismatches.

## Common Pitfalls

An unhandled promise rejection terminates the process by default in Node 20+. Every async code path must either be awaited in a context with error handling, or have an explicit `.catch()`.

Synchronous operations (`fs.readFileSync`, `crypto.pbkdf2Sync`, CPU-heavy loops) block the event loop and stall all concurrent requests. Prefer async equivalents for I/O. For CPU-bound work, prefer `worker_threads`.

Import paths in TypeScript ESM must end in `.js`, not `.ts`. `import { foo } from './bar.js'` resolves to `./bar.ts` at compile time and `./bar.js` at runtime. This confuses developers new to TypeScript ESM. The TypeScript team has confirmed this is intentional: import paths describe the output, not the source.

Event listeners accumulate silently when `.on()` is called repeatedly without corresponding `.off()`. The default MaxListeners warning (11 listeners) exists to catch this; raising the limit to silence the warning is almost always the wrong fix.

`process.env` values are always strings. `process.env.PORT` is `"3000"`, not `3000`. The typed config module pattern catches this at the boundary; without it, `===` against a number silently fails.

Type assertions on `req.body` (`req.body as CreateUserDto`) bypass runtime validation. The request body is `unknown` at the network boundary regardless of what TypeScript says. Always validate with a schema or validation library before narrowing the type.
