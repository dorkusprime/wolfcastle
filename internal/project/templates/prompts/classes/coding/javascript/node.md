# Node.js

When the codebase you're working in has established conventions that differ from what's described here, follow the codebase.

## Module System

Prefer ES modules when the project uses `"type": "module"` in `package.json` or `.mjs` extensions. When CommonJS is established, stay with it. Mixing the two in a single project creates subtle interop friction: `require()` of ESM returns a promise in older Node versions and behaves differently across versions. If migration is in progress, convert leaf modules first (those with few dependents) and bridge at the edges with dynamic `import()`.

Prefer the `exports` field in `package.json` over bare `main` when publishing or organizing entry points. It controls what consumers can import and supports conditional exports for ESM and CJS simultaneously.

## Runtime Patterns

Prefer `async`/`await` throughout the request lifecycle. When wrapping callback-based APIs, use `node:util` `promisify` or construct a `new Promise` at the boundary rather than mixing callbacks with awaited code.

Prefer Node.js streams with async iteration (`for await (const chunk of stream)`) over manual `.on('data')` listeners. Async iterators handle backpressure implicitly and compose cleanly with `try`/`catch`. Use `node:stream/promises` `pipeline()` for connecting transform chains; it handles cleanup and error propagation across the full pipe.

Prefer `Buffer.from()` and `Buffer.alloc()` over the deprecated `new Buffer()` constructor. When converting between strings and binary data, always specify the encoding explicitly (`'utf8'`, `'base64'`, `'hex'`).

Prefer `node:` prefixed imports (`node:fs`, `node:path`, `node:crypto`) to make it unambiguous that you're importing a built-in rather than an npm package with a colliding name.

## HTTP Frameworks

Prefer the project's established framework. In Express, register error-handling middleware (the four-argument `(err, req, res, next)` signature) after all routes; without it, async errors crash the process or produce empty 500 responses. Wrap async route handlers so rejected promises reach the error middleware: `app.get('/path', (req, res, next) => handleAsync(req, res).catch(next))`, or use `express-async-errors` to patch this automatically.

In Fastify, prefer the plugin system for encapsulating related routes, hooks, and decorators. Fastify catches async errors from route handlers automatically; explicit `.catch()` wiring is unnecessary. Prefer `fastify.register()` with `fastify-plugin` only when state must be shared across encapsulation boundaries.

## Environment and Configuration

Prefer `--env-file` (Node 20.6+) for loading `.env` files without external dependencies. For projects already using `dotenv`, follow the existing pattern. Never commit `.env` files; use `.env.example` to document required variables without values.

Prefer reading configuration at startup and validating eagerly. A missing `DATABASE_URL` that surfaces on the first request ten minutes into production is harder to diagnose than one that crashes the process immediately on boot.

## Testing

Prefer the project's established test runner. The built-in `node:test` runner (stable since Node 20) provides `describe`/`it`, mocking via `mock.method()`, timer mocking via `mock.timers`, and coverage reporting without external dependencies. For projects on Jest, use `jest.fn()` and `jest.spyOn()` in the usual way.

Prefer `supertest` (or the framework's injection method, like Fastify's `app.inject()`) for HTTP-level tests that exercise middleware, routing, and serialization together without binding a port.

Prefer mocking `node:fs`, `node:child_process`, and other system interfaces at the module level rather than touching the real filesystem or spawning real processes in unit tests. Use the test runner's built-in mock facilities when available.

## Common Pitfalls

An unhandled promise rejection terminates the process by default in Node 20+. Every async code path must either be awaited in a context with error handling, or have an explicit `.catch()`. The `unhandledRejection` process event is a diagnostic backstop, not a recovery mechanism.

Synchronous operations (`fs.readFileSync`, `crypto.pbkdf2Sync`, CPU-heavy loops) block the event loop and stall all concurrent requests. Prefer async equivalents for I/O. For CPU-bound work, prefer `worker_threads` or offloading to a separate process.

Event listeners accumulate silently when `.on()` is called repeatedly without corresponding `.off()` or `.removeListener()`. This is the most common source of memory leaks in long-running Node processes. The default MaxListeners warning (11 listeners) exists to catch this; raising the limit to silence the warning is almost always the wrong fix.

The `error` event on streams, sockets, and other `EventEmitter` subclasses is special: if no listener is registered, emitting `error` throws and crashes the process. Always attach an `error` handler to any emitter you create or receive.

`process.env` values are always strings. `process.env.PORT` is `"3000"`, not `3000`. Comparing with `===` against a number silently fails. Parse environment values explicitly at the configuration boundary.
