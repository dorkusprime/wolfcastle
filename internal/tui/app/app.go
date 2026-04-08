// Package app contains the top-level TUI orchestrator that wires sub-models
// together, routes messages, manages focus, and handles layout. It lives in
// its own package to break the circular dependency between the parent tui
// package (which holds shared message types) and the sub-model packages
// (detail, footer, welcome, search, help) that import those types.
package app

import (
	"bytes"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"syscall"
	"time"

	"charm.land/bubbles/v2/key"
	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"
	"github.com/charmbracelet/x/ansi"

	"github.com/dorkusprime/wolfcastle/internal/config"
	"github.com/dorkusprime/wolfcastle/internal/daemon"
	"github.com/dorkusprime/wolfcastle/internal/instance"
	"github.com/dorkusprime/wolfcastle/internal/state"
	"github.com/dorkusprime/wolfcastle/internal/tui"
	"github.com/dorkusprime/wolfcastle/internal/tui/clipboard"
	"github.com/dorkusprime/wolfcastle/internal/tui/detail"
	"github.com/dorkusprime/wolfcastle/internal/tui/footer"
	"github.com/dorkusprime/wolfcastle/internal/tui/header"
	"github.com/dorkusprime/wolfcastle/internal/tui/help"
	"github.com/dorkusprime/wolfcastle/internal/tui/notify"
	"github.com/dorkusprime/wolfcastle/internal/tui/search"
	"github.com/dorkusprime/wolfcastle/internal/tui/tree"
	"github.com/dorkusprime/wolfcastle/internal/tui/welcome"
)

// FocusedPane identifies which content pane holds keyboard focus.
type FocusedPane int

const (
	PaneTree FocusedPane = iota
	PaneDetail
)

// EntryState tracks the lifecycle phase of the TUI session.
type EntryState int

const (
	StateLive EntryState = iota
	StateCold
	StateWelcome
)

type errorEntry struct {
	filename string
	message  string
}

// TUIModel is the top-level Bubbletea model. It owns every sub-model, routes
// messages between them, manages focus, and computes layout.
type TUIModel struct {
	width       int
	height      int
	treeVisible bool

	focused     FocusedPane
	lastFocused FocusedPane

	header  header.HeaderModel
	tree    tree.TreeModel
	detail  detail.DetailModel
	footer  footer.FooterModel
	welcome *welcome.WelcomeModel
	search  search.SearchModel
	help    help.HelpOverlayModel
	notify  notify.NotificationModel

	// State diffing for toast notifications (Phase 5).
	prevIndex *state.RootIndex
	prevNodes map[string]*state.NodeState

	switching   bool   // true while instance switch is in flight
	switchLabel string // label of the instance being switched to

	entryState  EntryState
	store       *state.Store
	daemonRepo  *daemon.DaemonRepository
	worktreeDir string
	version     string

	watcher       *tui.Watcher
	watcherEvents chan tea.Msg // owned by the model; passed to NewWatcher

	// Instance tracking (Phase 3)
	instances           []instance.Entry
	activeInstanceIndex int
	daemonStarting      bool
	daemonStopping      bool

	errors []errorEntry
}

// NewTUIModel creates a fully wired TUIModel. When store is nil (no
// .wolfcastle directory found), the model opens in welcome mode so the
// user can pick a directory and initialize.
func NewTUIModel(store *state.Store, daemonRepo *daemon.DaemonRepository, worktreeDir, version string) TUIModel {
	m := TUIModel{
		treeVisible: true,
		focused:     PaneTree,
		header:      header.NewHeaderModel(version),
		tree:        tree.NewTreeModel(),
		detail:      detail.NewDetailModel(),
		footer:      footer.NewFooterModel(),
		search:      search.NewSearchModel(),
		help:        help.NewHelpOverlayModel(),
		notify:      notify.NewNotificationModel(),
		prevNodes:   make(map[string]*state.NodeState),
		store:       store,
		daemonRepo:  daemonRepo,
		worktreeDir: worktreeDir,
		version:     version,
		// 256 is enough headroom that a brief render stall (the model
		// taking a few ms to drain) does not cause the watcher to drop
		// events. The watcher's emit is non-blocking; if the buffer
		// fills, events are dropped and the next mtime/poll cycle
		// resends an equivalent state update.
		watcherEvents: make(chan tea.Msg, 256),
	}

	if store == nil {
		m.entryState = StateWelcome
		// Discover running instances for the sessions panel.
		instances, _ := instance.List()
		w := welcome.NewWelcomeModel(worktreeDir, instances)
		m.welcome = &w
	} else {
		m.entryState = StateCold
	}

	m.syncFocus()
	return m
}

// Init returns the batch of startup commands: entry-state detection,
// filesystem watcher, polling, and initial state load.
func (m TUIModel) Init() tea.Cmd {
	var cmds []tea.Cmd

	cmds = append(cmds, m.detectEntryState())

	if m.store != nil {
		m.header.SetLoading(true)
		cmds = append(cmds,
			m.startWatcher(),
			m.startPoller(),
			m.loadInitialState(),
			waitForWatcherEvent(m.watcherEvents),
		)
	}

	return tea.Batch(cmds...)
}

// waitForWatcherEvent returns a tea.Cmd that blocks on the watcher's
// event channel and returns the next message it receives. Each call
// drains exactly one event; the WatcherMsg handler in Update reschedules
// another waitForWatcherEvent so the drain loop is continuous.
func waitForWatcherEvent(ch <-chan tea.Msg) tea.Cmd {
	if ch == nil {
		return nil
	}
	return func() tea.Msg {
		return <-ch
	}
}

// Update is the central message router. Data messages are always broadcast
// to the sub-models that care about them; key messages are routed according
// to overlay and focus state.
func (m TUIModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	var cmds []tea.Cmd

	switch msg := msg.(type) {

	// ---------------------------------------------------------------
	// Terminal
	// ---------------------------------------------------------------
	case tea.WindowSizeMsg:
		m.width = msg.Width
		m.height = msg.Height
		m.propagateSize()
		return m, nil

	// ---------------------------------------------------------------
	// Keys
	// ---------------------------------------------------------------
	case tea.KeyPressMsg:
		// Ctrl+C is the unconditional emergency exit.
		if key.Matches(msg, tui.GlobalKeyMap.ForceQuit) {
			return m, tea.Quit
		}

		// Detail pane capturing input (e.g., inbox text field) routes
		// directly to the detail model, bypassing all global bindings.
		if m.detail.IsCapturingInput() {
			d, cmd := m.detail.Update(msg)
			m.detail = d
			return m, cmd
		}

		// Welcome screen swallows everything else.
		if m.welcome != nil && m.entryState == StateWelcome {
			if key.Matches(msg, tui.GlobalKeyMap.Quit) {
				return m, tea.Quit
			}
			w, cmd := m.welcome.Update(msg)
			m.welcome = &w
			return m, cmd
		}

		// Help overlay absorbs all keys except its dismiss binding.
		if m.help.IsActive() {
			h, cmd := m.help.Update(msg)
			m.help = h
			return m, cmd
		}

		// Active search bar captures input.
		if m.search.IsActive() {
			prevQuery := m.search.Query()
			s, cmd := m.search.Update(msg)
			m.search = s
			if m.search.Query() != prevQuery {
				m.computeTreeSearchMatches()
			}
			return m, cmd
		}

		// Global bindings.
		switch {
		case key.Matches(msg, tui.GlobalKeyMap.Quit):
			return m, tea.Quit

		case key.Matches(msg, tui.GlobalKeyMap.LogStream):
			// Switch the detail pane to the log stream view from any
			// focus state. The binding is capital L so it doesn't
			// collide with the tree pane's vi-style l = expand.
			m.detail.SwitchToLogView()
			m.focused = PaneDetail
			m.syncFocus()
			return m, nil

		case key.Matches(msg, tui.GlobalKeyMap.Inbox):
			m.detail.SwitchToInbox()
			m.focused = PaneDetail
			m.syncFocus()
			return m, m.loadInbox()

		case key.Matches(msg, tui.GlobalKeyMap.ToggleTree):
			m.treeVisible = !m.treeVisible
			if !m.treeVisible && m.focused == PaneTree {
				m.lastFocused = PaneTree
				m.focused = PaneDetail
				m.syncFocus()
			} else if m.treeVisible && m.lastFocused == PaneTree {
				m.focused = PaneTree
				m.syncFocus()
			}
			m.propagateSize()
			return m, nil

		case key.Matches(msg, tui.GlobalKeyMap.CycleFocus):
			m.cycleFocus()
			return m, nil

		case key.Matches(msg, tui.GlobalKeyMap.Refresh):
			m.header.SetLoading(true)
			return m, m.handleRefresh()

		case key.Matches(msg, tui.GlobalKeyMap.ToggleHelp):
			m.help.Toggle()
			return m, nil

		case key.Matches(msg, tui.GlobalKeyMap.Search):
			m.search.Activate(int(m.focused))
			return m, nil

		case key.Matches(msg, tui.GlobalKeyMap.Copy):
			return m, m.handleCopy()

		case key.Matches(msg, tui.DaemonKeyMap.ToggleDaemon):
			cmd := m.handleToggleDaemon()
			return m, cmd

		case key.Matches(msg, tui.DaemonKeyMap.StopAll):
			cmd := m.handleStopAll()
			return m, cmd

		case key.Matches(msg, tui.DaemonKeyMap.PrevInstance):
			cmd := m.handleSwitchInstance(-1)
			return m, cmd

		case key.Matches(msg, tui.DaemonKeyMap.NextInstance):
			cmd := m.handleSwitchInstance(1)
			return m, cmd
		}

		// Digit keys 1-9 for instance selection.
		if ch := msg.String(); len(ch) == 1 && ch[0] >= '1' && ch[0] <= '9' {
			idx := int(ch[0]-'0') - 1
			if idx < len(m.instances) {
				cmd := m.switchInstance(m.instances[idx])
				return m, cmd
			}
		}

		// Esc clears the error bar when errors are visible.
		if msg.String() == "esc" && len(m.errors) > 0 {
			m.errors = nil
			return m, nil
		}

		// Esc clears persistent search highlights when the search
		// bar is inactive but matches were left highlighted from a
		// prior Confirm. The user can keep typing into the tree
		// without dragging the old highlights along forever.
		if msg.String() == "esc" && !m.search.IsActive() && (m.search.HasMatches() || m.tree.HasSearchHighlights()) {
			m.search.Dismiss()
			m.tree.SetSearchAddresses(nil, nil)
			return m, nil
		}

		// Esc in detail pane (non-dashboard mode) returns to dashboard.
		if msg.String() == "esc" && m.focused == PaneDetail && m.detail.Mode() != detail.ModeDashboard {
			m.detail.SwitchToDashboard()
			return m, nil
		}

		// n/N for search match navigation (when search has confirmed matches).
		if m.search.HasMatches() {
			prev := m.search.Current()
			s, cmd := m.search.Update(msg)
			m.search = s
			if m.search.Current() != prev {
				m.jumpTreeToSearchMatch()
				return m, cmd
			}
		}

		// Route remaining keys to the focused pane.
		switch m.focused {
		case PaneTree:
			t, cmd := m.tree.Update(msg)
			m.tree = t
			cmds = append(cmds, cmd)
			// On Enter/l/right in tree, load detail for the selected row.
			if key.Matches(msg, tui.TreeKeyMap.Expand) {
				m.loadDetailForSelection()
			}
		case PaneDetail:
			d, cmd := m.detail.Update(msg)
			m.detail = d
			cmds = append(cmds, cmd)
		}
		return m, tea.Batch(cmds...)

	// ---------------------------------------------------------------
	// Watcher envelope: unwrap, recursively dispatch the inner message
	// through Update so the existing typed handlers below run, then
	// reschedule the channel drain so the next event keeps flowing.
	// Without the reschedule the watcher's event channel would deliver
	// exactly one message and then go silent.
	// ---------------------------------------------------------------
	case tui.WatcherMsg:
		var inner tea.Cmd
		if msg.Inner != nil {
			updated, c := m.Update(msg.Inner)
			if tm, ok := updated.(TUIModel); ok {
				m = tm
			}
			inner = c
		}
		return m, tea.Batch(inner, waitForWatcherEvent(m.watcherEvents))

	// ---------------------------------------------------------------
	// Data messages: always broadcast regardless of overlay state
	// ---------------------------------------------------------------
	case tui.StateUpdatedMsg:
		m.header.SetLoading(false)
		h, hcmd := m.header.Update(header.StateUpdatedMsg{Index: msg.Index})
		m.header = h
		cmds = append(cmds, hcmd)

		m.tree.SetIndex(msg.Index)

		d, dcmd := m.detail.Update(msg)
		m.detail = d
		cmds = append(cmds, dcmd)

		// Phase 5: diff against previous index to detect new nodes.
		if msg.Index != nil && m.prevIndex != nil {
			for addr := range msg.Index.Nodes {
				if _, existed := m.prevIndex.Nodes[addr]; !existed {
					cmd := m.notify.Push(fmt.Sprintf("New target acquired: %s", addr))
					cmds = append(cmds, cmd)
				}
			}
		}
		m.prevIndex = msg.Index

		m.clearErrorsByFilename("state.json")
		return m, tea.Batch(cmds...)

	case tui.DaemonStatusMsg:
		// Update instance list on the TUI model.
		if msg.Instances != nil {
			m.instances = msg.Instances
			m.header.SetInstances(m.instances, m.activeInstanceIndex)
		}

		h, hcmd := m.header.Update(header.DaemonStatusMsg{
			Status:     msg.Status,
			Branch:     msg.Branch,
			Worktree:   msg.Worktree,
			PID:        msg.PID,
			IsRunning:  msg.IsRunning,
			IsDraining: msg.IsDraining,
			Instances:  msg.Instances,
		})
		m.header = h
		cmds = append(cmds, hcmd)

		d, dcmd := m.detail.Update(msg)
		m.detail = d
		cmds = append(cmds, dcmd)

		f, fcmd := m.footer.Update(msg)
		m.footer = f
		cmds = append(cmds, fcmd)

		if msg.IsRunning {
			m.entryState = StateLive
		} else {
			m.entryState = StateCold
		}
		return m, tea.Batch(cmds...)

	case tui.NodeUpdatedMsg:
		t, tcmd := m.tree.Update(tree.NodeUpdatedMsg{
			Address: msg.Address,
			Node:    msg.Node,
		})
		m.tree = t
		cmds = append(cmds, tcmd)

		// Phase 5: diff against previous node state for toast notifications.
		if msg.Node != nil {
			if prev, ok := m.prevNodes[msg.Address]; ok {
				cmds = append(cmds, m.diffNodeForToasts(msg.Address, prev, msg.Node)...)
			}
			cp := *msg.Node
			m.prevNodes[msg.Address] = &cp
		}

		return m, tea.Batch(cmds...)

	case tree.LoadNodeMsg:
		// The tree fires this when expanding a leaf whose NodeState isn't
		// cached. The command only carries the address; we do the actual
		// disk read here and feed the result back as a NodeUpdatedMsg.
		if msg.Node == nil && msg.Err == nil && m.store != nil {
			store := m.store
			addr := msg.Address
			return m, func() tea.Msg {
				ns, err := store.ReadNode(addr)
				if err != nil {
					return nil
				}
				return tui.NodeUpdatedMsg{Address: addr, Node: ns}
			}
		}
		// If the msg already carries a node (e.g. from tests), forward it.
		if msg.Node != nil {
			t, tcmd := m.tree.Update(msg)
			m.tree = t
			cmds = append(cmds, tcmd)
			return m, tea.Batch(cmds...)
		}
		return m, nil

	case tui.InstancesUpdatedMsg:
		m.instances = msg.Instances
		h, hcmd := m.header.Update(header.InstancesUpdatedMsg{
			Instances: msg.Instances,
		})
		m.header = h
		cmds = append(cmds, hcmd)
		m.header.SetInstances(msg.Instances, m.activeInstanceIndex)
		return m, tea.Batch(cmds...)

	case tui.DaemonStartedMsg:
		m.daemonStarting = false
		m.header.SetLoading(false)
		m.entryState = StateLive
		m.header.SetStatusHint("")
		return m, m.handleRefresh()

	case tui.DaemonStartFailedMsg:
		m.daemonStarting = false
		m.header.SetLoading(false)
		m.header.SetStatusHint("")
		// Prefer the child's stderr (real reason) over the bare exec
		// exit string. Fall back to the exit error if stderr was empty.
		raw := strings.TrimSpace(msg.Stderr)
		if raw == "" && msg.Err != nil {
			raw = msg.Err.Error()
		}
		// Collapse to a single line so the toast renders cleanly
		// regardless of how many lines stderr emitted.
		raw = sanitizeErrorLine(raw)
		var toastText string
		switch {
		case strings.Contains(raw, "already running"), strings.Contains(raw, "lock"):
			toastText = "Another daemon is already running here."
		case strings.Contains(raw, "no .wolfcastle"), strings.Contains(raw, "no such"), strings.Contains(raw, "config not found"):
			toastText = "No project found. Run wolfcastle init."
		case strings.Contains(raw, "identity not configured"):
			toastText = "Identity not configured. Run wolfcastle init."
		case strings.Contains(raw, "uncommitted changes"), strings.Contains(raw, "commit or stash"):
			toastText = "Uncommitted changes. Commit or stash first."
		case raw == "":
			toastText = "Daemon failed to start."
		default:
			toastText = "Daemon failed to start: " + raw
		}
		cmds = append(cmds, m.notify.Push(toastText))
		return m, tea.Batch(cmds...)

	case tui.DaemonStoppedMsg:
		m.daemonStopping = false
		m.entryState = StateCold
		m.header.SetStatusHint("")
		return m, m.handleRefresh()

	case tui.DaemonStopFailedMsg:
		m.daemonStopping = false
		m.header.SetStatusHint("")
		stopErr := ""
		if msg.Err != nil {
			stopErr = sanitizeErrorLine(msg.Err.Error())
		}
		if stopErr == "" {
			stopErr = "Daemon failed to stop."
		}
		m.appendError("daemon", stopErr)
		return m, nil

	case tui.InstanceSwitchedMsg:
		m.switching = false
		m.switchLabel = ""
		m.header.SetStatusHint("")
		m.header.SetLoading(false)

		// Reset diff state so the new instance's nodes don't appear as "new".
		m.prevIndex = msg.Index
		m.prevNodes = make(map[string]*state.NodeState)
		m.notify = notify.NewNotificationModel()

		// Reset tree: collapse all nodes, cursor to 0, then load new index.
		m.tree.Reset()
		m.tree.SetIndex(msg.Index)

		// Reset detail pane to dashboard.
		m.detail.SwitchToDashboard()

		h, hcmd := m.header.Update(header.StateUpdatedMsg{Index: msg.Index})
		m.header = h
		cmds = append(cmds, hcmd)
		m.header.SetInstances(m.instances, m.activeInstanceIndex)

		// Update store and daemon repo for the new worktree.
		wolfDir := filepath.Join(msg.Entry.Worktree, ".wolfcastle")
		m.store = storeFromWolfcastleDir(wolfDir)
		m.daemonRepo = daemon.NewDaemonRepository(wolfDir)
		m.worktreeDir = msg.Entry.Worktree

		// Restart watcher: stop old, create and start new.
		if m.watcher != nil {
			m.watcher.Stop()
		}
		logDir := ""
		if m.daemonRepo != nil {
			logDir = m.daemonRepo.LogDir()
		}
		instanceDir := ""
		if dir, err := instanceRegistryDir(); err == nil {
			instanceDir = dir
		}
		w := tui.NewWatcher(m.store, logDir, instanceDir, m.watcherEvents)
		if err := w.Start(); err != nil {
			w.StartPolling()
		}
		m.watcher = w

		// Immediately probe daemon status and inbox so the dashboard
		// populates without waiting for the next poll tick.
		cmds = append(cmds, m.detectEntryState(), m.loadInbox())
		return m, tea.Batch(cmds...)

	case tui.SpinnerTickMsg:
		h, hcmd := m.header.Update(header.SpinnerTickMsg{})
		m.header = h
		cmds = append(cmds, hcmd)

		if m.welcome != nil {
			w, wcmd := m.welcome.Update(msg)
			m.welcome = &w
			cmds = append(cmds, wcmd)
		}

		// Keep ticking while loading so the header spinner animates.
		if m.header.IsLoading() {
			cmds = append(cmds, tea.Tick(80*time.Millisecond, func(time.Time) tea.Msg {
				return tui.SpinnerTickMsg{}
			}))
		}
		return m, tea.Batch(cmds...)

	case tui.LogLinesMsg:
		d, dcmd := m.detail.Update(msg)
		m.detail = d
		cmds = append(cmds, dcmd)
		return m, tea.Batch(cmds...)

	case tui.NewLogFileMsg:
		d, dcmd := m.detail.Update(msg)
		m.detail = d
		cmds = append(cmds, dcmd)
		return m, tea.Batch(cmds...)

	case tui.ErrorMsg:
		m.header.SetLoading(false)
		m.appendError(msg.Filename, sanitizeErrorLine(msg.Message))
		return m, nil

	case tui.ErrorClearedMsg:
		m.clearErrorsByFilename(msg.Filename)
		return m, nil

	case tui.CopiedMsg:
		cmds = append(cmds, m.notify.Push("Copied."))
		return m, tea.Batch(cmds...)

	case tui.InitCompleteMsg:
		if m.welcome != nil {
			w, wcmd := m.welcome.Update(msg)
			m.welcome = &w
			cmds = append(cmds, wcmd)
		}
		if msg.Err == nil && msg.Dir != "" {
			m.entryState = StateCold
			m.welcome = nil
			m.worktreeDir = msg.Dir
			wolfDir := filepath.Join(msg.Dir, ".wolfcastle")
			m.daemonRepo = daemon.NewDaemonRepository(wolfDir)
			m.store = storeFromWolfcastleDir(wolfDir)
			m.header.SetLoading(true)
			cmds = append(cmds, m.detectEntryState(), m.startWatcher(), m.startPoller(), m.loadInitialState())
		}
		return m, tea.Batch(cmds...)

	case welcome.ConnectInstanceMsg:
		// User selected a running session from the welcome screen.
		m.entryState = StateLive
		m.welcome = nil
		m.prevIndex = nil
		m.prevNodes = make(map[string]*state.NodeState)
		m.notify = notify.NewNotificationModel()
		m.worktreeDir = msg.Entry.Worktree
		wolfDir := filepath.Join(msg.Entry.Worktree, ".wolfcastle")
		m.daemonRepo = daemon.NewDaemonRepository(wolfDir)
		m.store = storeFromWolfcastleDir(wolfDir)
		m.instances, _ = instance.List()
		for i, inst := range m.instances {
			if inst.PID == msg.Entry.PID {
				m.activeInstanceIndex = i
				break
			}
		}
		m.header.SetInstances(m.instances, m.activeInstanceIndex)
		m.header.SetLoading(true)
		m.propagateSize()
		cmds = append(cmds, m.detectEntryState(), m.startWatcher(), m.startPoller(), m.loadInitialState())
		return m, tea.Batch(cmds...)

	case tui.WorktreeGoneMsg:
		m.switching = false
		m.switchLabel = ""
		m.header.SetStatusHint("")
		// Remove the gone instance from the list.
		filtered := m.instances[:0]
		for _, inst := range m.instances {
			if inst.PID != msg.Entry.PID || inst.Worktree != msg.Entry.Worktree {
				filtered = append(filtered, inst)
			}
		}
		m.instances = filtered
		if m.activeInstanceIndex >= len(m.instances) && len(m.instances) > 0 {
			m.activeInstanceIndex = len(m.instances) - 1
		}
		m.header.SetInstances(m.instances, m.activeInstanceIndex)
		m.appendError("instance", sanitizeErrorLine(fmt.Sprintf("Worktree no longer exists: %s", msg.Entry.Worktree)))
		return m, nil

	case tui.ToastMsg:
		cmd := m.notify.Push(msg.Text)
		return m, cmd

	case notify.ToastDismissMsg:
		n, ncmd := m.notify.Update(msg)
		m.notify = n
		cmds = append(cmds, ncmd)
		return m, tea.Batch(cmds...)

	case tui.PollTickMsg:
		m.tree.CleanCache()
		// Re-read state and schedule the next tick.
		var pollCmds []tea.Cmd
		pollCmds = append(pollCmds, m.pollState(), m.detectEntryState(), m.schedulePollTick())
		return m, tea.Batch(pollCmds...)

	case tui.InboxUpdatedMsg:
		if msg.Inbox != nil {
			m.detail.SetDashboardInbox(msg.Inbox.Items)
		}
		d, dcmd := m.detail.Update(msg)
		m.detail = d
		cmds = append(cmds, dcmd)
		return m, tea.Batch(cmds...)

	case tui.AddInboxItemCmd:
		return m, m.addInboxItem(msg.Text)

	case tui.InboxItemAddedMsg:
		return m, m.loadInbox()

	case tui.InboxAddFailedMsg:
		errMsg := fmt.Sprintf("Inbox write failed: %s.", msg.Err)
		errText := msg.Err.Error()
		if strings.Contains(errText, "lock") || strings.Contains(errText, "timed out") {
			errMsg = "Failed to write inbox. Another process may hold the lock."
		}
		m.appendError("inbox", sanitizeErrorLine(errMsg))
		return m, nil
	}

	return m, nil
}

// View builds the full terminal output.
func (m TUIModel) View() tea.View {
	// Re-propagate size on every render so sub-models reflect any
	// runtime layout changes (tab bar appearance, error bar visibility).
	m.propagateSize()
	rendered := m.renderLayout()
	v := tea.NewView(rendered)
	v.AltScreen = true
	v.WindowTitle = "WOLFCASTLE"
	return v
}

// ---------------------------------------------------------------------------
// Layout
// ---------------------------------------------------------------------------

func (m TUIModel) renderLayout() string {
	if m.width < 20 || m.height < 5 {
		return lipgloss.Place(m.width, m.height, lipgloss.Center, lipgloss.Center, "Terminal too small.")
	}

	if m.entryState == StateWelcome && m.welcome != nil {
		return m.welcome.View()
	}

	headerView := m.header.View()
	footerView := m.footer.View()

	headerLines := strings.Count(headerView, "\n") + 1
	contentHeight := m.height - headerLines - 1

	errorBar := m.renderErrorBar()
	if errorBar != "" {
		errorLines := strings.Count(errorBar, "\n") + 1
		contentHeight -= errorLines
	}
	// Match the floor in propagateSize: panes need 2 rows for borders
	// plus at least 1 inner row, otherwise lipgloss produces malformed
	// output that leaves see-through gaps in the screen.
	if contentHeight < 3 {
		contentHeight = 3
	}

	contentView := m.renderContent(contentHeight)

	if m.help.IsActive() {
		contentView = m.help.View()
	}

	var parts []string
	parts = append(parts, headerView)
	parts = append(parts, contentView)
	if errorBar != "" {
		parts = append(parts, errorBar)
	}
	parts = append(parts, footerView)

	return strings.Join(parts, "\n")
}

func (m TUIModel) renderContent(contentHeight int) string {
	if !m.treeVisible || m.width < 60 {
		content := m.detail.View()
		if m.search.IsActive() && m.search.PaneType() == int(PaneDetail) {
			content += "\n" + m.search.View()
		}
		detailStyle := m.borderStyle(PaneDetail).
			Width(m.width).
			Height(contentHeight)
		rendered := detailStyle.Render(content)
		if m.notify.HasToasts() {
			rendered = m.overlayToasts(rendered, m.width)
		}
		return rendered
	}

	// In lipgloss v2, Width on a bordered style is the total rendered
	// width including borders. The two panes must sum to m.width.
	treeWidth := m.width * 30 / 100
	if treeWidth < 24 {
		treeWidth = 24
	}
	detailWidth := m.width - treeWidth
	if detailWidth < 10 {
		detailWidth = 10
	}

	treeContent := m.tree.View()
	if m.search.IsActive() && m.search.PaneType() == int(PaneTree) {
		treeContent += "\n" + m.search.View()
	}

	treePaneStyle := m.borderStyle(PaneTree).
		Width(treeWidth).
		Height(contentHeight).
		MaxHeight(contentHeight)
	treePane := treePaneStyle.Render(treeContent)

	detailContent := m.detail.View()
	if m.search.IsActive() && m.search.PaneType() == int(PaneDetail) {
		detailContent += "\n" + m.search.View()
	}
	detailPaneStyle := m.borderStyle(PaneDetail).
		Width(detailWidth).
		Height(contentHeight).
		MaxHeight(contentHeight)
	detailPane := detailPaneStyle.Render(detailContent)

	joined := lipgloss.JoinHorizontal(lipgloss.Top, treePane, detailPane)
	if m.notify.HasToasts() {
		joined = m.overlayToasts(joined, m.width)
	}
	return joined
}

func (m TUIModel) borderStyle(pane FocusedPane) lipgloss.Style {
	if m.focused == pane {
		return tui.FocusedBorderStyle
	}
	return tui.UnfocusedBorderStyle
}

func (m TUIModel) renderErrorBar() string {
	if len(m.errors) == 0 {
		return ""
	}

	maxShow := 3
	shown := m.errors
	if len(shown) > maxShow {
		shown = shown[:maxShow]
	}

	var lines []string
	for _, e := range shown {
		lines = append(lines, tui.ErrorBarStyle.Render(fmt.Sprintf(" %s: %s", e.filename, e.message)))
	}
	if overflow := len(m.errors) - maxShow; overflow > 0 {
		lines = append(lines, tui.ErrorBarStyle.Render(fmt.Sprintf(" +%d more errors", overflow)))
	}
	return strings.Join(lines, "\n")
}

// ---------------------------------------------------------------------------
// Focus & search helpers
// ---------------------------------------------------------------------------

func (m *TUIModel) cycleFocus() {
	if !m.treeVisible {
		return
	}
	if m.focused == PaneTree {
		m.focused = PaneDetail
	} else {
		m.focused = PaneTree
	}
	m.syncFocus()
}

func (m *TUIModel) syncFocus() {
	m.tree.SetFocused(m.focused == PaneTree)
	m.detail.SetFocused(m.focused == PaneDetail)
	m.footer.SetFocus(int(m.focused))
}

func (m *TUIModel) computeTreeSearchMatches() {
	query := strings.ToLower(m.search.Query())
	if query == "" {
		m.search.SetMatches(nil)
		m.tree.SetSearchAddresses(nil, nil)
		return
	}

	// When search was activated from the detail pane, match against detail content.
	if m.search.PaneType() == int(PaneDetail) {
		m.computeDetailSearchMatches(query)
		return
	}

	// Walk the full tree (root index plus per-leaf cached node states)
	// rather than the visible flat list. This is the structural change
	// that lets search find tasks in collapsed leaves and that lets
	// match highlights survive collapse/expand operations: matches are
	// keyed by tree address, not by flat-list row index, and the
	// highlight set is computed from the index even when the matching
	// rows are not currently visible.
	idx := m.tree.Index()
	if idx == nil {
		m.search.SetMatches(nil)
		m.tree.SetSearchAddresses(nil, nil)
		return
	}

	literal := make(map[string]bool)
	var matches []search.SearchMatch

	for addr, entry := range idx.Nodes {
		if strings.Contains(strings.ToLower(entry.Name), query) {
			literal[addr] = true
			matches = append(matches, search.SearchMatch{Address: addr})
		}
		if entry.Type != state.NodeLeaf {
			continue
		}
		ns := m.tree.CachedNode(addr)
		if ns == nil {
			continue
		}
		for _, task := range ns.Tasks {
			if strings.Contains(strings.ToLower(task.Title), query) {
				taskAddr := addr + "/" + task.ID
				literal[taskAddr] = true
				matches = append(matches, search.SearchMatch{Address: taskAddr})
			}
		}
	}

	// Compute the ancestor closure: every address on the path from
	// root down to a literal match. We strip trailing path segments
	// one at a time and add each to the ancestor set, stopping at
	// addresses that are themselves literal matches (those keep the
	// stronger treatment via the literal set).
	ancestor := make(map[string]bool)
	for addr := range literal {
		for {
			slash := strings.LastIndex(addr, "/")
			if slash < 0 {
				break
			}
			addr = addr[:slash]
			if literal[addr] {
				continue
			}
			ancestor[addr] = true
		}
	}

	m.search.SetMatches(matches)
	m.tree.SetSearchAddresses(literal, ancestor)
}

func (m *TUIModel) computeDetailSearchMatches(query string) {
	lines := m.detail.SearchContent()
	var matches []search.SearchMatch
	for i, line := range lines {
		if strings.Contains(strings.ToLower(line), query) {
			matches = append(matches, search.SearchMatch{Row: i})
		}
	}
	m.search.SetMatches(matches)
	// Clear tree highlights when searching detail.
	m.tree.SetSearchAddresses(nil, nil)
}

func (m *TUIModel) jumpTreeToSearchMatch() {
	match, ok := m.search.CurrentMatch()
	if !ok {
		return
	}
	// Address-keyed jumping. If the literal match is currently
	// visible in the flat list, the cursor lands on it directly.
	// If it isn't (the match is inside a collapsed branch), the
	// cursor lands on the deepest visible ancestor on the path to
	// the match. Pure-(a) search behavior: we never auto-expand
	// during typing OR navigation; the user is expected to expand
	// manually after seeing the highlighted path.
	if match.Address == "" {
		// Detail-pane match still uses Row.
		m.tree.SetCursor(match.Row)
		return
	}
	target := match.Address
	for target != "" {
		for i, row := range m.tree.FlatList() {
			if row.Addr == target {
				m.tree.SetCursor(i)
				return
			}
		}
		// Strip one segment and try again with the parent.
		slash := strings.LastIndex(target, "/")
		if slash < 0 {
			return
		}
		target = target[:slash]
	}
}

func (m TUIModel) handleCopy() tea.Cmd {
	var text string

	switch m.focused {
	case PaneDetail:
		// Copy the entire visible detail pane content as plain text.
		text = ansi.Strip(m.detail.View())
	case PaneTree:
		text = m.tree.SelectedAddr()
	}

	if text == "" {
		return nil
	}
	// Use the host clipboard tool (pbcopy/xclip/etc.) instead of OSC 52,
	// which silently drops or truncates payloads under several common
	// terminal and tmux configurations. tea.SetClipboard is kept as a
	// best-effort fallback for terminals where the host tool isn't
	// available.
	return tea.Batch(
		func() tea.Msg {
			_ = clipboard.WriteSystem(text)
			return tui.CopiedMsg{}
		},
		tea.SetClipboard(text),
	)
}

// loadDetailForSelection inspects the currently selected tree row and loads
// the appropriate detail view: node detail for orchestrators and leaves,
// task detail for tasks.
func (m *TUIModel) loadDetailForSelection() {
	row := m.tree.SelectedRow()
	if row == nil {
		return
	}

	idx := m.tree.Index()
	if idx == nil {
		return
	}

	if row.IsTask {
		// Task addr format: "nodeAddr/taskID" where the last segment is the task ID.
		lastSlash := strings.LastIndex(row.Addr, "/")
		if lastSlash < 0 {
			return
		}
		nodeAddr := row.Addr[:lastSlash]
		taskID := row.Addr[lastSlash+1:]

		ns := m.tree.CachedNode(nodeAddr)
		if ns == nil {
			return
		}
		for i := range ns.Tasks {
			if ns.Tasks[i].ID == taskID {
				m.detail.LoadTaskDetail(nodeAddr, taskID, &ns.Tasks[i])
				return
			}
		}
		return
	}

	// Node row: load node detail.
	entry, ok := idx.Nodes[row.Addr]
	if !ok {
		return
	}

	ns := m.tree.CachedNode(row.Addr)
	if ns == nil && m.store != nil {
		// Load the full node state from disk for detail view.
		loaded, err := m.store.ReadNode(row.Addr)
		if err == nil {
			ns = loaded
		}
	}
	if ns == nil {
		// Last resort: minimal stub from the index entry.
		ns = &state.NodeState{
			Name:  entry.Name,
			Type:  entry.Type,
			State: entry.State,
		}
	}

	isTarget := m.tree.SelectedAddr() == m.currentTarget()
	m.detail.LoadNodeDetail(row.Addr, ns, &entry, isTarget)
}

func (m TUIModel) currentTarget() string {
	// The header tracks the current target through daemon status. For now,
	// return empty; the watcher will set this when target tracking lands.
	return ""
}

func (m TUIModel) handleRefresh() tea.Cmd {
	var cmds []tea.Cmd

	if m.store != nil {
		store := m.store
		cmds = append(cmds, func() tea.Msg {
			idx, err := store.ReadIndex()
			if err != nil {
				return tui.ErrorMsg{
					Filename: "state.json",
					Message:  "State corruption detected: state.json. Run wolfcastle doctor.",
				}
			}
			return tui.StateUpdatedMsg{Index: idx}
		})
	}

	if m.daemonRepo != nil {
		cmds = append(cmds, m.detectEntryState())
	}

	return tea.Batch(cmds...)
}

// ---------------------------------------------------------------------------
// Inbox helpers
// ---------------------------------------------------------------------------

func (m TUIModel) loadInbox() tea.Cmd {
	store := m.store
	if store == nil {
		return nil
	}
	return func() tea.Msg {
		inbox, err := store.ReadInbox()
		if err != nil {
			return tui.ErrorMsg{
				Filename: "inbox.json",
				Message:  "Inbox unreadable. Run wolfcastle doctor.",
			}
		}
		return tui.InboxUpdatedMsg{Inbox: inbox}
	}
}

func (m TUIModel) addInboxItem(text string) tea.Cmd {
	store := m.store
	if store == nil {
		return nil
	}
	return func() tea.Msg {
		err := store.MutateInbox(func(f *state.InboxFile) error {
			f.Items = append(f.Items, state.InboxItem{
				Timestamp: time.Now().UTC().Format(time.RFC3339),
				Text:      text,
				Status:    state.InboxNew,
			})
			return nil
		})
		if err != nil {
			return tui.InboxAddFailedMsg{Err: err}
		}
		return tui.InboxItemAddedMsg{}
	}
}

// ---------------------------------------------------------------------------
// Size propagation
// ---------------------------------------------------------------------------

func (m *TUIModel) propagateSize() {
	m.header.SetSize(m.width)
	m.footer.SetSize(m.width)
	m.help.SetSize(m.width, m.height)
	m.notify.SetSize(m.width)

	if m.welcome != nil {
		m.welcome.SetSize(m.width, m.height)
	}

	// Use the actual rendered header line count, not a hardcoded
	// estimate, so the tab bar (3 lines instead of 2) is accounted for.
	headerLines := strings.Count(m.header.View(), "\n") + 1
	contentHeight := m.height - headerLines - 1
	if errorBar := m.renderErrorBar(); errorBar != "" {
		contentHeight -= strings.Count(errorBar, "\n") + 1
	}
	// Floor content height at the border budget (2 rows) + 1 inner row
	// so sub-models never receive a non-positive inner height. Without
	// this floor, a tall error bar shrinks contentHeight below 3 and
	// `contentHeight - 2` lands at zero or negative, which produces
	// malformed lipgloss output and leaves see-through gaps where the
	// panes should be.
	if contentHeight < 3 {
		contentHeight = 3
	}
	innerHeight := contentHeight - 2

	if !m.treeVisible || m.width < 60 {
		m.tree.SetSize(0, 0)
		// Sub-model gets the inner dimensions (full width minus 2 border cells).
		m.detail.SetSize(maxInt(m.width-2, 1), innerHeight)
		return
	}

	treeWidth := m.width * 30 / 100
	if treeWidth < 24 {
		treeWidth = 24
	}
	detailWidth := m.width - treeWidth
	if detailWidth < 10 {
		detailWidth = 10
	}

	// Sub-models receive inner dimensions; panes wrap with 2 border cells.
	m.tree.SetSize(maxInt(treeWidth-2, 1), innerHeight)
	m.detail.SetSize(maxInt(detailWidth-2, 1), innerHeight)
}

func maxInt(a, b int) int {
	if a > b {
		return a
	}
	return b
}

// ---------------------------------------------------------------------------
// Error helpers
// ---------------------------------------------------------------------------

func (m *TUIModel) clearErrorsByFilename(filename string) {
	filtered := m.errors[:0]
	for _, e := range m.errors {
		if e.filename != filename {
			filtered = append(filtered, e)
		}
	}
	m.errors = filtered
}

// maxErrorEntries caps how many error rows can pile up in memory. The
// error bar visually shows at most maxShow (3) plus a "+N more" line, so
// any value above that is just protecting layout math from runaway growth.
const maxErrorEntries = 8

// appendError adds an error to the bar with two safety nets: it skips
// the append if the most recent entry is identical (so spamming a key
// that re-fails doesn't stack duplicates), and it caps the total number
// of entries so the bar can't grow without bound and starve the panes
// of vertical space.
func (m *TUIModel) appendError(filename, message string) {
	if n := len(m.errors); n > 0 {
		last := m.errors[n-1]
		if last.filename == filename && last.message == message {
			return
		}
	}
	m.errors = append(m.errors, errorEntry{filename: filename, message: message})
	if len(m.errors) > maxErrorEntries {
		m.errors = m.errors[len(m.errors)-maxErrorEntries:]
	}
}

// sanitizeErrorLine collapses any error string into a single trimmed
// line so it can't blow up the error bar's height (which is what made
// the panes collapse on repeated start failures). Trailing context past
// a reasonable cap is dropped.
func sanitizeErrorLine(s string) string {
	s = strings.ReplaceAll(s, "\r\n", " ")
	s = strings.ReplaceAll(s, "\n", " ")
	s = strings.ReplaceAll(s, "\r", " ")
	// Squash runs of whitespace.
	for strings.Contains(s, "  ") {
		s = strings.ReplaceAll(s, "  ", " ")
	}
	s = strings.TrimSpace(s)
	const maxLen = 200
	if len(s) > maxLen {
		s = s[:maxLen-1] + "…"
	}
	return s
}

// ---------------------------------------------------------------------------
// Init commands
// ---------------------------------------------------------------------------

func (m TUIModel) detectEntryState() tea.Cmd {
	repo := m.daemonRepo
	worktreeDir := m.worktreeDir
	return func() tea.Msg {
		if repo == nil {
			return nil
		}

		instances, _ := instance.List()

		// Try to resolve the specific instance for this worktree.
		var pid int
		var branch string
		var running, draining bool

		if entry, err := instance.Resolve(worktreeDir); err == nil {
			pid = entry.PID
			branch = entry.Branch
			running = isProcessRunning(entry.PID)
			draining = repo.HasDrainFile()
		} else {
			// Fallback to repository-level check.
			running = repo.IsAlive()
			draining = repo.HasDrainFile()
		}

		status := "standing down"
		if running && !draining {
			status = "hunting"
		} else if running && draining {
			status = "draining"
		}

		msg := tui.DaemonStatusMsg{
			Status:     status,
			Branch:     branch,
			Worktree:   worktreeDir,
			PID:        pid,
			IsRunning:  running,
			IsDraining: draining,
			Instances:  instances,
		}

		// Read daemon activity snapshot for last-activity and current target.
		wolfDir := filepath.Join(worktreeDir, ".wolfcastle")
		if activity := daemon.LoadDaemonActivity(wolfDir); activity != nil {
			msg.LastActivity = activity.LastActivityAt
			msg.CurrentNode = activity.CurrentNode
			msg.CurrentTask = activity.CurrentTask
		}

		return msg
	}
}

// isProcessRunning checks if a PID is alive (signal 0).
func isProcessRunning(pid int) bool {
	return isProcessAlive(pid)
}

// storeFromWolfcastleDir loads config and identity from a .wolfcastle
// directory and returns a Store pointed at the correct namespace path.
// Returns nil if identity can't be resolved (the TUI will run without
// node data in that case).
func storeFromWolfcastleDir(wolfDir string) *state.Store {
	repo := config.NewRepository(wolfDir)
	cfg, err := repo.Load()
	if err != nil {
		return nil
	}
	id, err := config.IdentityFromConfig(cfg)
	if err != nil {
		return nil
	}
	return state.NewStore(id.ProjectsDir(wolfDir), state.DefaultLockTimeout)
}

func (m *TUIModel) startWatcher() tea.Cmd {
	store := m.store
	repo := m.daemonRepo
	events := m.watcherEvents
	return func() tea.Msg {
		if store == nil {
			return nil
		}
		logDir := ""
		instanceDir := ""
		if repo != nil {
			logDir = repo.LogDir()
		}
		if dir, err := instanceRegistryDir(); err == nil {
			instanceDir = dir
		}
		w := tui.NewWatcher(store, logDir, instanceDir, events)
		// Prefer fsnotify when available; fall back to mtime polling
		// if the OS watcher cannot be initialized. Either path drives
		// the same WatcherMsg envelope back through the model.
		if err := w.Start(); err != nil {
			w.StartPolling()
		}
		// Eagerly walk the index, populate the per-leaf cache, and
		// add an fsnotify subscription for each leaf so the cache
		// stays fresh as the daemon writes state.json updates.
		// Without this, the per-task glyphs in the tree go stale
		// the moment the daemon transitions a task, and search
		// can't find tasks in unexpanded leaves.
		_ = w.EagerPrefetchAndSubscribe()
		m.watcher = w
		return nil
	}
}

func (m TUIModel) startPoller() tea.Cmd {
	return m.schedulePollTick()
}

func (m TUIModel) schedulePollTick() tea.Cmd {
	return tea.Tick(2*time.Second, func(time.Time) tea.Msg {
		return tui.PollTickMsg{}
	})
}

// pollState re-reads the root index and returns a StateUpdatedMsg if
// the data has changed. This is the polling fallback that keeps the
// TUI in sync with daemon activity.
func (m TUIModel) pollState() tea.Cmd {
	store := m.store
	return func() tea.Msg {
		if store == nil {
			return nil
		}
		idx, err := store.ReadIndex()
		if err != nil {
			return nil // silent on poll errors; next tick will retry
		}
		return tui.StateUpdatedMsg{Index: idx}
	}
}

func (m TUIModel) loadInitialState() tea.Cmd {
	store := m.store
	return func() tea.Msg {
		if store == nil {
			return nil
		}
		idx, err := store.ReadIndex()
		if err != nil {
			return tui.ErrorMsg{
				Filename: "state.json",
				Message:  "State corruption detected: state.json. Run wolfcastle doctor.",
			}
		}
		return tui.StateUpdatedMsg{Index: idx}
	}
}

// ---------------------------------------------------------------------------
// Daemon control (Phase 3)
// ---------------------------------------------------------------------------

func (m *TUIModel) handleToggleDaemon() tea.Cmd {
	if m.daemonStarting || m.daemonStopping {
		return nil
	}
	if m.entryState == StateLive {
		return m.stopCurrentDaemon()
	}
	return m.startDaemon()
}

func (m *TUIModel) startDaemon() tea.Cmd {
	m.daemonStarting = true
	m.header.SetStatusHint("Starting daemon...")
	m.header.SetLoading(true)
	dir := m.worktreeDir
	return func() tea.Msg {
		// Re-exec the same binary that's running the TUI rather than
		// trusting PATH, which may resolve to a different version (or
		// nothing at all if the TUI was launched from a non-shell parent).
		exe, err := os.Executable()
		if err != nil {
			exe = "wolfcastle"
		}
		cmd := exec.Command(exe, "start", "-d")
		cmd.Dir = dir
		// Capture stderr so we can surface the real failure reason
		// instead of the bare "exit status 1" returned by exec.
		var stderr bytes.Buffer
		cmd.Stderr = &stderr
		if runErr := cmd.Run(); runErr != nil {
			return tui.DaemonStartFailedMsg{
				Err:    runErr,
				Stderr: stderr.String(),
			}
		}
		// Re-read instances to find the new entry.
		instances, _ := instance.List()
		for _, inst := range instances {
			if inst.Worktree == dir {
				return tui.DaemonStartedMsg{Entry: inst}
			}
		}
		return tui.DaemonStartedMsg{}
	}
}

func (m *TUIModel) stopCurrentDaemon() tea.Cmd {
	m.daemonStopping = true
	m.header.SetStatusHint("Stopping daemon...")

	// Find the PID of the current instance's daemon.
	var pid int
	if m.activeInstanceIndex < len(m.instances) {
		pid = m.instances[m.activeInstanceIndex].PID
	}
	if pid == 0 {
		// Fallback: try resolving from worktree dir.
		if entry, err := instance.Resolve(m.worktreeDir); err == nil {
			pid = entry.PID
		}
	}
	if pid == 0 {
		m.daemonStopping = false
		m.header.SetStatusHint("")
		return func() tea.Msg {
			return tui.DaemonStopFailedMsg{Err: fmt.Errorf("no daemon PID found")}
		}
	}

	return func() tea.Msg {
		if err := killProcess(pid, syscall.SIGTERM); err != nil {
			return tui.DaemonStopFailedMsg{Err: fmt.Errorf("sending SIGTERM to PID %d: %w", pid, err)}
		}
		// Wait up to 5 seconds for the process to exit.
		deadline := time.Now().Add(5 * time.Second)
		for time.Now().Before(deadline) {
			if !daemon.IsProcessRunning(pid) {
				return tui.DaemonStoppedMsg{}
			}
			time.Sleep(200 * time.Millisecond)
		}
		//nolint:staticcheck // ST1005: user-facing TUI message displayed in toast notification
		return tui.DaemonStopFailedMsg{Err: fmt.Errorf("Daemon not responding. Try wolfcastle stop --force.")}
	}
}

func (m *TUIModel) handleStopAll() tea.Cmd {
	if m.daemonStopping {
		return nil
	}
	m.daemonStopping = true
	m.header.SetStatusHint("Stopping all daemons...")
	instances := make([]instance.Entry, len(m.instances))
	copy(instances, m.instances)
	return func() tea.Msg {
		var lastErr error
		for _, inst := range instances {
			if err := killProcess(inst.PID, syscall.SIGTERM); err != nil {
				lastErr = fmt.Errorf("sending SIGTERM to PID %d: %w", inst.PID, err)
			}
		}
		// Wait up to 5 seconds for all to exit.
		deadline := time.Now().Add(5 * time.Second)
		for time.Now().Before(deadline) {
			allDead := true
			for _, inst := range instances {
				if daemon.IsProcessRunning(inst.PID) {
					allDead = false
					break
				}
			}
			if allDead {
				return tui.DaemonStoppedMsg{}
			}
			time.Sleep(200 * time.Millisecond)
		}
		if lastErr != nil {
			return tui.DaemonStopFailedMsg{Err: lastErr}
		}
		//nolint:staticcheck // ST1005: user-facing TUI message displayed in toast notification
		return tui.DaemonStopFailedMsg{Err: fmt.Errorf("Daemon not responding. Try wolfcastle stop --force.")}
	}
}

func (m *TUIModel) handleSwitchInstance(delta int) tea.Cmd {
	if len(m.instances) < 2 {
		return nil
	}
	next := (m.activeInstanceIndex + delta + len(m.instances)) % len(m.instances)
	m.activeInstanceIndex = next
	switchCmd := m.switchInstance(m.instances[next])
	// Kick off spinner animation.
	spinnerTick := tea.Tick(80*time.Millisecond, func(time.Time) tea.Msg {
		return tui.SpinnerTickMsg{}
	})
	return tea.Batch(switchCmd, spinnerTick)
}

func (m *TUIModel) switchInstance(entry instance.Entry) tea.Cmd {
	m.switching = true
	label := filepath.Base(entry.Worktree)
	if entry.Branch != "" && entry.Branch != label {
		label += " (" + entry.Branch + ")"
	}
	m.switchLabel = label
	m.header.SetStatusHint("Switching...")
	m.header.SetLoading(true)

	// Clear stale data from the previous instance immediately so the
	// user doesn't see old stats while the new instance loads.
	m.tree.Reset()
	m.detail.Reset()
	m.notify = notify.NewNotificationModel()
	m.prevIndex = nil
	m.prevNodes = make(map[string]*state.NodeState)

	// Find the index of this entry in our instances list.
	for i, inst := range m.instances {
		if inst.PID == entry.PID {
			m.activeInstanceIndex = i
			break
		}
	}
	m.header.SetInstances(m.instances, m.activeInstanceIndex)

	worktree := entry.Worktree
	return func() tea.Msg {
		// Verify the worktree directory exists.
		info, err := os.Stat(worktree)
		if err != nil || !info.IsDir() {
			return tui.WorktreeGoneMsg{Entry: entry}
		}

		wolfDir := filepath.Join(worktree, ".wolfcastle")
		store := storeFromWolfcastleDir(wolfDir)
		if store == nil {
			return tui.ErrorMsg{
				Filename: "state.json",
				Message:  fmt.Sprintf("Failed to resolve identity for %s", worktree),
			}
		}
		idx, err := store.ReadIndex()
		if err != nil {
			return tui.ErrorMsg{
				Filename: "state.json",
				Message:  fmt.Sprintf("Failed to read state for %s", worktree),
			}
		}
		return tui.InstanceSwitchedMsg{
			Index: idx,
			Entry: entry,
		}
	}
}

// ---------------------------------------------------------------------------
// Toast notification helpers (Phase 5)
// ---------------------------------------------------------------------------

// diffNodeForToasts compares old and new node states and generates toast
// notifications for task status changes and newly opened audit gaps.
func (m *TUIModel) diffNodeForToasts(addr string, old, new *state.NodeState) []tea.Cmd {
	var cmds []tea.Cmd

	// Build a map of old task statuses for quick lookup.
	oldTasks := make(map[string]state.NodeStatus, len(old.Tasks))
	for _, t := range old.Tasks {
		oldTasks[t.ID] = t.State
	}

	for _, t := range new.Tasks {
		oldStatus, existed := oldTasks[t.ID]
		if !existed || oldStatus == t.State {
			continue
		}
		switch t.State {
		case state.StatusComplete:
			cmds = append(cmds, m.notify.Push(fmt.Sprintf("Target eliminated: %s/%s", addr, t.ID)))
		case state.StatusBlocked:
			cmds = append(cmds, m.notify.Push(fmt.Sprintf("Blocked: %s/%s", addr, t.ID)))
		}
	}

	// Count open gaps in old vs new.
	oldOpen := 0
	for _, g := range old.Audit.Gaps {
		if g.Status == state.GapOpen {
			oldOpen++
		}
	}
	newOpen := 0
	for _, g := range new.Audit.Gaps {
		if g.Status == state.GapOpen {
			newOpen++
		}
	}
	if newOpen > oldOpen {
		// Find gaps that are new (not present in old set).
		oldGaps := make(map[string]bool, len(old.Audit.Gaps))
		for _, g := range old.Audit.Gaps {
			oldGaps[g.ID] = true
		}
		for _, g := range new.Audit.Gaps {
			if g.Status == state.GapOpen && !oldGaps[g.ID] {
				cmds = append(cmds, m.notify.Push(fmt.Sprintf("Gap opened: %s %s", addr, g.Description)))
			}
		}
	}

	return cmds
}

// overlayToasts places the notification toast stack in the upper-right
// corner of the content using ANSI-aware truncation so styled content
// isn't garbled.
func (m TUIModel) overlayToasts(content string, width int) string {
	m.notify.SetSize(width)
	toastView := m.notify.View()
	if toastView == "" {
		return content
	}

	contentLines := strings.Split(content, "\n")
	toastLines := strings.Split(toastView, "\n")

	for i, tl := range toastLines {
		tw := lipgloss.Width(tl)
		if tw == 0 {
			continue
		}
		if i >= len(contentLines) {
			break
		}
		// ANSI-aware truncation: cut the content line short to make
		// room for the toast on the right, preserving escape sequences.
		room := width - tw
		if room < 0 {
			room = 0
		}
		cl := ansi.Truncate(contentLines[i], room, "")
		// Pad the gap between truncated content and the toast.
		gap := room - lipgloss.Width(cl)
		if gap < 0 {
			gap = 0
		}
		contentLines[i] = cl + strings.Repeat(" ", gap) + tl
	}

	return strings.Join(contentLines, "\n")
}

// instanceRegistryDir resolves the instance registry path. It mirrors the
// logic in the instance package but is called here so the watcher can
// observe the directory for changes.
func instanceRegistryDir() (string, error) {
	if instance.RegistryDirOverride != "" {
		return instance.RegistryDirOverride, nil
	}
	home, err := os.UserHomeDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(home, ".wolfcastle", "instances"), nil
}
