# TypeScript

When the codebase you're working in has established conventions that differ from what's described here, follow the codebase.

## Style

Prefer `strict: true` in `tsconfig.json`. Strict mode enables `strictNullChecks`, `noImplicitAny`, `strictFunctionTypes`, and other checks that catch real bugs at compile time. Do not weaken strict settings to make code compile; fix the types instead.

Prefer discriminated unions over type assertions for modeling variants. A union like `type Result = { ok: true; value: T } | { ok: false; error: Error }` lets the compiler narrow exhaustively in `switch` and `if` blocks. An `as` cast tells the compiler to look away.

Prefer `type` over `interface` for data shapes and unions. Use `interface` when you need declaration merging (extending third-party types) or when the codebase convention favors it. Either way, be consistent within a project.

Prefer type-only imports (`import type { Foo } from "./bar"`) when the import is used only in type positions. This makes the erasure boundary explicit and prevents runtime side effects from type-only modules.

Prefer utility types (`Partial<T>`, `Pick<T, K>`, `Omit<T, K>`, `Record<K, V>`, `Readonly<T>`) over hand-rolled mapped types when they express the same transformation. They are well-known, self-documenting, and tested.

Prefer branded types for domain identifiers that share a primitive representation. A `UserId` and an `OrderId` are both strings, but they should not be interchangeable. A brand (`type UserId = string & { readonly __brand: "UserId" }`) prevents accidental mixing at zero runtime cost.

Prefer `unknown` over `any` for values whose type is genuinely uncertain. `unknown` forces you to narrow before use; `any` silently disables type checking for everything it touches.

Prefer `satisfies` to validate a value matches a type without widening it. `const config = { ... } satisfies Config` preserves literal types while still catching structural mismatches.

## Build and Test

Prefer `tsc --noEmit` for type checking as a separate step from bundling. Most modern projects use a bundler (esbuild, swc, Vite) for output and `tsc` purely for type verification.

Prefer `tsx` or `ts-node` for running TypeScript scripts directly. Check which the project uses before introducing the other.

Prefer ESLint with `typescript-eslint` for linting. Look for `eslint.config.*` (flat config) or `.eslintrc.*` (legacy). The `@typescript-eslint/recommended-type-checked` ruleset catches type-aware issues that plain ESLint misses.

Prefer Prettier for formatting. Check for `.prettierrc`, `prettier.config.*`, or a `"prettier"` key in `package.json`. When the project uses ESLint's formatting rules instead, follow that approach.

Prefer the project's existing test runner. Look for Vitest (`vitest.config.*`), Jest with `ts-jest` or `@swc/jest` (`jest.config.*`), or Node's built-in test runner with a TypeScript loader. When starting fresh, Vitest handles TypeScript natively without separate transform configuration.

## Testing

Prefer `describe`/`it` blocks organized by function or behavior. Name each test so the concatenated `describe > it` string reads as a sentence: `describe("parseConfig", () => { it("throws on missing required fields", ...) })`.

Prefer testing behavior over implementation. Assert on return values, side effects, and thrown errors rather than on which internal functions were called. Tests coupled to internals break on refactor.

Prefer keeping `any` out of test code. Test types should be as precise as production types. An `as any` in a test setup hides the exact class of bug the test exists to catch.

Prefer type-level tests for utility types and complex generics. Use `expectTypeOf` (Vitest) or the `tsd` library to assert that types resolve as expected. A type that compiles today can regress silently when its dependencies change.

Prefer mocking at the boundary: HTTP clients, timers, filesystem access. Use the test framework's built-in mocking (`vi.fn()`, `jest.fn()`) and restore mocks in `afterEach` to prevent test pollution.

## Common Pitfalls

`as` casts suppress type errors without changing runtime behavior. Code like `const x = value as number` compiles even if `value` is a string. Prefer type guards (`typeof`, `instanceof`, discriminant checks) that the compiler can verify, and reserve `as` for the rare cases where you genuinely know more than the type system.

Prefer string literal unions over `enum`. Numeric enums reverse-map (`MyEnum[0] === "A"`), which surprises code that iterates values. String enums avoid this but add ceremony for no benefit over a union type. `type Direction = "north" | "south" | "east" | "west"` is simpler, tree-shakes better, and works with `satisfies`.

Declaration merging lets multiple `interface` blocks with the same name combine silently. This is useful for augmenting third-party types but hazardous when two files accidentally define the same interface name, as both declarations merge without error. Prefer `type` when you do not need merging.

Index signatures (`[key: string]: T`) make every property access on the object return `T`, including typos. Prefer `Record<K, V>` with a known key union, or use `Map<K, V>` when keys are truly dynamic. Enable `noUncheckedIndexedAccess` in `tsconfig.json` to make index access return `T | undefined`.

Overusing generics creates types that are harder to read than the problem they solve. A function with four type parameters and conditional types is often clearer as two or three concrete overloads. Prefer generics when the type relationship is real and simple; prefer overloads or separate functions when the abstraction is strained.

`!` (non-null assertion) silently tells the compiler a value is not `null` or `undefined`. Like `as`, it is an escape hatch that hides bugs. Prefer narrowing with an explicit check or an early return. Reserve `!` for cases where the assertion is provably correct and a check would be misleading noise.
