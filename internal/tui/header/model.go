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
// HeaderModel
// ---------------------------------------------------------------------------

// HeaderModel is the sub-model for the TUI top bar.
type HeaderModel struct {
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
	spinner         int  // index into spinnerFrames
	loading         bool

	// Instance tab bar (Phase 3)
	instances   []instance.Entry
	activeIndex int
	statusHint  string // transient hint like "Starting daemon..."
}

// NewHeaderModel creates a HeaderModel with sensible zero-state defaults.
func NewHeaderModel(version string) HeaderModel {
	return HeaderModel{
		version:      version,
		daemonStatus: "standing down",
		nodeCounts:   make(map[state.NodeStatus]int),
		auditCounts:  make(map[state.AuditStatus]int),
	}
}

// SetSize updates the available width.
func (m *HeaderModel) SetSize(width int) {
	m.width = width
}

// SetLoading sets the loading spinner state.
func (m *HeaderModel) SetLoading(loading bool) {
	m.loading = loading
}

// SetInstances updates the instance list and active index for the tab bar.
func (m *HeaderModel) SetInstances(entries []instance.Entry, activeIdx int) {
	m.instances = entries
	m.activeIndex = activeIdx
	m.instanceCount = len(entries)
}

// SetStatusHint sets a transient status message (e.g. "Starting daemon...").
// Pass an empty string to clear.
func (m *HeaderModel) SetStatusHint(hint string) {
	m.statusHint = hint
}

// ---------------------------------------------------------------------------
// Update
// ---------------------------------------------------------------------------

// Update processes messages relevant to the header.
func (m HeaderModel) Update(msg tea.Msg) (HeaderModel, tea.Cmd) {
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

// View renders the header bar.
func (m HeaderModel) View() string {
	if m.width <= 0 {
		return ""
	}

	barStyle := lipgloss.NewStyle().
		Background(headerBg).
		Foreground(headerFg)

	boldStyle := barStyle.Bold(true)

	title := boldStyle.Render(fmt.Sprintf("WOLFCASTLE v%s", m.version))

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

	line1 := composeLine(barStyle, title, right1, m.width)

	// Narrow terminals: single line only.
	if m.width < 40 {
		return line1
	}

	// Line 2: node counts left, audit summary right.
	left2 := m.renderNodeCounts(barStyle)
	right2 := m.renderAuditSummary(barStyle)
	line2 := composeLine(barStyle, left2, right2, m.width)

	// Line 3 (optional): instance tab bar when wide enough and multiple instances exist.
	if m.width > 100 && len(m.instances) > 1 {
		tabBar := m.renderTabBar(barStyle, boldStyle)
		return line1 + "\n" + line2 + "\n" + tabBar
	}

	return line1 + "\n" + line2
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
func (m HeaderModel) renderNodeCounts(base lipgloss.Style) string {
	if m.totalNodes == 0 {
		return base.Render("0 nodes")
	}

	parts := []string{base.Render(fmt.Sprintf("%d nodes:", m.totalNodes))}
	for _, s := range statusOrder {
		n := m.nodeCounts[s]
		if n == 0 {
			continue
		}
		glyph := statusGlyph[s]
		colored := lipgloss.NewStyle().
			Background(headerBg).
			Foreground(statusColor[s]).
			Render(glyph)
		parts = append(parts, base.Render(fmt.Sprintf("%d", n))+colored)
	}
	return strings.Join(parts, " ")
}

// renderAuditSummary builds the "Audit: 5 passed, 2 gaps, 1 escalation" string.
func (m HeaderModel) renderAuditSummary(base lipgloss.Style) string {
	passed := m.auditCounts[state.AuditPassed]

	var segments []string
	if passed > 0 {
		segments = append(segments, fmt.Sprintf("%d passed", passed))
	}
	if m.openGaps > 0 {
		segments = append(segments, fmt.Sprintf("%d %s", m.openGaps, pluralize("gap", m.openGaps)))
	}
	if m.openEscalations > 0 {
		segments = append(segments, fmt.Sprintf("%d %s", m.openEscalations, pluralize("escalation", m.openEscalations)))
	}

	if len(segments) == 0 {
		return base.Render("Audit: no data")
	}
	return base.Render("Audit: " + strings.Join(segments, ", "))
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
func (m HeaderModel) renderTabBar(base, bold lipgloss.Style) string {
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
		label := inst.Branch
		if label == "" {
			label = fmt.Sprintf("pid:%d", inst.PID)
		}
		if i == m.activeIndex {
			tabs = append(tabs, activeStyle.Render("["+label+" ")+dotStyle.Render("●")+activeStyle.Render("]"))
		} else {
			tabs = append(tabs, dimStyle.Render("["+label+"]"))
		}
	}

	left := base.Render(strings.Join(tabs, " "))

	// Count running instances on the right.
	running := len(m.instances)
	right := base.Render(fmt.Sprintf("%d running", running))

	return composeLine(base, left, right, m.width)
}

// pluralize appends "s" when count != 1.
func pluralize(word string, count int) string {
	if count == 1 {
		return word
	}
	return word + "s"
}
