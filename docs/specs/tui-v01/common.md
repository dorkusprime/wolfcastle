# Wolfcastle TUI v0.1: Implementation Spec

## Status

Draft

## Supersedes

This spec fully replaces `2026-03-15T00-02Z-tui.md`. That document is superseded and should not be referenced for implementation decisions. Everything relevant from it has been incorporated here with more detail and updated design choices (multi-process awareness, Bubbletea v2 APIs, daemon lifecycle control from the TUI, vendored dependencies).

## Summary

`wolfcastle` with no subcommand launches the TUI. Three entry states: live dashboard (daemon running), cold-start (`.wolfcastle/` exists, no daemon), and welcome (no `.wolfcastle/`). The TUI renders a split-pane layout with a project tree on the left, a detail pane on the right, a two-line header, and a one-line footer. Real-time updates arrive through fsnotify with a polling fallback. All existing CLI commands remain unchanged.

Five phases deliver the full v0.1 scope. The original spec tags log stream, node detail, task detail, pane-local search, help overlay, and clipboard copy as v0.1 features. All of those land in the first three phases here, with the most foundational features in Phase 1.

| Phase | Theme | Summary |
|-------|-------|---------|
| 1 | Shell | App frame, layout, header, footer, tree, dashboard, fsnotify, three entry states, pane-local search, help overlay, clipboard copy (OSC 52) |
| 2 | Depth | Log stream, node detail, task detail views |
| 3 | Control | Instance switcher, daemon start/stop from TUI |
| 4 | Inbox | View and add inbox items |
| 5 | Polish | Notification toasts, additional refinements |

## Dependencies

All vendored per ADR-101. No CGO. Single binary, no headless build tag.

| Package | Import Path | Purpose |
|---------|-------------|---------|
| Bubbletea v2 | `charm.land/bubbletea/v2` | Model-Update-View loop, program lifecycle, tea.Cmd, tea.Msg |
| Lipgloss v2 | `charm.land/lipgloss/v2` | Styling: colors, borders, padding, alignment, inline joins |
| Bubbles v2/viewport | `charm.land/bubbles/v2/viewport` | Scrollable content panes (detail, logs) |
| Bubbles v2/textinput | `charm.land/bubbles/v2/textinput` | Search bar, inbox text entry |
| Bubbles v2/key | `charm.land/bubbles/v2/key` | Key binding declarations with help text |
| Bubbles v2/help | `charm.land/bubbles/v2/help` | Footer key hint rendering |
| fsnotify | `github.com/fsnotify/fsnotify` | Filesystem event watching for real-time updates |

### Bubbletea v2 API Reference

This project uses Bubbletea v2.0, which has breaking changes from v1. Key differences for implementors:

**Import paths changed:** `charm.land/bubbletea/v2` (not `github.com/charmbracelet/bubbletea`). Same for lipgloss and bubbles.

**View returns `tea.View`, not `string`.** Terminal features (alt screen, mouse mode, window title) are declarative fields on the View struct:

```go
func (m model) View() tea.View {
    v := tea.NewView(rendered)
    v.AltScreen = true
    v.WindowTitle = "WOLFCASTLE"
    return v
}
```

**Key events use `tea.KeyPressMsg`** (not `tea.KeyMsg` which is now an interface covering both presses and releases). Key fields: `msg.Code` (rune), `msg.Text` (string), `msg.Mod` (modifiers). Space bar returns `"space"` from `msg.String()`, not `" "`.

```go
case tea.KeyPressMsg:
    switch msg.String() {
    case "q", "ctrl+c":
        return m, tea.Quit
    case "space":
        // handle space
    }
```

**No more imperative program options.** `tea.WithAltScreen()`, `tea.WithMouseCellMotion()`, `tea.WithReportFocus()` are gone. Set the corresponding fields on `tea.View` in `View()` instead. `tea.NewProgram(model{})` is all you need.

**Removed commands.** `tea.EnterAltScreen`, `tea.ExitAltScreen`, `tea.EnableMouseCellMotion`, `tea.HideCursor`, `tea.ShowCursor`, `tea.SetWindowTitle()` are all gone. Set View fields instead.

**Mouse events split by type.** `tea.MouseClickMsg`, `tea.MouseReleaseMsg`, `tea.MouseWheelMsg`, `tea.MouseMotionMsg` replace the old `tea.MouseMsg` struct. Button constants shortened: `tea.MouseLeft` (not `tea.MouseButtonLeft`).

**New program options for testing:** `tea.WithColorProfile(p)` forces a color profile, `tea.WithWindowSize(w, h)` sets initial size. Both useful for deterministic test output.

Internal dependencies (already exist in the codebase):

| Package | Purpose |
|---------|---------|
| `internal/state` | `Store`, `RootIndex`, `NodeState`, `Task`, `InboxFile`, all state types |
| `internal/instance` | `Entry`, `List()`, `Resolve()`, `Slug()` for instance registry |
| `internal/logging` | `LatestLogFile()` for finding the current log file |
| `internal/logrender` | `Record`, `ParseRecord()` for NDJSON log parsing |
| `internal/daemon` | `DaemonRepository`, `IsProcessRunning()`, `HasDrainFile()` |
| `cmd/cmdutil` | `App` struct for Store and DaemonRepository access |

## Entry States

### State 1: Instance Running (Live Dashboard)

Detection: `instance.Resolve(cwd)` returns a live entry whose PID is running (`daemon.IsProcessRunning(entry.PID)` returns true).

Behavior:
- Header line 1 shows version, branch, PID, instance count.
- Header line 2 shows aggregate node status counts and audit summary.
- Tree loads from `Store.ReadIndex()`. Nodes show names and status glyphs.
- Detail pane opens to the dashboard view.
- fsnotify watcher starts immediately on all watched paths.
- Log tailing begins from the latest log file.

The TUI is fully interactive. All navigation, all panes, all key bindings are active.

### State 2: Cold Start (`.wolfcastle/` exists, no daemon)

Detection: `.wolfcastle/` directory exists in or above CWD (found by walking up the directory tree), but `instance.Resolve(cwd)` returns no live entry (either no registry file, or PID is dead).

Behavior:
- Header line 1 shows version and `standing down` (no PID, no branch).
- Header line 2 shows aggregate counts from the persisted state files (they survive between sessions).
- Tree loads from persisted state files. Full navigation works.
- Detail pane opens to dashboard. Dashboard shows `No transmissions. The daemon has not spoken.` if no log files exist, or shows a summary of the last session if logs exist.
- Log pane (Phase 2) shows the last session's log in replay mode (not tailing; the file is static).
- Footer shows `[s] start daemon` prominently (Phase 3).

The `.wolfcastle/` directory is located by walking CWD upward until a directory containing `.wolfcastle/` is found. This matches the behavior of `cmd/cmdutil.App` initialization.

### State 3: Welcome (No `.wolfcastle/`)

Detection: no `.wolfcastle/` directory found walking up from CWD.

Behavior: the normal split-pane layout is replaced entirely with a centered welcome screen.

```
WOLFCASTLE

No project found in this directory.

Initialize in:
> /Users/wild/repository/my-project    [Enter to confirm]

  (Use arrows to browse, or type a path)
```

The directory browser defaults to CWD. `j`/`k` or arrow keys navigate sibling directories. `Enter` on a directory descends into it. `h` or `Backspace` goes to the parent. `Enter` with the confirmation prompt visible runs `wolfcastle init` in the selected directory. On success, the TUI transitions to State 2 (cold-start).

The welcome model captures all input. Global keys except `q`/`Ctrl+C` are disabled.

#### Welcome Model Fields

```go
type WelcomeModel struct {
    currentDir  string      // resolved absolute path being displayed
    entries     []os.DirEntry // filtered directory listing (dirs only)
    cursor      int         // index into entries
    width       int
    height      int
    err         error       // last error (permission denied, etc.)
    initializing bool       // true while init is running
}
```

#### Welcome Messages

```go
// InitStartedMsg signals that wolfcastle init has begun.
type InitStartedMsg struct{}

// InitCompleteMsg signals that init finished. Err is nil on success.
type InitCompleteMsg struct {
    Dir string
    Err error
}
```

#### Welcome Key Bindings

| Key | Action |
|-----|--------|
| `j`, `↓` | Move cursor down |
| `k`, `↑` | Move cursor up |
| `Enter`, `l`, `→` | Descend into directory / confirm init |
| `h`, `←`, `Backspace` | Go to parent directory |
| `g` | Jump to first entry |
| `G` | Jump to last entry |
| `q`, `Ctrl+C` | Quit |

#### Welcome Rendering

The welcome screen is vertically and horizontally centered using `lipgloss.Place()`. The directory listing shows only directories (no files). Hidden directories (names starting with `.`) are excluded except `.wolfcastle` itself. The currently highlighted directory uses bold white on dark gray. Maximum 20 entries visible; scrolling if more exist.

When `initializing` is true, the screen shows:

```
WOLFCASTLE

Initializing in /Users/wild/repository/my-project...
```

With a spinner glyph cycling through `⠋⠙⠹⠸⠼⠴⠦⠧⠇⠏` at 80ms intervals.

On init failure, the screen shows the error in red below the directory browser: `Init failed: {error}`. The user can try again or navigate elsewhere.

## Layout

### Dimensions and Breakpoints

| Element | Width | Height | Notes |
|---------|-------|--------|-------|
| Header | 100% of terminal | 2 lines | Fixed, never scrolls |
| Tree pane | 30% of terminal, min 24 cols | Terminal height minus 3 | Scrollable |
| Detail pane | Remaining width | Terminal height minus 3 | Scrollable viewport |
| Footer | 100% of terminal | 1 line | Fixed, never scrolls |
| Pane divider | 1 col (drawn by border) | Full pane height | Part of tree's right border |

Breakpoints:
- Below 80 cols: tree shrinks to minimum 24 cols, detail gets the rest.
- Below 60 cols: tree hides entirely. Detail goes full-width. `t` toggles tree back.
- Below 40 cols: header line 2 hides. Single-line header with version and daemon status only.

Height: header is always 2 lines (1 line below 40 cols wide). Footer is always 1 line. Content area is `termHeight - 3` (or `termHeight - 2` when header collapses).

### Header Bar

Two lines, always visible (except the narrow-width collapse described above).

**Line 1:**

```
WOLFCASTLE v0.5.3        feat/auth (PID 12345)   [2 running]
```

- Left: `WOLFCASTLE` in bold white on dark red background, then version string in normal white.
- Right: current instance branch and PID. When multiple instances are running, a badge `[N running]` appears at the far right.
- When no daemon is running: right side reads `standing down`.
- When daemon is draining: right side reads `draining (PID 12345)`.

**Line 2:**

```
12 nodes: 4● 3◐ 3◯ 2☢    Audit: 2 passed, 1 gap
```

- Left: total node count, then per-status counts with glyphs.
- Right: audit summary. Count of passed audits, then open gap count if nonzero, then open escalation count if nonzero.

When a data read is in progress (takes >100ms), a spinner glyph appears after the daemon status text on line 1.

**Header data sources:**
- Daemon status: `instance.Resolve(cwd)` then `daemon.IsProcessRunning(entry.PID)` and `repo.HasDrainFile()`.
- Node counts: iterate `RootIndex.Nodes`, count by `IndexEntry.State`.
- Audit counts: iterate loaded `NodeState` files, count by `AuditState.Status`, sum open gaps and escalations.

#### Header Model

```go
type HeaderModel struct {
    version       string           // from build info or hardcoded
    daemonStatus  string           // "hunting (PID 12345)", "standing down", etc.
    branch        string           // from instance.Entry.Branch
    instanceCount int              // from len(instance.List())
    nodeCounts    map[state.NodeStatus]int
    totalNodes    int
    auditCounts   map[state.AuditStatus]int
    openGaps      int
    openEscalations int
    width         int
    spinner       spinner          // cycling glyph when loading
    loading       bool
}
```

#### Header Rendering (exact text)

Daemon status strings (matching the existing `getDaemonStatus` output format, translated to Wolfcastle voice):

| Condition | Line 1 right side |
|-----------|-------------------|
| Running, not draining | `hunting (PID {pid})` on `{branch}` |
| Running, draining | `draining (PID {pid})` on `{branch}` |
| Stopped, stale PID in registry | `presumed dead (stale PID {pid})` |
| Stopped, no registry entry | `standing down` |
| Status check failed | `status unknown` |

Node count format: `{total} nodes: {complete}● {in_progress}◐ {not_started}◯ {blocked}☢`. Omit any status with zero count. If total is zero: `0 nodes`.

Audit format: `Audit: {passed} passed{, {gaps} gap}{, {escalations} escalation}`. Pluralize "gaps"/"escalations" when count > 1. Omit passed/gap/escalation segments when their count is zero. If no audit data at all: `Audit: no data`.

### Status Glyphs

| Glyph | Status | Lipgloss Color |
|-------|--------|----------------|
| `●` | complete | `lipgloss.Color("2")` (green) |
| `◐` | in_progress | `lipgloss.Color("3")` (yellow) |
| `◯` | not_started | `lipgloss.Color("245")` (dim white) |
| `☢` | blocked | `lipgloss.Color("1")` (red) |

These are the same four glyphs from the original spec section 2.9. The CLI `status` command uses ASCII fallbacks (`✓`, `→`, `○`, `✖`); the TUI uses the Unicode originals since it controls the rendering environment.

#### Audit Status Glyphs

Used in node detail audit sections (Phase 2) and the audit trail view (future versions). The same position as node status glyphs but a different vocabulary:

| Glyph | Audit Status | Lipgloss Color |
|-------|-------------|----------------|
| `⏸` | passed | `lipgloss.Color("2")` (green) |
| `◐` | in_progress | `lipgloss.Color("3")` (yellow) |
| `◯` | pending | `lipgloss.Color("245")` (dim white) |
| `⊘` | failed | `lipgloss.Color("1")` (red) |

### Left Pane: Project Tree

Navigable tree built from `RootIndex`. Each line is one node.

Rendering per line:

```
{indent}{expand_marker} {name} {status_glyph}
```

- `indent`: 2 spaces per `DecompositionDepth` level.
- `expand_marker`: `▸` for collapsed orchestrator with children, `▾` for expanded orchestrator, empty for leaf nodes.
- `name`: `IndexEntry.Name`, truncated to fit pane width minus indent minus glyph minus 4 chars padding.
- `status_glyph`: from the glyph table above.

The selected line gets bold white text on a dark gray background (`lipgloss.Color("236")`).

When a node is the daemon's current target (derived from the most recent `stage_start` log record that names a node), it gets a `▶` prefix in bright yellow before the name.

Tree ordering follows `RootIndex.Root` for top-level nodes, `IndexEntry.Children` for children of orchestrators.

Expanding a leaf node shows its tasks indented one level deeper:

```
    {task_glyph} {task_id}: {title_or_description_truncated}
```

Task glyph uses the same status glyph table. `title` is preferred; fall back to first 40 chars of `description` if no title.

#### Tree Model

```go
type TreeModel struct {
    index      *state.RootIndex
    nodes      map[string]*state.NodeState // cached, loaded on expand
    flatList   []treeRow                   // flattened visible rows
    cursor     int                         // index into flatList
    scrollTop  int                         // first visible row
    expanded   map[string]bool             // addr -> expanded
    focused    bool
    width      int
    height     int
    currentTarget string                   // addr of daemon's active node
}

type treeRow struct {
    addr       string          // node address or "addr/taskID" for tasks
    name       string          // display name
    depth      int             // indentation level
    nodeType   state.NodeType  // orchestrator or leaf
    status     state.NodeStatus
    isTask     bool
    expandable bool            // true for orchestrators with children
    isExpanded bool
}
```

#### Tree Data Sources

- `Store.ReadIndex()` for the full tree structure.
- `Store.ReadNode(addr)` for task lists when a leaf is expanded. Cached in `nodes` map; cache entry evicted 30 seconds after the node is collapsed.

### Right Pane: Detail

The detail pane is a container that swaps between sub-views. Phase 1 ships with the dashboard only. Phases 2-5 add more views.

#### Detail Model

```go
type DetailModel struct {
    mode       DetailMode
    dashboard  DashboardModel
    nodeDetail NodeDetailModel   // Phase 2
    taskDetail TaskDetailModel   // Phase 2
    logView    LogViewModel      // Phase 2
    inbox      InboxModel        // Phase 4
    viewport   viewport.Model    // shared scrollable viewport
    width      int
    height     int
    focused    bool
}

type DetailMode int

const (
    ModeDashboard DetailMode = iota
    ModeNodeDetail
    ModeTaskDetail
    ModeLogStream
    ModeInbox
)
```

Note: `ModeNodeDetail`, `ModeTaskDetail`, and `ModeLogStream` are defined from Phase 1 for type completeness but their corresponding sub-models are only populated starting in Phase 2. In Phase 1, switching to these modes shows a placeholder: `Coming in the next phase.`

### Dashboard View (Phase 1)

The default detail view. Shows a summary of the project state.

```
MISSION BRIEFING

Status: hunting (PID 12345)
Branch: feat/auth
Uptime: 2h 34m

Progress:
  ● Complete    4/12  ████░░░░░░░░  33%
  ◐ In progress 3/12  ██░░░░░░░░░░  25%
  ◯ Not started 3/12
  ☢ Blocked     2/12

Recent Activity:
  14:32  Target eliminated: auth-system/login/task-0003
  14:28  [exec] Starting auth-system/oauth/task-0001
  14:25  Gap opened: api-gateway rate_limiting

Audit:
  2 passed, 1 in progress, 0 pending
  1 open gap, 0 escalations
```

When no daemon is running (cold-start): status reads `standing down`, uptime is omitted, recent activity shows `No transmissions. The daemon has not spoken.` if no log files exist, or the last 5 log entries from the most recent log file.

When no state exists at all: `No targets. Feed the inbox.`

#### Dashboard Model

```go
type DashboardModel struct {
    daemonStatus  string
    branch        string
    uptime        time.Duration
    nodeCounts    map[state.NodeStatus]int
    totalNodes    int
    auditCounts   map[state.AuditStatus]int
    openGaps      int
    openEscalations int
    recentActivity []activityEntry // last 10 log entries of interest
    width         int
    height        int
}

type activityEntry struct {
    timestamp time.Time
    text      string    // pre-formatted display string
}
```

#### Dashboard Data Sources

- Node/audit counts: same computation as header (shared via `StateUpdatedMsg`).
- Recent activity: parsed from the last 200 lines of the latest log file. Filter for types: `stage_start`, `stage_complete`, `stage_error`, `auto_block`, `failure_increment`. Keep the most recent 10.
- Uptime: `time.Since(instance.Entry.StartedAt)`.

Progress bars use block characters: `█` for filled, `░` for empty. Bar width is 12 characters. Percentage is `count*100/total`, integer-rounded.

### Footer

Single line. Shows key bindings relevant to the current context.

Rendering uses `bubbles/help`. The footer model implements `help.KeyMap` to return bindings for the current state.

```
[q]uit [d]ash [l]ogs [i]nbox [t]ree [?]help [Tab] focus
```

When tree has focus, tree-specific bindings appear. When detail has focus, detail-specific bindings appear. The footer never wraps; bindings are dropped from the right if the terminal is too narrow, starting with the least essential.

#### Footer Model

```go
type FooterModel struct {
    focusedPane FocusedPane
    detailMode  DetailMode
    daemonRunning bool
    width       int
}

type FocusedPane int

const (
    PaneTree FocusedPane = iota
    PaneDetail
)
```

Binding priority (highest to lowest, for truncation):
1. `q` quit
2. `Tab` focus
3. Mode-specific bindings (varies by detail mode)
4. `?` help
5. `R` refresh


---

Reference sections follow. These apply to all phases.

## Data Sources (Complete Reference)

Every file the TUI reads, and which code path reads it.

| File | Read By | Write By | Watch |
|------|---------|----------|-------|
| `~/.wolfcastle/instances/*.json` | `instance.List()`, `instance.Resolve()` | `instance.Register()` (daemon startup) | fsnotify |
| `{ns}/state.json` (root index) | `Store.ReadIndex()` | Daemon state propagation | fsnotify |
| `{ns}/{addr}/state.json` (per-node) | `Store.ReadNode(addr)` | Daemon task execution | fsnotify (expanded nodes only for trees >1000 nodes) |
| `{ns}/inbox.json` | `Store.ReadInbox()` | `Store.MutateInbox()` (Phase 4) | fsnotify |
| `{wt}/.wolfcastle/system/logs/*.jsonl` | Direct file I/O with offset tracking | Daemon logging | fsnotify (creation events for new files) |
| `{ns}/scope-locks.json` | `Store.ReadScopeLocks()` | Daemon parallel execution | Not watched (read on demand for detail views) |

Where:
- `{wt}` = worktree directory (e.g., `/Users/wild/repository/wolfcastle/feat/auth`)
- `{ns}` = namespace projects directory (e.g., `{wt}/.wolfcastle/projects/{identity}`)

The TUI does not read config files directly. Daemon status comes from the instance registry, not from config.

## Real-Time Update Strategy

### fsnotify Watcher

A single watcher goroutine manages all file watches. It runs for the lifetime of the TUI process.

#### Watched Paths

| Path Pattern | Events | Handler |
|--------------|--------|---------|
| `~/.wolfcastle/instances/` (directory) | Create, Write, Remove | Send `InstancesUpdatedMsg` after re-reading `instance.List()` |
| `{ns}/state.json` | Write | Send `StateUpdatedMsg` after re-reading `Store.ReadIndex()` |
| `{ns}/{addr}/state.json` for each expanded node | Write | Send `NodeUpdatedMsg` after re-reading `Store.ReadNode(addr)` |
| `{ns}/inbox.json` | Write | Send `InboxUpdatedMsg` after re-reading `Store.ReadInbox()` |
| `{wt}/.wolfcastle/system/logs/` (directory) | Create | Send `NewLogFileMsg` with the new file path |
| `{wt}/.wolfcastle/system/logs/{current}.jsonl` | Write | Send `LogLinesMsg` after reading new lines from offset |

#### Watcher Model

```go
type Watcher struct {
    watcher     *fsnotify.Watcher
    store       *state.Store
    logDir      string
    logFile     string
    logOffset   int64
    instanceDir string
    debounce    *time.Timer       // 100ms debounce window
    pending     map[string]bool   // paths with pending events
    program     *tea.Program      // for sending messages
    done        chan struct{}
}
```

#### Debouncing

Events accumulate in `pending` during the 100ms debounce window. When the timer fires, the watcher processes all pending paths in one batch: reads the affected files, sends the appropriate messages, and clears the pending set.

The debounce timer resets on each new event. If events keep arriving, the window slides. Maximum slide: 500ms. After 500ms of continuous events, the watcher forces a flush regardless of whether new events are still arriving. This prevents starvation during rapid state changes (e.g., parallel execution writing many node states).

### Polling Fallback

A background ticker fires every 2 seconds. On each tick:

1. Stat the root index file. If mtime changed since last check, re-read and send `StateUpdatedMsg`.
2. Stat the instance registry directory. If mtime changed, re-read and send `InstancesUpdatedMsg`.
3. Stat the current log file. If size changed, read new lines and send `LogLinesMsg`.
4. Stat inbox.json. If mtime changed, re-read and send `InboxUpdatedMsg`.

The poller stores the last-seen mtime and file size for each path. It does not re-read if nothing changed. If fsnotify is working correctly, the poller is a no-op (all mtimes match).

### Log Tailing

The TUI maintains a byte offset into the current log file. On each read event (fsnotify or poll):

1. Open the file, seek to `logOffset`.
2. Read all available bytes.
3. Split by newline. Parse each complete line with `logrender.ParseRecord()`.
4. Append parsed lines to the circular buffer. If buffer exceeds 10,000 lines, drop the oldest.
5. Update `logOffset` to the new file position.
6. Send `LogLinesMsg` with the new lines.

On iteration rollover (new `.jsonl` file appears):
1. Send `NewLogFileMsg`.
2. Reset `logOffset` to 0.
3. Set `logFile` to the new path.
4. Insert a separator line in the buffer: `── iteration {N} ──`.

Incomplete lines (no trailing newline) are buffered internally and completed on the next read.

## Bubbletea Architecture

### Model Composition

```
TUIModel
├── HeaderModel          Phase 1
├── TreeModel            Phase 1
├── DetailModel          Phase 1 (container)
│   ├── DashboardModel   Phase 1
│   ├── NodeDetailModel  Phase 2
│   ├── TaskDetailModel  Phase 2
│   ├── LogViewModel     Phase 2
│   └── InboxModel       Phase 4
├── FooterModel          Phase 1
├── SearchModel          Phase 1
├── HelpOverlayModel     Phase 1
├── NotificationModel    Phase 5
└── WelcomeModel         Phase 1 (replaces all above when in StateWelcome)
```

Each sub-model implements:
```go
type SubModel interface {
    Update(tea.Msg) (SubModel, tea.Cmd)
    View() string // sub-models return string; only the top-level TUIModel returns tea.View
    SetSize(width, height int)
}
```

This is a convention, not a Go interface constraint. Bubbletea does not require a shared interface for sub-models. In practice, each concrete sub-model's `Update` method returns its own type (e.g., `func (m TreeModel) Update(msg tea.Msg) (TreeModel, tea.Cmd)`), and the parent casts as needed. The interface above is illustrative of the shape, not a literal type to declare.

### Message Flow

1. `tea.Program` delivers a `tea.Msg` to `TUIModel.Update()`.
2. For `tea.KeyPressMsg` only: `Ctrl+C` always triggers `tea.Quit`, regardless of overlay state. This is the emergency exit.
3. If the help overlay is active, it captures the key event. Only `?` and `Esc` dismiss it; all other keys are absorbed (no pass-through to global or pane handlers).
4. If the search bar is active, it captures the key event. `Esc` dismisses; `Enter` confirms; printable keys append to the query. No pass-through.
5. Otherwise, `TUIModel` checks for global keys (quit, mode switches, focus cycling, search open, help toggle, copy, refresh). `Esc` at the global level dismisses the error bar if any errors are visible; if no errors, it passes through to the focused pane.
6. If no global key matched, the message routes to the focused pane's sub-model.
7. Data messages (`StateUpdatedMsg`, `NodeUpdatedMsg`, etc.) are always broadcast to all sub-models that care about them, regardless of overlay state. The header always gets state messages. The tree always gets state messages. The detail pane gets them only if relevant to the current mode.

### Focus Management

`TUIModel.focused` is a `FocusedPane` enum: `PaneTree` or `PaneDetail`.

`Tab` cycles between them. Visual indicator: the focused pane's border color changes to bright red (`lipgloss.Color("1")`); unfocused is dim gray (`lipgloss.Color("240")`).

When the tree is hidden (narrow terminal or `t` toggle), focus is locked to `PaneDetail`. `Tab` does nothing. When the tree becomes visible again, focus returns to whichever pane was last focused.

Overlay models (help, search) set a `TUIModel.overlayActive` flag. While true, all key events route to the overlay. The overlay clears the flag on dismiss.

### Responsive Sizing

On `tea.WindowSizeMsg`:

1. Store `width` and `height` on `TUIModel`.
2. Compute tree width: `max(24, width * 30 / 100)`. If `width < 60`, tree is hidden.
3. Detail width: `width - treeWidth - 1` (1 for the border divider). Full width if tree hidden.
4. Content height: `height - 3` (2 for header, 1 for footer). Or `height - 2` if header collapsed.
5. Propagate dimensions to all sub-models via `SetSize()`.

Each sub-model's `View()` respects its allocated dimensions. No sub-model renders wider than its allocation. Lipgloss `Width()` and `Height()` enforce truncation.

### Init

```go
func (m TUIModel) Init() tea.Cmd {
    return tea.Batch(
        m.detectEntryState,   // check for .wolfcastle, running daemon
        m.startWatcher,       // set up fsnotify
        m.startPoller,        // start 2s poll ticker
        m.loadInitialState,   // read root index, last log lines
    )
}
```

`detectEntryState` returns a `tea.Cmd` that:
1. Walks CWD upward looking for `.wolfcastle/`.
2. If not found, returns a message that sets `entryState = StateWelcome`.
3. If found, calls `instance.Resolve(cwd)`.
4. If no live instance, returns a message setting `entryState = StateCold`.
5. If live instance, returns a message setting `entryState = StateLive`.

The TUI renders immediately with placeholder content. Data populates asynchronously as the init commands complete.

## Performance

### Memory Budget

Target: 50MB under normal operation (trees under 100 nodes).

| Component | Budget | Strategy |
|-----------|--------|----------|
| Log line buffer | ~20MB | Circular buffer, 10,000 lines max, ~2KB avg per parsed line |
| Tree state (index + cached nodes) | ~5MB | Lazy loading: only expanded nodes cached |
| Viewport render buffers | ~10MB | Only visible content rendered |
| Runtime overhead | ~10MB | Bubbletea, lipgloss, fsnotify, Go runtime |
| Headroom | ~5MB | Spikes during bulk re-reads |

### Lazy Loading

Node states are loaded from disk only when:
- A node is expanded in the tree (Phase 1).
- A node's detail is viewed (Phase 2).
- The dashboard computes aggregate audit counts (loads all nodes, but only `AuditState` fields are retained; the rest is discarded after counting).

When a node is collapsed, its cached `NodeState` is eligible for eviction after 30 seconds. A background timer checks every 10 seconds and evicts expired entries. Re-expanding the node triggers a fresh read.

For trees with >100 nodes, the dashboard audit count computation reads nodes in batches of 20 with a 1ms sleep between batches to avoid blocking the event loop.

### Virtual Scrolling

The log stream and any list exceeding 500 items use virtual scrolling. Only the visible rows plus 20 rows above and below (overscan) are rendered. The `View()` method:

1. Computes visible window: `[scrollTop, scrollTop + visibleHeight + 40]`.
2. Renders only those rows.
3. Pads above and below with empty lines to maintain correct scroll position.

This keeps render time proportional to visible rows, not total rows.

### Startup Time

Target: interactive within 200ms.

Startup reads only:
1. `.wolfcastle/` detection (stat calls walking up CWD). ~1ms.
2. `instance.Resolve(cwd)` (read small JSON files). ~5ms.
3. `Store.ReadIndex()` (single JSON file). ~10ms for a 100-node tree.
4. First render with tree and dashboard skeleton. ~20ms.

All other data (node states, log files, audit counts) loads asynchronously after the first frame. The tree shows nodes with status glyphs from the index (which carries status); full node state populates detail views lazily.

### Large Trees (1000+ Nodes)

Trees over 500 nodes use incremental counters for header aggregate counts. Each `NodeUpdatedMsg` adjusts the delta: decrement old status count, increment new status count. No full rescan of the index.

fsnotify watch count: trees over 1000 nodes watch only the root index and currently expanded subtree's node files. Collapsed nodes rely on polling. This keeps watch counts under the Linux `inotify` default limit of 8192.

Tree rendering: the flat list is always fully computed (O(n) on index change), but `View()` renders only visible rows (virtual scrolling when total rows exceed visible height plus overscan).

## Package Structure

```
internal/tui/
    app.go              TUIModel: Init, Update, View, layout computation,
                        entry state detection, message routing, focus management.
                        Approximately 300 lines.

    keys.go             All key.Binding definitions grouped by context.
                        GlobalKeys, TreeKeys, LogKeys, InboxKeys, SearchKeys,
                        HelpKeys, WelcomeKeys structs.

    messages.go         All custom tea.Msg types. One file, all phases.
                        Types are grouped by phase with comments.

    styles.go           All lipgloss.Style definitions. Color palette constants.
                        Helper functions: glyphForStatus(NodeStatus) string,
                        colorForStatus(NodeStatus) lipgloss.Color.

    watcher.go          Watcher struct. fsnotify setup, debouncing, event
                        dispatch. Polling fallback. Log tailing with offset.
                        Approximately 250 lines.

internal/tui/header/
    model.go            HeaderModel. Update processes DaemonStatusMsg,
                        StateUpdatedMsg, InstancesUpdatedMsg, SpinnerTickMsg.
                        View renders two header lines.

internal/tui/tree/
    model.go            TreeModel. Flat list computation from RootIndex.
                        Cursor movement, expand/collapse, scroll tracking.
                        Cache management for expanded nodes.

    render.go           renderRow(treeRow, width, selected, isCurrentTarget) string.
                        Glyph rendering, indentation, truncation.

internal/tui/detail/
    model.go            DetailModel. Mode switching container. Routes Update
                        to active sub-view. View delegates to active sub-view.

    dashboard.go        DashboardModel. Progress bars, activity feed,
                        audit summary. Reads from StateUpdatedMsg and
                        LogLinesMsg for recent activity.

    nodedetail.go       NodeDetailModel (Phase 2). Renders NodeState fields.
                        Viewport for scrolling.

    taskdetail.go       TaskDetailModel (Phase 2). Renders Task fields.
                        Viewport for scrolling.

    logview.go          LogViewModel (Phase 2). Circular buffer, follow mode,
                        level/trace filtering, virtual scrolling.
                        Approximately 200 lines.

    inbox.go            InboxModel (Phase 4). Item list, text input,
                        add item mutation.

internal/tui/welcome/
    model.go            WelcomeModel. Directory browser, init flow.
                        Captures all input in StateWelcome.

internal/tui/search/
    model.go            SearchModel (Phase 1). Incremental pane-local search.
                        Match tracking, highlight rendering.

internal/tui/help/
    model.go            HelpOverlayModel (Phase 1). Grouped key binding display.
                        Scrollable. Captures input when active.

internal/tui/clipboard/
    osc52.go            OSC 52 clipboard copy (Phase 1). Encodes text as base64,
                        writes escape sequence to stdout. No platform detection.

internal/tui/notify/
    model.go            NotificationModel (Phase 5). Toast queue, rendering,
                        auto-dismiss timers. State diff comparison.

cmd/root.go             Modified: when no subcommand is given, launch the TUI.
                        The root command's RunE creates a state.Store, detects
                        the worktree directory, and starts the tea.Program.
```

File count: 18 Go files across 9 packages. Each file stays under 400 lines. If a file approaches that limit, split by responsibility (e.g., `logview.go` splits into `logview.go` + `logbuffer.go`).

## Voice and Personality

The TUI follows the Wolfcastle voice guide (`docs/agents/VOICE.md`). Intensity at "medium" for interface chrome, "dry" for error messages, "high" for headers and empty states.

### Every String the TUI Displays

Organized by location.

#### Header Strings

| Condition | Text |
|-----------|------|
| Product name | `WOLFCASTLE` (bold, always uppercase) |
| Daemon running | `hunting (PID {pid})` |
| Daemon draining | `draining (PID {pid})` |
| Daemon stopped (stale) | `presumed dead (stale PID {pid})` |
| Daemon stopped (clean) | `standing down` |
| Daemon status unknown | `status unknown` |
| Loading | `⠋` (spinner cycle, no text) |
| Instance count | `[{n} running]` |

#### Dashboard Strings

| Condition | Text |
|-----------|------|
| Section header | `MISSION BRIEFING` |
| Status label | `Status:` |
| Branch label | `Branch:` |
| Uptime label | `Uptime:` |
| Progress section | `Progress:` |
| Activity section | `Recent Activity:` |
| Audit section | `Audit:` |
| No activity, no logs | `No transmissions. The daemon has not spoken.` |
| All targets done | `All targets eliminated.` |
| All blocked | `Blocked on all fronts. Human intervention required.` |
| No nodes | `No targets. Feed the inbox.` |

#### Tree Strings

| Condition | Text |
|-----------|------|
| Node awaiting decomposition | `Awaiting decomposition.` |
| Empty tree (no index) | `No targets. Feed the inbox.` |

#### Log Strings

| Condition | Text |
|-----------|------|
| Pane header | `TRANSMISSIONS` |
| Level filter display | `Level: {all (unfiltered) | DEBUG and above | INFO and above | WARN and above | ERROR only}` |
| Trace filter display | `Trace: {all | exec | intake}` |
| Follow on | `[following]` |
| Follow off | `[paused]` |
| No log files | `No transmissions. The daemon has not spoken.` |
| Iteration separator | `── iteration {n} ──` |
| Log read error | `Transmissions intercepted. Unable to read log file.` |

#### Node Detail Strings

| Condition | Text |
|-----------|------|
| Read error | `Intelligence unavailable for {addr}. Run wolfcastle doctor.` |
| No audit data | `No audit data. The investigation has not begun.` |
| Node awaiting decomposition | `Awaiting decomposition.` |

#### Task Detail Strings

| Condition | Text |
|-----------|------|
| No block reason | `none` |
| No failures | `none` |

#### Inbox Strings

| Condition | Text |
|-----------|------|
| Pane header | `INBOX` |
| Empty inbox | `Inbox empty. The silence is temporary.` |
| Add prompt (inactive) | `Press [a] to add an item` |
| Read error | `Inbox unreadable. Run wolfcastle doctor.` |
| Write error (bar) | `Failed to write inbox. Another process may hold the lock.` |

#### Welcome Strings

| Condition | Text |
|-----------|------|
| Title | `WOLFCASTLE` |
| No project | `No project found in this directory.` |
| Init prompt | `Initialize in:` |
| Confirm hint | `[Enter to confirm]` |
| Navigation hint | `(Use arrows to browse, or type a path)` |
| Init in progress | `Initializing in {path}...` |
| Init failed | `Init failed: {error}` |

#### Search Strings

| Condition | Text |
|-----------|------|
| No matches | `No matches. Adjust your aim.` |
| Prompt | `/` (just the slash character as prompt) |

#### Help Strings

| Condition | Text |
|-----------|------|
| Title | `WOLFCASTLE KEY BINDINGS` |
| Dismiss hint | `Press ? or Esc to close.` |

#### Toast Strings (Phase 5)

| Event | Text |
|-------|------|
| Task complete | `Target eliminated: {addr}/{task}` |
| Task blocked | `Blocked: {addr}/{task}` |
| New node created | `New target acquired: {addr}` |
| Audit gap opened | `Gap opened: {node} {gap_type}` |
| Transient file error | `State file unreadable. Retrying.` |

#### Error Bar Strings

| Condition | Text |
|-----------|------|
| Corrupt state file | `State corruption detected: {filename}. Run wolfcastle doctor.` |
| Unreadable node | `Unreadable: {addr}/state.json. Run wolfcastle doctor.` |
| Permission denied | `Permission denied: {filename}.` |
| Daemon start failed | `Daemon failed to start: {error}` |
| Daemon stop timeout | `Daemon not responding. Try wolfcastle stop --force.` |
| Lock contention on start | `Another daemon is running in this worktree.` |
| No project on start | `No project found. Run wolfcastle init.` |
| Terminal too small | `Terminal too small.` |
| Overflow badge | `+{n} more errors` (when more than 3 errors are stacked) |

The error bar stacks up to 3 visible error messages. When more than 3 are present, a count badge shows the overflow: `+{n} more errors`. The doctor suggestion is the primary recovery path. The TUI does not attempt automatic repair. If the user runs `wolfcastle doctor` in another terminal and it fixes the file, the next fsnotify event or poll tick clears the corresponding error bar entry automatically.

#### Footer Strings

The footer only shows key binding hints, never prose. Format: `[{key}] {label}` with spaces between bindings.

| Binding | Label |
|---------|-------|
| `q` | `quit` |
| `d` | `dash` |
| `l` | `logs` |
| `i` | `inbox` |
| `t` | `tree` |
| `s` | `start` (when stopped) or `stop` (when running) |
| `S` | `stop all` |
| `/` | `search` |
| `?` | `help` |
| `Tab` | `focus` |
| `R` | `refresh` |
| `y` | `copy` |
| `<` `>` | `instance` |

#### Copy Confirmation

| Condition | Text |
|-----------|------|
| Success | `Copied.` |

## Error Handling (Complete Reference)

Every failure mode, what the user sees, and how recovery works.

### State File Errors

| Error | Detection | Display | Recovery |
|-------|-----------|---------|----------|
| Root index corrupt JSON | `Store.ReadIndex()` returns parse error | Error bar: `State corruption detected: state.json. Run wolfcastle doctor.` Last known good tree remains. | Auto-clears on next successful read (fsnotify or poll). User can press Esc to dismiss. |
| Node state corrupt JSON | `Store.ReadNode(addr)` returns parse error | Error bar: `Unreadable: {addr}/state.json. Run wolfcastle doctor.` | Same as above. |
| Inbox corrupt JSON | `Store.ReadInbox()` returns parse error | Inbox pane: `Inbox unreadable. Run wolfcastle doctor.` | Retry on next event. |
| File permission denied | Any `Store.Read*` returns permission error | Error bar: `Permission denied: {filename}.` | User fixes permissions. |
| File disappears mid-read | Read returns `os.ErrNotExist` | Treated as empty/default state. No error bar. | Normal behavior; file may be mid-atomic-write. |

### Instance Registry Errors

| Error | Detection | Display | Recovery |
|-------|-----------|---------|----------|
| Registry directory doesn't exist | `instance.List()` returns nil, nil | Normal cold-start behavior. No error. | Registry created on first daemon start. |
| Registry file corrupt | JSON parse fails in `instance.List()` | Entry skipped silently. | Stale file cleaned on next List() call if PID is dead. |
| PID check fails | `IsProcessRunning()` returns false | Stale entry removed. | Automatic cleanup. |

### Watcher Errors

| Error | Detection | Display | Recovery |
|-------|-----------|---------|----------|
| fsnotify init fails | Constructor returns error | No display. Debug log to stderr. Polling activates. | Polling fallback for TUI lifetime. |
| Watch add fails for path | `watcher.Add()` returns error | No display. That path uses polling. | Polling fallback for that path. |
| fsnotify event error | Error channel fires | No display. Logged to stderr. | Watcher continues; single errors don't kill it. |

### Daemon Control Errors (Phase 3)

| Error | Detection | Display | Recovery |
|-------|-----------|---------|----------|
| Start: lock contention | Start returns lock error | Error bar: `Another daemon is running in this worktree.` | User stops other daemon. |
| Start: missing .wolfcastle | Start returns init error | Error bar: `No project found. Run wolfcastle init.` | User runs init. |
| Start: generic failure | Start returns other error | Error bar: `Daemon failed to start: {error}` | User investigates. |
| Stop: SIGTERM timeout | Process alive after 5s | Error bar: `Daemon not responding. Try wolfcastle stop --force.` | User uses CLI. |
| Stop: PID already dead | `IsProcessRunning` false | Silent transition to cold-start. | Automatic. |

### Terminal Errors

| Error | Detection | Display | Recovery |
|-------|-----------|---------|----------|
| Terminal too small | `width < 20` or `height < 5` | Centered: `Terminal too small.` | Responds to resize. |
| Terminal resize during render | `tea.WindowSizeMsg` | Re-layout all panes. | Immediate. |
| OSC 52 not supported | Not detectable | Copy silently does nothing. | Documented limitation. |

## Open Questions

1. **Log buffer backward scroll.** The 10,000-line circular buffer means old lines are gone. Should the TUI support scrolling backward into the on-disk log file? This would require a reverse file reader that seeks backward from the current offset. Deferred: not in v0.1. The buffer is sufficient for live observation. Historical log browsing is `wolfcastle log`.

2. **Mouse support.** The original spec permanently excludes it. Revisit after v0.1 ships and we see usage patterns. Bubbletea supports mouse events; the cost is interaction surface complexity, not implementation.

3. **fsnotify on NFS/FUSE.** The 2-second polling fallback may feel slow on non-native filesystems. Should we detect the filesystem type and auto-lower the interval? Deferred: poll interval could be a config value in a future version.
