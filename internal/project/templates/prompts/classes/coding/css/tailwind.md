# Tailwind CSS

When the codebase you're working in has established conventions that differ from what's described here, follow the codebase.

## Utility-First Principles

Prefer composing styles from utility classes directly in markup. This is Tailwind's core design: colocate styling with structure. Resist the urge to extract CSS classes prematurely. A `div` with ten utility classes is normal in Tailwind and reads fine once you are accustomed to the convention.

Extract a component (React component, partial, template include) when you find yourself repeating the same cluster of utility classes across multiple templates. Component extraction is the primary abstraction mechanism, not CSS class extraction.

Prefer `@apply` sparingly. It exists for cases where you must write CSS (third-party component overrides, generated HTML you cannot control). For your own components, extracting a template component is almost always better than extracting an `@apply` class.

## Configuration

### Tailwind v4 (CSS-first, stable since January 2025)

Tailwind v4 moves configuration into CSS with the Oxide engine, a complete Rust rewrite that makes incremental builds over 100x faster (measured in microseconds). Add `@import "tailwindcss"` in your main stylesheet. No JavaScript config file is required. Content detection is automatic; no `content` array needed.

Customize the theme with the `@theme` directive. Values defined in `@theme` become CSS custom properties and generate corresponding utility classes. `--color-brand: oklch(0.65 0.15 250)` in a `@theme` block creates `bg-brand`, `text-brand`, and all related utilities.

Override default variants with `@custom-variant`. Dark mode, for example, uses `prefers-color-scheme` by default. Switch to class-based dark mode with `@custom-variant dark (&:where(.dark, .dark *))`.

v4 is built on cascade layers, registered custom properties via `@property`, and `color-mix()`. It adds built-in container query variants (`@sm`, `@lg`, `@min-*`, `@max-*`) with no plugin required, and 3D transform utilities (`rotate-x-*`, `rotate-y-*`, `scale-z-*`, `translate-z-*`, `perspective-*`).

### Tailwind v3 (JavaScript config)

Configure in `tailwind.config.js`. Extend the default theme under `theme.extend` to add values without losing defaults. Override keys directly under `theme` only when you intend to replace the defaults entirely.

Prefer the `content` array to specify all files that contain class names. Missing paths cause utilities to be purged from the production build. Glob patterns should cover templates, components, and any JavaScript files that construct class names dynamically.

## Dark Mode

In v4, dark mode works via `prefers-color-scheme` with zero configuration. The `dark:` variant is available immediately. For manual toggle support, override with `@custom-variant`.

In v3, set `darkMode: 'class'` in the config for manual control, or `darkMode: 'media'` for system preference. Prefer the class strategy when the application offers a theme toggle.

Prefer defining dark-mode colors as semantic tokens rather than hardcoding color values in every `dark:` utility. Map `--color-surface` to a light value by default and override it inside `.dark` or `@media (prefers-color-scheme: dark)`.

## Responsive Design

Tailwind's breakpoint prefixes (`sm:`, `md:`, `lg:`, `xl:`, `2xl:`) are mobile-first. Unprefixed utilities apply at all screen sizes; prefixed utilities apply at that breakpoint and above.

Prefer designing for mobile first, then layering on wider-screen adjustments. Start with the unprefixed utility and add breakpoint variants for larger viewports.

In v4, container queries are built in with no plugin required. Use `@container` utilities (`@sm:`, `@md:`, `@lg:`, etc.) for components that should respond to their parent's size rather than the viewport.

## Spacing and Sizing

Prefer the default spacing scale for consistency. Arbitrary values (`p-[13px]`) are available but should be rare. If you reach for arbitrary values frequently, extend the theme with the values you need.

Prefer `size-*` (v4) for setting width and height simultaneously on square elements. Prefer `w-full`, `max-w-prose`, `max-w-screen-lg` and similar constrained widths over arbitrary pixel values.

## Typography

Prefer the `@tailwindcss/typography` plugin (or its v4 equivalent) for styling prose content rendered from Markdown or a CMS. The `prose` class applies sensible typographic defaults to `<h1>` through `<p>`, lists, code blocks, and links.

Prefer `text-balance` for headings and `text-wrap` for body copy when the browser supports CSS `text-wrap: balance`.

## Common Pitfalls

Dynamic class construction defeats Tailwind's purge mechanism. `className={"bg-" + color}` will not work because the full class name never appears as a literal string. Prefer mapping values to complete class names: `const colors = { red: "bg-red-500", blue: "bg-blue-500" }`.

Conflicting utilities on the same element can produce unexpected results. `p-4 p-8` applies both padding declarations; the one that appears later in Tailwind's generated stylesheet wins, which is `p-8`. To conditionally apply classes, prefer a utility like `clsx` or `cn` (from `tailwind-merge`) that resolves conflicts explicitly.

Overriding Tailwind styles with custom CSS requires awareness of specificity. Tailwind utilities are low-specificity single-class selectors. A more specific custom selector will override them, but so will another Tailwind utility loaded later. Prefer `@layer` to control where custom CSS sits relative to Tailwind's layers.

Long lists of utility classes can become hard to read. Prefer consistent ordering: layout (display, position), sizing (width, height), spacing (margin, padding), typography (font, text), color (bg, text-color), borders, effects (shadow, opacity), state variants (hover, focus), responsive prefixes. Tools like `prettier-plugin-tailwindcss` enforce a canonical order automatically.
