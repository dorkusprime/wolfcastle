# Bootstrap

When the codebase you're working in has established conventions that differ from what's described here, follow the codebase.

## Grid System

Prefer the Bootstrap 5 grid (`container`, `row`, `col-*`) for page-level layouts. The current stable release is Bootstrap 5.3.x (5.3.8 as of late 2025). Bootstrap 5.4 is planned as the last feature release of v5, and Bootstrap 6 has no announced release date. The grid uses Flexbox under the hood and supports 12 columns by default with responsive breakpoints (`col-sm-6`, `col-lg-4`).

Prefer `row-cols-*` for uniform column widths in card grids: `row-cols-1 row-cols-md-2 row-cols-lg-3` is cleaner than putting `col-md-6 col-lg-4` on every child.

Prefer `g-*` gutter utilities for controlling row and column gaps. `gx-*` controls horizontal gutters, `gy-*` controls vertical gutters, `g-*` controls both. The default gutter is `1.5rem`.

Prefer `.container-fluid` for full-width layouts. Prefer `.container` or `.container-{breakpoint}` for max-width constrained layouts. Do not nest containers.

## Utility Classes

Bootstrap 5 ships utility classes for spacing (`m-*`, `p-*`), display (`d-flex`, `d-none`), text (`text-center`, `fw-bold`), color (`text-primary`, `bg-light`), and more.

Prefer utilities over custom CSS for one-off styling adjustments. `class="mb-3 text-center"` is easier to maintain than a single-use CSS class with `margin-bottom: 1rem; text-align: center`.

Prefer responsive utility variants (`d-none d-md-block`) over custom media queries for show/hide patterns. Bootstrap generates responsive variants for most utility classes.

## Customization with Sass

Prefer overriding Sass variables before importing Bootstrap. Bootstrap's `_variables.scss` provides defaults for colors, spacing, typography, breakpoints, and component styles. Override what you need, then import Bootstrap:

```scss
// your overrides
$primary: #0d6efd;
$border-radius: 0.5rem;
$spacer: 1rem;

// then import Bootstrap
@use "bootstrap/scss/bootstrap";
```

Prefer the Sass variable layer over CSS overrides for structural changes (grid columns, breakpoints, spacing scale). CSS overrides work for colors and visual tweaks via Bootstrap's CSS custom properties (available since 5.2), but changing the spacing scale or grid structure requires Sass recompilation.

Prefer Bootstrap's CSS custom properties (e.g., `--bs-primary`, `--bs-body-font-size`) for runtime theming when the project uses Bootstrap 5.2+. These properties allow color and typography changes without recompiling Sass.

## Utility API

Prefer the Utility API to add, modify, or remove utility classes when Bootstrap's defaults are insufficient. The API uses a Sass map (`$utilities`) to define every utility class Bootstrap generates.

Add custom utilities with `map-merge`. Remove unused utilities with `map-remove` to reduce CSS output size. Modify existing utilities by merging new values into their map entry.

Prefer generating responsive variants only for utilities that need them. Setting `responsive: true` on every custom utility inflates the stylesheet.

## Components

Prefer Bootstrap's built-in components (modals, dropdowns, toasts, accordions) with their data attributes API (`data-bs-toggle`, `data-bs-target`) for standard interactions. The data attributes approach requires no custom JavaScript.

Prefer the JavaScript API (`new bootstrap.Modal(element)`) when you need programmatic control: showing a modal in response to an async event, or disposing a tooltip after navigation.

Prefer `btn btn-primary` and similar component classes over custom button styles. Customize button appearance through Sass variables (`$btn-border-radius`, `$btn-padding-y`) rather than overriding `.btn` styles directly.

## Forms

Prefer Bootstrap's form classes (`.form-control`, `.form-select`, `.form-check`) for consistent form styling. These classes handle cross-browser normalization and provide accessible focus states.

Prefer `.form-floating` for floating label inputs. They require the `<input>` to appear before the `<label>` in the HTML, which is the opposite of the non-floating pattern.

Prefer Bootstrap's validation classes (`.is-valid`, `.is-invalid`) with `.valid-feedback` and `.invalid-feedback` for form validation UI. Add `novalidate` to the `<form>` element when using custom validation to prevent browser-native validation tooltips.

## Common Pitfalls

Importing all of Bootstrap when you need a few components wastes bytes. Prefer importing individual Sass files (`@use "bootstrap/scss/grid"`, `@use "bootstrap/scss/utilities"`) for smaller builds. This requires also importing Bootstrap's functions, variables, and mixins as prerequisites.

Bootstrap's JavaScript components use a specific DOM structure. Modifying the expected HTML hierarchy (moving the `.modal-dialog` outside `.modal`, for example) breaks the component silently. Check the documentation for the expected markup before customizing.

Bootstrap's spacing scale is based on `$spacer` (default `1rem`) with multipliers: 0 is `0`, 1 is `$spacer * .25`, 2 is `$spacer * .5`, 3 is `$spacer`, 4 is `$spacer * 1.5`, 5 is `$spacer * 3`. The jump from 4 to 5 (1.5rem to 3rem) surprises developers who expect a linear scale. Add intermediate values via the `$spacers` map if needed.

Mixing Bootstrap's grid with other layout systems (CSS Grid, Flexbox applied directly to `.row`) can produce conflicts. Bootstrap's `.row` is already a flex container with negative margins. Apply additional layout properties to a wrapper element instead of the `.row` itself.

`.container` applies horizontal padding. Nesting a `.container` inside another `.container` doubles the padding. Use a single container at the page level and avoid nesting them.
