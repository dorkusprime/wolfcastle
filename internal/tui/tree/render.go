package tree

import (
	"fmt"
	"strings"

	lipgloss "charm.land/lipgloss/v2"

	"github.com/dorkusprime/wolfcastle/internal/state"
)

// Colors and styles for the tree view.
var (
	colorSelected    = lipgloss.Color("236") // dark gray background
	colorNormal      = lipgloss.Color("252") // light gray text
	colorGreen       = lipgloss.Color("2")
	colorYellow      = lipgloss.Color("3")
	colorRed         = lipgloss.Color("1")
	colorDim         = lipgloss.Color("240")
	colorTargetMark  = lipgloss.Color("11") // bright yellow
	colorSearchMatch = lipgloss.Color("3")  // yellow background for search hits

	styleSelected = lipgloss.NewStyle().
			Bold(true).
			Foreground(lipgloss.Color("255")).
			Background(colorSelected)
	styleNormal      = lipgloss.NewStyle().Foreground(colorNormal)
	styleSearchMatch = lipgloss.NewStyle().Foreground(lipgloss.Color("0")).Background(colorSearchMatch)
)

// Status glyphs, each pre-styled.
func statusGlyph(s state.NodeStatus) string {
	switch s {
	case state.StatusComplete:
		return lipgloss.NewStyle().Foreground(colorGreen).Render("●")
	case state.StatusInProgress:
		return lipgloss.NewStyle().Foreground(colorYellow).Render("◐")
	case state.StatusBlocked:
		return lipgloss.NewStyle().Foreground(colorRed).Render("☢")
	default:
		return lipgloss.NewStyle().Foreground(colorDim).Render("◯")
	}
}

// RenderRow produces a single styled line for the given TreeRow.
func RenderRow(row TreeRow, width int, selected bool, isCurrentTarget bool, searchHit ...bool) string {
	hit := len(searchHit) > 0 && searchHit[0]
	if row.IsTask {
		return renderTaskRow(row, width, selected, hit)
	}
	return renderNodeRow(row, width, selected, isCurrentTarget, hit)
}

func renderNodeRow(row TreeRow, width int, selected bool, isCurrentTarget bool, searchHit bool) string {
	indent := strings.Repeat("  ", row.Depth)

	var marker string
	if row.Expandable {
		if row.IsExpanded {
			marker = "▾"
		} else {
			marker = "▸"
		}
	} else {
		marker = " "
	}

	// Build the line with plain glyphs first, then style the entire
	// thing in one pass. Pre-styled fragments contain ANSI reset codes
	// that clear the background mid-line when wrapped in selected.
	var target string
	if isCurrentTarget {
		target = "▶ "
	}

	glyphRune := plainStatusGlyph(row.Status)

	hintLen := len(row.TaskHint)
	if hintLen > 0 {
		hintLen++ // account for the leading space
	}
	overhead := len(indent) + 2 + len(target) + 2 + hintLen
	maxName := width - overhead
	if maxName < 4 {
		maxName = 4
	}

	name := truncate(row.Name, maxName)

	var hint string
	if row.TaskHint != "" {
		hint = " " + row.TaskHint
	}
	line := fmt.Sprintf("%s%s %s%s %s%s", indent, marker, target, name, glyphRune, hint)

	if selected {
		return styleSelected.Width(width).Render(line)
	}
	if searchHit {
		return styleSearchMatch.Width(width).Render(line)
	}
	// Unselected: apply per-element coloring on top of styleNormal.
	// Re-render the line piece-by-piece with foreground colors.
	var coloredTarget string
	if isCurrentTarget {
		coloredTarget = lipgloss.NewStyle().Foreground(colorTargetMark).Bold(true).Render("▶ ")
	}
	coloredGlyph := statusGlyph(row.Status)
	var coloredHint string
	if row.TaskHint != "" {
		coloredHint = " " + lipgloss.NewStyle().Foreground(colorDim).Render(row.TaskHint)
	}
	colored := fmt.Sprintf("%s%s %s%s %s%s", indent, marker, coloredTarget, name, coloredGlyph, coloredHint)
	return styleNormal.Width(width).Render(colored)
}

func renderTaskRow(row TreeRow, width int, selected bool, searchHit bool) string {
	indent := strings.Repeat("  ", row.Depth)
	// Plain glyph rune (un-styled). When the row is wrapped in
	// styleSelected/styleSearchMatch the inner color escapes from a
	// pre-styled glyph would reset the background mid-line.
	glyphRune := plainStatusGlyph(row.Status)

	// Extract the task ID from the address (last segment after /).
	taskID := row.Addr
	if idx := strings.LastIndex(row.Addr, "/"); idx >= 0 {
		taskID = row.Addr[idx+1:]
	}

	// Layout: {indent}{glyph} {taskID}: {title}
	prefix := fmt.Sprintf("%s%s %s: ", indent, glyphRune, taskID)
	maxTitle := width - len(prefix)
	if maxTitle < 4 {
		maxTitle = 4
	}

	title := truncate(row.Name, maxTitle)
	line := prefix + title

	if selected {
		return styleSelected.Width(width).Render(line)
	}
	if searchHit {
		return styleSearchMatch.Width(width).Render(line)
	}
	return styleNormal.Width(width).Render(line)
}

// plainStatusGlyph returns the unstyled glyph rune for a status.
func plainStatusGlyph(s state.NodeStatus) string {
	switch s {
	case state.StatusComplete:
		return "●"
	case state.StatusInProgress:
		return "◐"
	case state.StatusBlocked:
		return "☢"
	default:
		return "◯"
	}
}

// View renders the visible portion of the tree as a single string.
func (m TreeModel) View() string {
	if len(m.flatList) == 0 {
		return styleNormal.Render("  (no nodes)")
	}

	visibleEnd := m.scrollTop + m.height
	if visibleEnd > len(m.flatList) {
		visibleEnd = len(m.flatList)
	}
	start := m.scrollTop
	if start < 0 {
		start = 0
	}
	if start >= len(m.flatList) {
		return ""
	}

	lines := make([]string, 0, visibleEnd-start)
	for i := start; i < visibleEnd; i++ {
		row := m.flatList[i]
		selected := i == m.cursor
		isTarget := row.Addr == m.currentTarget
		searchHit := !selected && m.searchMatches[i]
		lines = append(lines, RenderRow(row, m.width, selected, isTarget, searchHit))
	}

	return strings.Join(lines, "\n")
}

// truncate shortens s to fit within maxLen, appending an ellipsis when
// the string is clipped.
func truncate(s string, maxLen int) string {
	if maxLen <= 0 {
		return ""
	}
	runes := []rune(s)
	if len(runes) <= maxLen {
		return s
	}
	if maxLen <= 1 {
		return "…"
	}
	return string(runes[:maxLen-1]) + "…"
}
