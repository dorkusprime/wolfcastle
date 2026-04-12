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

	"github.com/dorkusprime/wolfcastle/internal/config"
	"github.com/dorkusprime/wolfcastle/internal/instance"
	"github.com/dorkusprime/wolfcastle/internal/pipeline"
	"github.com/dorkusprime/wolfcastle/internal/project"
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

// Model drives the launcher screen.
type Model struct {
	// Sessions panel
	instances     []instance.Entry
	sessionCursor int

	// Directory browser panel
	currentDir string
	entries    []os.DirEntry
	dirCursor  int
	scrollTop  int

	// Shared
	focus        focusPanel
	width        int
	height       int
	err          error
	initializing bool
	spinnerFrame int
}

// NewModel creates a launcher rooted at startDir's parent directory.
// If instances are provided, the sessions panel starts with focus.
func NewModel(startDir string, instances []instance.Entry) Model {
	abs, err := filepath.Abs(startDir)
	if err != nil {
		abs = startDir
	}

	m := Model{
		currentDir: abs,
		instances:  instances,
	}
	m.loadDir()

	// Pre-select the first entry.
	for i, e := range m.entries {
		if e.Name() == ".wolfcastle" {
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

// SetSize updates the terminal dimensions for layout calculations.
func (m *Model) SetSize(width, height int) {
	m.width = width
	m.height = height
}

// SetInstances updates the running sessions list (called on InstancesUpdatedMsg).
func (m *Model) SetInstances(instances []instance.Entry) {
	m.instances = instances
	if m.sessionCursor >= len(instances) {
		m.sessionCursor = max(0, len(instances)-1)
	}
}

// Update handles keyboard input and messages for the welcome screen.
func (m Model) Update(msg tea.Msg) (Model, tea.Cmd) {
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

func (m Model) handleKey(msg tea.KeyPressMsg) (Model, tea.Cmd) {
	if m.initializing {
		if key.Matches(msg, tui.WelcomeKeyMap.Quit) {
			return m, tea.Quit
		}
		return m, nil
	}

	// Tab switches panel focus (both panels must exist to toggle).
	if msg.String() == "tab" && len(m.instances) > 0 {
		if m.focus == panelSessions {
			m.focus = panelDirs
		} else {
			m.focus = panelSessions
		}
		return m, nil
	}

	if m.focus == panelSessions {
		return m.handleSessionKey(msg)
	}
	return m.handleDirKey(msg)
}

func (m Model) handleSessionKey(msg tea.KeyPressMsg) (Model, tea.Cmd) {
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

func (m Model) handleDirKey(msg tea.KeyPressMsg) (Model, tea.Cmd) {
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
	case msg.String() == "I":
		// Init in the current directory regardless of contents.
		return m.startInit()
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

func (m Model) handleEnter(msg tea.KeyPressMsg) (Model, tea.Cmd) {
	if len(m.entries) == 0 {
		return m, nil
	}

	if m.dirCursor >= 0 && m.dirCursor < len(m.entries) {
		entry := m.entries[m.dirCursor]
		if entry.Name() == ".wolfcastle" {
			// Opening the .wolfcastle entry itself opens a session
			// rooted at the current directory. startInit's runInit
			// detects the existing scaffold and resolves as an
			// immediate success, which routes through the same
			// InitCompleteMsg handler the fresh-init path uses.
			return m.startInit()
		}
		child := filepath.Join(m.currentDir, entry.Name())
		// If the child is already a wolfcastle project, open a
		// session there instead of descending into its filesystem.
		if hasWolfcastle(child) {
			m.currentDir = child
			return m.startInit()
		}
		m.currentDir = child
		m.dirCursor = 0
		m.scrollTop = 0
		m.err = nil
		m.loadDir()
	}

	return m, nil
}

// hasWolfcastle reports whether dir contains a .wolfcastle directory.
func hasWolfcastle(dir string) bool {
	info, err := os.Stat(filepath.Join(dir, ".wolfcastle"))
	return err == nil && info.IsDir()
}

func (m Model) startInit() (Model, tea.Cmd) {
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

func (m Model) runInit(dir string) tea.Cmd {
	return func() tea.Msg {
		wcDir := filepath.Join(dir, ".wolfcastle")
		// If the directory already exists, treat init as a no-op success
		// so the user can press I in a worktree that's already been
		// initialized without seeing an error.
		if _, statErr := os.Stat(wcDir); statErr == nil {
			return tui.InitCompleteMsg{Dir: dir, Err: nil}
		}
		cfgRepo := config.NewRepository(wcDir)
		promptRepo := pipeline.NewPromptRepository(wcDir)
		svc := project.NewScaffoldService(cfgRepo, promptRepo, nil, wcDir)
		if err := svc.Init(config.DetectIdentity()); err != nil {
			return tui.InitCompleteMsg{Dir: dir, Err: err}
		}
		return tui.InitCompleteMsg{Dir: dir, Err: nil}
	}
}

func (m Model) tickSpinner() tea.Cmd {
	return tea.Tick(80*time.Millisecond, func(time.Time) tea.Msg {
		return tui.SpinnerTickMsg{}
	})
}

func (m *Model) loadDir() {
	dirEntries, err := os.ReadDir(m.currentDir)
	if err != nil {
		m.entries = nil
		m.err = err
		return
	}

	filtered := make([]os.DirEntry, 0, len(dirEntries))
	for _, e := range dirEntries {
		isDir := e.IsDir()
		// Follow symlinks: if it's a symlink, stat the target.
		if !isDir && e.Type()&os.ModeSymlink != 0 {
			target := filepath.Join(m.currentDir, e.Name())
			if info, err := os.Stat(target); err == nil {
				isDir = info.IsDir()
			}
		}
		if !isDir {
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

func (m *Model) scrollIntoCursor() {
	if m.dirCursor < m.scrollTop {
		m.scrollTop = m.dirCursor
	}
	if m.dirCursor >= m.scrollTop+maxVisible {
		m.scrollTop = m.dirCursor - maxVisible + 1
	}
}

// ---------------------------------------------------------------------------
// View renders the welcome screen with session and directory panels.
func (m Model) View() string {
	subtitleStyle := lipgloss.NewStyle().Foreground(tui.ColorDimWhite)
	spinnerStyle := lipgloss.NewStyle().Foreground(tui.ColorNeonCyan)

	var b strings.Builder

	b.WriteString(tui.RenderGradientTitle("WOLFCASTLE", nil))
	b.WriteString("\n\n")

	if m.initializing {
		frame := spinnerFrames[m.spinnerFrame%len(spinnerFrames)]
		b.WriteString(spinnerStyle.Render(frame))
		b.WriteString(" ")
		b.WriteString(subtitleStyle.Render(fmt.Sprintf("Initializing in %s...", m.currentDir)))
		return m.place(b.String())
	}

	// All panels share the same inner width, derived from terminal width.
	innerWidth := m.width*2/3 - 4
	if innerWidth < 40 {
		innerWidth = 40
	}
	if innerWidth > 80 {
		innerWidth = 80
	}

	panelBase := lipgloss.NewStyle().
		Background(tui.ColorBaseBg).
		Border(lipgloss.RoundedBorder()).
		BorderForeground(tui.ColorDeepCyan).
		BorderBackground(tui.ColorBaseBg).
		Padding(0, 1).
		Width(innerWidth)

	// Sessions panel
	if len(m.instances) > 0 {
		sessionsStyle := panelBase
		if m.focus == panelSessions {
			sessionsStyle = sessionsStyle.BorderForeground(tui.ColorNeonCyan)
		}
		b.WriteString(sessionsStyle.Render(m.renderSessions()))
		b.WriteString("\n")
	}

	// Directory browser panel
	dirStyle := panelBase
	if m.focus == panelDirs {
		dirStyle = dirStyle.BorderForeground(tui.ColorNeonCyan)
	}
	b.WriteString(dirStyle.Render(m.renderDirBrowser()))

	// Error
	if m.err != nil {
		errorStyle := lipgloss.NewStyle().Foreground(tui.ColorRed)
		b.WriteString("\n\n")
		b.WriteString(errorStyle.Render(fmt.Sprintf("Init failed: %s", m.err)))
	}

	// Key hints centered, same width as panels (border adds 4, so use innerWidth+4)
	hintBox := lipgloss.NewStyle().
		Width(innerWidth + 4).
		Align(lipgloss.Center)
	b.WriteString("\n")
	b.WriteString(hintBox.Render(m.renderHints()))

	return m.place(b.String())
}

func (m Model) renderSessions() string {
	headingStyle := lipgloss.NewStyle().Bold(true).Foreground(tui.ColorWhite)
	selectedStyle := lipgloss.NewStyle().Bold(true).Foreground(tui.ColorWhite).Background(tui.ColorSelection)
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
		name := tui.InstanceLabel(inst)

		if i == m.sessionCursor && m.focus == panelSessions {
			line := fmt.Sprintf("  %s %s  %s", activeMarker, name, pid)
			b.WriteString(selectedStyle.Render(line))
		} else {
			line := fmt.Sprintf("  %s %s  %s", activeMarker, name, pid)
			b.WriteString(normalStyle.Render(line))
		}
		b.WriteString("\n")
	}

	return b.String()
}

func (m Model) renderDirBrowser() string {
	headingStyle := lipgloss.NewStyle().Bold(true).Foreground(tui.ColorWhite)
	subtitleStyle := lipgloss.NewStyle().Foreground(tui.ColorDimWhite)
	selectedStyle := lipgloss.NewStyle().Bold(true).Foreground(tui.ColorWhite).Background(tui.ColorSelection)
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

		projectStyle := lipgloss.NewStyle().Foreground(tui.ColorGold)
		for i, entry := range visible {
			idx := m.scrollTop + i
			isLast := idx == len(m.entries)-1
			name := entry.Name()
			isProject := entry.Name() != ".wolfcastle" && hasWolfcastle(filepath.Join(m.currentDir, name))

			connector := "├── "
			if isLast {
				connector = "└── "
			}

			if idx == m.dirCursor && m.focus == panelDirs {
				marker := pathStyle.Render("▸ ")
				dirName := selectedStyle.Render(name + "/")
				hint := ""
				switch {
				case name == ".wolfcastle":
					hint = subtitleStyle.Render("  [Enter to open]")
				case isProject:
					hint = subtitleStyle.Render("  [Enter to open]")
				}
				b.WriteString(connectorStyle.Render(connector) + marker + dirName + hint)
			} else {
				style := normalStyle
				if isProject {
					style = projectStyle
				}
				suffix := ""
				if isProject {
					suffix = subtitleStyle.Render(" ◆")
				}
				b.WriteString(connectorStyle.Render(connector) + style.Render(name+"/") + suffix)
			}
			b.WriteString("\n")
		}

		below := len(m.entries) - m.scrollTop - maxVisible
		if below > 0 {
			b.WriteString(subtitleStyle.Render(fmt.Sprintf("    (%d more below)", below)))
			b.WriteString("\n")
		}
	} else {
		b.WriteString(subtitleStyle.Render("  (no subdirectories)"))
		b.WriteString("\n")
		b.WriteString(subtitleStyle.Render("  Press I to initialize wolfcastle here,"))
		b.WriteString("\n")
		b.WriteString(subtitleStyle.Render("  or h/← to go up."))
	}

	return b.String()
}

func (m Model) renderHints() string {
	hintStyle := lipgloss.NewStyle().Foreground(tui.ColorDimWhite)
	dir := filepath.Base(m.currentDir)
	if dir == "" || dir == "/" {
		dir = m.currentDir
	}

	// Line 1: movement
	var move []string
	move = append(move, "[j/↓] down", "[k/↑] up", "[h/←] back")
	if len(m.instances) > 0 {
		move = append(move, "[Tab] switch panel")
	}

	// Line 2: actions
	actions := []string{
		"[l/Enter] open",
		fmt.Sprintf("[I] init %s", dir),
		"[q] quit",
	}

	line1 := strings.Join(move, "  ")
	line2 := strings.Join(actions, "  ")

	return hintStyle.Render(line1) + "\n" + hintStyle.Render(line2)
}

func (m Model) visibleEntries() []os.DirEntry {
	end := m.scrollTop + maxVisible
	if end > len(m.entries) {
		end = len(m.entries)
	}
	return m.entries[m.scrollTop:end]
}

func (m Model) place(content string) string {
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
