// Package header provides the top-bar sub-model for the Wolfcastle TUI.
// It renders version info, daemon status, node counts, and audit summaries.
package header

import (
	"fmt"
	"image/color"
	"strings"

	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"

	"github.com/dorkusprime/wolfcastle/internal/instance"
	"github.com/dorkusprime/wolfcastle/internal/state"
	"github.com/dorkusprime/wolfcastle/internal/tui"
)

// ---------------------------------------------------------------------------
// Spinner frames
// ---------------------------------------------------------------------------

var spinnerFrames = []rune{'⠋', '⠙', '⠹', '⠸', '⠼', '⠴', '⠦', '⠧', '⠇', '⠏'}

// ---------------------------------------------------------------------------
// Temporary message types (will move to internal/tui when that package exists)
// ---------------------------------------------------------------------------

// StateUpdatedMsg carries a refreshed root index.
type StateUpdatedMsg struct{ Index *state.RootIndex }

// DaemonStatusMsg carries daemon health information.
type DaemonStatusMsg struct {
	Status     string
	Branch     string
	Worktree   string
	PID        int
	IsRunning  bool
	IsDraining bool
	Instances  []instance.Entry
}

// InstancesUpdatedMsg carries the current set of live instances.
type InstancesUpdatedMsg struct{ Instances []instance.Entry }

// SpinnerTickMsg advances the header spinner one frame.
type SpinnerTickMsg struct{}

// ---------------------------------------------------------------------------
// Local style constants (will migrate to internal/tui/styles)
// ---------------------------------------------------------------------------

var (
	headerBg  color.Color = lipgloss.Color("52")  // dark red
	headerFg  color.Color = lipgloss.Color("15")  // white
	clrGreen  color.Color = lipgloss.Color("2")   // ● complete
	clrYellow color.Color = lipgloss.Color("3")   // ◐ in_progress
	clrDim    color.Color = lipgloss.Color("245") // ◯ not_started
	clrRed    color.Color = lipgloss.Color("1")   // ☢ blocked
)

// Status glyphs keyed by NodeStatus.
var statusGlyph = map[state.NodeStatus]string{
	state.StatusComplete:   "●",
	state.StatusInProgress: "◐",
	state.StatusNotStarted: "◯",
	state.StatusBlocked:    "☢",
}

var statusColor = map[state.NodeStatus]color.Color{
	state.StatusComplete:   clrGreen,
	state.StatusInProgress: clrYellow,
	state.StatusNotStarted: clrDim,
	state.StatusBlocked:    clrRed,
}

// Canonical ordering so output is deterministic.
var statusOrder = []state.NodeStatus{
	state.StatusComplete,
	state.StatusInProgress,
	state.StatusNotStarted,
	state.StatusBlocked,
}

// ---------------------------------------------------------------------------
// Model
// ---------------------------------------------------------------------------

// Model is the sub-model for the TUI top bar.
type Model struct {
	version         string
	daemonStatus    string
	branch          string
	instanceCount   int
	nodeCounts      map[state.NodeStatus]int
	totalNodes      int
	auditCounts     map[state.AuditStatus]int
	openGaps        int
	openEscalations int
	width           int
	spinner         int // index into spinnerFrames
	loading         bool

	// Instance tab bar (Phase 3)
	instances   []instance.Entry
	activeIndex int
	statusHint  string // transient hint like "Starting daemon..."
}

// NewModel creates a Model with sensible zero-state defaults.
func NewModel(version string) Model {
	return Model{
		version:      version,
		daemonStatus: "standing down",
		nodeCounts:   make(map[state.NodeStatus]int),
		auditCounts:  make(map[state.AuditStatus]int),
	}
}

// SetSize updates the available width.
func (m *Model) SetSize(width int) {
	m.width = width
}

// SetLoading sets the loading spinner state.
func (m *Model) SetLoading(loading bool) {
	m.loading = loading
}

// IsLoading returns true when the loading spinner is active.
func (m Model) IsLoading() bool {
	return m.loading
}

// SetInstances updates the instance list and active index for the tab bar.
func (m *Model) SetInstances(entries []instance.Entry, activeIdx int) {
	m.instances = entries
	m.activeIndex = activeIdx
	m.instanceCount = len(entries)
}

// SetStatusHint sets a transient status message (e.g. "Starting daemon...").
// Pass an empty string to clear.
func (m *Model) SetStatusHint(hint string) {
	m.statusHint = hint
}

// ---------------------------------------------------------------------------
// Update
// ---------------------------------------------------------------------------

// Update processes messages relevant to the header.
func (m Model) Update(msg tea.Msg) (Model, tea.Cmd) {
	switch msg := msg.(type) {

	case DaemonStatusMsg:
		m.branch = msg.Branch
		m.instanceCount = len(msg.Instances)
		m.daemonStatus = daemonStatusString(msg)

	case StateUpdatedMsg:
		m.nodeCounts = make(map[state.NodeStatus]int)
		m.totalNodes = 0
		m.auditCounts = make(map[state.AuditStatus]int)
		m.openGaps = 0
		m.openEscalations = 0
		if msg.Index != nil {
			for _, entry := range msg.Index.Nodes {
				if entry.Archived {
					continue
				}
				m.nodeCounts[entry.State]++
				m.totalNodes++
			}
		}

	case InstancesUpdatedMsg:
		m.instanceCount = len(msg.Instances)

	case SpinnerTickMsg:
		m.spinner = (m.spinner + 1) % len(spinnerFrames)

	case tea.WindowSizeMsg:
		m.width = msg.Width
	}

	return m, nil
}

// ---------------------------------------------------------------------------
// View
// ---------------------------------------------------------------------------

// View renders the header bar with top/bottom and left/right padding.
func (m Model) View() string {
	if m.width <= 0 {
		return ""
	}

	barStyle := lipgloss.NewStyle().
		Background(headerBg).
		Foreground(headerFg)

	boldStyle := barStyle.Bold(true)

	// 2 cells of horizontal padding on each side keep text from touching
	// the terminal edge. The compose width shrinks accordingly.
	const hPad = 2
	innerWidth := m.width - hPad*2
	if innerWidth < 1 {
		innerWidth = 1
	}

	version := strings.TrimPrefix(m.version, "v")
	title := boldStyle.Render(fmt.Sprintf("WOLFCASTLE v%s", version))

	// Build right side of line 1: optional spinner, daemon status, instance badge.
	rightParts := []string{}
	if m.loading {
		rightParts = append(rightParts, string(spinnerFrames[m.spinner]))
	}
	if m.statusHint != "" {
		rightParts = append(rightParts, m.statusHint)
	} else {
		rightParts = append(rightParts, m.daemonStatus)
	}
	if m.instanceCount > 1 {
		rightParts = append(rightParts, fmt.Sprintf("[%d running]", m.instanceCount))
	}
	right1 := barStyle.Render(strings.Join(rightParts, " "))

	line1 := composeLine(barStyle, title, right1, innerWidth)

	// Pad each line with hPad cells of background on each side.
	pad := barStyle.Render(strings.Repeat(" ", hPad))

	wrap := func(line string) string {
		return pad + line + pad
	}

	// Narrow terminals: single line only.
	if m.width < 40 {
		return wrap(line1)
	}

	// Line 2: node counts left, blank right (audit summary removed
	// until daemon-side aggregation is wired up).
	left2 := m.renderNodeCounts(barStyle)
	line2 := composeLine(barStyle, left2, "", innerWidth)

	// Line 3 (optional): instance tab bar when wide enough and multiple instances exist.
	if m.width > 100 && len(m.instances) > 1 {
		tabBar := m.renderTabBar(barStyle, boldStyle, innerWidth)
		return wrap(line1) + "\n" + wrap(line2) + "\n" + wrap(tabBar)
	}

	return wrap(line1) + "\n" + wrap(line2)
}

// ---------------------------------------------------------------------------
// Internal helpers
// ---------------------------------------------------------------------------

// daemonStatusString computes the human-readable daemon status label.
// Format: "{worktree} ({branch}) hunting (PID 12345)" or just the
// status when worktree/branch are empty.
func daemonStatusString(msg DaemonStatusMsg) string {
	if msg.Status == "" {
		return "status unknown"
	}

	var state string
	switch {
	case msg.IsRunning && !msg.IsDraining:
		state = fmt.Sprintf("hunting (PID %d)", msg.PID)
	case msg.IsRunning && msg.IsDraining:
		state = fmt.Sprintf("draining (PID %d)", msg.PID)
	case !msg.IsRunning && msg.PID > 0:
		state = fmt.Sprintf("presumed dead (stale PID %d)", msg.PID)
	default:
		state = "standing down"
	}

	// Prefix with worktree path and branch when available.
	if msg.Worktree != "" {
		prefix := msg.Worktree
		if msg.Branch != "" {
			prefix += " (" + msg.Branch + ")"
		}
		return prefix + " " + state
	}

	return state
}

// renderNodeCounts builds the "12 nodes: 4● 3◐ 3◯ 2☢" string.
func (m Model) renderNodeCounts(base lipgloss.Style) string {
	if m.totalNodes == 0 {
		return base.Render("0 nodes")
	}

	parts := []string{base.Render(fmt.Sprintf("%d nodes:", m.totalNodes))}
	for _, s := range statusOrder {
		n := m.nodeCounts[s]
		if n == 0 {
			continue
		}
		// Style the glyph with the bar background AND the status foreground
		// so the rendered fragment carries both attributes. Otherwise the
		// reset code at the end of the glyph clears the background and the
		// next space renders with the terminal default.
		glyph := lipgloss.NewStyle().
			Background(headerBg).
			Foreground(statusColor[s]).
			Render(statusGlyph[s])
		parts = append(parts, base.Render(fmt.Sprintf("%d", n))+glyph)
	}
	return strings.Join(parts, base.Render(" "))
}

// composeLine pads the gap between left and right content to fill width,
// all painted with the bar background.
func composeLine(base lipgloss.Style, left, right string, width int) string {
	leftW := lipgloss.Width(left)
	rightW := lipgloss.Width(right)
	gap := width - leftW - rightW
	if gap < 1 {
		gap = 1
	}
	filler := base.Render(strings.Repeat(" ", gap))
	return left + filler + right
}

// renderTabBar builds the instance tab bar: [feat/auth ●] [fix/login]
func (m Model) renderTabBar(base, bold lipgloss.Style, width int) string {
	dimStyle := lipgloss.NewStyle().
		Background(headerBg).
		Foreground(clrDim)

	activeStyle := lipgloss.NewStyle().
		Background(headerBg).
		Foreground(headerFg).
		Bold(true)

	dotStyle := lipgloss.NewStyle().
		Background(headerBg).
		Foreground(clrGreen)

	var tabs []string
	for i, inst := range m.instances {
		label := instanceLabel(inst)
		if i == m.activeIndex {
			tabs = append(tabs, activeStyle.Render("["+label+" ")+dotStyle.Render("●")+activeStyle.Render("]"))
		} else {
			tabs = append(tabs, dimStyle.Render("["+label+"]"))
		}
	}

	// Join tabs with a styled space so the separator carries the bar background.
	left := strings.Join(tabs, base.Render(" "))

	// Count running instances on the right.
	running := len(m.instances)
	right := base.Render(fmt.Sprintf("%d running", running))

	return composeLine(base, left, right, width)
}

// instanceLabel delegates to the shared tui.InstanceLabel.
func instanceLabel(inst instance.Entry) string {
	return tui.InstanceLabel(inst)
}

// pluralize appends "s" when count != 1.
func pluralize(word string, count int) string {
	if count == 1 {
		return word
	}
	return word + "s"
}
