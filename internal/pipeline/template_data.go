package pipeline

// ADRData holds the template variables for ADR (Architecture Decision Record)
// generation via templates/artifacts/adr.md.tmpl.
type ADRData struct {
	Title string
	Date  string
	Body  string
}

// SpecData holds the template variables for spec document generation via
// templates/artifacts/spec.md.tmpl.
type SpecData struct {
	Title string
	Body  string
}

// TaskData holds the template variables for task markdown generation via
// templates/artifacts/task.md.tmpl.
type TaskData struct {
	ID                 string
	Title              string
	Description        string
	Body               string
	Type               string
	Class              string
	Deliverables       []string
	Constraints        []string
	References         []string
	AcceptanceCriteria []string
}

// AuditTaskData holds the template variables for audit-task markdown
// generation via templates/artifacts/audit-task.md.tmpl.
type AuditTaskData struct {
	NodeName string
}
