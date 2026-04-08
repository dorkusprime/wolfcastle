# TUI Phase 3: Instance Switcher, Daemon Control

Read `common.md` first. Builds on Phase 1 and 2 models.

## Phase 3: Instance Switcher, Daemon Control

Theme: the TUI can see all instances and start/stop the daemon.

### Scope

- Header instance switcher when multiple instances running.
- `s` to start daemon (cold-start) or stop daemon (running).
- `S` to stop all instances.
- Transitions between entry states when daemon starts/stops.

### Instance Switcher

When `len(instances) > 1`, the header line 1 becomes interactive. The `[N running]` badge is always shown. `<` and `>` keys (or `1`-`9` number keys) switch the active instance.

Switching the active instance:
1. Update `store` to point to the new instance's worktree `.wolfcastle/projects/{namespace}`.
2. Re-read the root index.
3. Reset the tree (collapse all, cursor to top).
4. Reset the detail pane to dashboard.
5. Restart log tailing from the new instance's log directory.
6. Update the watcher to monitor the new instance's paths.

The header shows all instances as a tab bar when terminal is wide enough (>100 cols):

```
WOLFCASTLE v0.5.3  [feat/auth ●] [fix/login] [main]   3 running
```

Active instance gets a `●` marker and bold text. Inactive instances are dim. Below 100 cols, only the active instance and the count badge are shown.

#### Instance Switcher Messages

```go
// SwitchInstanceMsg requests switching to a different instance.
type SwitchInstanceMsg struct {
    Entry instance.Entry
}

// InstanceSwitchedMsg confirms the switch completed and carries new state.
type InstanceSwitchedMsg struct {
    Index  *state.RootIndex
    Entry  instance.Entry
}
```

### Daemon Control

| Key | Current State | Action |
|-----|--------------|--------|
| `s` | No daemon running | Start daemon in background. Equivalent to `wolfcastle start -d`. |
| `s` | Daemon running | Stop daemon. Equivalent to `wolfcastle stop`. |
| `S` | Any | Stop all instances. Equivalent to `wolfcastle stop --all`. |

Starting the daemon: execute the start command in a goroutine. The TUI shows a spinner in the header with text `Starting daemon...`. When the daemon registers in the instance registry (detected via fsnotify on `~/.wolfcastle/instances/`), the TUI transitions to `StateLive`.

Stopping the daemon: send SIGTERM to the PID from the instance registry. The TUI shows `Stopping daemon...` in the header. When the registry file disappears (fsnotify), the TUI transitions to `StateCold`.

If start fails, the error bar shows: `Daemon failed to start: {error}`. Common causes: lock contention (another daemon in the same worktree), missing tiers (suggest `wolfcastle init`).

If stop fails (PID doesn't respond to SIGTERM within 5 seconds), the error bar shows: `Daemon not responding. Try wolfcastle stop --force.`

#### Daemon Control Messages

```go
// DaemonStartMsg requests starting the daemon.
type DaemonStartMsg struct{}

// DaemonStartedMsg confirms the daemon started successfully.
type DaemonStartedMsg struct {
    Entry instance.Entry
}

// DaemonStartFailedMsg indicates the daemon failed to start.
type DaemonStartFailedMsg struct {
    Err error
}

// DaemonStopMsg requests stopping the daemon.
type DaemonStopMsg struct{}

// DaemonStoppedMsg confirms the daemon stopped.
type DaemonStoppedMsg struct{}

// DaemonStopAllMsg requests stopping all instances.
type DaemonStopAllMsg struct{}
```

#### Daemon Control Key Bindings

| Key | Binding Name | Action |
|-----|-------------|--------|
| `s` | toggleDaemon | Start if stopped, stop if running |
| `S` | stopAll | Stop all instances |
| `<` | prevInstance | Switch to previous instance (by index) |
| `>` | nextInstance | Switch to next instance (by index) |
| `1`-`9` | selectInstance | Switch to instance N |

### Phase 3 Error Handling

| Failure | User sees | Recovery |
|---------|-----------|----------|
| Start fails (lock contention) | Error bar: `Another daemon is running in this worktree.` | User must stop the other daemon first. |
| Start fails (no .wolfcastle) | Error bar: `No project found. Run wolfcastle init.` | User runs init or uses welcome flow. |
| Stop fails (process already dead) | TUI transitions to cold-start silently. Stale registry cleaned. | Automatic. |
| Stop times out | Error bar: `Daemon not responding. Try wolfcastle stop --force.` | User can run the CLI command. |
| Instance switch fails (worktree removed) | Error bar: `Worktree no longer exists: {path}`. Instance removed from list. | Automatic cleanup. |

### Test Cases and Acceptance Criteria

#### Acceptance Criteria

1. The instance switcher tab bar appears in the header when `len(instances) > 1` and terminal width > 100 columns.
2. Below 100 columns, only the active instance name and `[N running]` badge are shown.
3. `<` and `>` keys switch the active instance to the previous/next by index.
4. Number keys `1`-`9` switch directly to the Nth instance.
5. Switching instances reloads the root index, resets the tree (collapse all, cursor to top), resets detail to dashboard, restarts log tailing from the new instance's log directory, and updates the watcher paths.
6. The active instance in the tab bar displays a `●` marker and bold text; inactive instances are dim.
7. Pressing `s` when no daemon is running starts the daemon in the background (equivalent to `wolfcastle start -d`).
8. While the daemon is starting, the header shows `Starting daemon...` with a spinner.
9. When the daemon registers (detected via fsnotify on instance registry), the TUI transitions from `StateCold` to `StateLive`.
10. Pressing `s` when a daemon is running sends SIGTERM to the daemon PID.
11. While the daemon is stopping, the header shows `Stopping daemon...`.
12. When the registry file disappears, the TUI transitions from `StateLive` to `StateCold`.
13. Pressing `S` stops all instances.
14. If daemon start fails due to lock contention, the error bar shows `Another daemon is running in this worktree.`
15. If daemon stop times out (5 seconds), the error bar shows `Daemon not responding. Try wolfcastle stop --force.`
16. If the switched-to instance's worktree no longer exists, the error bar shows `Worktree no longer exists: {path}` and the instance is removed from the list.

#### Test Cases

| ID | Description | Setup | Action | Expected Result |
|----|-------------|-------|--------|-----------------|
| P3-001 | Instance switcher appears | 2 instances running, terminal width 120 | Render header | Tab bar shows both instances: `[feat/auth ●] [fix/login]   2 running` |
| P3-002 | Instance switcher narrow terminal | 2 instances running, terminal width 80 | Render header | Only active instance shown with `[2 running]` badge; no tab bar |
| P3-003 | Single instance no switcher | 1 instance running | Render header | No tab bar, no `[N running]` badge; just branch and PID |
| P3-004 | Switch instance with > | 3 instances, active is index 0 | Press `>` | Active switches to index 1; tree resets (cursor 0, all collapsed); detail shows dashboard; log tailing restarts |
| P3-005 | Switch instance with < | 3 instances, active is index 1 | Press `<` | Active switches to index 0 |
| P3-006 | Switch instance wraps around | 3 instances, active is index 2 | Press `>` | Active wraps to index 0 |
| P3-007 | Switch instance by number | 3 instances | Press `2` | Active switches to instance at index 1 |
| P3-008 | Switch instance number out of range | 2 instances | Press `5` | No change; input ignored |
| P3-009 | Instance switch reloads tree | Active instance has 5 nodes; switch to instance with 10 nodes | Press `>` | Tree now shows 10 nodes; old tree completely replaced |
| P3-010 | Instance switch restarts log tail | Active instance logs in `/path/a/logs/`, switching to instance with logs in `/path/b/logs/` | Press `>` | Log view clears and begins tailing from new log directory; watcher updates to new paths |
| P3-011 | Start daemon from cold-start | `entryState == StateCold`, no daemon running | Press `s` | Header shows `Starting daemon...` with spinner; daemon starts in background |
| P3-012 | Start daemon transitions to live | Daemon start succeeded, instance registered via fsnotify | fsnotify fires on instance registry | `entryState` transitions to `StateLive`; header shows `hunting (PID {pid})`; spinner disappears |
| P3-013 | Stop daemon from live | `entryState == StateLive`, daemon PID 12345 | Press `s` | SIGTERM sent to PID 12345; header shows `Stopping daemon...` |
| P3-014 | Stop daemon transitions to cold | Daemon stopped, registry file removed, fsnotify fires | fsnotify fires removal | `entryState` transitions to `StateCold`; header shows `standing down` |
| P3-015 | Stop all instances | 3 instances running | Press `S` | SIGTERM sent to all 3 PIDs; all instances stop; TUI transitions to `StateCold` |
| P3-016 | Start fails lock contention | Another daemon running in same worktree | Press `s` | Error bar: `Another daemon is running in this worktree.`; state remains `StateCold` |
| P3-017 | Start fails no project | `.wolfcastle/` somehow removed while in cold-start | Press `s` | Error bar: `No project found. Run wolfcastle init.` |
| P3-018 | Stop times out | Daemon PID does not respond to SIGTERM within 5 seconds | Press `s` to stop | Error bar: `Daemon not responding. Try wolfcastle stop --force.` |
| P3-019 | Stop already-dead process | Registry has PID 99999 but process is not running | Press `s` to stop | TUI transitions to `StateCold` silently; stale registry entry cleaned |
| P3-020 | Instance switch worktree removed | Instance 2 points to `/path/that/no/longer/exists` | Press `>` to switch to instance 2 | Error bar: `Worktree no longer exists: /path/that/no/longer/exists`; instance removed from list |
| P3-021 | Footer shows start hint in cold-start | `entryState == StateCold` | Render footer | Footer includes `[s] start` binding |
| P3-022 | Footer shows stop hint when live | `entryState == StateLive` | Render footer | Footer includes `[s] stop` binding |
| P3-023 | Instance tab bar active marker | Instance `feat/auth` is active, `fix/login` is inactive | Render header tab bar | `feat/auth` has `●` marker and bold text; `fix/login` is dim |

