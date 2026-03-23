# CSS

When the codebase you're working in has established conventions that differ from what's described here, follow the codebase.

## Custom Properties

Prefer CSS custom properties (variables) for design tokens: colors, spacing scales, font sizes, border radii, shadows. Define them on `:root` for global scope or on component selectors for scoped overrides.

Prefer a naming convention that communicates purpose. `--color-surface-primary` is more useful than `--blue-500` because it survives a rebrand. Map semantic tokens to primitive tokens: `--color-surface-primary: var(--blue-500)`.

Prefer custom properties over preprocessor variables when the value needs to change at runtime (theming, dark mode, container-scoped overrides). Preprocessor variables are static after compilation; custom properties participate in the cascade.

## Layout

Prefer CSS Grid for two-dimensional layouts (rows and columns together). Prefer Flexbox for one-dimensional layouts (a row of items or a column of items). Both are production-ready and well-supported. Choose based on the layout's dimensionality, not habit.

Prefer `gap` over margins for spacing between grid and flex children. `gap` is set on the container and applies uniformly without needing to exclude the first or last child.

Prefer `grid-template-areas` for page-level layouts where named regions improve readability. Prefer `grid-template-columns` with `repeat()`, `minmax()`, and `auto-fit`/`auto-fill` for responsive card grids that reflow without media queries.

Do not use `float` for layout. Floats are a typographic tool for wrapping text around images. The clearfix hack is obsolete. If you find float-based layout in legacy code, refactor to Grid or Flexbox when touching that area.

## Container Queries

Prefer container queries (`@container`) over media queries when a component's layout should respond to its container's size rather than the viewport. Container queries make components portable: they adapt correctly whether placed in a sidebar, a main content area, or a modal.

Define containment with `container-type: inline-size` on the parent element. Use `container-name` when multiple nested containers exist and you need to query a specific ancestor.

## Cascade Layers

Prefer `@layer` for managing specificity across large codebases. Layers let you control which styles win without inflating selector specificity. A layer declared earlier in the `@layer` order loses to one declared later, regardless of selector weight.

Prefer a layer order like `@layer reset, base, components, utilities, overrides`. Reset handles normalization, base sets global typography and tokens, components holds UI elements, utilities are single-purpose helpers, overrides handles page-specific adjustments.

Prefer putting third-party CSS into its own layer so your styles always take precedence: `@layer third-party { @import "vendor.css"; }`.

## Logical Properties

Prefer logical properties (`margin-block-start`, `padding-inline-end`, `inline-size`, `block-size`) over physical properties (`margin-top`, `padding-right`, `width`, `height`) when the layout should adapt to different writing modes or text directions. Logical properties are the right default for any project that might be localized to RTL languages.

For projects that will never need RTL support, physical properties are fine. Consistency within the project matters more than dogma.

## Specificity Management

Prefer low-specificity selectors. A single class (`.card-title`) is almost always sufficient. Avoid ID selectors in stylesheets; IDs have high specificity and cannot be overridden without escalation.

Prefer the `:where()` pseudo-class to wrap selectors when you want zero specificity. `:where(.card .title)` matches the same elements as `.card .title` but adds no specificity, making it easy to override.

Prefer `:is()` for grouping selectors that should share specificity. `:is(.card, .panel) .title` is cleaner than duplicating the rule for each parent.

Prefer `!important` only within utility layers where it is the intended mechanism (as in utility-first frameworks). In component styles, `!important` is almost always a sign of a specificity problem that should be solved structurally.

## Naming Conventions

Prefer BEM (Block, Element, Modifier) or a similar structured naming convention when writing vanilla CSS without a framework's scoping. `.card`, `.card__title`, `.card--highlighted` communicates structure without nesting selectors.

Prefer flat selectors over deeply nested descendant selectors. `.card__title` is more resilient than `.card > .header > .title` because it doesn't break when the HTML structure changes.

## Nesting

CSS nesting is natively supported in all modern browsers. Prefer native nesting for grouping related rules under a parent selector. Keep nesting shallow: two levels is comfortable, three is the practical maximum. Deeply nested CSS produces high-specificity selectors and is hard to override.

## Modern Features

Prefer `color-mix()` for deriving color variants from a base color. Prefer `oklch()` or `oklab()` color spaces for perceptually uniform color manipulation.

Prefer `clamp()` for fluid typography and spacing: `font-size: clamp(1rem, 0.5rem + 1.5vw, 1.5rem)` scales smoothly between a minimum and maximum without media queries.

Prefer `aspect-ratio` over the padding-bottom hack for maintaining aspect ratios on elements.

Prefer anchor positioning (`anchor-name`, `position-anchor`, `position-area`) for tooltips, popovers, and dropdowns that need to attach to a trigger element. Anchor positioning handles fallback placement to avoid viewport overflow directly in CSS. All major browsers are converging on support via Interop 2026.

Prefer scroll-driven animations (`animation-timeline: scroll()` or `animation-timeline: view()`) for effects tied to scroll position. They replace JavaScript scroll listeners for parallax, progress bars, and reveal-on-scroll patterns.

Prefer view transitions (`view-transition-name`, the View Transition API) for smooth animated transitions between page states. Same-document transitions are well-supported; cross-document (multi-page) transitions are an Interop 2026 focus area.

Prefer `@starting-style` for entry animations on elements that appear via `display: none` toggling or DOM insertion. Without it, the browser renders the final state on the first frame, causing a flash.

Prefer `@supports` for feature detection when using newer properties that may not be available in all target browsers.

## Common Pitfalls

The `margin` shorthand with four values follows the order top, right, bottom, left (clockwise from the top). Confusing the order is a persistent source of subtle layout bugs.

Collapsing margins between adjacent block-level elements can produce unexpected spacing. The larger margin wins rather than both being applied. Flexbox and Grid containers do not collapse margins, which is one reason layout components behave more predictably with these systems.

`z-index` only works on positioned elements (`position: relative`, `absolute`, `fixed`, or `sticky`) and flex/grid children. Applying `z-index` to a statically positioned element has no effect. Stacking contexts created by `transform`, `opacity`, `filter`, and other properties can also interfere with expected stacking order.

`100vh` on mobile browsers includes the area behind the browser's address bar, causing content to overflow. Prefer `100dvh` (dynamic viewport height) for full-screen layouts on modern browsers.

Percentage-based `padding` and `margin` are always calculated relative to the element's containing block width, even for vertical padding. This surprises developers who expect vertical percentages to relate to height.
