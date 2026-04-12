# Wolfcastle TUI Design System

## Brand Foundation

The logo is a neon wireframe wolf superimposed on a dark castle. 1980s arcade cabinet energy: black background, geometry traced in electric cyan, magenta accents at the structural joints, yellow-gold teeth and nameplate. The neon-wolf variant adds blue-purple interior glow and orange heat at the jaw.

The TUI should feel like the logo: dark, clean, glowing where it matters. Not a rave. A machine room at 2 AM with one monitor still on.

## Color Palette

### Structural Colors (brand identity)

These define the TUI's visual identity. Used for chrome, borders, headers, and interactive highlights.

| Name | ANSI 256 | Hex | Usage |
|------|----------|-----|-------|
| Neon Cyan | 51 | #00FFFF | Primary brand color. Header background, active borders, focused-pane indicator |
| Deep Cyan | 30 | #008787 | Dimmed variant of primary. Inactive borders, secondary chrome |
| Magenta | 198 | #FF0087 | Accent. Active tab indicator, selection highlight, search match |
| Deep Magenta | 125 | #AF005F | Dimmed magenta. Search ancestor path, secondary accent |
| Gold | 220 | #FFD700 | Tertiary accent. Target mark, daemon "hunting" indicator, emphasis |
| Charcoal | 234 | #1C1C1C | Dark fill. Header bar background, toast background |
| Slate | 236 | #303030 | Neutral dark. Alt-row shading, divider lines |

### Status Colors (functional, universal)

These communicate state. Never use them for decoration. They mean what they mean everywhere.

| Name | ANSI 256 | Hex | Meaning |
|------|----------|-----|---------|
| Green | 2 | #00FF00 | Complete. Task done, audit passed, daemon confirmed |
| Yellow | 3 | #FFFF00 | In progress. Active work, pending, yielded |
| Red | 1 | #FF0000 | Blocked. Failed, error, hard stop |
| Dim | 245 | #8A8A8A | Not started. Inactive, placeholder, secondary text |

### Text Colors

| Name | ANSI 256 | Hex | Usage |
|------|----------|-----|-------|
| Bright | 15 | #FFFFFF | Primary text. Headings, active content |
| Normal | 252 | #D0D0D0 | Body text. Tree labels, detail content |
| Muted | 245 | #8A8A8A | Secondary text. Timestamps, hints, footer |
| Faint | 240 | #585858 | Tertiary text. Debug-level logs, disabled items |

### Background Colors

| Name | ANSI 256 | Hex | Usage |
|------|----------|-----|-------|
| Base | terminal default | - | Main content area. Respects user's terminal background |
| Header | 234 | #1C1C1C | Header bar background (charcoal, barely off-black) |
| Selection | 23 | #005F5F | Selected row in tree, cursor highlight (dark teal) |
| Modal | 235 | #262626 | Modal overlay fill |
| Error | 52 | #340000 | Error bar background |

## Component Specifications

### Header Bar (3 lines max)

```
[deep blue bg, neon cyan text]
WOLFCASTLE v0.6.2                    /path/to/project hunting (PID 12345)
22 nodes: 4● 3◐ 5◯ 10☢                                          1 running
[wc-tui-test ●] [my-saas-app] [+]
```

- Background: Charcoal (234)
- Title "WOLFCASTLE": Neon Cyan (51), bold
- Version, path, status: Bright (15)
- Node count glyphs: status colors
- Tab bar: active tab in Neon Cyan, inactive in Muted, running dot in Green
- `[+]` hint in Muted

### Tree Panel

```
[selection bg when selected, default bg otherwise]
▾ warzone ◐                    (3 tasks)
  ▾ backend ◐
    ● api
  ▶ ◐ auth                    ← target mark in Gold
    ◯ database
```

- Normal text: Normal (252)
- Selected row: Selection bg (23) with Bright text (15)
- Status glyphs: status colors, consistent everywhere
- Target mark (▶): Gold (220)
- Expand/collapse (▾/▸): Muted (245)
- Task hint counts: Muted (245)
- Search match: Magenta (198) bg with Bright text
- Search ancestor: Deep Magenta (125) fg, no bg change

### Focus Indicator

The focused pane gets a left border in Neon Cyan (51). The unfocused pane gets a left border in Slate (236). This replaces the current approach of using the header-red as the selected-row color.

### Detail Panel

- Heading: Bright (15), bold
- Body text: Normal (252)
- Labels ("Status:", "Tasks:", "Scope:"): Muted (245)
- Values: Normal or status-colored as appropriate
- Breadcrumb timestamps: Muted (245)
- Links/addresses: Neon Cyan (51)

### Footer Bar

```
[no background, muted text]
[q] quit  [Tab] focus  [d] dashboard  [s] start  [<>] tab  [+] new tab  ...
```

- All text: Muted (245)
- Keys in brackets: same color, no special highlight
- Truncates from the right when terminal is narrow

### Log Modal

```
[modal bg: 235, content bg: filled via canvas]
TRANSMISSIONS  Level: all (unfiltered)  Trace: all  [following]
10:54:41 [exec-0482] [tool: Read]
10:54:42 [exec-0482] [tool result]
10:54:43 [plan-0012] Planning started...
```

- Header: Bright (15), bold. Filter labels in Muted, active filter in Gold
- Timestamps: Muted (245)
- Trace prefix: Neon Cyan (51)
- Content: Normal (252) with level-based tinting:
  - Debug: Faint (240)
  - Info: no tint
  - Warn: Yellow (3)
  - Error: Red (1)
- Follow indicator: Green for `[following]`, Yellow for `[paused]`

### Inbox Modal

- Item bullets: status-colored (● filed, ○ new)
- Item text: Normal (252)
- Filed date: Muted (245)
- Input bar: Neon Cyan border when active

### Daemon Confirm Modal

- Title ("STOP DAEMON"): Bright, bold
- Body: Normal
- `[Enter] Confirm`: Gold (220)
- `[Esc] Cancel`: Muted (245)

### Tab Picker Modal

- "NEW TAB" title: Bright, bold
- Directory path: Muted
- Running sessions (●): Green + Neon Cyan text
- Directories with .wolfcastle (◆): Gold
- Plain directories: Muted
- Selected row: Selection bg (23) with Bright text

### Help Overlay

- Section titles: Neon Cyan (51), bold
- Key column: Gold (220)
- Description column: Normal (252)
- Scrollable, same border treatment as other modals

### Notification Toasts

- Background: Charcoal (234)
- Text: Bright (15)
- Border: Neon Cyan (51)
- Positioned top-right, auto-dismiss

## Principles

1. **Dark by default.** The terminal background is the canvas. The TUI adds color, it doesn't replace the surface.

2. **Status colors are sacred.** Green, yellow, red, and dim gray mean one thing each. They're never used for decoration or branding.

3. **Neon cyan is the brand.** It appears in the header, focused borders, trace prefixes, links, and section titles. It's the "Wolfcastle is here" signal.

4. **Magenta for interaction.** Search highlights, active selections, things the user is doing right now.

5. **Gold for targets.** What the daemon is working on, what the user should confirm, what demands attention.

6. **Less is more.** A wall of color is noise. Most text is Normal (252) on the default background. Color draws the eye to the few things that matter.

7. **No light mode.** Wolfcastle runs at 2 AM. The design assumes a dark terminal. A near-black base background (ANSI 234) fills the alt-screen so the TUI remains readable on light terminals.

8. **Terminal.app has known limitations.** macOS Terminal.app lacks true color support and has BCE (Background Color Erase) rendering quirks that produce faint 1-pixel stripes at border/content boundaries. This is a Terminal.app limitation, not a bug. Use iTerm2, Ghostty, Kitty, or Alacritty for the intended visual experience.

## Migration from Current Palette

| Current | New | Rationale |
|---------|-----|-----------|
| Header bg: Dark Red (52) | Charcoal (234) | Matches logo's blue-purple depth; red is reserved for errors |
| Selection bg: Dark Red (52) | Selection Teal (23) | Red selection conflicts with blocked-status red |
| Search highlight: Yellow (3) | Magenta (198) | Yellow is for in-progress status, not search |
| Search ancestor: Dark Olive (58) | Deep Magenta (125) | Olive doesn't exist in the brand palette |
| Target mark: Bright Yellow (11) | Gold (220) | Richer, more intentional than raw bright yellow |
| Trace prefix: Cyan (6) | Neon Cyan (51) | Brighter, matches brand primary |
| Modal title bg: none | Charcoal (234) | Consistent with header |
