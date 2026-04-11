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
	colorSelected    = lipgloss.Color("52")  // dark red background (matches header)
	colorNormal      = lipgloss.Color("252") // light gray text
	colorGreen       = lipgloss.Color("2")
	colorYellow      = lipgloss.Color("3")
	colorRed         = lipgloss.Color("1")
	colorDim         = lipgloss.Color("240")
	colorTargetMark  = lipgloss.Color("11") // bright yellow
	colorSearchMatch = lipgloss.Color("3")  // yellow background for search hits
	// Muted version of colorSearchMatch used for rows that are on the
	// path to a hidden literal match. Reads as related-but-secondary
	// at a glance so the user can follow the trail without confusing
	// ancestor markers for direct hits.
	colorSearchAncestor = lipgloss.Color("58") // dark olive

	styleNormal = lipgloss.NewStyle().Foreground(colorNormal)
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

// taskStatusGlyph returns the glyph for a task row. Tasks use the
// arrow indicator → for in_progress to match the wolfcastle status
// screen, which is the canonical representation of "the daemon is
// hammering on this." Other states fall through to the regular
// statusGlyph.
func taskStatusGlyph(s state.NodeStatus) string {
	if s == state.StatusInProgress {
		return lipgloss.NewStyle().Foreground(colorYellow).Render("→")
	}
	return statusGlyph(s)
}

// taskStatusGlyphOnBg is the background-aware variant of
// taskStatusGlyph for selected/search-highlighted task rows.
func taskStatusGlyphOnBg(s state.NodeStatus, bg color.Color) string {
	if s == state.StatusInProgress {
		return lipgloss.NewStyle().Background(bg).Bold(true).Foreground(colorYellow).Render("→")
	}
	return statusGlyphOnBg(s, bg)
}

// RenderRow produces a single styled line for the given Row.
//
// hits encodes the search highlight state for this row:
//
//	hits[0] (literal)  — the row's own name matches the search query
//	hits[1] (ancestor) — a descendant of this row matches the query,
//	                     but the row itself does not. Renders as a
//	                     muted version of the literal-match highlight
//	                     so the user can see the trail toward hidden
//	                     matches without confusing them for direct
//	                     hits.
//
// When both flags are set the literal treatment wins. Older callers
// may pass a single bool which is interpreted as literal-only for
// backward compatibility with the variadic shape.
func RenderRow(row Row, width int, selected bool, isCurrentTarget bool, hits ...bool) string {
	literal := len(hits) > 0 && hits[0]
	ancestor := len(hits) > 1 && hits[1]
	if row.IsTask {
		return renderTaskRow(row, width, selected, literal, ancestor)
	}
	return renderNodeRow(row, width, selected, isCurrentTarget, literal, ancestor)
}

func renderNodeRow(row Row, width int, selected bool, isCurrentTarget bool, literalHit, ancestorHit bool) string {
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

	// Literal match wins over ancestor when both are set.
	if literalHit {
		return renderNodeRowWithBg(row, width, indent, marker, name, isCurrentTarget, colorSearchMatch, lipgloss.Color("0"))
	}
	if ancestorHit {
		return renderNodeRowWithBg(row, width, indent, marker, name, isCurrentTarget, colorSearchAncestor, lipgloss.Color("253"))
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

// renderNodeRowWithBg renders a node row with a solid background
// color (used for both literal-match and ancestor-of-match
// highlights). The two cases differ only in the bg/fg pair, so the
// shared layout/padding logic lives here.
func renderNodeRowWithBg(row Row, width int, indent, marker, name string, isCurrentTarget bool, bgColor, fgColor color.Color) string {
	bg := lipgloss.NewStyle().Background(bgColor).Foreground(fgColor)
	var target string
	if isCurrentTarget {
		target = bg.Bold(true).Render("▶ ")
	}
	glyph := statusGlyphOnBg(row.Status, bgColor)
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

func renderTaskRow(row Row, width int, selected bool, literalHit, ancestorHit bool) string {
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
		// Tasks use the arrow indicator → for in_progress to match
		// the wolfcastle status screen.
		glyph := taskStatusGlyphOnBg(row.Status, colorSelected)
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
	// Literal match wins over ancestor when both are set.
	if literalHit {
		return renderTaskRowWithBg(row, width, indent, taskID, title, colorSearchMatch, lipgloss.Color("0"))
	}
	if ancestorHit {
		return renderTaskRowWithBg(row, width, indent, taskID, title, colorSearchAncestor, lipgloss.Color("253"))
	}

	glyph := taskStatusGlyph(row.Status)
	line := fmt.Sprintf("%s%s %s: %s", indent, glyph, taskID, title)
	return styleNormal.Width(width).Render(line)
}

// renderTaskRowWithBg renders a task row with a solid background
// for both literal-match and ancestor-of-match highlight cases.
// The two differ only in the bg/fg pair.
func renderTaskRowWithBg(row Row, width int, indent, taskID, title string, bgColor, fgColor color.Color) string {
	glyph := taskStatusGlyphOnBg(row.Status, bgColor)
	bg := lipgloss.NewStyle().Background(bgColor).Foreground(fgColor)
	text := bg.Render(indent) + glyph + bg.Render(" "+taskID+": "+title)
	used := lipgloss.Width(text)
	if used < width {
		text += bg.Render(strings.Repeat(" ", width-used))
	}
	return text
}

// View renders the visible portion of the tree as a single string.
func (m Model) View() string {
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
		// Address-keyed search lookups so highlights survive
		// flat-list rebuilds (collapse/expand). Selected row never
		// gets a search highlight overlay because the selection
		// styling already takes the foreground.
		var literal, ancestor bool
		if !selected {
			literal = m.searchLiteral[row.Addr]
			ancestor = m.searchAncestor[row.Addr]
		}
		lines = append(lines, RenderRow(row, m.width, selected, isTarget, literal, ancestor))
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
