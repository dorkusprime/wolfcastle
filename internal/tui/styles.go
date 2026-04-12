package tui

import (
	"fmt"
	"image/color"
	"path/filepath"

	"charm.land/lipgloss/v2"

	"github.com/dorkusprime/wolfcastle/internal/instance"
	"github.com/dorkusprime/wolfcastle/internal/state"
)

// Color palette constants.
//
// Structural colors carry the brand (neon cyan, magenta, gold, deep blue).
// Status colors (green, yellow, red, dim) are functional and universal.
// See docs/agents/design-system.md for the full rationale.
var (
	// Text
	ColorWhite     = lipgloss.Color("15")  // bright: headings, active content
	ColorDimWhite  = lipgloss.Color("245") // muted: timestamps, hints, footer
	ColorLightGray = lipgloss.Color("252") // normal: body text, tree labels
	ColorDimGray   = lipgloss.Color("240") // faint: debug logs, disabled items

	// Brand
	ColorNeonCyan    = lipgloss.Color("51")  // primary brand: header title, focused border, trace prefix
	ColorDeepCyan    = lipgloss.Color("30")  // dimmed primary: inactive borders, secondary chrome
	ColorMagenta     = lipgloss.Color("198") // accent: search match, active selection
	ColorDeepMagenta = lipgloss.Color("125") // dimmed accent: search ancestor path
	ColorGold        = lipgloss.Color("220") // target: daemon focus, confirm button, target mark

	// Backgrounds
	ColorBaseBg    = lipgloss.Color("234") // full-screen base (near-black, ANSI 256 for Terminal.app compat)
	ColorCharcoal  = lipgloss.Color("234") // header bg, toast bg (barely off-black)
	ColorSelection = lipgloss.Color("23")  // selected row in tree (dark teal)
	ColorOverlayBg = lipgloss.Color("235") // modal overlay fill
	ColorSlate     = lipgloss.Color("236") // neutral dark: dividers, alt-rows
	ColorDarkRed   = lipgloss.Color("52")  // error bar bg only

	// Status (functional, never decorative)
	ColorRed    = lipgloss.Color("1") // blocked, error
	ColorGreen  = lipgloss.Color("2") // complete, passed
	ColorYellow = lipgloss.Color("3") // in progress, pending

	// Legacy alias (kept for grep-ability during migration)
	ColorDarkGray = ColorSlate
)

// Header styles
var (
	HeaderStyle = lipgloss.NewStyle().
			Foreground(ColorWhite).
			Background(ColorCharcoal)

	HeaderBoldStyle = lipgloss.NewStyle().
			Foreground(ColorNeonCyan).
			Background(ColorCharcoal).
			Bold(true)
)

// Tree styles
var (
	TreeSelectedStyle = lipgloss.NewStyle().
				Foreground(ColorWhite).
				Background(ColorSelection).
				Bold(true)

	TreeNormalStyle = lipgloss.NewStyle().
			Foreground(ColorLightGray)

	TreeSearchHighlight = lipgloss.NewStyle().
				Background(ColorMagenta)
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
				Background(ColorBaseBg).
				Border(lipgloss.RoundedBorder()).
				BorderForeground(ColorNeonCyan).
				BorderBackground(ColorBaseBg)

	UnfocusedBorderStyle = lipgloss.NewStyle().
				Background(ColorBaseBg).
				Border(lipgloss.RoundedBorder()).
				BorderForeground(ColorSlate).
				BorderBackground(ColorBaseBg)
)

// Spinner style
var SpinnerStyle = lipgloss.NewStyle().
	Foreground(ColorNeonCyan)

// Overlay styles (shared by help and modals)
var (
	HelpOverlayStyle = lipgloss.NewStyle().
				Background(ColorOverlayBg).
				Border(lipgloss.RoundedBorder()).
				BorderForeground(ColorDeepCyan).
				BorderBackground(ColorOverlayBg)

	HelpTitleStyle = lipgloss.NewStyle().
			Foreground(ColorNeonCyan).
			Bold(true)

	ModalOverlayStyle = lipgloss.NewStyle().
				Background(ColorOverlayBg).
				Border(lipgloss.RoundedBorder()).
				BorderForeground(ColorDeepCyan).
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
				Foreground(ColorGold)
)

// Current target indicator
var TargetIndicatorStyle = lipgloss.NewStyle().
	Foreground(ColorGold).
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
