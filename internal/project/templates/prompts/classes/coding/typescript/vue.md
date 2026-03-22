# Vue

When the codebase you're working in has established conventions that differ from what's described here, follow the codebase.

## Composition API

Prefer `<script setup lang="ts">` for single-file components. It eliminates the boilerplate of `defineComponent`, `setup()` return objects, and manual component registration. Top-level bindings are automatically available in the template.

Prefer `ref` for primitive values and single objects whose identity may change. Prefer `reactive` for plain objects or collections where you want to access properties directly without `.value`. Do not mix `reactive` with re-assignment; `reactive` tracks the object itself, so replacing it (`state = newObj`) breaks reactivity silently.

Prefer extracting shared stateful logic into composables (functions named `useX` that call Composition API primitives). A composable returns refs or reactive objects, keeping reactivity intact across component boundaries.

## Props and Emits

Prefer type-based declarations for props and emits. `defineProps<{ title: string; count?: number }>()` derives runtime validation from the TypeScript type. `defineEmits<{ (e: "update", value: string): void }>()` gives type-safe emit calls without a redundant runtime array.

Prefer `withDefaults` when default values are needed for optional props: `withDefaults(defineProps<Props>(), { count: 0 })`. Default values for object or array props must use factory functions to avoid shared references.

## Templates and Rendering

Prefer templates over JSX for most components. Templates enable compile-time optimizations (static hoisting, patch flags) that the JSX path cannot apply. Reserve JSX or render functions for highly dynamic components where programmatic vnode construction is clearer than template directives.

Prefer `v-bind` shorthand (`:prop="value"`) for dynamic attributes. A bare attribute without the colon passes a static string, which is a common source of bugs when the value is meant to be reactive.

## State Management

Prefer Pinia for shared application state. Define stores with `defineStore` using the setup syntax (a function returning refs and computed properties) for full TypeScript inference. Option stores work but require manual type annotations for getters and actions.

Prefer keeping component-local state in `ref`/`reactive` rather than pushing everything into a store. Pinia is for state that crosses component boundaries or persists across route changes.

## Routing

Prefer typed route definitions with Vue Router. Use `RouteRecordRaw` for route configuration and access params through `useRoute()` in components. Prefer navigation guards (`beforeEach`, `beforeRouteEnter`) for authentication and data-loading gates over ad-hoc checks in component setup.

## Testing

Prefer Vue Test Utils with Vitest for component testing. Use `mount` for integration-style tests that exercise child components and `shallowMount` when you want to isolate the component under test from its children.

Prefer `await nextTick()` after triggering state changes before asserting on DOM updates. Vue batches reactive updates asynchronously; assertions without `nextTick` read stale DOM.

Prefer `wrapper.emitted()` to assert on emitted events: `expect(wrapper.emitted("update")?.[0]).toEqual(["newValue"])`. This tests the component's public contract without reaching into internals.

Prefer `wrapper.find` with CSS selectors or `findComponent` for locating elements. Use `wrapper.get` when the element must exist (throws on miss) and `wrapper.find` when absence is a valid outcome.

## Common Pitfalls

Destructuring a `reactive` object extracts plain values, severing reactivity. `const { count } = reactive({ count: 0 })` gives a non-reactive `count`. Prefer `toRefs` to destructure while preserving reactivity: `const { count } = toRefs(state)`.

Refs unwrap automatically in templates (`{{ myRef }}` works without `.value`) but require explicit `.value` access in `<script setup>`. Forgetting `.value` in script produces `undefined` or the ref object itself, not the inner value.

`watchEffect` runs immediately and re-runs whenever any reactive dependency accessed inside it changes. `watch` requires explicit sources and does not run until those sources change. Prefer `watch` with explicit sources when you need the previous value or want to control exactly which dependencies trigger the callback. Prefer `watchEffect` for simple synchronization where tracking all accessed dependencies automatically is clearer.

Passing a reactive object to a child via `v-bind` without specifying individual props (`:="state"`) spreads all properties, which can expose internal state and cause unexpected reactivity. Prefer binding named props explicitly.
