package tree

import (
	"fmt"
	"image/color"
	"strings"

	lipgloss "charm.land/lipgloss/v2"

	"github.com/dorkusprime/wolfcastle/internal/state"
)

// Colors and styles for the tree view.
var (
	colorSelected    = lipgloss.Color("52") // dark red background (matches header)
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

// Status glyphs, each pre-styled (no background, suitable for normal rows).
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

// statusGlyphOnBg returns the colored glyph with an explicit background
// so that wrapping in styleSelected/styleSearchMatch doesn't leave gaps.
func statusGlyphOnBg(s state.NodeStatus, bg color.Color) string {
	st := lipgloss.NewStyle().Background(bg).Bold(true)
	switch s {
	case state.StatusComplete:
		return st.Foreground(colorGreen).Render("●")
	case state.StatusInProgress:
		return st.Foreground(colorYellow).Render("◐")
	case state.StatusBlocked:
		return st.Foreground(colorRed).Render("☢")
	default:
		return st.Foreground(lipgloss.Color("250")).Render("◯")
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

	targetText := ""
	if isCurrentTarget {
		targetText = "▶ "
	}

	hintLen := len(row.TaskHint)
	if hintLen > 0 {
		hintLen++
	}
	overhead := len(indent) + 2 + len(targetText) + 2 + hintLen
	maxName := width - overhead
	if maxName < 4 {
		maxName = 4
	}
	name := truncate(row.Name, maxName)

	if selected {
		bg := lipgloss.NewStyle().Background(colorSelected).Foreground(lipgloss.Color("255")).Bold(true)
		var target string
		if isCurrentTarget {
			target = lipgloss.NewStyle().
				Background(colorSelected).
				Foreground(colorTargetMark).
				Bold(true).
				Render("▶ ")
		}
		glyph := statusGlyphOnBg(row.Status, colorSelected)
		var hint string
		if row.TaskHint != "" {
			hint = lipgloss.NewStyle().
				Background(colorSelected).
				Foreground(lipgloss.Color("250")).
				Render(" " + row.TaskHint)
		}
		text := bg.Render(indent+marker+" ") + target + bg.Render(name+" ") + glyph + hint
		used := lipgloss.Width(text)
		if used < width {
			text += bg.Render(strings.Repeat(" ", width-used))
		}
		return text
	}

	if searchHit {
		bg := lipgloss.NewStyle().Background(colorSearchMatch).Foreground(lipgloss.Color("0"))
		var target string
		if isCurrentTarget {
			target = bg.Bold(true).Render("▶ ")
		}
		glyph := statusGlyphOnBg(row.Status, colorSearchMatch)
		var hint string
		if row.TaskHint != "" {
			hint = bg.Render(" " + row.TaskHint)
		}
		text := bg.Render(indent+marker+" ") + target + bg.Render(name+" ") + glyph + hint
		used := lipgloss.Width(text)
		if used < width {
			text += bg.Render(strings.Repeat(" ", width-used))
		}
		return text
	}

	// Unselected: apply per-element coloring on top of styleNormal.
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

	// Extract the task ID from the address (last segment after /).
	taskID := row.Addr
	if idx := strings.LastIndex(row.Addr, "/"); idx >= 0 {
		taskID = row.Addr[idx+1:]
	}

	// Plain layout for width calculation (1 char glyph + spaces).
	plainPrefix := fmt.Sprintf("%s  %s: ", indent, taskID)
	maxTitle := width - len(plainPrefix)
	if maxTitle < 4 {
		maxTitle = 4
	}
	title := truncate(row.Name, maxTitle)

	if selected {
		// Render glyph with selected background so the color shows through.
		glyph := statusGlyphOnBg(row.Status, colorSelected)
		// Build the line: pad before/after glyph with the selected background.
		bg := lipgloss.NewStyle().Background(colorSelected).Foreground(lipgloss.Color("255")).Bold(true)
		text := bg.Render(indent) + glyph + bg.Render(" "+taskID+": "+title)
		// Fill remaining width with selected background.
		used := lipgloss.Width(text)
		if used < width {
			text += bg.Render(strings.Repeat(" ", width-used))
		}
		return text
	}
	if searchHit {
		glyph := statusGlyphOnBg(row.Status, colorSearchMatch)
		bg := lipgloss.NewStyle().Background(colorSearchMatch).Foreground(lipgloss.Color("0"))
		text := bg.Render(indent) + glyph + bg.Render(" "+taskID+": "+title)
		used := lipgloss.Width(text)
		if used < width {
			text += bg.Render(strings.Repeat(" ", width-used))
		}
		return text
	}

	glyph := statusGlyph(row.Status)
	line := fmt.Sprintf("%s%s %s: %s", indent, glyph, taskID, title)
	return styleNormal.Width(width).Render(line)
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
