package detail

import (
	"fmt"
	"strings"
	"time"

	"charm.land/bubbles/v2/viewport"
	tea "charm.land/bubbletea/v2"

	"github.com/dorkusprime/wolfcastle/internal/state"
	"github.com/dorkusprime/wolfcastle/internal/tui"
)

// NodeModel renders detailed information about a single node, including
// scope, success criteria, children or tasks, audit state, and specs.
type NodeModel struct {
	addr     string
	node     *state.NodeState
	index    *state.IndexEntry
	viewport viewport.Model
	width    int
	height   int
	readErr  bool
	isTarget bool
}

// NewNodeModel creates a NodeModel with an empty viewport.
func NewNodeModel() NodeModel {
	return NodeModel{
		viewport: viewport.New(),
	}
}

// Load populates the model with node data and rebuilds the viewport content.
func (m *NodeModel) Load(addr string, node *state.NodeState, entry *state.IndexEntry, isTarget bool) {
	m.addr = addr
	m.node = node
	m.index = entry
	m.isTarget = isTarget
	m.readErr = false
	m.rebuildContent()
}

// SetSize stores the available rendering area and propagates to the viewport.
func (m *NodeModel) SetSize(width, height int) {
	m.width = width
	m.height = height
	m.viewport.SetWidth(width)
	m.viewport.SetHeight(height)
	m.rebuildContent()
}

// SetReadError marks the model as unable to read node state.
func (m *NodeModel) SetReadError(addr string) {
	m.addr = addr
	m.node = nil
	m.index = nil
	m.readErr = true
	m.viewport.SetContent(
		tui.DashboardBodyStyle.Render(
			fmt.Sprintf("Intelligence unavailable for %s. Run wolfcastle doctor.", addr),
		),
	)
}

// Addr returns the node address, suitable for clipboard copy.
func (m NodeModel) Addr() string {
	return m.addr
}

// Update forwards key events to the viewport for scrolling.
func (m NodeModel) Update(msg tea.Msg) (NodeModel, tea.Cmd) {
	switch msg.(type) {
	case tea.KeyPressMsg:
		var cmd tea.Cmd
		m.viewport, cmd = m.viewport.Update(msg)
		return m, cmd
	}
	return m, nil
}

// View renders the viewport.
func (m NodeModel) View() string {
	return m.viewport.View()
}

func (m *NodeModel) rebuildContent() {
	if m.readErr || m.node == nil {
		return
	}

	heading := tui.DashboardHeadingStyle
	body := tui.DashboardBodyStyle
	n := m.node
	wrapWidth := m.width - 4
	if wrapWidth < 20 {
		wrapWidth = 76
	}

	var b strings.Builder

	// Title line: {▶ if target}{name}  {status_glyph} {status}
	if m.isTarget {
		b.WriteString(tui.TargetIndicatorStyle.Render("▶ "))
	}
	b.WriteString(heading.Render(n.Name))
	b.WriteString("  ")
	b.WriteString(tui.GlyphForStatus(string(n.State)))
	b.WriteString(" ")
	b.WriteString(body.Render(string(n.State)))
	b.WriteByte('\n')

	// Type
	b.WriteString(body.Render(fmt.Sprintf("Type: %s", n.Type)))
	b.WriteByte('\n')

	// Scope
	if n.Scope != "" {
		b.WriteByte('\n')
		b.WriteString(heading.Render("Scope:"))
		b.WriteByte('\n')
		b.WriteString(body.Render(wrapIndent(n.Scope, wrapWidth, "  ")))
		b.WriteByte('\n')
	}

	// Success Criteria
	if len(n.SuccessCriteria) > 0 {
		b.WriteByte('\n')
		b.WriteString(heading.Render("Success Criteria:"))
		b.WriteByte('\n')
		for _, c := range n.SuccessCriteria {
			b.WriteString(body.Render(wrapBullet(c, wrapWidth)))
			b.WriteByte('\n')
		}
	}

	// Children (orchestrator only)
	if n.Type == state.NodeOrchestrator && len(n.Children) > 0 {
		b.WriteByte('\n')
		b.WriteString(heading.Render("Children:"))
		b.WriteByte('\n')
		for _, child := range n.Children {
			glyph := tui.GlyphForStatus(string(child.State))
			b.WriteString(body.Render(fmt.Sprintf("  %s %s", glyph, child.ID)))
			b.WriteByte('\n')
		}
	}

	// Tasks (leaf only)
	if n.Type == state.NodeLeaf && len(n.Tasks) > 0 {
		b.WriteByte('\n')
		b.WriteString(heading.Render("Tasks:"))
		b.WriteByte('\n')
		for _, t := range n.Tasks {
			glyph := tui.GlyphForStatus(string(t.State))
			title := t.Title
			if title == "" {
				title = t.Description
			}
			header := fmt.Sprintf("  %s %s:", glyph, t.ID)
			b.WriteString(body.Render(header))
			b.WriteByte('\n')
			b.WriteString(body.Render(wrapIndent(title, wrapWidth, "    ")))
			b.WriteByte('\n')
		}
	}

	// Audit
	audit := n.Audit
	if audit.Status != "" {
		b.WriteByte('\n')
		auditGlyph := tui.GlyphForAuditStatus(string(audit.Status))
		b.WriteString(heading.Render("Audit: "))
		b.WriteString(auditGlyph)
		b.WriteString(" ")
		b.WriteString(body.Render(string(audit.Status)))
		b.WriteByte('\n')

		if audit.StartedAt != nil {
			b.WriteString(body.Render("  Started: " + relativeTime(*audit.StartedAt)))
			b.WriteByte('\n')
		}
		if audit.CompletedAt != nil {
			b.WriteString(body.Render("  Completed: " + relativeTime(*audit.CompletedAt)))
			b.WriteByte('\n')
		} else if audit.Status == state.AuditInProgress {
			b.WriteString(body.Render("  Completed: in progress"))
			b.WriteByte('\n')
		}

		if len(audit.Breadcrumbs) > 0 {
			b.WriteString(body.Render(fmt.Sprintf("  Breadcrumbs: %d", len(audit.Breadcrumbs))))
			b.WriteByte('\n')
			for _, bc := range audit.Breadcrumbs {
				header := fmt.Sprintf("    [%s] %s:", bc.Timestamp.Local().Format("15:04:05"), bc.Task)
				b.WriteString(body.Render(header))
				b.WriteByte('\n')
				b.WriteString(body.Render(wrapIndent(bc.Text, wrapWidth, "      ")))
				b.WriteByte('\n')
			}
		}

		openGaps, fixedGaps := countGaps(audit.Gaps)
		b.WriteString(body.Render(fmt.Sprintf("  Gaps: %d open, %d fixed", openGaps, fixedGaps)))
		b.WriteByte('\n')
		for _, g := range audit.Gaps {
			statusGlyph := "◯"
			if g.Status == state.GapFixed {
				statusGlyph = "●"
			}
			header := fmt.Sprintf("    %s [%s] %s:", statusGlyph, g.Timestamp.Local().Format("15:04:05"), g.ID)
			b.WriteString(body.Render(header))
			b.WriteByte('\n')
			b.WriteString(body.Render(wrapIndent(g.Description, wrapWidth, "      ")))
			b.WriteByte('\n')
			if g.Source != "" {
				b.WriteString(body.Render("      source: " + g.Source))
				b.WriteByte('\n')
			}
			if g.RemediationTaskID != "" {
				b.WriteString(body.Render("      remediation: " + g.RemediationTaskID))
				b.WriteByte('\n')
			}
		}

		openEsc := countOpenEscalations(audit.Escalations)
		b.WriteString(body.Render(fmt.Sprintf("  Escalations: %d open", openEsc)))
		b.WriteByte('\n')
		for _, e := range audit.Escalations {
			header := fmt.Sprintf("    [%s] %s (%s):", e.Timestamp.Local().Format("15:04:05"), e.ID, e.Status)
			b.WriteString(body.Render(header))
			b.WriteByte('\n')
			b.WriteString(body.Render(wrapIndent(e.Description, wrapWidth, "      ")))
			b.WriteByte('\n')
			if e.SourceNode != "" {
				b.WriteString(body.Render("      source: " + e.SourceNode))
				b.WriteByte('\n')
			}
		}

		b.WriteString(body.Render("  Summary:"))
		b.WriteByte('\n')
		summary := "none yet"
		if audit.ResultSummary != "" {
			summary = audit.ResultSummary
		}
		b.WriteString(body.Render(wrapIndent(summary, wrapWidth, "    ")))
		b.WriteByte('\n')
	}

	// Specs
	if len(n.Specs) > 0 {
		b.WriteByte('\n')
		b.WriteString(heading.Render("Specs:"))
		b.WriteByte('\n')
		for _, sp := range n.Specs {
			b.WriteString(body.Render("  " + sp))
			b.WriteByte('\n')
		}
	}

	m.viewport.SetContent(b.String())
}

// SearchContent returns the viewport content split into lines for search.
func (m NodeModel) SearchContent() []string {
	return strings.Split(m.viewport.GetContent(), "\n")
}

// relativeTime formats a timestamp as a human-friendly relative string.
// For times more than an hour ago, the exact clock time is appended.
func relativeTime(t time.Time) string {
	d := time.Since(t)
	if d < 0 {
		d = 0
	}

	switch {
	case d < time.Minute:
		s := int(d.Seconds())
		if s <= 1 {
			return "just now"
		}
		return fmt.Sprintf("%ds ago", s)
	case d < time.Hour:
		m := int(d.Minutes())
		if m == 1 {
			return "1m ago"
		}
		return fmt.Sprintf("%dm ago", m)
	case d < 24*time.Hour:
		h := int(d.Hours())
		exact := t.Local().Format("15:04:05")
		if h == 1 {
			return fmt.Sprintf("1h ago (%s)", exact)
		}
		return fmt.Sprintf("%dh ago (%s)", h, exact)
	default:
		days := int(d.Hours()) / 24
		exact := t.Local().Format("15:04:05")
		if days == 1 {
			return fmt.Sprintf("1d ago (%s)", exact)
		}
		return fmt.Sprintf("%dd ago (%s)", days, exact)
	}
}

func countGaps(gaps []state.Gap) (open, fixed int) {
	for _, g := range gaps {
		switch g.Status {
		case state.GapOpen:
			open++
		case state.GapFixed:
			fixed++
		}
	}
	return
}

func countOpenEscalations(escs []state.Escalation) int {
	n := 0
	for _, e := range escs {
		if e.Status == state.EscalationOpen {
			n++
		}
	}
	return n
}

// wrapBullet renders a bullet item with hanging indent: the first line
// starts with "  • " and continuation lines align with "    " so the
// bullet glyph isn't repeated.
func wrapBullet(text string, width int) string {
	const firstPrefix = "  \u2022 "
	const contPrefix = "    "
	if width < 20 {
		width = 20
	}

	words := strings.Fields(text)
	if len(words) == 0 {
		return firstPrefix
	}

	var lines []string
	line := firstPrefix + words[0]
	lineLen := len(firstPrefix) + len(words[0])

	for _, w := range words[1:] {
		if lineLen+1+len(w) > width {
			lines = append(lines, line)
			line = contPrefix + w
			lineLen = len(contPrefix) + len(w)
		} else {
			line += " " + w
			lineLen += 1 + len(w)
		}
	}
	lines = append(lines, line)
	return strings.Join(lines, "\n")
}

// wrapIndent performs simple word-wrapping with an indent prefix on each line.
func wrapIndent(text string, width int, indent string) string {
	if width < 20+len(indent) {
		width = 20 + len(indent)
	}

	words := strings.Fields(text)
	if len(words) == 0 {
		return ""
	}

	var lines []string
	line := indent + words[0]
	lineLen := len(indent) + len(words[0])

	for _, w := range words[1:] {
		if lineLen+1+len(w) > width {
			lines = append(lines, line)
			line = indent + w
			lineLen = len(indent) + len(w)
		} else {
			line += " " + w
			lineLen += 1 + len(w)
		}
	}
	lines = append(lines, line)
	return strings.Join(lines, "\n")
}
