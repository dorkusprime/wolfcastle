// Package state manages the persistent tree state for Wolfcastle projects.
// It provides types, I/O, mutations, navigation, and propagation logic for
// the distributed per-node state files described in ADR-024. Inbox and review
// batch types are co-located here per ADR-058.
package state

import "time"

// NodeStatus represents the four valid lifecycle states for nodes and tasks.
type NodeStatus string

const (
	// StatusNotStarted means no work has begun on this node or task.
	StatusNotStarted NodeStatus = "not_started"
	// StatusInProgress means work is actively underway.
	StatusInProgress NodeStatus = "in_progress"
	// StatusComplete means all work has been finished and verified.
	StatusComplete NodeStatus = "complete"
	// StatusBlocked means forward progress is prevented by an external dependency or failure.
	StatusBlocked NodeStatus = "blocked"
)

// AuditStatus represents the audit lifecycle states.
type AuditStatus string

const (
	// AuditPending means the audit has not yet started.
	AuditPending AuditStatus = "pending"
	// AuditInProgress means the audit is actively running.
	AuditInProgress AuditStatus = "in_progress"
	// AuditPassed means the audit completed with no open gaps.
	AuditPassed AuditStatus = "passed"
	// AuditFailed means the audit found unresolved gaps or the node is blocked.
	AuditFailed AuditStatus = "failed"
)

// GapStatus represents the lifecycle states of an audit gap.
type GapStatus string

const (
	// GapOpen indicates the gap has not yet been addressed.
	GapOpen GapStatus = "open"
	// GapFixed indicates the gap has been resolved.
	GapFixed GapStatus = "fixed"
)

// EscalationStatus represents the lifecycle states of an escalation.
type EscalationStatus string

const (
	// EscalationOpen indicates the escalation is unresolved.
	EscalationOpen EscalationStatus = "open"
	// EscalationResolved indicates the escalation has been addressed.
	EscalationResolved EscalationStatus = "resolved"
)

// NodeType distinguishes orchestrators from leaves.
type NodeType string

const (
	// NodeOrchestrator is a parent node whose state derives from its children.
	NodeOrchestrator NodeType = "orchestrator"
	// NodeLeaf is a terminal node containing tasks.
	NodeLeaf NodeType = "leaf"
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
	// Orchestrator planning
	Scope           string         `json:"scope,omitempty"`
	PendingScope    []string       `json:"pending_scope,omitempty"`
	SuccessCriteria []string       `json:"success_criteria,omitempty"`
	AuditEnrichment []string       `json:"audit_enrichment,omitempty"`
	NeedsPlanning   bool           `json:"needs_planning,omitempty"`
	PlanningTrigger string         `json:"planning_trigger,omitempty"`
	PlanningModel   string         `json:"planning_model,omitempty"`
	ReplanCount     map[string]int `json:"replan_count,omitempty"`  // deprecated: use TotalReplans
	TotalReplans    int            `json:"total_replans,omitempty"` // cumulative replan count across all triggers
	MaxReplans      int            `json:"max_replans,omitempty"`
	PlanningHistory []PlanningPass `json:"planning_history,omitempty"`
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
	ID                 string     `json:"id"`
	Title              string     `json:"title,omitempty"`
	Description        string     `json:"description"`
	State              NodeStatus `json:"state"`
	IsAudit            bool       `json:"is_audit,omitempty"`
	BlockedReason      string     `json:"block_reason,omitempty"`
	FailureCount       int        `json:"failure_count"`
	NeedsDecomposition bool       `json:"needs_decomposition,omitempty"`
	Deliverables       []string   `json:"deliverables,omitempty"`
	LastFailureType    string     `json:"last_failure_type,omitempty"`
	Body               string     `json:"body,omitempty"`
	TaskType           string     `json:"task_type,omitempty"`
	Class              string     `json:"class,omitempty"`
	Constraints        []string   `json:"constraints,omitempty"`
	AcceptanceCriteria []string   `json:"acceptance_criteria,omitempty"`
	References         []string   `json:"references,omitempty"`
	Integration        string     `json:"integration,omitempty"`
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
	Status      GapStatus  `json:"status"`
	FixedBy     string     `json:"fixed_by,omitempty"`
	FixedAt     *time.Time `json:"fixed_at,omitempty"`
}

// Escalation is a gap escalated to a parent node.
type Escalation struct {
	ID          string           `json:"id"`
	Timestamp   time.Time        `json:"timestamp"`
	Description string           `json:"description"`
	SourceNode  string           `json:"source_node"`
	SourceGapID string           `json:"source_gap_id,omitempty"`
	Status      EscalationStatus `json:"status"`
	ResolvedBy  string           `json:"resolved_by,omitempty"`
	ResolvedAt  *time.Time       `json:"resolved_at,omitempty"`
}

// PlanningPass records a single orchestrator planning iteration.
type PlanningPass struct {
	Timestamp time.Time `json:"timestamp"`
	Trigger   string    `json:"trigger"`
	Summary   string    `json:"summary"`
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
			Status: AuditPending,
			Scope: &AuditScope{
				Files:    []string{},
				Systems:  []string{},
				Criteria: []string{},
			},
			Breadcrumbs: []Breadcrumb{},
			Gaps:        []Gap{},
			Escalations: []Escalation{},
		},
	}
}
