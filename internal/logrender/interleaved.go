package logrender

import (
	"context"
	"fmt"
	"io"
	"time"
)

// InterleavedRenderer writes stage headers and agent thoughts together in
// chronological order, each prefixed with a wall-clock timestamp. Stage starts
// get the ▶ glyph, completions get ✓/✗ with duration, and assistant text is
// indented five spaces to separate it visually from stage headers. The behavior
// is identical for replay and follow since records are processed as they arrive.
type InterleavedRenderer struct {
	w io.Writer
}

// NewInterleavedRenderer returns a renderer that writes interleaved output to w.
func NewInterleavedRenderer(w io.Writer) *InterleavedRenderer {
	return &InterleavedRenderer{w: w}
}

// Render consumes records from the channel and writes timestamped, interleaved
// output. It returns when the channel closes or ctx is cancelled.
func (ir *InterleavedRenderer) Render(ctx context.Context, records <-chan Record) {
	starts := make(map[stageKey]time.Time)

	for {
		select {
		case <-ctx.Done():
			return
		case r, ok := <-records:
			if !ok {
				return
			}
			ir.handleRecord(r, starts)
		}
	}
}

// handleRecord processes a single record, writing formatted output for
// display-worthy types and silently dropping everything else.
func (ir *InterleavedRenderer) handleRecord(r Record, starts map[stageKey]time.Time) {
	switch r.Type {
	case "stage_start":
		if skipStage(r.Stage) {
			return
		}
		starts[keyFor(r)] = r.Timestamp
		_, _ = fmt.Fprintf(ir.w, "%s ▶ [%s] %s\n",
			formatTimestamp(r.Timestamp), r.StageLabel(), nodeAddress(r))

	case "stage_complete":
		if skipStage(r.Stage) {
			return
		}
		key := keyFor(r)
		dur := time.Duration(0)
		if t, ok := starts[key]; ok {
			dur = r.Timestamp.Sub(t)
			delete(starts, key)
		}
		_, _ = fmt.Fprintf(ir.w, "%s %s [%s] %s (%s)\n",
			formatTimestamp(r.Timestamp), glyphFor(r.ExitCode),
			r.StageLabel(), nodeAddress(r), FormatDuration(dur))

	case "planning_start":
		pk := stageKey{node: r.Node, task: r.Task, stage: "plan"}
		starts[pk] = r.Timestamp
		_, _ = fmt.Fprintf(ir.w, "%s ▶ [plan] %s\n",
			formatTimestamp(r.Timestamp), nodeAddress(r))

	case "planning_complete":
		pk := stageKey{node: r.Node, task: r.Task, stage: "plan"}
		dur := time.Duration(0)
		if t, ok := starts[pk]; ok {
			dur = r.Timestamp.Sub(t)
			delete(starts, pk)
		}
		_, _ = fmt.Fprintf(ir.w, "%s %s [plan] %s (%s)\n",
			formatTimestamp(r.Timestamp), glyphFor(r.ExitCode),
			nodeAddress(r), FormatDuration(dur))

	case "assistant":
		if r.Text != "" {
			_, _ = fmt.Fprintf(ir.w, "%s     %s\n",
				formatTimestamp(r.Timestamp), r.Text)
		}

	case "audit_report_written":
		_, _ = fmt.Fprintf(ir.w, "%s     report: %s\n",
			formatTimestamp(r.Timestamp), r.Path)
	}
}

// formatTimestamp renders a time as HH:MM:SS in the local timezone.
func formatTimestamp(t time.Time) string {
	return t.Local().Format("15:04:05")
}
