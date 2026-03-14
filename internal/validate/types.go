package validate

// Category constants for all 17 required validation issue categories.
const (
	CatRootIndexDanglingRef      = "ROOTINDEX_DANGLING_REF"
	CatRootIndexMissingEntry     = "ROOTINDEX_MISSING_ENTRY"
	CatOrphanState               = "ORPHAN_STATE"
	CatOrphanDefinition          = "ORPHAN_DEFINITION"
	CatPropagationMismatch       = "PROPAGATION_MISMATCH"
	CatMissingAuditTask          = "MISSING_AUDIT_TASK"
	CatAuditNotLast              = "AUDIT_NOT_LAST"
	CatMultipleAuditTasks        = "MULTIPLE_AUDIT_TASKS"
	CatInvalidStateValue         = "INVALID_STATE_VALUE"
	CatCompleteWithIncomplete    = "INVALID_TRANSITION_COMPLETE_WITH_INCOMPLETE"
	CatBlockedWithoutReason      = "INVALID_TRANSITION_BLOCKED_WITHOUT_REASON"
	CatStaleInProgress           = "STALE_IN_PROGRESS"
	CatMultipleInProgress        = "MULTIPLE_IN_PROGRESS"
	CatDepthMismatch             = "DEPTH_MISMATCH"
	CatNegativeFailureCount      = "NEGATIVE_FAILURE_COUNT"
	CatMissingRequiredField      = "MISSING_REQUIRED_FIELD"
	CatMalformedJSON             = "MALFORMED_JSON"
)

// FixType describes the repair strategy.
const (
	FixDeterministic  = "deterministic"
	FixModelAssisted  = "model-assisted"
	FixManual         = "manual"
)

// Severity levels.
const (
	SeverityError   = "error"
	SeverityWarning = "warning"
	SeverityInfo    = "info"
)

// Issue represents a single validation problem found in the tree.
type Issue struct {
	Severity    string `json:"severity"`
	Category    string `json:"category"`
	Node        string `json:"node,omitempty"`
	Description string `json:"description"`
	CanAutoFix  bool   `json:"can_auto_fix"`
	FixType     string `json:"fix_type,omitempty"`
}

// Report is the result of a full validation run.
type Report struct {
	Issues []Issue `json:"issues"`
	Errors int     `json:"errors"`
	Warnings int   `json:"warnings"`
}

// Counts populates the error and warning counts.
func (r *Report) Counts() {
	r.Errors = 0
	r.Warnings = 0
	for _, issue := range r.Issues {
		switch issue.Severity {
		case SeverityError:
			r.Errors++
		case SeverityWarning:
			r.Warnings++
		}
	}
}

// HasErrors returns true if there are error-severity issues.
func (r *Report) HasErrors() bool {
	for _, issue := range r.Issues {
		if issue.Severity == SeverityError {
			return true
		}
	}
	return false
}

// StartupCategories returns the subset of categories checked at daemon startup.
var StartupCategories = map[string]bool{
	CatMalformedJSON:          true,
	CatMissingRequiredField:   true,
	CatRootIndexDanglingRef:   true,
	CatRootIndexMissingEntry:  true,
	CatOrphanState:            true,
	CatInvalidStateValue:      true,
	CatCompleteWithIncomplete: true,
	CatPropagationMismatch:    true,
	CatMissingAuditTask:       true,
	CatStaleInProgress:        true,
	CatMultipleInProgress:     true,
}
