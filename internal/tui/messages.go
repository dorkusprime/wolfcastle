package tui

import (
	"time"

	tea "charm.land/bubbletea/v2"
	"github.com/dorkusprime/wolfcastle/internal/instance"
	"github.com/dorkusprime/wolfcastle/internal/state"
	"github.com/fsnotify/fsnotify"
)

// StateUpdatedMsg signals that the root index has been reloaded from disk.
// Worktree identifies which instance produced this read; the handler
// discards messages whose Worktree doesn't match the current worktreeDir
// to prevent stale in-flight reads from overwriting a freshly-switched
// context.
type StateUpdatedMsg struct {
	Index    *state.RootIndex
	Worktree string
}

// NodeUpdatedMsg signals that a single node's state file has been refreshed.
type NodeUpdatedMsg struct {
	Address string
	Node    *state.NodeState
}

// DaemonStatusMsg carries a snapshot of daemon process status for display.
type DaemonStatusMsg struct {
	Status       string
	Branch       string
	Worktree     string
	PID          int
	IsRunning    bool
	IsDraining   bool
	Instances    []instance.Entry
	LastActivity time.Time
	CurrentNode  string
	CurrentTask  string
}

// InstancesUpdatedMsg signals that the instance registry has been refreshed.
type InstancesUpdatedMsg struct {
	Instances []instance.Entry
}

// WatcherEventMsg wraps a single filesystem event from the state watcher.
type WatcherEventMsg struct {
	Path string
	Op   fsnotify.Op
}

// WatcherMsg wraps every message produced by the filesystem Watcher.
// This single envelope lets the model dispatch real-time events with
// just one handler that unwraps Inner, processes it, and reschedules
// the next channel drain. Without the wrapper, every watcher-sourced
// message type would need its own rescheduling logic and we'd be one
// missed call away from silently breaking real-time updates again.
type WatcherMsg struct {
	Inner tea.Msg
}

// PollTickMsg triggers a periodic state-refresh cycle from the model's tick scheduler.
type PollTickMsg struct{}

// SpinnerTickMsg advances the spinner animation by one frame.
type SpinnerTickMsg struct{}

// ErrorMsg reports a file-level error to be displayed in the error bar.
type ErrorMsg struct {
	Filename string
	Message  string
}

// ErrorClearedMsg signals that a previously reported file error has been resolved.
type ErrorClearedMsg struct {
	Filename string
}

// InitStartedMsg signals that project initialization has begun.
type InitStartedMsg struct{}

// InitCompleteMsg signals that project initialization finished, possibly with an error.
type InitCompleteMsg struct {
	Dir string
	Err error
}

// ToggleHelpMsg toggles the help overlay on or off.
type ToggleHelpMsg struct{}

// CopyMsg requests that the given text be copied to the clipboard.
type CopyMsg struct {
	Text string
}

// CopiedMsg confirms that a clipboard copy operation completed.
type CopiedMsg struct{}

// LogLinesMsg delivers one or more new log lines from the active log file.
type LogLinesMsg struct {
	Lines []string // raw JSON strings, one per log line
}

// NewLogFileMsg signals that the daemon rotated to a new log file.
type NewLogFileMsg struct {
	Path string
}

// SwitchInstanceMsg requests switching the TUI to a different daemon instance.
type SwitchInstanceMsg struct {
	Entry instance.Entry
}

// InstanceSwitchedMsg confirms that the active instance was changed successfully.
type InstanceSwitchedMsg struct {
	Index *state.RootIndex
	Entry instance.Entry
}

// DaemonStartMsg requests that a daemon process be started for the current worktree.
type DaemonStartMsg struct{}

// DaemonStartedMsg confirms that a daemon process launched successfully.
type DaemonStartedMsg struct {
	Entry instance.Entry
}

// DaemonStartFailedMsg reports that a daemon start attempt failed.
type DaemonStartFailedMsg struct {
	Err    error
	Stderr string
}

// DaemonStopMsg requests that the current daemon process be stopped.
type DaemonStopMsg struct{}

// DaemonStoppedMsg confirms that the daemon process was stopped.
type DaemonStoppedMsg struct{}

// DaemonStopAllMsg requests that all running daemon instances be stopped.
type DaemonStopAllMsg struct{}

// DaemonStopFailedMsg reports that a daemon stop attempt failed.
type DaemonStopFailedMsg struct {
	Err error
}

// WorktreeGoneMsg signals that a registered instance's worktree no longer exists.
type WorktreeGoneMsg struct {
	Entry instance.Entry
}

// InboxUpdatedMsg signals that the inbox file was reloaded from disk.
type InboxUpdatedMsg struct {
	Inbox *state.InboxFile
}

// InboxItemAddedMsg confirms that an inbox item was written successfully.
type InboxItemAddedMsg struct{}

// InboxAddFailedMsg reports that adding an inbox item failed.
type InboxAddFailedMsg struct {
	Err error
}

// AddInboxItemCmd is a command message requesting a new inbox item be persisted.
type AddInboxItemCmd struct {
	Text string
}

// DaemonConfirmedMsg signals that the user confirmed a daemon action in a modal dialog.
type DaemonConfirmedMsg struct{}

// DaemonDirtyConfirmedMsg signals that the user confirmed the dirty-tree
// start prompt in a modal dialog. The app responds by re-invoking
// startDaemonWithFlags(allowDirty=true) so the detached `start -d`
// process skips its interactive y/N prompt.
type DaemonDirtyConfirmedMsg struct{}

// ToastMsg requests that a transient notification toast be displayed.
type ToastMsg struct {
	Text string
}
