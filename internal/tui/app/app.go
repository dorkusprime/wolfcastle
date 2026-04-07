// Package app contains the top-level TUI orchestrator that wires sub-models
// together, routes messages, manages focus, and handles layout. It lives in
// its own package to break the circular dependency between the parent tui
// package (which holds shared message types) and the sub-model packages
// (detail, footer, welcome, search, help) that import those types.
package app

import (
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

	"github.com/dorkusprime/wolfcastle/internal/daemon"
	"github.com/dorkusprime/wolfcastle/internal/instance"
	"github.com/dorkusprime/wolfcastle/internal/state"
	"github.com/dorkusprime/wolfcastle/internal/tui"
	"github.com/dorkusprime/wolfcastle/internal/tui/clipboard"
	"github.com/dorkusprime/wolfcastle/internal/tui/detail"
	"github.com/dorkusprime/wolfcastle/internal/tui/footer"
	"github.com/dorkusprime/wolfcastle/internal/tui/header"
	"github.com/dorkusprime/wolfcastle/internal/tui/help"
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

	header        header.HeaderModel
	tree          tree.TreeModel
	detail        detail.DetailModel
	footer        footer.FooterModel
	welcome       *welcome.WelcomeModel
	search        search.SearchModel
	help          help.HelpOverlayModel
	overlayActive bool

	entryState  EntryState
	store       *state.Store
	daemonRepo  *daemon.DaemonRepository
	worktreeDir string
	version     string

	watcher *tui.Watcher

	// Instance tracking (Phase 3)
	instances          []instance.Entry
	activeInstanceIndex int
	daemonStarting     bool
	daemonStopping     bool

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
		store:       store,
		daemonRepo:  daemonRepo,
		worktreeDir: worktreeDir,
		version:     version,
	}

	if store == nil {
		m.entryState = StateWelcome
		w := welcome.NewWelcomeModel(worktreeDir)
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
		cmds = append(cmds, m.startWatcher(), m.startPoller(), m.loadInitialState())
	}

	return tea.Batch(cmds...)
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

		case key.Matches(msg, tui.GlobalKeyMap.Dashboard):
			m.detail.SwitchToDashboard()
			return m, nil

		case key.Matches(msg, tui.GlobalKeyMap.LogStream) && m.focused != PaneTree:
			m.detail.SwitchToLogView()
			return m, nil

		case key.Matches(msg, tui.GlobalKeyMap.Inbox):
			m.detail.SwitchToInbox()
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
			return m, m.handleToggleDaemon()

		case key.Matches(msg, tui.DaemonKeyMap.StopAll):
			return m, m.handleStopAll()

		case key.Matches(msg, tui.DaemonKeyMap.PrevInstance):
			return m, m.handleSwitchInstance(-1)

		case key.Matches(msg, tui.DaemonKeyMap.NextInstance):
			return m, m.handleSwitchInstance(1)
		}

		// Digit keys 1-9 for instance selection.
		if ch := msg.String(); len(ch) == 1 && ch[0] >= '1' && ch[0] <= '9' {
			idx := int(ch[0]-'0') - 1
			if idx < len(m.instances) {
				return m, m.switchInstance(m.instances[idx])
			}
		}

		// Esc clears the error bar when errors are visible.
		if msg.String() == "esc" && len(m.errors) > 0 {
			m.errors = nil
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

		m.clearErrorsByFilename("state.json")
		return m, tea.Batch(cmds...)

	case tui.DaemonStatusMsg:
		h, hcmd := m.header.Update(header.DaemonStatusMsg{
			Status:     msg.Status,
			Branch:     msg.Branch,
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
		return m, tea.Batch(cmds...)

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
		errText := msg.Err.Error()
		var displayMsg string
		switch {
		case strings.Contains(errText, "lock") || strings.Contains(errText, "already running"):
			displayMsg = "Another daemon is running in this worktree."
		case strings.Contains(errText, "not found") || strings.Contains(errText, "no such"):
			displayMsg = "No project found. Run wolfcastle init."
		default:
			displayMsg = fmt.Sprintf("Daemon failed to start: %s", errText)
		}
		m.errors = append(m.errors, errorEntry{
			filename: "daemon",
			message:  displayMsg,
		})
		return m, nil

	case tui.DaemonStoppedMsg:
		m.daemonStopping = false
		m.entryState = StateCold
		m.header.SetStatusHint("")
		return m, m.handleRefresh()

	case tui.DaemonStopFailedMsg:
		m.daemonStopping = false
		m.header.SetStatusHint("")
		m.errors = append(m.errors, errorEntry{
			filename: "daemon",
			message:  msg.Err.Error(),
		})
		return m, nil

	case tui.InstanceSwitchedMsg:
		m.header.SetStatusHint("")

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
		m.store = state.NewStore(wolfDir, 5*time.Second)
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
		m.watcher = tui.NewWatcher(m.store, logDir, instanceDir)

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
		m.errors = append(m.errors, errorEntry{
			filename: msg.Filename,
			message:  msg.Message,
		})
		return m, nil

	case tui.ErrorClearedMsg:
		m.clearErrorsByFilename(msg.Filename)
		return m, nil

	case tui.CopiedMsg:
		f, fcmd := m.footer.Update(msg)
		m.footer = f
		cmds = append(cmds, fcmd)
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
			wolfDir := filepath.Join(msg.Dir, ".wolfcastle")
			m.store = state.NewStore(wolfDir, 5*time.Second)
			m.daemonRepo = daemon.NewDaemonRepository(wolfDir)
			m.header.SetLoading(true)
			cmds = append(cmds, m.startWatcher(), m.startPoller(), m.loadInitialState())
		}
		return m, tea.Batch(cmds...)

	case tui.WorktreeGoneMsg:
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
		m.errors = append(m.errors, errorEntry{
			filename: "instance",
			message:  fmt.Sprintf("Worktree no longer exists: %s", msg.Entry.Worktree),
		})
		return m, nil

	case tui.PollTickMsg:
		m.tree.CleanCache()
		return m, nil

	case tui.InboxUpdatedMsg:
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
		m.errors = append(m.errors, errorEntry{
			filename: "inbox",
			message:  errMsg,
		})
		return m, nil
	}

	return m, nil
}

// View builds the full terminal output.
func (m TUIModel) View() tea.View {
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
	if contentHeight < 1 {
		contentHeight = 1
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
			Width(m.width - 2).
			Height(contentHeight)
		return detailStyle.Render(content)
	}

	treeWidth := m.width * 30 / 100
	if treeWidth < 24 {
		treeWidth = 24
	}

	// Borders consume 2 cells per pane (left+right).
	detailWidth := m.width - treeWidth - 4
	if detailWidth < 10 {
		detailWidth = 10
	}

	treeContent := m.tree.View()
	if m.search.IsActive() && m.search.PaneType() == int(PaneTree) {
		treeContent += "\n" + m.search.View()
	}

	treePaneStyle := m.borderStyle(PaneTree).
		Width(treeWidth).
		Height(contentHeight)
	treePane := treePaneStyle.Render(treeContent)

	detailContent := m.detail.View()
	if m.search.IsActive() && m.search.PaneType() == int(PaneDetail) {
		detailContent += "\n" + m.search.View()
	}

	detailPaneStyle := m.borderStyle(PaneDetail).
		Width(detailWidth).
		Height(contentHeight)
	detailPane := detailPaneStyle.Render(detailContent)

	return lipgloss.JoinHorizontal(lipgloss.Top, treePane, detailPane)
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
		m.tree.SetSearchMatches(nil)
		return
	}

	// When search was activated from the detail pane, match against detail content.
	if m.search.PaneType() == int(PaneDetail) {
		m.computeDetailSearchMatches(query)
		return
	}

	var matches []search.SearchMatch
	highlights := make(map[int]bool)
	for i, row := range m.tree.FlatList() {
		if strings.Contains(strings.ToLower(row.Name), query) {
			matches = append(matches, search.SearchMatch{Row: i})
			highlights[i] = true
		}
	}
	m.search.SetMatches(matches)
	m.tree.SetSearchMatches(highlights)
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
	m.tree.SetSearchMatches(nil)
}

func (m *TUIModel) jumpTreeToSearchMatch() {
	match, ok := m.search.CurrentMatch()
	if !ok {
		return
	}
	m.tree.SetCursor(match.Row)
}

func (m TUIModel) handleCopy() tea.Cmd {
	var text string

	switch {
	case m.focused == PaneDetail && m.detail.Mode() != detail.ModeDashboard:
		text = m.detail.CopyTarget()
	default:
		text = m.tree.SelectedAddr()
	}

	if text == "" {
		return nil
	}
	return func() tea.Msg {
		_ = clipboard.WriteOSC52(os.Stdout, text)
		return tui.CopiedMsg{}
	}
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
	if ns == nil {
		// If the node isn't cached yet, we can still show the index entry info
		// by constructing a minimal NodeState from the index entry.
		ns = &state.NodeState{
			Name:               entry.Name,
			Type:               entry.Type,
			State:              entry.State,
			DecompositionDepth: entry.DecompositionDepth,
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
		repo := m.daemonRepo
		cmds = append(cmds, func() tea.Msg {
			running := repo.IsAlive()
			draining := repo.HasDrainFile()
			instances, _ := instance.List()
			status := "standing down"
			if running && !draining {
				status = "hunting"
			} else if running && draining {
				status = "draining"
			}
			return tui.DaemonStatusMsg{
				Status:     status,
				IsRunning:  running,
				IsDraining: draining,
				Instances:  instances,
			}
		})
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

	if m.welcome != nil {
		m.welcome.SetSize(m.width, m.height)
	}

	headerLines := 2
	if m.width < 40 {
		headerLines = 1
	}
	contentHeight := m.height - headerLines - 1
	if contentHeight < 1 {
		contentHeight = 1
	}

	if !m.treeVisible || m.width < 60 {
		m.tree.SetSize(0, 0)
		m.detail.SetSize(m.width-2, contentHeight)
		return
	}

	treeWidth := m.width * 30 / 100
	if treeWidth < 24 {
		treeWidth = 24
	}
	detailWidth := m.width - treeWidth - 4
	if detailWidth < 10 {
		detailWidth = 10
	}

	m.tree.SetSize(treeWidth, contentHeight)
	m.detail.SetSize(detailWidth, contentHeight)
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

// ---------------------------------------------------------------------------
// Init commands
// ---------------------------------------------------------------------------

func (m TUIModel) detectEntryState() tea.Cmd {
	repo := m.daemonRepo
	return func() tea.Msg {
		if repo == nil {
			return nil
		}
		running := repo.IsAlive()
		draining := repo.HasDrainFile()
		instances, _ := instance.List()
		status := "standing down"
		if running && !draining {
			status = "hunting"
		} else if running && draining {
			status = "draining"
		}
		return tui.DaemonStatusMsg{
			Status:     status,
			IsRunning:  running,
			IsDraining: draining,
			Instances:  instances,
		}
	}
}

func (m *TUIModel) startWatcher() tea.Cmd {
	store := m.store
	repo := m.daemonRepo
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
		m.watcher = tui.NewWatcher(store, logDir, instanceDir)
		return nil
	}
}

func (m TUIModel) startPoller() tea.Cmd {
	return func() tea.Msg {
		return tui.PollTickMsg{}
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
		cmd := exec.Command("wolfcastle", "start", "-d")
		cmd.Dir = dir
		if err := cmd.Run(); err != nil {
			return tui.DaemonStartFailedMsg{Err: err}
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
		if err := syscall.Kill(pid, syscall.SIGTERM); err != nil {
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
			if err := syscall.Kill(inst.PID, syscall.SIGTERM); err != nil {
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
		return tui.DaemonStopFailedMsg{Err: fmt.Errorf("Daemon not responding. Try wolfcastle stop --force.")}
	}
}

func (m *TUIModel) handleSwitchInstance(delta int) tea.Cmd {
	if len(m.instances) < 2 {
		return nil
	}
	next := (m.activeInstanceIndex + delta + len(m.instances)) % len(m.instances)
	m.activeInstanceIndex = next
	return m.switchInstance(m.instances[next])
}

func (m *TUIModel) switchInstance(entry instance.Entry) tea.Cmd {
	m.header.SetStatusHint("Switching...")

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
		store := state.NewStore(wolfDir, 5*time.Second)
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
