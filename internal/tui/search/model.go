package search

import (
	"fmt"

	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"

	"charm.land/bubbles/v2/textinput"
	"github.com/dorkusprime/wolfcastle/internal/tui"
)

// SearchMatch identifies a single match position within pane content.
type SearchMatch struct {
	Row    int
	Col    int
	Length int
}

// SearchModel manages the search bar state, match navigation, and input
// handling for a single pane.
type SearchModel struct {
	input    textinput.Model
	active   bool
	query    string
	matches  []SearchMatch
	current  int // index into matches for n/N navigation
	paneType int // which pane this search is bound to (0=tree, 1=detail)
}

// NewSearchModel returns a SearchModel with a textinput configured for
// slash-prompt search entry.
func NewSearchModel() SearchModel {
	ti := textinput.New()
	ti.Prompt = "/"
	ti.CharLimit = 0
	return SearchModel{input: ti}
}

// Update processes key messages for the search bar. When active, it captures
// Esc, Enter, and printable input. When inactive with confirmed matches, it
// handles n/N for match cycling.
func (m SearchModel) Update(msg tea.Msg) (SearchModel, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.KeyPressMsg:
		if m.active {
			switch msg.String() {
			case "esc":
				m.Dismiss()
				return m, nil
			case "enter":
				m.Confirm()
				return m, nil
			default:
				var cmd tea.Cmd
				m.input, cmd = m.input.Update(msg)
				m.query = m.input.Value()
				return m, cmd
			}
		}

		// Not active, but we may have confirmed matches for n/N cycling.
		if len(m.matches) > 0 {
			switch msg.String() {
			case "n":
				m.NextMatch()
				return m, nil
			case "N", "shift+n":
				m.PrevMatch()
				return m, nil
			}
		}
	}
	return m, nil
}

// Activate opens the search bar for the given pane, clearing any previous
// search state and focusing the text input.
func (m *SearchModel) Activate(paneType int) {
	m.active = true
	m.paneType = paneType
	m.query = ""
	m.matches = nil
	m.current = 0
	m.input.SetValue("")
	m.input.Focus()
}

// Dismiss closes the search bar and clears all state.
func (m *SearchModel) Dismiss() {
	m.active = false
	m.query = ""
	m.matches = nil
	m.current = 0
	m.input.Blur()
}

// Confirm closes the search bar but preserves matches and the current index
// so that n/N navigation continues to work.
func (m *SearchModel) Confirm() {
	m.active = false
	m.input.Blur()
	if len(m.matches) > 0 && m.current >= len(m.matches) {
		m.current = 0
	}
}

// IsActive reports whether the search bar is open and accepting input.
func (m SearchModel) IsActive() bool {
	return m.active
}

// PaneType returns which pane this search is bound to (0=tree, 1=detail).
func (m SearchModel) PaneType() int {
	return m.paneType
}

// HasMatches reports whether there are any search matches.
func (m SearchModel) HasMatches() bool {
	return len(m.matches) > 0
}

// Query returns the current search string.
func (m SearchModel) Query() string {
	return m.query
}

// Current returns the index of the currently selected match.
func (m SearchModel) Current() int {
	return m.current
}

// CurrentMatch returns the match at the current index, or false if there are
// no matches.
func (m SearchModel) CurrentMatch() (SearchMatch, bool) {
	if len(m.matches) == 0 {
		return SearchMatch{}, false
	}
	return m.matches[m.current], true
}

// SetMatches replaces the match list. Called by the parent model after
// filtering pane content against the query.
func (m *SearchModel) SetMatches(matches []SearchMatch) {
	m.matches = matches
	if len(matches) == 0 {
		m.current = 0
	} else if m.current >= len(matches) {
		m.current = 0
	}
}

// NextMatch advances to the next match, wrapping to the beginning.
func (m *SearchModel) NextMatch() {
	if len(m.matches) == 0 {
		return
	}
	m.current = (m.current + 1) % len(m.matches)
}

// PrevMatch moves to the previous match, wrapping to the end.
func (m *SearchModel) PrevMatch() {
	if len(m.matches) == 0 {
		return
	}
	m.current = (m.current - 1 + len(m.matches)) % len(m.matches)
}

// View renders the search bar line. The parent is responsible for placing this
// at the bottom of the appropriate pane.
func (m SearchModel) View() string {
	if !m.active && len(m.matches) == 0 {
		return ""
	}

	dim := lipgloss.NewStyle().Foreground(tui.ColorDimWhite)

	if m.active {
		bar := m.input.View()
		if m.query != "" && len(m.matches) == 0 {
			bar += dim.Render("  No matches. Adjust your aim.")
		} else if m.query != "" {
			info := fmt.Sprintf("  %d/%d matches", m.current+1, len(m.matches))
			bar += dim.Render(info)
		}
		return bar
	}

	// Inactive with confirmed matches: show count for context.
	info := fmt.Sprintf("/%s  %d/%d matches", m.query, m.current+1, len(m.matches))
	return dim.Render(info)
}
