// Package app contains the top-level TUI orchestrator that wires sub-models
// together, routes messages, manages focus, and handles layout. It lives in
// its own package to break the circular dependency between the parent tui
// package (which holds shared message types) and the sub-model packages
// (detail, footer, welcome, search, help) that import those types.
package app

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
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
			m.detail.SetMode(detail.ModeDashboard)
			return m, nil

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
			return m, m.handleRefresh()

		case key.Matches(msg, tui.GlobalKeyMap.ToggleHelp):
			m.help.Toggle()
			return m, nil

		case key.Matches(msg, tui.GlobalKeyMap.Search):
			m.search.Activate(int(m.focused))
			return m, nil

		case key.Matches(msg, tui.GlobalKeyMap.Copy):
			return m, m.handleCopy()
		}

		// Esc clears the error bar when errors are visible.
		if msg.String() == "esc" && len(m.errors) > 0 {
			m.errors = nil
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
		h, hcmd := m.header.Update(header.InstancesUpdatedMsg{
			Instances: msg.Instances,
		})
		m.header = h
		cmds = append(cmds, hcmd)
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

	case tui.ErrorMsg:
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
			cmds = append(cmds, m.startWatcher(), m.startPoller(), m.loadInitialState())
		}
		return m, tea.Batch(cmds...)

	case tui.PollTickMsg:
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
		detailStyle := m.borderStyle(PaneDetail).
			Width(m.width - 2).
			Height(contentHeight)
		return detailStyle.Render(m.detail.View())
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
	if m.search.IsActive() && m.focused == PaneTree {
		treeContent += "\n" + m.search.View()
	}

	treePaneStyle := m.borderStyle(PaneTree).
		Width(treeWidth).
		Height(contentHeight)
	treePane := treePaneStyle.Render(treeContent)

	detailPaneStyle := m.borderStyle(PaneDetail).
		Width(detailWidth).
		Height(contentHeight)
	detailPane := detailPaneStyle.Render(m.detail.View())

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
		return
	}

	var matches []search.SearchMatch
	for i, row := range m.tree.FlatList() {
		if strings.Contains(strings.ToLower(row.Name), query) {
			matches = append(matches, search.SearchMatch{Row: i})
		}
	}
	m.search.SetMatches(matches)
}

func (m *TUIModel) jumpTreeToSearchMatch() {
	// TODO: expose tree.SetCursor(int) for direct cursor positioning.
	// For Phase 1 the match metadata is tracked but cursor jump is deferred.
}

func (m TUIModel) handleCopy() tea.Cmd {
	addr := m.tree.SelectedAddr()
	if addr == "" {
		return nil
	}
	return func() tea.Msg {
		_ = clipboard.WriteOSC52(os.Stdout, addr)
		return tui.CopiedMsg{}
	}
}

func (m TUIModel) handleRefresh() tea.Cmd {
	var cmds []tea.Cmd

	if m.store != nil {
		store := m.store
		cmds = append(cmds, func() tea.Msg {
			idx, err := store.ReadIndex()
			if err != nil {
				return tui.ErrorMsg{Filename: "state.json", Message: err.Error()}
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
			return tui.ErrorMsg{Filename: "state.json", Message: err.Error()}
		}
		return tui.StateUpdatedMsg{Index: idx}
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
