# TUI Phase 2: Log Stream, Node Detail, Task Detail

Read `common.md` first. Builds on Phase 1 models.

## Phase 2: Log Stream, Node Detail, Task Detail

Theme: the detail pane becomes useful. You can see what the daemon is doing and inspect any node or task.

### Scope

- Log stream view: live tailing, follow mode, manual scroll, level filtering, trace filtering.
- Node detail view: full node state, audit info, children, specs.
- Task detail view: task state, breadcrumbs, block reason, deliverables, failure info.
- Transition from tree selection to detail view on `Enter`.

### Log View Model

```go
type LogViewModel struct {
    lines       []logLine         // circular buffer, max 10000
    viewport    viewport.Model
    follow      bool              // auto-scroll to bottom
    levelFilter string            // "all", "debug", "info", "warn", "error"
    traceFilter string            // "all", "exec", "intake"
    logFile     string            // current file being tailed
    fileOffset  int64             // byte offset into current file
    width       int
    height      int
    focused     bool
}

type logLine struct {
    record    logrender.Record
    rendered  string             // pre-rendered display string (cached)
}
```

#### Log Data Sources

- `logging.LatestLogFile(logDir)` to find the current file. `logDir` is `{worktreeDir}/.wolfcastle/system/logs/`.
- Read from file offset, parse each line with `logrender.ParseRecord(line)`.
- On first load, seek to the end of file, read backward to find the last 50 lines, then stream forward.
- On `NewLogFileMsg` (new file appears in logs dir), print a separator `── iteration {N} ──` and switch to the new file from offset 0.

#### Log Rendering

Each log line renders as:

```
{timestamp} {trace_prefix} {formatted_content}
```

- `timestamp`: `record.Timestamp.Format("15:04:05")` in dim white.
- `trace_prefix`: `[{record.Trace}]` in cyan if present, omitted if empty.
- `formatted_content`: varies by `record.Type`:

| `record.Type` | Format | Color |
|---------------|--------|-------|
| `stage_start` | `[{Stage}] Starting {Node}/{Task}` | white |
| `stage_complete` | `[{Stage}] Complete (exit={ExitCode})` | green if exit 0, yellow otherwise |
| `stage_error` | `[{Stage}] Error: {Error}` | red |
| `assistant` | `{Text}` (verbatim, may be multi-line) | white |
| `failure_increment` | `[failure] {Node}/{Task} failure #{Counter}` | yellow |
| `auto_block` | `[blocked] {Node}/{Task}: {Reason}` | red, bold |
| `daemon_start` | `Daemon started` | white, bold |
| `daemon_lifecycle` | `[lifecycle] {Event}` | dim white |
| (unrecognized) | `[{Type}] {raw JSON}` | dim white |

Level-based coloring applies to the entire line as a tint:
- `debug`: dim (lipgloss `Color("240")`)
- `info`: normal
- `warn`: yellow tint
- `error`: red tint

Follow mode: when `follow` is true, the viewport auto-scrolls to the bottom on each new batch of lines. Scrolling up (`k`, `↑`, `PgUp`) disables follow. `f` toggles follow back on.

The log pane header (first line of the viewport area) shows:
```
TRANSMISSIONS  Level: {filter}  Trace: {filter}  {follow_indicator}
```
Where `follow_indicator` is `[following]` in green when follow is on, or `[paused]` in yellow when off.

#### Log Messages

```go
// LogLinesMsg carries a batch of new parsed log lines.
type LogLinesMsg struct {
    Lines []logLine
}

// NewLogFileMsg signals that a new log file appeared (iteration rollover).
type NewLogFileMsg struct {
    Path string
}
```

#### Log Key Bindings

| Key | Action |
|-----|--------|
| `f` | Toggle follow mode |
| `j`, `↓` | Scroll down one line. Disables follow if not at bottom |
| `k`, `↑` | Scroll up one line. Disables follow |
| `Ctrl+D`, `PgDn` | Scroll down half page |
| `Ctrl+U`, `PgUp` | Scroll up half page |
| `G` | Jump to bottom, enable follow |
| `g` | Jump to top, disable follow |
| `L` | Cycle level filter: all -> debug -> info -> warn -> error -> all |
| `T` | Cycle trace filter: all -> exec -> intake -> all |

Filtering is display-side only. Filtered-out lines remain in the buffer; changing the filter re-renders from the buffer without re-reading the file.

### Node Detail Model

```go
type NodeDetailModel struct {
    addr       string
    node       *state.NodeState
    index      *state.IndexEntry
    viewport   viewport.Model
    width      int
    height     int
}
```

#### Node Detail Rendering

```
{name}  {status_glyph} {status}
Type: {orchestrator|leaf}    Depth: {decomposition_depth}

Scope:
  {scope text, wrapped to pane width}

Success Criteria:
  • {criteria[0]}
  • {criteria[1]}

Children:                          (orchestrator only)
  {child_glyph} {child_name}  {child_status_glyph}
  ...

Tasks:                             (leaf only)
  {task_glyph} {task_id}: {title}
  ...

Audit: {audit_status}
  Started: {relative_time}
  Completed: {relative_time or "in progress"}
  Gaps: {open_count} open, {fixed_count} fixed
  Escalations: {open_count} open
  Summary: {result_summary or "none yet"}

Specs:
  {spec_path[0]}
  {spec_path[1]}
```

All timestamps render as relative time (`3m ago`, `2h ago`, `yesterday`). The exact timestamp appears in parentheses after the relative time for times older than 1 hour: `2h ago (14:32:01)`.

#### Node Detail Data Sources

- `Store.ReadNode(addr)` for the full `NodeState`.
- `RootIndex.Nodes[addr]` for the `IndexEntry`.

### Task Detail Model

```go
type TaskDetailModel struct {
    addr       string           // parent node address
    taskID     string
    task       *state.Task
    viewport   viewport.Model
    width      int
    height     int
}
```

#### Task Detail Rendering

```
{task_id}  {status_glyph} {status}
{title}

{description, wrapped to pane width}

Body:
  {body text, if present}

Class: {class or "default"}
Type: {task_type or "standard"}

Deliverables:
  • {deliverable[0]}
  • {deliverable[1]}

Acceptance Criteria:
  • {criteria[0]}

Constraints:
  • {constraint[0]}

References:
  {reference[0]}

Breadcrumbs: {breadcrumbs[0]}, {breadcrumbs[1]}, ...

Block Reason: {block_reason or "none"}
Failures: {failure_count}
Last Failure: {last_failure_type or "none"}
Needs Decomposition: {yes/no}
Is Audit: {yes/no}
```

`Breadcrumbs` is the `Task.Breadcrumbs` field, which is a `[]string` (plain text tags). These render as a comma-separated inline list. This is distinct from audit breadcrumbs (see node detail Audit section), which are `[]Breadcrumb` structs with timestamp, task, and text fields.

Sections with empty data are omitted entirely (no blank "Deliverables:" with nothing under it).

#### Task Detail Data Source

- The `Task` struct from the parent `NodeState.Tasks` slice, matched by `task.ID`.

### Phase 2 Search Extension

Pane-local search (Phase 1) extends to all new Phase 2 views. In the log stream, search matches against rendered log line text. In node detail and task detail, search matches against the rendered text content. Same highlighting and `n`/`N` navigation as the tree pane search.

### Phase 2 Clipboard Extension

Clipboard copy (`y`) extends to Phase 2 views:
- Log line selected (cursor on a log line in the log stream): copies the raw JSON of that line.
- Node detail visible: copies the node address.
- Task detail visible: copies the task address.

### Phase 2 Additional Key Bindings

| Key | Context | Action |
|-----|---------|--------|
| `l` | Global | Switch detail to log stream mode |
| `Enter` | Tree, on orchestrator | Load node detail into detail pane |
| `Enter` | Tree, on task | Load task detail into detail pane |
| `Esc` | Detail (node or task) | Return to dashboard |

### Phase 2 Error Handling

| Failure | User sees | Recovery |
|---------|-----------|----------|
| Log file read fails | Log pane shows `Transmissions intercepted. Unable to read log file.` | Retry on next poll/fsnotify. |
| Log file parse error (malformed JSON line) | Line skipped silently. Malformed lines do not break the stream. | Next valid line renders normally. |
| `Store.ReadNode(addr)` fails for detail | Detail pane shows `Intelligence unavailable for {addr}. Run wolfcastle doctor.` | Retry on next poll/fsnotify event for that node. |
| No log files exist | Log pane shows `No transmissions. The daemon has not spoken.` | Watcher monitors log directory for new files. |

### Test Cases and Acceptance Criteria

#### Acceptance Criteria

1. Pressing `l` switches the detail pane to log stream mode.
2. The log stream renders lines with timestamp, trace prefix, and formatted content in the correct colors per `record.Type`.
3. Follow mode is enabled by default; new log lines auto-scroll the viewport to the bottom.
4. Scrolling up (`k`, `↑`, `PgUp`) disables follow mode; the `[following]` indicator changes to `[paused]`.
5. Pressing `f` re-enables follow mode and scrolls to the bottom.
6. Pressing `G` jumps to the bottom and enables follow; pressing `g` jumps to the top and disables follow.
7. `L` cycles the level filter through `all -> debug -> info -> warn -> error -> all`; filtered-out lines are hidden without re-reading the file.
8. `T` cycles the trace filter through `all -> exec -> intake -> all`; filtered-out lines are hidden without re-reading the file.
9. The log pane header shows `TRANSMISSIONS  Level: {filter}  Trace: {filter}  {follow_indicator}` with correct values.
10. When no log files exist, the log pane shows `No transmissions. The daemon has not spoken.`
11. A new log file appearing (iteration rollover) inserts a separator `── iteration {N} ──` and switches tailing to the new file.
12. On first render, the log view loads the last 50 lines from the current log file (seeking backward from end).
13. Malformed JSON log lines are skipped silently; the stream continues with the next valid line.
14. Pressing `Enter` on an orchestrator node in the tree loads node detail into the detail pane.
15. Pressing `Enter` on a task row in the tree loads task detail into the detail pane.
16. Node detail displays name, status glyph, type, depth, scope, success criteria, children (orchestrator) or tasks (leaf), audit section, and specs.
17. Task detail displays task ID, status, title, description, body, class, type, deliverables, acceptance criteria, constraints, references, breadcrumbs, block reason, failure count, last failure type, needs decomposition flag, and is audit flag.
18. Sections with empty data are omitted from task detail rendering (no empty headers).
19. The current target indicator (`▶`) appears on the active node in both tree and node detail views.
20. All timestamps in node detail render as relative time with exact time in parentheses for times older than 1 hour.
21. `Esc` from node or task detail returns to the dashboard view.
22. Clipboard copy (`y`) in log stream copies the raw JSON of the selected line; in node detail copies the node address; in task detail copies the task address.
23. Pane-local search works in log stream (matching rendered text), node detail, and task detail.
24. A log file read failure shows `Transmissions intercepted. Unable to read log file.` and retries on next event.
25. A node read failure in detail view shows `Intelligence unavailable for {addr}. Run wolfcastle doctor.`

#### Test Cases

| ID | Description | Setup | Action | Expected Result |
|----|-------------|-------|--------|-----------------|
| P2-001 | Log stream renders stage_start | Log file contains a `stage_start` record for `auth/login/task-0001` | Switch to log view | Line renders as `{HH:MM:SS} [{trace}] [{stage}] Starting auth/login/task-0001` in white |
| P2-002 | Log stream renders stage_complete exit 0 | Log contains `stage_complete` with `ExitCode: 0` | Render log view | Line renders in green with `[{stage}] Complete (exit=0)` |
| P2-003 | Log stream renders stage_complete exit 1 | Log contains `stage_complete` with `ExitCode: 1` | Render log view | Line renders in yellow with `Complete (exit=1)` |
| P2-004 | Log stream renders stage_error | Log contains `stage_error` with error text | Render log view | Line renders in red with `[{stage}] Error: {error text}` |
| P2-005 | Log stream renders auto_block | Log contains `auto_block` record | Render log view | Line renders in red, bold: `[blocked] {node}/{task}: {reason}` |
| P2-006 | Log stream renders assistant text | Log contains `assistant` type with multi-line text | Render log view | Text renders verbatim in white, preserving line breaks |
| P2-007 | Log stream renders unrecognized type | Log contains a record with type `custom_event` | Render log view | Line renders as `[custom_event] {raw JSON}` in dim white |
| P2-008 | Follow mode auto-scrolls | Follow enabled, viewport at bottom | 5 new log lines arrive via `LogLinesMsg` | Viewport scrolls to show new lines; last line visible at bottom |
| P2-009 | Scroll up disables follow | Follow enabled | Press `k` | Viewport scrolls up one line; follow indicator changes to `[paused]` in yellow |
| P2-010 | Press f re-enables follow | Follow disabled, viewport scrolled to middle | Press `f` | Viewport jumps to bottom; follow indicator changes to `[following]` in green |
| P2-011 | Half-page scroll down | Viewport visible height 20, total 100 lines | Press `Ctrl+D` | Viewport scrolls down 10 lines; follow disabled if not at bottom |
| P2-012 | Half-page scroll up | Viewport at line 50, visible height 20 | Press `Ctrl+U` | Viewport scrolls up 10 lines; follow disabled |
| P2-013 | Level filter cycle | Level filter at `all` | Press `L` three times | Filter cycles: `all` -> `debug` -> `info` -> `warn`; only lines at or above current level shown |
| P2-014 | Level filter hides lines | 10 log lines: 3 debug, 4 info, 2 warn, 1 error; level filter set to `warn` | Render | Only 3 lines visible (2 warn + 1 error); other lines hidden |
| P2-015 | Trace filter cycle | Trace filter at `all` | Press `T` twice | Filter cycles: `all` -> `exec` -> `intake`; only matching trace lines shown |
| P2-016 | Filter display-side only | Filter changed from `all` to `error`, then back to `all` | Change filter twice | All 10 original lines reappear; no file re-read occurred |
| P2-017 | Log pane header text | Level `info`, trace `all`, follow on | Render log pane | First line: `TRANSMISSIONS  Level: INFO and above  Trace: all  [following]` |
| P2-018 | No log files | Log directory empty | Switch to log view | Pane shows `No transmissions. The daemon has not spoken.` |
| P2-019 | Iteration rollover | Tailing `iteration-001.jsonl`, new file `iteration-002.jsonl` appears | fsnotify create event on log directory | Separator `── iteration 2 ──` inserted in buffer; tailing switches to new file from offset 0 |
| P2-020 | Historical log lines on first render | Log file has 200 lines | Open log view for the first time | Last 50 lines loaded and displayed; viewport positioned at bottom with follow enabled |
| P2-021 | Malformed JSON line skipped | Log file has 10 lines, line 5 is `{invalid json` | Read log lines | Lines 1-4 and 6-10 render normally; line 5 is silently skipped |
| P2-022 | Node detail from tree Enter | Tree focused, cursor on orchestrator `auth-system` | Press `Enter` | Detail pane switches to `ModeNodeDetail`; shows `auth-system` with status glyph, type `orchestrator`, children listed |
| P2-023 | Task detail from tree Enter | Tree focused, cursor on task `auth-system/login/task-0003` | Press `Enter` | Detail pane switches to `ModeTaskDetail`; shows task ID, status, title, description, breadcrumbs |
| P2-024 | Node detail all fields | Node has scope, 3 success criteria, 2 children, audit with 1 gap, 2 specs | Render node detail | All fields render: scope wrapped to pane width, criteria bulleted, children with glyphs, audit section with gap count, specs listed |
| P2-025 | Task detail omits empty sections | Task has no deliverables, no constraints, no references, no body | Render task detail | No `Deliverables:`, `Constraints:`, `References:`, or `Body:` headers appear |
| P2-026 | Task detail breadcrumbs | Task has breadcrumbs `["auth", "oauth", "login"]` | Render task detail | `Breadcrumbs: auth, oauth, login` rendered as comma-separated inline list |
| P2-027 | Task detail block reason | Task is blocked with reason "Waiting for API schema" | Render task detail | `Block Reason: Waiting for API schema` displayed |
| P2-028 | Task detail failure count | Task has failure count 3, last failure type "exec_timeout" | Render task detail | `Failures: 3` and `Last Failure: exec_timeout` displayed |
| P2-029 | Current target in node detail | Daemon currently targeting `auth-system/login` | View node detail for `auth-system/login` | `▶` prefix appears before the node name in the detail header |
| P2-030 | Relative timestamp rendering | Audit started 45 minutes ago | Render node detail audit section | `Started: 45m ago` (no parenthesized exact time since < 1 hour) |
| P2-031 | Relative timestamp with exact time | Audit started 3 hours ago at 11:32:01 | Render node detail audit section | `Started: 3h ago (11:32:01)` |
| P2-032 | Esc returns to dashboard | Detail pane in `ModeNodeDetail` | Press `Esc` | Detail pane switches to `ModeDashboard`; dashboard renders |
| P2-033 | Clipboard copy in log stream | Log stream focused, cursor on a log line | Press `y` | OSC 52 sequence written with base64-encoded raw JSON of that line |
| P2-034 | Clipboard copy in node detail | Node detail showing `auth-system/login` | Press `y` | OSC 52 sequence written with base64-encoded `auth-system/login` |
| P2-035 | Search in log stream | Log stream with 50 lines, 3 contain "error" | Press `/`, type `error` | 3 lines highlighted in yellow; search bar shows `/error  3/3 matches` |
| P2-036 | Log file read failure | Log file becomes unreadable (permissions changed) | TUI attempts to read new lines | Pane shows `Transmissions intercepted. Unable to read log file.`; retries on next poll/fsnotify |
| P2-037 | Node detail read failure | `Store.ReadNode("auth")` returns error | Press `Enter` on `auth` node | Detail pane shows `Intelligence unavailable for auth. Run wolfcastle doctor.` |
| P2-038 | Circular buffer overflow | Buffer at 10,000 lines | 100 new lines arrive | Oldest 100 lines dropped; buffer stays at 10,000; newest lines visible |
| P2-039 | Level coloring tint | Debug log line | Render | Entire line rendered in dim color (`Color("240")`) |
| P2-040 | Incomplete log line buffering | File read ends mid-line (no trailing newline) | Read event fires | Partial line buffered internally; completes and renders on next read that provides the newline |

