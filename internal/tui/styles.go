package tui

import (
	"image/color"

	"charm.land/lipgloss/v2"
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

// Help overlay styles
var (
	HelpOverlayStyle = lipgloss.NewStyle().
				Background(ColorOverlayBg).
				Border(lipgloss.RoundedBorder()).
				BorderForeground(ColorDimWhite)

	HelpTitleStyle = lipgloss.NewStyle().
			Foreground(ColorWhite).
			Bold(true)
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

var NodeStatusGlyphs = map[string]StatusGlyph{
	"complete":    {Glyph: "●", Color: ColorGreen},
	"in_progress": {Glyph: "◐", Color: ColorYellow},
	"not_started": {Glyph: "◯", Color: ColorDimWhite},
	"blocked":     {Glyph: "☢", Color: ColorRed},
}

var AuditStatusGlyphs = map[string]StatusGlyph{
	"passed":      {Glyph: "⏸", Color: ColorGreen},
	"in_progress": {Glyph: "◐", Color: ColorYellow},
	"pending":     {Glyph: "◯", Color: ColorDimWhite},
	"failed":      {Glyph: "⊘", Color: ColorRed},
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
