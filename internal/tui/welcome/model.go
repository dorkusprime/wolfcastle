// Package welcome provides the launcher screen shown when wolfcastle
// opens outside an initialized project. It has two panels: running
// sessions (connect to an existing daemon) and a directory browser
// (initialize a new project). Tab switches focus between them.
package welcome

import (
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"charm.land/bubbles/v2/key"
	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"

	"github.com/dorkusprime/wolfcastle/internal/instance"
	"github.com/dorkusprime/wolfcastle/internal/tui"
)

const maxVisible = 20

var spinnerFrames = []string{"⠋", "⠙", "⠹", "⠸", "⠼", "⠴", "⠦", "⠧", "⠇", "⠏"}

// focusPanel tracks which panel has keyboard focus.
type focusPanel int

const (
	panelSessions focusPanel = iota
	panelDirs
)

// ConnectInstanceMsg is sent when the user selects a running session.
type ConnectInstanceMsg struct {
	Entry instance.Entry
}

// WelcomeModel drives the launcher screen.
type WelcomeModel struct {
	// Sessions panel
	instances     []instance.Entry
	sessionCursor int

	// Directory browser panel
	currentDir   string
	entries      []os.DirEntry
	dirCursor    int
	scrollTop    int

	// Shared
	focus        focusPanel
	width        int
	height       int
	err          error
	initializing bool
	spinnerFrame int
}

// NewWelcomeModel creates a launcher rooted at startDir's parent directory.
// If instances are provided, the sessions panel starts with focus.
func NewWelcomeModel(startDir string, instances []instance.Entry) WelcomeModel {
	abs, err := filepath.Abs(startDir)
	if err != nil {
		abs = startDir
	}

	parent := filepath.Dir(abs)
	baseName := filepath.Base(abs)

	m := WelcomeModel{
		currentDir: parent,
		instances:  instances,
	}
	m.loadDir()

	// Pre-select CWD in the parent listing.
	for i, e := range m.entries {
		if e.Name() == baseName {
			m.dirCursor = i
			m.scrollIntoCursor()
			break
		}
	}

	// Start with sessions panel focused if there are running instances.
	if len(instances) > 0 {
		m.focus = panelSessions
	} else {
		m.focus = panelDirs
	}

	return m
}

func (m *WelcomeModel) SetSize(width, height int) {
	m.width = width
	m.height = height
}

// SetInstances updates the running sessions list (called on InstancesUpdatedMsg).
func (m *WelcomeModel) SetInstances(instances []instance.Entry) {
	m.instances = instances
	if m.sessionCursor >= len(instances) {
		m.sessionCursor = max(0, len(instances)-1)
	}
}

func (m WelcomeModel) Update(msg tea.Msg) (WelcomeModel, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.KeyPressMsg:
		return m.handleKey(msg)

	case tui.InitCompleteMsg:
		m.initializing = false
		if msg.Err != nil {
			m.err = msg.Err
		}
		return m, nil

	case tui.SpinnerTickMsg:
		if m.initializing {
			m.spinnerFrame = (m.spinnerFrame + 1) % len(spinnerFrames)
			return m, m.tickSpinner()
		}
		return m, nil

	case tui.InstancesUpdatedMsg:
		m.SetInstances(msg.Instances)
		return m, nil

	case tea.WindowSizeMsg:
		m.width = msg.Width
		m.height = msg.Height
		return m, nil
	}

	return m, nil
}

func (m WelcomeModel) handleKey(msg tea.KeyPressMsg) (WelcomeModel, tea.Cmd) {
	if m.initializing {
		if key.Matches(msg, tui.WelcomeKeyMap.Quit) {
			return m, tea.Quit
		}
		return m, nil
	}

	// Tab switches panel focus.
	if msg.String() == "tab" {
		if m.focus == panelSessions && len(m.entries) > 0 {
			m.focus = panelDirs
		} else if m.focus == panelDirs && len(m.instances) > 0 {
			m.focus = panelSessions
		}
		return m, nil
	}

	if m.focus == panelSessions {
		return m.handleSessionKey(msg)
	}
	return m.handleDirKey(msg)
}

func (m WelcomeModel) handleSessionKey(msg tea.KeyPressMsg) (WelcomeModel, tea.Cmd) {
	switch {
	case key.Matches(msg, tui.WelcomeKeyMap.MoveDown):
		if m.sessionCursor < len(m.instances)-1 {
			m.sessionCursor++
		}
	case key.Matches(msg, tui.WelcomeKeyMap.MoveUp):
		if m.sessionCursor > 0 {
			m.sessionCursor--
		}
	case key.Matches(msg, tui.WelcomeKeyMap.Top):
		m.sessionCursor = 0
	case key.Matches(msg, tui.WelcomeKeyMap.Bottom):
		if len(m.instances) > 0 {
			m.sessionCursor = len(m.instances) - 1
		}
	case key.Matches(msg, tui.WelcomeKeyMap.Enter):
		if msg.String() == "enter" && m.sessionCursor < len(m.instances) {
			return m, func() tea.Msg {
				return ConnectInstanceMsg{Entry: m.instances[m.sessionCursor]}
			}
		}
	case key.Matches(msg, tui.WelcomeKeyMap.Quit):
		return m, tea.Quit
	}
	return m, nil
}

func (m WelcomeModel) handleDirKey(msg tea.KeyPressMsg) (WelcomeModel, tea.Cmd) {
	switch {
	case key.Matches(msg, tui.WelcomeKeyMap.MoveDown):
		if m.dirCursor < len(m.entries)-1 {
			m.dirCursor++
		}
		m.scrollIntoCursor()
	case key.Matches(msg, tui.WelcomeKeyMap.MoveUp):
		if m.dirCursor > 0 {
			m.dirCursor--
		}
		m.scrollIntoCursor()
	case key.Matches(msg, tui.WelcomeKeyMap.Top):
		m.dirCursor = 0
		m.scrollIntoCursor()
	case key.Matches(msg, tui.WelcomeKeyMap.Bottom):
		if len(m.entries) > 0 {
			m.dirCursor = len(m.entries) - 1
		}
		m.scrollIntoCursor()
	case key.Matches(msg, tui.WelcomeKeyMap.Enter):
		return m.handleEnter(msg)
	case key.Matches(msg, tui.WelcomeKeyMap.Back):
		parent := filepath.Dir(m.currentDir)
		if parent != m.currentDir {
			m.currentDir = parent
			m.dirCursor = 0
			m.scrollTop = 0
			m.err = nil
			m.loadDir()
		}
	case key.Matches(msg, tui.WelcomeKeyMap.Quit):
		return m, tea.Quit
	}
	return m, nil
}

func (m WelcomeModel) handleEnter(msg tea.KeyPressMsg) (WelcomeModel, tea.Cmd) {
	isConfirmKey := msg.String() == "enter"

	if len(m.entries) == 0 {
		if isConfirmKey {
			return m.startInit()
		}
		return m, nil
	}

	if m.dirCursor >= 0 && m.dirCursor < len(m.entries) {
		entry := m.entries[m.dirCursor]
		if entry.Name() == ".wolfcastle" {
			if isConfirmKey {
				return m.startInit()
			}
			return m, nil
		}
		child := filepath.Join(m.currentDir, entry.Name())
		m.currentDir = child
		m.dirCursor = 0
		m.scrollTop = 0
		m.err = nil
		m.loadDir()
	}

	return m, nil
}

func (m WelcomeModel) startInit() (WelcomeModel, tea.Cmd) {
	m.initializing = true
	m.spinnerFrame = 0
	m.err = nil
	dir := m.currentDir
	return m, tea.Batch(
		func() tea.Msg { return tui.InitStartedMsg{} },
		m.runInit(dir),
		m.tickSpinner(),
	)
}

func (m WelcomeModel) runInit(dir string) tea.Cmd {
	return func() tea.Msg {
		err := os.MkdirAll(filepath.Join(dir, ".wolfcastle"), 0o755)
		return tui.InitCompleteMsg{Dir: dir, Err: err}
	}
}

func (m WelcomeModel) tickSpinner() tea.Cmd {
	return tea.Tick(80*time.Millisecond, func(time.Time) tea.Msg {
		return tui.SpinnerTickMsg{}
	})
}

func (m *WelcomeModel) loadDir() {
	dirEntries, err := os.ReadDir(m.currentDir)
	if err != nil {
		m.entries = nil
		m.err = err
		return
	}

	filtered := make([]os.DirEntry, 0, len(dirEntries))
	for _, e := range dirEntries {
		if !e.IsDir() {
			continue
		}
		name := e.Name()
		if strings.HasPrefix(name, ".") && name != ".wolfcastle" {
			continue
		}
		filtered = append(filtered, e)
	}

	sort.Slice(filtered, func(i, j int) bool {
		return filtered[i].Name() < filtered[j].Name()
	})

	m.entries = filtered
}

func (m *WelcomeModel) scrollIntoCursor() {
	if m.dirCursor < m.scrollTop {
		m.scrollTop = m.dirCursor
	}
	if m.dirCursor >= m.scrollTop+maxVisible {
		m.scrollTop = m.dirCursor - maxVisible + 1
	}
}

// ---------------------------------------------------------------------------
// View
// ---------------------------------------------------------------------------

func (m WelcomeModel) View() string {
	titleStyle := lipgloss.NewStyle().Bold(true).Foreground(tui.ColorWhite)
	subtitleStyle := lipgloss.NewStyle().Foreground(tui.ColorDimWhite)
	spinnerStyle := lipgloss.NewStyle().Foreground(tui.ColorYellow)

	var b strings.Builder

	b.WriteString(titleStyle.Render("WOLFCASTLE"))
	b.WriteString("\n\n")

	if m.initializing {
		frame := spinnerFrames[m.spinnerFrame%len(spinnerFrames)]
		b.WriteString(spinnerStyle.Render(frame))
		b.WriteString(" ")
		b.WriteString(subtitleStyle.Render(fmt.Sprintf("Initializing in %s...", m.currentDir)))
		return m.place(b.String())
	}

	// Sessions panel
	if len(m.instances) > 0 {
		b.WriteString(m.renderSessions())
		b.WriteString("\n\n")
	}

	// Directory browser panel
	b.WriteString(m.renderDirBrowser())

	// Error
	if m.err != nil {
		errorStyle := lipgloss.NewStyle().Foreground(tui.ColorRed)
		b.WriteString("\n\n")
		b.WriteString(errorStyle.Render(fmt.Sprintf("Init failed: %s", m.err)))
	}

	// Key hints
	b.WriteString("\n\n")
	b.WriteString(m.renderHints())

	return m.place(b.String())
}

func (m WelcomeModel) renderSessions() string {
	headingStyle := lipgloss.NewStyle().Bold(true).Foreground(tui.ColorWhite)
	selectedStyle := lipgloss.NewStyle().Bold(true).Foreground(tui.ColorWhite).Background(tui.ColorDarkGray)
	normalStyle := lipgloss.NewStyle().Foreground(tui.ColorLightGray)
	dimStyle := lipgloss.NewStyle().Foreground(tui.ColorDimWhite)
	activeMarker := lipgloss.NewStyle().Foreground(tui.ColorGreen).Render("●")

	var b strings.Builder

	label := "RUNNING SESSIONS"
	if m.focus == panelSessions {
		b.WriteString(headingStyle.Render(label))
	} else {
		b.WriteString(dimStyle.Render(label))
	}
	b.WriteString(dimStyle.Render(fmt.Sprintf("  %d active", len(m.instances))))
	b.WriteString("\n")

	for i, inst := range m.instances {
		pid := dimStyle.Render(fmt.Sprintf("PID %d", inst.PID))
		branch := inst.Branch
		if branch == "" {
			branch = filepath.Base(inst.Worktree)
		}

		if i == m.sessionCursor && m.focus == panelSessions {
			line := fmt.Sprintf("  %s %s  %s", activeMarker, branch, pid)
			b.WriteString(selectedStyle.Render(line))
		} else {
			line := fmt.Sprintf("  %s %s  %s", activeMarker, branch, pid)
			b.WriteString(normalStyle.Render(line))
		}
		b.WriteString("\n")
	}

	return b.String()
}

func (m WelcomeModel) renderDirBrowser() string {
	headingStyle := lipgloss.NewStyle().Bold(true).Foreground(tui.ColorWhite)
	subtitleStyle := lipgloss.NewStyle().Foreground(tui.ColorDimWhite)
	selectedStyle := lipgloss.NewStyle().Bold(true).Foreground(tui.ColorWhite).Background(tui.ColorDarkGray)
	normalStyle := lipgloss.NewStyle().Foreground(tui.ColorLightGray)
	pathStyle := lipgloss.NewStyle().Foreground(tui.ColorWhite).Bold(true)
	dimStyle := lipgloss.NewStyle().Foreground(tui.ColorDimWhite)
	connectorStyle := subtitleStyle

	var b strings.Builder

	label := "INITIALIZE"
	if m.focus == panelDirs {
		b.WriteString(headingStyle.Render(label))
	} else {
		b.WriteString(dimStyle.Render(label))
	}
	b.WriteString("\n")

	// Breadcrumb
	b.WriteString(dimStyle.Render("/") + pathStyle.Render(m.currentDir))
	b.WriteString("\n\n")

	if len(m.entries) > 0 {
		visible := m.visibleEntries()

		if m.scrollTop > 0 {
			b.WriteString(subtitleStyle.Render(fmt.Sprintf("    (%d more above)", m.scrollTop)))
			b.WriteString("\n")
		}

		for i, entry := range visible {
			idx := m.scrollTop + i
			isLast := idx == len(m.entries)-1
			name := entry.Name()

			connector := "├── "
			if isLast {
				connector = "└── "
			}

			if idx == m.dirCursor && m.focus == panelDirs {
				marker := pathStyle.Render("▸ ")
				dirName := selectedStyle.Render(name + "/")
				hint := ""
				if name == ".wolfcastle" {
					hint = subtitleStyle.Render("  [Enter to init]")
				}
				b.WriteString(connectorStyle.Render(connector) + marker + dirName + hint)
			} else {
				b.WriteString(connectorStyle.Render(connector) + normalStyle.Render(name+"/"))
			}
			b.WriteString("\n")
		}

		below := len(m.entries) - m.scrollTop - maxVisible
		if below > 0 {
			b.WriteString(subtitleStyle.Render(fmt.Sprintf("    (%d more below)", below)))
			b.WriteString("\n")
		}
	} else {
		b.WriteString(subtitleStyle.Render("  (empty directory)"))
		b.WriteString("\n")
		b.WriteString(subtitleStyle.Render("  Press Enter to initialize wolfcastle here,"))
		b.WriteString("\n")
		b.WriteString(subtitleStyle.Render("  or h/← to go up."))
	}

	return b.String()
}

func (m WelcomeModel) renderHints() string {
	hintStyle := lipgloss.NewStyle().Foreground(tui.ColorDimWhite)
	hints := "[j/k] navigate  [Enter] select  [h] back  [q] quit"
	if len(m.instances) > 0 {
		hints = "[Tab] switch panel  [j/k] navigate  [Enter] select  [h] back  [q] quit"
	}
	return hintStyle.Render(hints)
}

func (m WelcomeModel) visibleEntries() []os.DirEntry {
	end := m.scrollTop + maxVisible
	if end > len(m.entries) {
		end = len(m.entries)
	}
	return m.entries[m.scrollTop:end]
}

func (m WelcomeModel) place(content string) string {
	w := m.width
	h := m.height
	if w <= 0 {
		w = 80
	}
	if h <= 0 {
		h = 24
	}

	contentWidth := lipgloss.Width(content)
	maxBox := w - 4
	if contentWidth > maxBox {
		contentWidth = maxBox
	}
	if contentWidth < 40 {
		contentWidth = 40
	}

	box := lipgloss.NewStyle().
		Width(contentWidth).
		Align(lipgloss.Left).
		Render(content)

	// Center the first line (WOLFCASTLE title) within the box.
	lines := strings.Split(box, "\n")
	if len(lines) > 0 {
		lines[0] = lipgloss.PlaceHorizontal(contentWidth, lipgloss.Center, strings.TrimRight(lines[0], " "))
		box = strings.Join(lines, "\n")
	}

	return lipgloss.Place(w, h, lipgloss.Center, lipgloss.Center, box)
}
