package logrender

import (
	"encoding/json"
	"time"
)

// Record holds a parsed NDJSON log line with typed fields for every record
// the renderers consume. Unrecognized fields land in Raw for forward
// compatibility; unrecognized Type values parse without error so renderers
// can skip them cleanly.
type Record struct {
	Type       string    `json:"type"`
	Timestamp  time.Time `json:"timestamp"`
	Level      string    `json:"level"`
	Trace      string    `json:"trace"`
	Stage      string    `json:"stage"`
	Node       string    `json:"node"`
	Task       string    `json:"task"`
	ExitCode   *int      `json:"exit_code"`
	DurationMS *int64    `json:"duration_ms"`
	Text       string    `json:"text"`
	Path       string    `json:"path"`
	Marker     string    `json:"marker"`
	Error      string    `json:"error"`

	// Lifecycle and event fields for daemon_lifecycle, self_heal, iteration_header, etc.
	Event     string  `json:"event,omitempty"`     // lifecycle: "engaged", "standing_down", "drain", "crash_restart"
	Action    string  `json:"action,omitempty"`     // self_heal, inbox_event, task_event, archive_event, spec_event
	Iteration int     `json:"iteration,omitempty"`  // iteration_header: iteration number
	Kind      string  `json:"kind,omitempty"`       // iteration_header: "execute" vs "plan"
	Reason    string  `json:"reason,omitempty"`     // idle_reason, shutdown reason
	Counter   int     `json:"counter,omitempty"`    // inbox_event: inbox counter
	Attempt   int     `json:"attempt,omitempty"`    // retry_event: attempt number
	DelayS    float64 `json:"delay_s,omitempty"`    // retry_event: delay in seconds
	Scope     string  `json:"scope,omitempty"`      // daemon_lifecycle: scope label

	// Raw preserves every field from the original JSON line, including
	// fields not mapped to typed struct members.
	Raw map[string]any `json:"-"`
}

// StageLabel returns the display label for this record's stage value.
func (r Record) StageLabel() string {
	return r.Stage
}

// ParseRecord unmarshals a single JSON line into a Record. Malformed input
// returns an error; the function never panics. Unrecognized type values
// parse successfully so renderers can decide what to skip.
func ParseRecord(line string) (Record, error) {
	var r Record

	// First pass: unmarshal into the raw map to capture all fields.
	if err := json.Unmarshal([]byte(line), &r.Raw); err != nil {
		return Record{}, err
	}

	// Second pass: unmarshal into the typed struct.
	if err := json.Unmarshal([]byte(line), &r); err != nil {
		return Record{}, err
	}

	return r, nil
}
