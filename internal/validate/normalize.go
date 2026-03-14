package validate

import (
	"strings"

	"github.com/dorkusprime/wolfcastle/internal/state"
)

// NormalizeStateValue maps common typos and aliases to canonical state values.
// Returns the normalized value and true if a mapping was found.
func NormalizeStateValue(s string) (state.NodeStatus, bool) {
	switch strings.ToLower(strings.TrimSpace(s)) {
	case "complete", "completed", "done":
		return state.StatusComplete, true
	case "not_started", "not-started", "pending", "todo":
		return state.StatusNotStarted, true
	case "in_progress", "in-progress", "started", "doing":
		return state.StatusInProgress, true
	case "blocked", "stuck":
		return state.StatusBlocked, true
	default:
		return "", false
	}
}
