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

// NewWelcomeModel creates a WelcomeModel rooted at startDir. The path is
// resolved to an absolute and the initial directory listing is loaded
// immediately (dirs only, hidden dirs excluded except .wolfcastle).
func NewWelcomeModel(startDir string) WelcomeModel {
	abs, err := filepath.Abs(startDir)
	if err != nil {
		abs = startDir
	}
	m := WelcomeModel{currentDir: abs}
	m.loadDir()
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
		return m.handleEnter()

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

func (m WelcomeModel) handleEnter() (WelcomeModel, tea.Cmd) {
	// No entries: treat Enter as "confirm init in currentDir".
	if len(m.entries) == 0 {
		return m.startInit()
	}

	if m.cursor >= 0 && m.cursor < len(m.entries) {
		entry := m.entries[m.cursor]
		if entry.Name() == ".wolfcastle" {
			// .wolfcastle is a confirmation target, not a navigation target.
			return m.startInit()
		}
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

	// Current path prompt
	b.WriteString(subtitleStyle.Render("Initialize in:"))
	b.WriteString("\n")
	b.WriteString(pathStyle.Render(fmt.Sprintf("> %s", m.currentDir)))

	if len(m.entries) == 0 {
		b.WriteString("    ")
		b.WriteString(subtitleStyle.Render("[Enter to confirm]"))
	}
	b.WriteString("\n\n")

	// Directory listing
	if len(m.entries) > 0 {
		visible := m.visibleEntries()
		for i, entry := range visible {
			idx := m.scrollTop + i
			name := entry.Name()
			if idx == m.cursor {
				line := fmt.Sprintf("  > %s/", name)
				if name == ".wolfcastle" {
					line = fmt.Sprintf("  > %s/    [Enter to confirm]", name)
				}
				b.WriteString(selectedStyle.Render(line))
			} else {
				line := fmt.Sprintf("    %s/", name)
				b.WriteString(normalStyle.Render(line))
			}
			b.WriteString("\n")
		}

		// Scroll indicators
		if m.scrollTop > 0 {
			b.WriteString(subtitleStyle.Render(fmt.Sprintf("  ... %d more above", m.scrollTop)))
			b.WriteString("\n")
		}
		below := len(m.entries) - m.scrollTop - maxVisible
		if below > 0 {
			b.WriteString(subtitleStyle.Render(fmt.Sprintf("  ... %d more below", below)))
			b.WriteString("\n")
		}

		b.WriteString("\n")
		b.WriteString(subtitleStyle.Render("  (Use arrows to browse, Enter to select)"))
	} else {
		b.WriteString(subtitleStyle.Render("  (No subdirectories. Press Enter to initialize here.)"))
	}

	// Error display
	if m.err != nil {
		b.WriteString("\n\n")
		b.WriteString(errorStyle.Render(fmt.Sprintf("Init failed: %s", m.err)))
	}

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

// place centers content within the terminal using lipgloss.Place.
func (m WelcomeModel) place(content string) string {
	w := m.width
	h := m.height
	if w <= 0 {
		w = 80
	}
	if h <= 0 {
		h = 24
	}
	return lipgloss.Place(w, h, lipgloss.Center, lipgloss.Center, content)
}
