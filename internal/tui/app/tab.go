package app

import (
	"os"
	"path/filepath"
	"time"

	tea "charm.land/bubbletea/v2"

	"github.com/dorkusprime/wolfcastle/internal/daemon"
	"github.com/dorkusprime/wolfcastle/internal/state"
	"github.com/dorkusprime/wolfcastle/internal/tui"
	"github.com/dorkusprime/wolfcastle/internal/tui/detail"
	"github.com/dorkusprime/wolfcastle/internal/tui/search"
	"github.com/dorkusprime/wolfcastle/internal/tui/tree"
)

const pollInterval = 2 * time.Second

// Tab is a self-contained workspace rooted at a single directory.
// Every piece of directory-dependent state lives here, never on
// TUIModel. Switching tabs means changing activeTabID, not
// coordinating updates to a dozen scattered fields.
type Tab struct {
	ID          int    // stable identifier, monotonically increasing, never reused
	Label       string // display name (basename of WorktreeDir)
	WorktreeDir string // absolute path to the directory

	Store      *state.Store       // nil until .wolfcastle/ is resolved
	DaemonRepo *daemon.Repository // nil until .wolfcastle/ is resolved
	Watcher    *tui.Watcher       // nil until Start(); stopped on Stop()
	Events     chan tea.Msg       // per-tab watcher channel
	EntryState EntryState         // Cold, Live, Welcome, Browse

	Tree   tree.Model
	Detail detail.Model
	Search search.Model

	// State diffing for toast notifications.
	PrevIndex *state.RootIndex
	PrevNodes map[string]*state.NodeState

	Focused     FocusedPane
	LastFocused FocusedPane
	TreeVisible bool
	Loading     bool

	DaemonStarting bool
	DaemonStopping bool

	Errors []errorEntry
}

// StateBrowse is the entry state for a tab whose directory has no
// .wolfcastle/ and the user declined init.
const StateBrowse EntryState = 3

// newTab creates a tab for the given directory. If store and
// daemonRepo are non-nil the tab starts in Cold state; otherwise
// it starts in Welcome state. Call Start() to wire up the watcher
// and poll chain.
func newTab(id int, worktreeDir string, store *state.Store, daemonRepo *daemon.Repository) *Tab {
	entryState := StateCold
	if store == nil {
		entryState = StateWelcome
	}
	t := &Tab{
		ID:          id,
		Label:       filepath.Base(worktreeDir),
		WorktreeDir: worktreeDir,
		Store:       store,
		DaemonRepo:  daemonRepo,
		Events:      make(chan tea.Msg, 256),
		EntryState:  entryState,
		Tree:        tree.NewModel(),
		Detail:      detail.NewModel(),
		Search:      search.NewModel(),
		PrevNodes:   make(map[string]*state.NodeState),
		Focused:     PaneTree,
		TreeVisible: true,
	}
	// Sync focus state to sub-models so the tree processes keys.
	t.Tree.SetFocused(true)
	t.Detail.SetFocused(false)
	return t
}

// Start wires up the tab's watcher, starts polling, loads initial
// state, and returns a batch of Cmds that must be fed to Bubbletea.
// Safe to call only when Store is non-nil.
func (t *Tab) Start() tea.Cmd {
	if t.Store == nil {
		return nil
	}
	t.Loading = true

	// Create and start the watcher.
	logDir := ""
	if t.DaemonRepo != nil {
		logDir = t.DaemonRepo.LogDir()
	}
	instanceDir := ""
	if dir, err := instanceRegistryDir(); err == nil {
		instanceDir = dir
	}
	t.Watcher = tui.NewWatcher(t.Store, logDir, instanceDir, t.Events)
	_ = t.Watcher.Start()
	t.Watcher.StartPolling()
	_ = t.Watcher.EagerPrefetchAndSubscribe()

	tabID := t.ID
	store := t.Store
	worktree := t.WorktreeDir
	events := t.Events

	// Load initial state.
	loadState := func() tea.Msg {
		idx, err := store.ReadIndex()
		if err != nil {
			return TabMsg{TabID: tabID, Inner: tui.ErrorMsg{
				Filename: "state.json",
				Message:  "State corruption detected: state.json. Run wolfcastle doctor.",
			}}
		}
		return TabMsg{TabID: tabID, Inner: tui.StateUpdatedMsg{Index: idx, Worktree: worktree}}
	}

	// Start the per-tab poll chain.
	pollTick := scheduleTabPollTick(tabID)

	// Start the per-tab event drain.
	drain := waitForTabEvent(tabID, events)

	return tea.Batch(loadState, pollTick, drain)
}

// Stop releases the tab's watcher and drains its event channel.
func (t *Tab) Stop() {
	if t.Watcher != nil {
		t.Watcher.Stop()
		t.Watcher = nil
	}
	// Drain any buffered events so senders don't block.
	for {
		select {
		case <-t.Events:
		default:
			return
		}
	}
}

// Reset clears all state, used when re-initializing a tab (e.g.,
// after running wolfcastle init in a Browse-state tab).
func (t *Tab) Reset() {
	t.Tree.Reset()
	t.Detail.Reset()
	t.Search = search.NewModel()
	t.PrevIndex = nil
	t.PrevNodes = make(map[string]*state.NodeState)
	t.Errors = nil
}

// TabMsg wraps a tea.Msg with a stable tab ID so the Update loop
// can route directory-scoped messages to the correct tab.
type TabMsg struct {
	TabID int
	Inner tea.Msg
}

// TabPollTickMsg triggers a poll cycle for a specific tab.
type TabPollTickMsg struct {
	TabID int
}

// waitForTabEvent blocks on a tab's event channel and wraps the
// result in a TabMsg. Each call drains one event; the handler
// reschedules another drain to keep the loop continuous.
func waitForTabEvent(tabID int, events <-chan tea.Msg) tea.Cmd {
	if events == nil {
		return nil
	}
	return func() tea.Msg {
		msg, ok := <-events
		if !ok {
			return nil
		}
		return TabMsg{TabID: tabID, Inner: msg}
	}
}

// scheduleTabPollTick returns a Cmd that fires a TabPollTickMsg
// after 2 seconds for the given tab.
func scheduleTabPollTick(tabID int) tea.Cmd {
	return tea.Tick(pollInterval, func(time.Time) tea.Msg {
		return TabPollTickMsg{TabID: tabID}
	})
}

// tabLabels returns display labels for all tabs.
func (m *TUIModel) tabLabels() []string {
	labels := make([]string, len(m.tabs))
	for i, t := range m.tabs {
		labels[i] = t.Label
	}
	return labels
}

// activeTabIndex returns the slice index of the active tab.
func (m *TUIModel) activeTabIndex() int {
	for i, t := range m.tabs {
		if t.ID == m.activeTabID {
			return i
		}
	}
	return 0
}

// tabRunningSet returns a set of slice indices whose tabs have a
// running daemon, used by the header to show the ● indicator.
func (m *TUIModel) tabRunningSet() map[int]bool {
	running := make(map[int]bool)
	for i, t := range m.tabs {
		if t.EntryState == StateLive {
			running[i] = true
		}
	}
	return running
}

// handleCloseTab closes the active tab with daemon-running confirmation.
func (m *TUIModel) handleCloseTab() tea.Cmd {
	if len(m.tabs) <= 1 {
		return m.notify.Push("Last tab. Press q to quit.")
	}
	tab := m.activeTab()
	if tab == nil {
		return nil
	}
	// If daemon is running, close without stopping (user can reconnect via +).
	tab.Stop()
	// Remove from slice.
	idx := m.activeTabIndex()
	m.tabs = append(m.tabs[:idx], m.tabs[idx+1:]...)
	// Select adjacent tab.
	if idx >= len(m.tabs) {
		idx = len(m.tabs) - 1
	}
	m.activeTabID = m.tabs[idx].ID
	m.header.SetTabs(m.tabLabels(), m.activeTabIndex(), m.tabRunningSet())
	m.propagateSize()
	return nil
}

// resolveStoreForDir attempts to load a Store and DaemonRepository
// from a directory's .wolfcastle/. Returns nils if not initialized.
func resolveStoreForDir(dir string) (*state.Store, *daemon.Repository) {
	wolfDir := filepath.Join(dir, ".wolfcastle")
	info, err := os.Stat(wolfDir)
	if err != nil || !info.IsDir() {
		return nil, nil
	}
	store := storeFromWolfcastleDir(wolfDir)
	daemonRepo := daemon.NewRepository(wolfDir)
	return store, daemonRepo
}
