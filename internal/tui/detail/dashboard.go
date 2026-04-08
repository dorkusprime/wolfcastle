package detail

import (
	"fmt"
	"image/color"
	"strings"
	"time"

	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"

	"github.com/dorkusprime/wolfcastle/internal/state"
	"github.com/dorkusprime/wolfcastle/internal/tui"
)

const (
	progressBarWidth = 12
	maxActivity      = 10
)

// activityEntry is a single line in the Recent Activity section.
type activityEntry struct {
	timestamp time.Time
	text      string
}

// DashboardModel renders the mission briefing overview: daemon status, node
// progress, recent activity, and audit summary.
type DashboardModel struct {
	daemonStatus    string
	branch          string
	uptime          time.Duration
	daemonRunning   bool
	nodeCounts      map[state.NodeStatus]int
	totalNodes      int
	auditCounts     map[state.AuditStatus]int
	openGaps        int
	openEscalations int
	recentActivity  []activityEntry
	inboxItems      []state.InboxItem
	lastActivity    time.Time
	currentNode     string
	currentTask     string
	width           int
	height          int
}

// NewDashboardModel creates a DashboardModel with sensible zero values.
func NewDashboardModel() DashboardModel {
	return DashboardModel{
		daemonStatus: "standing down",
		nodeCounts:   make(map[state.NodeStatus]int),
		auditCounts:  make(map[state.AuditStatus]int),
	}
}

// Update handles state, daemon, and log messages.
func (m DashboardModel) Update(msg tea.Msg) (DashboardModel, tea.Cmd) {
	switch msg := msg.(type) {
	case tui.StateUpdatedMsg:
		m.recomputeFromIndex(msg.Index)
	case tui.DaemonStatusMsg:
		m.daemonStatus = msg.Status
		m.branch = msg.Branch
		m.daemonRunning = msg.IsRunning
		if !msg.LastActivity.IsZero() {
			m.lastActivity = msg.LastActivity
		}
		m.currentNode = msg.CurrentNode
		m.currentTask = msg.CurrentTask
	case tui.LogLinesMsg:
		for _, s := range msg.Lines {
			m.pushActivity(s)
		}
	}
	return m, nil
}

func (m *DashboardModel) recomputeFromIndex(idx *state.RootIndex) {
	if idx == nil {
		return
	}
	counts := make(map[state.NodeStatus]int)
	auditCounts := make(map[state.AuditStatus]int)
	gaps := 0
	escalations := 0
	total := 0

	for _, entry := range idx.Nodes {
		if entry.Archived {
			continue
		}
		total++
		counts[entry.State]++
	}

	m.nodeCounts = counts
	m.totalNodes = total
	m.auditCounts = auditCounts
	m.openGaps = gaps
	m.openEscalations = escalations
}

func (m *DashboardModel) pushActivity(text string) {
	m.recentActivity = append(m.recentActivity, activityEntry{
		timestamp: time.Now(),
		text:      text,
	})
	if len(m.recentActivity) > maxActivity {
		m.recentActivity = m.recentActivity[len(m.recentActivity)-maxActivity:]
	}
}

// SetInbox updates the dashboard's inbox item list.
func (m *DashboardModel) SetInbox(items []state.InboxItem) {
	m.inboxItems = items
}

// SetDaemonActivity updates the dashboard's last-activity and current
// target/task fields from the daemon activity snapshot.
func (m *DashboardModel) SetDaemonActivity(lastActivity time.Time, currentNode, currentTask string) {
	m.lastActivity = lastActivity
	m.currentNode = currentNode
	m.currentTask = currentTask
}

// SetSize stores the available rendering dimensions.
func (m *DashboardModel) SetSize(width, height int) {
	m.width = width
	m.height = height
}

// View renders the full dashboard content.
func (m DashboardModel) View() string {
	if m.totalNodes == 0 {
		return m.renderEmpty()
	}
	if m.allComplete() {
		return m.renderAllComplete()
	}
	if m.allBlocked() {
		return m.renderAllBlocked()
	}
	return m.renderFull()
}

// --- empty states ---

func (m DashboardModel) renderEmpty() string {
	heading := tui.DashboardHeadingStyle.Render("MISSION BRIEFING")
	body := tui.DashboardBodyStyle.Render("No targets. Feed the inbox.")
	return heading + "\n\n" + body
}

func (m DashboardModel) renderAllComplete() string {
	return m.renderWithBanner("All targets eliminated.")
}

func (m DashboardModel) renderAllBlocked() string {
	return m.renderWithBanner("Blocked on all fronts. Human intervention required.")
}

func (m DashboardModel) renderWithBanner(banner string) string {
	var b strings.Builder
	b.WriteString(tui.DashboardHeadingStyle.Render("MISSION BRIEFING"))
	b.WriteByte('\n')
	b.WriteByte('\n')
	b.WriteString(m.renderStatusBlock())
	b.WriteByte('\n')
	b.WriteByte('\n')
	b.WriteString(tui.DashboardBodyStyle.Render(banner))
	return b.String()
}

// --- full render ---

func (m DashboardModel) renderFull() string {
	var b strings.Builder

	b.WriteString(tui.DashboardHeadingStyle.Render("MISSION BRIEFING"))
	b.WriteByte('\n')
	b.WriteByte('\n')
	b.WriteString(m.renderStatusBlock())
	b.WriteByte('\n')
	b.WriteByte('\n')
	b.WriteString(m.renderProgress())
	b.WriteByte('\n')
	b.WriteByte('\n')
	b.WriteString(m.renderInboxSummary())
	b.WriteByte('\n')
	b.WriteByte('\n')
	b.WriteString(m.renderActivity())
	b.WriteByte('\n')
	b.WriteByte('\n')
	b.WriteString(m.renderAudit())

	return b.String()
}

func (m DashboardModel) renderStatusBlock() string {
	body := tui.DashboardBodyStyle

	var b strings.Builder
	b.WriteString(body.Render(fmt.Sprintf("Status: %s", m.daemonStatus)))
	if m.branch != "" {
		b.WriteByte('\n')
		b.WriteString(body.Render(fmt.Sprintf("Branch: %s", m.branch)))
	}
	if m.daemonRunning && m.uptime > 0 {
		b.WriteByte('\n')
		b.WriteString(body.Render(fmt.Sprintf("Uptime: %s", formatDuration(m.uptime))))
	}
	if !m.lastActivity.IsZero() {
		b.WriteByte('\n')
		b.WriteString(body.Render(fmt.Sprintf("Last activity: %s", relativeTime(m.lastActivity))))
	}
	if m.currentNode != "" {
		current := m.currentNode
		if m.currentTask != "" {
			current += "/" + m.currentTask
		}
		b.WriteByte('\n')
		b.WriteString(body.Render(fmt.Sprintf("Current: %s", current)))
	}
	return b.String()
}

func (m DashboardModel) renderProgress() string {
	body := tui.DashboardBodyStyle
	var b strings.Builder

	b.WriteString(body.Render("Progress:"))

	type row struct {
		status state.NodeStatus
		glyph  string
		color  color.Color
		label  string
		bar    bool
	}
	rows := []row{
		{state.StatusComplete, "●", tui.ColorGreen, "Complete", true},
		{state.StatusInProgress, "◐", tui.ColorYellow, "In progress", true},
		{state.StatusNotStarted, "◯", tui.ColorDimWhite, "Not started", false},
		{state.StatusBlocked, "☢", tui.ColorRed, "Blocked", false},
	}

	for _, r := range rows {
		n := m.nodeCounts[r.status]
		if n == 0 {
			continue
		}
		glyph := lipgloss.NewStyle().Foreground(r.color).Render(r.glyph)
		line := fmt.Sprintf("  %s %-12s %d/%d", glyph, r.label, n, m.totalNodes)
		if r.bar && m.totalNodes > 0 {
			pct := float64(n) / float64(m.totalNodes) * 100
			bar := progressBar(n, m.totalNodes)
			line += fmt.Sprintf("  %s  %.0f%%", bar, pct)
		}
		b.WriteByte('\n')
		b.WriteString(body.Render(line))
	}

	return b.String()
}

func (m DashboardModel) renderActivity() string {
	body := tui.DashboardBodyStyle
	var b strings.Builder

	b.WriteString(body.Render("Recent Activity:"))

	if len(m.recentActivity) == 0 {
		if !m.daemonRunning {
			b.WriteByte('\n')
			b.WriteString(body.Render("  No transmissions. The daemon has not spoken."))
		}
		return b.String()
	}

	for _, entry := range m.recentActivity {
		ts := entry.timestamp.Format("15:04")
		b.WriteByte('\n')
		b.WriteString(body.Render(fmt.Sprintf("  %s  %s", ts, entry.text)))
	}

	return b.String()
}

func (m DashboardModel) renderAudit() string {
	body := tui.DashboardBodyStyle
	var b strings.Builder

	b.WriteString(body.Render("Audit:"))

	passed := m.auditCounts[state.AuditPassed]
	inProg := m.auditCounts[state.AuditInProgress]
	pending := m.auditCounts[state.AuditPending]
	b.WriteByte('\n')
	b.WriteString(body.Render(fmt.Sprintf("  %d passed, %d in progress, %d pending", passed, inProg, pending)))

	b.WriteByte('\n')
	b.WriteString(body.Render(fmt.Sprintf("  %d open gap(s), %d escalation(s)", m.openGaps, m.openEscalations)))

	return b.String()
}

func (m DashboardModel) renderInboxSummary() string {
	body := tui.DashboardBodyStyle
	newCount := 0
	filedCount := 0
	for _, item := range m.inboxItems {
		switch item.Status {
		case state.InboxNew:
			newCount++
		case state.InboxFiled:
			filedCount++
		}
	}
	return body.Render(fmt.Sprintf("Inbox: %d new, %d filed", newCount, filedCount))
}

// --- helpers ---

func (m DashboardModel) allComplete() bool {
	return m.totalNodes > 0 && m.nodeCounts[state.StatusComplete] == m.totalNodes
}

func (m DashboardModel) allBlocked() bool {
	return m.totalNodes > 0 && m.nodeCounts[state.StatusBlocked] == m.totalNodes
}

func progressBar(n, total int) string {
	if total == 0 {
		return strings.Repeat("░", progressBarWidth)
	}
	filled := n * progressBarWidth / total
	if filled > progressBarWidth {
		filled = progressBarWidth
	}
	return strings.Repeat("█", filled) + strings.Repeat("░", progressBarWidth-filled)
}

func formatDuration(d time.Duration) string {
	d = d.Round(time.Second)
	h := int(d.Hours())
	m := int(d.Minutes()) % 60
	s := int(d.Seconds()) % 60
	if h > 0 {
		return fmt.Sprintf("%dh%02dm%02ds", h, m, s)
	}
	if m > 0 {
		return fmt.Sprintf("%dm%02ds", m, s)
	}
	return fmt.Sprintf("%ds", s)
}
