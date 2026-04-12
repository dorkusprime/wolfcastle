package tui

import (
	"fmt"
	"image/color"
	"path/filepath"

	"charm.land/lipgloss/v2"

	"github.com/dorkusprime/wolfcastle/internal/instance"
	"github.com/dorkusprime/wolfcastle/internal/state"
)

// Color palette constants
var (
	ColorWhite     = lipgloss.Color("15")
	ColorDimWhite  = lipgloss.Color("245")
	ColorLightGray = lipgloss.Color("252")
	ColorDarkGray  = lipgloss.Color("236")
	ColorDimGray   = lipgloss.Color("240")
	ColorOverlayBg = lipgloss.Color("235")
	ColorDarkRed   = lipgloss.Color("52")
	ColorRed       = lipgloss.Color("1")
	ColorGreen     = lipgloss.Color("2")
	ColorYellow    = lipgloss.Color("3")
)

// Header styles
var (
	HeaderStyle = lipgloss.NewStyle().
			Foreground(ColorWhite).
			Background(ColorDarkRed)

	HeaderBoldStyle = lipgloss.NewStyle().
			Foreground(ColorWhite).
			Background(ColorDarkRed).
			Bold(true)
)

// Tree styles
var (
	TreeSelectedStyle = lipgloss.NewStyle().
				Foreground(ColorWhite).
				Background(ColorDarkGray).
				Bold(true)

	TreeNormalStyle = lipgloss.NewStyle().
			Foreground(ColorLightGray)

	TreeSearchHighlight = lipgloss.NewStyle().
				Background(ColorYellow)
)

// Footer styles
var FooterStyle = lipgloss.NewStyle().
	Foreground(ColorDimWhite)

// Error bar styles
var ErrorBarStyle = lipgloss.NewStyle().
	Foreground(ColorRed).
	Background(ColorDarkRed).
	Bold(true)

// Dashboard styles
var (
	DashboardHeadingStyle = lipgloss.NewStyle().
				Foreground(ColorWhite).
				Bold(true)

	DashboardBodyStyle = lipgloss.NewStyle().
				Foreground(ColorLightGray)
)

// Border styles
var (
	FocusedBorderStyle = lipgloss.NewStyle().
				Border(lipgloss.RoundedBorder()).
				BorderForeground(ColorRed)

	UnfocusedBorderStyle = lipgloss.NewStyle().
				Border(lipgloss.RoundedBorder()).
				BorderForeground(ColorDimGray)
)

// Spinner style
var SpinnerStyle = lipgloss.NewStyle().
	Foreground(ColorYellow)

// Overlay styles (shared by help and modals)
var (
	HelpOverlayStyle = lipgloss.NewStyle().
				Background(ColorOverlayBg).
				Border(lipgloss.RoundedBorder()).
				BorderForeground(ColorDimWhite)

	HelpTitleStyle = lipgloss.NewStyle().
			Foreground(ColorWhite).
			Bold(true)

	ModalOverlayStyle = lipgloss.NewStyle().
				Background(ColorOverlayBg).
				Border(lipgloss.RoundedBorder()).
				BorderForeground(ColorDimWhite).
				BorderBackground(ColorOverlayBg)

	ModalTitleStyle = lipgloss.NewStyle().
			Background(ColorOverlayBg).
			Foreground(ColorWhite).
			Bold(true)

	ModalDimStyle = lipgloss.NewStyle().
			Background(ColorOverlayBg).
			Foreground(ColorDimWhite)

	ModalAccentStyle = lipgloss.NewStyle().
				Background(ColorOverlayBg).
				Foreground(ColorYellow)
)

// Current target indicator
var TargetIndicatorStyle = lipgloss.NewStyle().
	Foreground(ColorYellow).
	Bold(true)

// StatusGlyph pairs a Unicode glyph with its display color.
type StatusGlyph struct {
	Glyph string
	Color color.Color
}

// NodeStatusGlyphs maps each node lifecycle status to its display glyph and color.
var NodeStatusGlyphs = map[string]StatusGlyph{
	string(state.StatusComplete):   {Glyph: "●", Color: ColorGreen},
	string(state.StatusInProgress): {Glyph: "◐", Color: ColorYellow},
	string(state.StatusNotStarted): {Glyph: "◯", Color: ColorDimWhite},
	string(state.StatusBlocked):    {Glyph: "☢", Color: ColorRed},
}

// AuditStatusGlyphs maps each audit lifecycle status to its display glyph and color.
var AuditStatusGlyphs = map[string]StatusGlyph{
	string(state.AuditPassed):     {Glyph: "⏸", Color: ColorGreen},
	string(state.AuditInProgress): {Glyph: "◐", Color: ColorYellow},
	string(state.AuditPending):    {Glyph: "◯", Color: ColorDimWhite},
	string(state.AuditFailed):     {Glyph: "⊘", Color: ColorRed},
}

// GlyphForStatus returns the styled glyph string for a node status.
func GlyphForStatus(status string) string {
	if sg, ok := NodeStatusGlyphs[status]; ok {
		return lipgloss.NewStyle().Foreground(sg.Color).Render(sg.Glyph)
	}
	return "?"
}

// GlyphForAuditStatus returns the styled glyph string for an audit status.
func GlyphForAuditStatus(status string) string {
	if sg, ok := AuditStatusGlyphs[status]; ok {
		return lipgloss.NewStyle().Foreground(sg.Color).Render(sg.Glyph)
	}
	return "?"
}

// InstanceLabel builds a human-readable label for an instance. Uses the
// worktree directory basename with branch in parens when the branch
// differs from the directory name (e.g., "wc-tui-test (main)").
// Falls back to branch or PID when the worktree is empty.
func InstanceLabel(inst instance.Entry) string {
	dir := filepath.Base(inst.Worktree)
	branch := inst.Branch

	if dir == "" || dir == "." {
		if branch != "" {
			return branch
		}
		return fmt.Sprintf("pid:%d", inst.PID)
	}

	if branch == "" || branch == dir {
		return dir
	}
	return dir + " (" + branch + ")"
}
