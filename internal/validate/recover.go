package validate

import (
	"bytes"
	"encoding/json"
	"fmt"
	"strings"

	"github.com/dorkusprime/wolfcastle/internal/state"
)

// RecoveryReport describes what was recovered and what was lost.
type RecoveryReport struct {
	Applied []string // list of sanitization steps that were applied
	Lost    []string // descriptions of data that could not be recovered
}

// RecoverNodeState attempts to recover a NodeState from malformed JSON data.
// It returns the best-effort parse, a report of what was recovered/lost, and
// an error only when recovery is completely impossible.
func RecoverNodeState(data []byte) (*state.NodeState, *RecoveryReport, error) {
	report := &RecoveryReport{}

	// Empty file: return a blank state.
	if len(bytes.TrimSpace(data)) == 0 {
		report.Applied = append(report.Applied, "empty file replaced with fresh state")
		ns := &state.NodeState{Version: 1, State: state.StatusNotStarted}
		return ns, report, nil
	}

	cleaned, steps := sanitizeJSON(data)
	report.Applied = steps

	// First, try the cleaned bytes directly.
	var ns state.NodeState
	if err := json.Unmarshal(cleaned, &ns); err == nil {
		detectLoss(&ns, cleaned, report)
		normalizeRecovered(&ns)
		return &ns, report, nil
	}

	// Trailing garbage: walk backward stripping bytes until we find valid JSON.
	if recovered, ok := tryStripTrailing(cleaned); ok {
		report.Applied = append(report.Applied, "stripped trailing garbage after valid JSON")
		var ns state.NodeState
		if err := json.Unmarshal(recovered, &ns); err == nil {
			detectLoss(&ns, cleaned, report)
			normalizeRecovered(&ns)
			return &ns, report, nil
		}
	}

	// Truncated: try closing open braces/brackets to form valid JSON.
	if recovered, ok := tryCloseTruncated(cleaned); ok {
		report.Applied = append(report.Applied, "closed truncated JSON structure")
		var ns state.NodeState
		if err := json.Unmarshal(recovered, &ns); err == nil {
			detectLoss(&ns, cleaned, report)
			normalizeRecovered(&ns)
			return &ns, report, nil
		}
	}

	return nil, report, fmt.Errorf("json recovery failed: data is too corrupted to salvage")
}

// RecoverRootIndex attempts to recover a RootIndex from malformed JSON data.
func RecoverRootIndex(data []byte) (*state.RootIndex, *RecoveryReport, error) {
	report := &RecoveryReport{}

	if len(bytes.TrimSpace(data)) == 0 {
		report.Applied = append(report.Applied, "empty file replaced with fresh index")
		idx := state.NewRootIndex()
		return idx, report, nil
	}

	cleaned, steps := sanitizeJSON(data)
	report.Applied = steps

	var idx state.RootIndex
	if err := json.Unmarshal(cleaned, &idx); err == nil {
		if idx.Nodes == nil {
			idx.Nodes = make(map[string]state.IndexEntry)
		}
		return &idx, report, nil
	}

	if recovered, ok := tryStripTrailing(cleaned); ok {
		report.Applied = append(report.Applied, "stripped trailing garbage after valid JSON")
		var idx state.RootIndex
		if err := json.Unmarshal(recovered, &idx); err == nil {
			if idx.Nodes == nil {
				idx.Nodes = make(map[string]state.IndexEntry)
			}
			return &idx, report, nil
		}
	}

	if recovered, ok := tryCloseTruncated(cleaned); ok {
		report.Applied = append(report.Applied, "closed truncated JSON structure")
		var idx state.RootIndex
		if err := json.Unmarshal(recovered, &idx); err == nil {
			if idx.Nodes == nil {
				idx.Nodes = make(map[string]state.IndexEntry)
			}
			return &idx, report, nil
		}
	}

	return nil, report, fmt.Errorf("json recovery failed: root index is too corrupted to salvage")
}

// sanitizeJSON applies non-destructive cleanups to raw bytes: BOM removal,
// null byte stripping, and whitespace trimming. Returns the cleaned bytes
// and a list of steps taken.
func sanitizeJSON(data []byte) ([]byte, []string) {
	var steps []string
	out := data

	// Strip UTF-8 BOM (EF BB BF).
	if len(out) >= 3 && out[0] == 0xEF && out[1] == 0xBB && out[2] == 0xBF {
		out = out[3:]
		steps = append(steps, "stripped UTF-8 BOM")
	}

	// Strip null bytes.
	if bytes.ContainsRune(out, 0) {
		out = bytes.ReplaceAll(out, []byte{0}, nil)
		steps = append(steps, "stripped null bytes")
	}

	// Trim surrounding whitespace.
	trimmed := bytes.TrimSpace(out)
	if len(trimmed) != len(out) {
		out = trimmed
	}

	return out, steps
}

// tryStripTrailing finds valid JSON at the front of data by removing trailing
// bytes. It looks for the last matching closing brace or bracket.
func tryStripTrailing(data []byte) ([]byte, bool) {
	if len(data) == 0 {
		return nil, false
	}

	// Determine expected closer based on first character.
	opener := data[0]
	var closer byte
	switch opener {
	case '{':
		closer = '}'
	case '[':
		closer = ']'
	default:
		return nil, false
	}

	// Walk backward to find the last occurrence of the closer, then try parsing.
	for i := len(data) - 1; i >= 0; i-- {
		if data[i] == closer {
			candidate := data[:i+1]
			var js json.RawMessage
			if json.Unmarshal(candidate, &js) == nil {
				return candidate, true
			}
		}
	}
	return nil, false
}

// tryCloseTruncated attempts to repair truncated JSON by removing the last
// incomplete value and closing all open structures (braces, brackets, strings).
func tryCloseTruncated(data []byte) ([]byte, bool) {
	if len(data) == 0 {
		return nil, false
	}

	// Must start with { or [
	if data[0] != '{' && data[0] != '[' {
		return nil, false
	}

	// Walk the bytes, tracking open structures.
	var stack []byte
	inString := false
	escaped := false
	lastComma := -1

	for i := 0; i < len(data); i++ {
		b := data[i]

		if escaped {
			escaped = false
			continue
		}

		if b == '\\' && inString {
			escaped = true
			continue
		}

		if b == '"' {
			inString = !inString
			continue
		}

		if inString {
			continue
		}

		switch b {
		case '{':
			stack = append(stack, '}')
		case '[':
			stack = append(stack, ']')
		case '}', ']':
			if len(stack) > 0 && stack[len(stack)-1] == b {
				stack = stack[:len(stack)-1]
			}
		case ',':
			lastComma = i
		}
	}

	// If nothing is left open, data isn't truncated in a way we can fix.
	if len(stack) == 0 {
		return nil, false
	}

	// If we were inside a string, close it first and strip the incomplete key/value.
	// Strategy: rewind to the last comma before the truncation point (or the
	// opening brace/bracket if no comma), then close the stack.
	base := data
	if inString {
		// Trim back to last comma or opening structure.
		if lastComma > 0 {
			base = data[:lastComma]
		} else {
			// No comma: just the opening brace/bracket with nothing salvageable.
			base = data[:1]
		}
		// Recompute the stack from the trimmed data.
		stack = recomputeStack(base)
	} else {
		// Trim any trailing comma or colon that would make the JSON invalid.
		trimmed := bytes.TrimRight(base, " \t\n\r,:")
		if len(trimmed) < len(base) {
			base = trimmed
		}
	}

	// Close every open structure.
	var buf bytes.Buffer
	buf.Write(base)
	for i := len(stack) - 1; i >= 0; i-- {
		buf.WriteByte(stack[i])
	}

	result := buf.Bytes()
	var js json.RawMessage
	if json.Unmarshal(result, &js) == nil {
		return result, true
	}

	// Fallback: try trimming back to last comma, then closing.
	if lastComma > 0 {
		base = data[:lastComma]
		stack = recomputeStack(base)
		buf.Reset()
		buf.Write(base)
		for i := len(stack) - 1; i >= 0; i-- {
			buf.WriteByte(stack[i])
		}
		result = buf.Bytes()
		if json.Unmarshal(result, &js) == nil {
			return result, true
		}
	}

	return nil, false
}

func recomputeStack(data []byte) []byte {
	var stack []byte
	inString := false
	escaped := false
	for i := 0; i < len(data); i++ {
		b := data[i]
		if escaped {
			escaped = false
			continue
		}
		if b == '\\' && inString {
			escaped = true
			continue
		}
		if b == '"' {
			inString = !inString
			continue
		}
		if inString {
			continue
		}
		switch b {
		case '{':
			stack = append(stack, '}')
		case '[':
			stack = append(stack, ']')
		case '}', ']':
			if len(stack) > 0 && stack[len(stack)-1] == b {
				stack = stack[:len(stack)-1]
			}
		}
	}
	return stack
}

// detectLoss checks whether the recovered NodeState looks incomplete compared
// to what the raw JSON suggests, and records any losses.
func detectLoss(ns *state.NodeState, data []byte, report *RecoveryReport) {
	raw := string(data)
	// Count how many "id" fields appear inside a "tasks" array context.
	// This is a rough heuristic, not a full parser.
	tasksIdx := strings.Index(raw, `"tasks"`)
	if tasksIdx >= 0 {
		taskFragment := raw[tasksIdx:]
		idCount := strings.Count(taskFragment, `"id"`)
		if idCount > len(ns.Tasks) {
			report.Lost = append(report.Lost,
				fmt.Sprintf("tasks: JSON references ~%d task(s) but only %d survived recovery", idCount, len(ns.Tasks)))
		}
	}

	childrenIdx := strings.Index(raw, `"children"`)
	if childrenIdx >= 0 {
		childFragment := raw[childrenIdx:]
		idCount := strings.Count(childFragment, `"id"`)
		if idCount > len(ns.Children) {
			report.Lost = append(report.Lost,
				fmt.Sprintf("children: JSON references ~%d child(ren) but only %d survived recovery", idCount, len(ns.Children)))
		}
	}
}

// normalizeRecovered applies the same audit state normalization that
// LoadNodeState would.
func normalizeRecovered(ns *state.NodeState) {
	if ns.Audit.Breadcrumbs == nil {
		ns.Audit.Breadcrumbs = []state.Breadcrumb{}
	}
	if ns.Audit.Gaps == nil {
		ns.Audit.Gaps = []state.Gap{}
	}
	if ns.Audit.Escalations == nil {
		ns.Audit.Escalations = []state.Escalation{}
	}
	if ns.Audit.Status == "" {
		ns.Audit.Status = state.AuditPending
	}
}
