package tui

import (
	"time"

	"github.com/dorkusprime/wolfcastle/internal/instance"
	"github.com/dorkusprime/wolfcastle/internal/state"
	"github.com/fsnotify/fsnotify"
)

// Phase 1 messages

type StateUpdatedMsg struct {
	Index *state.RootIndex
}

type NodeUpdatedMsg struct {
	Address string
	Node    *state.NodeState
}

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

type InstancesUpdatedMsg struct {
	Instances []instance.Entry
}

type WatcherEventMsg struct {
	Path string
	Op   fsnotify.Op
}

type PollTickMsg struct{}

type SpinnerTickMsg struct{}

type ErrorMsg struct {
	Filename string
	Message  string
}

type ErrorClearedMsg struct {
	Filename string
}

type InitStartedMsg struct{}

type InitCompleteMsg struct {
	Dir string
	Err error
}

type ToggleHelpMsg struct{}

type CopyMsg struct {
	Text string
}

type CopiedMsg struct{}

// Phase 2 placeholder messages

type LogLinesMsg struct {
	Lines []string // raw JSON strings, one per log line
}

type NewLogFileMsg struct {
	Path string
}

// Phase 3 messages

type SwitchInstanceMsg struct {
	Entry instance.Entry
}

type InstanceSwitchedMsg struct {
	Index *state.RootIndex
	Entry instance.Entry
}

type DaemonStartMsg struct{}

type DaemonStartedMsg struct {
	Entry instance.Entry
}

type DaemonStartFailedMsg struct {
	Err error
}

type DaemonStopMsg struct{}

type DaemonStoppedMsg struct{}

type DaemonStopAllMsg struct{}

type DaemonStopFailedMsg struct {
	Err error
}

type WorktreeGoneMsg struct {
	Entry instance.Entry
}

// Phase 4: Inbox

type InboxUpdatedMsg struct {
	Inbox *state.InboxFile
}

type InboxItemAddedMsg struct{}

type InboxAddFailedMsg struct {
	Err error
}

type AddInboxItemCmd struct {
	Text string
}

// Phase 5 messages

type ToastMsg struct {
	Text string
}
