package tui

import (
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
	Status     string
	Branch     string
	PID        int
	IsRunning  bool
	IsDraining bool
	Instances  []instance.Entry
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
	Lines []any // will be logrender.Record in Phase 2
}

type NewLogFileMsg struct {
	Path string
}

// Phase 4 placeholder

type InboxUpdatedMsg struct {
	Inbox *state.InboxFile
}
