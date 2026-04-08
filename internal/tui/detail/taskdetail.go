package detail

import (
	"fmt"
	"strings"

	"charm.land/bubbles/v2/viewport"
	tea "charm.land/bubbletea/v2"

	"github.com/dorkusprime/wolfcastle/internal/state"
	"github.com/dorkusprime/wolfcastle/internal/tui"
)

// TaskDetailModel renders detailed information about a single task, including
// description, body, deliverables, acceptance criteria, constraints, and
// failure information.
type TaskDetailModel struct {
	addr     string
	taskID   string
	task     *state.Task
	viewport viewport.Model
	width    int
	height   int
}

// NewTaskDetailModel creates a TaskDetailModel with an empty viewport.
func NewTaskDetailModel() TaskDetailModel {
	return TaskDetailModel{
		viewport: viewport.New(),
	}
}

// Load populates the model with task data and rebuilds the viewport content.
func (m *TaskDetailModel) Load(addr, taskID string, task *state.Task) {
	m.addr = addr
	m.taskID = taskID
	m.task = task
	m.rebuildContent()
}

// SetSize stores the available rendering area and propagates to the viewport.
func (m *TaskDetailModel) SetSize(width, height int) {
	m.width = width
	m.height = height
	m.viewport.SetWidth(width)
	m.viewport.SetHeight(height)
	m.rebuildContent()
}

// TaskAddr returns the fully qualified task address for clipboard copy.
func (m TaskDetailModel) TaskAddr() string {
	return m.addr + "/" + m.taskID
}

// Update forwards key events to the viewport for scrolling.
func (m TaskDetailModel) Update(msg tea.Msg) (TaskDetailModel, tea.Cmd) {
	switch msg.(type) {
	case tea.KeyPressMsg:
		var cmd tea.Cmd
		m.viewport, cmd = m.viewport.Update(msg)
		return m, cmd
	}
	return m, nil
}

// View renders the viewport.
func (m TaskDetailModel) View() string {
	return m.viewport.View()
}

// SearchContent returns the viewport content split into lines for search.
func (m TaskDetailModel) SearchContent() []string {
	return strings.Split(m.viewport.GetContent(), "\n")
}

func (m *TaskDetailModel) rebuildContent() {
	if m.task == nil {
		return
	}

	heading := tui.DashboardHeadingStyle
	body := tui.DashboardBodyStyle
	t := m.task
	wrapWidth := m.width
	if wrapWidth < 20 {
		wrapWidth = 80
	}

	var b strings.Builder

	// Title line: {task_id}  {status_glyph} {status}
	b.WriteString(heading.Render(t.ID))
	b.WriteString("  ")
	b.WriteString(tui.GlyphForStatus(string(t.State)))
	b.WriteString(" ")
	b.WriteString(body.Render(string(t.State)))
	b.WriteByte('\n')

	// Title
	if t.Title != "" {
		b.WriteString(body.Render(t.Title))
		b.WriteByte('\n')
	}

	// Description
	if t.Description != "" {
		b.WriteByte('\n')
		b.WriteString(body.Render(wrapIndent(t.Description, wrapWidth, "")))
		b.WriteByte('\n')
	}

	// Body
	if t.Body != "" {
		b.WriteByte('\n')
		b.WriteString(heading.Render("Body:"))
		b.WriteByte('\n')
		b.WriteString(body.Render(wrapIndent(t.Body, wrapWidth, "  ")))
		b.WriteByte('\n')
	}

	// Class and Type (shown on one block if either is present)
	if t.Class != "" || t.TaskType != "" {
		b.WriteByte('\n')
		if t.Class != "" {
			b.WriteString(body.Render("Class: " + t.Class))
			b.WriteByte('\n')
		}
		if t.TaskType != "" {
			b.WriteString(body.Render("Type: " + t.TaskType))
			b.WriteByte('\n')
		}
	}

	// Deliverables
	if len(t.Deliverables) > 0 {
		b.WriteByte('\n')
		b.WriteString(heading.Render("Deliverables:"))
		b.WriteByte('\n')
		for _, d := range t.Deliverables {
			b.WriteString(body.Render("  \u2022 " + d))
			b.WriteByte('\n')
		}
	}

	// Acceptance Criteria
	if len(t.AcceptanceCriteria) > 0 {
		b.WriteByte('\n')
		b.WriteString(heading.Render("Acceptance Criteria:"))
		b.WriteByte('\n')
		for _, c := range t.AcceptanceCriteria {
			b.WriteString(body.Render("  \u2022 " + c))
			b.WriteByte('\n')
		}
	}

	// Constraints
	if len(t.Constraints) > 0 {
		b.WriteByte('\n')
		b.WriteString(heading.Render("Constraints:"))
		b.WriteByte('\n')
		for _, c := range t.Constraints {
			b.WriteString(body.Render("  \u2022 " + c))
			b.WriteByte('\n')
		}
	}

	// References
	if len(t.References) > 0 {
		b.WriteByte('\n')
		b.WriteString(heading.Render("References:"))
		b.WriteByte('\n')
		for _, r := range t.References {
			b.WriteString(body.Render("  " + r))
			b.WriteByte('\n')
		}
	}

	// Block Reason, Failures, Last Failure, Needs Decomposition, Is Audit
	// Only rendered when they carry meaningful data.
	hasStatusFields := t.BlockedReason != "" || t.FailureCount > 0 || t.LastFailureType != "" || t.NeedsDecomposition || t.IsAudit
	if hasStatusFields {
		b.WriteByte('\n')

		if t.BlockedReason != "" {
			b.WriteString(body.Render("Block Reason: " + t.BlockedReason))
			b.WriteByte('\n')
		}

		if t.FailureCount > 0 {
			b.WriteString(body.Render(fmt.Sprintf("Failures: %d", t.FailureCount)))
			b.WriteByte('\n')
		}

		if t.LastFailureType != "" {
			b.WriteString(body.Render("Last Failure: " + t.LastFailureType))
			b.WriteByte('\n')
		}

		if t.NeedsDecomposition {
			b.WriteString(body.Render("Needs Decomposition: yes"))
			b.WriteByte('\n')
		}

		if t.IsAudit {
			b.WriteString(body.Render("Is Audit: yes"))
			b.WriteByte('\n')
		}
	}

	m.viewport.SetContent(b.String())
}

func boolYesNo(v bool) string {
	if v {
		return "yes"
	}
	return "no"
}
