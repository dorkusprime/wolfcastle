package daemon

import (
	"fmt"
	"strings"

	"github.com/dorkusprime/wolfcastle/internal/output"
	"github.com/dorkusprime/wolfcastle/internal/state"
)

// specReviewTaskType is the TaskType assigned to auto-created review tasks.
const specReviewTaskType = "spec-review"

// SpecReviewPromptFile is the prompt template used for spec review tasks.
// Exported for use by the pipeline context builder.
const SpecReviewPromptFile = "spec-review.md"

// checkSpecReviewNeeded examines a just-completed task. If it was a "spec"
// task, this function creates a sibling review task that will audit the
// spec before implementation proceeds. The review task references the
// same spec files, carries a dedicated prompt, and blocks further work
// until the review passes.
//
// Returns true if a review task was created.
func (d *Daemon) checkSpecReviewNeeded(nodeAddr, taskID string) bool {
	ns, err := d.Store.ReadNode(nodeAddr)
	if err != nil {
		return false
	}

	// Find the completed task and check its type.
	var specTask *state.Task
	for i := range ns.Tasks {
		if ns.Tasks[i].ID == taskID {
			specTask = &ns.Tasks[i]
			break
		}
	}
	if specTask == nil {
		return false
	}

	// Only spec tasks trigger review.
	if specTask.TaskType != "spec" {
		return false
	}

	// Don't create a review for a task that is itself a review.
	if specTask.TaskType == specReviewTaskType {
		return false
	}

	// Check whether a review task already exists for this spec task.
	// The review ID is deterministic: "{specTaskID}-review".
	reviewID := taskID + "-review"
	for _, t := range ns.Tasks {
		if t.ID == reviewID {
			// Already created (perhaps from a prior completion attempt).
			return false
		}
	}

	// Build the review task description from the spec task.
	desc := fmt.Sprintf("Review spec: %s", specTask.Description)

	// Collect spec references. The review task should point at the same
	// specs so the reviewer model can read them inline.
	var refs []string
	refs = append(refs, specTask.References...)
	// Also pull in any node-level specs.
	refs = append(refs, ns.Specs...)

	// Build a body that tells the reviewer what to look at.
	var body strings.Builder
	body.WriteString("## Review Target\n\n")
	fmt.Fprintf(&body, "This is an automated review of spec task `%s`.\n\n", taskID)
	if specTask.Body != "" {
		body.WriteString("### Original Spec Task Body\n\n")
		body.WriteString(specTask.Body)
		body.WriteString("\n\n")
	}
	if len(refs) > 0 {
		body.WriteString("### Spec Documents\n\n")
		for _, r := range refs {
			fmt.Fprintf(&body, "- `%s`\n", r)
		}
		body.WriteString("\n")
	}
	body.WriteString("Review this spec for: logical gaps, missing method signatures, ")
	body.WriteString("contradictions, under-specified behavior, incomplete error handling, ")
	body.WriteString("and missing edge cases.\n")

	// Create the review task via mutation.
	mutErr := d.Store.MutateNode(nodeAddr, func(ns *state.NodeState) error {
		reviewTask := state.Task{
			ID:          reviewID,
			Title:       fmt.Sprintf("Spec Review: %s", specTask.Description),
			Description: desc,
			State:       state.StatusNotStarted,
			TaskType:    specReviewTaskType,
			Body:        body.String(),
			References:  refs,
		}

		// Insert before the audit task (audit is always last).
		insertIdx := len(ns.Tasks)
		for i, t := range ns.Tasks {
			if t.IsAudit {
				insertIdx = i
				break
			}
		}
		ns.Tasks = append(ns.Tasks[:insertIdx],
			append([]state.Task{reviewTask}, ns.Tasks[insertIdx:]...)...)

		state.MoveAuditLast(ns)
		return nil
	})

	if mutErr != nil {
		_ = d.Logger.Log(map[string]any{
			"type":  "spec_review_create_error",
			"node":  nodeAddr,
			"task":  taskID,
			"error": mutErr.Error(),
		})
		return false
	}

	_ = d.Logger.Log(map[string]any{
		"type":      "spec_review_created",
		"node":      nodeAddr,
		"spec_task": taskID,
		"review_id": reviewID,
	})
	output.PrintHuman("  Spec review queued: %s", reviewID)
	return true
}

// handleSpecReviewBlocked processes a blocked spec-review task by feeding
// the review feedback back to the original spec task. It unblocks the
// original spec task (resetting it to not_started) so it can be revised,
// and records the review issues in the spec task's body.
//
// Returns true if feedback was delivered.
func (d *Daemon) handleSpecReviewBlocked(nodeAddr, taskID string) bool {
	ns, err := d.Store.ReadNode(nodeAddr)
	if err != nil {
		return false
	}

	// Find the review task.
	var reviewTask *state.Task
	for i := range ns.Tasks {
		if ns.Tasks[i].ID == taskID {
			reviewTask = &ns.Tasks[i]
			break
		}
	}
	if reviewTask == nil || reviewTask.TaskType != specReviewTaskType {
		return false
	}

	// The original spec task ID is the review ID minus the "-review" suffix.
	if !strings.HasSuffix(taskID, "-review") {
		return false
	}
	specTaskID := strings.TrimSuffix(taskID, "-review")

	// Feed the blocked reason back as revision guidance.
	feedback := reviewTask.BlockedReason
	if feedback == "" {
		feedback = "Spec review identified issues (see review task body for details)."
	}

	mutErr := d.Store.MutateNode(nodeAddr, func(ns *state.NodeState) error {
		for i := range ns.Tasks {
			if ns.Tasks[i].ID == specTaskID {
				// Reset the spec task for revision.
				ns.Tasks[i].State = state.StatusNotStarted
				ns.Tasks[i].FailureCount = 0
				ns.Tasks[i].LastFailureType = ""

				// Append review feedback to the body.
				ns.Tasks[i].Body += "\n\n## Review Feedback (Revision Required)\n\n" + feedback
				break
			}
		}
		return nil
	})

	if mutErr != nil {
		return false
	}

	_ = d.Logger.Log(map[string]any{
		"type":      "spec_review_feedback",
		"node":      nodeAddr,
		"review_id": taskID,
		"spec_task": specTaskID,
	})
	output.PrintHuman("  Spec review failed, revision queued for %s", specTaskID)
	return true
}
