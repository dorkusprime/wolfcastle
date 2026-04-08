# TUI Phase 1: App Shell

Read `common.md` first for layout, dependencies, entry states, and shared reference sections.

## Phase 1: App Shell

Theme: get the frame on screen. Tree navigable. Dashboard populated. File watching live.

### Scope

- Top-level Bubbletea program with Init/Update/View.
- Responsive layout: header, tree, detail, footer.
- Three entry states with correct detection and transitions.
- Header with daemon status and aggregate counts.
- Footer with context-sensitive key hints.
- Tree pane with navigation, expand/collapse, status glyphs.
- Current target indicator (`▶` prefix on active node/task).
- Dashboard detail view with progress summary.
- fsnotify watcher on state files and instance registry.
- Polling fallback at 2-second intervals.
- `tea.WindowSizeMsg` handling for responsive resize.
- Welcome screen with directory browser and init flow.
- Loading spinner in header for reads >100ms.
- Error bar for corrupt state files.
- Pane-local search (`/` key, incremental highlight, `n`/`N` navigation).
- Help overlay (`?` key, grouped key bindings, scrollable, captures input).
- Clipboard copy (`y` key, OSC 52 escape sequence).

### Phase 1 Models

All models described in the Layout section above are Phase 1: `HeaderModel`, `TreeModel`, `DetailModel`, `DashboardModel`, `FooterModel`, `WelcomeModel`, `SearchModel`, `HelpOverlayModel`.

The top-level model:

```go
type TUIModel struct {
    // Layout
    width      int
    height     int
    treeVisible bool

    // Focus
    focused    FocusedPane

    // Sub-models
    header     HeaderModel
    tree       TreeModel
    detail     DetailModel
    footer     FooterModel
    welcome    *WelcomeModel // nil unless in State 3
    search     SearchModel
    help       HelpOverlayModel
    overlayActive bool        // true when help or search captures input

    // State
    entryState EntryState
    store      *state.Store
    daemonRepo *daemon.DaemonRepository
    worktreeDir string // resolved .wolfcastle parent

    // Watcher
    watcher    *Watcher

    // Error bar
    errors     []errorEntry
}

type EntryState int

const (
    StateLive EntryState = iota
    StateCold
    StateWelcome
)

type errorEntry struct {
    filename string
    message  string
}
```

### Phase 1 Message Types

```go
// WindowSizeMsg is tea.WindowSizeMsg (built-in).

// StateUpdatedMsg carries a fresh RootIndex after a state.json change.
type StateUpdatedMsg struct {
    Index *state.RootIndex
}

// NodeUpdatedMsg carries a refreshed NodeState for a single address.
type NodeUpdatedMsg struct {
    Address string
    Node    *state.NodeState
}

// DaemonStatusMsg carries the daemon status string after a registry check.
type DaemonStatusMsg struct {
    Status      string
    Branch      string
    PID         int
    IsRunning   bool
    IsDraining  bool
    Instances   []instance.Entry
}

// InstancesUpdatedMsg carries the full list of live instances.
type InstancesUpdatedMsg struct {
    Instances []instance.Entry
}

// WatcherEventMsg signals that the watcher detected a file change.
type WatcherEventMsg struct {
    Path string
    Op   fsnotify.Op
}

// PollTickMsg signals a polling interval has elapsed.
type PollTickMsg struct{}

// SpinnerTickMsg advances the loading spinner.
type SpinnerTickMsg struct{}

// ErrorMsg carries a persistent error for the error bar.
type ErrorMsg struct {
    Filename string
    Message  string
}

// ErrorClearedMsg clears a specific error from the bar.
type ErrorClearedMsg struct {
    Filename string
}

// InitStartedMsg and InitCompleteMsg (see Welcome section above).

// ToggleHelpMsg toggles the help overlay visibility.
type ToggleHelpMsg struct{}

// CopyMsg requests copying text to clipboard via OSC 52.
type CopyMsg struct {
    Text string
}

// CopiedMsg confirms the OSC 52 escape was written to stdout.
type CopiedMsg struct{}
```

### Phase 1 Key Bindings

#### Global (all states except Welcome)

| Key | Binding Name | Action |
|-----|-------------|--------|
| `q` | quit | Send `tea.Quit` |
| `Ctrl+C` | forceQuit | Send `tea.Quit` |
| `d` | dashboard | Set detail mode to `ModeDashboard` |
| `t` | toggleTree | Toggle `treeVisible` |
| `Tab` | cycleFocus | Cycle `focused` between `PaneTree` and `PaneDetail` |
| `R` | refresh | Send force-refresh command (re-read all state) |
| `?` | toggleHelp | Toggle help overlay (captures all input when active) |
| `/` | search | Open pane-local search bar in focused pane |
| `y` | copy | Copy current selection to clipboard via OSC 52 |

#### Search (when search bar is active)

| Key | Binding Name | Action |
|-----|-------------|--------|
| Any printable | (text input) | Append to search query, update highlights |
| `Enter` | confirmSearch | Jump to first match, dismiss search bar, keep highlights |
| `Esc` | cancelSearch | Dismiss search bar, clear highlights |
| `n` | nextMatch | Jump to next match (after search dismissed with Enter) |
| `N` | prevMatch | Jump to previous match (after search dismissed with Enter) |

#### Help Overlay (when active)

| Key | Binding Name | Action |
|-----|-------------|--------|
| `?` | dismissHelp | Dismiss overlay |
| `Esc` | dismissHelp | Dismiss overlay |
| `j`, `↓` | scrollDown | Scroll overlay content down |
| `k`, `↑` | scrollUp | Scroll overlay content up |
| All other keys | (captured) | No action; overlay absorbs all other input |

#### Tree Pane (when `focused == PaneTree`)

| Key | Binding Name | Action |
|-----|-------------|--------|
| `j`, `↓` | moveDown | Increment `cursor`, clamp to `len(flatList)-1` |
| `k`, `↑` | moveUp | Decrement `cursor`, clamp to 0 |
| `Enter`, `l`, `→` | expand | Toggle expand on orchestrator; switch detail to node/task detail (Phase 2) |
| `Esc`, `h`, `←` | collapse | Collapse current node; if already collapsed, move cursor to parent |
| `g` | top | Set `cursor` to 0 |
| `G` | bottom | Set `cursor` to `len(flatList)-1` |

`Enter` on a tree row in Phase 1 expands/collapses the node. In Phase 2, it also loads the node or task detail into the detail pane.

### Phase 1 Rendering Details

#### Pane Borders

Focused pane border: `lipgloss.Border(lipgloss.RoundedBorder())` with `BorderForeground(lipgloss.Color("1"))` (bright red).

Unfocused pane border: same border shape, `BorderForeground(lipgloss.Color("240"))` (dim gray).

#### Color Palette

| Element | Foreground | Background |
|---------|-----------|------------|
| Header bar | `lipgloss.Color("15")` (white) | `lipgloss.Color("52")` (dark red, #5f0000) |
| Tree selected row | `lipgloss.Color("15")` (white, bold) | `lipgloss.Color("236")` (dark gray) |
| Tree normal row | `lipgloss.Color("252")` (light gray) | terminal default |
| Footer | `lipgloss.Color("245")` (dim white) | terminal default |
| Error bar | `lipgloss.Color("1")` (red, bold) | `lipgloss.Color("52")` (dark red) |
| Dashboard heading | `lipgloss.Color("15")` (white, bold) | terminal default |
| Dashboard body | `lipgloss.Color("252")` (light gray) | terminal default |
| Spinner | `lipgloss.Color("3")` (yellow) | inherits |

### Phase 1 Search Model

The search model and behavior defined here apply to Phase 1. In Phase 1, search operates over the tree pane (matching node names and task IDs). Phase 2 extends search to the log stream, node detail, and task detail panes. Phase 4 extends it to the inbox pane.

```go
type SearchModel struct {
    input     textinput.Model
    active    bool
    query     string
    matches   []searchMatch
    current   int              // index into matches for n/N navigation
    paneType  FocusedPane      // which pane this search is bound to
}

type searchMatch struct {
    row    int    // row index in the pane's content
    col    int    // character offset within the row
    length int    // match length
}
```

**Search behavior in tree pane:** search matches against node names and task IDs/titles. Matching rows are highlighted with a yellow background tint (`lipgloss.Color("3")` background). `n` jumps to the next match and moves the cursor; `N` jumps to the previous. Non-matching rows remain visible without highlighting. The search bar renders at the bottom of the tree pane:

```
/{query}  {current}/{total} matches
```

When no matches exist: `No matches. Adjust your aim.`

### Phase 1 Help Overlay Model

```go
type HelpOverlayModel struct {
    active    bool
    scroll    int
    width     int
    height    int
}
```

Centered overlay, 60% of terminal width and 80% of terminal height (minimum 40x20). Renders on a solid dark background (`lipgloss.Color("235")`). Border uses `lipgloss.RoundedBorder()` in dim white.

Content is a grouped list of all key bindings available in the current version. Groups: Global, Tree Navigation, Search. Phase 2 adds Log Stream. Phase 3 adds Daemon Control and Instance Switching. Phase 4 adds Inbox. The overlay is scrollable if content exceeds the overlay height.

The overlay captures all input except `?` and `Esc`, which dismiss it.

### Phase 1 Clipboard Copy

`y` copies the primary identifier of the currently selected item. In Phase 1, the tree is the only pane with selectable items:
- Tree node selected: copies the node address (e.g., `auth-system/login`).
- Tree task selected: copies the full task address (e.g., `auth-system/login/task-0003`).

Phase 2 extends clipboard to node detail (copies node address), task detail (copies task address), and log lines (copies raw JSON). Phase 4 extends it to inbox items (copies item text).

The copy uses the OSC 52 escape sequence: `\x1b]52;c;{base64_encoded_content}\x07`. This works in iTerm2, kitty, WezTerm, Alacritty, and recent versions of tmux with `set -g set-clipboard on`.

After a successful copy, the footer briefly (2 seconds) shows `Copied.` in place of the normal key hints, then reverts. No fallback to `pbcopy` or `xclip`. If OSC 52 is not supported by the terminal, the copy silently does nothing. This limitation is documented.

### Phase 1 Error Handling

| Failure | User sees | Recovery |
|---------|-----------|----------|
| `Store.ReadIndex()` returns error (corrupt JSON) | Error bar: `State corruption detected: state.json. Run wolfcastle doctor.` Last known good tree remains displayed. | Auto-clears on next successful read. User can dismiss with `Esc`. |
| `Store.ReadNode(addr)` returns error | Node shows in tree but expansion fails. Error bar: `Unreadable: {addr}/state.json. Run wolfcastle doctor.` | Retry on next poll tick or fsnotify event. |
| `instance.List()` returns error | Header shows `status unknown`. | Retry on next poll tick. |
| `instance.Resolve(cwd)` returns no match | Entry state set to `StateCold`. Normal cold-start behavior. | Watcher monitors instance registry for new entries. |
| fsnotify watcher fails to start | No error bar (silent). Polling fallback activates at 2-second intervals. A debug log line is emitted to stderr. | Polling continues for TUI lifetime. |
| fsnotify watch on specific path fails | That path falls back to polling. Other watches unaffected. | Polling at 2s for the failed path. |
| Terminal too small (<20 cols or <5 rows) | Centered message: `Terminal too small.` No panes rendered. | Responds to `tea.WindowSizeMsg` and re-renders when large enough. |
| Welcome init fails | Error shown in red below directory browser. User retries or quits. | `Enter` to retry, navigate to different directory, or `q` to quit. |
| `.wolfcastle/` detection walks to filesystem root without finding it | Entry state set to `StateWelcome`. | Normal welcome flow. |
| OSC 52 copy on unsupported terminal | Nothing visible. Copy silently does nothing. | Documented limitation. User uses `wolfcastle status` for copyable output. |

### Test Cases and Acceptance Criteria

#### Acceptance Criteria

1. The TUI detects `StateLive` when `instance.Resolve(cwd)` returns a live entry whose PID is running.
2. The TUI detects `StateCold` when `.wolfcastle/` exists but no live instance is resolved.
3. The TUI detects `StateWelcome` when no `.wolfcastle/` directory is found walking up from CWD.
4. The header displays two lines at terminal widths >= 40 columns.
5. The header collapses to one line at terminal widths < 40 columns.
6. The header line 1 shows `WOLFCASTLE`, version, daemon status text, and instance count badge when applicable.
7. The header line 2 shows aggregate node status counts with correct glyphs and audit summary.
8. The header displays the daemon PID when an instance is running (format: `hunting (PID {pid})`).
9. The header displays `standing down` when no daemon is running.
10. The tree pane renders at 30% of terminal width with a minimum of 24 columns.
11. The tree pane hides entirely below 60 columns terminal width.
12. The tree pane reappears when `t` is pressed while hidden.
13. Tree rows display indent, expand marker, name, and status glyph in the correct order.
14. The cursor moves down on `j`/`↓` and up on `k`/`↑`, clamped to valid bounds.
15. `g` moves cursor to the first row; `G` moves cursor to the last row.
16. `Enter`/`l`/`→` expands an orchestrator node, revealing its children.
17. `Esc`/`h`/`←` collapses an expanded node; if already collapsed, moves cursor to the parent.
18. Expanding a leaf node shows its tasks indented one level deeper.
19. The current target node displays a `▶` prefix in bright yellow.
20. The dashboard view renders the `MISSION BRIEFING` header, status, progress bars, recent activity, and audit summary.
21. The dashboard shows `No transmissions. The daemon has not spoken.` when no log files exist in cold-start state.
22. The dashboard shows `No targets. Feed the inbox.` when no nodes exist.
23. fsnotify file change events trigger `StateUpdatedMsg` that re-reads and updates the tree and header.
24. When fsnotify is unavailable, the 2-second polling fallback detects mtime changes and triggers view updates.
25. The search bar opens on `/`, accepts incremental input, highlights matching rows in yellow, and shows match count.
26. `n` advances to the next match; `N` returns to the previous match.
27. `Esc` during search dismisses the search bar and clears highlights.
28. `Enter` during search confirms the search, jumps to the first match, and keeps highlights for `n`/`N` navigation.
29. Search with no matches displays `No matches. Adjust your aim.`
30. The help overlay opens on `?`, renders grouped key bindings, and captures all input except `?` and `Esc`.
31. The help overlay is scrollable with `j`/`k` when content exceeds its height.
32. `y` copies the selected node address via OSC 52 escape sequence.
33. After copy, the footer shows `Copied.` for 2 seconds before reverting to normal key hints.
34. `Tab` cycles focus between tree and detail panes; the focused pane border color changes to bright red.
35. When tree is hidden, `Tab` does nothing and focus is locked to `PaneDetail`.
36. The focused pane border uses `lipgloss.Color("1")`; the unfocused pane border uses `lipgloss.Color("240")`.
37. Keyboard input routes to the help overlay when it is active, to the search bar when it is active, then to global bindings, then to the focused pane.
38. `Ctrl+C` always triggers `tea.Quit` regardless of overlay state.
39. A corrupt `state.json` shows error bar text `State corruption detected: state.json. Run wolfcastle doctor.` and retains the last known good tree.
40. A missing `.wolfcastle/` directory (walking to root) sets entry state to `StateWelcome`.
41. An unreadable node state file shows error bar text `Unreadable: {addr}/state.json. Run wolfcastle doctor.`
42. Terminal smaller than 20x5 renders only `Terminal too small.` centered.
43. The welcome screen renders a centered directory browser with `WOLFCASTLE` title.
44. Welcome screen `Enter` on a directory with `.wolfcastle/` absent runs init and transitions to `StateCold` on success.
45. Welcome screen init failure displays the error in red below the directory browser.
46. The loading spinner appears in the header when a data read exceeds 100ms.

#### Test Cases

| ID | Description | Setup | Action | Expected Result |
|----|-------------|-------|--------|-----------------|
| P1-001 | Live entry state detection | Daemon running, `.wolfcastle/` exists, instance registry has live PID | Launch TUI | `entryState == StateLive`; header shows `hunting (PID {pid})` |
| P1-002 | Cold-start entry state detection | `.wolfcastle/` exists, no live instance in registry | Launch TUI | `entryState == StateCold`; header shows `standing down` |
| P1-003 | Welcome entry state detection | No `.wolfcastle/` directory in CWD or any parent | Launch TUI | `entryState == StateWelcome`; welcome screen renders with directory browser |
| P1-004 | Layout at 80 columns | Terminal width 80, height 24 | Render | Tree pane is 24 cols (30% of 80), detail pane is 55 cols (80-24-1), header 2 lines, footer 1 line |
| P1-005 | Layout at 60 columns | Terminal width 60, height 24 | Render | Tree pane is 24 cols (minimum), detail pane is 35 cols |
| P1-006 | Layout at 59 columns (tree hidden) | Terminal width 59, height 24 | Render | Tree pane hidden, detail pane full width (59 cols) |
| P1-007 | Layout at 40 columns (header collapse) | Terminal width 39, height 24 | Render | Header collapses to 1 line; content area is `height - 2` |
| P1-008 | Tree cursor down | Tree with 5 nodes, cursor at row 0 | Press `j` | Cursor moves to row 1; row 1 has selected highlight |
| P1-009 | Tree cursor down at bottom | Tree with 5 nodes, cursor at row 4 | Press `j` | Cursor stays at row 4 (clamped) |
| P1-010 | Tree cursor up | Tree with 5 nodes, cursor at row 3 | Press `k` | Cursor moves to row 2 |
| P1-011 | Tree cursor up at top | Tree with 5 nodes, cursor at row 0 | Press `k` | Cursor stays at row 0 (clamped) |
| P1-012 | Tree jump to top | Cursor at row 4 | Press `g` | Cursor moves to row 0 |
| P1-013 | Tree jump to bottom | Cursor at row 0, 10 rows total | Press `G` | Cursor moves to row 9 |
| P1-014 | Expand orchestrator node | Orchestrator node with 3 children, collapsed | Press `Enter` on orchestrator row | Node expands; `flatList` grows by 3 rows (children visible); expand marker changes from `▸` to `▾` |
| P1-015 | Collapse orchestrator node | Orchestrator node expanded with 3 children visible | Press `Esc` on orchestrator row | Node collapses; `flatList` shrinks by 3; expand marker changes from `▾` to `▸` |
| P1-016 | Collapse already-collapsed node | Cursor on a collapsed orchestrator, parent is at row 2 | Press `h` | Cursor moves to row 2 (parent row) |
| P1-017 | Expand leaf node shows tasks | Leaf node with 2 tasks | Press `Enter` on leaf row | 2 task rows appear indented one level deeper, each with task glyph and ID |
| P1-018 | Current target indicator | Daemon's current target is `auth-system/login` | Render tree | Row for `auth-system/login` displays `▶` prefix in bright yellow |
| P1-019 | Dashboard live data | StateLive, 12 nodes (4 complete, 3 in_progress, 3 not_started, 2 blocked), daemon PID 12345 | Render dashboard | Shows `MISSION BRIEFING`, `Status: hunting (PID 12345)`, progress bars with correct percentages, recent activity entries |
| P1-020 | Dashboard cold-start no logs | StateCold, no log files in log directory | Render dashboard | Shows `Status: standing down`, no uptime line, recent activity shows `No transmissions. The daemon has not spoken.` |
| P1-021 | Dashboard cold-start with logs | StateCold, log files from previous session exist | Render dashboard | Shows `Status: standing down`, recent activity shows last 5 entries from most recent log file |
| P1-022 | Dashboard empty state | No nodes in RootIndex | Render dashboard | Shows `No targets. Feed the inbox.` |
| P1-023 | fsnotify triggers state update | Watcher active on `state.json` | External process writes to `state.json` | `StateUpdatedMsg` fires within debounce window (100-500ms); tree and header update with new data |
| P1-024 | fsnotify debounce coalescing | Watcher active | 10 rapid writes to `state.json` within 50ms | Single `StateUpdatedMsg` fires after debounce; no duplicate reads |
| P1-025 | fsnotify debounce max slide | Watcher active | Continuous writes for 600ms | Flush occurs at 500ms mark regardless of ongoing events |
| P1-026 | Polling fallback detects changes | fsnotify disabled (init failed) | External process writes `state.json`, 2+ seconds elapse | `StateUpdatedMsg` fires on next poll tick; tree and header update |
| P1-027 | Polling no-op when unchanged | Polling active, no external changes | 2 seconds elapse | Poll tick fires, mtimes match, no messages sent, no re-reads |
| P1-028 | Search open and filter | Tree with nodes "auth-system", "api-gateway", "billing" | Press `/`, type `api` | Search bar renders at tree bottom showing `/api  1/1 matches`; `api-gateway` row highlighted in yellow |
| P1-029 | Search navigate matches | Tree with "auth-login", "auth-oauth", "billing", search query "auth" | Press `Enter` to confirm, then press `n` | Cursor jumps to first match, then to second match on `n` |
| P1-030 | Search previous match | Search confirmed with 3 matches, cursor on match 2 | Press `N` | Cursor returns to match 1 |
| P1-031 | Search dismiss with Esc | Search bar active with query "api" | Press `Esc` | Search bar disappears, highlights clear, tree returns to normal rendering |
| P1-032 | Search no matches | Tree with "auth-system", "billing" | Press `/`, type `xyz` | Search bar shows `No matches. Adjust your aim.` |
| P1-033 | Help overlay opens | No overlay active | Press `?` | Help overlay renders centered (60% width, 80% height), shows `WOLFCASTLE KEY BINDINGS`, lists Global and Tree Navigation groups |
| P1-034 | Help overlay captures input | Help overlay active | Press `j`, then `d`, then `q` | `j` scrolls down, `d` is absorbed (does not switch to dashboard), `q` is absorbed (does not quit) |
| P1-035 | Help overlay dismisses | Help overlay active | Press `?` or `Esc` | Overlay disappears, `overlayActive` is false, normal input routing resumes |
| P1-036 | Help overlay scrollable | Help overlay with content taller than overlay height | Press `j` multiple times | Content scrolls down; additional bindings become visible |
| P1-037 | Clipboard copy node address | Tree focused, cursor on node `auth-system/login` | Press `y` | OSC 52 escape sequence written to stdout with base64-encoded `auth-system/login`; footer shows `Copied.` |
| P1-038 | Clipboard copy task address | Tree focused, cursor on task row `auth-system/login/task-0003` | Press `y` | OSC 52 sequence contains base64 of `auth-system/login/task-0003`; footer shows `Copied.` |
| P1-039 | Copy confirmation reverts | Copy just performed, footer shows `Copied.` | Wait 2 seconds | Footer reverts to normal key hint display |
| P1-040 | Focus cycle with Tab | Focus on `PaneTree` | Press `Tab` | Focus moves to `PaneDetail`; detail pane border turns bright red, tree pane border turns dim gray |
| P1-041 | Focus locked when tree hidden | Terminal width 50 (tree hidden), focus on `PaneDetail` | Press `Tab` | Focus remains on `PaneDetail`; no change |
| P1-042 | Key routing: Ctrl+C always quits | Help overlay active | Press `Ctrl+C` | `tea.Quit` fires; TUI exits |
| P1-043 | Key routing: overlay captures | Search bar active, user types `q` | Press `q` | Character `q` appended to search query; TUI does not quit |
| P1-044 | Corrupt state.json error bar | `Store.ReadIndex()` returns JSON parse error | TUI reads state | Error bar displays `State corruption detected: state.json. Run wolfcastle doctor.`; last known good tree remains |
| P1-045 | Error bar auto-clears | Error bar showing corrupt state message | External process fixes `state.json`, fsnotify fires | Error bar clears; tree updates with new data |
| P1-046 | Unreadable node file | `Store.ReadNode("auth")` returns error | Expand node `auth` in tree | Error bar displays `Unreadable: auth/state.json. Run wolfcastle doctor.`; node row remains in tree but does not expand |
| P1-047 | Terminal too small | Terminal width 15, height 4 | Render | Only `Terminal too small.` displayed centered; no panes rendered |
| P1-048 | Terminal resize recovery | Terminal at 15x4 showing "too small" message | Resize terminal to 80x24 | Full layout renders: header, tree, detail, footer |
| P1-049 | Welcome directory navigation | Welcome screen showing `/Users/wild/projects` with 3 subdirectories | Press `j` to move down, `Enter` to descend | Cursor moves; entering a directory updates listing to show that directory's children |
| P1-050 | Welcome init success | Welcome screen, cursor on directory without `.wolfcastle/` | Press `Enter` on confirm prompt | Spinner shows `Initializing in {path}...`, then transitions to `StateCold` with tree and dashboard |
| P1-051 | Welcome init failure | Welcome screen, init will fail (permission denied) | Press `Enter` on confirm prompt | Red error text below browser: `Init failed: {error}`; user can navigate elsewhere or retry |
| P1-052 | Loading spinner in header | Data read will take >100ms | Trigger state read | Spinner glyph appears in header line 1 cycling through `⠋⠙⠹⠸⠼⠴⠦⠧⠇⠏`; disappears when read completes |
| P1-053 | Node cache eviction | Expand node, then collapse it | Wait 30+ seconds | Cached `NodeState` evicted from `nodes` map; re-expanding triggers fresh `Store.ReadNode()` |
| P1-054 | Progress bar rendering | 4 of 12 nodes complete | Render dashboard | Progress bar shows `████░░░░░░░░  33%` (4 filled blocks out of 12) |
| P1-055 | Header node count omits zero statuses | 5 nodes: 3 complete, 2 in_progress, 0 not_started, 0 blocked | Render header line 2 | Shows `5 nodes: 3● 2◐`; does not include `0◯` or `0☢` |

