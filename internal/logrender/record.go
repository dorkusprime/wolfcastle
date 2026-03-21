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
	Type      string    `json:"type"`
	Timestamp time.Time `json:"timestamp"`
	Level     string    `json:"level"`
	Trace     string    `json:"trace"`
	Stage     string    `json:"stage"`
	Node      string    `json:"node"`
	Task      string    `json:"task"`
	ExitCode  *int      `json:"exit_code"`
	Text      string    `json:"text"`
	Path      string    `json:"path"`
	Marker    string    `json:"marker"`
	Error     string    `json:"error"`

	// Raw preserves every field from the original JSON line, including
	// fields not mapped to typed struct members.
	Raw map[string]any `json:"-"`
}

// stageLabels maps stage field values to their display labels.
var stageLabels = map[string]string{
	"intake":      "intake",
	"plan":        "plan",
	"execute":     "execute",
	"audit":       "audit",
	"remediate":   "remediate",
	"spec-review": "spec-review",
}

// StageLabel returns the display label for this record's stage value.
// If the stage is empty or unrecognized, the raw stage value is returned.
func (r Record) StageLabel() string {
	if label, ok := stageLabels[r.Stage]; ok {
		return label
	}
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
