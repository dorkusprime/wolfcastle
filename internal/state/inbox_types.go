package state

// InboxItemStatus represents the lifecycle state of an inbox item.
type InboxItemStatus string

const (
	// InboxNew means the item has been added but not yet processed.
	InboxNew InboxItemStatus = "new"
	// InboxFiled means the item has been processed by the intake stage.
	InboxFiled InboxItemStatus = "filed"
)

// InboxItem represents a single inbox entry.
type InboxItem struct {
	Timestamp string          `json:"timestamp"`
	Text      string          `json:"text"`
	Status    InboxItemStatus `json:"status"`
}

// InboxFile is the top-level inbox.json structure.
type InboxFile struct {
	Items []InboxItem `json:"items"`
}
