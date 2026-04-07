# TUI Phase 5: Notification Toasts

Read `common.md` first. Builds on Phase 1-4 models.

## Phase 5: Notification Toasts

Theme: polish the reactive feedback loop.

### Scope

- Toast notification system for state transitions.
- Notification queue and auto-dismiss.

### Notification Model

```go
type NotificationModel struct {
    toasts    []toast
    maxQueue  int           // 5
    width     int
}

type toast struct {
    text      string
    createdAt time.Time
    dismissed bool
}
```

#### Toast Rendering

Toasts appear in the upper-right corner of the detail pane, overlaying content briefly before fading. Styled as white text on dark gray background (`lipgloss.Color("236")`) with a red left border (`lipgloss.Color("1")`). Maximum width: 50 characters. Text wraps if longer.

Toasts auto-dismiss after 3 seconds. The queue holds up to 5 pending notifications; older ones are dropped if the queue overflows.

#### Toast Trigger Rules

State diffs drive the notification system. When a `StateUpdatedMsg` or `NodeUpdatedMsg` arrives, the TUI compares the new state against the previous snapshot:

- Task status changed to `complete`: `Target eliminated: {addr}/{task}`
- Task status changed to `blocked`: `Blocked: {addr}/{task}`
- New node appeared in `RootIndex.Nodes`: `New target acquired: {addr}`
- Audit gap opened: `Gap opened: {node} {gap_type}`

The diff is lightweight: compare status fields and counts, not deep equality on the full state.

#### Toast Messages

```go
// ToastMsg carries a notification to display.
type ToastMsg struct {
    Text string
}

// ToastDismissMsg signals a toast's 3-second timer expired.
type ToastDismissMsg struct {
    Index int
}
```

### Test Cases and Acceptance Criteria

#### Acceptance Criteria

1. A toast notification appears when a task's status changes to `complete`, displaying `Target eliminated: {addr}/{task}`.
2. A toast notification appears when a task's status changes to `blocked`, displaying `Blocked: {addr}/{task}`.
3. A toast notification appears when a new node appears in `RootIndex.Nodes`, displaying `New target acquired: {addr}`.
4. A toast notification appears when an audit gap opens, displaying `Gap opened: {node} {gap_type}`.
5. Toasts render in the upper-right corner of the detail pane as white text on dark gray background with a red left border.
6. Each toast auto-dismisses after 3 seconds.
7. The toast queue holds a maximum of 5 notifications; when overflow occurs, the oldest pending toast is dropped.
8. State diff detection compares status fields and counts (lightweight comparison, not deep equality) to determine which toasts to fire.
9. Multiple state changes in a single `StateUpdatedMsg` produce multiple toasts (one per change).
10. Toast text wraps at 50 characters maximum width.
11. Toasts do not block interaction; all input continues to route normally while toasts are visible.

#### Test Cases

| ID | Description | Setup | Action | Expected Result |
|----|-------------|-------|--------|-----------------|
| P5-001 | Toast on task completion | Task `auth/login/task-0003` status was `in_progress` | `NodeUpdatedMsg` arrives with task status `complete` | Toast appears: `Target eliminated: auth/login/task-0003` in upper-right of detail pane |
| P5-002 | Toast on task blocked | Task `api/rate-limit/task-0001` status was `in_progress` | `NodeUpdatedMsg` arrives with task status `blocked` | Toast appears: `Blocked: api/rate-limit/task-0001` |
| P5-003 | Toast on new node | `RootIndex` had 5 nodes | `StateUpdatedMsg` arrives with 6 nodes (new `billing` node) | Toast appears: `New target acquired: billing` |
| P5-004 | Toast on audit gap | Node `api-gateway` had 0 open gaps | `NodeUpdatedMsg` arrives with 1 open gap of type `rate_limiting` | Toast appears: `Gap opened: api-gateway rate_limiting` |
| P5-005 | Toast auto-dismiss after 3 seconds | Toast `Target eliminated: auth/login/task-0003` displayed | Wait 3 seconds | `ToastDismissMsg` fires; toast disappears from view |
| P5-006 | Toast auto-dismiss timing | Toast created at T=0 | Check at T=2.9s and T=3.1s | Still visible at 2.9s; gone at 3.1s |
| P5-007 | Toast queue limit 5 | 3 toasts already visible | 4 new state changes arrive simultaneously (would make 7 total) | Queue drops the 2 oldest; only 5 toasts in the queue |
| P5-008 | Toast queue overflow drops oldest | Queue at 5 toasts, oldest is "Target eliminated: X" | New toast "Blocked: Y" arrives | "Target eliminated: X" is dropped; "Blocked: Y" is added; queue still at 5 |
| P5-009 | Multiple toasts from single update | Two tasks change status in one `StateUpdatedMsg`: one to `complete`, one to `blocked` | Process the message | Two toasts appear: one `Target eliminated` and one `Blocked` |
| P5-010 | Toast rendering position | Detail pane width 60, height 20 | Toast appears | Toast renders in upper-right corner of detail pane, overlaying content |
| P5-011 | Toast styling | Toast with text "Target eliminated: auth/task-0001" | Render | White text, dark gray background (`Color("236")`), red left border (`Color("1")`) |
| P5-012 | Toast wraps long text | Toast text is 70 characters | Render | Text wraps within 50-character maximum width; toast grows vertically |
| P5-013 | Toast does not block input | Toast visible, tree focused | Press `j` to move cursor | Cursor moves normally; toast remains visible until its timer expires |
| P5-014 | State diff detects no change | `StateUpdatedMsg` arrives with identical state (no status changes, same node count) | Process the message | No toasts fired |
| P5-015 | State diff lightweight comparison | Large tree with 200 nodes, 3 tasks changed status | Process `StateUpdatedMsg` | Exactly 3 toasts fired; diff did not perform deep equality on all 200 nodes |
| P5-016 | Toast dismissed flag | Toast auto-dismissed | Check toast struct | `dismissed` field is `true`; toast no longer renders |
| P5-017 | Rapid state changes | 10 state updates arrive within 1 second, each with 1 task completion | Process all updates | 5 most recent toasts in queue (oldest 5 dropped due to overflow); each auto-dismisses after 3 seconds from its creation |

### Help Overlay Content (Complete)

The help overlay (Phase 1) grows its content as phases land. Here is the complete help text after all five phases:

```
WOLFCASTLE KEY BINDINGS

  Global
  ──────────────────────────────
  q, Ctrl+C     Quit
  d              Dashboard
  l              Log stream
  i              Inbox
  t              Toggle tree
  s              Start/stop daemon
  S              Stop all instances
  Tab            Switch focus
  /              Search
  y              Copy to clipboard
  R              Force refresh
  <, >           Switch instance
  ?              This screen

  Tree Navigation
  ──────────────────────────────
  j, k           Move up/down
  Enter, l       Expand / view detail
  Esc, h         Collapse / parent
  g, G           Top / bottom

  Log Stream
  ──────────────────────────────
  f              Toggle follow
  L              Cycle level filter
  T              Cycle trace filter
  Ctrl+D/Ctrl+U  Half page down/up

  Inbox
  ──────────────────────────────
  a              Add item
  Enter          Submit / view item
  Esc            Cancel input

  Search
  ──────────────────────────────
  n, N           Next / previous match

Press ? or Esc to close.
```

