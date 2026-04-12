package app

import (
	"os"
	"path/filepath"
	"sort"
	"strings"

	"charm.land/bubbles/v2/key"
	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"

	"github.com/dorkusprime/wolfcastle/internal/instance"
	"github.com/dorkusprime/wolfcastle/internal/tui"
)

// TabPickerModel is the directory picker overlay for creating new tabs.
// It shows running sessions at the top, followed by a filesystem browser.
type TabPickerModel struct {
	dir       string // current directory being browsed
	entries   []dirEntry
	cursor    int
	width     int
	height    int
	instances []instance.Entry
	err       string
}

type dirEntry struct {
	name          string
	isDir         bool
	hasWolfcastle bool   // .wolfcastle/ exists inside
	isSession     bool   // from instance registry
	worktree      string // for sessions: the worktree path
}

// TabPickerResultMsg is sent when the user selects a directory.
type TabPickerResultMsg struct {
	Dir string
}

// TabPickerCancelMsg is sent when the user dismisses the picker.
type TabPickerCancelMsg struct{}

func newTabPicker(startDir string, instances []instance.Entry) TabPickerModel {
	m := TabPickerModel{
		dir:       startDir,
		instances: instances,
	}
	m.loadDir()
	return m
}

func (m *TabPickerModel) SetSize(w, h int) {
	m.width = w
	m.height = h
}

func (m *TabPickerModel) loadDir() {
	m.entries = nil
	m.cursor = 0
	m.err = ""

	// Running sessions first.
	for _, inst := range m.instances {
		// Skip sessions that are in the current browse directory (they'll
		// show as regular entries with a project marker).
		if filepath.Dir(inst.Worktree) == m.dir {
			continue
		}
		m.entries = append(m.entries, dirEntry{
			name:      filepath.Base(inst.Worktree),
			isDir:     true,
			isSession: true,
			worktree:  inst.Worktree,
		})
	}

	entries, err := os.ReadDir(m.dir)
	if err != nil {
		m.err = err.Error()
		return
	}

	var dirs []dirEntry
	for _, e := range entries {
		if !e.IsDir() || strings.HasPrefix(e.Name(), ".") {
			continue
		}
		d := dirEntry{
			name:  e.Name(),
			isDir: true,
		}
		// Check for .wolfcastle/ inside.
		wolfPath := filepath.Join(m.dir, e.Name(), ".wolfcastle")
		if info, statErr := os.Stat(wolfPath); statErr == nil && info.IsDir() {
			d.hasWolfcastle = true
		}
		dirs = append(dirs, d)
	}
	sort.Slice(dirs, func(i, j int) bool {
		// Projects first, then alphabetical.
		if dirs[i].hasWolfcastle != dirs[j].hasWolfcastle {
			return dirs[i].hasWolfcastle
		}
		return dirs[i].name < dirs[j].name
	})
	m.entries = append(m.entries, dirs...)
}

func (m TabPickerModel) Update(msg tea.KeyPressMsg) (TabPickerModel, tea.Cmd) {
	switch {
	case key.Matches(msg, dismissKey):
		return m, func() tea.Msg { return TabPickerCancelMsg{} }

	case key.Matches(msg, tui.WelcomeKeyMap.MoveDown):
		if m.cursor < len(m.entries)-1 {
			m.cursor++
		}

	case key.Matches(msg, tui.WelcomeKeyMap.MoveUp):
		if m.cursor > 0 {
			m.cursor--
		}

	case key.Matches(msg, tui.WelcomeKeyMap.Top):
		m.cursor = 0

	case key.Matches(msg, tui.WelcomeKeyMap.Bottom):
		if len(m.entries) > 0 {
			m.cursor = len(m.entries) - 1
		}

	case key.Matches(msg, tui.WelcomeKeyMap.Enter):
		if m.cursor < len(m.entries) {
			entry := m.entries[m.cursor]
			if entry.isSession {
				return m, func() tea.Msg { return TabPickerResultMsg{Dir: entry.worktree} }
			}
			selected := filepath.Join(m.dir, entry.name)
			if entry.hasWolfcastle {
				return m, func() tea.Msg { return TabPickerResultMsg{Dir: selected} }
			}
			// Navigate into subdirectory.
			m.dir = selected
			m.loadDir()
		}

	case key.Matches(msg, tui.WelcomeKeyMap.Back):
		parent := filepath.Dir(m.dir)
		if parent != m.dir {
			m.dir = parent
			m.loadDir()
		}
	}
	return m, nil
}

func (m TabPickerModel) View() string {
	var b strings.Builder

	title := tui.ModalTitleStyle.Render("  NEW TAB  ")
	b.WriteString(title + "\n\n")

	dirLabel := lipgloss.NewStyle().Foreground(tui.ColorDimWhite).Render(m.dir)
	b.WriteString("  " + dirLabel + "\n\n")

	if m.err != "" {
		b.WriteString("  " + lipgloss.NewStyle().Foreground(tui.ColorRed).Render(m.err) + "\n")
		return b.String()
	}

	if len(m.entries) == 0 {
		b.WriteString("  " + lipgloss.NewStyle().Foreground(tui.ColorDimWhite).Render("Empty directory.") + "\n")
		return b.String()
	}

	visibleHeight := m.height - 6
	if visibleHeight < 1 {
		visibleHeight = 1
	}

	// Compute scroll window.
	start := 0
	if m.cursor >= visibleHeight {
		start = m.cursor - visibleHeight + 1
	}
	end := start + visibleHeight
	if end > len(m.entries) {
		end = len(m.entries)
	}

	for i := start; i < end; i++ {
		entry := m.entries[i]
		selected := i == m.cursor

		var icon, label string
		if entry.isSession {
			icon = "● "
			label = entry.name + " (" + filepath.Base(filepath.Dir(entry.worktree)) + ")"
		} else if entry.hasWolfcastle {
			icon = "◆ "
			label = entry.name
		} else {
			icon = "  "
			label = entry.name + "/"
		}

		line := "  " + icon + label

		if selected {
			line = lipgloss.NewStyle().
				Background(tui.ColorSelection).
				Foreground(tui.ColorWhite).
				Bold(true).
				Width(m.width - 4).
				Render(line)
		} else if entry.isSession {
			line = lipgloss.NewStyle().Foreground(tui.ColorGreen).Render(line)
		} else if entry.hasWolfcastle {
			line = lipgloss.NewStyle().Foreground(tui.ColorYellow).Render(line)
		} else {
			line = lipgloss.NewStyle().Foreground(tui.ColorDimWhite).Render(line)
		}
		b.WriteString(line + "\n")
	}

	b.WriteString("\n")
	hint := lipgloss.NewStyle().Foreground(tui.ColorDimWhite).Render(
		"  Enter: select  h/←: up  Esc: cancel")
	b.WriteString(hint)

	return b.String()
}
