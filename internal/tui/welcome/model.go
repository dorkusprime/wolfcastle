// Package welcome provides the directory-browser welcome screen shown when
// wolfcastle launches outside an initialized project. The user navigates
// the filesystem, picks a directory, and confirms to run project init.
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

	"github.com/dorkusprime/wolfcastle/internal/tui"
)

// maxVisible is the number of directory entries shown before scrolling kicks in.
const maxVisible = 20

// spinnerFrames is the braille-dot spinner sequence used during init.
var spinnerFrames = []string{"⠋", "⠙", "⠹", "⠸", "⠼", "⠴", "⠦", "⠧", "⠇", "⠏"}

// WelcomeModel drives the directory-browser welcome screen.
type WelcomeModel struct {
	currentDir   string
	entries      []os.DirEntry
	cursor       int
	width        int
	height       int
	err          error
	initializing bool
	spinnerFrame int
	scrollTop    int
}

// NewWelcomeModel creates a WelcomeModel rooted at startDir's parent
// directory, with startDir pre-selected in the listing. This ensures
// there's always a navigable list on first render.
func NewWelcomeModel(startDir string) WelcomeModel {
	abs, err := filepath.Abs(startDir)
	if err != nil {
		abs = startDir
	}

	// Start in the parent so the original CWD appears as a selectable entry.
	parent := filepath.Dir(abs)
	baseName := filepath.Base(abs)

	m := WelcomeModel{currentDir: parent}
	m.loadDir()

	// Pre-select the original CWD in the parent listing.
	for i, e := range m.entries {
		if e.Name() == baseName {
			m.cursor = i
			m.scrollIntoCursor()
			break
		}
	}

	return m
}

// SetSize updates the viewport dimensions.
func (m *WelcomeModel) SetSize(width, height int) {
	m.width = width
	m.height = height
}

// Update processes incoming messages and returns the updated model plus any
// commands that should fire next.
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

	case tea.WindowSizeMsg:
		m.width = msg.Width
		m.height = msg.Height
		return m, nil
	}

	return m, nil
}

func (m WelcomeModel) handleKey(msg tea.KeyPressMsg) (WelcomeModel, tea.Cmd) {
	if m.initializing {
		// Swallow all keys while init is running, except quit.
		if key.Matches(msg, tui.WelcomeKeyMap.Quit) {
			return m, tea.Quit
		}
		return m, nil
	}

	switch {
	case key.Matches(msg, tui.WelcomeKeyMap.MoveDown):
		if m.cursor < len(m.entries)-1 {
			m.cursor++
		}
		m.scrollIntoCursor()

	case key.Matches(msg, tui.WelcomeKeyMap.MoveUp):
		if m.cursor > 0 {
			m.cursor--
		}
		m.scrollIntoCursor()

	case key.Matches(msg, tui.WelcomeKeyMap.Top):
		m.cursor = 0
		m.scrollIntoCursor()

	case key.Matches(msg, tui.WelcomeKeyMap.Bottom):
		if len(m.entries) > 0 {
			m.cursor = len(m.entries) - 1
		}
		m.scrollIntoCursor()

	case key.Matches(msg, tui.WelcomeKeyMap.Enter):
		return m.handleEnter(msg)

	case key.Matches(msg, tui.WelcomeKeyMap.Back):
		parent := filepath.Dir(m.currentDir)
		if parent != m.currentDir {
			m.currentDir = parent
			m.cursor = 0
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
	// Only the actual Enter key confirms init. l/right just navigate.
	isConfirmKey := msg.String() == "enter"

	// No entries: Enter confirms init in currentDir; l/right do nothing.
	if len(m.entries) == 0 {
		if isConfirmKey {
			return m.startInit()
		}
		return m, nil
	}

	if m.cursor >= 0 && m.cursor < len(m.entries) {
		entry := m.entries[m.cursor]
		if entry.Name() == ".wolfcastle" {
			// .wolfcastle is a confirmation target, only via Enter.
			if isConfirmKey {
				return m.startInit()
			}
			return m, nil
		}
		// Navigate into directory (Enter, l, or right all work here).
		child := filepath.Join(m.currentDir, entry.Name())
		m.currentDir = child
		m.cursor = 0
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

// runInit returns a tea.Cmd that creates the .wolfcastle directory and
// reports back via InitCompleteMsg. The real project init will replace this
// when wired in app.go.
func (m WelcomeModel) runInit(dir string) tea.Cmd {
	return func() tea.Msg {
		err := os.MkdirAll(filepath.Join(dir, ".wolfcastle"), 0o755)
		return tui.InitCompleteMsg{Dir: dir, Err: err}
	}
}

// tickSpinner returns a tea.Cmd that sends a SpinnerTickMsg after 80ms.
func (m WelcomeModel) tickSpinner() tea.Cmd {
	return tea.Tick(80*time.Millisecond, func(time.Time) tea.Msg {
		return tui.SpinnerTickMsg{}
	})
}

// loadDir reads currentDir and populates entries with directories only,
// excluding hidden directories (names starting with '.') except for
// .wolfcastle. Results are sorted alphabetically.
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

// scrollIntoCursor adjusts scrollTop so the cursor row stays visible.
func (m *WelcomeModel) scrollIntoCursor() {
	if m.cursor < m.scrollTop {
		m.scrollTop = m.cursor
	}
	if m.cursor >= m.scrollTop+maxVisible {
		m.scrollTop = m.cursor - maxVisible + 1
	}
}

// ---------------------------------------------------------------------------
// View
// ---------------------------------------------------------------------------

// View renders the centered welcome screen with directory browser.
func (m WelcomeModel) View() string {
	titleStyle := lipgloss.NewStyle().Bold(true).Foreground(tui.ColorWhite)
	subtitleStyle := lipgloss.NewStyle().Foreground(tui.ColorDimWhite)
	selectedStyle := lipgloss.NewStyle().Bold(true).Foreground(tui.ColorWhite).Background(tui.ColorDarkGray)
	normalStyle := lipgloss.NewStyle().Foreground(tui.ColorLightGray)
	errorStyle := lipgloss.NewStyle().Foreground(tui.ColorRed)
	spinnerStyle := lipgloss.NewStyle().Foreground(tui.ColorYellow)
	pathStyle := lipgloss.NewStyle().Foreground(tui.ColorWhite).Bold(true)

	var b strings.Builder

	// Title
	b.WriteString(titleStyle.Render("WOLFCASTLE"))
	b.WriteString("\n\n")
	b.WriteString(subtitleStyle.Render("No project found in this directory."))
	b.WriteString("\n\n")

	// Initializing state
	if m.initializing {
		frame := spinnerFrames[m.spinnerFrame%len(spinnerFrames)]
		b.WriteString(spinnerStyle.Render(frame))
		b.WriteString(" ")
		b.WriteString(subtitleStyle.Render(fmt.Sprintf("Initializing in %s...", m.currentDir)))
		return m.place(b.String())
	}

	// Current path as a breadcrumb trail
	dimSlash := subtitleStyle.Render("/")
	breadcrumb := dimSlash + pathStyle.Render(m.currentDir)
	b.WriteString(breadcrumb)
	b.WriteString("\n\n")

	// Directory listing with tree connectors
	if len(m.entries) > 0 {
		connectorStyle := subtitleStyle
		visible := m.visibleEntries()

		if m.scrollTop > 0 {
			b.WriteString(subtitleStyle.Render(fmt.Sprintf("    (%d more above)", m.scrollTop)))
			b.WriteString("\n")
		}

		for i, entry := range visible {
			idx := m.scrollTop + i
			isLast := idx == len(m.entries)-1
			name := entry.Name()

			// Tree connector
			connector := "├── "
			if isLast {
				connector = "└── "
			}

			if idx == m.cursor {
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

	// Error display
	if m.err != nil {
		b.WriteString("\n\n")
		b.WriteString(errorStyle.Render(fmt.Sprintf("Init failed: %s", m.err)))
	}

	// Key hints
	hintStyle := lipgloss.NewStyle().Foreground(tui.ColorDimWhite)
	b.WriteString("\n\n")
	b.WriteString(hintStyle.Render("[j/k] navigate  [Enter] select  [h] back  [q] quit"))

	return m.place(b.String())
}

// visibleEntries returns the slice of entries that fit in the scroll window.
func (m WelcomeModel) visibleEntries() []os.DirEntry {
	end := m.scrollTop + maxVisible
	if end > len(m.entries) {
		end = len(m.entries)
	}
	return m.entries[m.scrollTop:end]
}

// place centers content as a left-aligned block within the terminal.
// The content is first wrapped in a fixed-width left-aligned box so that
// lines stay aligned with each other, then that box is centered.
func (m WelcomeModel) place(content string) string {
	w := m.width
	h := m.height
	if w <= 0 {
		w = 80
	}
	if h <= 0 {
		h = 24
	}

	// Determine the content's natural width (widest line).
	contentWidth := lipgloss.Width(content)
	maxBox := w - 4 // leave some margin
	if contentWidth > maxBox {
		contentWidth = maxBox
	}
	if contentWidth < 40 {
		contentWidth = 40
	}

	// Wrap in a left-aligned box so all lines share the same left edge.
	box := lipgloss.NewStyle().
		Width(contentWidth).
		Align(lipgloss.Left).
		Render(content)

	return lipgloss.Place(w, h, lipgloss.Center, lipgloss.Center, box)
}
