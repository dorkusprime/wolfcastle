package state

import "time"

// NodeStatus represents the four valid states for nodes and tasks.
type NodeStatus string

const (
	StatusNotStarted NodeStatus = "not_started"
	StatusInProgress NodeStatus = "in_progress"
	StatusComplete   NodeStatus = "complete"
	StatusBlocked    NodeStatus = "blocked"
)

// AuditStatus represents the audit lifecycle states.
type AuditStatus string

const (
	AuditPending    AuditStatus = "pending"
	AuditInProgress AuditStatus = "in_progress"
	AuditPassed     AuditStatus = "passed"
	AuditFailed     AuditStatus = "failed"
)

// NodeType distinguishes orchestrators from leaves.
type NodeType string

const (
	NodeOrchestrator NodeType = "orchestrator"
	NodeLeaf         NodeType = "leaf"
)

// RootIndex is the centralized tree index at the engineer namespace root.
type RootIndex struct {
	Version   int                   `json:"version"`
	RootID    string                `json:"root_id,omitempty"`
	RootName  string                `json:"root_name,omitempty"`
	RootState NodeStatus            `json:"root_state,omitempty"`
	Root      []string              `json:"root,omitempty"`
	Nodes     map[string]IndexEntry `json:"nodes"`
}

// IndexEntry is a single node in the root index.
type IndexEntry struct {
	Name               string     `json:"name"`
	Type               NodeType   `json:"type"`
	State              NodeStatus `json:"state"`
	Address            string     `json:"address"`
	DecompositionDepth int        `json:"decomposition_depth"`
	Parent             string     `json:"parent,omitempty"`
	Children           []string   `json:"children,omitempty"`
}

// NodeState is the per-node state file.
type NodeState struct {
	Version            int        `json:"version"`
	ID                 string     `json:"id"`
	Name               string     `json:"name"`
	Type               NodeType   `json:"type"`
	State              NodeStatus `json:"state"`
	DecompositionDepth int        `json:"decomposition_depth"`
	// Orchestrator fields
	Children []ChildRef `json:"children,omitempty"`
	// Leaf fields
	Tasks []Task `json:"tasks,omitempty"`
	// Common
	Audit AuditState `json:"audit"`
	Specs []string   `json:"specs,omitempty"`
}

// ChildRef is a reference to a child node in an orchestrator's state.
type ChildRef struct {
	ID      string     `json:"id"`
	Address string     `json:"address"`
	State   NodeStatus `json:"state"`
}

// Task is a single task within a leaf node.
type Task struct {
	ID            string     `json:"id"`
	Description   string     `json:"description"`
	State         NodeStatus `json:"state"`
	IsAudit       bool       `json:"is_audit,omitempty"`
	BlockedReason string     `json:"blocked_reason,omitempty"`
	FailureCount  int        `json:"failure_count"`
	Breadcrumbs   []string   `json:"breadcrumbs,omitempty"`
}

// AuditState tracks audit information for a node.
type AuditState struct {
	Scope         *AuditScope  `json:"scope,omitempty"`
	Breadcrumbs   []Breadcrumb `json:"breadcrumbs"`
	Gaps          []Gap        `json:"gaps"`
	Escalations   []Escalation `json:"escalations"`
	Status        AuditStatus  `json:"status"`
	StartedAt     *time.Time   `json:"started_at,omitempty"`
	CompletedAt   *time.Time   `json:"completed_at,omitempty"`
	ResultSummary string       `json:"result_summary,omitempty"`
}

// AuditScope defines what the audit must verify.
type AuditScope struct {
	Description string   `json:"description"`
	Files       []string `json:"files,omitempty"`
	Systems     []string `json:"systems,omitempty"`
	Criteria    []string `json:"criteria,omitempty"`
}

// Breadcrumb records a change made during task execution.
type Breadcrumb struct {
	Timestamp time.Time `json:"timestamp"`
	Task      string    `json:"task"`
	Text      string    `json:"text"`
}

// Gap represents an issue found during audit.
type Gap struct {
	ID          string     `json:"id"`
	Timestamp   time.Time  `json:"timestamp"`
	Description string     `json:"description"`
	Source      string     `json:"source"`
	Status      string     `json:"status"` // "open" or "fixed"
	FixedBy     string     `json:"fixed_by,omitempty"`
	FixedAt     *time.Time `json:"fixed_at,omitempty"`
}

// Escalation is a gap escalated to a parent node.
type Escalation struct {
	ID          string     `json:"id"`
	Timestamp   time.Time  `json:"timestamp"`
	Description string     `json:"description"`
	SourceNode  string     `json:"source_node"`
	SourceGapID string     `json:"source_gap_id,omitempty"`
	Status      string     `json:"status"` // "open" or "resolved"
	ResolvedBy  string     `json:"resolved_by,omitempty"`
	ResolvedAt  *time.Time `json:"resolved_at,omitempty"`
}

// NewRootIndex creates an empty root index.
func NewRootIndex() *RootIndex {
	return &RootIndex{
		Version: 1,
		Nodes:   make(map[string]IndexEntry),
	}
}

// NewNodeState creates a new node state with the given properties.
func NewNodeState(id, name string, nodeType NodeType) *NodeState {
	return &NodeState{
		Version: 1,
		ID:      id,
		Name:    name,
		Type:    nodeType,
		State:   StatusNotStarted,
		Audit: AuditState{
			Status:      AuditPending,
			Breadcrumbs: []Breadcrumb{},
			Gaps:        []Gap{},
			Escalations: []Escalation{},
		},
	}
}
