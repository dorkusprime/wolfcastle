# JavaScript

When the codebase you're working in has established conventions that differ from what's described here, follow the codebase.

## Style

Prefer `const` for all bindings. Use `let` only when reassignment is genuinely needed. Never use `var`; its function-scoping and hoisting create subtle bugs that block-scoped declarations avoid entirely.

Prefer destructuring for extracting properties from objects and arrays. `const { name, age } = user` is clearer than assigning each field on its own line, and it keeps the shape of the data visible at the call site.

Prefer template literals over string concatenation. `` `Hello, ${name}` `` reads better than `"Hello, " + name` and avoids type-coercion surprises when non-string values sneak in.

Prefer `async`/`await` over raw `.then()` chains. Awaited code reads top-to-bottom like synchronous code, and `try`/`catch` around `await` is easier to reason about than `.catch()` callbacks interleaved with `.then()`. Always `await` or return promises that matter; a floating promise swallows its rejection silently.

Prefer ES modules (`import`/`export`) over CommonJS (`require`/`module.exports`) when the project supports them. Check `"type": "module"` in `package.json` or the presence of `.mjs` files. When the project uses CommonJS, follow it.

Prefer arrow functions for callbacks and short expressions. Use `function` declarations for top-level named functions where hoisting or `this` binding to the caller is intentional.

Prefer early returns to reduce nesting. A guard clause that returns or throws on invalid input keeps the main logic at the left margin.

Prefer `===` and `!==` over `==` and `!=`. Strict equality avoids the abstract equality algorithm's implicit type coercion, where values like `0 == ""` and `null == undefined` silently evaluate to `true`.

## Build and Test

Prefer the project's existing package manager. Look for `package-lock.json` (npm), `pnpm-lock.yaml` (pnpm), or `yarn.lock` (yarn). Do not introduce a different lockfile into a project that already has one.

Prefer ESLint for linting. Check for `eslint.config.js` (flat config, current default) or `.eslintrc.*` (legacy format). When both exist, the flat config takes precedence. Run the linter before committing; most projects include a `lint` script in `package.json`.

Prefer Prettier for formatting. Check for `.prettierrc`, `prettier.config.js`, or a `"prettier"` key in `package.json`. When the project uses ESLint's formatting rules instead of Prettier, follow that approach.

Prefer the project's existing test runner. Look for configuration for Vitest (`vitest.config.*`), Jest (`jest.config.*` or `"jest"` in `package.json`), or Node's built-in test runner (`node --test`). When starting fresh, Vitest is the current standard for ESM projects; Jest remains common in older and CJS codebases.

## Testing

Prefer `describe`/`it` blocks for organizing tests. Group by function or behavior, not by implementation detail. Name each test so that the concatenated `describe > it` string reads as a sentence: `describe("parseUrl", () => { it("returns null for empty strings", ...) })`.

Prefer testing behavior over implementation. Assert on return values, side effects, and thrown errors rather than on which internal functions were called. Tests coupled to internals break on every refactor.

Prefer mocking at the boundary: HTTP clients, timers, filesystem access, environment variables. Use the test framework's built-in mocking (`vi.fn()`, `jest.fn()`, `mock.fn()`) rather than installing separate libraries when possible. Restore mocks in `afterEach` to prevent test pollution.

Prefer `fake-timers` (via `vi.useFakeTimers()` or `jest.useFakeTimers()`) for testing time-dependent code rather than introducing real `setTimeout` delays into tests.

Prefer snapshot tests sparingly and only for stable output like serialized HTML or CLI output. Snapshots of large objects become rubber-stamped diffs that reviewers ignore.

## Common Pitfalls

`this` inside a regular function depends on how the function is called, not where it's defined. Passing `obj.method` as a callback loses the receiver. Prefer arrow functions for callbacks that need the enclosing `this`, or bind explicitly.

Floating promises silently swallow rejections. Every `async` call in a non-async context must have a `.catch()` handler or be passed to a function that handles the error. In test runners, an unhandled rejection may silently pass the test.

`typeof null === "object"` is a language-level quirk. When checking for objects, guard against null first: `value !== null && typeof value === "object"`.

`for...in` iterates over enumerable properties including inherited ones from the prototype chain. Prefer `for...of` with `Object.keys()`, `Object.values()`, or `Object.entries()` for plain object iteration. Use `for...of` directly on arrays and iterables.

`parseInt("08")` works correctly in modern engines, but `parseInt` without a radix is a code-smell. Prefer `Number()` for full-string conversion or `parseInt(str, 10)` when you specifically need integer parsing with a known base.

Callback-style error handling (`if (err) return callback(err)`) and promise-style error handling (`try`/`catch` around `await`) must never be mixed in the same flow. Pick one pattern and carry it through.

Array methods like `.sort()` mutate the original array. When the caller expects the original to remain unchanged, copy first with `.slice()` or the spread operator before sorting.
