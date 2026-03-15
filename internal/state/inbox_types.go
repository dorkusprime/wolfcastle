package state

// InboxItem represents a single inbox entry.
type InboxItem struct {
	Timestamp string `json:"timestamp"`
	Text      string `json:"text"`
	Status    string `json:"status"`
}

// InboxFile is the top-level inbox.json structure.
type InboxFile struct {
	Items []InboxItem `json:"items"`
}
