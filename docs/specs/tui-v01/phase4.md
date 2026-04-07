# TUI Phase 4: Inbox

Read `common.md` first. Builds on Phase 1-3 models.

## Phase 4: Inbox

Theme: the first write operation.

### Scope

- Inbox view: list items, show status, add new items.
- Extend pane-local search (Phase 1) to the inbox pane.

### Inbox Model

```go
type InboxModel struct {
    items      []state.InboxItem
    cursor     int
    viewport   viewport.Model
    input      textinput.Model    // for adding new items
    inputMode  bool               // true when text input is active
    width      int
    height     int
    focused    bool
}
```

#### Inbox Rendering

```
INBOX  {count} items

  ○ Buy milk                                    new     2m ago
  ● Add rate limiting to API gateway            filed   1h ago
  ○ Investigate flaky test in auth module       new     3h ago

──────────────────────────────────────────────────────
> Add new item: _
```

Item glyphs: `○` for `new` (dim white), `●` for `filed` (green).

The text input appears at the bottom of the pane, always visible. When not in input mode, it shows `Press [a] to add an item`. When in input mode, it shows the blinking cursor with typed text.

The item list is scrollable. Selected item gets the same highlight style as the tree (bold white on dark gray).

#### Inbox Data Sources

- `Store.ReadInbox()` for the item list.
- `Store.MutateInbox()` to append a new item. The mutation adds an `InboxItem` with `Timestamp: time.Now().UTC().Format(time.RFC3339)`, `Text: input.Value()`, `Status: state.InboxNew`.

#### Inbox Messages

```go
// InboxUpdatedMsg carries a fresh InboxFile.
type InboxUpdatedMsg struct {
    Inbox *state.InboxFile
}

// InboxItemAddedMsg confirms a new item was written.
type InboxItemAddedMsg struct{}

// InboxAddFailedMsg indicates the write failed.
type InboxAddFailedMsg struct {
    Err error
}
```

#### Inbox Key Bindings

| Key | Context | Action |
|-----|---------|--------|
| `i` | Global | Switch detail to inbox mode |
| `a` | Inbox, not in input mode | Activate text input |
| `Enter` | Inbox, in input mode | Submit new item, clear input |
| `Esc` | Inbox, in input mode | Cancel input, deactivate |
| `j`, `↓` | Inbox, not in input mode | Move cursor down |
| `k`, `↑` | Inbox, not in input mode | Move cursor up |

#### Inbox Error Handling

| Failure | User sees | Recovery |
|---------|-----------|----------|
| `Store.ReadInbox()` fails | Inbox pane shows `Inbox unreadable. Run wolfcastle doctor.` | Retry on next poll/fsnotify. |
| `Store.MutateInbox()` fails (lock timeout) | Error bar: `Failed to write inbox. Another process may hold the lock.` | User retries. |
| `Store.MutateInbox()` fails (disk) | Error bar: `Inbox write failed: {error}.` | User retries. |
| Empty inbox | Pane shows `Inbox empty. The silence is temporary.` | Normal state, no action needed. |

### Phase 4 Search Extension

Pane-local search (Phase 1) already works in the tree pane. Phase 4 extends search to the inbox pane, matching against item text and status.

### Phase 4 Error Handling

| Failure | User sees | Recovery |
|---------|-----------|----------|
| Inbox search with no matches | Search bar: `No matches. Adjust your aim.` | Normal. |

### Test Cases and Acceptance Criteria

#### Acceptance Criteria

1. Pressing `i` switches the detail pane to inbox mode (`ModeInbox`).
2. The inbox view renders a header `INBOX  {count} items` and lists all items with status glyph, text, status label, and relative timestamp.
3. Item glyph `○` (dim white) appears for `new` items; `●` (green) appears for `filed` items.
4. The selected item row uses bold white on dark gray highlight (same as tree selection style).
5. The text input prompt `Press [a] to add an item` appears at the bottom of the pane when not in input mode.
6. Pressing `a` activates input mode with a blinking cursor.
7. Pressing `Enter` in input mode submits the new item via `Store.MutateInbox()` with `Status: InboxNew` and current UTC timestamp, then clears the input and deactivates input mode.
8. Pressing `Esc` in input mode cancels input and deactivates input mode without writing.
9. `j`/`↓` and `k`/`↑` navigate the item list when not in input mode.
10. The inbox updates live when `InboxUpdatedMsg` fires (via fsnotify on `inbox.json`).
11. An empty inbox displays `Inbox empty. The silence is temporary.`
12. A failed inbox read shows `Inbox unreadable. Run wolfcastle doctor.` in the pane.
13. A failed inbox write shows error bar text `Failed to write inbox. Another process may hold the lock.`
14. Pane-local search (`/`) works in the inbox, matching against item text and status.

#### Test Cases

| ID | Description | Setup | Action | Expected Result |
|----|-------------|-------|--------|-----------------|
| P4-001 | Inbox view renders items | 3 inbox items: 2 new, 1 filed | Press `i` | Detail pane shows `INBOX  3 items`; items listed with correct glyphs (`○` for new, `●` for filed), text, status, and relative timestamps |
| P4-002 | Inbox item selection | 3 items in inbox | Press `j` twice | Cursor moves to item 3; item 3 row has bold white on dark gray highlight |
| P4-003 | Inbox cursor bounds | 3 items, cursor at item 3 | Press `j` | Cursor stays at item 3 (clamped) |
| P4-004 | Add item activate input | Inbox visible, not in input mode | Press `a` | Input mode activates; text input field appears with blinking cursor; `Press [a] to add an item` prompt replaced by input field |
| P4-005 | Add item submit | Input mode active, user typed "Fix the login redirect bug" | Press `Enter` | `Store.MutateInbox()` called with text "Fix the login redirect bug", status `InboxNew`, RFC3339 timestamp; input clears; input mode deactivates; item appears in list |
| P4-006 | Add item cancel | Input mode active, user typed partial text | Press `Esc` | Input mode deactivates; typed text discarded; no mutation; prompt reverts to `Press [a] to add an item` |
| P4-007 | Add item empty submit | Input mode active, input field is empty | Press `Enter` | No mutation occurs; input mode remains active (or deactivates with no write); no empty item created |
| P4-008 | Inbox live update via fsnotify | Inbox showing 3 items | External process adds item to `inbox.json`, fsnotify fires | `InboxUpdatedMsg` delivers new inbox state; view updates to show 4 items |
| P4-009 | Empty inbox display | No items in inbox file | Press `i` | Pane shows `Inbox empty. The silence is temporary.`; add prompt still visible at bottom |
| P4-010 | Inbox read failure | `Store.ReadInbox()` returns parse error | Press `i` | Pane shows `Inbox unreadable. Run wolfcastle doctor.` |
| P4-011 | Inbox write failure lock timeout | `Store.MutateInbox()` returns lock timeout error | Submit new item | Error bar: `Failed to write inbox. Another process may hold the lock.`; item not added to list |
| P4-012 | Inbox write failure disk error | `Store.MutateInbox()` returns disk I/O error | Submit new item | Error bar: `Inbox write failed: {error}.` |
| P4-013 | Search in inbox | 5 inbox items, 2 contain "auth" | Press `/`, type `auth` | 2 items highlighted in yellow; search bar shows `/auth  2/2 matches` |
| P4-014 | Search no matches in inbox | 5 inbox items, none contain "xyz" | Press `/`, type `xyz` | Search bar shows `No matches. Adjust your aim.` |
| P4-015 | Inbox item timestamp rendering | Item added 2 minutes ago | Render inbox | Item row shows `2m ago` as the timestamp |
| P4-016 | Scrollable inbox | 30 inbox items, viewport height 20 | Scroll down with `j` | Items scroll; items 21-30 become visible as cursor moves past visible area |

