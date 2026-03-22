# React

When the codebase you're working in has established conventions that differ from what's described here, follow the codebase.

## Components

Prefer function components for all new code. Prefer named function declarations (`function UserList() {}`) over arrow expressions; they hoist, produce readable DevTools names, and make identity obvious at a glance.

Prefer small, single-responsibility components. Co-locate closely related components in the same file when they form a logical unit; extract to separate files when reused across multiple parents.

## Props and Runtime Type Checking

Prefer PropTypes for documenting and validating component props at development time. Define `ComponentName.propTypes` immediately after the component declaration so the contract is visible alongside the implementation. Pair with `ComponentName.defaultProps` for optional props that need fallback values.

Prefer JSDoc `@param` annotations on the component function to give editors type information for autocompletion and inline documentation. `/** @param {{ name: string, count: number }} props */` provides IDE support without a build step.

Prefer `PropTypes.shape({})` over `PropTypes.object` for object props. A bare `object` validation catches nothing; `shape` documents the expected structure and warns on missing fields. Use `PropTypes.exact({})` when extra properties should be flagged.

Prefer `PropTypes.oneOfType` for props that change shape based on a variant. For example, a button that is either a link or a click handler: `PropTypes.oneOfType([PropTypes.shape({ variant: ..., href: ... }), PropTypes.shape({ variant: ..., onClick: ... })])`. This is the closest JavaScript equivalent to discriminated unions.

Prefer `PropTypes.isRequired` on every prop that has no default value. Without a type system, missing a required prop produces no error at build time; PropTypes warnings in the console are the only safety net.

## Hooks

Prefer calling hooks at the top level of the component body, never inside conditions, loops, or nested functions. React relies on call order to associate hook state with the component instance.

Prefer `useState` for independent, simple values. Prefer `useReducer` when the next state depends on the previous state in non-trivial ways, or when multiple state values change together in response to a single event.

Prefer extracting related stateful logic into custom hooks (`useDebounce`, `useMediaQuery`, `usePagination`). A custom hook is a function starting with `use` that calls other hooks. It shares logic without sharing state.

Prefer `useMemo` and `useCallback` only when you have measured a performance problem or need referential stability for a dependency array. Memoizing everything adds complexity and memory overhead for no benefit when the computation is cheap.

## Effects and Lifecycle

Prefer keeping effects focused on a single concern. Prefer returning a cleanup function from effects that create subscriptions, event listeners, timers, or abort controllers. Prefer avoiding effects for logic that can run during render or in event handlers; deriving values from props or state belongs in the render body, and responding to user actions belongs in event handlers.

## State Management

Prefer `useState`/`useReducer` for state local to a single component or a small subtree. Prefer React Context for values that many components at different depths need (theme, locale, authenticated user) but that change infrequently. Prefer an external store (Zustand, Jotai, Redux Toolkit, TanStack Query for server state) when shared state is complex, updates frequently, or needs to be accessed outside the React tree.

## Testing

Prefer React Testing Library for component tests. Use queries that mirror how users find elements: `screen.getByRole`, `screen.getByLabelText`, `screen.getByText`. Avoid `getByTestId` unless the element has no accessible role, label, or text.

Prefer `userEvent` over `fireEvent` for simulating interactions. `userEvent.click` produces the full event sequence a real browser would, catching bugs that `fireEvent.click` misses.

Prefer `waitFor` or `findBy` queries for assertions that depend on async state updates. Avoid arbitrary `setTimeout` delays in tests.

## Common Pitfalls

PropTypes only run in development mode and are stripped from production builds. They catch type mismatches during manual testing, not in production. Errors like passing a string where a number is expected, or omitting a required prop, will silently succeed in production unless caught earlier by tests.

Stale closures in effects capture the values of state and props from the render in which the effect was created. If an effect reads a value that changes between renders but the dependency array does not include it, the effect sees stale data. The ESLint plugin `react-hooks/exhaustive-deps` catches most of these; without TypeScript's compile-time checks, this plugin is the primary guard.

Missing `defaultProps` for optional props can produce `undefined` values deep in the rendering tree. Unlike TypeScript where the type system flags nullable access, JavaScript silently propagates `undefined` until it causes a runtime error far from the source. Define defaults for every optional prop.

Object and array literals created inline in JSX (`style={{ color: "red" }}`, `options={[1, 2, 3]}`) produce a new reference every render. When passed to a memoized child, they defeat the memoization. Prefer lifting stable objects outside the component or wrapping them in `useMemo` when the child is expensive to re-render.

Missing or non-unique `key` props on list items cause React to misidentify which items changed, leading to stale state, broken animations, and incorrect DOM reuse. Prefer stable, unique identifiers from the data (database IDs, slugs) over array indices.
