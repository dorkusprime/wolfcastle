package detail

import (
	"bufio"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"

	"charm.land/bubbles/v2/viewport"
	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"

	"github.com/dorkusprime/wolfcastle/internal/logrender"
	"github.com/dorkusprime/wolfcastle/internal/tui"
)

const maxLogLines = 10000

type logLine struct {
	record   logrender.Record
	rendered string // cached rendered string
	rawJSON  string // original JSON line for clipboard
}

// LogViewModel displays a scrollable, filterable stream of daemon log lines
// inside the detail pane. It maintains a circular buffer of parsed records
// and renders them through the viewport with optional level and trace filters.
type LogViewModel struct {
	lines       []logLine
	viewport    viewport.Model
	follow      bool   // auto-scroll to bottom when new lines arrive
	levelFilter string // "all", "debug", "info", "warn", "error"
	traceFilter string // "all", "exec", "intake"
	logFile     string
	iteration   int // current iteration number, parsed from log filename
	width       int
	height      int
	focused     bool
	readErr     bool // true when log file read failed
}

// NewLogViewModel creates a LogViewModel with follow enabled and all filters
// passing everything through.
func NewLogViewModel() LogViewModel {
	vp := viewport.New()
	return LogViewModel{
		viewport:    vp,
		follow:      true,
		levelFilter: "all",
		traceFilter: "all",
	}
}

// Update handles keyboard input and incoming log data.
func (m LogViewModel) Update(msg tea.Msg) (LogViewModel, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.KeyPressMsg:
		return m.handleKey(msg)
	case tui.LogLinesMsg:
		m.AppendLines(msg.Lines)
		if m.follow {
			m.viewport.GotoBottom()
		}
		return m, nil
	case tui.NewLogFileMsg:
		m.logFile = msg.Path
		m.iteration = parseIterationFromPath(msg.Path)
		// Insert a visual separator so the operator can see where one file
		// ends and the next begins.
		label := "new log file"
		if m.iteration > 0 {
			label = fmt.Sprintf("iteration %d", m.iteration)
		}
		sep := logLine{
			rendered: lipgloss.NewStyle().Foreground(tui.ColorDimWhite).Render(
				fmt.Sprintf("── %s ──", label),
			),
		}
		m.lines = append(m.lines, sep)
		m.trimBuffer()
		m.rebuildViewport()
		if m.follow {
			m.viewport.GotoBottom()
		}
		return m, nil
	}
	return m, nil
}

func (m LogViewModel) handleKey(msg tea.KeyPressMsg) (LogViewModel, tea.Cmd) {
	switch msg.String() {
	case "f":
		m.follow = !m.follow
		if m.follow {
			m.viewport.GotoBottom()
		}
	case "j", "down":
		m.viewport.ScrollDown(1)
		if !m.viewport.AtBottom() {
			m.follow = false
		}
	case "k", "up":
		m.viewport.ScrollUp(1)
		m.follow = false
	case "ctrl+d", "pgdown":
		m.viewport.HalfPageDown()
		if !m.viewport.AtBottom() {
			m.follow = false
		}
	case "ctrl+u", "pgup":
		m.viewport.HalfPageUp()
		m.follow = false
	case "G":
		m.viewport.GotoBottom()
		m.follow = true
	case "g":
		m.viewport.GotoTop()
		m.follow = false
	case "L":
		m.cycleLevelFilter()
		m.rebuildViewport()
		if m.follow {
			m.viewport.GotoBottom()
		}
	case "T":
		m.cycleTraceFilter()
		m.rebuildViewport()
		if m.follow {
			m.viewport.GotoBottom()
		}
	default:
		// Pass unhandled keys through to the viewport for mouse wheel, etc.
		var cmd tea.Cmd
		m.viewport, cmd = m.viewport.Update(msg)
		return m, cmd
	}
	return m, nil
}

// AppendLines parses raw JSON lines, skips malformed ones, and appends them
// to the circular buffer.
func (m *LogViewModel) AppendLines(rawLines []string) {
	for _, raw := range rawLines {
		rec, err := logrender.ParseRecord(raw)
		if err != nil {
			continue
		}
		rendered := m.renderLine(rec)
		if rendered == "" {
			continue
		}
		m.lines = append(m.lines, logLine{
			record:   rec,
			rendered: rendered,
			rawJSON:  raw,
		})
	}
	m.trimBuffer()
	m.rebuildViewport()
}

func (m *LogViewModel) trimBuffer() {
	if len(m.lines) > maxLogLines {
		m.lines = m.lines[len(m.lines)-maxLogLines:]
	}
}

// SetSize propagates dimensions to the viewport, reserving space for the
// header line.
func (m *LogViewModel) SetSize(width, height int) {
	m.width = width
	m.height = height
	m.viewport.SetWidth(width)
	// Reserve one line for the header.
	vpHeight := height - 1
	if vpHeight < 0 {
		vpHeight = 0
	}
	m.viewport.SetHeight(vpHeight)
}

// SetFocused marks whether this view currently holds keyboard focus.
func (m *LogViewModel) SetFocused(focused bool) {
	m.focused = focused
}

// SetReadError records whether the log file could not be read.
func (m *LogViewModel) SetReadError(err bool) {
	m.readErr = err
}

// SelectedLineJSON returns the raw JSON of the line at the current viewport
// cursor position, suitable for clipboard copy.
func (m LogViewModel) SelectedLineJSON() string {
	visible := m.filteredLines()
	idx := m.viewport.YOffset()
	if idx < 0 || idx >= len(visible) {
		return ""
	}
	return visible[idx].rawJSON
}

// rebuildViewport applies the active filters and sets the viewport content
// to the rendered, visible lines.
func (m *LogViewModel) rebuildViewport() {
	visible := m.filteredLines()
	rendered := make([]string, len(visible))
	for i, ll := range visible {
		rendered[i] = ll.rendered
	}
	m.viewport.SetContent(strings.Join(rendered, "\n"))
}

func (m LogViewModel) filteredLines() []logLine {
	if m.levelFilter == "all" && m.traceFilter == "all" {
		return m.lines
	}
	out := make([]logLine, 0, len(m.lines))
	for _, ll := range m.lines {
		if !m.levelMatches(ll.record) {
			continue
		}
		if !m.traceMatches(ll.record) {
			continue
		}
		out = append(out, ll)
	}
	return out
}

// View renders the header, viewport, and any status messages.
func (m LogViewModel) View() string {
	var b strings.Builder

	// Header
	b.WriteString(m.renderHeader())
	b.WriteByte('\n')

	// Error / empty states
	if m.readErr {
		body := tui.DashboardBodyStyle.Render(
			"Unable to read log file. Check that the daemon has been started at least once, or run wolfcastle doctor.",
		)
		b.WriteString(body)
		return b.String()
	}
	if len(m.lines) == 0 {
		body := tui.DashboardBodyStyle.Render(
			"No transmissions. The daemon has not spoken.",
		)
		b.WriteString(body)
		return b.String()
	}

	b.WriteString(m.viewport.View())
	return b.String()
}

func (m LogViewModel) renderHeader() string {
	heading := tui.DashboardHeadingStyle

	levelDisplay := levelFilterDisplay(m.levelFilter)
	followIndicator := m.followIndicator()

	return heading.Render(fmt.Sprintf(
		"TRANSMISSIONS  Level: %s  Trace: %s  %s",
		levelDisplay, m.traceFilter, followIndicator,
	))
}

func levelFilterDisplay(f string) string {
	switch f {
	case "all":
		return "all (unfiltered)"
	case "debug":
		return "DEBUG and above"
	case "info":
		return "INFO and above"
	case "warn":
		return "WARN and above"
	case "error":
		return "ERROR only"
	default:
		return f
	}
}

func (m LogViewModel) followIndicator() string {
	if m.follow {
		return lipgloss.NewStyle().Foreground(tui.ColorGreen).Render("[following]")
	}
	return lipgloss.NewStyle().Foreground(tui.ColorYellow).Render("[paused]")
}

// renderLine produces a single styled line from a parsed record. Returns the
// empty string when the record has no human-readable content (e.g., an
// assistant envelope that contains only tool-use plumbing); callers should
// skip empty results rather than emit blank lines.
func (m LogViewModel) renderLine(rec logrender.Record) string {
	content := renderContent(rec)
	if content == "" {
		return ""
	}

	var b strings.Builder

	// Timestamp
	ts := rec.Timestamp.Local().Format("15:04:05")
	b.WriteString(lipgloss.NewStyle().Foreground(tui.ColorDimWhite).Render(ts))
	b.WriteByte(' ')

	// Trace prefix
	if rec.Trace != "" {
		b.WriteString(lipgloss.NewStyle().Foreground(tui.ColorNeonCyan).Render(
			fmt.Sprintf("[%s]", rec.Trace),
		))
		b.WriteByte(' ')
	}

	b.WriteString(content)

	// Apply level-based tint to the whole line
	return applyLevelTint(rec.Level, b.String())
}

func renderContent(rec logrender.Record) string {
	nodeTask := rec.Node
	if rec.Task != "" {
		nodeTask += "/" + rec.Task
	}

	switch rec.Type {
	case "stage_start":
		return lipgloss.NewStyle().Foreground(tui.ColorWhite).Render(
			fmt.Sprintf("[%s] Starting %s", rec.Stage, nodeTask),
		)
	case "stage_complete":
		exitStr := "?"
		clr := tui.ColorGreen
		if rec.ExitCode != nil {
			exitStr = fmt.Sprintf("%d", *rec.ExitCode)
			if *rec.ExitCode != 0 {
				clr = tui.ColorYellow
			}
		}
		return lipgloss.NewStyle().Foreground(clr).Render(
			fmt.Sprintf("[%s] Complete (exit=%s)", rec.Stage, exitStr),
		)
	case "stage_error":
		return lipgloss.NewStyle().Foreground(tui.ColorRed).Render(
			fmt.Sprintf("[%s] Error: %s", rec.Stage, rec.Error),
		)
	case "assistant":
		text := extractAssistantContent(rec.Text)
		if text == "" {
			return ""
		}
		return lipgloss.NewStyle().Foreground(tui.ColorWhite).Render(text)
	case "failure_increment":
		return lipgloss.NewStyle().Foreground(tui.ColorYellow).Render(
			fmt.Sprintf("[failure] %s failure #%d", nodeTask, rec.Counter),
		)
	case "auto_block":
		return lipgloss.NewStyle().Foreground(tui.ColorRed).Bold(true).Render(
			fmt.Sprintf("[blocked] %s: %s", nodeTask, rec.Reason),
		)
	case "daemon_start":
		return lipgloss.NewStyle().Foreground(tui.ColorWhite).Bold(true).Render(
			"Daemon started",
		)
	case "daemon_lifecycle":
		return lipgloss.NewStyle().Foreground(tui.ColorDimWhite).Render(
			fmt.Sprintf("[lifecycle] %s", rec.Event),
		)
	default:
		// Unknown record type. Render a compact tag rather than dumping the
		// raw JSON envelope, which would otherwise flood the viewport.
		if rec.Type == "" {
			return ""
		}
		return lipgloss.NewStyle().Foreground(tui.ColorDimWhite).Render(
			fmt.Sprintf("[%s]", rec.Type),
		)
	}
}

// extractAssistantContent pulls a one-line summary from a Claude API JSON
// envelope embedded in an assistant record's `text` field. It joins text
// content blocks, abbreviates thinking blocks, and tags tool_use blocks by
// name. Plain (non-JSON) input passes through unchanged. Returns the empty
// string only when there is genuinely nothing human-readable to show.
func extractAssistantContent(raw string) string {
	if raw == "" {
		return ""
	}
	var envelope struct {
		Type    string `json:"type"`
		Subtype string `json:"subtype"`
		Message struct {
			Content []struct {
				Type     string          `json:"type"`
				Text     string          `json:"text"`
				Thinking string          `json:"thinking"`
				Name     string          `json:"name"`
				Input    json.RawMessage `json:"input"`
				Content  json.RawMessage `json:"content"`
			} `json:"content"`
		} `json:"message"`
	}
	if err := json.Unmarshal([]byte(raw), &envelope); err != nil {
		// Not JSON; treat as plain text and pass through (truncated).
		return truncate(raw, 240)
	}

	// System frames (init, etc.) carry no operator-facing content.
	if envelope.Type == "system" {
		return ""
	}

	var parts []string
	for _, c := range envelope.Message.Content {
		switch c.Type {
		case "text":
			if c.Text != "" {
				parts = append(parts, truncate(c.Text, 240))
			}
		case "thinking":
			if c.Thinking != "" {
				parts = append(parts, "[thinking] "+truncate(c.Thinking, 200))
			}
		case "tool_use":
			if c.Name != "" {
				parts = append(parts, formatToolUse(c.Name, c.Input))
			}
		case "tool_result":
			parts = append(parts, formatToolResult(c.Content))
		}
	}
	return strings.Join(parts, " | ")
}

// formatToolUse renders a tool_use block as `[tool: Name] arg-summary`.
// The arg summary picks whichever of the well-known fields (command,
// file_path, path, url, pattern, description) the tool happened to
// populate, so Bash, Read, Edit, Grep, and WebFetch all produce a line
// with useful context instead of a bare tool name. Unknown tools fall
// back to the compact JSON of their input map.
func formatToolUse(name string, input json.RawMessage) string {
	label := "[tool: " + name + "]"
	if len(input) == 0 || string(input) == "null" || string(input) == "{}" {
		return label
	}
	var args map[string]any
	if err := json.Unmarshal(input, &args); err != nil {
		return label
	}
	for _, key := range []string{"command", "file_path", "path", "url", "pattern", "query", "description"} {
		if v, ok := args[key].(string); ok && v != "" {
			return label + " " + truncate(v, 200)
		}
	}
	compact, err := json.Marshal(args)
	if err != nil {
		return label
	}
	return label + " " + truncate(string(compact), 200)
}

// formatToolResult renders a tool_result block. The Claude envelope
// ships result content as either a plain string or an array of typed
// blocks ([{type:"text", text:"..."}, ...]). We accept both shapes and
// fall back to a bare marker when the result is empty or unreadable.
func formatToolResult(content json.RawMessage) string {
	label := "[tool result]"
	if len(content) == 0 || string(content) == "null" {
		return label
	}
	var text string
	if err := json.Unmarshal(content, &text); err == nil {
		if text == "" {
			return label
		}
		return label + " " + truncate(text, 240)
	}
	var blocks []struct {
		Type string `json:"type"`
		Text string `json:"text"`
	}
	if err := json.Unmarshal(content, &blocks); err == nil {
		var parts []string
		for _, b := range blocks {
			if b.Type == "text" && b.Text != "" {
				parts = append(parts, b.Text)
			}
		}
		if len(parts) == 0 {
			return label
		}
		return label + " " + truncate(strings.Join(parts, " "), 240)
	}
	return label
}

func truncate(s string, max int) string {
	s = strings.ReplaceAll(s, "\n", " ")
	if len(s) <= max {
		return s
	}
	return s[:max] + "…"
}

func applyLevelTint(level, s string) string {
	switch level {
	case "debug":
		return lipgloss.NewStyle().Foreground(tui.ColorDimGray).Render(s)
	case "warn":
		return lipgloss.NewStyle().Foreground(tui.ColorYellow).Render(s)
	case "error":
		return lipgloss.NewStyle().Foreground(tui.ColorRed).Render(s)
	default:
		// "info" and anything unrecognized pass through untinted.
		return s
	}
}

// levelMatches returns true if the record passes the current level filter.
func (m LogViewModel) levelMatches(rec logrender.Record) bool {
	if m.levelFilter == "all" {
		return true
	}
	return levelOrd(rec.Level) >= levelOrd(m.levelFilter)
}

// traceMatches returns true if the record passes the current trace filter.
// Trace IDs look like "exec-0002", "intake-10001", or "inbox-init-10003".
// The filter matches on the trace category, not the full ID.
func (m LogViewModel) traceMatches(rec logrender.Record) bool {
	if m.traceFilter == "all" {
		return true
	}
	cat := traceCategory(rec.Trace)
	return cat == m.traceFilter
}

// traceCategory extracts the category from a trace ID. "exec-0002" yields
// "exec", "inbox-init-10003" yields "inbox", "intake-10001" yields "inbox"
// (intake is the inbox processing stage).
func traceCategory(trace string) string {
	switch {
	case strings.HasPrefix(trace, "exec"):
		return "exec"
	case strings.HasPrefix(trace, "worker"):
		return "worker"
	case strings.HasPrefix(trace, "plan"):
		return "plan"
	case strings.HasPrefix(trace, "intake"), strings.HasPrefix(trace, "inbox"):
		return "inbox"
	case strings.HasPrefix(trace, "heal"):
		return "system"
	case strings.HasPrefix(trace, "shutdown"), strings.HasPrefix(trace, "crash"):
		return "system"
	default:
		return "other"
	}
}

func levelOrd(level string) int {
	switch level {
	case "debug":
		return 0
	case "info":
		return 1
	case "warn":
		return 2
	case "error":
		return 3
	default:
		return 1 // treat unknown as info
	}
}

var levelCycle = []string{"all", "debug", "info", "warn", "error"}

func (m *LogViewModel) cycleLevelFilter() {
	for i, v := range levelCycle {
		if v == m.levelFilter {
			m.levelFilter = levelCycle[(i+1)%len(levelCycle)]
			return
		}
	}
	m.levelFilter = "all"
}

var traceCycle = []string{"all", "exec", "worker", "plan", "inbox", "system"}

func (m *LogViewModel) cycleTraceFilter() {
	for i, v := range traceCycle {
		if v == m.traceFilter {
			m.traceFilter = traceCycle[(i+1)%len(traceCycle)]
			return
		}
	}
	m.traceFilter = "all"
}

// parseIterationFromPath extracts the iteration number from a log filename.
// Log filenames follow the pattern "{NNNN}-{prefix}-{timestamp}.jsonl".
func parseIterationFromPath(path string) int {
	base := filepath.Base(path)
	if idx := strings.IndexByte(base, '-'); idx > 0 {
		if n, err := strconv.Atoi(base[:idx]); err == nil {
			return n
		}
	}
	return 0
}

// LoadFromFile reads a log file and loads the last N lines into the buffer.
// It parses each line with logrender.ParseRecord, skipping malformed lines,
// then sets follow=true and scrolls to the bottom.
func (m *LogViewModel) LoadFromFile(path string, lastN int) error {
	f, err := os.Open(path)
	if err != nil {
		return err
	}
	defer func() { _ = f.Close() }()

	// Read all lines into a ring buffer of size lastN.
	var ring []string
	scanner := bufio.NewScanner(f)
	scanner.Buffer(make([]byte, 0, 256*1024), 1024*1024)
	for scanner.Scan() {
		line := scanner.Text()
		if line == "" {
			continue
		}
		ring = append(ring, line)
		if len(ring) > lastN {
			ring = ring[1:]
		}
	}
	if err := scanner.Err(); err != nil {
		return err
	}

	m.logFile = path
	m.iteration = parseIterationFromPath(path)

	for _, raw := range ring {
		rec, err := logrender.ParseRecord(raw)
		if err != nil {
			continue
		}
		rendered := m.renderLine(rec)
		if rendered == "" {
			continue
		}
		m.lines = append(m.lines, logLine{
			record:   rec,
			rendered: rendered,
			rawJSON:  raw,
		})
	}
	m.trimBuffer()
	m.rebuildViewport()
	m.follow = true
	m.viewport.GotoBottom()
	return nil
}

// SearchContent returns the rendered text of all visible log lines,
// one entry per line, suitable for search matching.
func (m LogViewModel) SearchContent() []string {
	visible := m.filteredLines()
	out := make([]string, len(visible))
	for i, ll := range visible {
		out[i] = ll.rendered
	}
	return out
}
