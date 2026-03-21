package logrender

import (
	"fmt"
	"io"
	"strings"
	"time"
)

// SummaryRenderer consumes a stream of records and produces the summary view:
// one line per completed stage with glyph, stage label, node address, and
// duration. Audit report paths appear indented on the line following their
// audit stage.
type SummaryRenderer struct {
	w io.Writer
}

// NewSummaryRenderer returns a renderer that writes summary output to w.
func NewSummaryRenderer(w io.Writer) *SummaryRenderer {
	return &SummaryRenderer{w: w}
}

// summaryLine holds a single formatted output line before column alignment.
type summaryLine struct {
	glyph   string // ✓ or ✗
	label   string // [execute], [plan], etc.
	address string // node/task-id
	dur     string // (1m22s)
	report  string // non-empty only for audit_report_written follow-ups
}

// stageKey uniquely identifies an in-flight stage so we can pair start with
// complete records. Node, task, and stage together form the identity.
type stageKey struct {
	node  string
	task  string
	stage string
}

// Replay drains records from the channel and writes the summary view. It
// buffers all output lines so columns can be aligned across the full session.
func (sr *SummaryRenderer) Replay(records <-chan Record) {
	starts := make(map[stageKey]time.Time)
	var lines []summaryLine

	for r := range records {
		switch r.Type {
		case "stage_start":
			if skipStage(r.Stage) {
				continue
			}
			starts[keyFor(r)] = r.Timestamp

		case "stage_complete":
			if skipStage(r.Stage) {
				continue
			}
			dur := time.Duration(0)
			if t, ok := starts[keyFor(r)]; ok {
				dur = r.Timestamp.Sub(t)
				delete(starts, keyFor(r))
			}
			lines = append(lines, summaryLine{
				glyph:   glyphFor(r.ExitCode),
				label:   fmt.Sprintf("[%s]", r.StageLabel()),
				address: nodeAddress(r),
				dur:     fmt.Sprintf("(%s)", FormatDuration(dur)),
			})

		case "planning_start":
			starts[stageKey{node: r.Node, task: r.Task, stage: "plan"}] = r.Timestamp

		case "planning_complete":
			pk := stageKey{node: r.Node, task: r.Task, stage: "plan"}
			dur := time.Duration(0)
			if t, ok := starts[pk]; ok {
				dur = r.Timestamp.Sub(t)
				delete(starts, pk)
			}
			lines = append(lines, summaryLine{
				glyph:   glyphFor(r.ExitCode),
				label:   "[plan]",
				address: nodeAddress(r),
				dur:     fmt.Sprintf("(%s)", FormatDuration(dur)),
			})

		case "audit_report_written":
			lines = append(lines, summaryLine{
				report: r.Path,
			})
		}
	}

	sr.writeAligned(lines)
}

// writeAligned pads labels and addresses so columns line up, then writes
// every line to the underlying writer.
func (sr *SummaryRenderer) writeAligned(lines []summaryLine) {
	maxLabel := 0
	maxAddr := 0
	for _, l := range lines {
		if l.report != "" {
			continue
		}
		if len(l.label) > maxLabel {
			maxLabel = len(l.label)
		}
		if len(l.address) > maxAddr {
			maxAddr = len(l.address)
		}
	}

	for _, l := range lines {
		if l.report != "" {
			fmt.Fprintf(sr.w, "  report: %s\n", l.report)
			continue
		}
		labelPad := maxLabel - len(l.label)
		addrPad := maxAddr - len(l.address)
		fmt.Fprintf(sr.w, "%s %s%s %s%s %s\n",
			l.glyph,
			l.label, strings.Repeat(" ", labelPad),
			l.address, strings.Repeat(" ", addrPad),
			l.dur,
		)
	}
}

// glyphFor returns ✓ when the exit code is zero (or absent) and ✗ otherwise.
func glyphFor(code *int) string {
	if code != nil && *code != 0 {
		return "✗"
	}
	return "✓"
}

// nodeAddress builds the display address from a record's node and task fields.
func nodeAddress(r Record) string {
	if r.Task != "" {
		return r.Node + "/" + r.Task
	}
	return r.Node
}

// keyFor builds a stageKey from a record's node, task, and stage fields.
func keyFor(r Record) stageKey {
	return stageKey{node: r.Node, task: r.Task, stage: r.Stage}
}

// skipStage returns true for record stages that should be filtered from
// summary output.
func skipStage(stage string) bool {
	switch stage {
	case "daemon_start", "daemon_stop":
		return true
	}
	return false
}
