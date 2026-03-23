# SCSS / Sass

When the codebase you're working in has established conventions that differ from what's described here, follow the codebase.

## Module System

Prefer `@use` and `@forward` over `@import`. The `@import` rule is deprecated as of Dart Sass 1.80.0 and will be removed in Dart Sass 3.0.0. New code should never use `@import`.

`@use` loads a module and makes its members available under a namespace: `@use 'variables' as vars` gives you `vars.$primary-color`. Prefer explicit namespaces over `@use 'variables' as *` to keep the origin of each member clear. Use `as *` sparingly and only for modules used so pervasively that the namespace becomes noise (a design-tokens file, for example).

`@forward` re-exports a module's members through the current file. Use it in index files (`_index.scss`) to create a public API for a directory. Members not forwarded are effectively private to the directory.

Prefer the Sass migrator (`sass-migrator module`) to convert existing `@import`-based codebases to the module system. Manual conversion is error-prone in large projects.

## Variables and Naming

Prefer `$kebab-case` for variable names. Prefix variables with the component or system they belong to: `$card-padding`, `$grid-gutter`, `$color-brand-primary`. Avoid generic names like `$padding` or `$blue` at the global level.

Prefer a dedicated `_tokens.scss` or `_variables.scss` partial for design tokens. Import it with `@use` where needed rather than relying on global scope.

Prefer CSS custom properties for values that change at runtime (themes, dark mode, responsive adjustments). Use Sass variables for values that are compile-time constants (breakpoint maps, mixin parameters, computed values).

## Nesting

Prefer a maximum nesting depth of 3 levels. Deep nesting produces long, high-specificity selectors that are brittle and hard to override. If you find yourself nesting deeper, extract a new class.

Prefer the `&` parent selector for pseudo-classes (`&:hover`), pseudo-elements (`&::before`), and modifier classes (`&--active`). This is the primary legitimate use of nesting. Avoid `&` for constructing class names across nesting levels (`&__element` inside a block) when it obscures the full class name; being able to search for `.card__title` in the codebase is more valuable than saving a few characters.

## Mixins, Functions, and Extends

Prefer mixins for reusable groups of declarations that may accept arguments. Media query wrappers, responsive typography, and component variants are good mixin candidates.

Prefer `@function` for computations that return a single value: unit conversions, color manipulation, spacing scale lookups. Functions should be pure (no side effects, no `@content` blocks).

Avoid `@extend` in most cases. It rewrites selectors in ways that are difficult to predict and debug, produces unexpected selector groupings in the output, and does not work across media queries. Prefer mixins or plain class composition in the HTML instead.

## File Organization

Prefer organizing partials into directories by concern. A common structure:

```
scss/
  _tokens.scss          # design tokens (colors, spacing, typography)
  _base.scss            # resets, element styles, global typography
  _mixins.scss          # shared mixins and functions
  components/
    _card.scss
    _button.scss
    _nav.scss
  layouts/
    _grid.scss
    _header.scss
    _footer.scss
  utilities/
    _spacing.scss
    _visibility.scss
  main.scss             # entry point, @use everything
```

The 7-1 pattern (7 directories, 1 main file) is a well-known variant of this approach. Use whatever directory structure fits the project's scale. Small projects need fewer directories; large design systems need more.

Prefer one component per partial file. Prefer `_index.scss` files in directories to `@forward` the directory's partials, giving consumers a single import point.

## Build and Tooling

Prefer Dart Sass (`sass` on npm, the `sass` package). LibSass and Node Sass are deprecated and do not receive new features or bug fixes.

Prefer `sass --style=compressed` for production builds. Prefer source maps in development so browser DevTools show the original SCSS file and line number.

Prefer `stylelint` with `stylelint-config-standard-scss` for linting. It catches nesting depth violations, unknown at-rules, duplicate selectors, and style inconsistencies.

## Common Pitfalls

Sass's `@use` loads each module only once. If two files `@use` the same module, they share the same instance. Configuring a module with `@use 'module' with ($var: value)` only works the first time the module is loaded. Later `@use` statements for the same module ignore the `with` clause silently.

Nesting media queries inside selectors compiles to duplicated media query blocks in the output, one per selector. This increases file size but does not affect behavior. In performance-critical stylesheets, prefer grouping media queries at a higher level or using a post-processing tool to merge them.

Color functions like `darken()`, `lighten()`, `saturate()`, and `desaturate()` operate in HSL space, which is not perceptually uniform. A 10% lightening of two different hues produces visually inconsistent results. For perceptually uniform color manipulation, prefer `color.adjust()` with `$space: oklch` (Dart Sass 1.62+) or defer to CSS `color-mix()` in oklch space.

Interpolation (`#{$var}`) in `calc()` expressions is no longer necessary in modern Sass. Write `calc(100% - $sidebar-width)` directly. The interpolation syntax was required in older versions and persists in many tutorials, but it obscures the expression unnecessarily.
