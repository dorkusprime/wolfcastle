# Vue (JavaScript)

When the codebase you're working in has established conventions that differ from what's described here, follow the codebase.

## Composition API

Prefer `<script setup>` for single-file components. It eliminates the boilerplate of `defineComponent`, `setup()` return objects, and manual component registration. Top-level bindings are automatically available in the template.

Prefer `ref` for primitive values and single objects whose identity may change. Prefer `reactive` for plain objects or collections where you want to access properties directly without `.value`. Do not mix `reactive` with re-assignment; `reactive` tracks the object itself, so replacing it (`state = newObj`) breaks reactivity silently.

Prefer extracting shared stateful logic into composables (functions named `useX` that call Composition API primitives). A composable returns refs or reactive objects, keeping reactivity intact across component boundaries.

## Props and Emits

Prefer object-syntax prop declarations with type constructors: `defineProps({ title: String, count: { type: Number, default: 0 }, items: { type: Array, required: true } })`. This is the JavaScript equivalent of the type-based declarations available in TypeScript. The `type` field accepts a constructor (`String`, `Number`, `Boolean`, `Array`, `Object`, `Function`, `Symbol`) or an array of constructors for union types.

Prefer the `validator` option for props with domain constraints: `{ status: { type: String, validator: v => ['active', 'inactive'].includes(v) } }`. Validators run in development mode and produce console warnings on mismatch.

Prefer array-syntax emit declarations for simple events: `defineEmits(['update', 'delete'])`. For events with payload validation, use object syntax: `defineEmits({ update: (value) => typeof value === 'string' })`. The validator function receives the emitted arguments and returns a boolean.

## Templates and Rendering

Prefer templates over JSX for most components. Templates enable compile-time optimizations (static hoisting, patch flags) that the JSX path cannot apply. Reserve JSX or render functions for highly dynamic components where programmatic vnode construction is clearer than template directives.

Prefer `v-bind` shorthand (`:prop="value"`) for dynamic attributes. A bare attribute without the colon passes a static string, which is a common source of bugs when the value is meant to be reactive.

Prefer `v-for` with a `:key` bound to a stable identifier from the data (database ID, slug). Using the array index as key causes incorrect DOM reuse when items are reordered, inserted, or deleted.

## State Management

Prefer Pinia for shared application state. Define stores with `defineStore` using either the setup syntax (a function returning refs and computed properties) or the options syntax (an object with `state`, `getters`, and `actions`). In JavaScript projects, the options syntax provides slightly better IDE autocompletion because the structure is more predictable without type inference.

Prefer keeping component-local state in `ref`/`reactive` rather than pushing everything into a store. Pinia is for state that crosses component boundaries or persists across route changes.

Prefer structuring stores around features, not data types. `useCheckoutStore` and `useProfileStore` map to application domains; `useUsersStore` and `useOrdersStore` map to database tables and tend to become catch-all dumping grounds.

## Routing

Prefer Vue Router with explicit route definitions using `createRouter` and `createWebHistory` (or `createWebHashHistory` for hash-mode routing). Use `useRoute()` to access the current route and `useRouter()` to navigate programmatically inside `<script setup>`.

Prefer navigation guards (`beforeEach`, `beforeRouteEnter`) for authentication and data-loading gates over ad-hoc checks in component setup. Guards centralize cross-cutting concerns and keep components focused on rendering.

Prefer lazy-loaded routes with dynamic imports: `component: () => import('./views/UserProfile.vue')`. This splits the bundle per route and loads components only when navigated to.

## Testing

Prefer Vue Test Utils with Vitest for component testing. Use `mount` for integration-style tests that exercise child components and `shallowMount` to isolate the component under test from its children.

Prefer `await nextTick()` after triggering state changes before asserting on DOM updates. Vue batches reactive updates asynchronously; assertions without `nextTick` read stale DOM.

Prefer `wrapper.emitted()` to assert on emitted events: `expect(wrapper.emitted('update')?.[0]).toEqual(['newValue'])`. This tests the component's public contract without reaching into internals.

Prefer testing Pinia stores by creating a test Pinia instance with `createTestingPinia({ createSpy: vi.fn })`. This auto-stubs actions and lets you set initial state directly. Import the store hook and call it after `createTestingPinia` is installed.

Without TypeScript, test setup code does not get compile-time feedback on prop shapes or mock structures. Prefer validating that test props match the component's `defineProps` declaration by running the full component render (which triggers prop validation warnings) rather than asserting on internal state.

## Common Pitfalls

Destructuring a `reactive` object extracts plain values, severing reactivity. `const { count } = reactive({ count: 0 })` gives a non-reactive `count`. Prefer `toRefs` to destructure while preserving reactivity: `const { count } = toRefs(state)`.

Refs unwrap automatically in templates (`{{ myRef }}` works without `.value`) but require explicit `.value` access in `<script setup>`. Forgetting `.value` in script produces `undefined` or the ref object itself, not the inner value. Without TypeScript, the editor cannot flag this; the symptom is a `[object Object]` rendering or a silent `undefined`.

Prop type mismatches in JavaScript are caught only at runtime in development mode. Passing a string where a number is expected silently succeeds in production. Prefer unit tests that exercise the component with representative prop combinations to catch type errors before deployment.

`watchEffect` runs immediately and re-runs whenever any reactive dependency accessed inside it changes. `watch` requires explicit sources and does not run until those sources change. Prefer `watch` with explicit sources when you need the previous value or want to control exactly which dependencies trigger the callback. Prefer `watchEffect` for simple synchronization where tracking all accessed dependencies automatically is clearer.

Passing a reactive object to a child via `v-bind` without specifying individual props (`:="state"`) spreads all properties, which can expose internal state and cause unexpected reactivity. Prefer binding named props explicitly.
