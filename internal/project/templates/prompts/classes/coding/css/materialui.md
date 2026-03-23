# Material UI (MUI)

When the codebase you're working in has established conventions that differ from what's described here, follow the codebase.

## Styling Approaches

### The `sx` Prop

Prefer the `sx` prop for one-off style overrides on a single component instance. It accepts theme-aware CSS properties and shorthand: `sx={{ p: 2 }}` resolves to `padding: 16px` using the theme's 8px spacing unit.

The `sx` prop supports responsive values as objects: `sx={{ width: { xs: '100%', md: '50%' } }}` applies different widths at different breakpoints. It also supports pseudo-classes and nested selectors: `sx={{ '&:hover': { bgcolor: 'action.hover' } }}`.

Prefer theme token paths over raw values in `sx`. Write `color: 'text.secondary'` rather than `color: '#666'`. Token paths ensure consistency with the theme and adapt to dark mode automatically.

### `styled()`

Prefer `styled()` for reusable styled components that appear in multiple places. `styled(Button)({ ... })` creates a new component with baked-in styles that can still accept `sx` for instance-level overrides.

Prefer `styled()` over `sx` when the style logic is complex (multiple conditional variants, responsive breakpoints combined with state-based styles). A styled component encapsulates this complexity behind a clean props interface.

### When to Use Which

Use `sx` for quick, single-instance adjustments. Use `styled()` for reusable components with consistent custom styling. Use theme `styleOverrides` for global component-level defaults. This layering keeps the specificity gradient predictable.

## Theme Customization

Prefer `createTheme()` for defining the application's visual language. Customize the palette, typography, spacing, shape, and breakpoints in one place.

```tsx
const theme = createTheme({
  palette: {
    primary: { main: '#1976d2' },
    secondary: { main: '#dc004e' },
  },
  typography: {
    fontFamily: '"Inter", "Helvetica", "Arial", sans-serif',
    h1: { fontSize: '2.5rem', fontWeight: 600 },
  },
  shape: { borderRadius: 8 },
});
```

Prefer `theme.palette.augmentColor()` to generate light, dark, and contrast text variants from a single main color. Manually specifying all variants is error-prone and breaks accessible contrast ratios.

Prefer nesting themes with `<ThemeProvider>` for sections of the app that need a different visual treatment (a dark sidebar within a light app, for example). MUI resolves the nearest `ThemeProvider` ancestor.

## Component Customization

Prefer theme `styleOverrides` for global changes to a component's appearance. Overrides apply to every instance of the component without modifying individual call sites:

```tsx
const theme = createTheme({
  components: {
    MuiButton: {
      styleOverrides: {
        root: { textTransform: 'none' },
      },
      defaultProps: {
        disableElevation: true,
      },
    },
  },
});
```

Prefer `defaultProps` in the theme for setting default prop values across all instances. Disabling button elevation, setting default TextField variants, or changing default Alert severity belong here.

Prefer `variants` in the theme for adding custom variant options to components. A custom `soft` variant for Button, for example, can be defined once in the theme and used as `<Button variant="soft">`.

## Component Composition

Prefer MUI's `component` and `slots`/`slotProps` props for structural customization. The `component` prop changes the rendered root element: `<Button component={Link}>` renders a link that looks like a button. The `slots` API (v5.10+) lets you replace internal sub-components.

Prefer composing MUI components with `Box`, `Stack`, and `Grid` for layout. `Stack` handles one-dimensional spacing (column or row of items with `spacing`). `Grid` handles two-dimensional responsive layouts. `Box` is a generic container with `sx` support.

Prefer `Stack` with `spacing` over manual margin utilities for vertical or horizontal lists of elements. Stack's `spacing` prop uses the theme's spacing scale and handles the gap uniformly.

## MUI v5/v6 Patterns

Prefer `@mui/material` v5 or v6 over `@material-ui/core` v4. The v4 package uses a different import structure and styling engine (JSS vs. Emotion).

System props (`color`, `bgcolor`, `p`, `m` directly on components) are deprecated in v6 in favor of the `sx` prop. Write `sx={{ p: 2 }}` rather than `p={2}` directly on the component.

MUI v6 introduces Pigment CSS, a zero-runtime CSS-in-JS engine that extracts styles at build time. It is opt-in and experimental as of v6. For new projects, Emotion remains the default and stable styling engine. Evaluate Pigment CSS when your project requires React Server Component compatibility or has strict bundle-size requirements.

Prefer the `@mui/material` import path for all components. Tree shaking works correctly with modern bundlers (Webpack 5, Vite, esbuild). The second-level import pattern (`@mui/material/Button`) is no longer necessary for bundle size but remains valid.

## Common Pitfalls

The `sx` prop creates a new style object on every render. For components in hot loops (table cells, list items rendered hundreds of times), this can cause measurable performance overhead. Prefer `styled()` or theme `styleOverrides` for styles applied to many instances of the same component.

Theme `styleOverrides` are not tree-shaken. Every override for a component is included in the bundle even if that component is never imported. Keep overrides to components you actually use.

MUI components render multiple DOM elements internally (a Button includes a root, a label, and a ripple overlay). Targeting the wrong internal element with `sx` or `styled()` produces no visible effect. Use the browser DevTools to inspect the actual DOM structure and the component's CSS classes documentation to find the right slot.

The `theme.spacing()` function returns a string with `px` units by default: `theme.spacing(2)` returns `'16px'`. When doing arithmetic in JavaScript, use `theme.spacing()` in CSS contexts only. For numeric values, access `theme.spacing` as a number via the theme's raw configuration.

Wrapping MUI components with `styled()` or `React.forwardRef` can lose the component's TypeScript prop types. Use the generic form: `styled(Button)<{ custom?: boolean }>({})` to extend the prop interface. Always forward refs when wrapping, since MUI's internal logic and transition components depend on ref access.
