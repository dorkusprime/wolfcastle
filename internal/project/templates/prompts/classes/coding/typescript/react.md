# React

When the codebase you're working in has established conventions that differ from what's described here, follow the codebase.

## Components

Prefer function components for all new code. Class components remain valid in existing codebases but offer no advantages for new work since hooks cover the same ground with less ceremony.

Prefer named function declarations (`function UserList() {}`) over arrow expressions for components. Named declarations hoist, show up with readable names in React DevTools and stack traces, and make the component's identity obvious at a glance.

Prefer small, single-responsibility components. When a component manages its own data fetching, transforms the data, and renders a complex layout, split it into a container that owns the data and a presentational component that receives props.

Prefer co-locating closely related components in the same file when they form a logical unit (a list and its list item, a form and its field group). Extract to separate files when a component is reused across multiple parents.

## Props and Typing

Prefer discriminated unions for props that change shape based on a variant. `type ButtonProps = { variant: "link"; href: string } | { variant: "button"; onClick: () => void }` lets the compiler enforce that `href` only appears on link buttons.

Prefer explicit `children: React.ReactNode` in the props type over `React.PropsWithChildren<T>`. It reads more clearly and makes the contract visible in the type definition.

Prefer `ComponentProps<"button">` or `ComponentProps<typeof SomeComponent>` when extending native or third-party element props. This tracks upstream type changes automatically.

In React 19, `ref` is a regular prop on function components. Accept it directly in the props type (`function Input({ ref, ...props }: { ref?: React.Ref<HTMLInputElement> } & InputProps)`) instead of wrapping with `forwardRef`. `forwardRef` is deprecated and will be removed in a future release. Prefer default parameter values in the component signature over `defaultProps`, which is deprecated in React 19 for function components.

## Hooks

Prefer calling hooks at the top level of the component body, never inside conditions, loops, or nested functions. React relies on call order to associate hook state with the component instance.

Prefer `useState` for independent, simple values. Prefer `useReducer` when the next state depends on the previous state in non-trivial ways, or when multiple state values change together in response to a single event.

Prefer `useActionState` for form submission handling in React 19. It manages the pending state, error state, and return value of a Server Action or form action in a single hook. Prefer `useOptimistic` for instant UI feedback while an async action is in flight; it shows a temporary value that reverts automatically on failure.

Prefer the `use` API (React 19) for reading promises and context in render. `use(somePromise)` suspends the component until the promise resolves. Unlike hooks, `use` can be called inside conditions and loops.

Prefer extracting related stateful logic into custom hooks (`useDebounce`, `useMediaQuery`, `usePagination`). A custom hook is a function starting with `use` that calls other hooks. It shares logic without sharing state.

Prefer `useMemo` and `useCallback` only when you have measured a performance problem or need referential stability for a dependency array. The React Compiler (stable since v1.0, October 2025) automates memoization at build time; in projects that use it, manual `useMemo`/`useCallback` is rarely needed.

## Effects and Lifecycle

Prefer keeping effects focused on a single concern: one effect for a subscription, another for a document title update. Combining unrelated side effects into one `useEffect` makes cleanup and dependency tracking harder.

Prefer returning a cleanup function from effects that create subscriptions, event listeners, timers, or abort controllers. Omitting cleanup causes memory leaks on unmount and stale callbacks on re-render.

Prefer avoiding effects for logic that can run during render or in event handlers. Deriving values from props or state belongs in the render body. Responding to user actions belongs in event handlers. Effects are for synchronizing with systems outside React (DOM APIs, network, timers).

Prefer `useEffect` with explicit dependency arrays. An empty array (`[]`) runs once on mount. Omitting the array entirely runs on every render, which is almost never what you want.

## State Management

Prefer `useState`/`useReducer` for state local to a single component or a small subtree. Prefer React Context for values that many components at different depths need (theme, locale, authenticated user) but that change infrequently. Prefer an external store (Zustand, Jotai, Redux Toolkit, TanStack Query for server state) when shared state is complex, updates frequently, or needs to be accessed outside the React tree.

Prefer placing state as close to where it is used as possible. Lifting state to a common ancestor is the right move when siblings need it; lifting to the root "just in case" creates unnecessary re-renders.

## Testing

Prefer React Testing Library for component tests. It renders components into a real DOM and exposes queries that mirror how users find elements: `screen.getByRole`, `screen.getByLabelText`, `screen.getByText`. Avoid `getByTestId` unless the element has no accessible role, label, or text.

Prefer `userEvent` over `fireEvent` for simulating interactions. `userEvent.click` produces the full event sequence (pointerdown, mousedown, pointerup, mouseup, click) that a real browser would, catching bugs that `fireEvent.click` misses.

Prefer wrapping state updates in tests with `act` only when the test framework does not do it automatically. React Testing Library's `render`, `userEvent`, and `waitFor` handle `act` internally. Manual `act` wrapping is needed mainly for direct state updates or custom async flows.

Prefer `waitFor` or `findBy` queries for assertions that depend on async state updates. Avoid arbitrary `setTimeout` delays in tests; they are slow, flaky, and mask timing bugs.

## Common Pitfalls

Stale closures in effects capture the values of state and props from the render in which the effect was created. If an effect reads a value that changes between renders but the dependency array does not include it, the effect sees stale data. The ESLint plugin `react-hooks/exhaustive-deps` catches most of these.

Object and array literals created inline in JSX (`style={{ color: "red" }}`, `options={[1, 2, 3]}`) produce a new reference every render. When passed to a memoized child, they defeat the memoization. Prefer lifting stable objects outside the component or wrapping them in `useMemo` when the child is expensive to re-render. Projects using the React Compiler handle this automatically.

Missing or non-unique `key` props on list items cause React to misidentify which items changed, leading to stale state, broken animations, and incorrect DOM reuse. Prefer stable, unique identifiers from the data (database IDs, slugs) over array indices.

Calling `setState` during render (outside an effect or event handler) triggers an infinite re-render loop. Derived values should be computed directly from props and state, not pushed into separate state via `setState` in the render body.
