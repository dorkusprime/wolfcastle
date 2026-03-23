package validate

import (
	"strings"

	"github.com/dorkusprime/wolfcastle/internal/state"
)

// NormalizeStateValue maps common typos and aliases to canonical state values.
// Returns the normalized value and true if a mapping was found.
func NormalizeStateValue(s string) (state.NodeStatus, bool) {
	switch strings.ToLower(strings.TrimSpace(s)) {
	case string(state.StatusComplete), "completed", "done":
		return state.StatusComplete, true
	case string(state.StatusNotStarted), "not-started", "pending", "todo":
		return state.StatusNotStarted, true
	case string(state.StatusInProgress), "in-progress", "started", "doing":
		return state.StatusInProgress, true
	case string(state.StatusBlocked), "stuck":
		return state.StatusBlocked, true
	default:
		return "", false
	}
}
