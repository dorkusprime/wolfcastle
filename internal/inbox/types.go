package inbox

// Item represents a single inbox entry.
type Item struct {
	Timestamp string `json:"timestamp"`
	Text      string `json:"text"`
	Status    string `json:"status"`
	Expanded  string `json:"expanded,omitempty"`
}

// File is the top-level inbox.json structure.
type File struct {
	Items []Item `json:"items"`
}
