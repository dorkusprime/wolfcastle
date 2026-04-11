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

// NodeUpdatedMsg signals that a single node's state has changed.
type NodeUpdatedMsg struct {
	Address string
	Node    *state.NodeState
}

// DaemonStatusMsg carries a snapshot of the daemon's current state.
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

// PollTickMsg triggers a periodic state refresh.
type PollTickMsg struct{}

// SpinnerTickMsg advances the loading spinner animation.
type SpinnerTickMsg struct{}

// ErrorMsg reports a file-scoped error to the TUI.
type ErrorMsg struct {
	Filename string
	Message  string
}

// ErrorClearedMsg signals that a previously reported error has been resolved.
type ErrorClearedMsg struct {
	Filename string
}

// InitStartedMsg signals that project initialization has begun.
type InitStartedMsg struct{}

// InitCompleteMsg signals that project initialization has finished.
type InitCompleteMsg struct {
	Dir string
	Err error
}

// ToggleHelpMsg requests the help overlay to toggle visibility.
type ToggleHelpMsg struct{}

// CopyMsg requests that the given text be copied to the clipboard.
type CopyMsg struct {
	Text string
}

// CopiedMsg confirms that text was successfully copied to the clipboard.
type CopiedMsg struct{}

// LogLinesMsg delivers new log output lines to the log viewer.
type LogLinesMsg struct {
	Lines []string // raw JSON strings, one per log line
}

// NewLogFileMsg signals that the daemon started writing to a new log file.
type NewLogFileMsg struct {
	Path string
}

// SwitchInstanceMsg requests switching the TUI to a different daemon instance.
type SwitchInstanceMsg struct {
	Entry instance.Entry
}

// InstanceSwitchedMsg confirms that the TUI switched to a new instance.
type InstanceSwitchedMsg struct {
	Index *state.RootIndex
	Entry instance.Entry
}

// DaemonStartMsg requests starting a new daemon process.
type DaemonStartMsg struct{}

// DaemonStartedMsg confirms that a daemon process was launched.
type DaemonStartedMsg struct {
	Entry instance.Entry
}

// DaemonStartFailedMsg reports that daemon startup failed.
type DaemonStartFailedMsg struct {
	Err    error
	Stderr string
}

// DaemonStopMsg requests stopping the current daemon process.
type DaemonStopMsg struct{}

// DaemonStoppedMsg confirms that the daemon was stopped.
type DaemonStoppedMsg struct{}

// DaemonStopAllMsg requests stopping all running daemon instances.
type DaemonStopAllMsg struct{}

// DaemonStopFailedMsg reports that stopping the daemon failed.
type DaemonStopFailedMsg struct {
	Err error
}

// WorktreeGoneMsg signals that the watched worktree no longer exists.
type WorktreeGoneMsg struct {
	Entry instance.Entry
}

// InboxUpdatedMsg signals that the inbox file was reloaded.
type InboxUpdatedMsg struct {
	Inbox *state.InboxFile
}

// InboxItemAddedMsg confirms a new item was added to the inbox.
type InboxItemAddedMsg struct{}

// InboxAddFailedMsg reports that adding an inbox item failed.
type InboxAddFailedMsg struct {
	Err error
}

// AddInboxItemCmd is a command message that triggers inbox item creation.
type AddInboxItemCmd struct {
	Text string
}

// DaemonConfirmedMsg signals that the user confirmed a daemon action in the modal.
type DaemonConfirmedMsg struct{}

// ToastMsg triggers a timed notification in the TUI.
type ToastMsg struct {
	Text string
}
