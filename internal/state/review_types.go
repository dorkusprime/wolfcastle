package state

import "time"

// BatchStatus represents the lifecycle of a review batch.
type BatchStatus string

const (
	// BatchPending indicates the batch is awaiting human review.
	BatchPending BatchStatus = "pending"
	// BatchCompleted indicates all findings in the batch have been decided.
	BatchCompleted BatchStatus = "completed"
)

// FindingStatus represents the disposition of a single finding.
type FindingStatus string

const (
	// FindingPending indicates no decision has been made yet.
	FindingPending FindingStatus = "pending"
	// FindingApproved indicates the finding was accepted and a node may be created.
	FindingApproved FindingStatus = "approved"
	// FindingRejected indicates the finding was dismissed.
	FindingRejected FindingStatus = "rejected"
)

// Batch is a collection of audit findings awaiting review.
type Batch struct {
	ID        string      `json:"id"`
	Timestamp time.Time   `json:"timestamp"`
	Scopes    []string    `json:"scopes"`
	Status    BatchStatus `json:"status"`
	Findings  []Finding   `json:"findings"`
	RawOutput string      `json:"raw_output,omitempty"`
}

// Finding is a single audit finding extracted from model output.
type Finding struct {
	ID          string        `json:"id"`
	Title       string        `json:"title"`
	Description string        `json:"description,omitempty"`
	Status      FindingStatus `json:"status"`
	DecidedAt   *time.Time    `json:"decided_at,omitempty"`
	CreatedNode string        `json:"created_node,omitempty"`
}

// HistoryEntry records a completed review batch with its decisions.
type HistoryEntry struct {
	BatchID     string     `json:"batch_id"`
	CompletedAt time.Time  `json:"completed_at"`
	Scopes      []string   `json:"scopes"`
	Decisions   []Decision `json:"decisions"`
}

// Decision records a single approve/reject action.
type Decision struct {
	FindingID   string    `json:"finding_id"`
	Title       string    `json:"title"`
	Action      string    `json:"action"` // "approved" or "rejected"
	Timestamp   time.Time `json:"timestamp"`
	CreatedNode string    `json:"created_node,omitempty"`
}

// History is the durable record of all completed review batches.
type History struct {
	Entries []HistoryEntry `json:"entries"`
}
