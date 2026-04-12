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
// messages between them, manages focus, and computes layout. Per-directory
// state lives on Tab; TUIModel holds only shared chrome and the tab list.
type TUIModel struct {
	width  int
	height int

	header  header.Model
	footer  footer.Model
	welcome *welcome.Model
	help    help.Model
	notify  notify.NotificationModel

	activeModal ActiveModal
	daemonModal DaemonModalModel
	tabPicker   TabPickerModel

	originalCWD string // the directory the TUI was launched from; never mutated
	version     string

	// Instance tracking (global, not per-tab).
	instances []instance.Entry

	// Tab management.
	tabs        []Tab
	activeTabID int
	nextTabID   int
}

// activeTab returns a pointer to the currently active tab, or nil.
func (m *TUIModel) activeTab() *Tab {
	for i := range m.tabs {
		if m.tabs[i].ID == m.activeTabID {
			return &m.tabs[i]
		}
	}
	return nil
}

// tabByID returns a pointer to the tab with the given ID, or nil.
func (m *TUIModel) tabByID(id int) *Tab {
	for i := range m.tabs {
		if m.tabs[i].ID == id {
			return &m.tabs[i]
		}
	}
	return nil
}

// createTab creates a new tab, adds it to the list, and returns it.
func (m *TUIModel) createTab(worktreeDir string, store *state.Store, daemonRepo *daemon.Repository) *Tab {
	t := newTab(m.nextTabID, worktreeDir, store, daemonRepo)
	m.nextTabID++
	m.tabs = append(m.tabs, *t)
	return &m.tabs[len(m.tabs)-1]
}

// NewTUIModel creates a TUIModel shell. Tab creation happens in Init()
// or via user actions. The constructor no longer takes store/daemonRepo.
func NewTUIModel(worktreeDir, version string) TUIModel {
	m := TUIModel{
		header:      header.NewModel(version),
		footer:      footer.NewModel(),
		help:        help.NewModel(),
		notify:      notify.NewNotificationModel(),
		originalCWD: worktreeDir,
		version:     version,
	}
	return m
}

// Init returns the batch of startup commands. It creates the initial tab
// based on CWD state: if .wolfcastle/ exists, the tab starts in Cold state
// with a watcher; otherwise, a Welcome-state tab is created.
func (m TUIModel) Init() tea.Cmd {
	var cmds []tea.Cmd

	wolfDir := filepath.Join(m.originalCWD, ".wolfcastle")
	if info, err := os.Stat(wolfDir); err == nil && info.IsDir() {
		store := storeFromWolfcastleDir(wolfDir)
		daemonRepo := daemon.NewRepository(wolfDir)
		tab := m.createTab(m.originalCWD, store, daemonRepo)
		m.activeTabID = tab.ID
		m.header.SetLoading(true)
		if startCmd := tab.Start(); startCmd != nil {
			cmds = append(cmds, startCmd)
		}
		cmds = append(cmds, m.detectEntryState())
	} else {
		// No .wolfcastle in CWD. Try instance resolution.
		if entry, resolveErr := instance.Resolve(m.originalCWD); resolveErr == nil {
			wDir := entry.Worktree
			wf := filepath.Join(wDir, ".wolfcastle")
			store := storeFromWolfcastleDir(wf)
			daemonRepo := daemon.NewRepository(wf)
			tab := m.createTab(wDir, store, daemonRepo)
			m.activeTabID = tab.ID
			m.header.SetLoading(true)
			if startCmd := tab.Start(); startCmd != nil {
				cmds = append(cmds, startCmd)
			}
			cmds = append(cmds, m.detectEntryState())
		} else {
			// Welcome state: no local project, no resolved instance.
			tab := m.createTab(m.originalCWD, nil, nil)
			m.activeTabID = tab.ID
			instances, _ := instance.List()
			w := welcome.NewModel(m.originalCWD, instances)
			m.welcome = &w
		}
	}

	// Global instance discovery poll.
	cmds = append(cmds, m.scheduleGlobalPollTick())

	return tea.Batch(cmds...)
}

// scheduleGlobalPollTick fires a global poll tick for instance discovery.
func (m TUIModel) scheduleGlobalPollTick() tea.Cmd {
	return tea.Tick(2*time.Second, func(time.Time) tea.Msg {
		return tui.PollTickMsg{}
	})
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

		tab := m.activeTab()

		// Detail pane capturing input (e.g., inbox text field) routes
		// directly to the detail model, bypassing all global bindings.
		if tab != nil && tab.Detail.IsCapturingInput() {
			d, cmd := tab.Detail.Update(msg)
			tab.Detail = d
			return m, cmd
		}

		// Welcome screen swallows everything else.
		if m.welcome != nil && tab != nil && tab.EntryState == StateWelcome {
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

		// Modal overlay absorbs all keys while active.
		if m.activeModal != ModalNone {
			return m.updateActiveModal(msg)
		}

		// Active search bar captures input.
		if tab != nil && tab.Search.IsActive() {
			prevQuery := tab.Search.Query()
			s, cmd := tab.Search.Update(msg)
			tab.Search = s
			if tab.Search.Query() != prevQuery {
				m.computeTreeSearchMatches()
			}
			return m, cmd
		}

		// Global bindings.
		switch {
		case key.Matches(msg, tui.GlobalKeyMap.Quit):
			return m, tea.Quit

		case key.Matches(msg, tui.GlobalKeyMap.Dashboard):
			if tab != nil {
				tab.Detail.SwitchToDashboard()
				tab.Focused = PaneDetail
				m.syncFocus()
			}
			return m, nil

		case key.Matches(msg, tui.GlobalKeyMap.LogStream):
			m.activeModal = ModalLog
			return m, nil

		case key.Matches(msg, tui.GlobalKeyMap.Inbox):
			m.activeModal = ModalInbox
			return m, m.loadInbox()

		case key.Matches(msg, tui.GlobalKeyMap.ToggleTree):
			if tab != nil {
				tab.TreeVisible = !tab.TreeVisible
				if !tab.TreeVisible && tab.Focused == PaneTree {
					tab.LastFocused = PaneTree
					tab.Focused = PaneDetail
					m.syncFocus()
				} else if tab.TreeVisible && tab.LastFocused == PaneTree {
					tab.Focused = PaneTree
					m.syncFocus()
				}
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
			if tab != nil {
				tab.Search.Activate(int(tab.Focused))
			}
			return m, nil

		case key.Matches(msg, tui.GlobalKeyMap.Copy):
			return m, m.handleCopy()

		case key.Matches(msg, tui.DaemonKeyMap.ToggleDaemon):
			if tab == nil {
				return m, nil
			}
			if tab.DaemonStarting || tab.DaemonStopping {
				return m, nil
			}
			action := "start"
			isRunning := tab.EntryState == StateLive
			if isRunning {
				action = "stop"
			}
			var pid int
			var branch string
			var isDraining bool
			worktree := tab.WorktreeDir
			if entry, err := instance.Resolve(worktree); err == nil {
				pid = entry.PID
				branch = entry.Branch
			}
			m.daemonModal.Open(action, isRunning, isDraining, pid, branch, worktree)
			m.daemonModal.SetSize(m.width, m.height)
			m.activeModal = ModalDaemon
			return m, nil

		case key.Matches(msg, tui.DaemonKeyMap.StopAll):
			cmd := m.handleStopAll()
			return m, cmd

		case key.Matches(msg, tui.DaemonKeyMap.PrevTab):
			m.switchTab(-1)
			return m, nil

		case key.Matches(msg, tui.DaemonKeyMap.NextTab):
			m.switchTab(1)
			return m, nil

		case key.Matches(msg, tui.DaemonKeyMap.NewTab):
			if !m.isModalActive() {
				m.tabPicker = newTabPicker(m.originalCWD, m.instances)
				m.activeModal = ModalNewTab
				return m, nil
			}

		case key.Matches(msg, tui.DaemonKeyMap.CloseTab):
			if !m.isModalActive() {
				cmd := m.handleCloseTab()
				return m, cmd
			}
		}

		// Esc clears the error bar when errors are visible.
		if tab != nil && msg.String() == "esc" && len(tab.Errors) > 0 {
			tab.Errors = nil
			return m, nil
		}

		// Esc clears persistent search highlights when the search
		// bar is inactive but matches were left highlighted from a
		// prior Confirm.
		if tab != nil && msg.String() == "esc" && !tab.Search.IsActive() && (tab.Search.HasMatches() || tab.Tree.HasSearchHighlights()) {
			tab.Search.Dismiss()
			tab.Tree.SetSearchAddresses(nil, nil)
			return m, nil
		}

		// Esc in detail pane (non-dashboard mode) returns to dashboard.
		if tab != nil && msg.String() == "esc" && tab.Focused == PaneDetail && tab.Detail.Mode() != detail.ModeDashboard {
			tab.Detail.SwitchToDashboard()
			return m, nil
		}

		// n/N for search match navigation (when search has confirmed matches).
		if tab != nil && tab.Search.HasMatches() {
			prev := tab.Search.Current()
			s, cmd := tab.Search.Update(msg)
			tab.Search = s
			if tab.Search.Current() != prev {
				m.jumpTreeToSearchMatch()
				return m, cmd
			}
		}

		// Route remaining keys to the focused pane.
		if tab != nil {
			switch tab.Focused {
			case PaneTree:
				t, cmd := tab.Tree.Update(msg)
				tab.Tree = t
				cmds = append(cmds, cmd)
				if key.Matches(msg, tui.TreeKeyMap.Expand) {
					m.loadDetailForSelection()
				}
			case PaneDetail:
				d, cmd := tab.Detail.Update(msg)
				tab.Detail = d
				cmds = append(cmds, cmd)
			}
		}
		return m, tea.Batch(cmds...)

	// ---------------------------------------------------------------
	// Tab-scoped message envelope: unwrap, find the tab by ID,
	// dispatch the inner message to that tab's state, reschedule
	// the event drain. Messages for dead tabs are silently dropped.
	// ---------------------------------------------------------------
	case TabMsg:
		tab := m.tabByID(msg.TabID)
		if tab == nil {
			return m, nil
		}
		var inner tea.Cmd
		if msg.Inner != nil {
			updated, c := m.updateTabMsg(tab, msg.Inner)
			m = updated
			inner = c
		}
		return m, tea.Batch(inner, waitForTabEvent(msg.TabID, tab.Events))

	// ---------------------------------------------------------------
	// Tab poll tick: re-read state for a specific tab.
	// ---------------------------------------------------------------
	case TabPollTickMsg:
		tab := m.tabByID(msg.TabID)
		if tab == nil {
			return m, nil // tab closed, poll chain dies
		}
		tab.Tree.CleanCache()
		var pollCmds []tea.Cmd
		if tab.Store != nil {
			store := tab.Store
			worktree := tab.WorktreeDir
			pollCmds = append(pollCmds, func() tea.Msg {
				idx, err := store.ReadIndex()
				if err != nil {
					return nil
				}
				return TabMsg{TabID: msg.TabID, Inner: tui.StateUpdatedMsg{Index: idx, Worktree: worktree}}
			})
		}
		pollCmds = append(pollCmds, m.detectEntryState())
		pollCmds = append(pollCmds, scheduleTabPollTick(msg.TabID))
		return m, tea.Batch(pollCmds...)

	// ---------------------------------------------------------------
	// Data messages: always broadcast regardless of overlay state
	// ---------------------------------------------------------------
	case tui.StateUpdatedMsg:
		tab := m.activeTab()
		if tab == nil {
			return m, nil
		}
		if msg.Worktree != "" && msg.Worktree != tab.WorktreeDir {
			return m, nil
		}

		m.header.SetLoading(false)
		h, hcmd := m.header.Update(header.StateUpdatedMsg{Index: msg.Index})
		m.header = h
		cmds = append(cmds, hcmd)

		tab.Tree.SetIndex(msg.Index)

		d, dcmd := tab.Detail.Update(msg)
		tab.Detail = d
		cmds = append(cmds, dcmd)

		// Phase 5: diff against previous index to detect new nodes.
		if msg.Index != nil && tab.PrevIndex != nil {
			for addr := range msg.Index.Nodes {
				if _, existed := tab.PrevIndex.Nodes[addr]; !existed {
					cmd := m.notify.Push(fmt.Sprintf("New target acquired: %s", addr))
					cmds = append(cmds, cmd)
				}
			}
		}
		tab.PrevIndex = msg.Index

		m.clearErrorsByFilename("state.json")
		return m, tea.Batch(cmds...)

	case tui.DaemonStatusMsg:
		tab := m.activeTab()
		if msg.Instances != nil {
			m.instances = msg.Instances
			m.updateTabsFromInstances(msg.Instances)
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

		if tab != nil {
			d, dcmd := tab.Detail.Update(msg)
			tab.Detail = d
			cmds = append(cmds, dcmd)

			f, fcmd := m.footer.Update(msg)
			m.footer = f
			cmds = append(cmds, fcmd)

			if msg.IsRunning {
				tab.EntryState = StateLive
			} else {
				tab.EntryState = StateCold
			}
		}

		return m, tea.Batch(cmds...)

	case tui.NodeUpdatedMsg:
		tab := m.activeTab()
		if tab == nil {
			return m, nil
		}
		t, tcmd := tab.Tree.Update(tree.NodeUpdatedMsg{
			Address: msg.Address,
			Node:    msg.Node,
		})
		tab.Tree = t
		cmds = append(cmds, tcmd)

		// Phase 5: diff against previous node state for toast notifications.
		if msg.Node != nil {
			if prev, ok := tab.PrevNodes[msg.Address]; ok {
				cmds = append(cmds, m.diffNodeForToasts(msg.Address, prev, msg.Node)...)
			}
			cp := *msg.Node
			tab.PrevNodes[msg.Address] = &cp
		}

		return m, tea.Batch(cmds...)

	case tree.CollapseAtRootMsg:
		tab := m.activeTab()
		if tab != nil {
			tab.Detail.SwitchToDashboard()
		}
		return m, nil

	case tree.LoadNodeMsg:
		tab := m.activeTab()
		if tab == nil {
			return m, nil
		}
		if msg.Node == nil && msg.Err == nil && tab.Store != nil {
			store := tab.Store
			addr := msg.Address
			return m, func() tea.Msg {
				ns, err := store.ReadNode(addr)
				if err != nil {
					return nil
				}
				return tui.NodeUpdatedMsg{Address: addr, Node: ns}
			}
		}
		if msg.Node != nil {
			t, tcmd := tab.Tree.Update(msg)
			tab.Tree = t
			cmds = append(cmds, tcmd)
			return m, tea.Batch(cmds...)
		}
		return m, nil

	case tui.InstancesUpdatedMsg:
		m.instances = msg.Instances
		m.updateTabsFromInstances(msg.Instances)
		h, hcmd := m.header.Update(header.InstancesUpdatedMsg{
			Instances: msg.Instances,
		})
		m.header = h
		cmds = append(cmds, hcmd)
		return m, tea.Batch(cmds...)

	case tui.DaemonConfirmedMsg:
		m.closeModal()
		cmd := m.handleToggleDaemon()
		return m, cmd

	case tui.DaemonStartedMsg:
		tab := m.activeTab()
		if tab != nil {
			tab.DaemonStarting = false
			tab.EntryState = StateLive
		}
		m.header.SetLoading(false)
		m.header.SetStatusHint("")
		return m, m.handleRefresh()

	case tui.DaemonStartFailedMsg:
		tab := m.activeTab()
		if tab != nil {
			tab.DaemonStarting = false
		}
		m.header.SetLoading(false)
		m.header.SetStatusHint("")
		raw := strings.TrimSpace(msg.Stderr)
		if raw == "" && msg.Err != nil {
			raw = msg.Err.Error()
		}
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
		tab := m.activeTab()
		if tab != nil {
			tab.DaemonStopping = false
			tab.EntryState = StateCold
			tab.Tree.Reset()
			tab.Detail.Reset()
			tab.Detail.SwitchToDashboard()
			tab.PrevIndex = nil
			tab.PrevNodes = make(map[string]*state.NodeState)
			// Restart the tab's watcher for the updated state.
			tab.Stop()
			if tab.Store != nil {
				if startCmd := tab.Start(); startCmd != nil {
					cmds = append(cmds, startCmd)
				}
			}
		}
		m.header.SetStatusHint("")
		refreshCmd := m.handleRefresh()
		cmds = append(cmds, refreshCmd)
		return m, tea.Batch(cmds...)

	case tui.DaemonStopFailedMsg:
		tab := m.activeTab()
		if tab != nil {
			tab.DaemonStopping = false
		}
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

	case tui.SpinnerTickMsg:
		h, hcmd := m.header.Update(header.SpinnerTickMsg{})
		m.header = h
		cmds = append(cmds, hcmd)

		if m.welcome != nil {
			w, wcmd := m.welcome.Update(msg)
			m.welcome = &w
			cmds = append(cmds, wcmd)
		}

		if m.header.IsLoading() {
			cmds = append(cmds, tea.Tick(80*time.Millisecond, func(time.Time) tea.Msg {
				return tui.SpinnerTickMsg{}
			}))
		}
		return m, tea.Batch(cmds...)

	case tui.LogLinesMsg:
		tab := m.activeTab()
		if tab != nil {
			d, dcmd := tab.Detail.Update(msg)
			tab.Detail = d
			cmds = append(cmds, dcmd)
		}
		return m, tea.Batch(cmds...)

	case tui.NewLogFileMsg:
		tab := m.activeTab()
		if tab != nil {
			d, dcmd := tab.Detail.Update(msg)
			tab.Detail = d
			cmds = append(cmds, dcmd)
		}
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
			tab := m.activeTab()
			if tab != nil {
				m.welcome = nil
				wolfDir := filepath.Join(msg.Dir, ".wolfcastle")
				tab.WorktreeDir = msg.Dir
				tab.DaemonRepo = daemon.NewRepository(wolfDir)
				tab.Store = storeFromWolfcastleDir(wolfDir)
				tab.EntryState = StateCold
				tab.Label = filepath.Base(msg.Dir)
				m.header.SetLoading(true)
				if startCmd := tab.Start(); startCmd != nil {
					cmds = append(cmds, startCmd)
				}
				cmds = append(cmds, m.detectEntryState())
			}
		}
		return m, tea.Batch(cmds...)

	case welcome.ConnectInstanceMsg:
		// User selected a running session from the welcome screen.
		tab := m.activeTab()
		if tab != nil {
			m.welcome = nil
			wolfDir := filepath.Join(msg.Entry.Worktree, ".wolfcastle")
			tab.WorktreeDir = msg.Entry.Worktree
			tab.DaemonRepo = daemon.NewRepository(wolfDir)
			tab.Store = storeFromWolfcastleDir(wolfDir)
			tab.EntryState = StateLive
			tab.Label = filepath.Base(msg.Entry.Worktree)
			tab.PrevIndex = nil
			tab.PrevNodes = make(map[string]*state.NodeState)
			m.notify = notify.NewNotificationModel()
			m.instances, _ = instance.List()
			m.header.SetLoading(true)
			m.propagateSize()
			if startCmd := tab.Start(); startCmd != nil {
				cmds = append(cmds, startCmd)
			}
			cmds = append(cmds, m.detectEntryState())
		}
		return m, tea.Batch(cmds...)

	case tui.WorktreeGoneMsg:
		m.header.SetStatusHint("")
		filtered := m.instances[:0]
		for _, inst := range m.instances {
			if inst.PID != msg.Entry.PID || inst.Worktree != msg.Entry.Worktree {
				filtered = append(filtered, inst)
			}
		}
		m.instances = filtered
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

	case TabPickerResultMsg:
		m.closeModal()
		dir := msg.Dir
		// Duplicate check: if a tab already exists for this directory, switch to it.
		for i := range m.tabs {
			if m.tabs[i].WorktreeDir == dir {
				m.activeTabID = m.tabs[i].ID
				m.header.SetTabs(m.tabLabels(), m.activeTabIndex(), m.tabRunningSet())
				return m, nil
			}
		}
		// Create a new tab for this directory.
		store, daemonRepo := resolveStoreForDir(dir)
		tab := m.createTab(dir, store, daemonRepo)
		m.activeTabID = tab.ID
		m.header.SetTabs(m.tabLabels(), m.activeTabIndex(), m.tabRunningSet())
		m.propagateSize()
		if tab.Store != nil {
			return m, tab.Start()
		}
		return m, nil

	case TabPickerCancelMsg:
		m.closeModal()
		return m, nil

	case tui.PollTickMsg:
		// Global poll tick: instance discovery and entry state detection.
		var pollCmds []tea.Cmd
		pollCmds = append(pollCmds, m.detectEntryState(), m.scheduleGlobalPollTick())
		return m, tea.Batch(pollCmds...)

	case tui.InboxUpdatedMsg:
		tab := m.activeTab()
		if tab != nil {
			if msg.Inbox != nil {
				tab.Detail.SetDashboardInbox(msg.Inbox.Items)
			}
			d, dcmd := tab.Detail.Update(msg)
			tab.Detail = d
			cmds = append(cmds, dcmd)
		}
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

// updateTabMsg dispatches a message to a specific tab's state. This is the
// inner dispatch for TabMsg routing, handling the same message types that
// Update handles at the top level but scoped to a particular tab.
func (m *TUIModel) updateTabMsg(tab *Tab, msg tea.Msg) (TUIModel, tea.Cmd) {
	// Unwrap WatcherMsg envelopes from the per-tab watcher.
	if wm, ok := msg.(tui.WatcherMsg); ok && wm.Inner != nil {
		return m.updateTabMsg(tab, wm.Inner)
	}
	var cmds []tea.Cmd
	switch msg := msg.(type) {
	case tui.StateUpdatedMsg:
		if msg.Worktree != "" && msg.Worktree != tab.WorktreeDir {
			return *m, nil
		}
		tab.Loading = false
		tab.Tree.SetIndex(msg.Index)
		d, dcmd := tab.Detail.Update(msg)
		tab.Detail = d
		cmds = append(cmds, dcmd)
		if msg.Index != nil && tab.PrevIndex != nil {
			for addr := range msg.Index.Nodes {
				if _, existed := tab.PrevIndex.Nodes[addr]; !existed {
					cmd := m.notify.Push(fmt.Sprintf("New target acquired: %s", addr))
					cmds = append(cmds, cmd)
				}
			}
		}
		tab.PrevIndex = msg.Index
		// If this is the active tab, also update the header.
		if tab.ID == m.activeTabID {
			m.header.SetLoading(false)
			h, hcmd := m.header.Update(header.StateUpdatedMsg{Index: msg.Index})
			m.header = h
			cmds = append(cmds, hcmd)
			m.clearErrorsByFilename("state.json")
		}
	case tui.NodeUpdatedMsg:
		t, tcmd := tab.Tree.Update(tree.NodeUpdatedMsg{
			Address: msg.Address,
			Node:    msg.Node,
		})
		tab.Tree = t
		cmds = append(cmds, tcmd)
		if msg.Node != nil {
			if prev, ok := tab.PrevNodes[msg.Address]; ok {
				cmds = append(cmds, m.diffNodeForToasts(msg.Address, prev, msg.Node)...)
			}
			cp := *msg.Node
			tab.PrevNodes[msg.Address] = &cp
		}
	case tui.LogLinesMsg:
		d, dcmd := tab.Detail.Update(msg)
		tab.Detail = d
		cmds = append(cmds, dcmd)
	case tui.NewLogFileMsg:
		d, dcmd := tab.Detail.Update(msg)
		tab.Detail = d
		cmds = append(cmds, dcmd)
	case tui.ErrorMsg:
		tab.Errors = append(tab.Errors, errorEntry{filename: msg.Filename, message: sanitizeErrorLine(msg.Message)})
	case tui.ErrorClearedMsg:
		filtered := tab.Errors[:0]
		for _, e := range tab.Errors {
			if e.filename != msg.Filename {
				filtered = append(filtered, e)
			}
		}
		tab.Errors = filtered
	}
	return *m, tea.Batch(cmds...)
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

	tab := m.activeTab()
	if tab != nil && tab.EntryState == StateWelcome && m.welcome != nil {
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
	if contentHeight < 3 {
		contentHeight = 3
	}

	contentView := m.renderContent(contentHeight)

	if m.help.IsActive() {
		contentView = m.help.View()
	} else if m.activeModal != ModalNone {
		contentView = m.renderActiveModal(contentHeight)
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
	tab := m.activeTab()
	if tab == nil {
		return ""
	}

	if !tab.TreeVisible || m.width < 60 {
		content := tab.Detail.View()
		if tab.Search.IsActive() && tab.Search.PaneType() == int(PaneDetail) {
			content += "\n" + tab.Search.View()
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

	treeWidth := m.width * 30 / 100
	if treeWidth < 24 {
		treeWidth = 24
	}
	detailWidth := m.width - treeWidth
	if detailWidth < 10 {
		detailWidth = 10
	}

	treeContent := tab.Tree.View()
	if tab.Search.IsActive() && tab.Search.PaneType() == int(PaneTree) {
		treeContent += "\n" + tab.Search.View()
	}

	treePaneStyle := m.borderStyle(PaneTree).
		Width(treeWidth).
		Height(contentHeight).
		MaxHeight(contentHeight)
	treePane := treePaneStyle.Render(treeContent)

	detailContent := tab.Detail.View()
	if tab.Search.IsActive() && tab.Search.PaneType() == int(PaneDetail) {
		detailContent += "\n" + tab.Search.View()
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
	tab := m.activeTab()
	if tab != nil && tab.Focused == pane {
		return tui.FocusedBorderStyle
	}
	return tui.UnfocusedBorderStyle
}

func (m TUIModel) renderErrorBar() string {
	tab := m.activeTab()
	if tab == nil || len(tab.Errors) == 0 {
		return ""
	}

	maxShow := 3
	shown := tab.Errors
	if len(shown) > maxShow {
		shown = shown[:maxShow]
	}

	var lines []string
	for _, e := range shown {
		lines = append(lines, tui.ErrorBarStyle.Render(fmt.Sprintf(" %s: %s", e.filename, e.message)))
	}
	if overflow := len(tab.Errors) - maxShow; overflow > 0 {
		lines = append(lines, tui.ErrorBarStyle.Render(fmt.Sprintf(" +%d more errors", overflow)))
	}
	return strings.Join(lines, "\n")
}

// ---------------------------------------------------------------------------
// Focus & search helpers
// ---------------------------------------------------------------------------

func (m *TUIModel) cycleFocus() {
	tab := m.activeTab()
	if tab == nil || !tab.TreeVisible {
		return
	}
	if tab.Focused == PaneTree {
		tab.Focused = PaneDetail
	} else {
		tab.Focused = PaneTree
	}
	m.syncFocus()
}

func (m *TUIModel) syncFocus() {
	tab := m.activeTab()
	if tab == nil {
		return
	}
	tab.Tree.SetFocused(tab.Focused == PaneTree)
	tab.Detail.SetFocused(tab.Focused == PaneDetail)
	m.footer.SetFocus(int(tab.Focused))
}

func (m *TUIModel) computeTreeSearchMatches() {
	tab := m.activeTab()
	if tab == nil {
		return
	}
	query := strings.ToLower(tab.Search.Query())
	if query == "" {
		tab.Search.SetMatches(nil)
		tab.Tree.SetSearchAddresses(nil, nil)
		return
	}

	// When search was activated from the detail pane, match against detail content.
	if tab.Search.PaneType() == int(PaneDetail) {
		m.computeDetailSearchMatches(query)
		return
	}

	idx := tab.Tree.Index()
	if idx == nil {
		tab.Search.SetMatches(nil)
		tab.Tree.SetSearchAddresses(nil, nil)
		return
	}

	literal := make(map[string]bool)
	var matches []search.Match

	for addr, entry := range idx.Nodes {
		if strings.Contains(strings.ToLower(entry.Name), query) {
			literal[addr] = true
			matches = append(matches, search.Match{Address: addr})
		}
		if entry.Type != state.NodeLeaf {
			continue
		}
		ns := tab.Tree.CachedNode(addr)
		if ns == nil {
			continue
		}
		for _, task := range ns.Tasks {
			if strings.Contains(strings.ToLower(task.Title), query) {
				taskAddr := addr + "/" + task.ID
				literal[taskAddr] = true
				matches = append(matches, search.Match{Address: taskAddr})
			}
		}
	}

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

	tab.Search.SetMatches(matches)
	tab.Tree.SetSearchAddresses(literal, ancestor)
}

func (m *TUIModel) computeDetailSearchMatches(query string) {
	tab := m.activeTab()
	if tab == nil {
		return
	}
	lines := tab.Detail.SearchContent()
	var matches []search.Match
	for i, line := range lines {
		if strings.Contains(strings.ToLower(line), query) {
			matches = append(matches, search.Match{Row: i})
		}
	}
	tab.Search.SetMatches(matches)
	tab.Tree.SetSearchAddresses(nil, nil)
}

func (m *TUIModel) jumpTreeToSearchMatch() {
	tab := m.activeTab()
	if tab == nil {
		return
	}
	match, ok := tab.Search.CurrentMatch()
	if !ok {
		return
	}
	if match.Address == "" {
		tab.Tree.SetCursor(match.Row)
		return
	}
	target := match.Address
	for target != "" {
		for i, row := range tab.Tree.FlatList() {
			if row.Addr == target {
				tab.Tree.SetCursor(i)
				return
			}
		}
		slash := strings.LastIndex(target, "/")
		if slash < 0 {
			return
		}
		target = target[:slash]
	}
}

func (m TUIModel) handleCopy() tea.Cmd {
	tab := m.activeTab()
	if tab == nil {
		return nil
	}
	var text string

	switch tab.Focused {
	case PaneDetail:
		text = ansi.Strip(tab.Detail.View())
	case PaneTree:
		text = tab.Tree.SelectedAddr()
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
	tab := m.activeTab()
	if tab == nil {
		return
	}
	row := tab.Tree.SelectedRow()
	if row == nil {
		return
	}

	idx := tab.Tree.Index()
	if idx == nil {
		return
	}

	if row.IsTask {
		lastSlash := strings.LastIndex(row.Addr, "/")
		if lastSlash < 0 {
			return
		}
		nodeAddr := row.Addr[:lastSlash]
		taskID := row.Addr[lastSlash+1:]

		ns := tab.Tree.CachedNode(nodeAddr)
		if ns == nil {
			return
		}
		for i := range ns.Tasks {
			if ns.Tasks[i].ID == taskID {
				tab.Detail.LoadTaskDetail(nodeAddr, taskID, &ns.Tasks[i])
				return
			}
		}
		return
	}

	entry, ok := idx.Nodes[row.Addr]
	if !ok {
		return
	}

	ns := tab.Tree.CachedNode(row.Addr)
	if ns == nil && tab.Store != nil {
		loaded, err := tab.Store.ReadNode(row.Addr)
		if err == nil {
			ns = loaded
		}
	}
	if ns == nil {
		ns = &state.NodeState{
			Name:  entry.Name,
			Type:  entry.Type,
			State: entry.State,
		}
	}

	isTarget := tab.Tree.SelectedAddr() == m.currentTarget()
	tab.Detail.LoadNodeDetail(row.Addr, ns, &entry, isTarget)
}

func (m TUIModel) currentTarget() string {
	// The header tracks the current target through daemon status. For now,
	// return empty; the watcher will set this when target tracking lands.
	return ""
}

func (m TUIModel) handleRefresh() tea.Cmd {
	tab := m.activeTab()
	if tab == nil {
		return nil
	}
	var cmds []tea.Cmd

	if tab.Store != nil {
		store := tab.Store
		worktree := tab.WorktreeDir
		cmds = append(cmds, func() tea.Msg {
			idx, err := store.ReadIndex()
			if err != nil {
				return tui.ErrorMsg{
					Filename: "state.json",
					Message:  "State corruption detected: state.json. Run wolfcastle doctor.",
				}
			}
			return tui.StateUpdatedMsg{Index: idx, Worktree: worktree}
		})
	}

	if tab.DaemonRepo != nil {
		cmds = append(cmds, m.detectEntryState())
	}

	return tea.Batch(cmds...)
}

// ---------------------------------------------------------------------------
// Inbox helpers
// ---------------------------------------------------------------------------

func (m TUIModel) loadInbox() tea.Cmd {
	tab := m.activeTab()
	if tab == nil || tab.Store == nil {
		return nil
	}
	store := tab.Store
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
	tab := m.activeTab()
	if tab == nil || tab.Store == nil {
		return nil
	}
	store := tab.Store
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
	m.daemonModal.SetSize(m.width, m.height)
	m.notify.SetSize(m.width)

	if m.welcome != nil {
		m.welcome.SetSize(m.width, m.height)
	}

	tab := m.activeTab()
	if tab == nil {
		return
	}

	headerLines := strings.Count(m.header.View(), "\n") + 1
	contentHeight := m.height - headerLines - 1
	if errorBar := m.renderErrorBar(); errorBar != "" {
		contentHeight -= strings.Count(errorBar, "\n") + 1
	}
	if contentHeight < 3 {
		contentHeight = 3
	}
	innerHeight := contentHeight - 2

	if !tab.TreeVisible || m.width < 60 {
		tab.Tree.SetSize(0, 0)
		tab.Detail.SetSize(maxInt(m.width-2, 1), innerHeight)
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

	tab.Tree.SetSize(maxInt(treeWidth-2, 1), innerHeight)
	tab.Detail.SetSize(maxInt(detailWidth-2, 1), innerHeight)
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
	tab := m.activeTab()
	if tab == nil {
		return
	}
	filtered := tab.Errors[:0]
	for _, e := range tab.Errors {
		if e.filename != filename {
			filtered = append(filtered, e)
		}
	}
	tab.Errors = filtered
}

// maxErrorEntries caps how many error rows can pile up in memory.
const maxErrorEntries = 8

func (m *TUIModel) appendError(filename, message string) {
	tab := m.activeTab()
	if tab == nil {
		return
	}
	if n := len(tab.Errors); n > 0 {
		last := tab.Errors[n-1]
		if last.filename == filename && last.message == message {
			return
		}
	}
	tab.Errors = append(tab.Errors, errorEntry{filename: filename, message: message})
	if len(tab.Errors) > maxErrorEntries {
		tab.Errors = tab.Errors[len(tab.Errors)-maxErrorEntries:]
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
	tab := m.activeTab()
	if tab == nil || tab.DaemonRepo == nil {
		return nil
	}
	repo := tab.DaemonRepo
	worktreeDir := tab.WorktreeDir
	return func() tea.Msg {
		instances, _ := instance.List()

		var pid int
		var branch string
		var running, draining bool

		if entry, err := instance.Resolve(worktreeDir); err == nil {
			pid = entry.PID
			branch = entry.Branch
			running = isProcessRunning(entry.PID)
			draining = repo.HasDrainFile()
		} else {
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

// switchTab rotates the activeTabID by delta positions (positive = forward).
func (m *TUIModel) switchTab(delta int) {
	if len(m.tabs) < 2 || m.activeModal != ModalNone {
		return
	}
	var idx int
	for i := range m.tabs {
		if m.tabs[i].ID == m.activeTabID {
			idx = i
			break
		}
	}
	idx = (idx + delta + len(m.tabs)) % len(m.tabs)
	m.activeTabID = m.tabs[idx].ID
	m.propagateSize()
	m.syncFocus()
}

// updateTabsFromInstances updates existing tabs' EntryState from the
// instance registry. New daemons that don't match any tab are ignored.
func (m *TUIModel) updateTabsFromInstances(instances []instance.Entry) {
	for i := range m.tabs {
		found := false
		for _, inst := range instances {
			if inst.Worktree == m.tabs[i].WorktreeDir {
				found = true
				if isProcessAlive(inst.PID) {
					m.tabs[i].EntryState = StateLive
				} else {
					if m.tabs[i].EntryState == StateLive {
						m.tabs[i].EntryState = StateCold
					}
				}
				break
			}
		}
		if !found && m.tabs[i].EntryState == StateLive {
			m.tabs[i].EntryState = StateCold
		}
	}
}

// ---------------------------------------------------------------------------
// Daemon control (Phase 3)
// ---------------------------------------------------------------------------

func (m *TUIModel) handleToggleDaemon() tea.Cmd {
	tab := m.activeTab()
	if tab == nil {
		return nil
	}
	if tab.DaemonStarting || tab.DaemonStopping {
		return nil
	}
	if tab.EntryState == StateLive {
		return m.stopCurrentDaemon()
	}
	return m.startDaemon()
}

func (m *TUIModel) startDaemon() tea.Cmd {
	tab := m.activeTab()
	if tab == nil {
		return nil
	}
	tab.DaemonStarting = true
	m.header.SetStatusHint("Starting daemon...")
	m.header.SetLoading(true)
	dir := tab.WorktreeDir
	return func() tea.Msg {
		exe, err := os.Executable()
		if err != nil {
			exe = "wolfcastle"
		}
		cmd := exec.Command(exe, "start", "-d")
		cmd.Dir = dir
		var stderr bytes.Buffer
		cmd.Stderr = &stderr
		if runErr := cmd.Run(); runErr != nil {
			return tui.DaemonStartFailedMsg{
				Err:    runErr,
				Stderr: stderr.String(),
			}
		}
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
	tab := m.activeTab()
	if tab == nil {
		return nil
	}
	tab.DaemonStopping = true
	m.header.SetStatusHint("Stopping daemon...")

	var pid int
	if entry, err := instance.Resolve(tab.WorktreeDir); err == nil {
		pid = entry.PID
	}
	if pid == 0 {
		tab.DaemonStopping = false
		m.header.SetStatusHint("")
		return func() tea.Msg {
			return tui.DaemonStopFailedMsg{Err: fmt.Errorf("no daemon PID found")}
		}
	}

	return func() tea.Msg {
		if err := killProcess(pid, syscall.SIGTERM); err != nil {
			return tui.DaemonStopFailedMsg{Err: fmt.Errorf("sending SIGTERM to PID %d: %w", pid, err)}
		}
		deadline := time.Now().Add(5 * time.Second)
		for time.Now().Before(deadline) {
			if !daemon.IsProcessRunning(pid) {
				return tui.DaemonStoppedMsg{}
			}
			time.Sleep(200 * time.Millisecond)
		}
		return tui.DaemonStopFailedMsg{Err: fmt.Errorf("daemon not responding, try wolfcastle stop --force")}
	}
}

func (m *TUIModel) handleStopAll() tea.Cmd {
	tab := m.activeTab()
	if tab != nil && tab.DaemonStopping {
		return nil
	}
	if tab != nil {
		tab.DaemonStopping = true
	}
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
		return tui.DaemonStopFailedMsg{Err: fmt.Errorf("daemon not responding, try wolfcastle stop --force")}
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
